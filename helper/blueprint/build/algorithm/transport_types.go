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
	// AreaIndex 职责：标识最终建筑所在蓝图区域；使用：结合本地坐标定位端点；边界：不是行星世界区域编号。
	AreaIndex int `json:"areaIndex"`
	// X 职责：记录最终编码建筑的 Area 局部横坐标；使用：定位端口；边界：不是装配平面或游戏世界绝对坐标。
	X float64 `json:"x"`
	// Y 职责：记录最终编码建筑的 Area 局部纵坐标；使用：定位端口；边界：必须结合 AreaIndex，不能跨 Area 直接比较距离。
	Y float64 `json:"y"`
	// Z 职责：记录最终编码建筑的 Area 局部高度；使用：定位端口；边界：与蓝图 LocalOffset.Z 同单位。
	Z float64 `json:"z"`
}

// ProductionRateAnnotation 职责：关联机器带数字与期望、配置满负荷两种速率；使用：预览定位和产能核对；边界：机器口为毛流，不是全厂实际吞吐或新的守恒边。
type ProductionRateAnnotation struct {
	// ProductionIndex 职责：关联可信产线；使用：卡片联动；边界：不从配方编号推测归属。
	ProductionIndex int `json:"productionIndex"`
	// RecipeID 职责：记录本行真实配方；使用：诊断；边界：不改变所选配方。
	RecipeID int `json:"recipeId"`
	// ItemID 职责：确定机器带物品；使用：图标和速率核对；边界：不同物品不共享标注。
	ItemID int `json:"itemId"`
	// Direction 职责：区分机器 input/output；使用：解释毛流方向；边界：不是蓝图外部边界方向。
	Direction string `json:"direction"`
	// MachineCount 职责：记录该带实际服务的整数机器数；使用：拆行速率换算；边界：包含堆叠机器而非地面列数。
	MachineCount int `json:"machineCount"`
	// BuildingIndex 职责：定位最终机器分拣器接带格；使用：预览和编码复核；边界：是最终建筑数组下标。
	BuildingIndex int `json:"buildingIndex"`
	// PlannedRatePerMinute 职责：记录本行按期望利用率折算的机器毛流；使用：双速率对比；边界：含本地回流，不可累加替代规范净流边。
	PlannedRatePerMinute float64 `json:"plannedRatePerMinute"`
	// PlannedNetRatePerMinute 职责：记录本行分得的期望净流量；使用：与量化账对照；边界：回流专用端口可为零。
	PlannedNetRatePerMinute float64 `json:"plannedNetRatePerMinute"`
	// ConfiguredRatePerMinute 职责：记录本行设备配置满负荷毛流；使用：蓝图机器带数字标注；边界：需充足供料、电力和畅通出货，不等于实测。
	ConfiguredRatePerMinute float64 `json:"configuredRatePerMinute"`
	// DisplayedRatePerMinute 职责：记录实际写入游戏传送带标记的整数；使用：独立核对编码结果；边界：向上取整显示，不改变精确容量计算。
	DisplayedRatePerMinute int `json:"displayedRatePerMinute"`
	// Basis 职责：标识配置满负荷来源；使用：旧结果兼容；边界：缺少可信能力字段时不伪造。
	Basis string `json:"basis"`
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

// SearchCandidateDiagnostic 职责：记录一次有限布局候选的稳定身份和终检结果；使用：成功摘要复核搜索决策；边界：仅记录已运行候选，不改变候选预算、排序或生成结果。
type SearchCandidateDiagnostic struct {
	// ParentKey 职责：关联外层布局候选；使用：区分嵌套球面候选；边界：顶层候选为空。
	ParentKey string `json:"parentKey,omitempty"`
	// Key 职责：保存候选稳定键；使用：重放与回归；边界：不包含运行时计时或建筑对象地址。
	Key string `json:"key"`
	// Parameters 职责：按固定字段顺序保存影响本候选的参数；使用：复现高度、轮廓、入口或宽度设置；边界：不包含由候选计算出的完整建筑数据。
	Parameters string `json:"parameters"`
	// Scope 职责：标识候选搜索类型；使用：分组审计；边界：采用稳定机器标识而非界面文案。
	Scope string `json:"scope"`
	// Stage 职责：说明候选经过的最后阶段；使用：区分装配、球面装箱与最终终检；边界：不是整体生成阶段进度。
	Stage string `json:"stage"`
	// Status 职责：标识候选通过或被拒绝；使用：统计实际可行候选；边界：只有通过完整相应阶段的候选才为 passed。
	Status string `json:"status"`
	// ErrorCode 职责：记录失败分类；使用：定位拒绝规则；边界：内部非蓝图错误使用 GEN_E_INTERNAL 诊断归类，不改写实际返回错误。
	ErrorCode string `json:"errorCode,omitempty"`
	// Reason 职责：保存中文通过或拒绝原因；使用：解释搜索选择；边界：最长512个字符，截断不影响实际错误返回。
	Reason string `json:"reason"`
}

// LayoutMetrics 职责：记录候选预算内可复现的空间和路线指标；使用：紧凑度回归；边界：有界启发式不等于数学全局最优。
type LayoutMetrics struct {
	// EnvelopeArea 职责：记录所有 Area 包络面积总和；使用：空间比较；边界：不是建筑占用并集。
	EnvelopeArea float64 `json:"envelopeArea"`
	// EnvelopeAspectRatio 职责：记录各 Area 长宽比的面积加权值；使用：偏矩形布局比较；边界：至少为一，不把球面多区假装成同一平面矩形。
	EnvelopeAspectRatio float64 `json:"envelopeAspectRatio"`
	// ShapeObjective 职责：记录包络面积乘过长惩罚；使用：可行候选排序；边界：长宽比不超过二时不惩罚，不能替代硬规则校验。
	ShapeObjective float64 `json:"shapeObjective"`
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
	// CrossLayerConnections 职责：记录真实相连且高度变化的带段数；使用：升降成本和立体走廊诊断；边界：逐实体连接计数，共享带不按物料边重复计算，不把平行叠层或设施连接当作升降。
	CrossLayerConnections int `json:"crossLayerConnections"`
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
	// RateAnnotations 职责：记录真实机器带的期望与配置速率；使用：蓝图数字和卡片对齐；边界：不代表跨产线物流同时满负荷已验证。
	RateAnnotations []ProductionRateAnnotation `json:"rateAnnotations,omitempty"`
	// Checks 职责：解释当前覆盖；使用：诊断；边界：不能用告警掩盖硬失败。
	Checks []LogisticsCheck `json:"checks"`
	// Metrics 职责：记录空间目标和搜索预算；使用：回归比较；边界：保留有限候选含义。
	Metrics LayoutMetrics `json:"metrics"`
	// SearchCandidates 职责：保存固定候选预算内按实际顺序评估的布局结果；使用：审计成功蓝图为何选中及其它候选为何被拒；边界：数量受各候选搜索的既有限制约束，不收录 A* 节点或部分装箱探测。
	SearchCandidates []SearchCandidateDiagnostic `json:"searchCandidates,omitempty"`
}
