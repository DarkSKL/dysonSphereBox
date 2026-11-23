package planet

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// TestMinerBodyAndFeetClearOre 在赤道按原型坐标区分压矿的机身、支脚和可采的空隙。
func TestMinerBodyAndFeetClearOre(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := catalog.Shapes(2301, 57)
	if err != nil {
		t.Fatal(err)
	}
	site := Position{}
	pt, err := world(site)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		right, front float64
		kind         int64
		overlap      bool
	}{
		{"煤矿压住机身", 0, -2, 6, true},
		{"煤矿压住支脚", 2, -3.5, 6, true},
		{"煤矿在扇区内远离支脚", 0, -4, 6, false},
		{"相同位置的大油井碰到机身", 0, -4, 7, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lift, radius := veinCollider(tc.kind)
			base := placement.Vector{float64(pt.X) + tc.right, float64(pt.Y) + tc.front, float64(pt.Z)}
			points := []veinPoint{{kind: tc.kind, v: base, colliderCenter: base.Add(placement.Vector{0, 0, -lift}), colliderRadius: radius}}
			got := minerOverlapsVein(site, pt, 0, shapes, points, indexVeinNeighborhoods(points, minerScanRadius), 8)
			if got != tc.overlap {
				t.Fatalf("矿物与原版建造盒相交=%t，期望%t", got, tc.overlap)
			}
		})
	}
}

// TestMinerOnReformedOre 复现 2401 星的右支脚，核对原地形与铺地后矿石实际高度。
func TestMinerOnReformedOre(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := catalog.Shapes(2301, 57)
	if err != nil {
		t.Fatal(err)
	}
	site := Position{}
	center, err := world(site)
	if err != nil {
		t.Fatal(err)
	}
	local := func(right, up, front float64) placement.Vector {
		return placement.Vector{float64(center.X) + right, float64(center.Y) + front, float64(center.Z) - up}
	}
	saved, paved := local(1.142, 2.564, -3.395), local(1.129, .239, -3.356)
	points := []veinPoint{{kind: 6, v: saved, colliderCenter: saved, colliderRadius: .5}}
	nearby := indexVeinNeighborhoods(points, minerScanRadius)
	hit := func() bool { return minerOverlapsVein(site, center, 0, shapes, points, nearby, 8) }
	if hit() {
		t.Fatal("原始高度的固矿在右支脚上方")
	}
	points[0].pavedCenter = paved
	if !hit() {
		t.Fatal("铺地后的固矿侵入右支脚")
	}
	points[0].pavedCenter = local(0, .239, -4)
	if hit() {
		t.Fatal("两支脚之间的矿石仍有净空")
	}
}

// TestOrdinaryMinerSharedFallback 核对缺口才共享、原有矿机隔离、四个登记槽及任务内回放。
func TestOrdinaryMinerSharedFallback(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := miningPartialInput(180)
	input.Resources.SHA256 = "共采回放"
	run := func() (Request, []MiningCoverage, industryplan.PlanetActual, error) {
		req := Request{PlanetID: input.Plan.PlanetID, TerrainKnown: true}
		var mines []MiningCoverage
		var actual industryplan.PlanetActual
		err := appendMining(ctx, &req, input, &actual, &mines, catalog)
		return req, mines, actual, err
	}
	req, rows, actual, err := run()
	if err != nil || len(rows) != 2 || len(actual.Warnings) != 1 || !strings.Contains(actual.Warnings[0], "1 个矿点") {
		t.Fatalf("单矿点需两台新矿机合供180：%v 设施%d %v", err, len(rows), actual.Warnings)
	}
	var unique int64
	for _, m := range rows {
		if !reflect.DeepEqual(m.VeinIDs, []int64{1}) || m.CoveredItems == nil || *m.CoveredItems != 5000 || m.RatePerMinute != 90 {
			t.Fatal("每台须保留自己真实覆盖的矿点和余量", m)
		}
		unique += m.NewCoveredItems
	}
	if unique != 5000 {
		t.Fatal("共享矿点在整星总量重复或遗漏", unique)
	}
	_, warm, next, err := run()
	if err != nil || !reflect.DeepEqual(rows, warm) || !reflect.DeepEqual(actual, next) {
		t.Fatal("重放需恢复原共享覆盖、唯一储量及提示", err)
	}
	point := []veinPoint{{id: 1, item: 1001, kind: 1, v: placement.Vector{0, 0, -200}, newMiners: 3}}
	miner := req.Objects[rows[0].BuildingIndex]
	pt, err := world(miner.First)
	if err != nil {
		t.Fatal(err)
	}
	covers := func(owners uint8) int {
		return len(minerCoverage(miner.First, pt, float64(miner.Building.Yaw), false, 1001, point, indexVeinNeighborhoods(point, minerScanRadius), owners))
	}
	if covers(1) != 0 || covers(industryplan.MaxOrdinaryMinersPerVein) != 1 {
		t.Fatal("只允许共享后备登记第四台")
	}
	point[0].newMiners = 4
	if covers(industryplan.MaxOrdinaryMinersPerVein) != 0 {
		t.Fatal("第五台在原版不能登记")
	}
	input.Plan.Mines[0].RatePerMinute = 90
	_, single, act, err := run()
	if err != nil || len(single) != 1 || len(act.Warnings) != 0 || math.Abs(single[0].RatePerMinute-90) > 1e-8 {
		t.Fatal("独占已满足时不应共采", err, single, act.Warnings)
	}
	input.Resources.Veins[0].MinerEntityIDs = []int64{123}
	_, blocked, _, err := run()
	if err == nil || len(blocked) != 0 {
		t.Fatal("严禁新矿机使用玩家既有矿机的矿点")
	}
}
