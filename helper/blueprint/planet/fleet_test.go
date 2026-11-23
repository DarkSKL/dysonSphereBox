package planet

// 文件职责：验证补充船队塔的实体、库存角色和本地转运；使用：矿区船队与多物品分组回归；边界：总站数和物品角色不代表逐航线船队已最优分配。

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestFleetStationsForSingleMine 职责：把一座实际矿区塔补到三座船队塔；使用：单物品大船队回归；边界：矿机带口能力仍只有原回执，新增塔依赖无人机接货。
func TestFleetStationsForSingleMine(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, StationCount: 3, VesselCount: 21,
		Mines: []industryplan.Mine{{ItemID: 1001, Kind: "solid", FacilityID: 2301, RatePerMinute: 30}},
		Ports: []industryplan.Port{{ItemID: 1001, Kind: "mining", RatePerMinute: 30}, {ItemID: 1001, Kind: "export", PeerPlanetID: 102, RatePerMinute: 30}},
	}, Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain(),
		Veins: []saveanalysis.PlanetVein{{ID: 1, ItemID: 1001, VeinType: 1, Amount: 5000, Position: galaxy.Vector{Z: -200}}}},
	}
	req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
	var mining []MiningCoverage
	if err := appendMining(ctx, &req, input, &actual, &mining, catalog); err != nil {
		t.Fatal(err)
	}
	before, originalObjects := slices.Clone(actual.Ports), len(req.Objects)
	if err := appendFleetStations(ctx, &req, input, catalog); err != nil {
		t.Fatal(err)
	}
	if len(req.Objects) != originalObjects+2 || !reflect.DeepEqual(before, actual.Ports) {
		t.Fatal("补塔不应制造额外带口或增加采矿、出口吞吐回执")
	}
	built, err := Build(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if built.StationCount != 3 || built.StationCount*10 < input.Plan.VesselCount || doc.Summary.ItemCounts[2104] != 3 || !slices.Contains(built.LocalDroneItems, 1001) || built.LocalModes[1001] != 3 {
		t.Fatalf("真实总站数、船位或本地供需不成立：%+v", built)
	}
	supply, transfer := 0, 0
	for _, station := range doc.Buildings {
		if station.ItemID != 2104 {
			continue
		}
		if station.Parameters[0] != 1001 || station.Parameters[2] != 1 {
			t.Fatalf("补塔没有使用真实出口角色：%v", station.Parameters[:6])
		}
		if station.Parameters[1] == 1 {
			supply++
		} else if station.Parameters[1] == 2 {
			transfer++
			assertFleetTowerHasNoBelts(t, station)
		}
	}
	if supply != 1 || transfer != 2 {
		t.Fatalf("矿区应一座本地供给塔向两座转运塔供货：%d/%d", supply, transfer)
	}
	assertFleetStationSpacing(t, req.Objects)
	count := len(req.Objects)
	if err := appendFleetStations(ctx, &req, input, catalog); err != nil || len(req.Objects) != count {
		t.Fatalf("已经足够的塔仍被重复添加：%v", err)
	}
}

// TestFleetStationsGroupCargoDirections 职责：跨五槽分组且保持同物品相反方向分塔；使用：多物品进出口和纯收货转运回归；边界：不将不同对端重复计作新槽，不添加无合同物品。
func TestFleetStationsGroupCargoDirections(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req, input := Request{PlanetID: 101}, ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101}}
	want := map[[2]int]int{}
	for i, item := range []int{1001, 1002, 1003, 1004, 1005, 1006, 1007, 1001} {
		remote, kind := 1, "export"
		if i%2 == 1 {
			remote, kind = 2, "import"
		}
		local := remote
		if i == 1 {
			local = 0 // 纯收货原本没有本地物流，补塔时需要形成真实需求端。
		}
		req.Objects = append(req.Objects, stationObject(int32(i), Position{Longitude: float64(i) * .3}, item, local, remote, false))
		input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: item, Kind: kind, PeerPlanetID: 102, RatePerMinute: 30})
		want[[2]int{item, remote}] = 1
	}
	input.Plan.Ports = append(input.Plan.Ports,
		industryplan.Port{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: 15},
		industryplan.Port{ItemID: 1208, Kind: "external", RatePerMinute: 30})
	input.Plan.StationCount = len(req.Objects) + 2
	original := len(req.Objects)
	if err := appendFleetStations(context.Background(), &req, input, catalog); err != nil {
		t.Fatal(err)
	}
	if req.Objects[1].Building.Parameters[1] != 2 || req.Objects[1].Building.Parameters[2] != 2 {
		t.Fatal("纯进口收货塔没有承接新增塔的本地供给，或远程角色被覆盖")
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]int]int{}
	for _, station := range doc.Buildings[original:] {
		assertFleetTowerHasNoBelts(t, station)
		items := map[int]bool{}
		count := 0
		for slot := range 5 {
			base := slot * 6
			item, local, remote := int(station.Parameters[base]), int(station.Parameters[base+1]), int(station.Parameters[base+2])
			if item == 0 {
				continue
			}
			if items[item] || local != 3-remote || !slices.Contains([]int{1, 2}, remote) || station.Parameters[base+3] <= 0 {
				t.Fatalf("运输塔槽位重复、方向冲突或缺少库存：%v", station.Parameters[:30])
			}
			items[item], count = true, count+1
			got[[2]int{item, remote}]++
			if !slices.Contains(built.LocalDroneItems, item) || built.LocalModes[item] != 3 {
				t.Fatalf("物品 %d 缺少已有设施与新塔的本地配对", item)
			}
		}
		if count == 0 || count > 5 {
			t.Fatalf("运输塔槽位数量错误：%d", count)
		}
	}
	if built.StationCount != input.Plan.StationCount || !reflect.DeepEqual(got, want) {
		t.Fatalf("分组遗漏、重复或添加了非合同物品：站数 %d，槽位 %v，期望 %v", built.StationCount, got, want)
	}
	assertFleetStationSpacing(t, req.Objects)
}

// TestFleetStationsRejectMissingLocalSource 职责：拒绝无本地承接来源的空运力；使用：不能靠新塔互相供需伪造货源；边界：真正的制造或采集端口仍需此前装配。
func TestFleetStationsRejectMissingLocalSource(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 101, Objects: []Object{stationObject(0, Position{}, 1002, 1, 1, false)}}
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, StationCount: 2,
		Ports: []industryplan.Port{{ItemID: 1001, Kind: "export", RatePerMinute: 30, PeerPlanetID: 102}}}}
	err = appendFleetStations(context.Background(), &req, input, catalog)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_FLEET_ROUTE" || len(req.Objects) != 1 {
		t.Fatalf("没有货源仍补出虚假船队塔：%v，实体 %d", err, len(req.Objects))
	}
}

// assertFleetTowerHasNoBelts 职责：核对补塔只依赖无人机；使用：严格解码后的记录；边界：不把库存槽当作传送带吞吐证据。
func assertFleetTowerHasNoBelts(t *testing.T, station blueprint.Building) {
	t.Helper()
	if station.ItemID != 2104 || len(station.Parameters) != 2048 || station.InputObjIdx != -1 || station.OutputObjIdx != -1 {
		t.Fatalf("补充运输塔记录无效：%+v", station)
	}
	for _, value := range station.Parameters[192:240] {
		if value != 0 {
			t.Fatal("补塔伪造了没有连接实体的传送带端口")
		}
	}
}

// assertFleetStationSpacing 职责：独立核对塔心之间的原版排斥距离；使用：新增塔与既有塔混合落位；边界：采用球面端点的游戏米距离。
func assertFleetStationSpacing(t *testing.T, objects []Object) {
	t.Helper()
	for i, left := range objects {
		if left.Building.ItemID != 2104 {
			continue
		}
		for _, right := range objects[:i] {
			if right.Building.ItemID == 2104 && energyWorldDistance(left.First, right.First) < 29-1e-5 {
				t.Fatal("星际塔间距小于原版 29 米")
			}
		}
	}
}
