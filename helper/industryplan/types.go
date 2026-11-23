package industryplan

import (
	"DysonSphereBox/helper/quant"
	"context"
)

// Request 职责：定义全新产线与全球规划设置；使用：调用 Plan；边界：固定量化配方与设备。
type Request struct {
	energySupply []quant.ItemRate
	// 原配方施工后备只运行一次，避免自动裂解方案失败后再次选择同一配方。
	keepOilRecipe bool
	// 只在本次能源迭代回收过量辅助生产后，优先继续使用已选中的可行自动电源。
	settledAutoPower map[int64]int
	// 仅给实际蓝图复核的单星试排或已确认总量不足的矿种开放共享上界。
	probeSharedMining bool
	sharedMiningItems map[int]bool
	// 补计发电燃料、透镜与副产物在定电源之后才分配的外运负荷。
	postPowerShipping map[int64]float64
	// ConstructionInserterLevel 由施工入口按存档设定，用于估算各配方必须安装的分拣器功耗。
	ConstructionInserterLevel int `json:"-"`
	// Energy、Dyson 职责：保存电源和光子前提；使用：完整规划；边界：只对本次快照生效。
	Energy EnergySettings `json:"energy"`
	Dyson  DysonSettings  `json:"dyson"`
	// DevelopFully 职责：在获选星球扩充同一目标链；使用：可选充分开发；边界：不增加无关产品。
	DevelopFully            bool              `json:"developFully"`
	Quant                   quant.Request     `json:"quant"`
	CenterPlanetID          int64             `json:"centerPlanetId"`
	DeliveryPlanetIDs       map[int]int64     `json:"deliveryPlanetIds"`
	TargetPlanetIDs         []int64           `json:"targetPlanetIds,omitempty"`
	Preference              string            `json:"preference"`
	Profile                 string            `json:"profile"`
	EnableMining            *bool             `json:"enableMining"`
	EnableOrbitalCollectors *bool             `json:"enableOrbitalCollectors"`
	EnableLogistics         *bool             `json:"enableLogistics"`
	EnableStorage           *bool             `json:"enableStorage"`
	EnablePower             bool              `json:"enablePower"`
	EnableFuel              bool              `json:"enableFuel"`
	StoragePlanetID         int64             `json:"storagePlanetId"`
	StorageMinutes          float64           `json:"storageMinutes"`
	Mining                  MiningSettings    `json:"mining"`
	Logistics               LogisticsSettings `json:"logistics"`
}

// deliveryPlanet 优先使用交付行指定的星球，兼容按物品设置目的地的规划请求。
func (r Request) deliveryPlanet(index int, itemID int) int64 {
	if len(r.TargetPlanetIDs) > 0 {
		return r.TargetPlanetIDs[index]
	}
	return r.DeliveryPlanetIDs[itemID]
}

// MiningSettings 职责：描述固体矿设备与科技；使用：由资源点数估算新建设备；边界：速率采用件/分钟。
type MiningSettings struct {
	FacilityID         int     `json:"facilityId"`
	CoveredVeins       float64 `json:"coveredVeins"`
	SpeedMultiplier    float64 `json:"speedMultiplier"`
	FacilityMultiplier float64 `json:"facilityMultiplier"`
}

// LogisticsSettings 职责：定义固定舰船物流参数；使用：估算线路运力；边界：速度采用游戏米/秒。
type LogisticsSettings struct {
	// WarpEnabled 由科技或模拟设置决定曲速航行；nil 沿用调用方以速度配置曲速的旧行为。
	WarpEnabled          *bool   `json:"warpEnabled,omitempty"`
	CargoPerVessel       float64 `json:"cargoPerVessel"`
	CruiseSpeed          float64 `json:"cruiseSpeed"`
	WarpSpeed            float64 `json:"warpSpeed"`
	WarpDistance         float64 `json:"warpDistance"`
	TurnaroundSeconds    float64 `json:"turnaroundSeconds"`
	SpeedMultiplier      float64 `json:"speedMultiplier"`
	CargoMultiplier      float64 `json:"cargoMultiplier"`
	VesselsPerStation    int     `json:"vesselsPerStation"`
	MaxStationsPerPlanet int     `json:"maxStationsPerPlanet"`
}

// World 职责：提供同一存档时刻的星球快照；使用：全局分工；边界：规划后不访问存档。
type World struct {
	RemoteRules    func(from, to, item int64) (routed, banned bool) `json:"-"`
	RecipeUnlocked func(id int) bool                                `json:"-"`
	SnapshotID     string                                           `json:"snapshotId"`
	Planets        []Planet                                         `json:"planets"`
	// ResolveResources 按需要精算一个候选星球，不在页面选档时展开整个星系的地形。
	ResolveResources func(context.Context, int64) ([]Resource, bool, error) `json:"-"`
	// CachedResources 职责：读取当前快照已精算的资源；使用：跨任务复用；边界：可选，found 表示完成，known 表示有可用的确定数据。
	CachedResources func(planetID int64) (resources []Resource, known bool, found bool) `json:"-"`
}

// Planet 职责：描述候选星球；使用：分工和距离估算；边界：位置采用存档的游戏米。
type Planet struct {
	Radius         float64    `json:"radius,omitempty"`
	ID             int64      `json:"id"`
	StarID         int64      `json:"starId"`
	Name           string     `json:"name"`
	Position       [3]float64 `json:"position"`
	Buildable      bool       `json:"buildable"`
	HasFactory     bool       `json:"hasFactory"`
	ResourcesKnown bool       `json:"resourcesKnown"`
	Resources      []Resource `json:"resources"`
	WindFactor     float64    `json:"windFactor"`
	Lava           bool       `json:"lava"`
	// DrySurface 仅在固态星主题确认没有液面时为真，用于制造溢出的选址顺序。
	DrySurface bool `json:"-"`
	// resourcesChecked 标出本快照尝试过的星球，精算后仍未知的来源保留为未知。
	resourcesChecked bool
}

// Resource 职责：描述单星球单物品的采集依据；使用：外供与新采集分配；边界：零速率表示缺少可用速率。
type Resource struct {
	ItemID            int      `json:"itemId"`
	Kind              string   `json:"kind"`
	Source            string   `json:"source"`
	Count             float64  `json:"count"`
	RemainingItems    *float64 `json:"remainingItems"`
	BaseRatePerMinute float64  `json:"baseRatePerMinute"`
	CapacityPerMinute float64  `json:"capacityPerMinute"`
	UnitRatePerMinute float64  `json:"unitRatePerMinute"`
}

// Port 职责：描述星球边界净流量；使用：布局验证；边界：速率采用件/分钟，星际角色保留对端星球。
type Port struct {
	ItemID        int     `json:"itemId"`
	Kind          string  `json:"kind"`
	RatePerMinute float64 `json:"ratePerMinute"`
	StorageItems  int     `json:"storageItems"`
	PeerPlanetID  int64   `json:"peerPlanetId"`
}

// Mine 职责：记录外部布置的矿产采集设施；使用：逐星球交付与复核；边界：记录速率依据与可知的耗尽时间。
type Mine struct {
	ItemID        int     `json:"itemId"`
	Kind          string  `json:"kind"`
	Source        string  `json:"source"`
	FacilityID    int     `json:"facilityId"`
	Count         int     `json:"count"`
	RatePerMinute float64 `json:"ratePerMinute"`
	CoveredItems  *int64  `json:"coveredItems,omitempty"`
	// AvailableMinutes 按实际覆盖储量与目标产量折算，假设采出一件消耗一份矿，不预测单矿枯竭时刻。
	AvailableMinutes   *float64 `json:"availableMinutes,omitempty"`
	SpeedMultiplier    float64  `json:"speedMultiplier"`
	FacilityMultiplier float64  `json:"facilityMultiplier"`
}

// PlanetPlan 职责：传递一颗星球的可信制造切片和出入口；使用：逐星球蓝图；边界：可能只有采集或交付。
type PlanetPlan struct {
	// OutboundVesselMW 为本星按满载发船且不抵扣回程带货的平均派船用电。
	OutboundVesselMW float64 `json:"outboundVesselMw,omitempty"`
	// OutboundFleet 按货种列出本星发船的并发船时；每座实际远供塔共享其全部船位。
	OutboundFleet     []FleetCargo `json:"outboundFleet,omitempty"`
	VesselsPerStation int          `json:"vesselsPerStation,omitempty"`
	FleetStationLimit int          `json:"fleetStationLimit,omitempty"`
	// CanRelocate 是本次单星核验的搜索提示。另有已开工星球可承接时，可先反馈拥挤地带，完整纬区仍保留后备。
	CanRelocate bool `json:"-"`
	// CenterRegionsOnly 先恢复前四档中心的完整选优，剩余纬区仍能以原分工重试。
	CenterRegionsOnly bool `json:"-"`
	// SinglePlanet 标记整次分工仅在本星施工，允许单星直连组合复用同轮的另一种已验证形状。
	SinglePlanet bool `json:"-"`
	// BlockIDs 职责：关联稳定生产模块；使用：局部重排；边界：与 Productions 逐项对应。
	BlockIDs     []string     `json:"-"`
	Photons      *PhotonPlan  `json:"photons,omitempty"`
	ManualRoutes bool         `json:"manualRoutes,omitempty"`
	PlanetID     int64        `json:"planetId"`
	Plan         quant.Result `json:"plan"`
	Ports        []Port       `json:"ports"`
	Mines        []Mine       `json:"mines"`
	StationCount int          `json:"stationCount"`
	VesselCount  int          `json:"vesselCount"`
	// WarpDistance 将估算所用的曲速启用距离写入本星新塔，单位游戏米。
	WarpDistance float64    `json:"warpDistance"`
	Power        *PowerPlan `json:"power,omitempty"`
}

// PowerPlan 职责：定义逐星供电、储能和燃料合同；使用：蓝图发电设施；边界：额定发电与实际负荷分别记录。
type PowerPlan struct {
	// Sources 职责：保存混合电源及储能设备；使用：新规划；边界：旧字段仅回显第一种燃料设备。
	Sources           []PowerSource `json:"sources,omitempty"`
	FacilityID        int           `json:"facilityId"`
	FuelItemID        int           `json:"fuelItemId"`
	Count             int           `json:"count"`
	FuelRatePerMinute float64       `json:"fuelRatePerMinute"`
	PowerMW           float64       `json:"powerMw"`
	ExternalGridMW    float64       `json:"externalGridMw,omitempty"`
	RequiredMW        float64       `json:"requiredMw"`
}

// PlanetActual 职责：回传蓝图可验证吞吐与缓存；使用：规划终检；边界：只填实际支持的角色。
type PlanetActual struct {
	Fleet    []FleetStation `json:"fleet,omitempty"`
	Ports    []Port         `json:"ports"`
	Warnings []string       `json:"warnings"`
	Artifact any            `json:"-"`
	Power    *PowerPlan     `json:"power,omitempty"`
}

// Validator 职责：在父任务下生成和复核一颗星球；使用：Plan；边界：取消随父任务传播。
type Validator func(context.Context, PlanetPlan) (PlanetActual, error)

// FleetCargo 给出一种远供货物的等效在途船数，船时可由多座实际具有此货槽的塔承担。
type FleetCargo struct {
	ItemID     int     `json:"itemId"`
	VesselLoad float64 `json:"vesselLoad"`
	// MaxVesselMW 是该货种各出航路线中每艘占用船时的最大平均扣电，用于逐塔核对充电档位。
	MaxVesselMW float64 `json:"maxVesselMw,omitempty"`
}

// FleetStation 是玩家在实物远供塔上装船的清单。经纬度为度，建筑索引取整星蓝图拆分前的编号。
type FleetStation struct {
	BuildingIndex int32        `json:"buildingIndex"`
	Longitude     float64      `json:"longitude"`
	Latitude      float64      `json:"latitude"`
	Vessels       int          `json:"vessels"`
	Cargo         []FleetCargo `json:"cargo"`
}

// Route 职责：说明星际物品运输与船队负载；使用：全局复核和界面；边界：速率和航时是固定参数下的估计。
type Route struct {
	// VesselEnergyMJ 与 VesselPowerMW 分别是一次派遣扣电与本行满载船次的平均用电。
	VesselEnergyMJ      float64 `json:"vesselEnergyMj,omitempty"`
	VesselPowerMW       float64 `json:"vesselPowerMw,omitempty"`
	FromPlanetID        int64   `json:"fromPlanetId"`
	ToPlanetID          int64   `json:"toPlanetId"`
	ItemID              int     `json:"itemId"`
	RatePerMinute       float64 `json:"ratePerMinute"`
	Distance            float64 `json:"distance"`
	RoundTripSeconds    float64 `json:"roundTripSeconds"`
	VesselLoad          float64 `json:"vesselLoad"`
	WarperRatePerMinute float64 `json:"warperRatePerMinute"`
	Interstellar        bool    `json:"interstellar"`
}

// External 职责：说明需要调用方补充的前提；使用：结果展示与验收；边界：按地点、物品和真实缺口记录。
type External struct {
	PlanetID      int64   `json:"planetId"`
	PeerPlanetID  int64   `json:"peerPlanetId,omitempty"`
	ItemID        int     `json:"itemId"`
	RatePerMinute float64 `json:"ratePerMinute"`
	PowerMW       float64 `json:"powerMw,omitempty"`
	Kind          string  `json:"kind"`
}

// Progress 职责：报告候选和验证进度；使用：父任务进度；边界：调用在任务主协程串行执行。
type Progress struct {
	Phase     string `json:"phase"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	PlanetID  int64  `json:"planetId"`
	// WorkDone 和 WorkTotal 为本次分工的布局工作量，仅供任务估时使用。
	WorkDone     int  `json:"-"`
	WorkTotal    int  `json:"-"`
	SimpleLayout bool `json:"-"`
	// RepairReason 保留真实存档验收需要的首个布局失败原因，不作为界面文案。
	RepairReason string `json:"-"`
}

// ResourceGap 说明全星区采集能力与原料需求的差额，供布局前判断；可用速率仍须通过矿机落位复核。
type ResourceGap struct {
	ItemID             int     `json:"itemId"`             // 所需原料。
	RequiredPerMinute  float64 `json:"requiredPerMinute"`  // 量化后的净需求。
	AvailablePerMinute float64 `json:"availablePerMinute"` // 已查明矿点按当前矿速估算的采集上限。
	PendingPlanets     int     `json:"pendingPlanets"`     // 可能供给该原料、尚可精算的星球。
	UnknownPlanets     int     `json:"unknownPlanets"`     // 已查询但仍无法确认供给的星球。
}

// Result 职责：返回完整全局路线与经核对的星球切片；使用：规划调用方；边界：带前提的交付必须显示条件。
type Result struct {
	postPowerShipping map[int64]float64
	settledAutoPower  map[int64]int
	AchievedTargets   []quant.Target `json:"achievedTargets"`
	ExtraTargets      []quant.Target `json:"extraTargets"`
	ExpansionNote     string         `json:"expansionNote"`
	// EnergyNeeds 职责：回传发电燃料与透镜的制造需求；使用：能源供应链闭合；边界：不作为玩家净交付。
	EnergyNeeds        []quant.ItemRate       `json:"-"`
	ResourcesInspected int                    `json:"resourcesInspected"`
	LimitStage         string                 `json:"limitStage,omitempty"`   // 达到本轮上限的阶段。
	ResourceGaps       []ResourceGap          `json:"resourceGaps,omitempty"` // 还没有足够供给依据的原料。
	CenterPlanetID     int64                  `json:"centerPlanetId"`
	StoragePlanetID    int64                  `json:"storagePlanetId,omitempty"`
	Plan               quant.Result           `json:"plan"`
	Planets            []PlanetPlan           `json:"planets"`
	Routes             []Route                `json:"routes"`
	External           []External             `json:"external"`
	Status             string                 `json:"status"`
	Verified           bool                   `json:"verified"`
	Warnings           []string               `json:"warnings"`
	Preference         string                 `json:"preference"`
	Score              float64                `json:"score"`
	ScoreBasis         string                 `json:"scoreBasis"`
	Logistics          LogisticsSettings      `json:"logistics"`
	Candidates         int                    `json:"candidates"`
	Repairs            int                    `json:"repairs"`
	Actuals            map[int64]PlanetActual `json:"-"`
}
