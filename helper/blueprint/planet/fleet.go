package planet

// 文件职责：为逐星船队估算补齐真实星际塔；使用：制造、采矿、收货和能源实体落位后；边界：只增加运输设施，不增加端口吞吐，逐航线船队分配仍需在游戏内确认。

import (
	"context"
	"fmt"
	"sort"

	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
)

// appendFleetStations 在矿区、制造和接驳完成后，按实际远供槽、货物船时与每塔共享船位分配出航船队。
func appendFleetStations(ctx context.Context, req *Request, input ProductionRequest, catalog *placement.Catalog, actual ...*industryplan.PlanetActual) error {
	if len(input.Plan.OutboundFleet) == 0 && input.Plan.VesselCount > 0 {
		// 旧单货合同的全部船时都有唯一归属；多货合同须重新从逐航线船时生成。
		item := 0
		for _, p := range input.Plan.Ports {
			if p.Kind != "export" || p.RatePerMinute <= 0 {
				continue
			}
			if item != 0 && item != p.ItemID {
				return &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_ROUTE", Message: "这份旧计划只有总船数。请重新规划，以便按每种货物的航线配船。"}
			}
			item = p.ItemID
		}
		if item == 0 {
			return &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_ROUTE", Message: "这份计划列出了发货船队，但缺少对应的出口。请重新规划。"}
		}
		input.Plan.OutboundFleet = []industryplan.FleetCargo{{ItemID: item, VesselLoad: float64(input.Plan.VesselCount)}}
	}
	if len(input.Plan.OutboundFleet) > 0 {
		fleet, err := dispatchFleet(ctx, req, input, catalog)
		if err != nil {
			return err
		}
		for _, a := range actual {
			a.Fleet = fleet
		}
	}
	if err := appendFleetStationMinimum(ctx, req, input, catalog); err != nil {
		return err
	}
	if input.Plan.VesselsPerStation > 0 {
		return fleetImportSlots(req.Objects, input.Plan.Ports, req.PlanetID)
	}
	return nil
}

// appendFleetStationMinimum 补齐进出口合同原定的总站数，兼容未带逐货船时的旧单星输入。
func appendFleetStationMinimum(ctx context.Context, req *Request, input ProductionRequest, catalog *placement.Catalog) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	existing := 0
	for _, object := range req.Objects {
		if object.Building.ItemID == 2104 {
			existing++
		}
	}
	missing := input.Plan.StationCount - existing
	if missing <= 0 {
		return nil
	}
	type slot struct{ item, remote int }
	seen := map[slot]bool{}
	var slots []slot
	for _, port := range input.Plan.Ports {
		if port.ItemID <= 0 || port.RatePerMinute <= 0 || port.Kind != "import" && port.Kind != "export" {
			continue
		}
		remote := 1
		if port.Kind == "import" {
			remote = 2
		}
		key := slot{port.ItemID, remote}
		if !seen[key] {
			seen[key] = true
			slots = append(slots, key)
		}
	}
	if len(slots) == 0 {
		return &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_ROUTE", Message: "需要补充运输塔，但本星没有可配置的真实进出口物品"}
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].item < slots[j].item || slots[i].item == slots[j].item && slots[i].remote < slots[j].remote
	})
	var groups [][]slot
	for _, entry := range slots {
		group := -1
		for i, values := range groups {
			if len(values) >= 5 {
				continue
			}
			duplicate := false
			for _, value := range values {
				duplicate = duplicate || value.item == entry.item
			}
			if !duplicate {
				group = i
				break
			}
		}
		if group < 0 {
			groups = append(groups, nil)
			group = len(groups) - 1
		}
		groups[group] = append(groups[group], entry)
	}
	// 只与补塔前的实际生产/收货设备配对，不能靠新塔互相供需冒充原货物流。
	original := len(req.Objects)
	for i := 0; i < missing; i++ {
		if err := search.Checkpoint(ctx); err != nil {
			return err
		}
		group := groups[i%len(groups)]
		for _, entry := range group {
			paired := false
			for index := 0; index < original; index++ {
				building := &req.Objects[index].Building
				count := 0
				switch building.ItemID {
				case 2104:
					count = 5
				case 2103:
					count = 4
				case 2316:
					count = 1
				}
				if len(building.Parameters) < count*6 {
					continue
				}
				for n := 0; n < count; n++ {
					base := n * 6
					if int(building.Parameters[base]) != entry.item {
						continue
					}
					// 纯收货槽原先无需无人机；补塔后让它接收新进口塔的本地供给。
					if building.ItemID == 2104 && building.Parameters[base+1] == 0 && int(building.Parameters[base+2]) == entry.remote {
						building.Parameters[base+1] = int32(entry.remote)
					}
					paired = paired || int(building.Parameters[base+1]) == entry.remote
				}
			}
			if !paired {
				return &Error{PlanetID: req.PlanetID, Item: entry.item, Code: "PLANET_FLEET_ROUTE", Message: "补充运输塔缺少已有设备的相反本地供需槽，无法形成无人机转运"}
			}
		}
		position, ok := stationGridSite(ctx, append(extendOccupancy(*req, catalog), req.minerals...), req.wet, catalog, true)
		if !ok {
			if err := ctx.Err(); err != nil {
				return err
			}
			return &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_SITE", Message: fmt.Sprintf("缺少满足间距的运输塔落点，还需 %d 座", missing-i)}
		}
		first := group[0]
		station := stationObject(nextObjectIndex(req.Objects), position, first.item, 3-first.remote, first.remote, false)
		station.Module = "船队转运"
		for n, entry := range group {
			base := n * 6
			station.Building.Parameters[base], station.Building.Parameters[base+1], station.Building.Parameters[base+2], station.Building.Parameters[base+3] = int32(entry.item), int32(3-entry.remote), int32(entry.remote), station.Building.Parameters[3]
		}
		req.Objects = append(req.Objects, station)
	}
	return nil
}
