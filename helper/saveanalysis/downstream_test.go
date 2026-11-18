package saveanalysis

import "testing"

// TestStellarWarperIntake 区分已满的普通货槽与未核实的星际站翘曲器缓冲。
func TestStellarWarperIntake(t *testing.T) {
	w := outputWalk{
		g: deviceGraph{f: &factoryLine{}}, receivers: map[[2]int64]int{},
		stations: map[int64]*layoutStationComponent{10: {isStellar: 1,
			storage: []*layoutStationStore{{itemId: 1210, count: 5, maxWithKeepMode: 5}}}},
	}
	i := w.receiver(10, 1210)
	v := w.r.Receivers[i]
	if v.Status != "unresolved-intake" || v.Requested != nil || v.Slot == nil || v.Slot.Inventory != 5 {
		t.Fatal(v)
	}
}

// TestDropRefusal 验证普通投放点的两个充分拒收条件，以及存在空位时保留未知。
func TestDropRefusal(t *testing.T) {
	buf := make([]byte, 20)
	for i := 5; i <= 9; i++ {
		buf[i] = 1
	}
	if got := dropRefusal(buf, 4); got != "occupied-window" {
		t.Fatal(got)
	}
	buf[5] = 0
	if got := dropRefusal(buf, 4); got != "path-boundary" {
		t.Fatal(got)
	}
	clear(buf)
	for _, pos := range []int64{-1, 0, 4, 14, 18} {
		if got := dropRefusal(buf, pos); got != "" {
			t.Fatal(pos, got)
		}
	}
}

// TestOutputLimitCycle 验证循环、汇流和多种成品不会遗漏或重复关联实体。
func TestOutputLimitCycle(t *testing.T) {
	want := false
	r := OutputDiagnosis{
		Sources: []OutputSource{
			{EntityID: 7, ItemID: 1, Roots: []int64{8}, Shared: true, Ports: []OutputPort{{Route: -1, Receivers: []int{0}}}},
			{EntityID: 7, ItemID: 2, Roots: []int64{9}, Unresolved: true, Ports: []OutputPort{{Route: -1, Receivers: []int{0}}}},
		},
		Receivers: []OutputReceiver{{EntityID: 11, ItemID: 3, Status: "station-full", Requested: &want, Slot: &SupplySlot{StationID: 1, Inventory: 5, Capacity: 5}, Next: []int{0, 1}}},
	}
	r.propagateRouteLimits()
	r.prioritize()
	if len(r.Checks) != 1 {
		t.Fatal(r.Checks)
	}
	c := r.Checks[0]
	if len(c.DirectEntities) != 1 || len(c.RootEntities) != 2 || !c.Shared || !c.Unresolved {
		t.Fatal(c)
	}
}

// TestOutputRoom 货槽还有空间且无其他受阻证据时，建议停在设备出料端。
func TestOutputRoom(t *testing.T) {
	yes := true
	r := OutputDiagnosis{Sources: []OutputSource{{EntityID: 9, ItemID: 1, Roots: []int64{9}, Ports: []OutputPort{{Route: -1, Receivers: []int{0}}}}},
		Receivers: []OutputReceiver{{EntityID: 10, Requested: &yes, Slot: &SupplySlot{ItemID: 1, Inventory: 1, Capacity: 3}}}}
	r.prioritize()
	if len(r.Checks) != 1 || r.Checks[0].Status != "check-output" || !r.Checks[0].Unresolved {
		t.Fatal(r.Checks)
	}
}
