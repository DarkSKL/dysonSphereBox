package industryworkspace

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// Describe 在相同快照上关联资源与星球，保存时刻位置采用游戏米，星球无需已有厂区。
func Describe(saveID string, snapshot *saveanalysis.Snapshot) (Selection, industryplan.World) {
	world, resources := snapshot.World(), snapshot.PlanetResourceCatalog()
	stars := make(map[int64]string, len(world.Stars))
	for _, star := range world.Stars {
		stars[star.StarID] = star.Name
	}
	summaries := make(map[int64]saveanalysis.PlanetResourceSummary, len(resources.Planets))
	for _, summary := range resources.Planets {
		summaries[summary.PlanetID] = summary
	}
	view := Selection{SaveID: saveID, Technology: snapshot.PlanningTechnology(), SnapshotID: world.SHA256, Tick: world.Tick, GameVersion: resources.GameVersion, Stars: len(stars), StarSystems: world.Stars, Sites: make([]Site, 0, len(world.Planets))}
	input := industryplan.World{SnapshotID: world.SHA256, Planets: make([]industryplan.Planet, 0, len(world.Planets))}
	input.RemoteRules = snapshot.RemoteAstroRules
	input.RecipeUnlocked = snapshot.RecipeUnlocked
	input.CachedResources = func(id int64) ([]industryplan.Resource, bool, bool) {
		summary, found := snapshot.CachedPlanetResources(id)
		if !found {
			return nil, false, false
		}
		return summaryCapacities(summary), summary.Source == "saved" || summary.Source == "generated", true
	}
	input.ResolveResources = func(ctx context.Context, id int64) ([]industryplan.Resource, bool, error) {
		resolved, err := snapshot.ResolvePlanetResources(ctx, id)
		if err != nil {
			return nil, false, err
		}
		return summaryCapacities(resolved), resolved.Source == "saved" || resolved.Source == "generated", nil
	}
	for _, body := range world.Planets {
		res := summaries[body.PlanetID]
		if res.Source == "" {
			res.Source = "unknown"
		}
		view.Sites = append(view.Sites, Site{ID: body.PlanetID, StarID: body.StarID, Name: body.Name, Star: stars[body.StarID], Position: body.Position, ParentPlanetID: body.ParentPlanetID, Gas: res.Gas, Source: res.Source, Resources: res.Resources})
		p := industryplan.Planet{
			Radius: 200,
			ID:     body.PlanetID, StarID: body.StarID, Name: body.Name, Position: [3]float64{body.Position.X, body.Position.Y, body.Position.Z},
			Buildable: !res.Gas, HasFactory: body.HasFactory, ResourcesKnown: res.Source == "saved" || res.Source == "generated", Resources: summaryCapacities(res),
		}
		if res.Gas {
			p.Radius = 800
		}
		p.WindFactor, p.Lava = snapshot.PlanetEnergyEnvironment(body.PlanetID)
		p.DrySurface = p.Buildable && snapshot.PlanetHasDrySurface(body.PlanetID)
		input.Planets = append(input.Planets, p)
	}
	return view, input
}

// summaryCapacities 保留候选矿种供预算内精化，扫描地质量不计作可采供给。
func summaryCapacities(summary saveanalysis.PlanetResourceSummary) []industryplan.Resource {
	if summary.Source != "candidate" {
		return capacities(summary.Resources)
	}
	resources := make([]industryplan.Resource, 0, len(summary.Resources))
	for _, row := range summary.Resources {
		if row.Kind == "solid" || row.Kind == "oil" {
			row.Source = "candidate"
		}
		resources = append(resources, resourceCapacity(row))
	}
	return resources
}

func capacities(values []saveanalysis.PlanetResource) []industryplan.Resource {
	resources := make([]industryplan.Resource, 0, len(values))
	for _, value := range values {
		resources = append(resources, resourceCapacity(value))
	}
	return resources
}

// resourceCapacity 只用空闲矿点与可解释的速率供选址使用，原油编码量不作为固体件数。
func resourceCapacity(v saveanalysis.PlanetResource) industryplan.Resource {
	r := industryplan.Resource{ItemID: int(v.ItemID), Kind: v.Kind, Source: v.Source}
	if v.Source == "candidate" {
		return r
	}
	if v.Kind == "gas" {
		r.Kind = "orbital"
	}
	if v.Kind == "water" || v.Kind == "ocean" {
		r.Kind = "liquid"
	}
	if r.Kind == "liquid" && v.Source != "candidate" {
		// 原版抽水站周期为 1.2 秒。实际可放下多少台由陆地支点、进液点和带口验证。
		r.UnitRatePerMinute = 50
	}
	if v.AvailableCount != nil {
		r.Count = math.Max(0, float64(*v.AvailableCount))
	} else if v.Count != nil && v.OccupiedCount != nil {
		r.Count = math.Max(0, float64(*v.Count-*v.OccupiedCount))
	}
	if !v.Infinite && v.Amount != nil && v.Unit == "items" {
		amount := float64(*v.Amount)
		if v.AvailableAmount != nil {
			amount = math.Max(0, float64(*v.AvailableAmount))
		} else if v.OccupiedAmount != nil {
			amount = math.Max(0, amount-float64(*v.OccupiedAmount))
		}
		r.RemainingItems = &amount
	}
	if v.Rate != nil {
		rate := *v.Rate
		if v.AvailableRate != nil {
			rate = *v.AvailableRate
		} else if v.OccupiedRate != nil {
			rate -= *v.OccupiedRate
		}
		r.BaseRatePerMinute = math.Max(0, rate) * 60
	}
	return r
}

// Validate 检查桌面端独有的选项，量化和物流参数继续由领域引擎验证。
func (r *Request) Validate() error {
	switch r.Options.TechnologyMode {
	case "", "save", "custom":
	default:
		return fmt.Errorf("请选择存档科技或模拟科技")
	}
	if !quant.IsVanillaDatasetKey(r.Planning.Quant.Dataset) {
		return fmt.Errorf("跨星球规划使用原版数据集")
	}
	r.Planning.Quant.Dataset = "Vanilla"
	r.Planning.EnablePower, r.Planning.EnableFuel = r.Options.Power, r.Options.Fuel
	if len(r.Planning.Quant.Settings.NaturalProductionLine) > 0 {
		return fmt.Errorf("该规划只接受新建产线，现有设施按存档占用处理")
	}
	if r.Options.Foundation == "" {
		r.Options.Foundation = "none"
	}
	switch r.Options.Foundation {
	case "none", "construction", "planet":
	default:
		return fmt.Errorf("地基模式无效")
	}
	if r.Options.BeltLevel == 0 {
		r.Options.BeltLevel = 1
	}
	if r.Options.InserterLevel == 0 {
		r.Options.InserterLevel = 1
	}
	if r.Options.BeltLevel < 1 || r.Options.BeltLevel > 3 || r.Options.InserterLevel < 1 || r.Options.InserterLevel > 4 {
		return fmt.Errorf("传送带或分拣器等级无效")
	}
	if !r.Options.Spray {
		for _, scheme := range r.Planning.Quant.Scheme.SchemeForRecipe {
			if scheme.ProductionMode != 0 || scheme.IncreasePoints != 0 {
				return fmt.Errorf("关闭增产时，配方不能使用增产或加速收益")
			}
		}
	}
	return nil
}
