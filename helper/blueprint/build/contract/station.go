package contract

// 文件职责：定义物流塔布局阶段的输入、输出与组合能力契约。
// 使用：公共准备、各模式组合和装配阶段按值交接已验证的计划数据。
// 边界：不依赖 build 私有 Builder，不持有建筑指针、不执行装配或发布事件。

import (
	"context"

	"DysonSphereBox/helper/quant"
)

// StationConfig 职责：声明一个物流塔组合的实体能力；使用：模式目录提供给布局和装配阶段；边界：不根据 Mode 隐式选择其他字段。
type StationConfig struct {
	// Mode 职责：保存兼容模式身份；使用：输入一致性检查和既有机器行调用；边界：不参与共享布局算法分支。
	Mode int
	// StationID 职责：指定物流塔原型；使用：实体创建；边界：必须与已核对原型一致。
	StationID int
	// ItemSlots 职责：声明塔的物品槽数；使用：分组及端口预算；边界：不能作为放宽十二物理端口的依据。
	ItemSlots int
	// DualRoleFields 职责：声明是否同时写本地和远程角色；使用：塔参数装配；边界：不推测运行时机队吞吐。
	DualRoleFields bool
	// LargeCapacity 职责：声明大塔专属运行参数；使用：参数装配；边界：实际物品容量仍由原型能力表决定。
	LargeCapacity bool
	// EnableLeftovers 职责：启用真实副产物回收链；使用：装配收尾；边界：回收份额必须来自规范物料图。
	EnableLeftovers bool
}

// PreparedStationProduction 职责：承载公共准备已核验的配方与容量拆列；使用：布局器只做组合特有分组；边界：不把准备函数或 Builder 回调传入模式包。
type PreparedStationProduction struct {
	// Recipe 职责：保存经过生产选项转换的有效配方；使用：物品槽位规划；边界：视为只读。
	Recipe quant.Recipe
	// Counts 职责：保存按真实单带容量拆出的机器数；使用：环绕布局重新均衡；边界：重新分组不得增加机器或超过最大行长。
	Counts []int
	// SprayItemID 职责：保存实际增产剂原型；使用：增加独立供剂入口预算；边界：零表示未启用喷涂。
	SprayItemID int
}

// StationLayoutInput 职责：限定物流塔布局可读取的数据；使用：组合布局函数；边界：不包含可变实体状态，切片和物料图由调用方保持只读。
type StationLayoutInput struct {
	// Plan 职责：保存规范量化产线与目标；使用：分区及产线身份；边界：不是最终蓝图。
	Plan quant.Result
	// Productions 职责：按原产线下标保存准备结果；使用：稳定索引交接；边界：长度必须等于 Plan.Productions。
	Productions []PreparedStationProduction
	// FlowGraph 职责：提供依赖顺序和规范剩余边；使用：排序及端口预留；边界：不能为 nil，不得被布局器改写。
	FlowGraph *quant.FlowGraph
}

// StationProductionPlan 职责：保存一个产线的布局装配计划；使用：布局输出交给装配；边界：不含建筑指针或最终记录编号。
type StationProductionPlan struct {
	// Index 职责：标识原量化产线；使用：实体归属；边界：不通过配方编号推测。
	Index int
	// Counts 职责：保存均衡后的每行机器数；使用：逐行创建机器；边界：总数不变。
	Counts []int
	// PerGroup 职责：保存每个闭合塔组可装配的最大行数；使用：分组循环；边界：同时满足物品端口和副产物出口预算。
	PerGroup int
}

// StationZonePlan 职责：保存分区及按依赖排序的产线计划；使用：确定性装配；边界：尚未确定实体坐标。
type StationZonePlan struct {
	// Zone 职责：保存分区身份；使用：结果归属；边界：没有最终包围范围。
	Zone Zone
	// Productions 职责：保存装配顺序；使用：分区内逐条产线生成；边界：不共享可变 Counts 切片。
	Productions []StationProductionPlan
}

// StationLayoutPlan 职责：汇总可独立验证的物流塔布局计划；使用：布局到装配交接；边界：没有任何实体或蓝图串。
type StationLayoutPlan struct {
	// Zones 职责：保存稳定分区顺序；使用：空间布局；边界：每条产线只装配一次。
	Zones []StationZonePlan
	// ProductionCount 职责：保存实际产线数量；使用：装配进度计数；边界：不是建筑数量或完成百分比。
	ProductionCount int
}

// StationAdapter 职责：组合单一模式与密度的物流塔阶段；使用：具体组合目录定义并交给统一入口；边界：不依赖旧 build 包，也不携带可变任务状态。
type StationAdapter struct {
	// Config 职责：提供该组合的装配能力；使用：布局和实体创建；边界：按值复制。
	Config StationConfig
	// Layout 职责：执行本组合的布局阶段；使用：统一阶段编排；边界：只消费显式输入，不回调私有 Builder。
	Layout func(context.Context, StationLayoutInput) (StationLayoutPlan, error)
}
