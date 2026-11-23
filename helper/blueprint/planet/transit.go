package planet

// 文件职责：装配不经过制造或能源设备的同物品中转；使用：其他实体端口完成后的余量结算；边界：只承接守恒的边界直通，不增加制造、采矿或仓储能力。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// appendTransit 职责：为未覆盖且收支相等的物料建立双塔无人机中转；使用：能源回执完成后、船队补塔及接电之前；边界：按对端补齐流量，不用库存或中转替代未完成的生产消耗。
func appendTransit(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	handled := map[int]bool{}
	for _, first := range input.Plan.Ports {
		if first.Kind != "import" && first.Kind != "external" || handled[first.ItemID] {
			continue
		}
		handled[first.ItemID] = true
		var pending []industryplan.Port
		var inbound, outbound float64
		roles := map[[2]int]bool{}
		for _, want := range input.Plan.Ports {
			if want.ItemID != first.ItemID {
				continue
			}
			var local, remote int
			switch want.Kind {
			case "import":
				local, remote = 1, 2
			case "external":
				local, remote = 1, 0
			case "export":
				local, remote = 2, 1
			case "external-export":
				local, remote = 2, 0
			default:
				continue
			}
			left := want.RatePerMinute
			for _, covered := range actual.Ports {
				if covered.ItemID == want.ItemID && covered.Kind == want.Kind && covered.PeerPlanetID == want.PeerPlanetID {
					left -= covered.RatePerMinute
				}
			}
			if left <= 1e-8 {
				continue
			}
			if local == 1 {
				inbound += left
			} else {
				outbound += left
			}
			roles[[2]int{local, remote}] = true
			// 只有新增中转速率经过验证；原有回执和库存由 addActualPort 保留。
			want.StorageItems = 0
			pending = append(pending, want)
		}
		if inbound <= 1e-8 || outbound <= 1e-8 || math.Abs(inbound-outbound) > quant.FlowTolerance(inbound, outbound) {
			continue
		}
		// 同向多对端可以共用一槽；玩家外供与远程运输仍使用不同实体，避免覆盖角色。
		for _, role := range [][2]int{{1, 2}, {1, 0}, {2, 1}, {2, 0}} {
			if !roles[role] {
				continue
			}
			if err := search.Checkpoint(ctx); err != nil {
				return err
			}
			position, ok := req.standaloneSite(append(extendOccupancy(*req, catalog), req.minerals...), catalog)
			if !ok {
				return &Error{PlanetID: req.PlanetID, Item: first.ItemID, Code: "PLANET_TRANSIT_SITE", Message: fmt.Sprintf("物品 %d 缺少满足间距的中转塔落点", first.ItemID)}
			}
			station := stationObject(nextObjectIndex(req.Objects), position, first.ItemID, role[0], role[1], false)
			station.Module = "物料中转"
			req.Objects = append(req.Objects, station)
		}
		for _, port := range pending {
			addActualPort(actual, port)
		}
	}
	return nil
}
