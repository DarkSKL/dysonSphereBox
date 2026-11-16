package build

// 文件职责：提供每项真实规则的具名纯核心入口。
// 使用：生产目录为每个入口登记独立工具ID和typed proof key；边界：不在工具内循环全部规则。

import "context"

// CheckReferences 职责：执行references单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckReferences(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleReferences, deps...)
}

// CheckLatitude 职责：执行latitude单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckLatitude(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleLatitude, deps...)
}

// CheckMachines 职责：执行machines单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckMachines(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleMachines, deps...)
}

// CheckPastedSlots 职责：执行pasted-slots单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckPastedSlots(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RulePastedSlots, deps...)
}

// CheckBeltDistance 职责：执行belt-distance单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckBeltDistance(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleBeltDistance, deps...)
}

// CheckStaticColliders 职责：执行static-colliders单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckStaticColliders(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleStaticColliders, deps...)
}

// CheckTankPorts 职责：执行tank-ports单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckTankPorts(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleTankPorts, deps...)
}

// CheckPowerSpacing 职责：执行power-spacing单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckPowerSpacing(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RulePowerSpacing, deps...)
}

// CheckStationSpacing 职责：执行station-spacing单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckStationSpacing(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleStationSpacing, deps...)
}

// CheckBeltCrossings 职责：执行belt-crossings单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckBeltCrossings(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleBeltCrossings, deps...)
}

// CheckBeltBuildingOccupancy 职责：执行belt-building-occupancy单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckBeltBuildingOccupancy(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleBeltBuildingOccupancy, deps...)
}

// CheckInserterGeometry 职责：执行inserter-geometry单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckInserterGeometry(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleInserterGeometry, deps...)
}

// CheckSplitterClearance 职责：执行splitter-clearance单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckSplitterClearance(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleSplitterClearance, deps...)
}

// CheckStationPorts 职责：执行station-ports单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckStationPorts(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleStationPorts, deps...)
}

// CheckStorageRoof 职责：执行storage-roof单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckStorageRoof(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleStorageRoof, deps...)
}

// CheckPowerNetwork 职责：执行power-network单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckPowerNetwork(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RulePowerNetwork, deps...)
}

// CheckSprayChannels 职责：执行spray-channels单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckSprayChannels(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleSprayChannels, deps...)
}

// CheckTransportTopology 职责：执行transport-topology单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckTransportTopology(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleTransportTopology, deps...)
}

// CheckByproductPriority 职责：执行byproduct-priority单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckByproductPriority(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleByproductPriority, deps...)
}

// CheckBuffers 职责：执行buffers单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckBuffers(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleBuffers, deps...)
}

// CheckDeliveries 职责：执行deliveries单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckDeliveries(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleDeliveries, deps...)
}

// CheckProducerSplits 职责：执行producer-splits单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckProducerSplits(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleProducerSplits, deps...)
}

// CheckMaterialNetworks 职责：执行material-networks单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckMaterialNetworks(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleMaterialNetworks, deps...)
}

// CheckRoutes 职责：执行routes单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckRoutes(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleRoutes, deps...)
}

// CheckRecycles 职责：执行recycles单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckRecycles(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleRecycles, deps...)
}

// CheckSelfSpray 职责：执行self-spray单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckSelfSpray(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleSelfSpray, deps...)
}

// CheckLeftovers 职责：执行leftovers单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckLeftovers(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleLeftovers, deps...)
}

// CheckSharedCapacity 职责：执行shared-capacity单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckSharedCapacity(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleSharedCapacity, deps...)
}

// CheckConservation 职责：执行conservation单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckConservation(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleConservation, deps...)
}

// CheckBoundaries 职责：执行boundaries单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckBoundaries(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleBoundaries, deps...)
}

// CheckMonitors 职责：执行monitors单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func CheckMonitors(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleMonitors, deps...)
}

// MeasureLayout 职责：执行layout-metrics单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func MeasureLayout(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleLayoutMetrics, deps...)
}

// MeasureDensity 职责：执行density-report单项职责；使用：独立工具步骤；边界：只消费明确前置证明，不补跑其它规则。
func MeasureDensity(ctx context.Context, g GeometrySnapshot, deps ...RuleProof) (RuleProof, error) {
	return CheckRule(ctx, g, RuleDensityReport, deps...)
}
