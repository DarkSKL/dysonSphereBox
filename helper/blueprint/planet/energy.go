package planet

// 文件职责：装配光子、无燃料电源和双向电池物流；使用：特殊蓝图；边界：环境和接收条件作为明确施工前提。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// geothermalPoints 职责：恢复原版模型 255 的熔岩采样点；使用：落位和地基保护；边界：此处设备朝向为零。
func geothermalPoints(p Position) []Position {
	points := make([]Position, 0, 3)
	for _, v := range [][2]float64{{0, -1.3}, {.5, 1.3}, {-.5, 1.3}} {
		points = append(points, Position{Longitude: p.Longitude + v[0]/(200*math.Cos(p.Latitude)), Latitude: p.Latitude + v[1]/200})
	}
	return points
}

func geothermalSite(input ProductionRequest, p Position) bool {
	t := input.Resources.Terrain
	if t.WaterItemID == nil || *t.WaterItemID != -1 || t.WaterHeight == nil {
		return false
	}
	for _, sample := range geothermalPoints(p) {
		v, err := world(sample)
		if err != nil {
			return false
		}
		height, known := t.HeightAt(galaxy.Vector{X: float64(v.X), Y: float64(v.Y), Z: float64(v.Z)})
		if !known || *t.WaterHeight-height <= .32 {
			return false
		}
	}
	return true
}

// energyPortBlock 职责：连接接收器/枢纽的真实直连带口；使用：一个输入和一个输出；边界：端口来自对应原型，不使用分拣器代替。
func energyPortBlock(index int32, position Position, step float64, facility, mode, incoming, outgoing int, plan industryplan.PlanetPlan, catalog *placement.Catalog) ([]Object, error) {
	models := catalog.ModelsForItem(int16(facility))
	if len(models) == 0 {
		return nil, fmt.Errorf("缺少能源设施的模型")
	}
	prefab, err := catalog.Geometry(int16(facility), models[0])
	if err != nil {
		return nil, err
	}
	device := Object{First: position, Second: position, Module: "能源", Building: blueprint.Building{Index: index, ItemID: int16(facility), ModelIndex: models[0], Yaw: 90, Yaw2: 90, InputObjIdx: -1, OutputObjIdx: -1, Parameters: []int32{int32(mode)}}}
	objects := []Object{device}
	grid := 200 * math.Cos(position.Latitude) * step
	offset := func(x float64) Position { p := position; p.Longitude += x * step; return p }
	for _, io := range []struct {
		item, sign, port int
		input            bool
	}{{outgoing, 1, 0, false}, {incoming, -1, len(prefab.PortPoses) / 2, true}} {
		if io.item == 0 {
			continue
		}
		if io.port >= len(prefab.PortPoses) {
			return nil, fmt.Errorf("能源设施缺少输入输出带口")
		}
		portDistance := math.Abs(float64(prefab.PortPoses[io.port].Position[2]))
		if portDistance <= 0 {
			return nil, fmt.Errorf("能源设施的直连口方向无效")
		}
		towerIndex := index + int32(len(objects))
		local, remote := 1, 0
		if io.input {
			local = 2
		}
		for _, p := range plan.Ports {
			if p.ItemID == io.item && p.Kind == "import" && io.input {
				remote = 2
			}
			if p.ItemID == io.item && p.Kind == "export" && !io.input {
				remote = 1
			}
		}
		tower := stationObject(towerIndex, offset(float64(io.sign*18)), io.item, local, remote, false)
		stationPort := 4
		if io.input {
			stationPort = 10
			tower.Building.Parameters[192+stationPort*4] = 1
			tower.Building.Parameters[193+stationPort*4] = 1
		} else {
			tower.Building.Parameters[192+stationPort*4] = 2
		}
		objects = append(objects, tower)
		first := max(1, int(math.Round(portDistance/grid)))
		firstBelt := index + int32(len(objects))
		lastBelt := firstBelt + int32(16-first)
		for x := first; x <= 16; x++ {
			idx := index + int32(len(objects))
			p := offset(float64(io.sign * x))
			if x == first {
				p = offset(float64(io.sign) * (portDistance/grid - .025))
			}
			if x == 16 {
				p = offset(float64(io.sign) * (16 + .1464))
			}
			b := blueprint.Building{Index: idx, ItemID: 2003, ModelIndex: 37, Yaw: 90, InputObjIdx: -1, OutputObjIdx: idx + 1, InputToSlot: 1}
			if io.input {
				b.OutputObjIdx = idx - 1
				if idx == firstBelt {
					b.OutputObjIdx = index
					b.OutputToSlot = int8(io.port)
				}
				if idx == lastBelt {
					b.InputObjIdx = towerIndex
					b.InputFromSlot = int8(stationPort)
					b.Parameters = []int32{int32(io.item), 0}
				}
			} else {
				if idx == firstBelt {
					b.InputObjIdx = index
					b.InputFromSlot = int8(io.port)
				}
				if idx == lastBelt {
					b.OutputObjIdx = towerIndex
					b.OutputToSlot = int8(stationPort)
					b.Parameters = []int32{int32(io.item), 0}
				}
			}
			objects = append(objects, Object{First: p, Second: p, Item: io.item, Module: "能源物流", Building: b})
		}
	}
	return objects, nil
}

// placeEnergyUnits 职责：寻找设备和全部带路的空位；使用：风、日照、接收与充放电；边界：同类风机至少间隔原版要求距离。
func placeEnergyUnits(ctx context.Context, req *Request, input ProductionRequest, catalog *placement.Catalog, facility, count, mode, incoming, outgoing int) error {
	if facility == 2213 {
		req.geothermal = func(p Position) bool { return geothermalSite(input, p) }
	}
	models := catalog.ModelsForItem(int16(facility))
	if len(models) == 0 {
		return fmt.Errorf("缺少能源设施的模型")
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	firstCandidate, previousBatch := 0, 0
	for unit := 0; unit < count; {
		batch := 1
		if facility == 2208 && (incoming != 0 || outgoing != 0) {
			batch = min(3, count-unit)
		}
		if err := search.Checkpoint(ctx); err != nil {
			return err
		}
		// 同尺寸模块按固定顺序试摆。新建筑只会增加占用，前一批已排除的位置不必再生成带路。
		// 最后一批可能缩小，小模块仍从第一格寻找原来容不下整批的间隙。
		if previousBatch != batch {
			firstCandidate, previousBatch = 0, batch
		}
		candidate := 0
		next := nextObjectIndex(req.Objects)
		occupied := newObstacleIndex(append(extendOccupancy(*req, catalog), req.minerals...))
		placed := false
		for y := 0; y <= 210 && !placed; y += 10 {
			for _, sign := range []int{1, -1} {
				if y == 0 && sign < 0 {
					continue
				}
				lat := float64(float32(y*sign) * float32(latitudeStep))
				step, _ := geometry.PasteLongitudeStep32(geometry.Radians(lat), geometry.Vanilla200)
				width := int(math.Round(2 * math.Pi / float64(step)))
				for x := 0; x < width && !placed; x += 5 {
					candidate++
					if candidate <= firstCandidate {
						continue
					}
					if err := search.Checkpoint(ctx); err != nil {
						return err
					}
					p := Position{Latitude: lat, Longitude: float64(x) * float64(step)}
					objects := []Object{{First: p, Second: p, Module: "能源", Building: blueprint.Building{Index: next, ItemID: int16(facility), ModelIndex: models[0], InputObjIdx: -1, OutputObjIdx: -1, Parameters: []int32{int32(mode)}}}}
					if incoming != 0 || outgoing != 0 {
						var err error
						if facility == 2208 {
							objects, err = energyReceiverBatch(next, p, float64(step), batch, mode, incoming, outgoing, input.Plan, catalog)
						} else {
							objects, err = energyPortBlock(next, p, float64(step), facility, mode, incoming, outgoing, input.Plan, catalog)
						}
						if err != nil {
							return err
						}
					}
					if req.siteOccupied(ctx, objects, occupied, catalog) {
						continue
					}
					if facility == 2213 {
						if !geothermalSite(input, p) || req.capsuleConflict(objects, catalog) {
							continue
						}
					} else if req.terrainConflict(objects, catalog) {
						continue
					}
					if facility == 2203 || facility == 2213 {
						center, _ := world(p)
						tooClose := false
						minimum := 10.5
						if facility == 2213 {
							minimum = 12
						}
						for _, old := range req.Objects {
							if old.Building.ItemID != int16(facility) {
								continue
							}
							q, _ := world(old.First)
							if math.Hypot(math.Hypot(float64(center.X-q.X), float64(center.Y-q.Y)), float64(center.Z-q.Z)) < minimum {
								tooClose = true
								break
							}
						}
						for _, old := range input.Resources.Buildings {
							if old.ItemID == int64(facility) && math.Hypot(math.Hypot(float64(center.X)-old.Position.X, float64(center.Y)-old.Position.Y), float64(center.Z)-old.Position.Z) < minimum {
								tooClose = true
								break
							}
						}
						if tooClose {
							continue
						}
					}
					req.Objects = append(req.Objects, objects...)
					if facility == 2213 {
						for _, sample := range geothermalPoints(p) {
							req.Protected = append(req.Protected, Cell{Longitude: sample.Longitude, Latitude: sample.Latitude})
						}
					}
					placed = true
					firstCandidate = candidate
				}
			}
		}
		if !placed {
			return &Error{PlanetID: req.PlanetID, Item: facility, Code: "PLANET_POWER_SITE", Message: fmt.Sprintf("没有找到第 %d 台能源设施和物流的可用落点", unit+1)}
		}
		unit += batch
	}
	return nil
}

// creditEnergyPorts 职责：把实装设备的流量分配给尚未覆盖的边界；使用：避免制造和能源重复记账；边界：收发方向分别结算。
func creditEnergyPorts(plan industryplan.PlanetPlan, actual *industryplan.PlanetActual, item int, rate float64, input bool) {
	kind := "mining"
	if input {
		kind = "power-fuel"
	}
	total := rate
	for _, p := range actual.Ports {
		if p.Kind == kind && p.ItemID == item {
			total += p.RatePerMinute
		}
	}
	addActualPort(actual, industryplan.Port{ItemID: item, Kind: kind, RatePerMinute: total})
	for _, want := range plan.Ports {
		if want.ItemID != item || input && (want.Kind != "import" && want.Kind != "external") || !input && (want.Kind != "export" && want.Kind != "delivery" && want.Kind != "storage" && want.Kind != "external-export") {
			continue
		}
		covered := 0.0
		for _, old := range actual.Ports {
			if old.ItemID == item && old.Kind == want.Kind && old.PeerPlanetID == want.PeerPlanetID {
				covered = old.RatePerMinute
			}
		}
		use := min(rate, max(0, want.RatePerMinute-covered))
		rate -= use
		if use > 0 {
			p := want
			p.RatePerMinute = covered + use
			// 这里只核对带口流量，仓储容量由后续真实仓储塔补足。
			p.StorageItems = 0
			addActualPort(actual, p)
		}
	}
}

func appendPhotons(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	p := input.Plan.Photons
	if p == nil {
		return nil
	}
	lens := 0
	if p.LensRatePerMinute > 0 {
		lens = 1209
	}
	if err := placeEnergyUnits(ctx, req, input, catalog, 2208, p.Count, 1208, lens, 1208); err != nil {
		return &industryplan.SourcePlacementError{ItemID: 1208, Err: err}
	}
	creditEnergyPorts(input.Plan, actual, 1208, p.RatePerMinute, false)
	if lens != 0 {
		creditEnergyPorts(input.Plan, actual, lens, p.LensRatePerMinute, true)
	}
	return nil
}

// appendEnergy 职责：装配混合能源合同并按实际电网复核；使用：制造之后；边界：每个电池充电端同样必须获得电源。
func appendEnergy(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	power := input.Plan.Power
	if power == nil || len(power.Sources) == 0 && power.ExternalGridMW == 0 {
		return appendPower(ctx, req, input, actual, catalog)
	}
	req.externalGrid = power.ExternalGridMW > 0
	if req.externalGrid {
		actual.Warnings = append(actual.Warnings, fmt.Sprintf("将蓝图电网接入已有电网，已声明可用余量 %.3f MW", power.ExternalGridMW))
	}
	var fuel *industryplan.PowerSource
	for i := range power.Sources {
		source := &power.Sources[i]
		switch source.FacilityID {
		case 2204, 2210, 2211:
			fuel = source
			continue
		}
		mode, incoming, outgoing := 0, 0, 0
		if source.FacilityID == 2208 {
			incoming = source.FuelItemID
		}
		if source.FacilityID == 2209 {
			mode = -1
			incoming, outgoing = 2207, 2206
			if source.Mode == "charge" {
				mode = 1
				incoming, outgoing = 2206, 2207
			}
		}
		if err := placeEnergyUnits(ctx, req, input, catalog, source.FacilityID, source.Count, mode, incoming, outgoing); err != nil {
			return err
		}
		if source.FacilityID == 2209 {
			rate := float64(source.Count) * 6
			creditEnergyPorts(input.Plan, actual, incoming, rate, true)
			creditEnergyPorts(input.Plan, actual, outgoing, rate, false)
		} else if incoming > 0 {
			creditEnergyPorts(input.Plan, actual, incoming, source.FuelRatePerMinute, true)
		}
	}
	if fuel != nil {
		copy := input
		one := *power
		one.FacilityID = fuel.FacilityID
		one.FuelItemID = fuel.FuelItemID
		one.Count = fuel.Count
		one.FuelRatePerMinute = fuel.FuelRatePerMinute
		one.PowerMW = fuel.PowerMW
		copy.Plan.Power = &one
		if err := appendPower(ctx, req, copy, actual, catalog); err != nil {
			return err
		}
	} else {
		if err := appendTransit(ctx, req, input, actual, catalog); err != nil {
			return err
		}
		if err := appendFleetStations(ctx, req, input, catalog, actual); err != nil {
			return err
		}
		if err := accountLocalFreight(req, input, actual); err != nil {
			return err
		}
		facts, err := facts.Load()
		if err != nil {
			return err
		}
		if err := connectPowerGrid(ctx, req, catalog, facts); err != nil {
			return err
		}
		draw := actualElectricalDemand(*req, input.Plan, facts)
		if draw > power.PowerMW+power.ExternalGridMW+1e-6 {
			return &industryplan.PowerLoadError{RequiredMW: draw, Err: &Error{PlanetID: req.PlanetID, Code: "PLANET_POWER", Message: "实装辅助设备耗电超出供电能力，需要增加电源"}}
		}
	}
	copy := *power
	copy.Sources = append([]industryplan.PowerSource(nil), power.Sources...)
	// 合同只在对应模式的设备实际存在时成立，不能由计划回填掩盖漏装。
	type sourceKey struct{ facility, mode int }
	wanted, installed := map[sourceKey]int{}, map[sourceKey]int{}
	for _, source := range power.Sources {
		mode := 0
		if source.FacilityID == 2209 {
			mode = -1
			if source.Mode == "charge" {
				mode = 1
			}
		}
		wanted[sourceKey{source.FacilityID, mode}] += source.Count
	}
	for _, object := range req.Objects {
		mode := 0
		if (object.Building.ItemID == 2209 || object.Building.ItemID == 2208) && len(object.Building.Parameters) > 0 {
			mode = int(object.Building.Parameters[0])
		}
		installed[sourceKey{int(object.Building.ItemID), mode}]++
	}
	for key, count := range wanted {
		if installed[key] < count {
			return &Error{PlanetID: req.PlanetID, Code: "PLANET_POWER", Message: "实装发电或储能设备没有满足供电方案"}
		}
	}
	factCatalog, err := facts.Load()
	if err != nil {
		return err
	}
	copy.RequiredMW = actualElectricalDemand(*req, input.Plan, factCatalog)
	if copy.RequiredMW > copy.PowerMW+copy.ExternalGridMW+1e-6 {
		return &industryplan.PowerLoadError{RequiredMW: copy.RequiredMW, Err: &Error{PlanetID: req.PlanetID, Code: "PLANET_POWER", Message: "实装设备及充电端负荷超出供电能力"}}
	}
	actual.Power = &copy
	if copy.ExternalGridMW > 0 {
		req.requiredGridMW = max(0, copy.RequiredMW-copy.PowerMW)
	}
	return nil
}
