package planet

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/saveanalysis"
)

func TestDirectAlternativeOnlyAfterExhaustedPrimary(t *testing.T) {
	for _, sample := range []struct {
		name                                             string
		primaryFits, exhausted, canceled, useAlternative bool
	}{
		{"首选落位", true, false, false, false},
		{"时间窗结束", false, false, false, false},
		{"父任务取消", false, true, true, false},
		{"首选穷尽且备用可放", false, true, false, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if sample.canceled {
				cancel()
			}
			loads, placements := 0, 0
			primary := moduleTemplate{generated: build.Result{SurfacePlan: &build.SurfacePlan{AnchorLatitudeGrid: -24}}}
			primary.alternative = func(context.Context) moduleTemplate {
				loads++
				return moduleTemplate{generated: build.Result{SurfacePlan: &build.SurfacePlan{AnchorLatitudeGrid: -30}}}
			}
			objects, selected := placeDirectShapes(ctx, primary, func(shape moduleTemplate, alternate bool) ([]Object, bool) {
				if alternate {
					placements++
					return []Object{{}}, true
				}
				if sample.primaryFits {
					return []Object{{}}, false
				}
				return nil, sample.exhausted
			})
			want := 0
			if sample.useAlternative {
				want = 1
			}
			if loads != want || placements != want || (len(objects) > 0) != (sample.primaryFits || sample.useAlternative) || (selected.generated.SurfacePlan.AnchorLatitudeGrid == -30) != sample.useAlternative {
				t.Fatalf("备用取数 %d 次、试放 %d 次，首选/备用结果锚纬 %d，实际落位 %d", loads, placements, selected.generated.SurfacePlan.AnchorLatitudeGrid, len(objects))
			}
		})
	}
}

func TestDirectSitePrefixMatchesGeometryAndCertifiedCell(t *testing.T) {
	cache := &sync.Map{}
	doc := blueprint.Document{Buildings: []blueprint.Building{{ItemID: 2104, ModelIndex: 50}}}
	template := moduleTemplate{document: doc}
	resources := saveanalysis.PlanetResourceDetail{SHA256: "save-a", Tick: 42}
	req := Request{PlanetID: 501, Objects: []Object{{Building: doc.Buildings[0]}}, minerals: []Obstacle{{Radius: .4}}}
	var cursor moduleSiteCursor
	record := resumeModuleSite(cache, req, resources, template, 0, &cursor)
	cursor.row, cursor.x = 3, 25
	cursor.placedAt = Position{Longitude: .2, Latitude: .1}
	record(true)
	check := func(request Request, detail saveanalysis.PlanetResourceDetail, shape moduleTemplate, row, x int) {
		t.Helper()
		var next moduleSiteCursor
		resumeModuleSite(cache, request, detail, shape, 0, &next)
		if next.row != row || next.x != x {
			t.Fatalf("续查起点为 %d/%d，预期 %d/%d", next.row, next.x, row, x)
		}
		if next.certified != (row == 3) || next.certified && next.placedAt != cursor.placedAt {
			t.Fatalf("坐标或占地变更后的复用状态错误：%+v", next)
		}
	}
	req.Objects[0].Building.Parameters = []int32{6001, 0, 2}
	check(req, resources, template, 3, 20)
	req.Objects[0].Second.Latitude = .1
	check(req, resources, template, 0, 0)
	req.Objects[0].Second.Latitude = 0
	req.minerals[0].Radius = .5
	check(req, resources, template, 0, 0)
	req.minerals[0].Radius = .4
	other := resources
	other.Tick++
	check(req, other, template, 0, 0)
	trial := template
	trial.document.Buildings = slices.Clone(doc.Buildings)
	check(req, resources, trial, 0, 0)
	req.PlanetID++
	check(req, resources, template, 0, 0)
}

// TestRepeatedModuleSiteCursor 复核同一模板的后续工区。搜索起点或块号变化时，先前一段尚未逐格核验。
func TestRepeatedModuleSiteCursor(t *testing.T) {
	cache := &sync.Map{}
	template := moduleTemplate{document: blueprint.Document{Buildings: []blueprint.Building{{ItemID: 2104}}}}
	detail := saveanalysis.PlanetResourceDetail{SHA256: "snapshot", Tick: 42}
	req := Request{PlanetID: 1504}
	first := moduleSiteCursor{row: 3, x: 15}
	remember := resumeModuleSite(cache, req, detail, template, 4, &first)
	last := errors.New("末行带段交叉")
	first.row, first.x, first.last, first.exhausted = 101, 0, last, true
	remember(false)
	for _, sample := range []struct{ block, row, x, want int }{{4, 3, 15, 101}, {3, 3, 15, 3}, {4, 3, 10, 3}} {
		next := moduleSiteCursor{row: sample.row, x: sample.x}
		resumeModuleSite(cache, req, detail, template, sample.block, &next)
		if next.row != sample.want || (next.last == last) != (sample.want == 101) {
			t.Fatalf("工区和起点 %+v 得到行 %d、原因 %v", sample, next.row, next.last)
		}
	}
}

func TestDirectSiteInterruptionKeepsUndecidedCell(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	doc := blueprint.Document{Areas: []blueprint.Area{{AreaSegments: 200, ParentIndex: -1}},
		Buildings: []blueprint.Building{{ItemID: 2001, ModelIndex: 35, InputObjIdx: -1, OutputObjIdx: -1}}}
	region := build.DensityLatitudeOption{Value: "equator", LatitudeLow: -30, LatitudeHigh: 30}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := Request{wet: func(Position) (bool, bool) {
		cancel()
		return false, true
	}}
	var cursor moduleSiteCursor
	occupied := newObstacleIndex(nil)
	objects, err := placeManufacturingTemplate(ctx, req, doc, 0, region, "候选", &cursor, occupied, catalog)
	if !errors.Is(err, context.Canceled) || len(objects) != 0 || cursor.exhausted || cursor.row != 0 || cursor.x != 0 {
		t.Fatalf("中断越过仍可能成立的位置：%d/%d，%v", cursor.row, cursor.x, err)
	}
	req.wet = nil
	objects, err = placeManufacturingTemplate(context.Background(), req, doc, 0, region, "候选", &cursor, occupied, catalog)
	if err != nil || len(objects) != 1 || objects[0].First.Longitude != 0 || cursor.x != moduleLongitudeStride {
		t.Fatalf("未重新核验上轮中断的首格：%+v，%v", cursor, err)
	}
}

// TestChangedOccupancyRechecksHint 旧落点被占则从原搜索起点选址，空闲旧落点通过重验后可以优先放置。
func TestChangedOccupancyRechecksHint(t *testing.T) {
	cat, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	doc := blueprint.Document{Areas: []blueprint.Area{{AreaSegments: 200, ParentIndex: -1}}, Buildings: []blueprint.Building{{ItemID: 2001, ModelIndex: 35, InputObjIdx: -1, OutputObjIdx: -1}}}
	region := build.DensityLatitudeOption{Value: "equator", LatitudeLow: -30, LatitudeHigh: 30}
	hint := Position{Longitude: .3}
	for _, blocked := range []bool{false, true} {
		var obs []Obstacle
		if blocked {
			obs = []Obstacle{{Position: hint, Radius: 3}}
		}
		cur := moduleSiteCursor{preferred: &hint}
		objs, err := placeManufacturingTemplate(t.Context(), Request{}, doc, 0, region, "试放", &cur, newObstacleIndex(obs), cat)
		if err != nil || len(objs) != 1 || cur.outOfOrder == blocked || blocked && cur.placedAt.Longitude != 0 || !blocked && cur.placedAt != hint {
			t.Fatalf("旧落点受阻=%t 坐标=%+v 错误=%v", blocked, cur, err)
		}
	}
}
