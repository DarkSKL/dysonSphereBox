package saveanalysis

// 文件职责：把成品积压接收端的缺料接入同快照的来源查询。使用：由产物诊断调用。
// 边界：仅以实际未请求来料且缺料的接收机为起点；共线可达不能确定某一件货的归属。

import "slices"

// OutputSupply 包含下游接收机的上游供应图。节点索引局限于此图；根实体仍是所选产物的设备。
type OutputSupply struct {
	Nodes  []ChainNode  `json:"nodes"`
	Checks []ChainCheck `json:"checks"`
}

// unionEntities 合并两组可达设备，不修改输入或按经过路径的次数重复计数。
func unionEntities(a, b []int64) []int64 {
	all := append(slices.Clone(a), b...)
	slices.Sort(all)
	return slices.Compact(all)
}

// linkReceiverSupply 只展开实际遇到的接收机，共用配方与寻路缓存；新图单独分配节点下标。
func (w *chainWalk) linkReceiverSupply(out *OutputDiagnosis) error {
	planet := w.report.PlanetID
	supply := chainWalk{s: w.s, ids: map[chainKey]int{}, catalog: w.catalog, inputs: w.inputs}
	entries := map[int64]chainImpact{}
	for _, c := range out.Checks {
		if c.Status != "consumer-input-missing" {
			continue
		}
		for _, idx := range c.Receivers {
			v := &out.Receivers[idx]
			// 同一配方的缺料约束与选择哪个成品无关，固定用首个产物作节点身份。
			if v.Machine == nil || len(v.Machine.Outputs) == 0 {
				continue
			}
			product := v.Machine.Outputs[0].ItemID
			node := ChainNode{PlanetID: planet, EntityID: v.EntityID, ProductID: product, Machine: v.Machine}
			i := supply.intern(chainKey{planet, v.EntityID, product, -1}, node)
			missing := ChainConstraint{Status: "input-missing", ItemID: c.ItemID}
			pivot := &supply.report.Nodes[i]
			if !slices.Contains(pivot.Constraints, missing) {
				pivot.Constraints = append(pivot.Constraints, missing)
			}
			v.SupplyNode = &i
			entry, exists := entries[v.EntityID]
			if !exists {
				supply.report.Roots = append(supply.report.Roots, i)
			}
			entry.roots = unionEntities(entry.roots, c.RootEntities)
			entry.shared = entry.shared || c.Shared
			entry.unresolved = entry.unresolved || c.Unresolved
			entries[v.EntityID] = entry
		}
	}
	if len(entries) == 0 {
		return nil
	}
	if err := supply.expandAll(); err != nil {
		return err
	}
	impacts := supply.report.impacts()
	for i := range impacts {
		v := &impacts[i]
		v.receivers = v.roots
		slices.Sort(v.receivers)
		v.roots = nil
		for _, entity := range v.receivers {
			entry := entries[entity]
			v.roots = unionEntities(v.roots, entry.roots)
			v.shared = v.shared || entry.shared
			v.unresolved = v.unresolved || entry.unresolved
		}
	}
	supply.report.prioritizeImpacts(impacts)
	out.Supply = &OutputSupply{Nodes: supply.report.Nodes, Checks: supply.report.Checks}
	return nil
}
