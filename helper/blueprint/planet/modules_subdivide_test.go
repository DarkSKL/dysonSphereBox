package planet

import (
	"math"
	"reflect"
	"testing"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestWideRecipePreservesAssignment 验证实物分块保留原分工编号、工序流量、额定能力与未拆分的普通工序。
func TestWideRecipePreservesAssignment(t *testing.T) {
	ctx := t.Context()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	request := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6006, RatePerMinute: 2000}}}
	for _, recipe := range dataset.Recipes {
		setting := quant.RecipeScheme{}
		if recipe.Proliferator&2 != 0 {
			setting.ProductionMode, setting.IncreasePoints = 2, 4
		}
		request.Scheme.SchemeForRecipe = append(request.Scheme.SchemeForRecipe, setting)
	}
	base, err := quant.Solve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	base, err = industryplan.SplitProductions(ctx, base, 32)
	if err != nil {
		t.Fatal(err)
	}
	var matrix quant.Production
	for _, row := range base.Productions {
		if row.MainItemID == 6006 && row.MachineCount == 32 {
			matrix = row
			break
		}
	}
	if matrix.Capacity == nil || len(matrix.Inputs) < 7 {
		t.Fatal("测试需要具备可信额定能力的七路进料合同")
	}
	ordinary := moduleRegionTestPlan(t, ctx, 60).Productions[0]
	base, err = manufacturingMaterial(ctx, base, []quant.Production{ordinary, matrix}, 0)
	if err != nil {
		t.Fatal(err)
	}
	site := industryplan.PlanetPlan{PlanetID: 303, Plan: base, BlockIDs: []string{"铸造", "研究"}}
	built, parents, err := subdivideWideRecipes(ctx, site)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parents, []int{0, 1, 1, 1, 1}) || len(site.Plan.Productions) != 2 || !reflect.DeepEqual(site.BlockIDs, []string{"铸造", "研究"}) ||
		built.PlanetID != site.PlanetID || built.BlockIDs[0] != "铸造" || built.BlockIDs[1] == built.BlockIDs[4] || !reflect.DeepEqual(built.Plan.Productions[0], ordinary) {
		t.Fatalf("原星、原块或施工小块映射错误：%v %v", parents, built.BlockIDs)
	}
	var amount, rated float64
	for _, row := range built.Plan.Productions[1:] {
		if row.MachineCount != 8 {
			t.Fatal("施工小块机器数", row.MachineCount)
		}
		amount += row.OutputCount
		rated += row.Capacity.OutputPerMinute
	}
	if math.Abs(amount-matrix.OutputCount) > 1e-6 || math.Abs(rated-matrix.Capacity.OutputPerMinute) > 1e-6 || built.Plan.FlowGraph == nil {
		t.Fatal("分块改变了净产量、额定产能或供需图", amount, rated)
	}
}
