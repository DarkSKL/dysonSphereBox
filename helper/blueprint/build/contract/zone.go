package contract

// 文件职责：定义与建筑坐标无关的共享分区契约。
// 使用：多个模式的布局器与结果转换层共同引用；边界：不包含模式能力或最终空间包围范围。

// Zone 职责：描述没有建筑坐标的目标分区；使用：布局阶段安排产线归属；边界：最终范围由实体装配和编码阶段计算。
type Zone struct {
	// Key 职责：保存稳定分区键；使用：阶段间关联；边界：不是模式或密度键。
	Key string
	// TargetItemID 职责：标识分区服务的目标；使用：目标归属；边界：共享分区为零。
	TargetItemID int
	// ProductionIndices 职责：保存原量化产线下标；使用：依赖排序与装配；边界：布局器不得修改请求原数组。
	ProductionIndices []int
}
