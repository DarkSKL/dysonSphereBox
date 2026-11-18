package saveanalysis

// 文件职责：将物流槽配置、活动运输订单与飞行实载分别归属到同一快照的站点。

type stationBelt struct {
	dir, belt int64
	slot      int
}
type freightOrder struct {
	other                           int64
	thisIndex, otherIndex           int
	item, thisOrdered, otherOrdered int64
}
type freightRecord struct {
	ship                    bool
	planetA, planetB, other int64
	direction               float32
	item, count             int64
	order                   freightOrder
}

// SupplySlot 保存星球内站号、零基槽号和本地、远程净订单。零库存也是有效观测。
type SupplySlot struct {
	PlanetID    int64 `json:"planetId"`
	StationID   int64 `json:"stationId"`
	EntityID    int64 `json:"entityId"`
	GlobalID    int64 `json:"globalId"`
	Index       int   `json:"index"`
	ItemID      int64 `json:"itemId"`
	Inventory   int64 `json:"inventory"`
	Capacity    int64 `json:"capacity"`
	LocalMode   int64 `json:"localMode"`
	RemoteMode  int64 `json:"remoteMode"`
	LocalOrder  int64 `json:"localOrder"`
	RemoteOrder int64 `json:"remoteOrder"`
}

func (s *layoutStationComponent) slot(i int) SupplySlot {
	v := s.storage[i]
	return SupplySlot{s.planetId, s.id, s.entityId, s.gid, i, v.itemId, v.count, v.maxWithKeepMode % 100000000, v.localLogic, v.remoteLogic, v.localOrder, v.remoteOrder}
}
func (s *layoutStationComponent) matches(index int, item int64) bool {
	return index >= 0 && index < len(s.storage) && s.storage[index].itemId == item
}
func (s *layoutStationComponent) uniqueSlot(item int64) int {
	found := -1
	for i, v := range s.storage {
		if v.itemId == item {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}

// FreightTask 表示一条指向被查货槽的活动任务。空载取货和携货运输分别计量。
type FreightTask struct {
	Kind               string `json:"kind"`
	Phase              string `json:"phase"`
	OwnerPlanet        int64  `json:"ownerPlanet"`
	OwnerStation       int64  `json:"ownerStation"`
	WorkIndex          int    `json:"workIndex"`
	SourcePlanet       int64  `json:"sourcePlanet"`
	SourceStation      int64  `json:"sourceStation"`
	SourceIndex        int    `json:"sourceIndex"`
	SourceSlotEvidence string `json:"sourceSlotEvidence"`
	Cargo              int64  `json:"cargo"`
	Reserved           int64  `json:"reserved"`
}

// StationFreight 分别列出模式匹配的货槽与有活动任务证据的来源；任务目的地为 Slot。
type StationFreight struct {
	Slot           SupplySlot    `json:"slot"`
	ModeCandidates []SupplySlot  `json:"modeCandidates"`
	CargoInFlight  int64         `json:"cargoInFlight"`
	AwaitingPickup int64         `json:"awaitingPickup"`
	Tasks          []FreightTask `json:"tasks"`
}

func (s *Snapshot) stationFreight(target *layoutStationComponent, index int) StationFreight {
	r := StationFreight{Slot: target.slot(index), ModeCandidates: []SupplySlot{}, Tasks: []FreightTask{}}
	item := r.Slot.ItemID
	local := map[[2]int64]*layoutStationComponent{}
	global := map[int64]*layoutStationComponent{}
	for _, st := range s.stations {
		local[[2]int64{st.planetId, st.id}] = st
		if st.gid > 0 {
			if _, exists := global[st.gid]; exists {
				global[st.gid] = nil
			} else {
				global[st.gid] = st
			}
		}
		if st == target {
			continue
		}
		for i, v := range st.storage {
			if v.itemId != item {
				continue
			}
			nearby := st.planetId == target.planetId && v.localLogic == 1 && r.Slot.LocalMode == 2
			remote := st.planetId != target.planetId && st.isStellar != 0 && target.isStellar != 0 && v.remoteLogic == 1 && r.Slot.RemoteMode == 2
			if nearby || remote {
				r.ModeCandidates = append(r.ModeCandidates, st.slot(i))
			}
		}
	}
	for _, owner := range s.stations {
		drone, ship := 0, 0
		for _, d := range owner.work {
			workIndex, kind := drone, "drone"
			other := local[[2]int64{owner.planetId, d.other}]
			if d.ship {
				workIndex, kind = ship, "ship"
				ship++
				other = global[d.other]
				if other == nil || owner.planetId != d.planetA || other.planetId != d.planetB {
					continue
				}
			} else {
				drone++
			}
			if other == nil || other == owner || (other != target && owner != target) {
				continue
			}
			o := d.order
			if o.item != item || d.count < 0 || (d.count > 0 && d.item != item) {
				continue
			}
			source, sourceIndex := owner, o.thisIndex
			t := FreightTask{Kind: kind, OwnerPlanet: owner.planetId, OwnerStation: owner.id, WorkIndex: workIndex, SourceSlotEvidence: "order", Cargo: d.count}
			switch {
			case d.direction == 1 && d.count > 0 && other == target && o.other == d.other && o.otherIndex == index && o.otherOrdered > 0:
				t.Phase, t.Reserved = "carrying", o.otherOrdered
			case d.direction == 1 && d.count == 0 && owner == target && o.thisIndex == index && o.thisOrdered > 0 && o.otherOrdered < 0 && o.other == d.other:
				source, sourceIndex = other, o.otherIndex
				t.Phase, t.Reserved = "collecting", o.thisOrdered
			case d.direction == -1 && (o.other == 0 || o.other == d.other) && d.count > 0 && owner == target && o.thisIndex == index && o.thisOrdered > 0:
				source, sourceIndex = other, o.otherIndex
				t.Phase, t.Reserved = "carrying", o.thisOrdered
				// 取货返航时对端订单可能已清零，不能将清零后的下标当作第一个货槽。
				if o.other != d.other {
					sourceIndex = other.uniqueSlot(item)
					t.SourceSlotEvidence = "unique-item"
				}
			default:
				continue
			}
			if !source.matches(sourceIndex, item) {
				sourceIndex = -1
				t.SourceSlotEvidence = "unresolved"
			}
			t.SourcePlanet, t.SourceStation, t.SourceIndex = source.planetId, source.id, sourceIndex
			r.CargoInFlight += t.Cargo
			if t.Phase == "collecting" {
				r.AwaitingPickup += t.Reserved
			}
			r.Tasks = append(r.Tasks, t)
		}
	}
	return r
}
