package planet

import (
	"errors"
	"math"
	"sync"
	"testing"

	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

func TestManufacturingPowerFloorUsesMandatoryArms(t *testing.T) {
	plan := industryplan.PlanetPlan{Power: &industryplan.PowerPlan{PowerMW: 100},
		Plan: quant.Result{Dataset: "vanilla", Productions: []quant.Production{{RecipeID: 17, MachineCount: 96, PowerMW: 90}}}}
	var source SourceLoadMW
	if got := source.shortfall(plan, 3); math.Abs(got-103.824) > 1e-6 {
		t.Fatalf("96 台石墨设备的机器加 192 把三级分拣器最低 %.6f MW", got)
	}
	plan.Power.PowerMW = 104
	if got := source.shortfall(plan, 3); got != 0 {
		t.Fatalf("补足供电后仍被提前排除：%.6f", got)
	}
	plan.Power.PowerMW = 100
	if got := source.shortfall(plan, 1); got != 0 {
		t.Fatalf("低档分拣器被按高档用电排除：%.6f", got)
	}
	plan.Plan.Productions[0].MachineType = 6
	if got := source.shortfall(plan, 3); got != 0 {
		t.Fatalf("堆叠设备被逐层重复计臂：%.6f", got)
	}
	plan.Plan.Productions[0].MachineType = 8
	plan.Plan.Productions[0].RecipeID = 115
	if got := source.shortfall(plan, 3); got != 0 {
		t.Fatalf("分馏塔的原生带口被计为分拣器：%.6f", got)
	}
}

// TestSourceLoadRechecksCurrentContract 使用同一已落位塔，分别改变发船、制造与供电合同，核对缓存命中时的补电量和完整搜索后备。
func TestSourceLoadRechecksCurrentContract(t *testing.T) {
	req := Request{Objects: []Object{stationObject(0, Position{}, 1001, 1, 0, false)}}
	load := measuredSourceLoad(req)
	if math.Abs(float64(load)-.06) > 1e-8 {
		t.Fatalf("采集供货塔的实装待机负荷 %.6f MW", load)
	}
	site := industryplan.PlanetPlan{CanRelocate: true, Power: &industryplan.PowerPlan{PowerMW: 104},
		Plan: quant.Result{Dataset: "vanilla", Productions: []quant.Production{{RecipeID: 17, MachineCount: 96, PowerMW: 90}}}}
	if err := load.CheckManufacturingPower(site, 3); err != nil {
		t.Fatal(err)
	}
	site.OutboundVesselMW = 10
	var shortage *industryplan.PowerLoadError
	if err := load.CheckManufacturingPower(site, 3); !errors.As(err, &shortage) || math.Abs(shortage.RequiredMW-113.884) > 1e-6 {
		t.Fatalf("新航线未按本轮合同补电：%v %+v", err, shortage)
	}
	site.Plan.Productions[0].PowerMW -= 4
	site.Power.ExternalGridMW = 6
	if err := load.CheckManufacturingPower(site, 3); err != nil {
		t.Fatalf("分工减载、外电补足后要求重复加电：%v", err)
	}
	site.Power.ExternalGridMW = 0
	site.CanRelocate = false
	if err := load.CheckManufacturingPower(site, 3); err != nil {
		t.Fatalf("完整搜索应保留实放前缀：%v", err)
	}
}

// TestPostponedHighLatitudeAfterPowerChange 复现一次低纬试摆、一次欠电完整搜索，再按原星补电后的真实调度次序核对。
func TestPostponedHighLatitudeAfterPowerChange(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	plan := moduleRegionTestPlan(t, ctx, 60)
	plan.Productions = append(plan.Productions, plan.Productions[0])
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 501, Plan: plan, Power: &industryplan.PowerPlan{PowerMW: .01}}}
	material, err := moduleMaterialPlan(ctx, plan, plan.Productions[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	id, err := independentTemplateIdentity(material, input.Options.Setting, 2)
	if err != nil {
		t.Fatal(err)
	}
	cache := ctx.Value(templateCacheKey{}).(*sync.Map)
	for _, region := range build.DensityLatitudeOptions() {
		if region.Position == "center" && region.Tier <= 4 {
			cache.Store(region.Value+"/"+id, moduleTemplate{err: errors.New("已核验低纬候选没有落点")})
		}
	}
	for _, step := range []struct {
		relocate, powerShortage bool
		supply                  float64
	}{{true, false, .01}, {false, true, .01}, {true, true, .01}, {true, false, 500}} {
		input.Plan.CanRelocate, input.Plan.Power.PowerMW = step.relocate, step.supply
		err := appendManufacturingRows(ctx, &Request{PlanetID: 501}, input, &industryplan.PlanetActual{}, catalog, nil, make([]int, 2))
		var miss *industryplan.FactoryPlacementError
		if !errors.As(err, &miss) || !miss.DeferredRegions || (miss.MinimumPowerMW > 0) != step.powerShortage {
			t.Fatalf("低纬试摆 %t，供电 %.2f，预期因欠电推迟 %t，实际 %+v", step.relocate, step.supply, step.powerShortage, err)
		}
	}
}
