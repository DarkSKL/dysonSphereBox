package saveanalysis

// 文件职责：从供应证据图计算可达目标、直接缺料设备和检查顺序。
// 使用：查询组图完成后调用；边界：共享分支按实体去重，覆盖数只作为检查顺序依据。

import (
	"cmp"
	"fmt"
	"slices"
)

// chainImpact 记录一个检查点可关联的目标设备，以及途中的共享和未决证据。
type chainImpact struct {
	roots              []int64
	receivers          []int64
	shared, unresolved bool
}

// impacts 从各目标设备沿证据图传播可达性。即使多路汇合或环回，同一目标只计一次。
func (r *ProductionDiagnosis) impacts() []chainImpact {
	impacts := make([]chainImpact, len(r.Nodes))
	const seen, shared, unresolved = 1, 2, 4
	for _, root := range r.Roots {
		if len(r.Nodes[root].Constraints) == 0 {
			continue
		}
		flags := map[int]int{root: seen}
		queue := []int{root}
		for pos := 0; pos < len(queue); pos++ {
			i := queue[pos]
			n := r.Nodes[i]
			visit := func(dst, extra int) {
				v := flags[i] | extra
				if flags[dst]|v != flags[dst] {
					flags[dst] |= v
					queue = append(queue, dst)
				}
			}
			for _, l := range n.Inputs {
				extra := 0
				if l.Shared {
					extra |= shared
				}
				if l.Unresolved {
					extra |= unresolved
				}
				for _, dst := range l.Sources {
					visit(dst, extra)
				}
			}
			for _, dst := range n.LoadedSources {
				visit(dst, 0)
			}
		}
		for idx, f := range flags {
			v := &impacts[idx]
			v.roots = append(v.roots, r.Nodes[root].EntityID)
			v.shared = v.shared || f&shared != 0
			v.unresolved = v.unresolved || f&unresolved != 0
		}
	}
	return impacts
}

// prioritize 优先列已观测限制，再按关联目标数和直接缺料设备数排序；同等覆盖优先核对空槽。
func (r *ProductionDiagnosis) prioritize() {
	r.prioritizeImpacts(r.impacts())
}

// prioritizeImpacts 使用传入的目标实体归属生成建议。下游转查的覆盖数仍以最初选择的产品设备计。
func (r *ProductionDiagnosis) prioritizeImpacts(impacts []chainImpact) {
	direct := make([][]int64, len(r.Nodes))
	drops := map[[3]int64]DropObservation{}
	for _, n := range r.Nodes {
		for _, l := range n.Inputs {
			for _, feed := range l.BlockedFeeds {
				for _, d := range feed.Drops {
					key := [3]int64{n.PlanetID, d.Source, d.ItemID}
					if prev, ok := drops[key]; !ok || d.Sorter < prev.Sorter {
						drops[key] = d
					}
				}
			}
			for _, src := range l.Sources {
				if n.Freight == nil && r.Nodes[src].Freight != nil {
					direct[src] = append(direct[src], n.EntityID)
				}
			}
		}
	}
	type machineIssue struct {
		planet, entity, item int64
		status               string
	}
	machineChecks := map[machineIssue]int{}
	for idx, n := range r.Nodes {
		impact := impacts[idx]
		if len(impact.roots) == 0 {
			continue
		}
		check := ChainCheck{Node: idx, RootEntities: impact.roots, ReceiverEntities: impact.receivers, Shared: impact.shared || len(impact.roots) > 1, Unresolved: impact.unresolved}
		if n.Freight != nil {
			affected := direct[idx]
			if len(affected) == 0 {
				continue
			}
			slices.Sort(affected)
			check.DirectEntities = slices.Compact(affected)
			f := n.Freight
			check.Evidence = fmt.Sprintf("槽内 %d/%d 件，%d 台直接缺料设备可达该槽；%d 条活动来货或取货任务", f.Slot.Inventory, f.Slot.Capacity, len(check.DirectEntities), len(f.Tasks))
			check.Unresolved = check.Unresolved || slices.ContainsFunc(f.Tasks, func(t FreightTask) bool { return t.Phase == "carrying" && t.SourceIndex < 0 })
			check.ItemID = f.Slot.ItemID
			check.Shared = check.Shared || len(check.DirectEntities) > 1
			if f.Slot.Inventory == 0 {
				check.Status = "empty-slot"
				check.Advice = fmt.Sprintf("先查 %d 星球 %d 号站第 %d 槽。库存为零，实载在途 %d 件，空载预约 %d 件；核对来货和入槽后的出带", n.PlanetID, f.Slot.StationID, f.Slot.Index+1, f.CargoInFlight, f.AwaitingPickup)
			} else {
				check.Status = "stocked-slot"
				check.Advice = fmt.Sprintf("查 %d 星球 %d 号站第 %d 槽的出带和取料口，槽内还有 %d 件", n.PlanetID, f.Slot.StationID, f.Slot.Index+1, f.Slot.Inventory)
			}
			r.Checks = append(r.Checks, check)
			continue
		}
		for _, c := range n.Constraints {
			v := check
			v.Status = c.Status
			v.ItemID = c.ItemID
			v.DirectEntities = []int64{n.EntityID}
			v.Evidence = n.evidence(c)
			switch c.Status {
			case "input-missing":
				v.Advice = fmt.Sprintf("查实体 %d 的物品 %d 进料口，核对来料与本批需求", n.EntityID, c.ItemID)
				for _, l := range n.Inputs {
					if l.ItemID == c.ItemID {
						v.UpstreamConstraint = slices.ContainsFunc(l.Sources, func(i int) bool {
							src := r.Nodes[i]
							if src.Freight != nil {
								return src.Freight.Slot.Inventory == 0
							}
							return slices.ContainsFunc(src.Constraints, func(c ChainConstraint) bool { return c.Status != "undetermined" })
						})
						v.Shared = v.Shared || l.Shared
						v.Unresolved = v.Unresolved || l.Unresolved || len(l.Sources) == 0
					}
				}
			case "output-blocked":
				v.Advice = fmt.Sprintf("查实体 %d 的物品 %d 输出缓冲及接收端", n.EntityID, c.ItemID)
				if d, ok := drops[[3]int64{n.PlanetID, n.EntityID, c.ItemID}]; ok {
					v.Drop = &d
					v.Evidence += fmt.Sprintf("，出料分拣器 %d 正持有 %d 件，带段 %d 的投放点拒收", d.Sorter, d.Count, d.Belt)
					v.Advice = fmt.Sprintf("检查实体 %d 的出料分拣器 %d 和带段 %d 的投放点", n.EntityID, d.Sorter, d.Belt)
				}
			case "return-blocked":
				v.Advice = fmt.Sprintf("查实体 %d 的回流口与回流带", n.EntityID)
			case "low-power":
				v.Advice = fmt.Sprintf("查实体 %d 的供电", n.EntityID)
			case "no-source":
				v.Advice = fmt.Sprintf("查实体 %d 的采集覆盖与已绑定矿点", n.EntityID)
			default:
				v.Unresolved = true
				v.Advice = fmt.Sprintf("核实实体 %d 的工作状态，快照证据不足", n.EntityID)
			}
			// 同一实体可同时供应两种产物，同一条本机限制只推荐一次。
			key := machineIssue{n.PlanetID, n.EntityID, c.ItemID, c.Status}
			if j, ok := machineChecks[key]; ok {
				old := &r.Checks[j]
				ids := append(slices.Clone(old.RootEntities), v.RootEntities...)
				slices.Sort(ids)
				old.RootEntities = slices.Compact(ids)
				old.ReceiverEntities = unionEntities(old.ReceiverEntities, v.ReceiverEntities)
				old.Shared = true
				old.Unresolved = old.Unresolved || v.Unresolved
				old.UpstreamConstraint = old.UpstreamConstraint || v.UpstreamConstraint
			} else {
				machineChecks[key] = len(r.Checks)
				r.Checks = append(r.Checks, v)
			}
		}
	}
	compare := func(a, b ChainCheck) int {
		if (a.Status == "undetermined") != (b.Status == "undetermined") {
			if a.Status == "undetermined" {
				return 1
			}
			return -1
		}
		if n := cmp.Compare(len(b.RootEntities), len(a.RootEntities)); n != 0 {
			return n
		}
		if n := cmp.Compare(len(b.DirectEntities), len(a.DirectEntities)); n != 0 {
			return n
		}
		if (a.Status == "empty-slot") != (b.Status == "empty-slot") {
			if a.Status == "empty-slot" {
				return -1
			}
			return 1
		}
		if a.UpstreamConstraint != b.UpstreamConstraint {
			if a.UpstreamConstraint {
				return 1
			}
			return -1
		}
		return 0
	}
	slices.SortStableFunc(r.Checks, compare)
	rank := 0
	for i := range r.Checks {
		if i == 0 || compare(r.Checks[i-1], r.Checks[i]) != 0 {
			rank++
		}
		r.Checks[i].Priority = rank
	}
}

// evidence 提取支持当前检查项的保存字段，具体设备的完整数值仍可在节点中复查。
func (n ChainNode) evidence(c ChainConstraint) string {
	var inputs []CircuitInput
	if m := n.Machine; m != nil {
		inputs = m.Inputs
		if c.Status == "output-blocked" {
			for _, o := range m.Outputs {
				if o.ItemID == c.ItemID {
					return fmt.Sprintf("成品缓冲 %d 件，当前配方交付门槛 %d 件", o.Buffered, o.DeliveryLimit)
				}
			}
		}
		if c.Status == "low-power" {
			return fmt.Sprintf("设备保存的供电比例 %.3f", m.Power)
		}
	} else if d := n.Device; d != nil {
		inputs = d.Inputs
		switch c.Status {
		case "low-power":
			return fmt.Sprintf("设备保存的供电比例 %.3f", d.Power)
		case "no-source":
			return "设备没有绑定矿点"
		case "return-blocked":
			return fmt.Sprintf("回流缓冲 %d/%d", d.FluidReturn, d.ReturnCapacity)
		case "output-blocked":
			return fmt.Sprintf("成品缓冲 %d 件，当前容量 %d", d.Buffered, d.Capacity)
		case "input-missing":
			if d.Kind == "fractionator" {
				return fmt.Sprintf("分馏塔未运行，进料缓冲 %d", d.FluidInput)
			}
		}
	}
	for _, v := range inputs {
		if c.Status == "input-missing" && v.ItemID == c.ItemID {
			return fmt.Sprintf("尚未开始下一批，物品 %d 机内有 %d 件，每批需 %d 件", v.ItemID, v.Available, v.Required)
		}
	}
	return "保存的设备状态不足以确定当前限制"
}
