package industryplan

// 文件职责：核对实装负荷反馈后的原星补电与燃料闭合；使用：规划修复回归；边界：合成反馈只验证修复策略，实物负荷由蓝图生成器核对。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// TestPowerLoadFeedbackPreservesAssignment 职责：按上一轮估算递增倍率并固定当前全部制造块；使用：多星分工回归；边界：不污染原候选映射。
func TestPowerLoadFeedbackPreservesAssignment(t *testing.T) {
	policy := assignment{center: 101, strategy: 2,
		moves: map[string]int64{"a": 101}, changed: map[string]bool{"a": true},
		fullFactories: map[int64]bool{102: true}, excludedSources: map[resourceSite]bool{{planetID: 103, itemID: 1006}: true},
		powerMultipliers: map[int64]float64{101: 1.2, 102: 2.5}}
	previous := Result{Planets: []PlanetPlan{
		{PlanetID: 101, BlockIDs: []string{"a", "b"}, Power: &PowerPlan{RequiredMW: 144}},
		{PlanetID: 102, BlockIDs: []string{"c"}},
	}}
	rootCause := errors.New("实装负荷为 156.456 MW，供电 144 MW")
	feedback := &PowerLoadError{RequiredMW: 156.456, Err: rootCause}
	cause := fmt.Errorf("逐星复核：%w", feedback)
	if feedback.Error() != rootCause.Error() || !errors.Is(cause, rootCause) {
		t.Fatal("负荷反馈丢失原始错误")
	}
	got, ok := repairAssignment(policy, previous, 101, cause, nil, nil)
	if !ok || math.Abs(got.powerMultipliers[101]-1.2*156.456/144) > 1e-12 || got.powerMultipliers[102] != 2.5 {
		t.Fatalf("未按前轮负荷递增本星倍率：%+v，%t", got.powerMultipliers, ok)
	}
	if !reflect.DeepEqual(got.moves, map[string]int64{"a": 101, "b": 101, "c": 102}) {
		t.Fatalf("补电迁移或遗漏了既有制造块：%v", got.moves)
	}
	if got.center != policy.center || got.strategy != policy.strategy || !reflect.DeepEqual(got.changed, policy.changed) || !reflect.DeepEqual(got.fullFactories, policy.fullFactories) || !reflect.DeepEqual(got.excludedSources, policy.excludedSources) {
		t.Fatal("负荷反馈修改了其他分工条件")
	}
	if policy.powerMultipliers[101] != 1.2 || len(policy.moves) != 1 {
		t.Fatal("修复污染了原候选映射")
	}
	for _, old := range []float64{0, .5, 1} {
		policy.powerMultipliers[101] = old
		got, ok := repairAssignment(policy, previous, 101, cause, nil, nil)
		if !ok || math.Abs(got.powerMultipliers[101]-156.456/144) > 1e-12 {
			t.Fatalf("旧倍率 %g 未从 1 起算：%v，%t", old, got.powerMultipliers, ok)
		}
	}
}

// TestPowerLoadFeedbackRejectsNonIncreasing 职责：结束无效或不增大的负荷修复；使用：防止重复反馈与错误制造迁移；边界：保留正常容量失败的独立修复路径。
func TestPowerLoadFeedbackRejectsNonIncreasing(t *testing.T) {
	for _, tc := range []struct {
		name           string
		required, base float64
		old            float64
		missingPower   bool
		missingSite    bool
	}{
		{name: "equal", required: 144, base: 144},
		{name: "lower", required: 100, base: 144},
		{name: "negative", required: -1, base: 144},
		{name: "nan", required: math.NaN(), base: 144},
		{name: "infinite", required: math.Inf(1), base: 144},
		{name: "zero-base", required: 150},
		{name: "nan-base", required: 150, base: math.NaN()},
		{name: "infinite-base", required: 150, base: math.Inf(1)},
		{name: "nan-multiplier", required: 150, base: 144, old: math.NaN()},
		{name: "infinite-multiplier", required: 150, base: 144, old: math.Inf(1)},
		{name: "overflow", required: math.MaxFloat64, base: 1, old: 2},
		{name: "missing-power", required: 150, base: 144, missingPower: true},
		{name: "missing-site", required: 150, base: 144, missingSite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := assignment{center: 101, moves: map[string]int64{"a": 101}, powerMultipliers: map[int64]float64{101: tc.old}}
			site := PlanetPlan{PlanetID: 101, BlockIDs: []string{"a"}, Power: &PowerPlan{RequiredMW: tc.base},
				Plan: quant.Result{Productions: []quant.Production{{MachineCount: 10}}}}
			if tc.missingPower {
				site.Power = nil
			}
			previous := Result{Planets: []PlanetPlan{site}}
			if tc.missingSite {
				previous.Planets = nil
			}
			planets := map[int64]Planet{101: {ID: 101}, 102: {ID: 102, Position: [3]float64{100, 0, 0}}}
			cause := fmt.Errorf("复核：%w", &PowerLoadError{RequiredMW: tc.required, Err: errors.New("负荷反馈")})
			for repeat := 0; repeat < 3; repeat++ {
				got, ok := repairAssignment(policy, previous, 101, cause, planets, []int64{101, 102})
				if ok || !reflect.DeepEqual(got.moves, policy.moves) || math.Float64bits(got.powerMultipliers[101]) != math.Float64bits(tc.old) {
					t.Fatalf("第 %d 次无效反馈仍修复或迁移：%+v，%t", repeat+1, got, ok)
				}
				policy = got
			}
		})
	}
}

// TestPowerLoadFeedbackReplansFuelOnSamePlanet 职责：实装额外负荷同时补足电源、燃料制造和采矿；使用：完整规划入口回归；边界：成功回执只模拟蓝图复核，不替代设备落位测试。
func TestPowerLoadFeedbackReplansFuelOnSamePlanet(t *testing.T) {
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
			{ItemID: 1001, Kind: "solid", Count: 1000}, {ItemID: 1006, Kind: "solid", Count: 1000},
		}},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{1000, 0, 0}},
	}}
	req := Request{CenterPlanetID: 101, EnablePower: true, EnableFuel: true,
		Quant:  quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 240}}},
		Energy: EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2204, FuelItemID: 1109}}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var first *PlanetPlan
	var required float64
	var calls int
	result, err := Plan(ctx, req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 && site.Power != nil {
			calls++
			if first == nil {
				copy := site
				first = &copy
				required = site.Power.PowerMW + 4.32
			}
			if site.Power.PowerMW+1e-6 < required {
				return PlanetActual{}, &PowerLoadError{RequiredMW: required, Err: errors.New("加入实装分拣器后供电不足")}
			}
		}
		return PlanetActual{Ports: site.Ports, Power: site.Power}, nil
	}, nil)
	if err != nil || !result.Verified || result.Repairs != 1 || calls != 2 || first == nil {
		t.Fatalf("未一次补电完成复核：错误 %v，已验证 %t，修复 %d，调用 %d，状态 %s，说明 %v", err, result.Verified, result.Repairs, calls, result.Status, result.Warnings)
	}
	blocks := map[string]int64{}
	var final *PlanetPlan
	for i := range result.Planets {
		site := &result.Planets[i]
		if site.PlanetID == 101 {
			final = site
		}
		for _, blockID := range site.BlockIDs {
			blocks[blockID] = site.PlanetID
		}
		if site.PlanetID != 101 && len(site.Plan.Productions) > 0 {
			t.Fatalf("供电失败导致制造外迁：星球 %d，%v", site.PlanetID, site.BlockIDs)
		}
	}
	for _, blockID := range first.BlockIDs {
		if blocks[blockID] != 101 {
			t.Fatalf("原制造块 %s 被迁移或遗失：%v", blockID, blocks)
		}
	}
	if final == nil || final.Power == nil || final.Power.Count <= first.Power.Count || final.Power.FuelRatePerMinute <= first.Power.FuelRatePerMinute || final.Power.RequiredMW < required || final.Power.PowerMW < final.Power.RequiredMW {
		t.Fatalf("负荷反馈未补足本星电厂和燃料：初始 %+v，最终 %+v，要求 %g", first.Power, final, required)
	}
	fuelOutput, coalRate := 0.0, 0.0
	for _, production := range final.Plan.Productions {
		for _, output := range production.Outputs {
			if output.ItemID == 1109 {
				fuelOutput += output.RatePerMinute
			}
		}
		for _, input := range production.Inputs {
			if input.ItemID == 1109 {
				fuelOutput -= input.RatePerMinute
			}
		}
	}
	for _, mine := range final.Mines {
		if mine.ItemID == 1006 {
			coalRate += mine.RatePerMinute
		}
	}
	if fuelOutput+1e-6 < final.Power.FuelRatePerMinute || coalRate+1e-6 < final.Power.FuelRatePerMinute*2 || hasExternalKind(result, "fuel-input") || hasExternalKind(result, "grid-power") {
		t.Fatalf("新增燃料链未闭合：石墨产出 %g，煤矿 %g，燃料 %g，外供 %+v", fuelOutput, coalRate, final.Power.FuelRatePerMinute, result.External)
	}
}
