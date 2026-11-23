package industryplan

// 文件职责：定义能源与戴森球规划条件；使用：工作区请求和逐星蓝图；边界：用户声明的供能不冒充存档测量值。

// EnergyChoice 职责：限定一颗星的供电方式；使用：默认策略与逐星覆盖；边界：空模式兼容旧版煤火电请求。
type EnergyChoice struct {
	Mode           string  `json:"mode"`           // auto 自动比较，fixed 指定，grid 接入现有电网。
	FacilityID     int     `json:"facilityId"`     // 指定的原版发电或放电设施。
	FuelItemID     int     `json:"fuelItemId"`     // 燃料物品；零按该设施的常规燃料选择。
	GridCapacityMW float64 `json:"gridCapacityMw"` // 用户声明可用于新增产线的电网余量。
	ChargePlanetID int64   `json:"chargePlanetId"` // 能量枢纽充电端星球，必须不同于用电星。
	SolarFactor    float64 `json:"solarFactor"`    // 玩家确认的日照期间可保证倍率。
	SolarDuty      float64 `json:"solarDuty"`      // 可保证的日照时间比例。
	NightSeconds   float64 `json:"nightSeconds"`   // 最长连续无光时长，用于蓄电容量。
}

// EnergyOverride 职责：覆盖单颗星球的电源；使用：星球检查面板；边界：同星球只允许一条。
type EnergyOverride struct {
	PlanetID int64        `json:"planetId"`
	Choice   EnergyChoice `json:"choice"`
}

// EnergySettings 职责：保存本轮能源策略；使用：分工候选；边界：不修改游戏设置。
type EnergySettings struct {
	Default            EnergyChoice     `json:"default"`
	AllowedFacilityIDs []int            `json:"allowedFacilityIds"` // 自动选择允许的设备；空表示全部。
	Overrides          []EnergyOverride `json:"overrides"`
}

// DysonSupply 职责：声明同一恒星可分配的戴森球功率；使用：光子和射线发电共享额度；边界：不得按每颗接收星重复计算。
type DysonSupply struct {
	StarID      int64   `json:"starId"`
	PlanetIDs   []int64 `json:"planetIds"`
	AvailableMW float64 `json:"availableMw"` // 已扣除其他用途的戴森球可用功率。
	Efficiency  float64 `json:"efficiency"`  // 射线传输效率，范围 (0,1]，由玩家按科技声明。
	Uptime      float64 `json:"uptime"`      // 声明可保证的接收连续率，范围 (0,1]。
}

// DysonSettings 职责：选择光子来源与透镜；使用：含光子需求的产业链；边界：未选择时不能隐式外供。
type DysonSettings struct {
	Mode     string        `json:"mode"` // unconfigured、sphere 或 external。
	Supplies []DysonSupply `json:"supplies"`
	Lenses   bool          `json:"lenses"`
}

// PowerSource 职责：记录已分配的供电或储能设备；使用：真实落位和结果显示；边界：PowerMW 为环境折算后的持续能力。
type PowerSource struct {
	FacilityID        int     `json:"facilityId"`
	FuelItemID        int     `json:"fuelItemId"`
	Count             int     `json:"count"`
	FuelRatePerMinute float64 `json:"fuelRatePerMinute"`
	PowerMW           float64 `json:"powerMw"`
	Factor            float64 `json:"factor"`         // 已核对的环境/燃料功率倍率。
	Mode              string  `json:"mode,omitempty"` // 枢纽 charge/discharge；蓄电器 buffer。
	PeerPlanetID      int64   `json:"peerPlanetId,omitempty"`
	StoredMJ          float64 `json:"storedMj,omitempty"`
	SpherePowerMW     float64 `json:"spherePowerMw,omitempty"`
}

// PhotonPlan 职责：描述光子接收设备和透镜供给；使用：专用接收模块；边界：功率来自已声明的同恒星额度。
type PhotonPlan struct {
	Count             int     `json:"count"`
	RatePerMinute     float64 `json:"ratePerMinute"`
	LensRatePerMinute float64 `json:"lensRatePerMinute"`
	SpherePowerMW     float64 `json:"spherePowerMw"`
	Efficiency        float64 `json:"efficiency"`
	Uptime            float64 `json:"uptime"`
}
