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

// TestMiningTowerDistanceReplay 职责：验证同一星球两套塔距能独立复用；使用：岸边重排回归；边界：同一矿点与合同仅改变接驳策略。
func TestMiningTowerDistanceReplay(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	input := miningPartialInput(50)
	input.Resources.SHA256 = "同一采集快照"
	req := Request{PlanetID: input.Plan.PlanetID}
	points := []veinPoint{{id: 1, item: 1001}}
	choice := miningChoice{item: 1001, facility: 2301, remaining: 50}
	for _, near := range []bool{false, true} {
		input.Options.nearMineTowers = near
		replay := beginMiningReplay(ctx, &req, input, points)
		if _, _, _, found := replay.restore(choice); found {
			t.Fatal("尚未按本塔距施工，已取到另一套矿机")
		}
		distance := 15.
		if near {
			distance = 8
		}
		replay.remember(choice, []Object{{First: Position{Longitude: distance / 200}}}, []int{0}, 50)
		replay.finish()
	}
	for _, near := range []bool{true, false} {
		input.Options.nearMineTowers = near
		objects, _, _, ok := beginMiningReplay(ctx, &req, input, points).restore(choice)
		if !ok || len(objects) != 1 || (objects[0].First.Longitude == 8./200) != near {
			t.Fatalf("塔距回执串用，短带 %t，实物 %+v", near, objects)
		}
	}
}

// TestMiningRetryDiscardsPartialExports 职责：检查后续矿种失败后的整段回滚；使用：采集段重排回归；边界：两种塔距均无第二矿种。
func TestMiningRetryDiscardsPartialExports(t *testing.T) {
	ctx := WithTemplateCache(t.Context())
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := miningPartialInput(50)
	input.Resources.SHA256 = "缺少第二矿种"
	input.Plan.Ports = []industryplan.Port{{Kind: "export", ItemID: 1001, PeerPlanetID: 102, RatePerMinute: 50}}
	input.Plan.Mines = append(input.Plan.Mines, industryplan.Mine{ItemID: 1002, FacilityID: 2301, RatePerMinute: 50})
	req := Request{PlanetID: input.Plan.PlanetID, wet: func(Position) (bool, bool) { return false, true }, stationPad: func(Position) bool { return true }}
	for _, near := range []bool{false, true} {
		input.Options.nearMineTowers = near
		trial := req
		var coverage []MiningCoverage
		var actual industryplan.PlanetActual
		failure := appendMining(ctx, &trial, input, &actual, &coverage, catalog)
		issue, ok := failure.(*Error)
		if !ok || issue.Item != 1002 || len(trial.Objects) == 0 || len(trial.exportFeeds) == 0 || len(actual.Ports) != 2 || len(coverage) != 1 {
			t.Fatalf("未复现第一种矿已出货、第二种矿缺失：短带 %t，错误 %v", near, failure)
		}
	}
	input.Options.nearMineTowers = false
	actual, coverage, err := placeMining(ctx, &req, input, catalog)
	if err == nil || len(req.Objects) != 0 || len(req.exportFeeds) != 0 || len(actual.Ports) != 0 || len(coverage) != 0 {
		t.Fatalf("不完整的采集合同留下了实体或出货记录：%v，%d 个建筑，%+v", err, len(req.Objects), actual)
	}
}
