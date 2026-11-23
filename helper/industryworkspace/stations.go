package industryworkspace

import (
	"cmp"
	"slices"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
)

// existingSlots 查找同星本地或已开通天体航线上的相反旧货槽，保留游戏中的物流站和货槽序号。
func existingSlots(snapshot *saveanalysis.Snapshot, planetID int64, modes map[int]uint8, ports []industryplan.Port) []ExistingSlot {
	var matches []ExistingSlot
	remote := map[int]uint8{}
	items := map[int]bool{}
	for item := range modes {
		items[item] = true
	}
	for _, p := range ports {
		switch p.Kind {
		case "export":
			remote[p.ItemID] |= 1
		case "import":
			remote[p.ItemID] |= 2
		default:
			continue
		}
		items[p.ItemID] = true
	}
	for item := range items {
		records, err := snapshot.Item(int64(item), 60)
		if err != nil {
			continue
		} // 该物品在存档中没有物流或历史记录。
		for _, p := range records.Planets {
			other := p.PlanetID != planetID
			mode := modes[item]
			if other {
				mode = remote[item]
				routed, banned := snapshot.RemoteAstroRules(planetID, p.PlanetID, int64(item))
				if !routed || banned {
					continue
				}
			}
			if mode == 0 {
				continue
			}
			for _, slot := range p.Slots {
				logic := slot.LocalLogic
				if other {
					if !slot.Stellar {
						continue
					}
					logic = slot.RemoteLogic
				}
				if logic == 1 && mode&2 != 0 || logic == 2 && mode&1 != 0 {
					matches = append(matches, ExistingSlot{ItemID: item, PlanetID: p.PlanetID, StationID: slot.StationID, Slot: slot.Index + 1, Logic: logic, Remote: other})
				}
			}
		}
	}
	slices.SortFunc(matches, func(a, b ExistingSlot) int {
		if a.PlanetID != b.PlanetID {
			return cmp.Compare(a.PlanetID, b.PlanetID)
		}
		if a.StationID != b.StationID {
			return cmp.Compare(a.StationID, b.StationID)
		}
		return cmp.Compare(a.Slot, b.Slot)
	})
	return matches
}
