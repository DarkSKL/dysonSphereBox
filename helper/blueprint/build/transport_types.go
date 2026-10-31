package build

// 文件职责：定义生成器的逐物料运输证据和空间指标。
// 使用：前端展示容量、入口与规则覆盖；边界：只汇报真实建筑与连接，不声称游戏运行验收。

// TransportLane 职责：记录规范物料边的一条实际运输路线；使用：容量与连通自检；边界：同一边可以因机器端口分组产生多条 lane。
type TransportLane struct {
	// EdgeID 职责：关联规范供需边；使用：画板联动；边界：不由坐标推测。
	EdgeID string `json:"edgeId"`
	// ItemID 职责：确定物料与过滤器；使用：运输诊断；边界：单 lane 不混淆不同物品。
	ItemID int `json:"itemId"`
	// SourceNodeID 职责：关联规范上游节点；使用：供需追溯；边界：外部供应保留其节点类型。
	SourceNodeID string `json:"sourceNodeId"`
	// TargetNodeID 职责：关联规范下游节点；使用：供需追溯；边界：目标和副产物为不同节点。
	TargetNodeID string `json:"targetNodeId"`
	// FlowKind 职责：区分内部、原料、目标和剩余边界；使用：诊断分类；边界：沿用规范流图。
	FlowKind string `json:"flowKind"`
	// RatePerMinute 职责：记录分配后的真实每分钟流量；使用：守恒与容量；边界：不隐式换算秒。
	RatePerMinute float64 `json:"ratePerMinute"`
	// CapacityPerMinute 职责：记录所选等级单带运力；使用：利用率展示；边界：不乘虚构层数。
	CapacityPerMinute float64 `json:"capacityPerMinute"`
	// Utilization 职责：记录流量与运力比；使用：瓶颈展示；边界：不得大于一。
	Utilization float64 `json:"utilization"`
	// BuildingIndices 职责：关联完整路由链及过滤分拣器；使用：编码后逐带检查；边界：是数组下标而非任意外部建筑。
	BuildingIndices []int `json:"buildingIndices"`
	// InternalConnected 职责：标记内部边已验证存在实体链；使用：结果说明；边界：外部边不伪装成外界已经供货。
	InternalConnected bool `json:"internalConnected"`
}

// BoundaryPort 职责：记录玩家仍需接入的真实边界端点；使用：入口数与逐入口流量展示；边界：不包含内部尚未接好的假入口。
type BoundaryPort struct {
	// EdgeID 职责：关联供需边；使用：来源追踪；边界：不生成独立于流图的需求。
	EdgeID string `json:"edgeId"`
	// ItemID 职责：记录端口物料；使用：带标记与展示；边界：不同物料不共用无过滤出口。
	ItemID int `json:"itemId"`
	// Kind 职责：区分 external-input、target-output、leftover；使用：操作提示；边界：不推断游戏外设施。
	Kind string `json:"kind"`
	// RatePerMinute 职责：记录该端口分配流量；使用：入口守恒；边界：总量严格匹配规范边。
	RatePerMinute float64 `json:"ratePerMinute"`
	// BuildingIndex 职责：定位可继续连接的端带；使用：预览与诊断；边界：最终编号后回填。
	BuildingIndex int `json:"buildingIndex"`
	// X 职责：记录最终装配坐标；使用：定位端口；边界：不是锁定的游戏绝对坐标。
	X float64 `json:"x"`
	// Y 职责：记录最终装配坐标；使用：定位端口；边界：不是锁定的游戏绝对坐标。
	Y float64 `json:"y"`
	// Z 职责：记录高度层；使用：定位端口；边界：与蓝图局部高度同单位。
	Z float64 `json:"z"`
}

// LogisticsCheck 职责：解释已验证和未覆盖的物流规则；使用：结果诊断；边界：unverified 不能显示为通过。
type LogisticsCheck struct {
	// RuleID 职责：稳定标识规则；使用：回归与展示；边界：不是本地化文本。
	RuleID string `json:"ruleId"`
	// Status 职责：记录 passed 或 unverified；使用：覆盖提示；边界：硬失败不返回成功结果。
	Status string `json:"status"`
	// Message 职责：解释覆盖和边界；使用：用户提示；边界：不声称 Unity Physics 验收。
	Message string `json:"message"`
}

// LayoutMetrics 职责：记录候选预算内可复现的空间和路线指标；使用：紧凑度回归；边界：有界启发式不等于数学全局最优。
type LayoutMetrics struct {
	// EnvelopeArea 职责：记录所有 Area 包络面积总和；使用：空间比较；边界：不是建筑占用并集。
	EnvelopeArea float64 `json:"envelopeArea"`
	// FootprintArea 职责：记录离散格建筑与路线并集；使用：空白率；边界：格网近似而非美术网格。
	FootprintArea float64 `json:"footprintArea"`
	// BlankRatio 职责：记录包络中的空白比例；使用：紧凑诊断；边界：不以缩小碰撞外形压低此值。
	BlankRatio float64 `json:"blankRatio"`
	// InternalBeltLength 职责：记录内部 lane 的带长；使用：候选目标比较；边界：按实际点距计算。
	InternalBeltLength float64 `json:"internalBeltLength"`
	// RouteCrossings 职责：记录无拓扑连接的路线交叉；使用：安全验收；边界：成功必须为零。
	RouteCrossings int `json:"routeCrossings"`
	// CrossAreaConnections 职责：记录跨原生 Area 的带连接；使用：球面诊断；边界：按最终记录统计。
	CrossAreaConnections int `json:"crossAreaConnections"`
	// LaneCount 职责：记录实际分组路线数量；使用：容量诊断；边界：不是理论带数下界。
	LaneCount int `json:"laneCount"`
	// ExternalInputCount 职责：记录自动生成的真实外部入口数量；使用：接入说明；边界：不要求用户配置。
	ExternalInputCount int `json:"externalInputCount"`
	// CandidateBudget 职责：记录固定排布搜索预算；使用：最优性说明；边界：没有搜索的候选不计入评估。
	CandidateBudget int `json:"candidateBudget"`
	// EvaluatedCandidates 职责：记录已评估排布数；使用：性能与选择证据；边界：包含拒绝的候选。
	EvaluatedCandidates int `json:"evaluatedCandidates"`
	// RejectedCandidates 职责：记录硬约束拒绝数；使用：诊断；边界：拒绝不能返回半成品蓝图。
	RejectedCandidates int `json:"rejectedCandidates"`
}

// LogisticsSummary 职责：汇总规范流图、实际 lane、边界与有限规则验证；使用：所有模式统一诊断；边界：不代替真实游戏运行窗口。
type LogisticsSummary struct {
	// FlowGraphDigest 职责：记录规范边指纹；使用：前后端一致性；边界：与画板坐标无关。
	FlowGraphDigest string `json:"flowGraphDigest"`
	// BeltLevel 职责：记录用户选择的等级；使用：诊断；边界：生成器不静默升级。
	BeltLevel int `json:"beltLevel"`
	// BeltCapacityPerMinute 职责：记录唯一容量表结果；使用：UI 展示；边界：始终为单带。
	BeltCapacityPerMinute float64 `json:"beltCapacityPerMinute"`
	// Lanes 职责：记录各实体运输分组；使用：流量和端口追踪；边界：不制造没有建筑的装饰 lane。
	Lanes []TransportLane `json:"lanes"`
	// ExternalPorts 职责：记录外部边界；使用：入口统计；边界：只含玩家仍需接入的端点。
	ExternalPorts []BoundaryPort `json:"externalPorts"`
	// Checks 职责：解释当前覆盖；使用：诊断；边界：不能用告警掩盖硬失败。
	Checks []LogisticsCheck `json:"checks"`
	// Metrics 职责：记录空间目标和搜索预算；使用：回归比较；边界：保留有限候选含义。
	Metrics LayoutMetrics `json:"metrics"`
}
