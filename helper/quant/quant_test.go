package quant

// 文件职责：覆盖量化内核 P1 规则的离线回归；使用：开发和验收运行 go test；边界：只验证内置数据与纯 Go 行为，不触发 UI、蓝图或游戏。

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// TestSolveMachineCountCeiling 职责：验证设备台数覆盖精确需求；使用：R-QC-001 回归；边界：只断言量化结果，不推断蓝图布局。
func TestSolveMachineCountCeiling(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	iron := d.ByName["铁块"]
	for _, rate := range []float64{60.24, 60, 59.9999999999, 60.0000000001} {
		result, err := Solve(context.Background(), Request{Targets: []Target{{ItemID: iron.ID, RatePerMinute: rate}}, Dataset: "Vanilla"})
		if err != nil {
			t.Fatalf("rate %v: %v", rate, err)
		}
		if len(result.Productions) != 1 {
			t.Fatalf("rate %v: got %d productions", rate, len(result.Productions))
		}
		p := result.Productions[0]
		if float64(p.MachineCount)+1e-9 < p.ExactMachineCount || p.RoundingMargin < -1e-9 {
			t.Fatalf("rate %v: count=%d exact=%.12f margin=%.12f", rate, p.MachineCount, p.ExactMachineCount, p.RoundingMargin)
		}
		if rate == 60.24 && p.MachineCount != 2 {
			t.Fatalf("60.24/min requires 2 machines, got %d", p.MachineCount)
		}
	}
}

// TestProductionModeZeroNormalizesPoints 职责：验证禁用喷涂时残留点数不影响结果；使用：R-QC-002 回归；边界：仅检查量化成本、流量、功耗与响应方案。
func TestProductionModeZeroNormalizesPoints(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	iron := d.ByName["铁块"]
	base := Request{Targets: []Target{{ItemID: iron.ID, RatePerMinute: 60.24}}, Dataset: "Vanilla"}
	withResidual := base
	withResidual.Scheme = Scheme{SchemeForRecipe: []RecipeScheme{{IncreasePoints: 4}}}
	a, err := Solve(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Solve(context.Background(), withResidual)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(a.LPCost-b.LPCost) > 1e-10 || math.Abs(a.PowerMW-b.PowerMW) > 1e-10 || len(a.Amounts) != len(b.Amounts) {
		t.Fatalf("mode0 residual points changed cost/power/amounts: cost %.12g/%.12g power %.12g/%.12g", a.LPCost, b.LPCost, a.PowerMW, b.PowerMW)
	}
	for item, amount := range a.Amounts {
		if math.Abs(amount-b.Amounts[item]) > 1e-10 {
			t.Fatalf("mode0 residual points changed %s: %v != %v", item, amount, b.Amounts[item])
		}
	}
	if b.Productions[0].IncreasePoints != 0 {
		t.Fatalf("normalized production retained %d points", b.Productions[0].IncreasePoints)
	}
}

// TestNaturalProductionValidationAndMultiplier 职责：验证固定产线复用方案校验和采集倍率；使用：R-QC-003 回归；边界：不调用蓝图生成或真实资源。
func TestNaturalProductionValidationAndMultiplier(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := normalizeSettings(Settings{MiningSpeedMultiple: 1})
	if err != nil {
		t.Fatal(err)
	}
	r := d.Recipes[d.Choices["铁矿"][0]]
	if _, err := normalizeRecipeScheme(d, r, RecipeScheme{Building: 0, ProductionMode: 2}); err == nil {
		t.Fatal("mining recipe accepted unsupported production mode")
	}
	base := &graph{settings: settings, dataset: d}
	needs1 := newRates()
	base.settings.NaturalProductionLine = []NaturalProduction{{TargetItem: "铁矿", RecipeChoice: 1, Building: 0, MachineCount: 1}}
	if err := base.balanceNatural(needs1); err != nil {
		t.Fatal(err)
	}
	boostedSettings := settings
	boostedSettings.MiningSpeedMultiple = 2
	boosted := &graph{settings: boostedSettings, dataset: d}
	needs2 := newRates()
	boosted.settings.NaturalProductionLine = base.settings.NaturalProductionLine
	if err := boosted.balanceNatural(needs2); err != nil {
		t.Fatal(err)
	}
	if needs1.values["铁矿"] == 0 || math.Abs(needs2.values["铁矿"]/needs1.values["铁矿"]-2) > 1e-10 {
		t.Fatalf("natural mining multiplier mismatch: %v vs %v", needs1.values["铁矿"], needs2.values["铁矿"])
	}
}

// TestSettingsRejectInvalidValues 职责：验证全部倍率字段拒绝非法值；使用：P2 设置入口回归；边界：零值仍代表兼容默认值。
func TestSettingsRejectInvalidValues(t *testing.T) {
	typ := reflect.TypeOf(Settings{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type.Kind() != reflect.Float64 {
			continue
		}
		t.Run(typ.Field(i).Name, func(t *testing.T) {
			for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1000001} {
				settings := Settings{}
				reflect.ValueOf(&settings).Elem().Field(i).SetFloat(value)
				if _, err := normalizeSettings(settings); err == nil {
					t.Errorf("accepted invalid value %v", value)
				}
			}
			for _, value := range []float64{0, 1e-6, 1e6} {
				settings := Settings{}
				reflect.ValueOf(&settings).Elem().Field(i).SetFloat(value)
				if _, err := normalizeSettings(settings); err != nil {
					t.Errorf("rejected valid value %v: %v", value, err)
				}
			}
		})
	}
	for _, digits := range []int{-1, 9} {
		if _, err := normalizeSettings(Settings{FixedNum: &digits}); err == nil {
			t.Errorf("accepted invalid precision %d", digits)
		}
	}
}

// TestSettingsDefaultsAndJSON 职责：冻结默认值和 JSON 字段名；使用：P2 请求兼容回归；边界：不验证前端控件布局或存储。
func TestSettingsDefaultsAndJSON(t *testing.T) {
	const defaultsJSON = `{
		"miningSpeedOil":3,"miningSpeedHydrogen":1,"miningSpeedDeuterium":0.05,"miningSpeedGasHydrate":0.8,
		"miningSpeedHelium":0.02,"miningSpeedAmmonia":0.3,"miningSpeedNitrogen":1.2,"miningSpeedOxygen":0.6,
		"miningSpeedCarbonDioxide":0.4,"miningSpeedSulfurDioxide":0.6,"coveredVeinsSmall":8,"coveredVeinsLarge":16,
		"miningEfficiencyLarge":3,"miningSpeedMultiple":1,"enemyDropMultiple":1,"icarusManufacturingSpeed":1,
		"fractionatingSpeed":30,"fixedNum":2,"stackResearchLab":15,"proliferateItself":true,"accRate":1,"incRate":1
	}`
	var expected Settings
	if err := json.Unmarshal([]byte(defaultsJSON), &expected); err != nil {
		t.Fatal(err)
	}
	settings, err := normalizeSettings(Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings, expected) {
		t.Fatalf("defaults changed: got %+v want %+v", settings, expected)
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip Settings
	if err := json.Unmarshal(raw, &roundtrip); err != nil || !reflect.DeepEqual(roundtrip, expected) {
		t.Fatalf("settings JSON roundtrip changed: %s (%v)", raw, err)
	}

	var explicit Settings
	if err := json.Unmarshal([]byte(`{"fixedNum":0,"proliferateItself":false,"blueBuff":true,"mineralizeList":{"铁矿":true},"naturalProductionLine":[{"targetItem":"铁矿","recipeChoice":1,"machineCount":2}]}`), &explicit); err != nil {
		t.Fatal(err)
	}
	explicit, err = normalizeSettings(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if *explicit.FixedNum != 0 || *explicit.ProliferateItself || !explicit.BlueBuff || !explicit.MineralizeList["铁矿"] || len(explicit.NaturalProductionLine) != 1 || explicit.NaturalProductionLine[0].MachineCount != 2 {
		t.Fatalf("explicit zero/false or existing line was lost: %+v", explicit)
	}
}
