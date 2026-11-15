package statistics

// 文件职责：定义量化和实体蓝图共用的分析数据；使用：算法返回值及后续分析页；边界：静态预测不冒充游戏实测，未知值使用nil及覆盖计数。

// Rate 职责：统一物料速率单位；使用：产线输入输出；边界：每分钟件数，不是库存。
type Rate struct {
	// ItemID/Name 职责：关联原型；使用：分类和展示；边界：名称不作为主键。
	ItemID int    `json:"itemId"`
	Name   string `json:"name"`
	// RatePerMinute 职责：记录已计增产的速率；使用：流量图；边界：口径由所在列表决定。
	RatePerMinute float64 `json:"ratePerMinute"`
}

// Power 职责：区分基准、满载和预测功耗；使用：单台/产线/分类/全厂汇总；边界：UnknownCount表示未估算的运行负载，MissingCount表示原型也未知。
type Power struct {
	// IdleMW 职责：已知设施全待机功率；使用：静态比较；边界：不表示运行时最低值。
	IdleMW float64 `json:"idleMw"`
	// BaseWorkMW 职责：未喷涂额定功率；使用：增产对照；边界：不是无喷涂方案重新求解的全厂功耗。
	BaseWorkMW float64 `json:"baseWorkMw"`
	// ConfiguredWorkMW 职责：配置额定工作功率；使用：供电规划；边界：含制造增产倍率，物流充电及科技可能改变运行值。
	ConfiguredWorkMW float64 `json:"configuredWorkMw"`
	// KnownExpectedMW 职责：已可估算设备的工作/待机加权值；使用：部分覆盖时仍能分析；边界：不可当全厂总量。
	KnownExpectedMW float64 `json:"knownExpectedMw"`
	// ExpectedMW 职责：全部设备均可估算时提供总预测；使用：图表；边界：任何未知负载则null。
	ExpectedMW *float64 `json:"expectedMw"`
	// UnknownCount/MissingCount 职责：明确缺失覆盖；使用：分析可信度；边界：不得把缺失解释成零。
	UnknownCount int `json:"unknownCount"`
	MissingCount int `json:"missingCount"`
}

// Thermal 职责：记录燃料化学能与发电原型；使用：热值/能量分析；边界：物料热值通量不是烧掉的燃料或建筑散热。
type Thermal struct {
	// InputPotentialMW/OutputPotentialMW 职责：每分钟毛物料热值换算MW；使用：产线能量载体分析；边界：不将中间流累加为外部发电。
	InputPotentialMW  float64 `json:"inputPotentialMw"`
	OutputPotentialMW float64 `json:"outputPotentialMw"`
	// RatedGenerationMW/RatedFuelUseMW 职责：保存原型额定发电及燃料热消耗；使用：设施分类；边界：不含天气、燃料喷涂、供给和运行负荷。
	RatedGenerationMW float64 `json:"ratedGenerationMw"`
	RatedFuelUseMW    float64 `json:"ratedFuelUseMw"`
	// DissipationMW 职责：为散热分析保留明确未知；使用：区别电耗与热量；边界：当前游戏规则没有建筑散热模型，始终null。
	DissipationMW *float64 `json:"dissipationMw"`
}

// Line 职责：保存可关联量化产线的能力与负载；使用：产能/热力图；边界：满负荷能力逐线独立，不代表全厂可同时满产。
type Line struct {
	// ProductionIndex/RecipeID/MachineID 职责：关联原方案；使用：节点联动；边界：下标只对同一PlanID有效。
	ProductionIndex int `json:"productionIndex"`
	RecipeID        int `json:"recipeId"`
	MachineID       int `json:"machineId"`
	// MachineName/Count/ExactCount 职责：记录实际取整配置与需求；使用：利用率；边界：量化阶段只有制造设备。
	MachineName string  `json:"machineName"`
	Count       int     `json:"count"`
	ExactCount  float64 `json:"exactCount"`
	// Utilization 职责：期望产量/配置产能；使用：统一平均负载热力图；边界：不声称单台真实开停状态。
	Utilization float64 `json:"utilization"`
	// ProliferatorMode/Points/PowerMultiplier 职责：记录有效喷涂配置；使用：能耗解释；边界：毛流和功率已应用效果，不能再次乘倍率。
	ProliferatorMode int     `json:"proliferatorMode"`
	Points           int     `json:"points"`
	PowerMultiplier  float64 `json:"powerMultiplier"`
	// Inputs/Outputs 职责：期望毛流，包含本地循环和供剂；使用：实际机器输送；边界：不得当作蓝图边界供需。
	Inputs  []Rate `json:"inputs"`
	Outputs []Rate `json:"outputs"`
	// NetInputs/NetOutputs 职责：同线相同物料抵消后的期望流量；使用：跨线收支；边界：不替代毛流。
	NetInputs  []Rate `json:"netInputs"`
	NetOutputs []Rate `json:"netOutputs"`
	// ConfiguredInputs/ConfiguredOutputs 职责：全线整数设备满载毛流；使用：容量对照；边界：需供料、电力和出货畅通。
	ConfiguredInputs  []Rate `json:"configuredInputs"`
	ConfiguredOutputs []Rate `json:"configuredOutputs"`
	// JointProducts 职责：标识配方联产物品；使用：副产品分析；边界：不等于持续剩余。
	JointProducts []int `json:"jointProducts"`
	// Power/Thermal 职责：产线级能量；使用：按建筑和配方分组；边界：热流不包括凭空设定的散热。
	Power   Power   `json:"power"`
	Thermal Thermal `json:"thermal"`
}

// Material 职责：按物品汇总净边界与毛流；使用：输入输出、副产品及能量分析；边界：余量取自守恒方案，不能用缓存抵扣。
type Material struct {
	// ItemID/Name 职责：物品标识；使用：分类；边界：同ID仅一行。
	ItemID int    `json:"itemId"`
	Name   string `json:"name"`
	// Imported/Produced/Consumed/Target/Surplus 职责：每分钟净收支；使用：守恒及外运；边界：Produced/Consumed为跨线净流。
	Imported float64 `json:"importedPerMinute"`
	Produced float64 `json:"producedPerMinute"`
	Consumed float64 `json:"consumedPerMinute"`
	Target   float64 `json:"targetPerMinute"`
	Surplus  float64 `json:"surplusPerMinute"`
	// GrossInput/GrossOutput/LocalRecycle 职责：机器毛流和同线循环量；使用：输送成本；边界：不把跨线转运当成本地回流。
	GrossInput   float64 `json:"grossInputPerMinute"`
	GrossOutput  float64 `json:"grossOutputPerMinute"`
	LocalRecycle float64 `json:"localRecyclePerMinute"`
	// BalanceResidual 职责：进口+生产-消耗-交付-余量；使用：独立核对；边界：浮点误差允许，不应有实质差额。
	BalanceResidual float64 `json:"balanceResidualPerMinute"`
	// JointProduct/HasSurplus 职责：区分联产物与持续过剩；使用：阻塞风险；边界：取整余量不自动当作期望剩余。
	JointProduct bool `json:"jointProduct"`
	HasSurplus   bool `json:"hasSurplus"`
	// HeatValueMJ 职责：物品未喷涂原型热值；使用：能量载体分析；边界：缺少原型则null。
	HeatValueMJ *float64 `json:"heatValueMj"`
	// ImportPotentialMW/ExportPotentialMW 职责：外部净输入和目标加剩余的热值通量；使用：全厂能量边界；边界：不意味着已经燃烧发电。
	ImportPotentialMW float64 `json:"importPotentialMw"`
	ExportPotentialMW float64 `json:"exportPotentialMw"`
}

// BuildingGroup 职责：按实际建筑原型聚合数量与能量；使用：分类图表；边界：制造物品的目标数不算已建建筑。
type BuildingGroup struct {
	// ItemID/Name/Category/Count 职责：识别设施分类；使用：清单；边界：传送带按实体节数计。
	ItemID   int    `json:"itemId"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
	// Inputs/Outputs/ConfiguredInputs/ConfiguredOutputs 职责：本类制造设施的期望及满载毛流汇总；使用：按建筑分析产能；边界：物流设施没有配方产能。
	Inputs            []Rate `json:"inputs"`
	Outputs           []Rate `json:"outputs"`
	ConfiguredInputs  []Rate `json:"configuredInputs"`
	ConfiguredOutputs []Rate `json:"configuredOutputs"`
	// Power/Thermal 职责：本类设施汇总；使用：能耗及能量结构；边界：工作功率是原型额定值。
	Power   Power   `json:"power"`
	Thermal Thermal `json:"thermal"`
	// Utilization 职责：按数量加权的制造平均负载；使用：热力图图例；边界：含运行负载未知设备则null。
	Utilization *float64 `json:"utilization"`
}

// HeatmapEntry 职责：为带负载意义的实体提供定位和指标；使用：后续热力图；边界：省略无耗电的纯运输带，未建布局时列表为空。
type HeatmapEntry struct {
	// BuildingIndex/ItemID/ProductionIndex 职责：关联最终蓝图及产线；使用：点击定位；边界：非生产设施ProductionIndex为-1。
	BuildingIndex   int `json:"buildingIndex"`
	ItemID          int `json:"itemId"`
	ProductionIndex int `json:"productionIndex"`
	// AreaIndex/X/Y/Z 职责：保存蓝图局部坐标；使用：交给预览器变换；边界：不同Area坐标不能直接相减。
	AreaIndex int     `json:"areaIndex"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	// Utilization/ExpectedMW/ConfiguredWorkMW 职责：单台预测指标；使用：颜色及提示；边界：null为运行状态未知，不是零负荷。
	Utilization      *float64 `json:"utilization"`
	ExpectedMW       *float64 `json:"expectedMw"`
	ConfiguredWorkMW float64  `json:"configuredWorkMw"`
	// Thermal 职责：按设备均摊的物料能量和原型能力；使用：热值视图；边界：不是温度。
	Thermal Thermal `json:"thermal"`
}

// Space 职责：保存最终布局而非制造成本权重；使用：占地和带长；边界：量化阶段返回nil，格网面积不冒充平方米。
type Space struct {
	// BuildingFootprintAreaGrid2 职责：固定设施投影格网并集；使用：建筑总占地；边界：排除传送带和分拣器，叠层不重复计，保守近似而非真实地表面积。
	BuildingFootprintAreaGrid2 float64 `json:"buildingFootprintAreaGrid2"`
	// EnvelopeAreaGrid2/OccupiedAreaGrid2/Utilization 职责：所有建筑与路线的包络及占地并集近似；使用：紧凑度比较；边界：包含架空路线投影。
	EnvelopeAreaGrid2 float64 `json:"envelopeAreaGrid2"`
	OccupiedAreaGrid2 float64 `json:"occupiedAreaGrid2"`
	Utilization       float64 `json:"utilization"`
	// BeltCount/TotalBeltLengthMeters 职责：最终实体带和唯一带间连接弦长；使用：建造成本；边界：不按物料路线重复累计。
	BeltCount             int     `json:"beltCount"`
	TotalBeltLengthMeters float64 `json:"totalBeltLengthMeters"`
	// AreaCount 职责：最终蓝图区域数量；使用：解释坐标；边界：不代表工厂数量。
	AreaCount int `json:"areaCount"`
}

// Report 职责：统一分析数据接口；使用：Solve及Generate返回；边界：SchemaVersion变化需调用方迁移，未算字段不伪装成零。
type Report struct {
	// SchemaVersion/Scope 职责：标识版本及量化/实体范围；使用：接口兼容；边界：planned-manufacturing不含运输设施。
	SchemaVersion string `json:"schemaVersion"`
	Scope         string `json:"scope"`
	// PlanID/DataVersion/GameVersion/FactsVersion 职责：追溯数据来源；使用：缓存与对比；边界：不代表正在运行游戏版本。
	PlanID       string `json:"planId"`
	DataVersion  string `json:"dataVersion"`
	GameVersion  string `json:"gameVersion"`
	FactsVersion string `json:"factsVersion"`
	// BuildingCount/ProductionMachineCount/ProductionLineCount 职责：区分总实体和制造设备；使用：总览；边界：统计范围由Scope决定。
	BuildingCount          int `json:"buildingCount"`
	ProductionMachineCount int `json:"productionMachineCount"`
	ProductionLineCount    int `json:"productionLineCount"`
	// Buildings/Lines/Materials 职责：三类稳定明细；使用：分析页分组；边界：物品与建筑按ID排序，产线保留原顺序。
	Buildings []BuildingGroup `json:"buildings"`
	Lines     []Line          `json:"lines"`
	Materials []Material      `json:"materials"`
	// Power/Thermal 职责：总电力与外部物料能量边界；使用：总览；边界：不加总内部中间物料热流。
	Power   Power   `json:"power"`
	Thermal Thermal `json:"thermal"`
	// HasSurplus/SurplusItemCount 职责：明确持续外运需要；使用：运行分析；边界：缓存不能消除过剩。
	HasSurplus       bool `json:"hasSurplus"`
	SurplusItemCount int  `json:"surplusItemCount"`
	// Space/Heatmap 职责：最终实体布局统计；使用：生成后分析；边界：量化阶段没有假想坐标。
	Space   *Space         `json:"space"`
	Heatmap []HeatmapEntry `json:"heatmap"`
	// Notes 职责：明示估算方法和缺口；使用：解释静态指标；边界：不能把注释当成实测。
	Notes []string `json:"notes"`
}
