package build

// 文件职责：删除当前请求中明确无效的塔候选修饰，并保留其他有效布局能力。
// 使用：普通与超密度物流塔候选派发前；边界：不把无合流时的错误返回当作成品图等价证明。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/candidates"
)

// bufferStationExportCanMerge 职责：保守判断产物是否可能形成多个同物料出口；使用：排除必定不能合流的修饰；边界：未知生产者、无效数量或多台机器可能拆行时一律保留。
func (b *builder) bufferStationExportCanMerge(req Request) bool {
	if b.flowGraph == nil {
		return true
	}
	counts := map[int]int{}
	production := map[string]int{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			production[node.ID] = node.ProductionIndex
		}
	}
	for _, edge := range b.flowGraph.Edges {
		if edge.FlowKind != "target-output" && edge.FlowKind != "leftover" {
			continue
		}
		counts[edge.ItemID]++
		if counts[edge.ItemID] > 1 {
			return true
		}
		index, exists := production[edge.SourceNodeID]
		if !exists || index < 0 || index >= len(req.Plan.Productions) || req.Plan.Productions[index].MachineCount != 1 {
			return true
		}
	}
	return false
}

// stationSingleOutputRow 职责：证明单台独立生产者不需要出货分流或输入分流器；使用：移除无缓存时的产物与双侧过滤修饰；边界：内部边、未知节点和多出口一律保留，不从目标数量推断拓扑。
func (b *builder) stationSingleOutputRow(req Request) bool {
	if b.flowGraph == nil || len(req.Plan.Productions) != 1 || req.Plan.Productions[0].MachineCount != 1 {
		return false
	}
	production := ""
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			if production != "" || node.ProductionIndex != 0 {
				return false
			}
			production = node.ID
		}
	}
	if production == "" {
		return false
	}
	outputs := 0
	for _, edge := range b.flowGraph.Edges {
		if edge.FlowKind == "internal" {
			return false
		}
		if edge.SourceNodeID == production {
			if edge.FlowKind != "target-output" && edge.FlowKind != "leftover" {
				return false
			}
			outputs++
		} else if edge.FlowKind != "external-input" || edge.TargetNodeID != production {
			return false
		}
	}
	return outputs == 1
}

// normalizeStationPlans 职责：移除已证明不可用的修饰后保留具名计划顺序；使用：塔候选装配之前；边界：共享出口无合流原本返回错误，此处只去掉该修饰，保留压缩等其余能力并重新完整验证。
func (b *builder) normalizeStationPlans(ctx context.Context, req Request, plans []stationCandidatePlan) ([]stationCandidatePlan, int) {
	sharedOutputs := b.bufferStationExportCanMerge(req)
	root, _ := ctx.Value(densityPolarRootKey{}).(bool)
	noOutputBranches := b.setting.EnableIntermediateBuffer || b.stationSingleOutputRow(req)
	result := make([]stationCandidatePlan, 0, len(plans))
	inactive := 0
	balanced := []stationCandidatePlan{}
	preferBalanced := b.setting.EnableIntermediateBuffer && root && denseNativeLargeRequest(ctx, req)
	for _, plan := range plans {
		spec := &plan.spec
		isBalanced := spec.BalancedFrame
		for _, entry := range []struct {
			enabled *bool
			remove  bool
		}{
			{&spec.ProducerSplits, noOutputBranches}, {&spec.PairedOutputs, noOutputBranches},
			{&spec.SharedOutputs, !sharedOutputs}, {&spec.SharedInputs, b.setting.EnableIntermediateBuffer && !root},
			{&spec.IndependentStations, root},
		} {
			if *entry.enabled && entry.remove {
				*entry.enabled = false
				inactive++
			}
		}
		if !spec.ProducerSplits {
			spec.SplitterModel = 0
		}
		if preferBalanced && isBalanced {
			balanced = append(balanced, plan)
		} else {
			result = append(result, plan)
		}
	}
	return candidates.StableUnique(append(balanced, result...), func(plan stationCandidatePlan) stationCandidatePlan { return plan }), inactive
}
