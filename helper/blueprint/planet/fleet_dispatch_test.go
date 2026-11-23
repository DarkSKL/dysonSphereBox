package planet

import (
	"context"
	"errors"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// dispatchFixture 按每种货的实际出口组装分配合同，用实体塔槽和逐塔整数船数核对结果。
func dispatchFixture(t *testing.T, req *Request, cargo []industryplan.FleetCargo, capacity int) []industryplan.FleetStation {
	t.Helper()
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: req.PlanetID, OutboundFleet: cargo, VesselsPerStation: capacity}}
	for _, c := range cargo {
		input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{Kind: "export", ItemID: c.ItemID, RatePerMinute: 100})
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	actual := industryplan.PlanetActual{}
	if err := appendFleetStations(context.Background(), req, input, catalog, &actual); err != nil {
		t.Fatal(err)
	}
	assigned := map[int]float64{}
	seen := map[int32]bool{}
	for _, station := range actual.Fleet {
		if seen[station.BuildingIndex] {
			t.Fatal("同一座塔重复派船", station)
		}
		seen[station.BuildingIndex] = true
		b := req.Objects[station.BuildingIndex].Building
		total := 0.0
		for _, c := range station.Cargo {
			slot := stationSlot(&b, c.ItemID)
			if b.ItemID != 2104 || slot < 0 || b.Parameters[slot*6+2] != 1 || b.Parameters[slot*6+3] <= 0 {
				t.Fatalf("给没有相应远供库存的塔派船：%+v", station)
			}
			assigned[c.ItemID] += c.VesselLoad
			total += c.VesselLoad
		}
		if station.Vessels != int(math.Ceil(total-1e-9)) || station.Vessels > capacity {
			t.Fatalf("一座塔的多种货未共用整数船位：%+v", station)
		}
	}
	for _, c := range cargo {
		if math.Abs(assigned[c.ItemID]-c.VesselLoad) > 1e-8 {
			t.Fatalf("物品 %d 需要 %g 艘船时，分配 %g", c.ItemID, c.VesselLoad, assigned[c.ItemID])
		}
	}
	return actual.Fleet
}

func TestFleetSharedBerthsReassignExistingCargo(t *testing.T) {
	// 1 号塔只能直接发铁，0 号塔既能收铁转运又是齿轮的唯一来源。
	// 先给 0 号塔安排的铁必须迁到 1 号塔，才能给齿轮让出八个船位。
	shared := stationObject(0, Position{}, 1101, 2, 1, false)
	copy(shared.Building.Parameters[6:], stationObject(0, Position{}, 1104, 1, 1, false).Building.Parameters[:6])
	req := Request{PlanetID: 1202, Objects: []Object{shared, stationObject(1, Position{Longitude: 1}, 1101, 1, 1, false)}}
	rows := dispatchFixture(t, &req, []industryplan.FleetCargo{{ItemID: 1101, VesselLoad: 8}, {ItemID: 1104, VesselLoad: 8}}, 10)
	if len(req.Objects) != 2 || len(rows) != 2 || len(rows[0].Cargo) != 2 || rows[0].Vessels != 10 || rows[1].Vessels != 6 ||
		req.Objects[1].Building.Parameters[6] != 0 {
		t.Fatalf("既有槽足够仍增加槽或重复派船：%+v", rows)
	}
}

func TestFleetDirectSourcesCannotShipEachOthersOutput(t *testing.T) {
	// 两座直产塔分别只产 95 和 5 件/分钟。第二座虽有空船位，最多只能承担总船时的 5%。
	// 第三座塔应保留原进口槽，并利用自己的空槽由无人机接驳第一座塔的剩余产量。
	const item = 1104
	req := Request{PlanetID: 1202, Objects: []Object{
		stationObject(0, Position{}, item, 1, 1, false),
		stationObject(1, Position{Longitude: 1}, item, 1, 1, false),
		stationObject(2, Position{Longitude: 2}, 1001, 0, 2, false),
	}}
	req.addFleetFeed(0, item, 95)
	req.addFleetFeed(1, item, 5)
	rows := dispatchFixture(t, &req, []industryplan.FleetCargo{{ItemID: item, VesselLoad: 14}}, 10)
	if len(req.Objects) != 3 || len(rows) != 3 || math.Abs(rows[1].Cargo[0].VesselLoad-.7) > 1e-9 ||
		math.Abs(rows[2].Cargo[0].VesselLoad-3.3) > 1e-9 {
		t.Fatalf("直产塔领取了超出自己出带量的船时，或未利用空塔：%+v", rows)
	}
	slots := req.Objects[2].Building.Parameters
	if slots[0] != 1001 || slots[2] != 2 || slots[6] != item || slots[7] != 2 || slots[8] != 1 {
		t.Fatal("复用的收货塔要保留原进口，并为新货设置本地需求、远程供给", slots[:12])
	}
}

func TestFleetRoundsShipsPerPhysicalTower(t *testing.T) {
	req := Request{PlanetID: 101, Objects: []Object{
		stationObject(0, Position{}, 1101, 1, 1, false),
		stationObject(1, Position{Longitude: 1}, 1104, 1, 1, false),
		stationObject(2, Position{Longitude: 2}, 1001, 2, 2, false),
	}}
	for i := range req.Objects {
		req.Objects[i].Building.Parameters[327] = 10
	}
	rows := dispatchFixture(t, &req, []industryplan.FleetCargo{{ItemID: 1101, VesselLoad: .4}, {ItemID: 1104, VesselLoad: .4}}, 4)
	if len(rows) != 2 || rows[0].Vessels != 1 || rows[1].Vessels != 1 {
		t.Fatalf("两座独立发货塔各需至少一艘飞船：%+v", rows)
	}
	if req.Objects[0].Building.Parameters[327] != 100 || req.Objects[1].Building.Parameters[327] != 100 || req.Objects[2].Building.Parameters[327] != 10 {
		t.Fatal("应只将承担往返负载的实际发货塔设为满载起运")
	}
}

func TestFleetRemoteDemandCannotBecomeAnUnfedSupply(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 101, Objects: []Object{stationObject(0, Position{}, 1104, 2, 2, false)}}
	plan := industryplan.PlanetPlan{PlanetID: 101, VesselsPerStation: 10, OutboundFleet: []industryplan.FleetCargo{{ItemID: 1104, VesselLoad: 5}},
		Ports: []industryplan.Port{{Kind: "export", ItemID: 1104, RatePerMinute: 40}}}
	err = appendFleetStations(context.Background(), &req, ProductionRequest{Plan: plan}, catalog)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_FLEET_ROUTE" || len(req.Objects) != 1 || req.Objects[0].Building.Parameters[2] != 2 {
		t.Fatalf("用没有货源的远需槽或新塔冒充了远供：%v", err)
	}
}
