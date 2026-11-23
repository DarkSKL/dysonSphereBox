package planet

// 文件职责：按真实矿点和原版矿机扫描形状装配矿区；使用：单星生产规划；边界：不推测矿点、油田速率或科技倍率。

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/transport"
)

// veinPoint 职责：关联存档矿点与世界坐标；使用：计算覆盖和互斥；边界：油型矿点单独处理。
type veinPoint struct {
	id, item, kind int64
	p              Position
	v              placement.Vector
	colliderCenter placement.Vector
	pavedCenter    placement.Vector
	colliderRadius float64
	occupied       bool
	newMiners      uint8 // 仅登记本张图已放下的普通矿机，已有矿机、油井与大型矿机点仍由 occupied 独占。
	rate           float64
	amount         int64
}

// veinNeighborhoods 按采矿扇区与建造盒的最大半径登记矿石和油井，索引保持原矿点顺序。
type veinNeighborhoods map[[3]int][]int

const minerScanRadius = 12

// veinCollider 返回原版矿物碰撞球沿星面法向的位移与球半径，单位为游戏米。
func veinCollider(kind int64) (lift, radius float64) {
	if kind == 7 {
		return -.35, 1.5
	}
	return .4, .5
}

// indexVeinNeighborhoods 仅按固定坐标缩小原版扫描范围，使用矿点编号回查本轮最新的矿机占用。
func indexVeinNeighborhoods(points []veinPoint, radius float64) veinNeighborhoods {
	index := make(veinNeighborhoods)
	for i := range points {
		v := &points[i]
		for x := obstacleCell(v.v[0] - radius); x <= obstacleCell(v.v[0]+radius); x++ {
			for y := obstacleCell(v.v[1] - radius); y <= obstacleCell(v.v[1]+radius); y++ {
				for z := obstacleCell(v.v[2] - radius); z <= obstacleCell(v.v[2]+radius); z++ {
					key := [3]int{x, y, z}
					index[key] = append(index[key], i)
				}
			}
		}
	}
	return index
}

// MiningCoverage 职责：记录已建矿机与真实矿点及有效能力；使用：施工清单；边界：速率受基础采矿及带容量共同约束。
type MiningCoverage struct {
	// ItemID/FacilityID 职责：标识采出物及采集设备；使用：与规划对拍；边界：原版物品号。
	ItemID, FacilityID int
	// BuildingIndex 职责：定位最终蓝图记录；使用：游戏施工和诊断；边界：为最终建筑编号。
	BuildingIndex int
	// Position 职责：原版矿机真实落点；使用：核对矿区；边界：经纬弧度。
	Position Position
	// VeinIDs 职责：保存覆盖的存档矿点；使用：独立核对；边界：同星球同快照。
	VeinIDs []int64
	// RatePerMinute 职责：记录矿机与接驳带联合额定能力；使用：产量检查；边界：基础科技条件。
	RatePerMinute float64
	// CoveredItems 职责：汇总实际覆盖的有限固矿剩余件数；使用：施工清单；边界：无限矿、油井与液体为 nil。
	CoveredItems *int64
	// NewCoveredItems 供整星按唯一矿点累计储量；本机原始覆盖仍完整记录在 VeinIDs 与 CoveredItems。
	NewCoveredItems int64
}

// placeMining 职责：完成整份采集合同；使用：在首个施工阶段、尚无本图实体或出口记录时调用。
// 边界：岸边普通矿机按常规塔距缺矿时才从头换用短带，只有成功尝试的实体、出口与覆盖记录进入后续施工。
func placeMining(ctx context.Context, req *Request, input ProductionRequest, catalog *placement.Catalog) (industryplan.PlanetActual, []MiningCoverage, error) {
	trial := *req
	var actual industryplan.PlanetActual
	var coverage []MiningCoverage
	err := appendMining(ctx, &trial, input, &actual, &coverage, catalog)
	if issue, ok := err.(*Error); ok && issue.Code == "PLANET_MINING_SITE" && req.stationPad != nil &&
		slices.ContainsFunc(input.Plan.Mines, func(m industryplan.Mine) bool { return m.ItemID == issue.Item && m.FacilityID == 2301 }) {
		// 缩短首台矿机的接驳带也会改变后续塔位，不能接着失败尝试的实体编号继续摆放。
		input.Options.nearMineTowers = true
		trial, actual, coverage = *req, industryplan.PlanetActual{}, nil
		if retry := appendMining(ctx, &trial, input, &actual, &coverage, catalog); retry == nil {
			err = nil
		} else if ctx.Err() != nil {
			err = ctx.Err()
		}
	}
	if err != nil {
		return industryplan.PlanetActual{}, nil, err
	}
	*req = trial
	return actual, coverage, nil
}

// appendMining 职责：放置真实矿机和接驳；使用：逐星可验证矿业流量；边界：仅按每矿点基础速率计数。
func appendMining(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, mining *[]MiningCoverage, catalog *placement.Catalog) error {
	priorPorts := append([]industryplan.Port(nil), actual.Ports...)
	points := make([]veinPoint, 0, len(input.Resources.Veins))
	for _, vein := range input.Resources.Veins {
		p, err := savePosition(vein.Position)
		if err != nil || vein.ID <= 0 || vein.Amount <= 0 || vein.IsSurfaceMiningBlocked() {
			continue
		}
		rate := 0.0
		if vein.Rate != nil {
			rate = *vein.Rate * 60
		}
		v := placement.Vector{vein.Position.X, vein.Position.Y, vein.Position.Z}
		lift, radius := veinCollider(vein.VeinType)
		normal := v.Scale(1 / math.Sqrt(v.Dot(v)))
		point := veinPoint{id: vein.ID, item: vein.ItemID, kind: vein.VeinType, p: p, v: v, colliderCenter: v.Add(normal.Scale(lift)), colliderRadius: radius, occupied: len(vein.MinerEntityIDs) != 0, rate: rate, amount: vein.Amount}
		if vein.VeinType != 7 && (req.Foundation == "construction" || req.Foundation == "planet") {
			// 原版铺地将露出的固矿移到改造地高下 0.13 米，再沿法向更新其碰撞球。
			point.pavedCenter = normal.Scale(200.2 - .13 + lift)
		}
		points = append(points, point)
	}
	infinite := map[int]bool{}
	for _, resource := range input.Resources.Planet.Resources {
		infinite[int(resource.ItemID)] = resource.Infinite
	}
	replay := beginMiningReplay(ctx, req, input, points)
	defer replay.finish()
	var neighborhoods veinNeighborhoods
	for _, mine := range input.Plan.Mines {
		if mine.FacilityID != 2301 && mine.FacilityID != 2316 && mine.FacilityID != 2307 && mine.FacilityID != 2306 {
			return &Error{PlanetID: req.PlanetID, Item: mine.ItemID, Code: "PLANET_MINING_DEVICE", Message: "此采集设备缺少可靠的原版落点与接驳规则"}
		}
		if mine.SpeedMultiplier == 0 {
			mine.SpeedMultiplier = 1
		}
		if mine.FacilityMultiplier == 0 {
			mine.FacilityMultiplier = 1
		}
		large := mine.FacilityID == 2316
		factor := mine.FacilityMultiplier
		if large && (factor < 1 || factor > 3 || math.Abs(factor*10-math.Round(factor*10)) > 1e-6) || !large && factor != 1 {
			return &Error{PlanetID: req.PlanetID, Item: mine.ItemID, Code: "PLANET_MINING_SPEED", Message: "设备倍率超出所选原版矿机的可达档位"}
		}
		remaining, covered := mine.RatePerMinute, 0
		var sharedPoints int
		owners := uint8(1)
		var shore liquidSiteCursor
		var sites map[minerSite]minerCandidate
		firstMiner := len(*mining)
		connectedStorage := 0
		exporting := false
		for _, port := range input.Plan.Ports {
			if port.ItemID == mine.ItemID && port.Kind == "export" {
				exporting = true
			}
		}
		for remaining > 1e-6 {
			if err := ctx.Err(); err != nil {
				return err
			}
			choice := miningChoice{item: mine.ItemID, facility: mine.FacilityID, speed: mine.SpeedMultiplier, setting: factor, remaining: remaining, exporting: exporting}
			// 油井选实际速率最高的可接驳井，泵选第一段可接驳岸线。两者的单台选址都与剩余需求无关。
			if mine.FacilityID == 2306 || mine.FacilityID == 2307 {
				choice.remaining = 0
			}
			layout, ids, rate, reused := replay.restore(choice)
			if !reused {
				var err error
				switch mine.FacilityID {
				case 2307:
					layout, ids, rate, err = findOil(ctx, *req, input.Options, mine, points, catalog, exporting)
				case 2306:
					layout, rate, err = findLiquid(ctx, *req, input, mine, catalog, exporting, &shore)
				default:
					if neighborhoods == nil {
						reach := float64(minerScanRadius)
						if slices.ContainsFunc(input.Plan.Mines, func(m industryplan.Mine) bool { return m.FacilityID == 2316 }) {
							// 大矿机的高架机翼伸出根节点十余米；按实际建造盒和最大矿石球扩展候选。
							reach = max(reach, recordRadius(catalog, blueprint.Building{ItemID: 2316, ModelIndex: 256})+2)
						}
						neighborhoods = indexVeinNeighborhoods(points, reach)
					}
					if sites == nil {
						sites = make(map[minerSite]minerCandidate)
					}
					layout, ids, rate, err = findMiner(ctx, *req, input.Options, mine, points, neighborhoods, catalog, exporting, remaining, owners, sites, false)
					// 只有独占落点已耗尽且合同仍缺货，才复核本图新普通矿机用过的矿点。已占空地只会增加。
					if err == nil && len(layout) == 0 && mine.FacilityID == 2301 && owners == 1 && slices.ContainsFunc(points, func(p veinPoint) bool {
						return p.item == int64(mine.ItemID) && p.newMiners > 0 && p.newMiners < industryplan.MaxOrdinaryMinersPerVein
					}) {
						owners = industryplan.MaxOrdinaryMinersPerVein
						layout, ids, rate, err = findMiner(ctx, *req, input.Options, mine, points, neighborhoods, catalog, exporting, remaining, owners, sites, false)
					}
					if err == nil && len(layout) == 0 {
						layout, ids, rate, err = findMiner(ctx, *req, input.Options, mine, points, neighborhoods, catalog, exporting, remaining, owners, sites, true)
					}
				}
				if err != nil {
					return err
				}
			}
			if len(layout) == 0 || rate <= 0 {
				return &Error{PlanetID: req.PlanetID, Item: mine.ItemID, Code: "PLANET_MINING_SITE", AvailableRatePerMinute: mine.RatePerMinute - remaining, Message: fmt.Sprintf("可避让并接驳的采集点不足，已接驳 %d 个矿点，尚缺 %.3f 件/分钟", covered, remaining)}
			}
			if !reused {
				replay.remember(choice, layout, ids, rate)
			}
			report := MiningCoverage{BuildingIndex: len(req.Objects), ItemID: mine.ItemID, FacilityID: mine.FacilityID, Position: layout[0].First, RatePerMinute: rate}
			var coveredItems int64
			for _, i := range ids {
				v := &points[i]
				if v.newMiners == 0 {
					report.NewCoveredItems += v.amount
					covered++
				} else {
					owners = industryplan.MaxOrdinaryMinersPerVein // 从任务内缓存恢复已共享的真实矿机时，同步恢复搜索阶段。
					if v.newMiners == 1 {
						sharedPoints++
					}
				}
				if mine.FacilityID == 2301 {
					v.newMiners++
				} else {
					v.occupied = true
				}
				report.VeinIDs = append(report.VeinIDs, v.id)
				coveredItems += v.amount
			}
			if (mine.FacilityID == 2301 || mine.FacilityID == 2316) && !infinite[mine.ItemID] {
				report.CoveredItems = &coveredItems
			}
			*mining = append(*mining, report)
			req.Objects = append(req.Objects, layout...)
			// 采集接驳的第二条记录为带路直连的同物品库存塔。
			connectedStorage += int(layout[1].Building.Parameters[3])
			remaining -= rate
		}
		if sharedPoints > 0 {
			name := fmt.Sprint(mine.ItemID)
			if fs, err := facts.Load(); err == nil {
				if f, ok := fs.Lookup(mine.ItemID); ok {
					name = f.Name
				}
			}
			actual.Warnings = append(actual.Warnings, fmt.Sprintf("%s有 %d 个矿点由本图多台普通矿机共同开采。剩余矿量按唯一矿点统计，共用矿点可能较早枯竭，届时请复核采速。", name, sharedPoints))
		}
		actual.Ports = append(actual.Ports, industryplan.Port{ItemID: mine.ItemID, Kind: "mining", RatePerMinute: mine.RatePerMinute})
		if mine.FacilityID == 2306 {
			actual.Warnings = append(actual.Warnings, "抽水站落位已经核对原版的四个陆地点和四个进液点")
		}
		remaining = mine.RatePerMinute
		exportRate := 0.0
		for _, port := range input.Plan.Ports {
			if port.ItemID != mine.ItemID || port.Kind != "export" && port.Kind != "external-export" {
				continue
			}
			covered := 0.0
			for _, existing := range actual.Ports {
				if existing.ItemID == port.ItemID && existing.Kind == port.Kind && existing.PeerPlanetID == port.PeerPlanetID {
					covered += existing.RatePerMinute
				}
			}
			use := min(port.RatePerMinute-covered, remaining)
			if use > 1e-6 {
				remaining -= use
				if port.Kind == "export" {
					exportRate += use
				}
				port.RatePerMinute = covered + use
				// 出矿带只证明出口流量，已有库存回执由 addActualPort 保留。
				port.StorageItems = 0
				addActualPort(actual, port)
			}
		}
		// 只把矿源实际承担的出口分摊给直连库存塔，单塔直接发货量不能超过其矿机和带路的实测容量。
		for _, placed := range (*mining)[firstMiner:] {
			if exportRate <= 0 {
				break
			}
			use := min(exportRate, placed.RatePerMinute)
			req.addFleetFeed(req.Objects[placed.BuildingIndex+1].Building.Index, mine.ItemID, use)
			exportRate -= use
		}
		// 制造可由进口共同供料；只把守恒后的矿源余量分配到真实直连塔库存。
		remaining = min(remaining, miningStockSurplus(input.Plan, *actual, priorPorts, mine.ItemID))
		for _, port := range input.Plan.Ports {
			if port.ItemID != mine.ItemID || port.Kind != "delivery" && port.Kind != "storage" {
				continue
			}
			covered, stored := 0.0, 0
			for _, old := range actual.Ports {
				if old.ItemID == port.ItemID && old.Kind == port.Kind && old.PeerPlanetID == port.PeerPlanetID {
					covered, stored = old.RatePerMinute, old.StorageItems
				}
			}
			use := min(max(0, port.RatePerMinute-covered), remaining)
			if use > 1e-6 {
				capacity := min(connectedStorage, max(0, port.StorageItems-stored))
				remaining -= use
				connectedStorage -= capacity
				port.RatePerMinute, port.StorageItems = covered+use, stored+capacity
				addActualPort(actual, port)
			}
		}
	}
	return nil
}

// miningStockSurplus 职责：核对真实矿源在制造、供电和外送后的可储存余量；使用：直连矿区塔库存分配；边界：进口仍须由整星终检证明，不把额定制造容量当作实际工艺消耗。
func miningStockSurplus(plan industryplan.PlanetPlan, actual industryplan.PlanetActual, priorPorts []industryplan.Port, item int) float64 {
	remaining, fuel, priorExportSupply := 0.0, 0.0, 0.0
	// 前序模块已完成的出口不占用新矿；先扣除已知来源，避免同一进口或制造产出重复计入。
	for _, port := range priorPorts {
		if port.ItemID != item {
			continue
		}
		switch port.Kind {
		case "mining":
			priorExportSupply -= port.RatePerMinute
		case "export", "external-export":
			for _, desired := range plan.Ports {
				if desired.ItemID == item && desired.Kind == port.Kind && desired.PeerPlanetID == port.PeerPlanetID {
					priorExportSupply += min(port.RatePerMinute, desired.RatePerMinute)
					break
				}
			}
		}
	}
	for _, port := range actual.Ports {
		if port.ItemID != item {
			continue
		}
		switch port.Kind {
		case "mining":
			remaining += port.RatePerMinute
		case "delivery", "storage":
			remaining -= port.RatePerMinute
		}
	}
	for _, port := range plan.Ports {
		if port.ItemID != item {
			continue
		}
		switch port.Kind {
		case "import", "external":
			remaining += port.RatePerMinute
			priorExportSupply -= port.RatePerMinute
		case "export", "external-export":
			remaining -= port.RatePerMinute
		case "power-fuel":
			fuel += port.RatePerMinute
		}
	}
	for _, production := range plan.Plan.Productions {
		for _, input := range production.Inputs {
			if input.ItemID == item {
				remaining -= input.RatePerMinute
			}
		}
		for _, output := range production.Outputs {
			if output.ItemID == item {
				remaining += output.RatePerMinute
				priorExportSupply -= output.RatePerMinute
			}
		}
	}
	if power := plan.Power; power != nil {
		ratedFuel := 0.0
		for _, source := range power.Sources {
			if source.FuelItemID == item {
				ratedFuel += source.FuelRatePerMinute
			}
		}
		if len(power.Sources) == 0 && power.FuelItemID == item {
			ratedFuel = power.FuelRatePerMinute
		}
		fuel = max(fuel, ratedFuel)
	}
	return max(0, remaining-fuel+max(0, priorExportSupply))
}

// minerSite 使用半格整数坐标和朝向区分同一矿区的候选，不把本机后面新增的采矿占用计入静态覆盖。
type minerSite struct{ latitude, longitude, yaw int }

// minerCandidate 保存当前矿区快照的真实覆盖。选址时仍按现有矿机数、占地和带路判断。
type minerCandidate struct {
	point geometry.WorldPoint
	ids   []int
}

// findMiner 职责：枚举实际矿点附近的原版矿机位姿；使用：当前物品还缺矿；边界：混矿、已占用矿和不真实的接驳直接排除。
func findMiner(ctx context.Context, req Request, options ProductionOptions, mine industryplan.Mine, points []veinPoint, neighborhoods veinNeighborhoods, catalog *placement.Catalog, exporting bool, remaining float64, owners uint8, sites map[minerSite]minerCandidate, fine bool) ([]Object, []int, float64, error) {
	occupied := newObstacleIndex(extendOccupancy(req, catalog))
	minerals := newObstacleIndex(req.minerals)
	large := mine.FacilityID == 2316
	model := int16(57)
	if large {
		model = 256
	}
	indices := nextObjectIndex(req.Objects)
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	base := blueprint.Building{Index: indices, ItemID: int16(mine.FacilityID), ModelIndex: model, InputObjIdx: -1, OutputObjIdx: -1}
	if large {
		base.Parameters = make([]int32, 2048)
		base.Parameters[0], base.Parameters[1], base.Parameters[3] = int32(mine.ItemID), 1, 10000
		base.Parameters[192+4], base.Parameters[193+4] = 1, 1
		// BuildingParameters.ApplyPrebuildParametersToEntity 的原版索引为 320+9，10000 表示 100%。
		base.Parameters[329] = int32(math.Round(mine.FacilityMultiplier * 10000))
	}
	minerRadius := groundRadius(catalog, base)
	bodyReach := recordRadius(catalog, base) + 1.5
	shapes, err := catalog.Shapes(base.ItemID, base.ModelIndex)
	if err != nil {
		return nil, nil, 0, err
	}
	var best []Object
	var bestIDs []int
	bestRate := 0.0
	lateral := []float64{0, -1, 1, -2, 2, -3, 3, -4, 4}
	radial := []float64{3, 5, 7, 9, 11, 13, 4, 6, 8, 10, 12, 2, 14, 1, 15}
	if fine {
		lateral = []float64{-.5, .5, -1.5, 1.5, -2.5, 2.5, -3.5, 3.5, 0, -1, 1, -2, 2}
		radial = []float64{3.5, 5.5, 7.5, 4.5, 6.5, 2.5, 8.5, 9.5, 10.5, 3, 5, 7, 4, 6, 8}
	}
	for _, seed := range points {
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		if seed.item != int64(mine.ItemID) || seed.kind == 7 || seed.occupied || seed.newMiners >= owners {
			continue
		}
		for _, dy := range lateral {
			seedGrid := math.Round(seed.p.Latitude / float64(latitudeStep))
			latitude := (seedGrid + dy) * float64(latitudeStep)
			if math.Abs(latitude) > 1.52 {
				continue
			}
			step, err := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
			if err != nil {
				continue
			}
			for _, yaw := range []float64{90, -90, 0, 180} {
				east, north := math.Round(math.Sin(yaw*math.Pi/180)), math.Round(math.Cos(yaw*math.Pi/180))
				for _, dx := range radial {
					candidateLatitude := latitude + north*dx*float64(latitudeStep)
					candidateStep := step
					if north != 0 {
						candidateLatitude = (seedGrid + north*dx) * float64(latitudeStep)
						candidateStep, err = geometry.PasteLongitudeStep32(geometry.Radians(candidateLatitude), geometry.Vanilla200)
						if err != nil {
							continue
						}
					}
					lonGrid := math.Round(seed.p.Longitude/float64(candidateStep)) + east*dx + math.Abs(north)*dy
					p := Position{Latitude: candidateLatitude, Longitude: lonGrid * float64(candidateStep)}
					key := minerSite{int(math.Round(2 * candidateLatitude / float64(latitudeStep))), int(2 * lonGrid), int(yaw)}
					site, seen := sites[key]
					if !seen {
						pt, err := world(p)
						if err == nil && !minerOverlapsVein(p, pt, yaw, shapes, points, neighborhoods, bodyReach) {
							site = minerCandidate{pt, minerCoverage(p, pt, yaw, large, mine.ItemID, points, neighborhoods, 255)}
						}
						sites[key] = site
					}
					ids := site.ids
					if len(ids) == 0 || slices.ContainsFunc(ids, func(i int) bool { return points[i].occupied || points[i].newMiners >= owners }) {
						continue
					}
					rate := float64(len(ids)) * industryplan.SolidMiningRate(mine.FacilityID, mine.SpeedMultiplier, mine.FacilityMultiplier)
					if rate <= bestRate {
						continue
					}
					if occupied.conflicts(site.point, minerRadius) {
						sites[key] = minerCandidate{}
						continue
					}
					miner := Object{Building: base, First: p, Second: p, Item: mine.ItemID, Module: "矿区"}
					miner.Building.Yaw, miner.Building.Yaw2 = float32(yaw), float32(yaw)
					layout, capacity := req.minerTransport(miner, options, occupied, minerals, catalog, exporting, yaw, float64(candidateStep))
					if len(layout) == 0 {
						sites[key] = minerCandidate{}
						continue
					}
					rate = min(rate, capacity)
					if rate+1e-6 >= remaining {
						return layout, ids, rate, nil
					}
					if rate > bestRate {
						best, bestIDs, bestRate = layout, ids, rate
					}
				}
			}
		}
	}
	return best, bestIDs, bestRate, nil
}

// minerOverlapsVein 核对原版矿机的机身及支脚与矿物碰撞球；开采扇区内的矿石仍可落在盒外。
func minerOverlapsVein(p Position, pt geometry.WorldPoint, yaw float64, shapes []placement.Collider, points []veinPoint, neighborhoods veinNeighborhoods, reach float64) bool {
	center := placement.Vector{float64(pt.X), float64(pt.Y), float64(pt.Z)}
	up := center.Scale(1 / math.Sqrt(center.Dot(center)))
	east := placement.Vector{math.Cos(p.Longitude), 0, math.Sin(p.Longitude)}
	north := east.Cross(up)
	s, c := math.Sincos(yaw * math.Pi / 180)
	right, forward := east.Scale(c).Sub(north.Scale(s)), east.Scale(s).Add(north.Scale(c))
	cell := [3]int{obstacleCell(center[0]), obstacleCell(center[1]), obstacleCell(center[2])}
	for _, index := range neighborhoods[cell] {
		point := &points[index]
		// 原版角色实测支脚相切距离 0.487～0.500 米，保留粘贴投影误差。
		radius := point.colliderRadius + .04
		if minerHitsSphere(point.colliderCenter.Sub(center), right, up, forward, shapes, reach, radius) ||
			point.pavedCenter != (placement.Vector{}) && minerHitsSphere(point.pavedCenter.Sub(center), right, up, forward, shapes, reach, radius) {
			return true
		}
	}
	return false
}

// minerHitsSphere 核对一种真实地高下的矿石球与原版矿机的局部轴对齐建造盒。
func minerHitsSphere(delta, right, up, forward placement.Vector, shapes []placement.Collider, reach, radius float64) bool {
	if delta.Dot(delta) > reach*reach {
		return false
	}
	local := [3]float64{delta.Dot(right), delta.Dot(up), delta.Dot(forward)}
	for _, shape := range shapes {
		distance := 0.
		for axis, value := range local {
			gap := max(0, math.Abs(value-float64(shape.Position[axis]))-float64(shape.Extents[axis]))
			distance += gap * gap
		}
		if distance <= radius*radius {
			return true
		}
	}
	return false
}

// minerCoverage 职责：复现原版 paste 的采矿区域形状；使用：枚举实际矿点；边界：混矿或联用既有矿点的候选作废。
func minerCoverage(p Position, pt geometry.WorldPoint, yaw float64, large bool, item int, points []veinPoint, neighborhoods veinNeighborhoods, owners uint8) []int {
	center := placement.Vector{float64(pt.X), float64(pt.Y), float64(pt.Z)}
	up := placement.Vector{math.Cos(p.Latitude) * math.Sin(p.Longitude), math.Sin(p.Latitude), -math.Cos(p.Latitude) * math.Cos(p.Longitude)}
	east := placement.Vector{math.Cos(p.Longitude), 0, math.Sin(p.Longitude)}
	north := east.Cross(up)
	s, c := math.Sincos(yaw * math.Pi / 180)
	forward := east.Scale(s).Add(north.Scale(c))
	right := forward.Cross(up)
	distance := 1.2
	if large {
		distance = 10
	}
	for i := range center {
		center[i] -= forward[i] * distance
	}
	var ids []int
	cell := [3]int{obstacleCell(center[0]), obstacleCell(center[1]), obstacleCell(center[2])}
	for _, i := range neighborhoods[cell] {
		v := &points[i]
		if v.kind == 7 {
			continue
		}
		d := placement.Vector{v.v[0] - center[0], v.v[1] - center[1], v.v[2] - center[2]}
		squaredDistance := d.Dot(d)
		if squaredDistance > minerScanRadius*minerScanRadius {
			continue
		}
		var inside bool
		back := -d.Dot(forward)
		if large {
			inside = squaredDistance <= 100 && math.Abs(back) <= 7.75 && math.Abs(d.Dot(right)) <= 6.25
		} else {
			height := d.Dot(up)
			ground := math.Sqrt(max(0, squaredDistance-height*height))
			inside = ground > 0 && ground <= 7.75 && back/ground >= .73 && math.Abs(height) <= 2
		}
		if !inside {
			continue
		}
		if v.occupied || v.newMiners >= owners || v.item != int64(item) {
			return nil
		}
		ids = append(ids, i)
	}
	return ids
}

// minerTransport 职责：串接矿机出矿端、带链与星际塔；使用：已选真实矿机候选；边界：仅选择同纬带直线带路，吞吐受单带限制。
func (req *Request) minerTransport(miner Object, options ProductionOptions, occupied, minerals obstacleIndex, catalog *placement.Catalog, exporting bool, yaw, step float64) ([]Object, float64) {
	level := options.Setting.BeltLv
	if level == 0 {
		level = 3
	}
	capacity, err := transport.BeltCapacityPerMinute(level)
	if err != nil {
		return nil, 0
	}
	prefab, err := catalog.Geometry(miner.Building.ItemID, miner.Building.ModelIndex)
	if err != nil || len(prefab.PortPoses) == 0 {
		return nil, 0
	}
	port := 0
	if miner.Building.ItemID == 2316 {
		port = 1
	}
	portDistance := float64(prefab.PortPoses[port].Position[2])
	towerBuilding := blueprint.Building{ItemID: 2104, ModelIndex: 50}
	towerRadius := groundRadius(catalog, towerBuilding)
	beltRadius := recordRadius(catalog, blueprint.Building{ItemID: int16(2000 + level), ModelIndex: int16(34 + level)})
	latStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	east, north := math.Round(math.Sin(yaw*math.Pi/180)), math.Round(math.Cos(yaw*math.Pi/180))
	offset := func(value float64) Position {
		p := miner.First
		p.Longitude += east * step * value
		p.Latitude += north * float64(latStep) * value
		p.Height = 0
		return p
	}
	gridSize := 200 * (math.Abs(east)*math.Cos(miner.First.Latitude)*step + math.Abs(north)*float64(latStep))
	sourceOffset := max(1, int(math.Round(portDistance/gridSize)))
	stationSlot := 4
	if east < 0 {
		stationSlot = 10
	} else if north > 0 {
		stationSlot = 7
	} else if north < 0 {
		stationSlot = 1
	}
	// 密集矿区的近端塔位可能已被同区另一座塔占用，沿同一条出矿带继续核对远端塔位。
	lengths := []int{8, 10, 12, 15, 20, 25, 32, 40, 48, 56, 64, 80, 96}
	if !options.nearMineTowers || miner.Building.ItemID != 2301 {
		// 油井与大矿机的机身可能碰到八格外的塔，沿用各自已验证的接驳距离。
		lengths = lengths[3:]
	}
	for _, length := range lengths {
		if sourceOffset >= length-2 {
			continue
		}
		towerPoint := offset(float64(length))
		if !validPosition(towerPoint) {
			continue
		}
		pw, err := world(towerPoint)
		if err != nil || occupied.conflicts(pw, towerRadius) || occupied.stationConflict(pw, towerBuilding.ItemID) || minerals.conflicts(pw, groundRadius(catalog, towerBuilding)) {
			continue
		}
		towerIndex := miner.Building.Index + 1
		remote := 0
		if exporting {
			remote = 1
		}
		tower := stationObject(towerIndex, towerPoint, miner.Item, 1, remote, options.Setting.RequestStationWarpers)
		tower.Building.Parameters[192+stationSlot*4] = 2
		route := []Object{miner, tower}
		valid := true
		for col := sourceOffset; col <= length-2; col++ {
			point := offset(float64(col))
			if col == sourceOffset {
				point = offset(portDistance/gridSize - .025)
			} else if col == length-2 {
				point = offset(float64(col) + .1464)
			}
			w, err := world(point)
			if err != nil || occupied.conflicts(w, beltRadius) || minerals.conflicts(w, .45) {
				valid = false
				break
			}
			index := miner.Building.Index + int32(len(route))
			belt := blueprint.Building{Index: index, ItemID: int16(2000 + level), ModelIndex: int16(34 + level), Yaw: float32(yaw),
				InputObjIdx: -1, OutputObjIdx: index + 1, InputToSlot: 1, OutputFromSlot: 0}
			if col == sourceOffset {
				belt.InputObjIdx = miner.Building.Index
				belt.InputFromSlot = int8(port)
			}
			if col == length-2 {
				belt.OutputObjIdx, belt.OutputToSlot = towerIndex, int8(stationSlot)
				belt.Parameters = []int32{int32(miner.Item), 0}
			}
			route = append(route, Object{Building: belt, First: point, Second: point, Item: miner.Item, Module: "矿区接驳"})
		}
		if valid && occupied.surfaceRouteConflict(route, catalog) < 0 && !req.terrainConflict(route, catalog) {
			return route, capacity
		}
	}
	return nil, 0
}

// addActualPort 职责：合并受同一边界身份约束的实际流量；使用：矿区和制造共同输出；边界：不同对端从不合并。
func addActualPort(actual *industryplan.PlanetActual, next industryplan.Port) {
	for i := range actual.Ports {
		p := &actual.Ports[i]
		if p.Kind == next.Kind && p.ItemID == next.ItemID && p.PeerPlanetID == next.PeerPlanetID {
			p.RatePerMinute = next.RatePerMinute
			p.StorageItems = max(p.StorageItems, next.StorageItems)
			return
		}
	}
	actual.Ports = append(actual.Ports, next)
}
