package quant

// 文件职责：定义与几何无关的规范物料图契约；使用：量化、画板和所有蓝图模式共享；边界：每分钟是唯一速率单位，节点顺序不是蓝图坐标。

// FlowGraph 职责：保存守恒物料边、确定性 SCC 和生产顺序；使用：BuildFlowGraph 及其消费者；边界：ProductionIndex 只用于映射原快照，不参与规范摘要。
type FlowGraph struct {
	// Nodes 职责：列举稳定节点；使用：按 ID 查找生产者和边界；边界：每个生产实例独立，不按主产物合并。
	Nodes []FlowNode `json:"nodes"`
	// Edges 职责：列举真实供需配对；使用：画板和实体路线共同读取；边界：不产生全连接比例边。
	Edges []FlowEdge `json:"edges"`
	// Components 职责：保存 SCC 缩并后上游优先顺序；使用：循环布局；边界：循环边仍保留在 Edges。
	Components []FlowComponent `json:"components"`
	// ProductionOrder 职责：返回上游到下游的原生产索引；使用：局部装配顺序；边界：同 SCC 内采用稳定语义键。
	ProductionOrder []int `json:"productionOrder"`
	// Balances 职责：记录逐物品供需账；使用：诊断和回归；边界：浮点容差不代表额外可用流量。
	Balances []FlowBalance `json:"balances"`
	// Digest 职责：标识规范物料账和连边；使用：跨模式与数组置换回归；边界：不包含画板坐标、原索引或布局配置。
	Digest string `json:"digest"`
}

// FlowNode 职责：标识生产实例或蓝图边界；使用：物料边端点；边界：production ID 的序号是语义规范序，不是请求数组下标。
type FlowNode struct {
	// ID 职责：保存稳定唯一键；使用：边引用；边界：不用于推测物品或生产索引。
	ID string `json:"id"`
	// Kind 职责：区分 production/raw-source/external-source/target-sink/leftover-sink；使用：边界与内部拓扑；边界：外供不伪造本地图内来源。
	Kind string `json:"kind"`
	// ItemID 职责：保存边界物品或生产主产物；使用：名称展示；边界：生产节点可能还有其它物品端口。
	ItemID int `json:"itemId"`
	// ProductionIndex 职责：映射原 Productions 数组；使用：实体装配和预览联动；边界：边界节点为 -1，不参与 Digest。
	ProductionIndex int `json:"productionIndex"`
	// Level 职责：保存 SCC DAG 最长上游层级；使用：上游到下游排布；边界：同一 SCC 层级相同。
	Level int `json:"level"`
	// ComponentID 职责：标识所属 SCC；使用：循环路线约束；边界：不删除回边。
	ComponentID string `json:"componentId"`
}

// FlowEdge 职责：承载某生产者向某消费者的唯一物料配对；使用：物流 lane 规划；边界：速率必须有限且为正。
type FlowEdge struct {
	// ID 职责：保存稳定边键；使用：路线诊断与索引；边界：同物品同端点只存在一条。
	ID string `json:"edgeId"`
	// ItemID 职责：指定运输物品；使用：端口和过滤；边界：必须存在于数据集。
	ItemID int `json:"itemId"`
	// SourceNodeID 职责：引用供给节点；使用：连接起点；边界：不能引用不存在的节点。
	SourceNodeID string `json:"sourceNodeId"`
	// TargetNodeID 职责：引用消费节点；使用：连接终点；边界：不将原料入口当作消费端。
	TargetNodeID string `json:"targetNodeId"`
	// RatePerMinute 职责：保存分配后的每分钟流量；使用：容量核算；边界：不是设备满负载额定能力。
	RatePerMinute float64 `json:"ratePerMinute"`
	// FlowKind 职责：区分 internal/external-input/target-output/leftover；使用：边界说明；边界：不改变供需账。
	FlowKind string `json:"flowKind"`
}

// FlowComponent 职责：保存不可拆掉循环边的布局组；使用：拓扑排布；边界：组顺序是 DAG 顺序，不代表循环运输已验证。
type FlowComponent struct {
	// ID 职责：保存稳定 SCC 键；使用：节点归属；边界：由组内稳定节点生成。
	ID string `json:"id"`
	// NodeIDs 职责：保存组内稳定节点序；使用：循环组装配；边界：不包含重复节点。
	NodeIDs []string `json:"nodeIds"`
	// Level 职责：保存上游层级；使用：布局；边界：非负。
	Level int `json:"level"`
	// Cyclic 职责：标识多节点循环或自环；使用：保留循环路由；边界：不靠 KeyItems 猜测。
	Cyclic bool `json:"cyclic"`
}

// FlowBalance 职责：提供逐物品守恒证据；使用：生成前预检和诊断；边界：所有字段均为计划流量，不添加机器取整能力。
type FlowBalance struct {
	// ItemID 职责：保存核算物品；使用：错误定位；边界：数据集有效 ID。
	ItemID int `json:"itemId"`
	// SupplyPerMinute 职责：累计外供及产线输出；使用：守恒检查；边界：非负有限。
	SupplyPerMinute float64 `json:"supplyPerMinute"`
	// DemandPerMinute 职责：累计产线输入及目标/剩余；使用：守恒检查；边界：非负有限。
	DemandPerMinute float64 `json:"demandPerMinute"`
	// DifferencePerMinute 职责：记录供给减需求的数值残差；使用：解释容差；边界：仅容许浮点误差。
	DifferencePerMinute float64 `json:"differencePerMinute"`
}
