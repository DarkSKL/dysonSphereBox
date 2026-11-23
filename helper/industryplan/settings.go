package industryplan

import (
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"fmt"
	"math"
)

// MaxOrdinaryMinersPerVein 是原版矿点可登记的不同矿机数；实际覆盖、落位和出带由蓝图逐台核实。
const MaxOrdinaryMinersPerVein = 4

// enabled 职责：读取可缺省的开关；使用：规划选项；边界：显式 false 始终生效。
func enabled(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// normalize 职责：补齐可调的设计估值并拒绝非法设置；使用：Plan；边界：不修改输入引用。
func normalize(req Request) (Request, transport.VesselProfile, error) {
	if req.Energy.Default.Mode != "" && req.Dyson.Mode == "" {
		req.Dyson.Mode = "unconfigured"
	}
	settings := &req.Logistics
	defaults := map[*float64]float64{
		&settings.CargoPerVessel: 200, &settings.CruiseSpeed: 300,
		&settings.WarpSpeed: 1000000, &settings.WarpDistance: 120000,
		&settings.TurnaroundSeconds: 10, &settings.SpeedMultiplier: 1,
		&settings.CargoMultiplier: 1, &req.StorageMinutes: 60,
	}
	mining := &req.Mining
	if mining.FacilityID == 0 {
		mining.FacilityID = 2301
	}
	if mining.FacilityID != 2301 && mining.FacilityID != 2316 {
		return req, transport.VesselProfile{}, fmt.Errorf("不支持矿机原型 %d", mining.FacilityID)
	}
	covered, facility := req.Quant.Settings.CoveredVeinsSmall, 1.0
	if mining.FacilityID == 2316 {
		covered, facility = req.Quant.Settings.CoveredVeinsLarge, req.Quant.Settings.MiningEfficiencyLarge
		if covered == 0 {
			covered = 16
		}
		if facility == 0 {
			facility = 3
		}
	}
	if covered == 0 {
		covered = 8
	}
	multiplier := req.Quant.Settings.MiningSpeedMultiple
	if multiplier == 0 {
		multiplier = 1
	}
	defaults[&mining.CoveredVeins], defaults[&mining.FacilityMultiplier], defaults[&mining.SpeedMultiplier] = covered, facility, multiplier
	for value, fallback := range defaults {
		if *value == 0 {
			*value = fallback
		}
		if *value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0) {
			return req, transport.VesselProfile{}, fmt.Errorf("速率、倍率和仓储时长必须为有限正数")
		}
	}
	if mining.FacilityID == 2301 && mining.FacilityMultiplier != 1 || mining.FacilityID == 2316 && (mining.FacilityMultiplier < 1 || mining.FacilityMultiplier > 3) {
		return req, transport.VesselProfile{}, fmt.Errorf("小矿机工作倍率固定为 1，大矿机工作倍率须在 1 至 3 之间")
	}
	// 原版物流塔的最小曲速启用距离为 0.5 AU，蓝图参数使用整数游戏米。
	if settings.WarpDistance < 20000 || settings.WarpDistance > math.MaxInt32 {
		return req, transport.VesselProfile{}, fmt.Errorf("曲速启用距离须在 20000 至 2147483647 游戏米之间")
	}
	settings.WarpDistance = math.Round(settings.WarpDistance)
	if settings.VesselsPerStation == 0 {
		settings.VesselsPerStation = 10
	}
	if settings.MaxStationsPerPlanet < 0 || settings.VesselsPerStation < 1 || settings.VesselsPerStation > 10 {
		return req, transport.VesselProfile{}, fmt.Errorf("物流塔上限无效或每塔船数不在 1 到 10 范围")
	}
	profile := transport.VesselProfile{
		Cargo: settings.CargoPerVessel * settings.CargoMultiplier, CruiseSpeed: settings.CruiseSpeed * settings.SpeedMultiplier,
		WarpSpeed: settings.WarpSpeed * settings.SpeedMultiplier, WarpDistance: settings.WarpDistance, TurnaroundSeconds: settings.TurnaroundSeconds,
	}
	if !enabled(settings.WarpEnabled, true) {
		profile.WarpSpeed = 0
	}
	if _, _, err := profile.VesselTrip(1); err != nil {
		return req, profile, err
	}
	if req.Profile == "" {
		req.Profile = "standard"
	}
	if req.Profile != "standard" && req.Profile != "low" {
		return req, profile, fmt.Errorf("未知计算档位 %q", req.Profile)
	}
	if req.Preference == "" {
		req.Preference = "construction"
	}
	switch req.Preference {
	case "construction", "logistics", "energy", "game-load":
	default:
		return req, profile, fmt.Errorf("未知规划偏好 %q", req.Preference)
	}
	req.Quant.Settings.NaturalProductionLine = nil
	return req, profile, nil
}

// resourceCapacity 职责：从明确的采集依据推出速率和设施；使用：候选矿源；边界：液体与巨星无本地验证速率时返回零。
func resourceCapacity(req Request, res Resource, demand float64) (capacity, unit float64, facility int) {
	facility = req.Mining.FacilityID
	switch res.Kind {
	case "orbital":
		// 原版粘贴工具在巨星关闭。气体速率可展示，无法分配给一份可施工蓝图。
		return 0, 0, 0
	case "oil", "liquid", "solid":
		if !enabled(req.EnableMining, true) {
			return 0, 0, 0
		}
		if res.Kind == "oil" {
			facility = 2307
		}
		if res.Kind == "liquid" {
			facility = 2306
		}
	default:
		return 0, 0, 0
	}
	if res.RemainingItems != nil && *res.RemainingItems <= 0 {
		return 0, 0, facility
	}
	if res.CapacityPerMinute > 0 && res.UnitRatePerMinute > 0 {
		return res.CapacityPerMinute, res.UnitRatePerMinute, facility
	}
	if res.Kind == "liquid" && res.UnitRatePerMinute > 0 {
		// 已确认液面尚无岸线几何；本轮只预留所需泵数，最终回执须逐台证明进液和出带。
		return demand, res.UnitRatePerMinute * req.Mining.SpeedMultiplier, facility
	}
	if res.Kind == "solid" && res.Count > 0 {
		pointRate := SolidMiningRate(facility, req.Mining.SpeedMultiplier, req.Mining.FacilityMultiplier)
		capacity = res.Count * pointRate
		if facility == 2301 && (res.Source == "saved" || res.Source == "generated") && (req.probeSharedMining || req.sharedMiningItems[res.ItemID]) {
			capacity *= MaxOrdinaryMinersPerVein
		}
		return capacity, math.Min(res.Count, req.Mining.CoveredVeins) * pointRate, facility
	}
	if res.Kind == "oil" && res.Count > 0 && res.BaseRatePerMinute > 0 {
		total := res.BaseRatePerMinute * req.Mining.SpeedMultiplier
		return total, total / res.Count, facility
	}
	if res.BaseRatePerMinute > 0 && res.UnitRatePerMinute > 0 {
		return res.BaseRatePerMinute, res.UnitRatePerMinute, facility
	}
	return 0, 0, facility
}

// SolidMiningRate 根据固矿机的原版采矿周期、科技速度和工作档位，计算每个覆盖矿点的件/分。
func SolidMiningRate(facilityID int, speed, setting float64) float64 {
	if facilityID == 2316 {
		return 60 * speed * setting
	}
	return 30 * speed
}

// significant 职责：过滤量化容差内的数值误差；使用：净流与校验；边界：只接受正余量。
func significant(amount float64) bool {
	return amount > quant.FlowTolerance(0, 0)
}
