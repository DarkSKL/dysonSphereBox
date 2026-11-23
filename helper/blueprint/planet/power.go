package planet

// 文件职责：装配有实际燃料路径的原版电厂；使用：逐球供电合同；边界：按未喷涂额定能力验证，机队及启动用电仍需实际配置。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/transport"
)

// appendPower 职责：验证并落位按原版燃料工作的发电机；使用：有供电合同的星球；边界：整星失败时不交付部分电厂。
func appendPower(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	power := input.Plan.Power
	if power == nil {
		if err := appendTransit(ctx, req, input, actual, catalog); err != nil {
			return err
		}
		if err := appendFleetStations(ctx, req, input, catalog, actual); err != nil {
			return err
		}
		if err := accountLocalFreight(req, input, actual); err != nil {
			return err
		}
		factCatalog, err := facts.Load()
		if err != nil {
			return err
		}
		req.requiredGridMW = actualElectricalDemand(*req, input.Plan, factCatalog)
		if input.Options.Setting.NeedPower == nil || *input.Options.Setting.NeedPower {
			actual.Warnings = append(actual.Warnings, "当前规划需要玩家接入可用电网")
		}
		return nil
	}
	fail := func(message string) error {
		return &Error{PlanetID: req.PlanetID, Item: power.FuelItemID, Module: "燃料电厂", Code: "PLANET_POWER", Message: message}
	}
	if power.Count <= 0 || !(power.FuelRatePerMinute > 0) || !(power.PowerMW > 0) {
		return fail("供电合同缺少有效的设备数量、燃料供给或发电容量")
	}
	factCatalog, err := facts.Load()
	if err != nil {
		return err
	}
	generator, gok := factCatalog.Lookup(power.FacilityID)
	fuel, fok := factCatalog.Lookup(power.FuelItemID)
	if !gok || !fok {
		return fail("电厂或燃料无已核对的原版数据")
	}
	ratedMW, ratedFuel, err := fuelGeneratorRating(generator, fuel)
	if err != nil {
		return fail(err.Error())
	}
	if float64(power.Count)*ratedMW+1e-6 < power.PowerMW {
		return fail("发电设备额定容量低于逐星合同")
	}
	if power.FuelRatePerMinute+1e-6 < float64(power.Count)*ratedFuel {
		return fail(fmt.Sprintf("%s供给低于合同所列%s的额定消耗", fuel.Name, generator.Name))
	}
	sorterLevel := input.Options.Setting.InserterLv
	if sorterLevel == 0 {
		sorterLevel = 3
	}
	perGenerator := power.FuelRatePerMinute / float64(power.Count)
	armCapacity, err := transport.InserterCapacityPerMinute(sorterLevel, 1)
	if err != nil || armCapacity+1e-6 < perGenerator {
		return fail(fmt.Sprintf("单台%s所需 %.3f 件%s/分钟超出选用的分拣器容量", generator.Name, perGenerator, fuel.Name))
	}
	beltCapacity, err := transport.BeltCapacityPerMinute(1)
	if err != nil || beltCapacity+1e-6 < perGenerator {
		return fail(fmt.Sprintf("单台%s所需 %.3f 件%s/分钟超出燃料输入带容量", generator.Name, perGenerator, fuel.Name))
	}
	remote := 0
	for _, p := range input.Plan.Ports {
		if p.ItemID == power.FuelItemID && p.Kind == "import" {
			remote = 2
			break
		}
	}
	batchLimit := min(8, int(math.Floor((beltCapacity+1e-6)/perGenerator)))
	for i := 0; i < power.Count; {
		if err := ctx.Err(); err != nil {
			return err
		}
		occupied := newObstacleIndex(append(extendOccupancy(*req, catalog), req.minerals...))
		startIndex := nextObjectIndex(req.Objects)
		placed := 0
		gridStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
		// 优先共塔，整组无空位时缩小到单台；不因狭窄空隙放弃原有可用落点。
		for batch := min(batchLimit, power.Count-i); batch >= 1 && placed == 0; batch /= 2 {
			for _, y := range []int{0, 10, -10, 35, -35, 65, -65, 110, -110, 160, -160} {
				latitude := float64(float32(y) * float32(gridStep))
				step, _ := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
				width := int(math.Round(2 * math.Pi / float64(step)))
				for x := 0; x < width-12; x += 5 {
					if err := ctx.Err(); err != nil {
						return err
					}
					p := Position{Latitude: latitude, Longitude: float64(x) * float64(step)}
					layout, err := fuelBatch(startIndex, p, float64(step), batch, generator, int32(power.FuelItemID), sorterLevel, remote, input.Options.Setting.RequestStationWarpers, catalog)
					if err != nil {
						return fail(err.Error())
					}
					if req.terrainConflict(layout, catalog) || placementConflict(layout, occupied, catalog) {
						continue
					}
					valid := true
					for _, object := range layout {
						if object.Building.ItemID != int16(2010+sorterLevel) {
							continue
						}
						capacity, capacityErr := transport.InserterCapacityPerMinute(sorterLevel, int(object.Building.Parameters[0]))
						if capacityErr != nil || capacity+1e-6 < perGenerator {
							valid = false
							break
						}
					}
					if !valid {
						continue
					}
					if _, err := Build(ctx, Request{PlanetID: req.PlanetID, Objects: layout}); err != nil {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						continue
					}
					req.Objects = append(req.Objects, layout...)
					req.localPorts = append(req.localPorts, build.BoundaryPort{BuildingIndex: int(layout[0].Building.Index),
						ItemID: power.FuelItemID, Kind: "external-input", RatePerMinute: float64(batch) * perGenerator})
					placed = batch
					break
				}
				if placed != 0 {
					break
				}
			}
		}
		if placed == 0 {
			available := installedPowerCapacity(*req, power, i, ratedMW)
			draw := actualElectricalDemand(*req, input.Plan, factCatalog)
			return &industryplan.PowerLoadError{RequiredMW: max(power.RequiredMW, draw), AvailableMW: &available,
				Err: fail(fmt.Sprintf("已有电站和存量占用后缺少第 %d 台%s和燃料输入带的空地", i+1, generator.Name))}
		}
		i += placed
	}
	creditEnergyPorts(input.Plan, actual, power.FuelItemID, power.FuelRatePerMinute, true)
	if err := appendTransit(ctx, req, input, actual, catalog); err != nil {
		return err
	}
	if err := appendFleetStations(ctx, req, input, catalog, actual); err != nil {
		return err
	}
	if err := accountLocalFreight(req, input, actual); err != nil {
		return err
	}
	if err := connectPowerGrid(ctx, req, catalog, factCatalog); err != nil {
		return err
	}
	draw := actualElectricalDemand(*req, input.Plan, factCatalog)
	available := ratedMW * float64(power.Count)
	if draw > available+1e-6 {
		return &industryplan.PowerLoadError{RequiredMW: draw, Err: fail(fmt.Sprintf("实际已落位设施的持续额定负荷 %.3f MW 超出 %.3f MW，需增加电厂和燃料", draw, available))}
	}
	actual.Power = &industryplan.PowerPlan{FacilityID: power.FacilityID, FuelItemID: power.FuelItemID, Count: power.Count,
		PowerMW: available, FuelRatePerMinute: ratedFuel * float64(power.Count), RequiredMW: draw}
	actual.Warnings = append(actual.Warnings, fmt.Sprintf("%s已装配 %d 台，可供 %.3f MW，持续额定负荷 %.3f MW，消耗%s %.3f 件/分钟。启动时需接电并配置物流机队。", generator.Name, power.Count, available, draw, fuel.Name, ratedFuel*float64(power.Count)))
	return nil
}

// installedPowerCapacity 职责：记录燃料机组空间失败前实际放下的供电能力；使用：按容量迁移制造负荷；边界：其他电源只计匹配模式的已落位实体，外接电网沿用玩家声明，部分电厂仍不作为成功蓝图交付。
func installedPowerCapacity(req Request, power *industryplan.PowerPlan, fuelCount int, ratedMW float64) float64 {
	available := float64(fuelCount)*ratedMW + power.ExternalGridMW
	type sourceKey struct{ facility, mode int }
	installed := map[sourceKey]int{}
	for _, object := range req.Objects {
		mode := 0
		if (object.Building.ItemID == 2209 || object.Building.ItemID == 2208) && len(object.Building.Parameters) > 0 {
			mode = int(object.Building.Parameters[0])
		}
		installed[sourceKey{int(object.Building.ItemID), mode}]++
	}
	for _, source := range power.Sources {
		if source.FacilityID == power.FacilityID || source.PowerMW <= 0 || source.Count <= 0 {
			continue
		}
		mode := 0
		if source.FacilityID == 2209 {
			mode = -1
			if source.Mode == "charge" {
				mode = 1
			}
		}
		key := sourceKey{source.FacilityID, mode}
		count := min(installed[key], source.Count)
		available += source.PowerMW * float64(count) / float64(source.Count)
		installed[key] -= count
	}
	return available
}

// fuelGeneratorRating 职责：核对燃料类别并换算电厂额定；使用：燃料模块与供给校验；边界：只覆盖原版燃料电厂和未喷涂燃料，金色燃料棒按游戏规则翻倍功率。
func fuelGeneratorRating(generator, fuel facts.Item) (ratedMW, ratedFuel float64, err error) {
	switch generator.ID {
	case 2204, 2211, 2210:
	default:
		return 0, 0, fmt.Errorf("燃料电厂模块不支持所选发电设施")
	}
	power := generator.Power
	if !generator.PowerKnown || power == nil || !power.Generator || power.Generation <= 0 || power.FuelUse <= 0 || fuel.HeatValue <= 0 || power.FuelMask&fuel.FuelType == 0 {
		return 0, 0, fmt.Errorf("所选燃料与电厂的原版燃料类别或额定数据不匹配")
	}
	factor := 1.0
	if generator.ID == 2210 && fuel.ID == 1804 {
		factor = 2
	}
	return float64(power.Generation) * .00006 * factor, float64(power.FuelUse) * 3600 * factor / float64(fuel.HeatValue), nil
}

// actualElectricalDemand 从落位实体重算持续耗电，制造采用含喷涂的额定负荷，物流塔的充电上限在冻结图纸时单独统计。
func actualElectricalDemand(req Request, plan industryplan.PlanetPlan, catalog *facts.Catalog) float64 {
	draw := req.dronePowerMW + plan.OutboundVesselMW
	for _, production := range plan.Plan.Productions {
		if production.Capacity == nil {
			draw += production.PowerMW
		} else {
			draw += production.Capacity.PowerMW
		}
	}
	for _, object := range req.Objects {
		b := object.Building
		if b.RecipeID > 0 {
			continue
		}
		fact, ok := catalog.Lookup(int(b.ItemID))
		if !ok || fact.Power == nil {
			continue
		}
		rate := float64(fact.Power.Work) * .00006
		switch b.ItemID {
		case 2103, 2104:
			// 原版 SetPCState 满电仍取待机功率；InternalTickLocal 每 tick 扣除对应的 1000 J。
			draw += float64(fact.Power.Idle) * .00006
		case 2209:
			if len(b.Parameters) > 0 && b.Parameters[0] == 1 {
				draw += float64(fact.Power.ExchangeEnergyPerTick) * .00006
			}
		case 2316:
			multiplier := float64(b.Parameters[329]) / 10000
			draw += rate * multiplier * multiplier
		default:
			draw += rate
		}
	}
	return draw
}

// thermalBlock 职责：保留既有火电模块入口；使用：原有布局回归；边界：目录失败返回空模块，正式装配使用可返回错误的fuelBlock。
func thermalBlock(index int32, start Position, step float64, fuel int32, sorterLevel, remote int, warpers bool) []Object {
	factCatalog, err := facts.Load()
	if err != nil {
		return nil
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil
	}
	generator, _ := factCatalog.Lookup(2204)
	objects, _ := fuelBlock(index, start, step, generator, fuel, sorterLevel, remote, warpers, catalog)
	return objects
}

// fuelBlock 职责：生成燃料塔、输入带、分拣器和电厂原版实体；使用：远离存量的同纬带；边界：零度电厂取朝西居中的真实插口，不代用火电几何。
func fuelBlock(index int32, start Position, step float64, facility facts.Item, fuel int32, sorterLevel, remote int, warpers bool, catalog *placement.Catalog) ([]Object, error) {
	ports, err := catalog.Geometry(int16(facility.ID), int16(facility.ModelIndex))
	if err != nil {
		return nil, err
	}
	intakeSlot := -1
	for i, pose := range ports.SlotPoses {
		if pose.Position[0] < 0 && math.Abs(float64(pose.Position[2])) < 1e-6 {
			intakeSlot = i
			break
		}
	}
	if intakeSlot < 0 {
		return nil, fmt.Errorf("%s缺少已核对的朝西燃料插口", facility.Name)
	}
	offset := func(x float64) Position {
		p := start
		p.Longitude += x * step
		return p
	}
	object := func(b blueprint.Building, a, z Position) Object {
		return Object{Building: b, First: a, Second: z, Item: int(fuel), Module: "燃料电厂"}
	}
	tower := stationObject(index, start, int(fuel), 2, remote, warpers)
	tower.Building.Parameters[192+10*4], tower.Building.Parameters[193+10*4] = 1, 1
	genIndex := index + 1
	generator := object(blueprint.Building{Index: genIndex, ItemID: int16(facility.ID), ModelIndex: int16(facility.ModelIndex), InputObjIdx: -1, OutputObjIdx: -1}, offset(12), offset(12))
	result := []Object{tower, generator}
	var lastBelt int32
	for cell := 2; cell <= 9; cell++ {
		beltIndex := index + int32(len(result))
		belt := blueprint.Building{Index: beltIndex, ItemID: 2001, ModelIndex: 35, InputObjIdx: -1, OutputObjIdx: beltIndex + 1, InputToSlot: 1, Yaw: 90}
		pos := offset(float64(cell))
		if cell == 2 {
			belt.InputObjIdx, belt.InputFromSlot = index, 10
			belt.Parameters = []int32{fuel, 0}
			pos = offset(2 - .1464)
		}
		if cell == 9 {
			belt.OutputObjIdx = -1
		}
		result = append(result, object(belt, pos, pos))
		lastBelt = beltIndex
	}
	model := int16(40 + sorterLevel)
	if sorterLevel == 4 {
		model = 483
	}
	from := offset(9)
	to, err := fuelIntakePosition(generator.First, ports.SlotPoses[intakeSlot])
	if err != nil {
		return nil, err
	}
	arm := blueprint.Building{Index: lastBelt + 1, ItemID: int16(2010 + sorterLevel), ModelIndex: model, Yaw: 90, Yaw2: 90,
		InputObjIdx: lastBelt, InputFromSlot: -1, OutputObjIdx: genIndex, OutputToSlot: int8(intakeSlot), InputToSlot: 1, FilterID: int16(fuel), Parameters: []int32{1}}
	result = append(result, object(arm, from, to))
	pole := offset(6)
	pole.Latitude += 3 * step
	result = append(result, object(blueprint.Building{Index: lastBelt + 2, ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}, pole, pole))
	return result, nil
}

// fuelBatch 职责：让同燃料电厂共用一座塔和连续带路；使用：最多八台的两列机组；边界：各台保持独立分拣器，单台沿用已验证模块，吞吐由装配入口核对。
func fuelBatch(index int32, start Position, step float64, count int, facility facts.Item, fuel int32, sorterLevel, remote int, warpers bool, catalog *placement.Catalog) ([]Object, error) {
	if count < 1 || count > 8 {
		return nil, fmt.Errorf("共享燃料模块须包含一至八台电厂")
	}
	if count == 1 {
		return fuelBlock(index, start, step, facility, fuel, sorterLevel, remote, warpers, catalog)
	}
	ports, err := catalog.Geometry(int16(facility.ID), int16(facility.ModelIndex))
	if err != nil {
		return nil, err
	}
	slots := [2]int{-1, -1}
	for i, pose := range ports.SlotPoses {
		if math.Abs(float64(pose.Position[2])) > 1e-6 {
			continue
		}
		if pose.Position[0] < 0 {
			slots[0] = i
		} else if pose.Position[0] > 0 {
			slots[1] = i
		}
	}
	if slots[0] < 0 || slots[1] < 0 {
		return nil, fmt.Errorf("%s缺少已核对的东西燃料插口", facility.Name)
	}
	latitudeStep, err := geometry.LatitudeStep32(geometry.Vanilla200)
	if err != nil {
		return nil, err
	}
	direction := 1.0
	if start.Latitude < 0 {
		direction = -1
	}
	offset := func(x, row float64) Position {
		return Position{Longitude: start.Longitude + x*step, Latitude: start.Latitude + direction*row*float64(latitudeStep), Height: start.Height}
	}
	object := func(b blueprint.Building, a, z Position) Object {
		return Object{Building: b, First: a, Second: z, Item: int(fuel), Module: "燃料电厂"}
	}
	tower := stationObject(index, start, int(fuel), 2, remote, warpers)
	tower.Building.Parameters[192+10*4], tower.Building.Parameters[193+10*4] = 1, 1
	objects := []Object{tower}
	// 共用一条燃料带的两台电厂各取一格，避免两把分拣器的原版取料碰撞体重合。
	generatorRow := func(i int) int { return 8 + 8*(i/2) + i%2 }
	for i := 0; i < count; i++ {
		sign := 1.0
		if i%2 != 0 {
			sign = -1
		}
		slot := ports.SlotPoses[slots[i%2]]
		row := float64(generatorRow(i))
		latitude := offset(9, row).Latitude
		width := 200 * math.Cos(latitude) * step
		gap := math.Ceil((math.Abs(float64(slot.Position[0])) + 2.4) / width)
		if facility.ID == 2204 && fuel == 1006 && sorterLevel == 1 {
			// 火电东西插口不对称。按该行实际经格让两侧各跨 1.2 格，处于一级爪单格范围中间。
			rowStep, err := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
			if err != nil {
				return nil, err
			}
			gap = math.Abs(float64(slot.Position[0]))/width + 1.2*float64(rowStep)/step
		}
		p := offset(9+sign*gap, row)
		objects = append(objects, object(blueprint.Building{Index: index + int32(len(objects)), ItemID: int16(facility.ID), ModelIndex: int16(facility.ModelIndex), InputObjIdx: -1, OutputObjIdx: -1}, p, p))
	}
	points := []Position{offset(2-.1464, 0)}
	for x := 3; x <= 9; x++ {
		points = append(points, offset(float64(x), 0))
	}
	lastRow := generatorRow(count - 1)
	for row := 1; row <= lastRow; row++ {
		points = append(points, offset(9, float64(row)))
	}
	firstBelt := index + int32(len(objects))
	for i, p := range points {
		idx := firstBelt + int32(i)
		b := blueprint.Building{Index: idx, ItemID: 2001, ModelIndex: 35, InputObjIdx: -1, OutputObjIdx: idx + 1, InputToSlot: 1}
		if i == 0 {
			b.InputObjIdx, b.InputFromSlot, b.Parameters = index, 10, []int32{fuel, 0}
		}
		if i == len(points)-1 {
			b.OutputObjIdx = -1
		}
		from, to := p, p
		if i+1 < len(points) {
			to = points[i+1]
		} else {
			from = points[i-1]
		}
		b.Yaw = float32(math.Atan2((to.Longitude-from.Longitude)*math.Cos(p.Latitude), to.Latitude-from.Latitude) * 180 / math.Pi)
		b.Yaw2 = b.Yaw
		objects = append(objects, object(b, p, p))
	}
	model := int16(40 + sorterLevel)
	if sorterLevel == 4 {
		model = 483
	}
	for i := 0; i < count; i++ {
		generator := objects[1+i]
		slot := slots[i%2]
		to, err := fuelIntakePosition(generator.First, ports.SlotPoses[slot])
		if err != nil {
			return nil, err
		}
		row := generatorRow(i)
		yaw := float32(90)
		if i%2 != 0 {
			yaw = -90
		}
		from := offset(9, float64(row))
		center, _ := world(generator.First)
		first, _ := world(from)
		second, _ := world(to)
		across, err := geometry.SegmentsAcross32(center, first, second)
		if err != nil {
			return nil, err
		}
		span := int32(math.RoundToEven(float64(max(float32(1), min(float32(3), float32(across))))))
		b := blueprint.Building{Index: index + int32(len(objects)), ItemID: int16(2010 + sorterLevel), ModelIndex: model, Yaw: yaw, Yaw2: yaw,
			InputObjIdx: firstBelt + int32(7+row), InputFromSlot: -1, OutputObjIdx: generator.Building.Index, OutputToSlot: int8(slot), InputToSlot: 1, FilterID: int16(fuel), Parameters: []int32{span}}
		objects = append(objects, object(b, from, to))
	}
	return objects, nil
}

// fuelIntakePosition 职责：把东西插口的横向与高度偏移投到球面；使用：零度燃料电厂分拣器终点；边界：调用方已选择局部z为零的插口。
func fuelIntakePosition(center Position, slot placement.Pose) (Position, error) {
	point, err := world(center)
	if err != nil {
		return Position{}, err
	}
	x, y := float64(slot.Position[0]), float64(slot.Position[1])
	sinLon, cosLon := math.Sincos(center.Longitude)
	sinLat, cosLat := math.Sincos(center.Latitude)
	point.X += geometry.WorldLength(x*cosLon + y*cosLat*sinLon)
	point.Y += geometry.WorldLength(y * sinLat)
	point.Z += geometry.WorldLength(x*sinLon - y*cosLat*cosLon)
	position, err := geometry.WorldToSurface(point)
	if err != nil {
		return Position{}, err
	}
	return Position{Longitude: float64(position.Longitude), Latitude: float64(position.Latitude), Height: float64(position.Height)}, nil
}
