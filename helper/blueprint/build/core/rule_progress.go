package build

// 文件职责：将实际进入的最终规则转换为可视化阶段；使用：单项规则入口；边界：只报告当前工作，不估算总百分比，不宣称规则已经通过。

// ruleProgressMessage 职责：描述当前规则正在检查的实体关系；使用：真实规则调用前；边界：未知规则只显示通用校验状态。
func ruleProgressMessage(kind RuleKind) string {
	switch kind {
	case RuleReferences, RulePastedSlots:
		return "正在核验最终建筑与端口连接"
	case RuleLatitude:
		return "正在核验最终建筑的纬带范围"
	case RuleBeltDistance, RuleBeltCrossings, RuleBeltBuildingOccupancy:
		return "正在核验传送带间距与占位"
	case RuleStaticColliders, RuleSplitterClearance, RuleStorageRoof:
		return "正在核验建筑净空与碰撞"
	case RuleInserterGeometry:
		return "正在核验分拣器跨度与动态占位"
	case RuleTankPorts, RuleStationPorts, RuleStationSpacing:
		return "正在核验物流设施间距与接驳"
	case RulePowerSpacing, RulePowerNetwork:
		return "正在核验供电覆盖与电网连通"
	case RuleMachines, RuleSprayChannels, RuleSelfSpray:
		return "正在核验生产设备与喷涂供给"
	case RuleLayoutMetrics, RuleDensityReport:
		return "正在测量最终布局形状与密度"
	case RuleTransportTopology, RuleByproductPriority, RuleBuffers, RuleDeliveries, RuleProducerSplits, RuleMaterialNetworks, RuleRoutes, RuleRecycles, RuleLeftovers, RuleSharedCapacity, RuleConservation, RuleBoundaries, RuleMonitors:
		return "正在核验物料路径、容量与守恒"
	default:
		return "正在核验最终蓝图"
	}
}
