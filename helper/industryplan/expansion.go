package industryplan

// 文件职责：在已验证的获选星球内扩产；使用：充分开发选项；边界：保留最近可施工结果，不改变用户原始目标和供能声明。

import (
	"context"
	"slices"

	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// Plan 职责：先验证净交付，再按需充分开发获选星球；使用：完整规划入口；边界：扩产始终保留最近一次可施工结果。
func Plan(ctx context.Context, req Request, world World, validate Validator, progress func(Progress)) (Result, error) {
	best, err := planOnce(ctx, req, world, validate, progress)
	if err != nil {
		return best, err
	}
	if best.Verified {
		best.AchievedTargets = slices.Clone(req.Quant.Targets)
	}
	if !req.DevelopFully || !best.Verified {
		return best, nil
	}
	selected := map[int64]bool{}
	for _, p := range best.Planets {
		selected[p.PlanetID] = true
	}
	fixed := world
	fixed.Planets = nil
	for _, p := range world.Planets {
		if selected[p.ID] {
			fixed.Planets = append(fixed.Planets, p)
		}
	}
	req = expansionReferences(req, selected)
	base := slices.Clone(req.Quant.Targets)
	req.CenterPlanetID = best.CenterPlanetID
	req.DevelopFully = false
	low, high, factor := 1.0, 0.0, 2.0
	for {
		if err := search.Checkpoint(ctx); err != nil {
			return Result{}, err
		}
		q := req
		q.Quant.Targets = slices.Clone(base)
		for i := range q.Quant.Targets {
			q.Quant.Targets[i].RatePerMinute *= factor
		}
		if progress != nil {
			progress(Progress{Phase: "expand"})
		}
		next, err := planOnce(ctx, q, fixed, validate, progress)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if err == nil && next.Verified {
			best = next
			best.AchievedTargets = slices.Clone(q.Quant.Targets)
			low = factor
		} else {
			high = factor
			best.ExpansionNote = "已保留选中星球上通过全部核验的增产方案；继续扩产受到资源、运输或可用布局限制"
		}
		if high > 0 && (high-low)/low <= .02 {
			break
		}
		if high == 0 {
			factor *= 2
		} else {
			factor = (low + high) / 2
		}
	}
	best.ExtraTargets = nil
	for i, target := range best.AchievedTargets {
		if extra := target.RatePerMinute - base[i].RatePerMinute; extra > quant.FlowTolerance(0, 0) {
			best.ExtraTargets = append(best.ExtraTargets, quant.Target{ItemID: target.ItemID, RatePerMinute: extra})
		}
	}
	return best, nil
}

// expansionReferences 职责：清理扩产范围之外的可选引用；使用：固定获选星球后构造局部请求；边界：不修改原请求，不增加恒星功率，不把被移除的光子接收声明改成外供。
func expansionReferences(req Request, selected map[int64]bool) Request {
	if !selected[req.StoragePlanetID] {
		req.StoragePlanetID = 0
	}
	// 首轮没有用到的仓储位置交回范围内自动选择；启用中的充电端已由首轮计划保留。
	choice := func(value EnergyChoice) EnergyChoice {
		if value.FacilityID == 2209 && !selected[value.ChargePlanetID] && (!req.EnablePower || value.Mode != "fixed") {
			value.FacilityID, value.ChargePlanetID = 0, 0
			if value.Mode == "fixed" {
				value.Mode = "auto"
			}
		}
		return value
	}
	req.Energy.Default = choice(req.Energy.Default)
	overrides := req.Energy.Overrides
	req.Energy.Overrides = nil
	for _, override := range overrides {
		if selected[override.PlanetID] {
			override.Choice = choice(override.Choice)
			req.Energy.Overrides = append(req.Energy.Overrides, override)
		}
	}
	supplies := req.Dyson.Supplies
	req.Dyson.Supplies = nil
	for _, supply := range supplies {
		planets := supply.PlanetIDs
		supply.PlanetIDs = nil
		for _, id := range planets {
			if selected[id] {
				supply.PlanetIDs = append(supply.PlanetIDs, id)
			}
		}
		if len(supply.PlanetIDs) > 0 {
			req.Dyson.Supplies = append(req.Dyson.Supplies, supply)
		}
	}
	if req.Dyson.Mode == "sphere" && len(req.Dyson.Supplies) == 0 {
		req.Dyson.Mode = "unconfigured"
	}
	return req
}
