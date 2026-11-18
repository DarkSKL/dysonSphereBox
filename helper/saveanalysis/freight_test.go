package saveanalysis

import "testing"

// TestFreightReturnEvidence 验证返程对端订单已清空、多槽同物和异星重号不能造成来源误归。
func TestFreightReturnEvidence(t *testing.T) {
	const item = 1120
	target := &layoutStationComponent{id: 1, gid: 10, planetId: 104, isStellar: 1, storage: []*layoutStationStore{{itemId: item, remoteLogic: 2}}}
	source := &layoutStationComponent{id: 1, gid: 20, planetId: 105, isStellar: 1, storage: []*layoutStationStore{{itemId: item, remoteLogic: 1}, {itemId: item}}}
	d := freightRecord{ship: true, planetA: 104, planetB: 105, other: 20, direction: -1, item: item, count: 12, order: freightOrder{item: item, thisOrdered: 12}}
	target.work = []freightRecord{d}
	s := Snapshot{stations: []*layoutStationComponent{target, source}}
	report := s.stationFreight(target, 0)
	if len(report.Tasks) != 1 || report.Tasks[0].SourcePlanet != 105 || report.Tasks[0].SourceIndex != -1 || report.Tasks[0].SourceSlotEvidence != "unresolved" || report.CargoInFlight != 12 {
		t.Fatal(report)
	}
	for _, mutate := range []func(*freightRecord){
		func(v *freightRecord) { v.count = 0 },
		func(v *freightRecord) { v.planetB = 104 },
		func(v *freightRecord) { v.item = 1305 },
		func(v *freightRecord) { v.order.other = 21 },
		func(v *freightRecord) { v.order.thisIndex = 1 },
	} {
		copy := d
		mutate(&copy)
		target.work = []freightRecord{copy}
		r := s.stationFreight(target, 0)
		if len(r.Tasks) != 0 || len(r.ModeCandidates) != 1 || r.CargoInFlight != 0 {
			t.Fatal(r)
		}
	}
}
