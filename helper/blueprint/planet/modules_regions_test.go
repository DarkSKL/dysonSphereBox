package planet

// 文件职责：验证制造分块使用生成器真实选区而非稀疏固定纬行；使用：空地利用率回归；边界：合成干地证明漏试区域可用，不代表任意存档整星均已搜尽。

import (
	"context"
	"math"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// TestManufacturingUsesPreviouslySkippedSouthernBand 职责：在原固定列表漏试的南第二纬带连续放下两个32机模块；使用：真实生成、地形与整星编解码；边界：测试只有此纬带是陆地，不能借用赤道落位。
func TestManufacturingUsesPreviouslySkippedSouthernBand(t *testing.T) {
	t.Run("完整选区", func(t *testing.T) { verifySouthernBand(t, false) })
	t.Run("可在已用星之间分摊", func(t *testing.T) { verifySouthernBand(t, true) })
}

func verifySouthernBand(t *testing.T, canRelocate bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = search.WithControl(ctx, search.New(2*time.Minute, nil))
	var generationAttempts []int
	ctx = build.WithWorkflowFinish(ctx, func(trace *workflow.RunResult, _ error) {
		attempts := 0
		if trace != nil {
			for _, step := range trace.Trace {
				if step.Tool == "station.assemble@1" {
					attempts++
				}
			}
		}
		generationAttempts = append(generationAttempts, attempts)
	})
	base := moduleRegionTestPlan(t, ctx, 4800)
	if len(base.Productions) != 2 || base.MachineCount != 64 || base.Productions[0].MachineCount != 32 || base.Productions[1].MachineCount != 32 {
		t.Fatalf("回归样例必须是两个32机喷涂铁块模块：%+v", base.Productions)
	}
	needPower := true
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Plan: base, CanRelocate: canRelocate},
		Options: ProductionOptions{Setting: build.Setting{NeedPower: &needPower, BeltLv: 3, InserterLv: 3}}}
	for _, raw := range base.RawInputs {
		input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: raw.ItemID, Kind: "external", RatePerMinute: raw.RatePerMinute})
	}
	input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: 1101, Kind: "export", PeerPlanetID: 102, RatePerMinute: 4800})
	var south, north float64
	for _, region := range build.DensityLatitudeOptions() {
		if region.Value == "ring2-south-center" {
			south, north = region.LatitudeLow*math.Pi/180, region.LatitudeHigh*math.Pi/180
		}
	}
	if south >= north || north >= 0 {
		t.Fatal("生成器未提供南第二纬带")
	}
	req := Request{PlanetID: 101, wet: func(p Position) (bool, bool) {
		return p.Latitude < south || p.Latitude > north, true
	}}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	actual := industryplan.PlanetActual{}
	if err := appendManufacturing(ctx, &req, input, &actual, catalog); err != nil {
		t.Fatalf("已有完整南纬干地却提前迁移制造：%v", err)
	}
	// 赤道、北第二带和南第二带各完整选优一次；第二个相同模块应复用这三个模板。
	if len(generationAttempts) != 3 {
		t.Fatalf("未复用同规格的完整选优模板：各次装配候选数 %v", generationAttempts)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	modules := map[string]int{}
	minRow, maxRow := math.Inf(1), math.Inf(-1)
	for _, object := range req.Objects {
		if object.Building.ItemID != 2302 {
			continue
		}
		if object.First.Latitude < south || object.First.Latitude > north {
			t.Fatalf("制造借用了区域外的水面：%+v", object.First)
		}
		row := object.First.Latitude / float64(latitudeStep)
		minRow, maxRow = min(minRow, row), max(maxRow, row)
		modules[object.Module]++
	}
	if len(modules) != 2 || modules["制造模块 1"] != 32 || modules["制造模块 2"] != 32 {
		t.Fatalf("制造模块被丢失、缩小或重复：%v", modules)
	}
	for _, want := range input.Plan.Ports {
		rate := 0.0
		for _, port := range actual.Ports {
			if port.ItemID == want.ItemID && port.Kind == want.Kind && port.PeerPlanetID == want.PeerPlanetID {
				rate += port.RatePerMinute
			}
		}
		if math.Abs(rate-want.RatePerMinute) > quant.FlowTolerance(rate, want.RatePerMinute) {
			t.Fatalf("分块落位改变物料合同：期望 %+v，实际 %g", want, rate)
		}
	}
	built, err := Build(ctx, req)
	if err != nil {
		t.Fatalf("南纬模块不能形成合法整星蓝图：%v", err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2302] != 64 {
		t.Fatalf("严格解码或64台机器计数错误：%v，%+v", err, doc.Summary.ItemCounts)
	}
	t.Logf("两个32机模块成功装入南第二纬带，机器纬行 %.2f..%.2f，实体 %d；实际生成 %d 次，各次装配候选数 %v", minRow, maxRow, built.BuildingCount, len(generationAttempts), generationAttempts)
}

// TestManufacturingRegionalTemplateProjectsAcrossBands 职责：核对邻带扩展结果能转为整星实体；使用：同32机喷涂模块的高纬投影回归；边界：通过生成器后仍须通过 ProjectDocument 与 Build 的原生几何检查。
func TestManufacturingRegionalTemplateProjectsAcrossBands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = search.WithControl(ctx, search.New(time.Minute, nil))
	// 真实2000宇宙矩阵方案中的32机铁块切片产量，尾数保留低于满负荷的实际合同。
	base := moduleRegionTestPlan(t, ctx, 2394.728541090909)
	needPower := true
	request := build.Request{Plan: base, Mode: 4, CandidateConcurrency: 2,
		Setting:       build.Setting{NeedPower: &needPower, BeltLv: 3, InserterLv: 3},
		SurfaceLayout: &build.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring3-north-center", ExpandAdjacent: true}}
	started := time.Now()
	generated, err := build.Generate(ctx, request)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if generated.SurfacePlan == nil || len(generated.SurfacePlan.Bands) < 2 || generated.SurfacePlan.AnchorLatitudeGrid < 100 {
		t.Fatalf("高纬候选没有使用相邻区域或被挪回赤道：%+v", generated.SurfacePlan)
	}
	if generated.LogisticsSummary == nil {
		t.Fatal("生成器缺少完整边界核验")
	}
	for _, kind := range []string{"external-input", "target-output"} {
		want := map[int]float64{}
		if kind == "external-input" {
			for _, raw := range base.RawInputs {
				want[raw.ItemID] += raw.RatePerMinute
			}
		} else {
			for _, target := range base.Targets {
				want[target.ItemID] += target.RatePerMinute
			}
		}
		for _, port := range generated.LogisticsSummary.ExternalPorts {
			if port.Kind == kind {
				want[port.ItemID] -= port.RatePerMinute
			}
		}
		for item, remaining := range want {
			if math.Abs(remaining) > quant.FlowTolerance(remaining, 0) {
				t.Fatalf("%s 物品%d边界流量未守恒：剩余%g", kind, item, remaining)
			}
		}
	}
	doc, err := blueprint.Decode(generated.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2302] != 32 {
		t.Fatalf("完整生成结果不能严格解码或机器数错误：%v，%+v", err, doc.Summary.ItemCounts)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	anchor := Position{Latitude: float64(float32(generated.SurfacePlan.AnchorLatitudeGrid) * float32(latitudeStep))}
	objects, err := ProjectDocument(ctx, doc, anchor, "高纬制造")
	if err != nil {
		t.Fatal(err)
	}
	built, err := Build(ctx, Request{PlanetID: 101, Objects: objects})
	if err != nil {
		t.Fatalf("生成器高纬结果不能装配成整星蓝图：%v", err)
	}
	decoded, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || decoded.Summary.ItemCounts[2302] != 32 {
		t.Fatalf("高纬整星蓝图丢失机器或无法严格解码：%v，%+v", err, decoded.Summary.ItemCounts)
	}
	t.Logf("完整生成耗时 %s；32机锚点 %d、跨度 %dx%d、原生区域 %d；整星投影、边界流量与严格解码通过", elapsed, generated.SurfacePlan.AnchorLatitudeGrid, generated.SurfacePlan.LayoutWidth, generated.SurfacePlan.LayoutHeight, len(generated.SurfacePlan.Bands))
}

// moduleRegionTestPlan 职责：创建整数32机为单位的喷涂铁块生产；使用：南纬及跨带实体回归；边界：显式外供增产剂，不引入与选址无关的制造链。
func moduleRegionTestPlan(t *testing.T, ctx context.Context, rate float64) quant.Result {
	t.Helper()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	request := quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: rate}},
		Settings: quant.Settings{MineralizeList: map[string]bool{dataset.ByID[1143].Name: true}},
		Scheme:   quant.Scheme{SchemeForRecipe: make([]quant.RecipeScheme, len(dataset.Recipes))}}
	for index, recipe := range dataset.Recipes {
		if recipe.ID == 1 {
			request.Scheme.SchemeForRecipe[index] = quant.RecipeScheme{ProductionMode: 2, IncreasePoints: 4}
		}
	}
	base, err := quant.Solve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	base, err = industryplan.SplitProductions(ctx, base, 32)
	if err != nil {
		t.Fatal(err)
	}
	return base
}
