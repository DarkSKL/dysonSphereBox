package industryworkspace

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/saveanalysis"
)

// applyTechnology 在求解与实物核验之前统一采矿和船运输入。旧请求保留显式参数，未指定部分使用存档。
func applyTechnology(req *Request, tech *saveanalysis.PlanningTechnology) (changes []string, err error) {
	if tech == nil {
		if req.Options.TechnologyMode == "save" {
			return nil, fmt.Errorf("未能读取此存档的采矿、运输和施工科技")
		}
		return nil, nil
	}
	saved := req.Options.TechnologyMode == "save"
	p := &req.Planning
	l := &p.Logistics
	if saved || l.SpeedMultiplier == 0 {
		l.SpeedMultiplier = 1
	}
	if saved || l.CargoMultiplier == 0 {
		l.CargoMultiplier = 1
	}
	if !saved && p.Mining.SpeedMultiplier == 0 {
		p.Mining.SpeedMultiplier = p.Quant.Settings.MiningSpeedMultiple
	}
	set := func(value *float64, factor, actual float64, label string) {
		if saved || *value == 0 {
			*value = actual
		}
		if effective := *value * factor; math.Abs(effective-actual) > 0.00001 {
			changes = append(changes, fmt.Sprintf("%s，存档 %g，模拟 %g", label, actual, effective))
		}
	}
	set(&p.Mining.SpeedMultiplier, 1, tech.MiningSpeed, "采矿倍率")
	p.Quant.Settings.MiningSpeedMultiple = p.Mining.SpeedMultiplier
	set(&l.CargoPerVessel, l.CargoMultiplier, float64(tech.ShipCargo), "单船运量")
	set(&l.CruiseSpeed, l.SpeedMultiplier, tech.CruiseSpeed, "运输船巡航速度")
	set(&l.WarpSpeed, l.SpeedMultiplier, tech.WarpSpeed, "运输船曲速")
	if saved || l.WarpEnabled == nil {
		warp := tech.WarpUnlocked
		l.WarpEnabled = &warp
	} else if *l.WarpEnabled != tech.WarpUnlocked {
		changes = append(changes, "曲速航行开关与存档解锁状态不同")
	}
	return changes, nil
}
