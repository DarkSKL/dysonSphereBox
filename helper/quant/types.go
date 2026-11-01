package quant

// 文件职责：定义量化请求、参考设置与结果；使用：纯 Go 引擎及 Wails 服务；边界：内部物品保持名称键，不承载持久化。

// Item 职责：承载原型物品；使用：量化边界；边界：不访问系统资源。
type Item struct {
	// ID 职责：保存物品编号；使用：引擎传递；边界：不隐式改变单位。
	ID int `json:"id"`
	// Name 职责：保存名称键；使用：引擎传递；边界：不隐式改变单位。
	Name string `json:"name"`
	// Type 职责：保存原型类别；使用：引擎传递；边界：不隐式改变单位。
	Type int `json:"type"`
	// GridIndex 职责：保存展示位置；使用：引擎传递；边界：不隐式改变单位。
	GridIndex int `json:"gridIndex"`
	// Speed 职责：保存工作倍率；使用：引擎传递；边界：不隐式改变单位。
	Speed float64 `json:"speed"`
	// Space 职责：保存占地；使用：引擎传递；边界：不隐式改变单位。
	Space float64 `json:"space"`
	// WorkEnergyPerTick 职责：保存每 tick 工作能耗；使用：引擎传递；边界：不隐式改变单位。
	WorkEnergyPerTick float64 `json:"workEnergyPerTick"`
	// IdleEnergyPerTick 职责：保存每 tick 待机能耗；使用：引擎传递；边界：不隐式改变单位。
	IdleEnergyPerTick float64 `json:"idleEnergyPerTick"`
	// ModelIndex 职责：保存模型编号；使用：引擎传递；边界：不隐式改变单位。
	ModelIndex int `json:"modelIndex"`
}

// Recipe 职责：承载原型配方；使用：量化边界；边界：不访问系统资源。
type Recipe struct {
	// ID 职责：保存配方编号；使用：引擎传递；边界：不隐式改变单位。
	ID int `json:"id"`
	// Name 职责：保存配方名；使用：引擎传递；边界：不隐式改变单位。
	Name string `json:"name"`
	// Type 职责：保存机器类型；使用：引擎传递；边界：不隐式改变单位。
	Type int `json:"type"`
	// Factories 职责：保存设施原型序列；使用：引擎传递；边界：不隐式改变单位。
	Factories []int `json:"factories"`
	// Items 职责：保存原料原型序列；使用：引擎传递；边界：不隐式改变单位。
	Items []int `json:"items"`
	// ItemCounts 职责：保存原料数量；使用：引擎传递；边界：不隐式改变单位。
	ItemCounts []float64 `json:"itemCounts"`
	// Results 职责：保存产物序列；使用：引擎传递；边界：不隐式改变单位。
	Results []int `json:"results"`
	// ResultCounts 职责：保存产物数量；使用：引擎传递；边界：不隐式改变单位。
	ResultCounts []float64 `json:"resultCounts"`
	// TimeSpend 职责：保存耗时 tick；使用：引擎传递；边界：不隐式改变单位。
	TimeSpend float64 `json:"timeSpend"`
	// Proliferator 职责：保存可选喷涂模式位域；使用：引擎传递；边界：不隐式改变单位。
	Proliferator int `json:"proliferator"`
}

// DatasetInfo 职责：承载内置数据摘要；使用：量化边界；边界：不访问系统资源。
type DatasetInfo struct {
	// Key 职责：保存数据集键；使用：引擎传递；边界：不隐式改变单位。
	Key string `json:"key"`
	// Name 职责：保存显示名称；使用：引擎传递；边界：不隐式改变单位。
	Name string `json:"name"`
	// GameVersion 职责：保存参考游戏版本；使用：引擎传递；边界：不隐式改变单位。
	GameVersion string `json:"gameVersion"`
	// DataVersion 职责：保存数据摘要；使用：引擎传递；边界：不隐式改变单位。
	DataVersion string `json:"dataVersion"`
	// ItemCount 职责：保存物品数量；使用：引擎传递；边界：不隐式改变单位。
	ItemCount int `json:"itemCount"`
	// RecipeCount 职责：保存配方数量；使用：引擎传递；边界：不隐式改变单位。
	RecipeCount int `json:"recipeCount"`
}

// RecipeScheme 职责：承载每配方人工选择；使用：量化边界；边界：不访问系统资源。
type RecipeScheme struct {
	// Building 职责：保存建筑序号，从零开始；使用：引擎传递；边界：不隐式改变单位。
	Building int `json:"building"`
	// IncreasePoints 职责：保存增产点数 0 到 4；使用：引擎传递；边界：只支持原版的 1/2/4 点档位。
	IncreasePoints int `json:"increasePoints"`
	// ProductionMode 职责：保存参考模式 0 到 4；使用：引擎传递；边界：不隐式改变单位。
	ProductionMode int `json:"productionMode"`
}

// ItemCost 职责：承载额外成本；使用：量化边界；边界：不访问系统资源。
type ItemCost struct {
	// Cost 职责：保存单位成本；使用：引擎传递；边界：不隐式改变单位。
	Cost float64 `json:"cost"`
	// Enabled 职责：保存是否启用；使用：引擎传递；边界：不隐式改变单位。
	Enabled bool `json:"enabled"`
	// Cumulative 职责：保存是否累计其它成本；使用：引擎传递；边界：不隐式改变单位。
	Cumulative bool `json:"cumulative"`
	// OverflowCost 职责：保存副产物处理成本；使用：引擎传递；边界：不隐式改变单位。
	OverflowCost float64 `json:"overflowCost"`
}

// CostWeight 职责：承载参考成本权重；使用：量化边界；边界：不访问系统资源。
type CostWeight struct {
	// Space 职责：保存占地权重，缺省为 1；使用：引擎传递；边界：不隐式改变单位。
	Space *float64 `json:"space,omitempty"`
	// Power 职责：保存电力权重；使用：引擎传递；边界：不隐式改变单位。
	Power float64 `json:"power,omitempty"`
	// BuildingCost 职责：保存按建筑名称的成本；使用：引擎传递；边界：不隐式改变单位。
	BuildingCost map[string]float64 `json:"buildingCost,omitempty"`
	// ItemExtraCost 职责：保存按物品名称的额外成本；使用：引擎传递；边界：不隐式改变单位。
	ItemExtraCost map[string]ItemCost `json:"itemExtraCost,omitempty"`
}

// Scheme 职责：承载参考方案；使用：量化边界；边界：不访问系统资源。
type Scheme struct {
	// ItemRecipeChoices 职责：保存物品到可选配方序号，从 1 开始；使用：引擎传递；边界：不隐式改变单位。
	ItemRecipeChoices map[string]int `json:"itemRecipeChoices,omitempty"`
	// SchemeForRecipe 职责：保存按数据集配方序号的配置；使用：引擎传递；边界：不隐式改变单位。
	SchemeForRecipe []RecipeScheme `json:"schemeForRecipe,omitempty"`
	// CostWeight 职责：保存成本权重；使用：引擎传递；边界：不隐式改变单位。
	CostWeight CostWeight `json:"costWeight"`
}

// Target 职责：承载目标产量；使用：量化边界；边界：不访问系统资源。
type Target struct {
	// ItemID 职责：保存目标物品；使用：引擎传递；边界：不隐式改变单位。
	ItemID int `json:"itemId"`
	// RatePerMinute 职责：保存每分钟目标；使用：引擎传递；边界：不隐式改变单位。
	RatePerMinute float64 `json:"ratePerMinute"`
}

// NaturalProduction 职责：承载现有固定产线；使用：量化边界；边界：不访问系统资源。
type NaturalProduction struct {
	// TargetItem 职责：保存目标名称；使用：引擎传递；边界：不隐式改变单位。
	TargetItem string `json:"targetItem"`
	// RecipeChoice 职责：保存配方选项，从 1 开始；使用：引擎传递；边界：不隐式改变单位。
	RecipeChoice int `json:"recipeChoice"`
	// Building 职责：保存设施序号；使用：引擎传递；边界：不隐式改变单位。
	Building int `json:"building"`
	// MachineCount 职责：保存已有机器数；使用：引擎传递；边界：不隐式改变单位。
	MachineCount float64 `json:"machineCount"`
	// ProductionMode 职责：保存喷涂模式；使用：引擎传递；边界：不隐式改变单位。
	ProductionMode int `json:"productionMode"`
	// IncreasePoints 职责：保存增产点数；使用：引擎传递；边界：不隐式改变单位。
	IncreasePoints int `json:"increasePoints"`
}

// Settings 职责：承载量化参考设置；使用：量化边界；边界：不访问系统资源。
type Settings struct {
	// MiningSpeedOil 职责：保存油井面板速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedOil float64 `json:"miningSpeedOil,omitempty"`
	// MiningSpeedHydrogen 职责：保存巨星氢速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedHydrogen float64 `json:"miningSpeedHydrogen,omitempty"`
	// MiningSpeedDeuterium 职责：保存重氢速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedDeuterium float64 `json:"miningSpeedDeuterium,omitempty"`
	// MiningSpeedGasHydrate 职责：保存可燃冰速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedGasHydrate float64 `json:"miningSpeedGasHydrate,omitempty"`
	// MiningSpeedHelium 职责：保存氦速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedHelium float64 `json:"miningSpeedHelium,omitempty"`
	// MiningSpeedAmmonia 职责：保存氨速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedAmmonia float64 `json:"miningSpeedAmmonia,omitempty"`
	// MiningSpeedNitrogen 职责：保存氮速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedNitrogen float64 `json:"miningSpeedNitrogen,omitempty"`
	// MiningSpeedOxygen 职责：保存氧速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedOxygen float64 `json:"miningSpeedOxygen,omitempty"`
	// MiningSpeedCarbonDioxide 职责：保存二氧化碳速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedCarbonDioxide float64 `json:"miningSpeedCarbonDioxide,omitempty"`
	// MiningSpeedSulfurDioxide 职责：保存二氧化硫速率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedSulfurDioxide float64 `json:"miningSpeedSulfurDioxide,omitempty"`
	// CoveredVeinsSmall 职责：保存小矿机覆盖；使用：引擎传递；边界：不隐式改变单位。
	CoveredVeinsSmall float64 `json:"coveredVeinsSmall,omitempty"`
	// CoveredVeinsLarge 职责：保存大矿机覆盖；使用：引擎传递；边界：不隐式改变单位。
	CoveredVeinsLarge float64 `json:"coveredVeinsLarge,omitempty"`
	// MiningEfficiencyLarge 职责：保存大矿机速度；使用：引擎传递；边界：不隐式改变单位。
	MiningEfficiencyLarge float64 `json:"miningEfficiencyLarge,omitempty"`
	// MiningSpeedMultiple 职责：保存采矿倍率；使用：引擎传递；边界：不隐式改变单位。
	MiningSpeedMultiple float64 `json:"miningSpeedMultiple,omitempty"`
	// EnemyDropMultiple 职责：保存掉落倍率；使用：引擎传递；边界：不隐式改变单位。
	EnemyDropMultiple float64 `json:"enemyDropMultiple,omitempty"`
	// IcarusManufacturingSpeed 职责：保存机甲制造倍率；使用：引擎传递；边界：不隐式改变单位。
	IcarusManufacturingSpeed float64 `json:"icarusManufacturingSpeed,omitempty"`
	// FractionatingSpeed 职责：保存分馏每秒过带速率；使用：引擎传递；边界：不隐式改变单位。
	FractionatingSpeed float64 `json:"fractionatingSpeed,omitempty"`
	// FixedNum 职责：保存显示小数位，缺省 2；使用：引擎传递；边界：不隐式改变单位。
	FixedNum *int `json:"fixedNum,omitempty"`
	// StackResearchLab 职责：保存研究站堆叠；使用：引擎传递；边界：不隐式改变单位。
	StackResearchLab float64 `json:"stackResearchLab,omitempty"`
	// ProliferateItself 职责：保存自喷涂，缺省 true；使用：引擎传递；边界：不隐式改变单位。
	ProliferateItself *bool `json:"proliferateItself,omitempty"`
	// AccRate 职责：保存加速修正；使用：引擎传递；边界：不隐式改变单位。
	AccRate float64 `json:"accRate,omitempty"`
	// IncRate 职责：保存增产修正；使用：引擎传递；边界：不隐式改变单位。
	IncRate float64 `json:"incRate,omitempty"`
	// MineralizeList 职责：保存原矿化名称集合；使用：引擎传递；边界：不隐式改变单位。
	MineralizeList map[string]bool `json:"mineralizeList,omitempty"`
	// NaturalProductionLine 职责：保存已有产线；使用：引擎传递；边界：不隐式改变单位。
	NaturalProductionLine []NaturalProduction `json:"naturalProductionLine,omitempty"`
}

// Request 职责：承载量化请求；使用：量化边界；边界：不访问系统资源。
type Request struct {
	// Targets 职责：保存目标集合；使用：引擎传递；边界：不隐式改变单位。
	Targets []Target `json:"targets"`
	// Dataset 职责：保存数据集键；使用：引擎传递；边界：不隐式改变单位。
	Dataset string `json:"dataset"`
	// Scheme 职责：保存人工选择方案；使用：引擎传递；边界：不隐式改变单位。
	Scheme Scheme `json:"scheme"`
	// Settings 职责：保存设置；使用：引擎传递；边界：不隐式改变单位。
	Settings Settings `json:"settings"`
}

// Production 职责：承载量化到生成的产线桥梁；使用：量化边界；边界：不访问系统资源。
type Production struct {
	// ProliferatorMode 职责：保存量化喷涂模式 0..4；使用：显示原方案；边界：区别蓝图 acceleratorMode。
	ProliferatorMode int `json:"proliferatorMode"`
	// RecipeID 职责：保存原型配方；使用：引擎传递；边界：不隐式改变单位。
	RecipeID int `json:"recipeId"`
	// MachineCount 职责：保存取整机器数；使用：引擎传递；边界：不隐式改变单位。
	MachineCount int `json:"machineCount"`
	// ExactMachineCount 职责：保存精确机器数；使用：引擎传递；边界：不隐式改变单位。
	ExactMachineCount float64 `json:"exactMachineCount"`
	// RoundingMargin 职责：保存取整余量；使用：引擎传递；边界：不隐式改变单位。
	RoundingMargin float64 `json:"roundingMargin"`
	// MachineType 职责：保存参考机器枚举；使用：引擎传递；边界：不隐式改变单位。
	MachineType int `json:"machineType"`
	// MachineLv 职责：保存机器等级；使用：引擎传递；边界：不隐式改变单位。
	MachineLv int `json:"machineLv"`
	// BeltLv 职责：保存传送带等级；使用：引擎传递；边界：不隐式改变单位。
	BeltLv int `json:"beltLv"`
	// MachineID 职责：保存设施原型；使用：引擎传递；边界：不隐式改变单位。
	MachineID int `json:"machineId"`
	// OutputCount 职责：保存期望的主产物每分钟净产出；使用：需求守恒及节点计划；边界：不随机器取整扩大，不等于满负荷能力。
	OutputCount float64 `json:"outputCount"`
	// MainItemID 职责：保存主产物原型；使用：引擎传递；边界：不隐式改变单位。
	MainItemID int `json:"mainItemId"`
	// ProductionMode 职责：保存参考喷涂模式；使用：引擎传递；边界：不隐式改变单位。
	ProductionMode int `json:"productionMode"`
	// IncreaseLv 职责：保存喷涂等级；使用：引擎传递；边界：不隐式改变单位。
	IncreaseLv int `json:"increaseLv"`
	// IncreasePoints 职责：保存实际点数；使用：引擎传递；边界：不隐式改变单位。
	IncreasePoints int `json:"increasePoints"`
	// Inputs 职责：保存本产线每分钟净消耗（含增产剂）；使用：节点图入边；边界：同一物品出入相抵后只出现在一侧。
	Inputs []ItemRate `json:"inputs"`
	// Outputs 职责：保存本产线每分钟净产出（主产物与副产物）；使用：节点图出边；边界：主产物等于 OutputCount。
	Outputs []ItemRate `json:"outputs"`
	// PowerMW 职责：保存本产线制造工作功耗 MW；使用：节点功耗显示；边界：按精确台数计算，合计等于 Result.PowerMW。
	PowerMW float64 `json:"powerMw"`
	// Capacity 职责：保存配置下整数机器的满负荷能力；使用：区分期望净流与额定能力；边界：旧快照可缺省，不代表游戏实测或整线供料保证。
	Capacity *ProductionCapacity `json:"capacity,omitempty"`
	// TargetItemIDs 职责：保存本产线服务的目标物品（按目标顺序）；使用：蓝图按目标分区与节点流着色；边界：多于一个即为共用产线，空表示只供副产物剩余。
	TargetItemIDs []int `json:"targetItemIds"`
}

// ProductionCapacity 职责：承载已选设备、配方和喷涂的整数机器满负荷能力；使用：能力展示及物理端口规划；边界：要求原料、增产剂、电力充足且出货畅通，不代表游戏实测速率。
type ProductionCapacity struct {
	// Basis 职责：标识能力来自配置计算；使用：前端区分测产与额定值；边界：configured-full-load 不证明蓝图运输或外部供给足额。
	Basis string `json:"basis"`
	// OutputPerMinute 职责：保存全线主产物满负荷净产出；使用：与 OutputCount 的期望值并列；边界：同物品循环原料已抵消。
	OutputPerMinute float64 `json:"outputPerMinute"`
	// OutputPerMachinePerMinute 职责：保存单机主产物满负荷净产出；使用：整数设备能力解释；边界：已计设备速度、每次件数和喷涂效果。
	OutputPerMachinePerMinute float64 `json:"outputPerMachinePerMinute"`
	// CyclesPerMinute 职责：保存单机每分钟等效配方执行次数；使用：解释周期与每次产出；边界：按所选量化模式和倍率推导，不乘每次产物件数，不保证与游戏面板数值相同。
	CyclesPerMinute float64 `json:"cyclesPerMinute"`
	// Inputs 职责：保存全线满负荷净消耗；使用：条件供料能力展示；边界：不能替换期望流图的 Inputs。
	Inputs []ItemRate `json:"inputs"`
	// Outputs 职责：保存全线满负荷净产出；使用：主副产物能力展示；边界：不能替换期望流图的 Outputs。
	Outputs []ItemRate `json:"outputs"`
	// GrossInputs 职责：保存全线满负荷毛消耗；使用：包含循环物料及增产剂的设备端口规划；边界：与 GrossOutputs 同物品时必须保留两侧。
	GrossInputs []ItemRate `json:"grossInputs"`
	// GrossOutputs 职责：保存全线满负荷毛产出；使用：包含循环及副产物的设备端口规划；边界：毛产出不全部等于可对外净供给。
	GrossOutputs []ItemRate `json:"grossOutputs"`
	// Utilization 职责：保存期望负荷占整数机器能力的比例；使用：解释取整余量；边界：0 到 1，不是游戏实时利用率。
	Utilization float64 `json:"utilization"`
	// PowerMW 职责：保存全线整数机器满负荷工作功耗 MW；使用：与期望功耗对照；边界：不含待机、电网、运输设施和外部供料设施。
	PowerMW float64 `json:"powerMw"`
}

// ItemRate 职责：承载每分钟物品量；使用：量化边界；边界：不访问系统资源。
type ItemRate struct {
	// ItemID 职责：保存物品原型；使用：引擎传递；边界：不隐式改变单位。
	ItemID int `json:"itemId"`
	// Name 职责：保存物品名；使用：引擎传递；边界：不隐式改变单位。
	Name string `json:"name"`
	// RatePerMinute 职责：保存每分钟速率；使用：引擎传递；边界：不隐式改变单位。
	RatePerMinute float64 `json:"ratePerMinute"`
}

// Result 职责：承载已求解方案；使用：量化边界；边界：不访问系统资源。
type Result struct {
	// Targets 职责：保存原目标；使用：总线区分净导出与内部流量；边界：不从产线顺序推测。
	Targets []Target `json:"targets"`
	// Dataset 职责：保存数据集键；使用：引擎传递；边界：不隐式改变单位。
	Dataset string `json:"dataset"`
	// DataVersion 职责：保存数据指纹；使用：引擎传递；边界：不隐式改变单位。
	DataVersion string `json:"dataVersion"`
	// GameVersion 职责：保存游戏版本；使用：引擎传递；边界：不隐式改变单位。
	GameVersion string `json:"gameVersion"`
	// PlanID 职责：保存请求指纹；使用：引擎传递；边界：不隐式改变单位。
	PlanID string `json:"planId"`
	// Productions 职责：保存制造产线；使用：引擎传递；边界：不隐式改变单位。
	Productions []Production `json:"productions"`
	// RawInputs 职责：保存原矿或外部输入；使用：引擎传递；边界：不隐式改变单位。
	RawInputs []ItemRate `json:"rawInputs"`
	// ExternalInputItemIDs 职责：标识显式外供而非原矿的 RawInputs；使用：流图与边界诊断；边界：只记录来源语义，不重复累计流量，旧快照缺省按原矿处理。
	ExternalInputItemIDs []int `json:"externalInputItemIds"`
	// FlowGraph 职责：保存后端生成的规范供需边和 SCC 顺序；使用：画板展示与蓝图规划共用；边界：生成时必须由可信物料字段重建，不信任请求中任意修改的边。
	FlowGraph *FlowGraph `json:"flowGraph,omitempty"`
	// Leftovers 职责：保存净溢出副产物；使用：引擎传递；边界：不隐式改变单位。
	Leftovers []ItemRate `json:"leftovers"`
	// PowerMW 职责：保存制造工作能耗 MW；使用：引擎传递；边界：不隐式改变单位。
	PowerMW float64 `json:"powerMw"`
	// MachineCount 职责：保存制造建筑总数；使用：引擎传递；边界：不隐式改变单位。
	MachineCount int `json:"machineCount"`
	// LPCost 职责：保存LP 诊断目标；使用：引擎传递；边界：不隐式改变单位。
	LPCost float64 `json:"lpCost"`
	// KeyItems 职责：保存循环关键物品；使用：引擎传递；边界：不隐式改变单位。
	KeyItems []int `json:"keyItems"`
	// Warnings 职责：保存明确警告；使用：引擎传递；边界：不隐式改变单位。
	Warnings []string `json:"warnings"`
	// Amounts 职责：保存参考每分钟净执行量，供对拍；使用：引擎传递；边界：不隐式改变单位。
	Amounts map[string]float64 `json:"amounts"`
}
