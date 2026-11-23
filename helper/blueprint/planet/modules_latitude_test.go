package planet

// 文件职责：验证制造模板在同一原生纬带内利用非默认纬行；使用：真实模板、地形与编解码回归；边界：不将单 Area 的平移能力推广到多 Area。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// TestManufacturingUsesShiftedLatitudeOnNarrowLand 职责：在默认纬行全为水面的窄陆带装入真实制造模板；使用：覆盖平移、原生边界、拒绝条件与取消；边界：只生成一次模板，负例直接检查同一有限模板的全部落点。
func TestManufacturingUsesShiftedLatitudeOnNarrowLand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = search.WithControl(ctx, search.New(2*time.Minute, nil))
	generations := 0
	ctx = build.WithWorkflowFinish(ctx, func(_ *workflow.RunResult, _ error) { generations++ })
	ctx = WithTemplateCache(ctx)
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	base := moduleRegionTestPlan(t, ctx, 300)
	if len(base.Productions) != 1 || base.MachineCount != 4 {
		t.Fatalf("回归样例应为4台真实熔炉：%+v", base.Productions)
	}
	needPower := true
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Plan: base},
		Options: ProductionOptions{Setting: build.Setting{NeedPower: &needPower, BeltLv: 3, InserterLv: 3}}}
	for _, raw := range base.RawInputs {
		input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: raw.ItemID, Kind: "external", RatePerMinute: raw.RatePerMinute})
	}
	input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: 1101, Kind: "export", PeerPlanetID: 102, RatePerMinute: 300})
	baseline := Request{PlanetID: 101}
	if err := appendManufacturing(ctx, &baseline, input, &industryplan.PlanetActual{}, catalog); err != nil {
		t.Fatal(err)
	}
	var template moduleTemplate
	cache := ctx.Value(templateCacheKey{}).(interface{ Range(func(any, any) bool) })
	cache.Range(func(_, value any) bool {
		template = value.(moduleTemplate)
		return false
	})
	if generations != 1 || len(template.document.Areas) != 1 || template.generated.SurfacePlan == nil {
		t.Fatalf("回归须使用一份真实单 Area 赤道模板：生成%d次，区域%d", generations, len(template.document.Areas))
	}
	document := template.document
	anchor := template.generated.SurfacePlan.AnchorLatitudeGrid
	var region build.DensityLatitudeOption
	for _, option := range build.DensityLatitudeOptions() {
		if option.Value == "equator" {
			region = option
		}
	}
	before, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	dryStrip := func(p Position) (bool, bool) {
		row := p.Latitude / float64(latitudeStep)
		return row < 35 || row > 65, true
	}
	if !terrainConflict(baseline.Objects, dryStrip, catalog) {
		t.Fatal("默认锚点意外位于窄陆带内，无法证明新增纬行有效")
	}

	t.Run("narrow_land", func(t *testing.T) {
		shiftedInput := input
		shiftedInput.Plan.Ports = append([]industryplan.Port(nil), input.Plan.Ports...)
		for i := range shiftedInput.Plan.Ports {
			if shiftedInput.Plan.Ports[i].ItemID == 1001 {
				shiftedInput.Plan.Ports[i].Kind, shiftedInput.Plan.Ports[i].PeerPlanetID = "import", 103
			}
		}
		request := Request{PlanetID: 101, wet: dryStrip}
		actual := industryplan.PlanetActual{}
		if err := appendManufacturing(ctx, &request, shiftedInput, &actual, catalog); err != nil {
			t.Fatalf("同纬带存在完整干地却拒绝平移：%v", err)
		}
		if generations != 1 {
			t.Fatalf("同纬带平移不应再次生成模板：%d", generations)
		}
		if terrainConflict(request.Objects, dryStrip, catalog) {
			t.Fatal("平移候选占用了水面")
		}
		for _, want := range shiftedInput.Plan.Ports {
			rate := 0.0
			for _, port := range actual.Ports {
				if port.ItemID == want.ItemID && port.Kind == want.Kind && port.PeerPlanetID == want.PeerPlanetID {
					rate += port.RatePerMinute
				}
			}
			if math.Abs(rate-want.RatePerMinute) > quant.FlowTolerance(rate, want.RatePerMinute) {
				t.Fatalf("纬行平移改变边界合同：期望%+v，实际%g", want, rate)
			}
		}
		built, err := Build(ctx, request)
		if err != nil {
			t.Fatalf("平移后不能装配整星蓝图：%v", err)
		}
		decoded, err := blueprint.Decode(built.Blueprint, true)
		if err != nil || decoded.Summary.ItemCounts[2302] != 4 || len(decoded.Areas) != 1 {
			t.Fatalf("平移后机器数或严格解码不符：%v，%v，区域%d", err, decoded.Summary.ItemCounts, len(decoded.Areas))
		}
		row := request.Objects[0].First.Latitude/float64(latitudeStep) - float64(document.Buildings[0].LocalOffset.Y)
		if math.Abs(row-float64(anchor)) < .01 {
			t.Fatal("落位仍使用水面中的默认锚点")
		}
		t.Logf("模板原锚点%d，实际锚点%.2f；35..65纬行窄陆带成功放置4炉、%d实体，边界与严格编解码通过", anchor, row, built.BuildingCount)
	})

	t.Run("native_bounds", func(t *testing.T) {
		rows := moduleLatitudeRows(document, anchor, region, catalog)
		if len(rows) < 2 || rows[0] != anchor {
			t.Fatalf("没有保留原锚点优先及新增纬行：%v", rows)
		}
		for _, row := range rows[1:] {
			for _, building := range document.Buildings {
				margin := math.Asin(groundRadius(catalog, building) / 200)
				for _, point := range []blueprint.Point{building.LocalOffset, building.LocalOffset2} {
					latitude := float64(float32(row)*float32(latitudeStep) + point.Y*float32(latitudeStep))
					for _, edge := range []float64{latitude - margin, latitude + margin} {
						step, err := geometry.PasteLongitudeStep32(geometry.Radians(edge), geometry.Vanilla200)
						if err != nil || math.Abs(float64(step)*float64(document.Areas[0].AreaSegments)*5-2*math.Pi) > 1e-5 {
							t.Fatalf("纬行%d的完整占地跨越原生经度分段：%v", row, err)
						}
						if degrees := edge * 180 / math.Pi; degrees < region.LatitudeLow || degrees > region.LatitudeHigh {
							t.Fatalf("纬行%d的完整占地越过选区：%g度", row, degrees)
						}
					}
				}
			}
		}
		multi := document
		multi.Areas = append(append([]blueprint.Area(nil), document.Areas...), blueprint.Area{Index: 1, AreaSegments: 160})
		if rows := moduleLatitudeRows(multi, anchor, region, catalog); len(rows) != 1 || rows[0] != anchor {
			t.Fatalf("多 Area 模板被纬向平移：%v", rows)
		}
		long := document
		long.Buildings = append([]blueprint.Building(nil), document.Buildings...)
		long.Buildings[0].LocalOffset2.Y = 200
		if rows := moduleLatitudeRows(long, anchor, region, catalog); len(rows) != 1 || rows[0] != anchor {
			t.Fatalf("只检查了第一端点，跨带第二端点仍获平移许可：%v", rows)
		}
	})

	for _, name := range []string{"water", "occupied"} {
		t.Run(name, func(t *testing.T) {
			request := Request{PlanetID: 101, wet: func(Position) (bool, bool) { return true, true }}
			if name == "occupied" {
				request.wet = dryStrip
				// 十二个真实球面占位覆盖窄陆带整圈，其余地表仍为水面。
				for i := range 12 {
					request.Obstacles = append(request.Obstacles, Obstacle{Position: Position{Latitude: 50 * float64(latitudeStep), Longitude: float64(i) * math.Pi / 6}, Radius: 70})
				}
			}
			cursor := &moduleSiteCursor{}
			index := newObstacleIndex(request.Obstacles)
			objects, err := placeManufacturingTemplate(ctx, request, document, anchor, region, "不能放置的模块", cursor, index, catalog)
			if err != nil || len(objects) != 0 {
				t.Fatalf("完整模板扫描应拒绝%q，不得借平移绕过条件：对象%d，错误%v", name, len(objects), err)
			}
			request.wet = func(Position) (bool, bool) {
				t.Fatal("同次装配重复扫描了已经耗尽的同规格落点")
				return false, true
			}
			objects, err = placeManufacturingTemplate(ctx, request, document, anchor, region, "同一模板的下一块", cursor, index, catalog)
			if err != nil || len(objects) != 0 {
				t.Fatalf("已耗尽的选区发生变化：对象%d，错误%v", len(objects), err)
			}
		})
	}
	t.Run("cancel_during_scan", func(t *testing.T) {
		child, stop := context.WithCancel(ctx)
		defer stop()
		request := Request{PlanetID: 101, wet: func(Position) (bool, bool) { stop(); return true, true }}
		actual := industryplan.PlanetActual{}
		err := appendManufacturing(child, &request, input, &actual, catalog)
		if !errors.Is(err, context.Canceled) || len(request.Objects) != 0 || len(actual.Ports) != 0 {
			t.Fatalf("扫描中取消未直接传回：对象%d，端口%d，错误%v", len(request.Objects), len(actual.Ports), err)
		}
	})
	after, err := json.Marshal(document)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("纬行探测、局部核验或端口配置改写了缓存模板：%v", err)
	}
}
