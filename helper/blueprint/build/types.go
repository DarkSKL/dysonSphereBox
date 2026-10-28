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
	// SurfaceLayout 职责：保存行星网格布局偏好；使用：生成器选择建议纬圈与检查单区容量；边界：缺省时保持旧平面兼容，不等价于真实碰撞验收。
	SurfaceLayout *SurfaceLayout `json:"surfaceLayout,omitempty"`
}

// SurfaceLayout 描述目标行星网格与纬圈选择偏好。
// 职责：为保持旧平面排版的整块蓝图选择可容纳完整 footprint 的粘贴纬圈。
// 使用：量化蓝图工作台传入 Generate；缺省模式由旧调用保留平面行为。
// 边界：纬圈只作为粘贴锚点建议，不把建筑拆到多个 Area，也不替代游戏 Physics 碰撞检查。
type SurfaceLayout struct {
	// Mode 职责：选择 planar、autoTropic 或 explicitTropic；使用：生成流程决定是否运行纬圈网格预检；边界：未知值拒绝。
	Mode string `json:"mode"`
	// GridSegment 职责：保存目标行星纬向分段数；使用：复刻当前 PlanetGrid 的离散经纬网格；边界：必须位于支持范围。
	GridSegment int `json:"gridSegment"`
	// LatitudeGridIndex 职责：保存目标纬圈中心索引；使用：explicitTropic 模式定位首选圈；边界：单位为纬度细分格，不是度数。
	LatitudeGridIndex int `json:"latitudeGridIndex"`
	// ExpandAdjacent 职责：允许完整平面 footprint 在首选圈容量不足时搜索相邻纬圈；使用：单 Area 锚点选择；边界：不会把建筑分散到多个 Area。
	ExpandAdjacent bool `json:"expandAdjacent"`
}

// SurfaceBandPlan 职责：描述一个实际使用的恒定经度段区域；使用：复现多 Area 编码与展示容量；边界：纬度是绝对细格索引，不能作为预览局部坐标。
type SurfaceBandPlan struct {
	// AreaIndex 职责：标识蓝图区域；使用：关联建筑记录；边界：最多64区。
	AreaIndex int `json:"areaIndex"`
	// MinLatitudeGrid 职责：记录区域南界；使用：游戏坐标验证；边界：闭区间。
	MinLatitudeGrid int `json:"minLatitudeGrid"`
	// MaxLatitudeGrid 职责：记录区域北界；使用：游戏坐标验证；边界：闭区间。
	MaxLatitudeGrid int `json:"maxLatitudeGrid"`
	// LongitudeSegments 职责：记录区域的经度段数；使用：Area比例验证；边界：区域内保持不变。
	LongitudeSegments int `json:"longitudeSegments"`
	// BuildingCount 职责：记录实际分配建筑数；使用：分项核对；边界：含块内全部辅助设施。
	BuildingCount int `json:"buildingCount"`
}

// SurfacePlan 汇报按游戏纬圈分段表选出的建议落点网格。
// 职责：向前端解释目标纬圈容量和相对用户偏好调整量。
// 使用：生成成功后展示建议放置圈；边界：不声称已通过建筑碰撞 Physics 检查。
type SurfacePlan struct {
	// AnchorLatitudeGrid 职责：记录根区域原点的建议纬度；使用：按当前游戏光标锚点验证；边界：不是绝对位置锁定。
	AnchorLatitudeGrid int `json:"anchorLatitudeGrid"`
	// PrimaryAreaIndex 职责：记录承载整块平面蓝图的主区域；使用：粘贴与坐标核对；边界：当前球面策略固定为 0。
	PrimaryAreaIndex int `json:"primaryAreaIndex"`
	// Bands 职责：保存单一平面 Area 对应的建议纬向范围；使用：展示 footprint 与容量；边界：不表示建筑被拆分到多个纬带。
	Bands []SurfaceBandPlan `json:"bands"`
	// Mode 职责：保存实际使用的选择模式；使用：显示自动或手动选择状态；边界：planar 时为空。
	Mode string `json:"mode"`
	// GridSegment 职责：保存行星网格分段；使用：复现纬圈容量；边界：只适用于本次配置。
	GridSegment int `json:"gridSegment"`
	// PreferredLatitudeGrid 职责：保存用户首选纬圈中心索引；使用：解释自动回退距离；边界：auto 模式为零。
	PreferredLatitudeGrid int `json:"preferredLatitudeGrid"`
	// SelectedLatitudeGrid 职责：保存最终推荐圈中心索引；使用：前端展示；边界：游戏粘贴时仍由玩家选择锚点。
	SelectedLatitudeGrid int `json:"selectedLatitudeGrid"`
	// LatitudeDegrees 职责：保存纬度索引换算的近似角度；使用：辅助显示；边界：实际游戏位置以离散索引为准。
	LatitudeDegrees float64 `json:"latitudeDegrees"`
	// LongitudeSegments 职责：保存该纬度段采用的离散经度分段数；使用：容量和 Area 元数据；边界：来自当前游戏版本段表。
	LongitudeSegments int `json:"longitudeSegments"`
	// RingGridCells 职责：保存整圈离散细格容量；使用：评估蓝图宽度；边界：不含建筑安全余量。
	RingGridCells int `json:"ringGridCells"`
	// LayoutWidth 职责：保存蓝图横向占用格数；使用：结果诊断；边界：来源为生成布局包围范围。
	LayoutWidth int `json:"layoutWidth"`
	// LayoutHeight 职责：保存蓝图纵向占用格数；使用：结果诊断；边界：当前要求完整 footprint 不跨纬圈分段。
	LayoutHeight int `json:"layoutHeight"`
	// ShiftedGridRows 职责：保存最终纬圈相对首选圈的偏移；使用：提示用户；边界：零表示未移动。
	ShiftedGridRows int `json:"shiftedGridRows"`
	// MachineFootprintsChecked 职责：表示已检查同层制造设备参考 footprint；使用：呈现有限范围校验；边界：不代表带/分拣器/塔及游戏 Physics 全部通过。
	MachineFootprintsChecked bool `json:"machineFootprintsChecked"`
	// BeltConnectionsChecked 职责：表示最终坐标通过已知的带接线距离限制；使用：诊断；边界：不覆盖路线碰撞、科技或端口修正。
	BeltConnectionsChecked bool `json:"beltConnectionsChecked"`
	// StaticCollidersChecked 职责：标记静态建筑三维建造盒已检查；使用：碰撞诊断；边界：不包含带和分拣器的动态几何。
	StaticCollidersChecked bool `json:"staticCollidersChecked"`
	// ColliderGameVersion 职责：记录碰撞目录覆盖版本；使用：来源追踪；边界：不是运行中游戏的自动检测结果。
	ColliderGameVersion string `json:"colliderGameVersion"`
	// CollisionCoverage 职责：说明本次静态碰撞检查覆盖范围；使用：前端显示未覆盖对象；边界：machineFootprintsOnly 不是游戏碰撞通过证明。
	CollisionCoverage string `json:"collisionCoverage"`
	// PlacementStatus 职责：标记本地预检范围内的状态；使用：前端解释结果；边界：partial 不能作为游戏内可放置保证。
	PlacementStatus string `json:"placementStatus"`
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
	// SurfacePlan 职责：保存游戏纬圈网格预检结果；使用：前端说明推荐放置位置；边界：不是完整游戏碰撞证明。
	SurfacePlan *SurfacePlan `json:"surfacePlan,omitempty"`
}
