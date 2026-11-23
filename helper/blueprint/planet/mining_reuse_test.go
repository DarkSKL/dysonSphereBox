package planet

import (
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// TestMiningReuseRechecksContracts 比对同矿区两次真实接驳，改变交付对端后须重建回执；开采条件变化后不能沿用旧矿点。
func TestMiningReuseRechecksContracts(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := miningPartialInput(50)
	input.Resources.SHA256 = "固定的矿位样例"
	input.Plan.Ports = []industryplan.Port{{ItemID: 1001, Kind: "export", PeerPlanetID: 102, RatePerMinute: 50}}
	run := func() ([]Object, []MiningCoverage, industryplan.PlanetActual, error) {
		req := Request{PlanetID: input.Plan.PlanetID, TerrainKnown: true}
		var rows []MiningCoverage
		var actual industryplan.PlanetActual
		err := appendMining(ctx, &req, input, &actual, &rows, catalog)
		return req.Objects, rows, actual, err
	}
	objects, coverage, first, err := run()
	if err != nil || len(objects) < 2 || len(coverage) != 1 || len(first.Ports) != 2 {
		t.Fatalf("第一次真实落位失败：%v，设备 %d，矿机 %d，回执 %+v", err, len(objects), len(coverage), first.Ports)
	}
	// 模拟整星后续给已生成的塔写入分组/仓储，缓存副本仍应保持矿区原参数。
	old := objects[1].Building.Parameters[3]
	objects[1].Building.Parameters[3] = 7
	input.Plan.Ports[0].PeerPlanetID = 103
	again, nextCoverage, next, err := run()
	if err != nil || !reflect.DeepEqual(coverage, nextCoverage) || len(again) != len(objects) || again[1].Building.Parameters[3] != old ||
		len(next.Ports) != 2 || next.Ports[1].PeerPlanetID != 103 || next.Ports[1].RatePerMinute != 50 {
		t.Fatalf("矿区复用后连接、覆盖、独立塔参数或实际出口有误：%v，回执 %+v", err, next.Ports)
	}
	input.Resources.Veins[0].MinerEntityIDs = []int64{5}
	if _, _, _, err = run(); err == nil {
		t.Fatal("已经被其他矿机占用的矿点仍在引用旧规划")
	}
}

// TestMiningReuseEndsAtFirstDifference 首个几何或额定需求差异会使其后的旧选址失效，即使后续矿机规格相同。
func TestMiningReuseEndsAtFirstDifference(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	input := miningPartialInput(90)
	input.Resources.SHA256 = "同一存档"
	req := Request{PlanetID: input.Plan.PlanetID}
	point := []veinPoint{{id: 3, item: 1001}}
	write := beginMiningReplay(ctx, &req, input, point)
	a := miningChoice{item: 1001, facility: 2301, remaining: 90, speed: 3, setting: 1}
	b := miningChoice{item: 1001, facility: 2301, remaining: 40, speed: 3, setting: 1}
	write.remember(a, []Object{{Item: 1001}}, []int{0}, 50)
	write.remember(b, []Object{{Item: 1001}}, []int{1}, 50)
	write.finish()
	read := beginMiningReplay(ctx, &req, input, point)
	changed := a
	changed.remaining = 91
	if _, _, _, ok := read.restore(changed); ok {
		t.Fatal("额定剩余已改变，仍复用了首台矿机")
	}
	if _, _, _, ok := read.restore(b); ok {
		t.Fatal("首个落位分歧后仍复用了后续矿机")
	}
	req.Obstacles = []Obstacle{{Radius: 3}}
	read = beginMiningReplay(ctx, &req, input, point)
	if _, _, _, ok := read.restore(a); ok {
		t.Fatal("初始障碍改变后仍沿用旧矿机")
	}
}
