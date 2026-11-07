package build

// 文件职责：在确定的原生纬度中对齐局部第三输入臂和同物料取货点。
// 使用：第4档局部单元验证；边界：当前不接入整厂生成，不改机器朝向、带路、碰撞阈值或设备等级。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// alignNativeThirdInputPorts 职责：按真实槽位消除第三输入臂的横向偏斜并重标定货物采样；使用：已定位的单Area局部单元；边界：完整校验成功才提交，失败恢复全部引用与端点。
func (b *builder) alignNativeThirdInputPorts(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (result []blueprint.Building, err error) {
	if encoding == nil || encoding.planar || encoding.plan == nil || encoding.plan.GridSegment != 200 || encoding.plan.PrimaryAreaIndex != 0 || len(encoding.areas) != 1 || encoding.relocation != nil || len(records) != len(b.all) || len(records) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "原生局部端口适配需要已定位的200分段单Area实体"}
	}
	if int(encoding.areas[0].AreaSegments) != longitudeSegmentCount(encoding.plan.AnchorLatitudeGrid, 200) || !candidateFitsRing(encoding.plan.AnchorLatitudeGrid, 0, int(encoding.areas[0].Height)-1, int(encoding.areas[0].Width), 200) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "原生局部单元的区域段数与实际纬度不匹配"}
	}
	for i, p := range b.all {
		r := records[i]
		if r.Index != int32(i) || r.AreaIndex != 0 || r.ItemID != p.record.ItemID || r.ModelIndex != p.record.ModelIndex || r.Yaw != p.record.Yaw || r.Yaw2 != p.record.Yaw2 || r.LocalOffset != (blueprint.Point{X: float32(p.a.x), Y: float32(p.a.y), Z: float32(p.a.z)}) || r.LocalOffset2 != (blueprint.Point{X: float32(p.b.x), Y: float32(p.b.y), Z: float32(p.b.z)}) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "原生端口记录与当前实体坐标不一致"}
		}
	}
	snapshots := make([]placed, len(b.all))
	for i, p := range b.all {
		snapshots[i] = *p
	}
	checks := 0
	if b.logistics != nil {
		checks = len(b.logistics.Checks)
	}
	defer func() {
		if err != nil {
			for i, p := range b.all {
				*p = snapshots[i]
			}
			if b.logistics != nil {
				b.logistics.Checks = b.logistics.Checks[:checks]
			}
		}
	}()
	result = append([]blueprint.Building(nil), records...)
	trial, plan := *encoding, *encoding.plan
	plan.Bands = append([]SurfaceBandPlan(nil), encoding.plan.Bands...)
	trial.plan = &plan
	world, err := trial.surfaceWorldPositions(result)
	if err != nil {
		return nil, err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	indices := map[int32]int{}
	for i, r := range records {
		indices[r.Index] = i
	}
	adjusted := 0
	for i, arm := range b.all {
		if !arm.densityThirdInput || !arm.densityCargo.enabled || arm.densityCargo.output {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r := &result[i]
		host, exists := indices[r.OutputObjIdx]
		if !exists || denseReferenceSource(int(result[host].ItemID)) != 2318 {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "原生第三输入臂需要已核对的制造台宿主"}
		}
		geometry, err := catalog.Geometry(result[host].ItemID, result[host].ModelIndex)
		if err != nil {
			return nil, err
		}
		slot := int(r.OutputToSlot)
		if slot < 0 || slot >= len(geometry.SlotPoses) {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "第三输入臂连接了无效机器槽位"}
		}
		base, err := trial.recordPose(world[host], result[host].Yaw, 0, 0)
		if err != nil {
			return nil, err
		}
		port := surfaceSlotPose(base, geometry.SlotPoses[slot], true)
		slotPoint := nativeSingleAreaPoint(&trial, port.position, float64(r.LocalOffset.X))
		northFrame, err := trial.recordPose(world[host], 0, 0, 0)
		if err != nil {
			return nil, err
		}
		old := r.LocalOffset
		if math.Abs(port.axes[2].Dot(northFrame.axes[2])) >= math.Abs(port.axes[2].Dot(northFrame.axes[0])) {
			r.LocalOffset.X = float32(slotPoint.x)
		} else {
			r.LocalOffset.Y = float32(slotPoint.y)
		}
		if r.LocalOffset.X < 0 || r.LocalOffset.Y < 0 || r.LocalOffset.X > float32(trial.areas[0].Width-1) || r.LocalOffset.Y > float32(trial.areas[0].Height-1) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "原生端口对齐后的端点超出局部单元包络"}
		}
		dx, dy := float64(r.LocalOffset.X-old.X), float64(r.LocalOffset.Y-old.Y)
		if math.Hypot(dx, dy) > 1 {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "原生第三输入臂所需横向修正超过一格"}
		}
		arm.a.x, arm.a.y = float64(r.LocalOffset.X), float64(r.LocalOffset.Y)
		arm.densityCargo.relative.x += dx
		arm.densityCargo.relative.y += dy
		adjusted++
	}
	if adjusted == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "局部单元缺少可适配的第三输入臂"}
	}
	if err = b.rebindReferenceCargo(ctx, &trial, result); err != nil {
		return nil, err
	}
	disabled := false
	if err = validateRelocatedSurface(ctx, Request{Setting: Setting{NeedPower: &disabled}}, &trial, result); err != nil {
		return nil, err
	}
	if _, err = validateFinalPlanetRules(ctx, Request{Mode: 1, Density: densityDense, Setting: Setting{NeedPower: &disabled}}, &trial, result); err != nil {
		return nil, err
	}
	if err = b.validateRelocatedCargoOffsets(&trial, result); err != nil {
		return nil, err
	}
	encoding.plan = trial.plan
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "native-third-input-port", Status: "passed", Message: fmt.Sprintf("%d根第三输入臂按原生机器槽位对齐并通过真实货物采样和几何检查", adjusted)})
	}
	return result, nil
}

// nativeSingleAreaPoint 职责：将真实槽位位置反算为当前单Area坐标；使用：局部端点对齐；边界：仅200分段球面，环向取与原端点最近的等价位置，不调整高度。
func nativeSingleAreaPoint(encoding *surfaceEncoding, point placement.Vector, nearX float64) vector {
	step := float32(2*float32(math.Pi)) / 1000
	latitude := float32(encoding.plan.AnchorLatitudeGrid) * step
	longitudeStep := surfacePasteLongitudeStep(latitude)
	x := math.Atan2(point[0], -point[2]) / float64(longitudeStep)
	period := 2 * math.Pi / float64(longitudeStep)
	x += math.Round((nearX-x)/period) * period
	y := (math.Atan2(point[1], math.Hypot(point[0], point[2])) - float64(latitude)) / float64(step)
	return vector{x: x, y: y}
}
