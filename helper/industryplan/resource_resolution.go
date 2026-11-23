package industryplan

import (
	"context"
	"maps"
	"math"
	"slices"
	"sort"

	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// resolveCandidateResources 先扣除有实数的供给，再精算可能填补缺口的行星。
// 每次规划只计算预算内的新地形，并在已知能力充足后停止。显式中心享有本地取材优先权。
func resolveCandidateResources(ctx context.Context, req Request, world World, base quant.Result, planets map[int64]Planet, ids []int64, limit int, progress func(Progress)) (int, bool, error) {
	if world.ResolveResources == nil || !enabled(req.EnableMining, true) {
		return 0, false, nil
	}
	gaps := map[int]float64{}
	explicit := map[int]bool{}
	for _, item := range base.ExternalInputItemIDs {
		explicit[item] = true
	}
	for _, raw := range base.RawInputs {
		if !explicit[raw.ItemID] {
			gaps[raw.ItemID] += raw.RatePerMinute
		}
	}
	if req.Energy.Default.Mode == "" && req.EnablePower && req.EnableFuel && base.PowerMW > 0 {
		gaps[coalItemID] += 60 * math.Ceil(base.PowerMW/2.16)
	}
	if len(gaps) == 0 {
		return 0, false, nil
	}
	demand := make(map[int]bool, len(gaps))
	for item := range gaps {
		demand[item] = true
	}
	credit := func(resources []Resource) {
		for _, res := range resources {
			if gap := gaps[res.ItemID]; gap > 0 {
				rate, _, _ := resourceCapacity(req, res, gap)
				gaps[res.ItemID] = math.Max(0, gap-rate)
			}
		}
	}
	type candidate struct {
		id       int64
		matches  int
		distance float64
	}
	var candidates []candidate
	for _, id := range ids {
		p := planets[id]
		if p.ResourcesKnown || p.resourcesChecked {
			credit(p.Resources)
			continue
		}
		if !p.Buildable {
			continue
		}
		score := 0
		for _, res := range p.Resources {
			if demand[res.ItemID] {
				score++
			}
		}
		if score == 0 && len(p.Resources) > 0 {
			continue
		}
		c := candidate{id: id, matches: score}
		if center, ok := planets[req.CenterPlanetID]; ok {
			c.distance = distance(center, p)
		}
		candidates = append(candidates, c)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if (a.id == req.CenterPlanetID) != (b.id == req.CenterPlanetID) {
			return a.id == req.CenterPlanetID
		}
		if a.matches != b.matches {
			return a.matches > b.matches
		}
		if a.distance != b.distance {
			return a.distance < b.distance
		}
		return a.id < b.id
	})
	inspected := 0
	for _, candidate := range candidates {
		if err := search.Checkpoint(ctx); err != nil {
			return inspected, false, err
		}
		p := planets[candidate.id]
		needed := false
		for _, r := range p.Resources {
			if significant(gaps[r.ItemID]) {
				needed = true
				break
			}
		}
		if len(p.Resources) == 0 {
			for _, gap := range gaps {
				if significant(gap) {
					needed = true
					break
				}
			}
		}
		if !needed && candidate.id != req.CenterPlanetID {
			continue
		}
		if inspected >= limit {
			return inspected, true, nil
		}
		resources, known, err := world.ResolveResources(ctx, p.ID)
		if err != nil {
			return inspected, false, err
		}
		inspected++
		p.Resources, p.ResourcesKnown, p.resourcesChecked = resources, known, true
		planets[p.ID] = p
		credit(resources)
		progress(Progress{Phase: "resources", Completed: inspected, Total: min(limit, len(candidates)), PlanetID: p.ID})
	}
	return inspected, false, nil
}

// allowSharedMining 在所有相关星球都已勘测且独占矿点仍不足时，允许本次规划尝试真实共享落位。
func allowSharedMining(req Request, base quant.Result, planets map[int64]Planet, ids []int64, validate Validator) Request {
	if validate == nil || req.Mining.FacilityID != 2301 || !enabled(req.EnableMining, true) {
		return req
	}
	gaps := miningSupplyGaps(req, base, planets, ids)
	if len(gaps) == 0 {
		return req
	}
	shared := make(map[int]bool, len(req.sharedMiningItems)+len(gaps))
	maps.Copy(shared, req.sharedMiningItems)
	for _, gap := range gaps {
		if gap.PendingPlanets == 0 && gap.UnknownPlanets == 0 {
			shared[gap.ItemID] = true
		}
	}
	req.sharedMiningItems = shared
	return req
}

// miningSupplyGaps 在星球分工前核对可采原料的全星区速率。只拦截即使汇总已知矿源仍有缺口的物品，
// 玩家外供与巨星供给沿用各自的规则；未精算星球与精算后资料不全的星球分别列出。
func miningSupplyGaps(req Request, base quant.Result, planets map[int64]Planet, ids []int64) []ResourceGap {
	var gaps []ResourceGap
	for _, raw := range base.RawInputs {
		if slices.Contains(base.ExternalInputItemIDs, raw.ItemID) {
			continue
		}
		gap := ResourceGap{ItemID: raw.ItemID, RequiredPerMinute: raw.RatePerMinute}
		ground, otherSource := false, false
		for _, id := range ids {
			p := planets[id]
			unknown := p.Buildable && !p.ResourcesKnown && len(p.Resources) == 0
			for _, res := range p.Resources {
				if res.ItemID != raw.ItemID {
					continue
				}
				if !p.Buildable || (res.Kind != "solid" && res.Kind != "oil" && res.Kind != "liquid") {
					otherSource = true
					continue
				}
				ground = true
				rate, unit, _ := resourceCapacity(req, res, raw.RatePerMinute)
				gap.AvailablePerMinute += rate
				unknown = !p.ResourcesKnown || unit <= 0 && res.Count > 0 && (res.RemainingItems == nil || *res.RemainingItems > 0)
			}
			if unknown {
				if p.resourcesChecked || p.ResourcesKnown {
					gap.UnknownPlanets++
				} else {
					gap.PendingPlanets++
				}
			}
		}
		if ground && !otherSource && gap.RequiredPerMinute-gap.AvailablePerMinute > quant.FlowTolerance(gap.RequiredPerMinute, gap.AvailablePerMinute) {
			gaps = append(gaps, gap)
		}
	}
	return gaps
}
