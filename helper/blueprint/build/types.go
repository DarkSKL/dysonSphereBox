package build

// 文件职责：定义六种布局的边界契约；使用：生成服务；边界：不依赖 Wails、数据库或游戏目录。

import "DysonSphereBox/helper/quant"

// Setting 职责：镜像参考布局配置；使用：生成请求；边界：缺省补齐后仍做范围验证。
type Setting struct {
	// MaxMachineInALine 职责：保存单列机器总上限；使用：布局约束；边界：不修改全局设置。
	MaxMachineInALine int `json:"maxMachineInALine"`
	// BeltLv 职责：保存带等级；使用：布局约束；边界：不修改全局设置。
	BeltLv int `json:"beltLv"`
	// InserterLv 职责：保存分拣器等级；使用：布局约束；边界：不修改全局设置。
	InserterLv int `json:"inserterLv"`
	// MaxLays 职责：保存研究站层数；使用：布局约束；边界：不修改全局设置。
	MaxLays int `json:"maxLays"`
	// NeedPower 职责：保存是否创建供电；使用：布局约束；边界：不修改全局设置。
	NeedPower *bool `json:"needPower,omitempty"`
	// BeltSpeed 职责：保存参考运力上限每分钟；使用：布局约束；边界：不修改全局设置。
	BeltSpeed float64 `json:"beltSpeed"`
	// SmelterMaxCount 职责：保存熔炉单列上限；使用：布局约束；边界：不修改全局设置。
	SmelterMaxCount int `json:"smelterMaxCount"`
	// WorkbenchMaxCount 职责：保存制造台单列上限；使用：布局约束；边界：不修改全局设置。
	WorkbenchMaxCount int `json:"workbenchMaxCount"`
	// RefineryMaxCount 职责：保存精炼单列上限；使用：布局约束；边界：不修改全局设置。
	RefineryMaxCount int `json:"refineryMaxCount"`
	// ChemicalMaxCount 职责：保存化工单列上限；使用：布局约束；边界：不修改全局设置。
	ChemicalMaxCount int `json:"chemicalMaxCount"`
	// ColliderMaxCount 职责：保存对撞机单列上限；使用：布局约束；边界：不修改全局设置。
	ColliderMaxCount int `json:"colliderMaxCount"`
	// MatrixMaxCount 职责：保存研究站单列上限；使用：布局约束；边界：不修改全局设置。
	MatrixMaxCount int `json:"matrixMaxCount"`
	// MergeType 职责：保存1 纵向排列，2 限宽；使用：布局约束；边界：不修改全局设置。
	MergeType int `json:"mergeType"`
	// BlockWidthLimit 职责：保存街区宽度上限；使用：布局约束；边界：不修改全局设置。
	BlockWidthLimit float64 `json:"blockWidthLimit"`
	// LocalStationHitRadius 职责：保存塔碰撞距离；使用：布局约束；边界：不修改全局设置。
	LocalStationHitRadius float64 `json:"localStationHitRadius"`
	// UseMixed 职责：保存使用混带输入；使用：布局约束；边界：不修改全局设置。
	UseMixed bool `json:"useMixed"`
	// Mix2 职责：保存混带 2.0；使用：布局约束；边界：不修改全局设置。
	Mix2 *bool `json:"mix2,omitempty"`
	// SmelterOtherSpace 职责：保存熔炉附加间距；使用：布局约束；边界：不修改全局设置。
	SmelterOtherSpace int `json:"smelterOtherSpace"`
	// WorkbenchOtherSpace 职责：保存制造台附加间距；使用：布局约束；边界：不修改全局设置。
	WorkbenchOtherSpace int `json:"workbenchOtherSpace"`
	// RefineryOtherSpace 职责：保存精炼附加间距；使用：布局约束；边界：不修改全局设置。
	RefineryOtherSpace int `json:"refineryOtherSpace"`
	// ChemicalOtherSpace 职责：保存化工附加间距；使用：布局约束；边界：不修改全局设置。
	ChemicalOtherSpace int `json:"chemicalOtherSpace"`
	// ColliderOtherSpace 职责：保存对撞机附加间距；使用：布局约束；边界：不修改全局设置。
	ColliderOtherSpace int `json:"colliderOtherSpace"`
	// MatrixOtherSpace 职责：保存研究站附加间距；使用：布局约束；边界：不修改全局设置。
	MatrixOtherSpace int `json:"matrixOtherSpace"`
	// NeedCoaterNet 职责：保存喷涂供给网络；使用：布局约束；边界：不修改全局设置。
	NeedCoaterNet bool `json:"needCoaterNet"`
}

// Request 职责：承载生成边界数据；使用：服务与预览联动；边界：不承诺游戏实际验收。
type Request struct {
	// Plan 职责：保存求解后的产线；使用：桥接传递；边界：保持生成时序。
	Plan quant.Result `json:"plan"`
	// Mode 职责：保存策略编号 1 到 6；使用：桥接传递；边界：保持生成时序。
	Mode int `json:"mode"`
	// Setting 职责：保存布局参数；使用：桥接传递；边界：保持生成时序。
	Setting Setting `json:"setting"`
	// Name 职责：保存蓝图标题；使用：桥接传递；边界：保持生成时序。
	Name string `json:"name"`
}

// Zone 职责：承载一个目标（或共用）分区的蓝图范围；使用：预览分区标注与节点流着色；边界：坐标为最终蓝图局部坐标。
type Zone struct {
	// Key 职责：分区键，`target-<物品ID>` 或 `shared`；使用：前端匹配节点归属；边界：稳定且唯一。
	Key string `json:"key"`
	// TargetItemID 职责：目标物品；使用：标注；边界：共用分区为 0。
	TargetItemID int `json:"targetItemId"`
	// ProductionIndices 职责：分区内产线下标；使用：联动；边界：每条产线只属于一个分区。
	ProductionIndices []int `json:"productionIndices"`
	// MinX 职责：分区左下角 X；使用：标注；边界：含分区内全部建筑。
	MinX float64 `json:"minX"`
	// MinY 职责：分区左下角 Y；使用：标注；边界：含分区内全部建筑。
	MinY float64 `json:"minY"`
	// MaxX 职责：分区右上角 X；使用：标注；边界：含分区内全部建筑。
	MaxX float64 `json:"maxX"`
	// MaxY 职责：分区右上角 Y；使用：标注；边界：含分区内全部建筑。
	MaxY float64 `json:"maxY"`
}

// ProductionBinding 职责：承载生成边界数据；使用：服务与预览联动；边界：不承诺游戏实际验收。
type ProductionBinding struct {
	// MachineIndices 职责：制造机器的数组下标；使用：定位首台机器；边界：不让前端按建筑拼接顺序推断。
	MachineIndices []int `json:"machineIndices,omitempty"`
	// ProductionIndex 职责：保存量化行下标；使用：桥接传递；边界：保持生成时序。
	ProductionIndex int `json:"productionIndex"`
	// BuildingIndices 职责：保存建筑数组下标，区别记录 index；使用：桥接传递；边界：保持生成时序。
	BuildingIndices []int `json:"buildingIndices"`
}

// Result 职责：承载生成边界数据；使用：服务与预览联动；边界：不承诺游戏实际验收。
type Result struct {
	// BlueprintRef 职责：大对象临时令牌；使用：分片读取；边界：不是任意文件路径。
	BlueprintRef string `json:"blueprintRef,omitempty"`
	// BlueprintLength 职责：生成文本长度；使用：分片进度；边界：单位为字节。
	BlueprintLength int `json:"blueprintLength,omitempty"`
	// Blueprint 职责：保存已通过 Go 自检的串；使用：桥接传递；边界：保持生成时序。
	Blueprint string `json:"blueprint"`
	// BuildingCount 职责：保存建筑数量；使用：桥接传递；边界：保持生成时序。
	BuildingCount int `json:"buildingCount"`
	// Areas 职责：保存区域数量；使用：桥接传递；边界：保持生成时序。
	Areas int `json:"areas"`
	// Warnings 职责：保存运力及容量警告；使用：桥接传递；边界：保持生成时序。
	Warnings []string `json:"warnings"`
	// SourcePlanID 职责：保存来源指纹；使用：桥接传递；边界：保持生成时序。
	SourcePlanID string `json:"sourcePlanId"`
	// Bindings 职责：保存产线映射；使用：桥接传递；边界：保持生成时序。
	Bindings []ProductionBinding `json:"bindings"`
	// SharedBuildingIndices 职责：保存公共设施映射；使用：桥接传递；边界：保持生成时序。
	SharedBuildingIndices []int `json:"sharedBuildingIndices"`
	// Zones 职责：保存多目标分区；使用：预览区分不同目标的节点流；边界：单目标或总线模式只有一个分区。
	Zones []Zone `json:"zones"`
}
