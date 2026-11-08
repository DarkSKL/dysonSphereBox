package build

// 文件职责：把已完成线路和供电的密铺几何迁移到所选区域，前三档保持建筑正交朝向。
// 使用：最终记录生成后、货物重绑前；边界：只改编码坐标，不扭曲内部装配视图，不放宽任一建造或能力阈值。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/density"
)

// denseSurfaceRelocation 职责：记录球面位置及整数原点变换；使用：编码实体和真实cargo目标采用同一映射；边界：仅原版200分段的单Area，内部指针与物料身份不变。
type denseSurfaceRelocation struct {
	sourceRecords                  []blueprint.Building
	sourceAnchor, targetAnchor     int
	sourceSegments, targetSegments int
	shiftX, shiftY                 float64
	safeAnchors                    []int
	// orthogonal 职责：沿纬线补偿横向格距并保持正交排向；使用：前三档；边界：变形后必须重新通过全部几何和能力校验。
	orthogonal bool
	centerX    float64
	height     float64
	halfTurn   bool
}

// mapPoint 职责：将位置转换到目标纬度，前三档按行补偿格距并保持正交朝向；使用：两端姿态及cargo目标；边界：高度独立于XY，堆叠和箱顶附件保持同轴，不修改pitch/tilt。
func (move *denseSurfaceRelocation) mapPoint(point blueprint.Point, yaw float32) (blueprint.Point, float32) {
	step := float32(2*float32(math.Pi)) / 1000
	if move.orthogonal {
		return move.mapOrthogonalPoint(point, yaw)
	}
	oldLongitude := point.X * (float32(2*float32(math.Pi)) / float32(move.sourceSegments*5))
	oldLatitude := float32(move.sourceAnchor)*step + point.Y*step
	cosLatitude := float32(math.Cos(float64(oldLatitude)))
	oldUp := surfaceUnit(placement.Vector{float64(cosLatitude * float32(math.Sin(float64(oldLongitude)))), float64(float32(math.Sin(float64(oldLatitude)))), float64(-cosLatitude * float32(math.Cos(float64(oldLongitude))))})
	delta := float64(move.targetAnchor-move.sourceAnchor) * float64(step)
	sine, cosine := math.Sin(delta), math.Cos(delta)
	rotate := func(p placement.Vector) placement.Vector {
		return placement.Vector{p[0], cosine*p[1] - sine*p[2], sine*p[1] + cosine*p[2]}
	}
	newUp := rotate(oldUp)
	oldEast := surfaceUnit(oldUp.Cross(placement.Vector{0, 1, 0}))
	newEast := surfaceUnit(newUp.Cross(placement.Vector{0, 1, 0}))
	newNorth := newEast.Cross(newUp)
	transportedNorth := rotate(oldEast.Cross(oldUp))
	yawShift := math.Atan2(transportedNorth.Dot(newEast), transportedNorth.Dot(newNorth)) * 180 / math.Pi
	longitude := math.Atan2(newUp[0], -newUp[2])
	latitude := math.Asin(max(-1., min(1., newUp[1])))
	newLongitudeStep := float64(float32(2*float32(math.Pi)) / float32(move.targetSegments*5))
	return blueprint.Point{X: float32(longitude/newLongitudeStep - move.shiftX), Y: float32(latitude/float64(step) - float64(move.targetAnchor) - move.shiftY), Z: point.Z}, float32(math.Mod(float64(yaw)+yawShift+360, 360))
}

// relocatedSurfaceRecords 职责：生成目标纬度的新记录和Area；使用：边缘与高纬候选；边界：不修改source或records，整数原点平移与建议纬度同步，变形后的几何须重新校验。
func relocatedSurfaceRecords(source *surfaceEncoding, records []blueprint.Building, target int, orthogonal, halfTurn bool) (*surfaceEncoding, []blueprint.Building, error) {
	if source == nil || source.planar || source.plan == nil || source.plan.GridSegment != 200 || len(source.areas) != 1 || source.plan.PrimaryAreaIndex != 0 || source.relocation != nil {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "球面位置适配只支持已核对的原版单Area"}
	}
	move := &denseSurfaceRelocation{sourceRecords: append([]blueprint.Building(nil), records...), sourceAnchor: source.plan.AnchorLatitudeGrid, targetAnchor: target, sourceSegments: longitudeSegmentCount(source.plan.AnchorLatitudeGrid, 200), targetSegments: longitudeSegmentCount(target, 200)}
	move.orthogonal, move.centerX = orthogonal, float64(source.areas[0].Width-1)/2
	move.halfTurn, move.height = halfTurn, float64(source.areas[0].Height)
	transformed := append([]blueprint.Building(nil), records...)
	view := make([]*placed, len(records))
	for i := range transformed {
		record := &transformed[i]
		if record.AreaIndex != 0 {
			return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "单Area重定位记录包含其它区域"}
		}
		record.LocalOffset, record.Yaw = move.mapPoint(record.LocalOffset, record.Yaw)
		record.LocalOffset2, record.Yaw2 = move.mapPoint(record.LocalOffset2, record.Yaw2)
		view[i] = &placed{record: *record, a: vector{float64(record.LocalOffset.X), float64(record.LocalOffset.Y), float64(record.LocalOffset.Z)}, b: vector{float64(record.LocalOffset2.X), float64(record.LocalOffset2.Y), float64(record.LocalOffset2.Z)}}
	}
	low, high := bounds(view)
	move.shiftX, move.shiftY = math.Floor(low.x), math.Floor(low.y)
	width, height := int(math.Ceil(high.x-move.shiftX))+1, int(math.Ceil(high.y-move.shiftY))+1
	anchor := target + int(move.shiftY)
	if width < 1 || height < 1 || width > 32767 || height > 32767 || longitudeSegmentCount(anchor, 200) != move.targetSegments || !candidateFitsRing(anchor, 0, height-1, width, 200) {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "球面位置适配后的完整几何无法放入同一原生纬带"}
	}
	for i := range transformed {
		transformed[i].LocalOffset.X -= float32(move.shiftX)
		transformed[i].LocalOffset2.X -= float32(move.shiftX)
		transformed[i].LocalOffset.Y -= float32(move.shiftY)
		transformed[i].LocalOffset2.Y -= float32(move.shiftY)
	}
	plan := *source.plan
	plan.AnchorLatitudeGrid, plan.SelectedLatitudeGrid = anchor, anchor
	plan.LatitudeDegrees = float64(anchor) * .36
	plan.ShiftedGridRows = anchor - plan.PreferredLatitudeGrid
	plan.LayoutWidth, plan.LayoutHeight = width, height
	plan.LongitudeSegments, plan.RingGridCells = move.targetSegments, move.targetSegments*5
	plan.Bands = []SurfaceBandPlan{{AreaIndex: 0, MinLatitudeGrid: anchor, MaxLatitudeGrid: anchor + height - 1, LongitudeSegments: move.targetSegments, BuildingCount: len(records)}}
	plan.BeltConnectionsChecked, plan.StaticCollidersChecked, plan.CollisionCoverage = false, false, "none"
	result := *source
	result.plan, result.anchorCandidates, result.relocation = &plan, nil, move
	result.areas = []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(move.targetSegments), Width: int16(width), Height: int16(height)}}
	return &result, transformed, nil
}

// validateRelocatedSurface 职责：复核最终实际碰撞、跨格能力及供电；使用：建议点和连续有效区间；边界：几何刚体不代表网格跨度不变，实际变长导致能力下降时明确拒绝。
func validateRelocatedSurface(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) error {
	if err := encoding.validateBeltDistancesContext(ctx, records); err != nil {
		return err
	}
	encoding.plan.BeltConnectionsChecked = true
	if err := encoding.validateStaticColliders(ctx, records); err != nil {
		return err
	}
	if err := encoding.validateDynamicFacilities(ctx, records); err != nil {
		return err
	}
	spans := map[int32]int32{}
	if _, err := encoding.surfaceInserterBoxesMeasured(ctx, records, spans); err != nil {
		return err
	}
	for _, record := range records {
		if reach, exists := spans[record.Index]; exists && (len(record.Parameters) == 0 || reach > record.Parameters[0]) {
			return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("球面重定位后分拣器%d实际跨格%d超过原能力配置%v", record.Index, reach, record.Parameters)}
		}
	}
	if req.Setting.NeedPower == nil || *req.Setting.NeedPower {
		networks, err := validateFinalPowerCoverage(ctx, encoding, records)
		if err != nil {
			return err
		}
		if networks > 1 {
			return &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: "球面重定位后内部电网不连通"}
		}
	}
	return nil
}

// relocateDenseSurface 职责：将已通过的密铺结果迁移到明确中部或南北边缘，并实测连续安全起点；使用：finish供电裁剪之后；边界：失败不退回同名选区的其它位置，普通和平面请求不变。
func (b *builder) relocateDenseSurface(ctx context.Context, req Request, source *surfaceEncoding, records []blueprint.Building) (*surfaceEncoding, []blueprint.Building, error) {
	if denseNativeLatitude(req) {
		return source, records, validateDensityLatitudePlan(req.SurfaceLayout, source.plan)
	}
	if req.Density != densityDense || req.SurfaceLayout == nil || req.SurfaceLayout.LatitudeZone == "" {
		return source, records, nil
	}
	band, _, err := densityLayoutBand(req.SurfaceLayout)
	if err != nil {
		return nil, nil, err
	}
	if source == nil || source.plan == nil || len(source.areas) != 1 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "所选密铺纬带缺少完整单Area几何"}
	}
	target, direction := band.high-int(source.areas[0].Height)+1, -1
	selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	center := selection.position == "center"
	orthogonal := selection.tier <= 3 && (selection.tier > 1 || !center)
	orientations := []bool{false}
	if orthogonal && req.Mode != 1 {
		// 塔与配送器没有固定左侧原料接口，可比较两个正交排向；北侧优先采用南侧布局的半周对称排向。
		preferHalfTurn := selection.position == "north"
		if selection.tier >= 2 && selection.tier <= 3 && selection.hemisphere < 0 {
			preferHalfTurn = selection.position != "south"
		}
		orientations = []bool{preferHalfTurn, !preferHalfTurn}
	}
	if center {
		target = bandCenter(band) - (int(source.areas[0].Height)-1)/2
	} else if selection.position == "south" {
		target, direction = band.low, 1
	}
	var last error
	for attempt := 0; attempt < 5*len(orientations); attempt++ {
		adjustment, halfTurn := attempt/len(orientations), orientations[attempt%len(orientations)]
		if ctx.Err() != nil {
			return nil, nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "球面重定位已取消或超时"}
		}
		candidateTarget := target + direction*adjustment
		if center {
			candidateTarget = target + []int{0, -1, 1, -2, 2}[adjustment]
		}
		encoding, result, err := relocatedSurfaceRecords(source, records, candidateTarget, orthogonal, halfTurn)
		if err != nil {
			last = err
			continue
		}
		if err = validateDensityLatitudePlan(req.SurfaceLayout, encoding.plan); err != nil {
			last = err
			continue
		}
		if !center {
			// 刚体投影会改变整数包络高度；先按最终包络补偿到所选分界线，再保留原候选。
			gap := band.high - encoding.plan.AnchorLatitudeGrid - int(encoding.areas[0].Height) + 1
			if direction > 0 {
				gap = encoding.plan.AnchorLatitudeGrid - band.low
			}
			if gap > 0 {
				aligned, alignedRecords, alignErr := relocatedSurfaceRecords(source, records, candidateTarget-direction*gap, orthogonal, halfTurn)
				if alignErr == nil && validateDensityLatitudePlan(req.SurfaceLayout, aligned.plan) == nil {
					if alignErr = validateRelocatedSurface(ctx, req, aligned, alignedRecords); alignErr == nil {
						return aligned, alignedRecords, nil
					}
				}
			}
		}
		if err = validateRelocatedSurface(ctx, req, encoding, result); err != nil {
			last = err
			continue
		}
		return encoding, result, nil
	}
	return nil, nil, fmt.Errorf("所选建造位置没有通过完整球面几何与运输能力检查：%w", last)
}

// validateRelocatedCargoOffsets 职责：验证固定offset及实际五样本取货窗口仍有合法物料身份和喷涂顺序；使用：有效落点区间；边界：不调用游戏自动纠偏，不把生成时.08米理想目标保真误作原生跨纬限制。
func (b *builder) validateRelocatedCargoOffsets(encoding *surfaceEncoding, records []blueprint.Building) error {
	if len(records) != len(b.all) {
		return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "落点范围的实体与记录数量不一致"}
	}
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	coaterPivots := map[*placed]density.CargoSample{}
	positions := make([]density.CargoPoint, len(world))
	indices := make(map[*placed]int, len(b.all))
	for i, p := range world {
		positions[i] = density.CargoPoint{p.x, p.y, p.z}
		indices[b.all[i]] = i
	}
	sampler, err := density.NewCargoSampler(records, positions)
	if err != nil {
		return err
	}
	scopes, err := b.denseMachineCargoScopes()
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		arm := scope.arm
		i := indices[arm]
		anchor, index, offset := arm.input, records[i].InputObjIdx, records[i].InputOffset
		if scope.output {
			anchor, index, offset = arm.output, records[i].OutputObjIdx, records[i].OutputOffset
		}
		j, ok := indices[anchor]
		if !ok || records[j].Index != index {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "落点范围检查缺少存活货物锚点"}
		}
		allowed := map[int32]bool{}
		for _, node := range scope.track {
			if n, alive := indices[node]; alive && surfaceIsBelt(records[n].ItemID) {
				allowed[records[n].Index] = true
			}
		}
		if !allowed[index] {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "落点范围的货物锚点越过物料轨道身份"}
		}
		var samples [5]density.CargoSample
		count := 5
		if scope.output {
			samples[0], err = sampler.ResolveAt(index, int(offset))
			count = 1
		} else {
			samples, err = sampler.PickupWindowAt(index, int(offset))
		}
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("落点范围内分拣器%d固定货物offset无效：%v", records[i].Index, err)}
		}
		for _, sample := range samples[:count] {
			if !allowed[sample.NodeIndex] {
				return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("分拣器%d的实际取放窗口跨出已证明物料轨道", records[i].Index)}
			}
		}
		if coater := scope.coater; coater != nil && !scope.output {
			pivot, known := coaterPivots[coater]
			if !known {
				host, hostErr := b.denseCargoCoaterHost(coater)
				if hostErr != nil {
					return hostErr
				}
				n, alive := indices[host]
				if !alive {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "取货窗口缺少真实喷涂宿主带"}
				}
				pivot, err = sampler.ResolveAt(records[n].Index, 0)
				if err != nil {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: fmt.Sprintf("喷涂枢轴无法沿实际货物路径定位：%v", err)}
				}
				coaterPivots[coater] = pivot
			}
			gap, orderErr := sampler.ForwardSampleDistance(pivot, samples[0])
			if orderErr != nil || gap <= 0 {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: fmt.Sprintf("分拣器%d的五样本取货窗口没有完整位于实际喷涂枢轴下游", records[i].Index)}
			}
		}
	}
	return nil
}

// finalizeDenseRelocationRange 职责：以最终引用、固定货物offset和完整供电证明连续可用起点；使用：cargo重绑之后；边界：首次失败即停止扩区，不把中间失败点跨过去拼成范围。
func (b *builder) finalizeDenseRelocationRange(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) error {
	if encoding == nil || (encoding.relocation == nil && !denseNativeLatitude(req)) {
		return nil
	}
	band, _, err := densityLayoutBand(req.SurfaceLayout)
	if err != nil {
		return err
	}
	check := func(candidate *surfaceEncoding) error {
		if err := validateRelocatedSurface(ctx, req, candidate, records); err != nil {
			return err
		}
		return b.validateRelocatedCargoOffsets(candidate, records)
	}
	if err := check(encoding); err != nil {
		return err
	}
	anchor := encoding.plan.AnchorLatitudeGrid
	safe := []int{anchor}
	for _, direction := range []int{-1, 1} {
		if encoding.rootCoordinates {
			break
		}
		for next := anchor + direction; next >= band.low && next+int(encoding.areas[0].Height)-1 <= band.high; next += direction {
			trial, plan := *encoding, *encoding.plan
			plan.AnchorLatitudeGrid, plan.BeltConnectionsChecked, plan.StaticCollidersChecked, plan.CollisionCoverage = next, false, false, "none"
			trial.plan = &plan
			if err := check(&trial); err != nil {
				if ctx.Err() != nil {
					return err
				}
				break
			}
			safe = append(safe, next)
		}
	}
	sort.Ints(safe)
	if encoding.relocation != nil {
		encoding.relocation.safeAnchors = safe
	}
	minimum := float64(safe[0]) * 360 / float64(encoding.plan.GridSegment*5)
	maximum := float64(safe[len(safe)-1]) * 360 / float64(encoding.plan.GridSegment*5)
	encoding.plan.SafeLatitudeMinDegrees, encoding.plan.SafeLatitudeMaxDegrees = &minimum, &maximum
	if b.logistics != nil {
		scopes, err := b.denseMachineCargoScopes()
		if err != nil {
			return err
		}
		rule, method := "dense-surface-rigid-relocation", "刚体迁移"
		if encoding.relocation == nil || encoding.relocation.orthogonal {
			rule, method = "dense-surface-orthogonal", "保持建筑朝向并补偿横向格距"
		}
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: rule, Status: "passed", Message: fmt.Sprintf("球面几何从纬圈%d到%d，%s；机器、带臂、塔距和真实线路重新校验；%d个带侧端点的固定offset及完整取货窗口、跨度能力和供电已核验连续起点%d..%d，范围外不承诺可用", nativeSourceAnchor(encoding), anchor, method, len(scopes), safe[0], safe[len(safe)-1])})
	}
	return nil
}

// relocatedProductionZones 职责：按最终编码坐标重建展示区域；使用：record层重定位之后；边界：仅复制坐标视图，不移动内部物料图或逻辑宿主。
func (b *builder) relocatedProductionZones(records []blueprint.Building) []Zone {
	view := *b
	view.all = make([]*placed, len(b.all))
	for i, original := range b.all {
		entity := *original
		entity.record = records[i]
		entity.a = vector{float64(records[i].LocalOffset.X), float64(records[i].LocalOffset.Y), float64(records[i].LocalOffset.Z)}
		entity.b = vector{float64(records[i].LocalOffset2.X), float64(records[i].LocalOffset2.Y), float64(records[i].LocalOffset2.Z)}
		view.all[i] = &entity
	}
	return view.finalProductionZones()
}

// nativeSourceAnchor 职责：提供诊断中的源锚点；使用：连续范围记录；边界：原生生成没有重定位来源。
func nativeSourceAnchor(encoding *surfaceEncoding) int {
	if encoding.relocation != nil {
		return encoding.relocation.sourceAnchor
	}
	return encoding.plan.AnchorLatitudeGrid
}
