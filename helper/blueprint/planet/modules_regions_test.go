package planet

// 文件职责：验证制造分块使用生成器真实选区而非稀疏固定纬行；使用：空地利用率回归；边界：合成干地证明漏试区域可用，不代表任意存档整星均已搜尽。

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
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
	t.Run("完整选区", func(t *testing.T) { verifySouthernBand(t, false, false) })
	t.Run("可在已用星之间分摊", func(t *testing.T) { verifySouthernBand(t, true, false) })
	t.Run("原星中心选区复核", func(t *testing.T) { verifySouthernBand(t, false, true) })
}

func verifySouthernBand(t *testing.T, canRelocate, centersOnly bool) {
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
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Plan: base, CanRelocate: canRelocate, CenterRegionsOnly: centersOnly},
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
	// 三档都可把同一赤道原图转到南侧，两个模块在不同经度复用这份图。
	if len(generationAttempts) != 1 {
		t.Fatalf("南半球已有可施工形状却重新生成了模板，各次装配候选数 %v", generationAttempts)
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
	// 将跨多个原生 Area 的北纬原图转到南面，核对额定货口仍指向同一实体及新的局部坐标。
	shape := rotateModuleTemplate(ctx, moduleTemplate{generated: generated, document: doc}, request.SurfaceLayout.LatitudeZone, build.DensityLatitudeOption{Value: "ring3-south-center"})
	if shape.generated.SurfacePlan == nil || shape.generated.SurfacePlan.AnchorLatitudeGrid >= 0 || shape.document.Summary.ItemCounts[2302] != 32 {
		t.Fatal("半周旋转丢失了高纬熔炉或南侧锚点")
	}
	ports := shape.generated.LogisticsSummary.ExternalPorts
	if len(ports) != len(generated.LogisticsSummary.ExternalPorts) {
		t.Fatal("旋转改变了外部货口数量")
	}
	for i, port := range ports {
		before := generated.LogisticsSummary.ExternalPorts[i]
		end := shape.document.Buildings[port.BuildingIndex]
		if before.BuildingIndex != port.BuildingIndex || before.ItemID != port.ItemID || before.RatePerMinute != port.RatePerMinute || before.Kind != port.Kind ||
			port.AreaIndex != int(end.AreaIndex) || port.X != float64(end.LocalOffset.X) || port.Y != float64(end.LocalOffset.Y) || port.Z != float64(end.LocalOffset.Z) {
			t.Fatalf("南侧货口与实图不符：原 %+v，旋转后 %+v", before, port)
		}
	}
	t.Logf("完整生成耗时 %s；32机锚点 %d、跨度 %dx%d、原生区域 %d；整星投影、边界流量与严格解码通过", elapsed, generated.SurfacePlan.AnchorLatitudeGrid, generated.SurfacePlan.LayoutWidth, generated.SurfacePlan.LayoutHeight, len(generated.SurfacePlan.Bands))
}

// TestManufacturingEquatorTurnsToBothMidlatitudes 用同一赤道工区在两半球的第二、第四纬区核对纬度、货口和实图引用。
func TestManufacturingEquatorTurnsToBothMidlatitudes(t *testing.T) {
	ctx := t.Context()
	plan := moduleRegionTestPlan(t, ctx, 240)
	power := true
	regions := build.DensityLatitudeOptions()
	options := ProductionOptions{firstVerifiedLayout: true, Setting: build.Setting{NeedPower: &power, BeltLv: 3, InserterLv: 3}}
	source, err := generateIndependentTemplate(ctx, options, plan, 2, regions[0], false)
	if err != nil {
		t.Fatal(err)
	}
	cache := &sync.Map{}
	cache.Store("relocate/equator/iron", source)
	step, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	for _, target := range regions {
		if (target.Tier != 2 && target.Tier != 4) || target.Position != "center" {
			continue
		}
		shape, key := cachedRegionRotation(ctx, cache, "equator", target, "iron")
		if key == "" || len(shape.document.Buildings) != len(source.document.Buildings) {
			t.Fatalf("%s 缺少完整实体", target.Value)
		}
		anchor := Position{Latitude: float64(float32(shape.generated.SurfacePlan.AnchorLatitudeGrid) * float32(step))}
		objects, err := ProjectDocument(ctx, shape.document, anchor, "")
		if err != nil {
			t.Fatal(err)
		}
		var sum float64
		for _, obj := range objects {
			sum += obj.First.Latitude
		}
		mean := sum / float64(len(objects)) * 180 / math.Pi
		if mean < target.LatitudeLow || mean > target.LatitudeHigh {
			t.Fatalf("%s 的建筑平均纬度为 %.3f°", target.Value, mean)
		}
		ports := shape.generated.LogisticsSummary.ExternalPorts
		if len(ports) == 0 || len(ports) != len(source.generated.LogisticsSummary.ExternalPorts) {
			t.Fatal("与赤道原工区的外部货口数不符")
		}
		for i, p := range ports {
			old := source.generated.LogisticsSummary.ExternalPorts[i]
			b := shape.document.Buildings[p.BuildingIndex]
			if p.BuildingIndex != old.BuildingIndex || p.ItemID != old.ItemID || p.RatePerMinute != old.RatePerMinute || p.AreaIndex != int(b.AreaIndex) || p.X != float64(b.LocalOffset.X) || p.Y != float64(b.LocalOffset.Y) {
				t.Fatalf("%s 的货口 %d 与实物不符", target.Value, i)
			}
		}
	}
}

// TestTimedTemplateSurvivesPause 在第四纬区的真实工区生成中核对原栈继续、主动取消和实际计算耗尽。
func TestTimedTemplateSurvivesPause(t *testing.T) {
	plan := moduleRegionTestPlan(t, t.Context(), 240)
	opts := ProductionOptions{firstVerifiedLayout: true, Setting: build.Setting{BeltLv: 3, InserterLv: 3}}
	var region build.DensityLatitudeOption
	for _, r := range build.DensityLatitudeOptions() {
		if r.Value == "ring4-north-center" {
			region = r
			break
		}
	}
	if region.Value == "" {
		t.Fatal("缺少北第四纬区")
	}
	t.Run("有效计算耗尽", func(t *testing.T) {
		ctx := search.WithControl(t.Context(), search.New(time.Hour, nil))
		ctx = build.WithWorkflowStart(ctx, func(workflow.PlanDescription) error {
			// 可暂停的父任务一直运行。让本次模板在工作流登记阶段用完自己的计算预算。
			select {
			case <-time.After(initialTemplateWindow + 100*time.Millisecond):
				return nil
			case <-t.Context().Done():
				return t.Context().Err()
			}
		})
		_, err := generateIndependentTemplate(ctx, opts, plan, 2, region, true)
		var issue *blueprint.Error
		if ctx.Err() != nil || !errors.As(err, &issue) || issue.Code != "GEN_E_TIMEOUT" {
			t.Fatalf("试摆时窗耗尽后模板返回 %v，父任务返回 %v", err, ctx.Err())
		}
	})
	for _, action := range []string{"继续", "取消"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*initialTemplateWindow)
			defer cancel()
			paused := make(chan struct{})
			var first atomic.Bool
			var control *search.Control
			control = search.New(time.Millisecond, func() {
				if first.CompareAndSwap(false, true) {
					close(paused)
				} else {
					control.Continue()
				}
			})
			done := make(chan error, 1)
			go func() {
				template, err := generateIndependentTemplate(search.WithControl(ctx, control), opts, plan, 2, region, true)
				if err == nil && len(template.document.Buildings) == 0 {
					err = errors.New("恢复后返回了空模板")
				}
				done <- err
			}()
			select {
			case <-paused:
			case err := <-done:
				t.Fatalf("模板未进入暂停：%v", err)
			case <-ctx.Done():
				t.Fatal("未收到暂停通知")
			}
			if action == "取消" {
				cancel()
			} else {
				select {
				case err := <-done:
					t.Fatalf("等待继续期间模板已退出：%v", err)
				case <-time.After(initialTemplateWindow + 100*time.Millisecond):
				}
				if !control.Continue() {
					t.Fatal("未能恢复模板")
				}
			}
			select {
			case err := <-done:
				var problem *blueprint.Error
				if action == "继续" && err != nil || action == "取消" && (!errors.As(err, &problem) || problem.Code != "GEN_E_CANCELED") {
					t.Fatalf("%s后模板返回 %v", action, err)
				}
			case <-time.After(initialTemplateWindow):
				t.Fatal("模板未响应" + action)
			}
		})
	}
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
