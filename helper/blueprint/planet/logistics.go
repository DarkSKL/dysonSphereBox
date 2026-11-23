package planet

// 文件职责：配置星际物流角色及核对端口能力；使用：已生成的实际塔路线和单星规划；边界：飞船及燃料由玩家实际配置。

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/transport"
)

// localLogistics 职责：收集本地供需角色及不同实体间匹配的物品；使用：装配报告和旧塔相反角色查找；边界：不从远程库存推断。
func localLogistics(buildings []blueprint.Building) ([]int, map[int]uint8) {
	sources, sinks := map[int32]int32{}, map[int32]int32{}
	modes := map[int]uint8{}
	for _, b := range buildings {
		count := 0
		switch b.ItemID {
		case 2103:
			count = 4
		case 2104:
			count = 5
		case 2316:
			count = 1
		}
		if len(b.Parameters) < count*6 {
			continue
		}
		for slot := 0; slot < count; slot++ {
			item, role := b.Parameters[slot*6], b.Parameters[slot*6+1]
			if item <= 0 {
				continue
			}
			if role == 1 {
				sources[item] = b.Index
				modes[int(item)] |= 1
			} else if role == 2 {
				sinks[item] = b.Index
				modes[int(item)] |= 2
			}
		}
	}
	var items []int
	for item, source := range sources {
		if sink, found := sinks[item]; found && sink != source {
			items = append(items, int(item))
		}
	}
	slices.Sort(items)
	return items, modes
}

// configureRemoteGroup 职责：只为真实远程供需塔写入原版指定优先模式和 64 位专组；使用：整星最终编码；边界：不改纯本地塔。
func configureRemoteGroup(station *blueprint.Building, mask uint64) {
	if station.ItemID != 2104 || len(station.Parameters) != 2048 {
		return
	}
	for i := range 5 {
		base := i * 6
		if station.Parameters[base] != 0 && station.Parameters[base+2] != 0 {
			// 原版 BuildingParameters 按低字、高字恢复 remoteGroupMask，4 表示 Designated。
			station.Parameters[332] = int32(uint32(mask))
			station.Parameters[333] = int32(uint32(mask >> 32))
			station.Parameters[334] = 4
			return
		}
	}
}

// configureProductionPorts 职责：抵扣已有回执后分配制造塔的进出口并设置远程角色；使用：制造模块落位后；边界：矿机与电厂的本地总量不占制造边界，带速和新增库存仅分配一次。
func configureProductionPorts(plan industryplan.PlanetPlan, buildings []blueprint.Building, ports []build.BoundaryPort, prior ...industryplan.Port) ([]industryplan.Port, error) {
	return configureProductionFeeds(plan, buildings, ports, nil, prior...)
}

// configureProductionFeeds 在同一次边界扣账中把实际交给每个制造塔的出口速率回传给配船计算。
func configureProductionFeeds(plan industryplan.PlanetPlan, buildings []blueprint.Building, ports []build.BoundaryPort, feed func(int32, int, float64), prior ...industryplan.Port) ([]industryplan.Port, error) {
	manufacturing := map[int]bool{}
	for _, port := range ports {
		manufacturing[port.BuildingIndex] = true
	}
	for i := range buildings {
		if !manufacturing[i] {
			continue
		}
		b := &buildings[i]
		if b.ItemID != 2104 || len(b.Parameters) != 2048 {
			continue
		}
		for slot := 0; slot < 5; slot++ {
			base := slot * 6
			if b.Parameters[base+1] != 0 {
				b.Parameters[base+2] = 0
			}
		}
	}
	var actual []industryplan.Port
	left := make([]float64, len(ports))
	for i, port := range ports {
		left[i] = port.RatePerMinute
	}
	storageUsed := map[[2]int]int{}
	for _, want := range plan.Ports {
		input := want.Kind == "import" || want.Kind == "external"
		output := want.Kind == "export" || want.Kind == "external-export" || want.Kind == "delivery" || want.Kind == "storage"
		if !input && !output {
			continue
		}
		covered, stored := 0.0, 0
		for _, old := range prior {
			if old.ItemID == want.ItemID && old.Kind == want.Kind && old.PeerPlanetID == want.PeerPlanetID {
				covered += old.RatePerMinute
				stored += old.StorageItems
			}
		}
		remaining := want.RatePerMinute - covered
		if remaining <= 1e-8 {
			continue
		}
		rate, capacity := 0.0, 0
		seenSlots := map[[2]int]bool{}
		for i, port := range ports {
			if port.ItemID != want.ItemID || (port.Kind == "external-input") != input || left[i] <= 0 {
				continue
			}
			target := port.BuildingIndex
			if target < 0 || target >= len(buildings) || buildings[target].ItemID != 2104 || len(buildings[target].Parameters) != 2048 {
				return nil, &Error{PlanetID: plan.PlanetID, Item: want.ItemID, Code: "PLANET_PORT_ROUTE", Message: "已有制造端口未指向有效星际塔"}
			}
			tower := &buildings[target]
			slot := stationSlot(tower, want.ItemID)
			if slot < 0 {
				return nil, &Error{PlanetID: plan.PlanetID, Item: want.ItemID, Code: "PLANET_PORT_SLOT", Message: "制造端口在星际塔没有匹配物品槽"}
			}
			if want.Kind == "import" {
				tower.Parameters[slot*6+2] = 2
			}
			if want.Kind == "export" {
				tower.Parameters[slot*6+2] = 1
			}
			key := [2]int{target, slot}
			if !seenSlots[key] {
				available := max(0, int(tower.Parameters[slot*6+3])-storageUsed[key])
				added := min(available, max(0, want.StorageItems-stored-capacity))
				capacity += added
				storageUsed[key] += added
				seenSlots[key] = true
			}
			use := min(left[i], remaining-rate)
			if feed != nil && want.Kind == "export" && use > 0 {
				feed(tower.Index, want.ItemID, use)
			}
			rate += use
			left[i] -= use
			if rate+1e-8 >= remaining && stored+capacity >= want.StorageItems {
				break
			}
		}
		if rate > 0 {
			port := want
			port.RatePerMinute = covered + rate
			port.StorageItems = min(want.StorageItems, stored+capacity)
			actual = append(actual, port)
		}
	}
	return actual, nil
}

// stationSlot 职责：定位原版塔物品槽；使用：边界角色匹配；边界：物品不存在返回负数。
func stationSlot(tower *blueprint.Building, item int) int {
	for i := range 5 {
		if int(tower.Parameters[i*6]) == item {
			return i
		}
	}
	return -1
}

// stationObject 职责：创建标准星际塔记录；使用：独立物流或采矿转运；边界：单物品槽，燃料为补给请求。
func stationObject(index int32, p Position, item, local, remote int, fuel bool) Object {
	params := make([]int32, 2048)
	capacity, _ := transport.StationItemCapacity(2104)
	params[0], params[1], params[2], params[3] = int32(item), int32(local), int32(remote), int32(capacity)
	if fuel && item != 1210 {
		params[6], params[8], params[9] = 1210, 2, 100
	}
	params[320], params[321], params[322] = 1000000, -100000000, 240000000
	params[323], params[324], params[325], params[326], params[327] = 1, 480000, 1, 10, 100
	params[330], params[331] = 1, 1
	return Object{First: p, Second: p, Item: item, Module: "星际转运", Building: blueprint.Building{
		Index: index, ItemID: 2104, ModelIndex: 50, InputObjIdx: -1, OutputObjIdx: -1, Parameters: params,
	}}
}

// nextObjectIndex 职责：取得新设施可用编号；使用：保留原生产模块引用；边界：所有新编号高于已有编号。
func nextObjectIndex(objects []Object) int32 {
	var index int32
	for i := range objects {
		index = max(index, objects[i].Building.Index+1)
	}
	return index
}

// extendOccupancy 职责：将已安放模块加入选址障碍；使用：后续设施的空间搜索；边界：新实体取水平包围，物流塔专用间距单独记录，已有存档范围保持原样。
func extendOccupancy(req Request, catalog *placement.Catalog) []Obstacle {
	obstacles := append([]Obstacle(nil), req.Obstacles...)
	for _, o := range req.Objects {
		radius := groundRadius(catalog, o.Building)
		station := int16(0)
		if o.Building.ItemID == 2104 || o.Building.ItemID == 2103 {
			station = o.Building.ItemID
		}
		obstacles = append(obstacles, Obstacle{Position: o.First, Radius: radius, stationItem: station})
		if o.Second != o.First {
			obstacles = append(obstacles, Obstacle{Position: o.Second, Radius: radius, stationItem: station})
		}
	}
	return obstacles
}

// standaloneSite 找出常规独立塔候选，供可更换位置的辅助设施使用。
func (req *Request) standaloneSite(obstacles []Obstacle, catalog *placement.Catalog) (Position, bool) {
	return req.stationGridSite(context.Background(), obstacles, catalog, false)
}

// stationGridSite 在指定交付时补查全部非极点整格；塔位仍须满足陆面、原有设施、矿点和塔间距要求。
func (req *Request) stationGridSite(ctx context.Context, obstacles []Obstacle, catalog *placement.Catalog, allRows bool) (Position, bool) {
	index := newObstacleIndex(obstacles)
	tower := Object{Building: blueprint.Building{ItemID: 2104, ModelIndex: 50}}
	towerRadius := groundRadius(catalog, tower.Building)
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	rows := []int{0, 25, -25, 65, -65, 100, -100, 150, -150, 190, -190}
	checkRow := func(row, stride int, skipRegular bool) (Position, bool) {
		latitude := float64(float32(row) * float32(latitudeStep))
		lonStep, _ := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
		width := int(math.Round(2 * math.Pi / float64(lonStep)))
		for col := 0; col < width; col += stride {
			if skipRegular && col%5 == 0 {
				continue
			}
			p := Position{Latitude: latitude, Longitude: float64(col) * float64(lonStep)}
			if stride == 1 && req.wet != nil {
				if water, known := req.wet(p); !known || water {
					continue
				}
			}
			tower.First, tower.Second = p, p
			pt, err := world(p)
			if err == nil && !index.conflicts(pt, towerRadius) && !index.stationConflict(pt, 2104) && !req.terrainConflict([]Object{tower}, catalog) {
				return p, true
			}
		}
		return Position{}, false
	}
	for _, row := range rows {
		if p, ok := checkRow(row, 5, false); ok {
			return p, true
		}
	}
	if allRows {
		for n := 0; n < 499 && ctx.Err() == nil; n++ {
			row := (n + 1) / 2
			if n%2 == 0 {
				row = -row
			}
			if p, ok := checkRow(row, 1, slices.Contains(rows, row)); ok {
				return p, true
			}
		}
	}
	return Position{}, false
}

// appendReceiving 职责：为进口交付和本地余料补足连通库存；使用：跨星进口或玩家外供的直通物料；边界：纯收货不请求本地物料，混合仓储只连接已有真实供给槽。
func appendReceiving(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	var stations []blueprint.Building
	for _, object := range req.Objects {
		if object.Building.ItemID == 2103 || object.Building.ItemID == 2104 || object.Building.ItemID == 2316 {
			stations = append(stations, object.Building)
		}
	}
	_, localModes := localLogistics(stations)
	handled := map[int]bool{}
	for _, first := range input.Plan.Ports {
		if first.Kind != "import" && first.Kind != "external" || handled[first.ItemID] {
			continue
		}
		handled[first.ItemID] = true
		var incoming, demand []industryplan.Port
		var remoteModes []int
		var inboundRate, demandRate float64
		storage, local := 0, 0
		for _, wanted := range input.Plan.Ports {
			if wanted.ItemID != first.ItemID {
				continue
			}
			left, storageLeft := wanted.RatePerMinute, wanted.StorageItems
			for _, existing := range actual.Ports {
				if existing.Kind == wanted.Kind && existing.ItemID == wanted.ItemID && existing.PeerPlanetID == wanted.PeerPlanetID {
					left -= existing.RatePerMinute
					storageLeft -= existing.StorageItems
				}
			}
			if left <= 1e-8 {
				continue
			}
			switch wanted.Kind {
			case "import", "external":
				inboundRate += left
				incoming = append(incoming, wanted)
				remote := 0
				if wanted.Kind == "import" {
					remote = 2
				}
				if !slices.Contains(remoteModes, remote) {
					remoteModes = append(remoteModes, remote)
				}
			case "storage", "delivery":
				demandRate += left
				demand = append(demand, wanted)
				storage += max(0, storageLeft)
				// 已有本地余料与进口共用新增库存，必须让真实供给塔能用无人机送入。
				if wanted.Kind == "storage" && wanted.RatePerMinute-left > 1e-8 && localModes[wanted.ItemID]&1 != 0 {
					local = 2
				}
			}
		}
		if len(demand) == 0 || math.Abs(inboundRate-demandRate) > 1e-8 {
			continue
		}
		capacity, _ := transport.StationItemCapacity(2104)
		copies := max(len(remoteModes), 1+max(0, storage-1)/capacity)
		for i := 0; i < copies; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			p, ok := req.stationGridSite(ctx, append(extendOccupancy(*req, catalog), req.minerals...), catalog, true)
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return &Error{PlanetID: input.Plan.PlanetID, Item: first.ItemID, Code: "PLANET_PORT_SITE", Message: fmt.Sprintf("可施工纬度的整格中未找到同时满足陆面、占用与塔距的星际收货塔位置，还需 %d 座", copies-i)}
			}
			remote := remoteModes[min(i, len(remoteModes)-1)]
			req.Objects = append(req.Objects, stationObject(nextObjectIndex(req.Objects), p, first.ItemID, local, remote, input.Options.Setting.RequestStationWarpers))
		}
		for _, port := range append(incoming, demand...) {
			addActualPort(actual, port)
		}
	}
	for _, want := range input.Plan.Ports {
		if want.Kind != "storage" || want.StorageItems <= 0 {
			continue
		}
		for _, port := range actual.Ports {
			if port.Kind != "storage" || port.ItemID != want.ItemID || port.PeerPlanetID != want.PeerPlanetID ||
				port.RatePerMinute+1e-6 < want.RatePerMinute || port.StorageItems >= want.StorageItems {
				continue
			}
			capacity, _ := transport.StationItemCapacity(2104)
			for copies := 1 + (want.StorageItems-port.StorageItems-1)/capacity; copies > 0; copies-- {
				if err := ctx.Err(); err != nil {
					return err
				}
				p, ok := req.standaloneSite(append(extendOccupancy(*req, catalog), req.minerals...), catalog)
				if !ok {
					return &Error{PlanetID: input.Plan.PlanetID, Item: want.ItemID, Code: "PLANET_STORAGE_SITE", Message: fmt.Sprintf("缺少本地仓储塔落点，还需 %d 座", copies)}
				}
				req.Objects = append(req.Objects, stationObject(nextObjectIndex(req.Objects), p, want.ItemID, 2, 0, input.Options.Setting.RequestStationWarpers))
			}
			addActualPort(actual, want)
			break
		}
	}
	return nil
}
