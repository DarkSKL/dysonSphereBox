package build

// 文件职责：由有向带链计算监测器实际朝向，消除路径构造的默认Yaw残留。
// 使用：选点与最终球面记录；边界：只调整监测器及其宿主带，不转动生产建筑，不猜测断开的宿主。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
)

// alignFlowMonitorDirections 职责：按同区同点唯一宿主的实际输送切线同步朝向；使用：几何检查前；边界：每次重新计算，不把历史Yaw相等当作真实方向证明。
func (b *builder) alignFlowMonitorDirections(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building, updatePlaced bool) error {
	monitors := []int{}
	indices := map[int32]int{}
	previous := map[int32][]int{}
	for i, r := range records {
		indices[r.Index] = i
		if r.ItemID == 2030 {
			monitors = append(monitors, i)
		}
		if surfaceIsBelt(r.ItemID) {
			previous[r.OutputObjIdx] = append(previous[r.OutputObjIdx], i)
		}
	}
	if len(monitors) == 0 {
		return nil
	}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	for _, i := range monitors {
		if err := ctx.Err(); err != nil {
			return err
		}
		host := -1
		for j, r := range records {
			if surfaceIsBelt(r.ItemID) && r.AreaIndex == records[i].AreaIndex && r.LocalOffset == records[i].LocalOffset {
				if host >= 0 {
					return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "监测器对应多个宿主带"}
				}
				host = j
			}
		}
		if host < 0 {
			return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: fmt.Sprintf("监测器%d缺少宿主带", records[i].Index)}
		}
		point := func(index int) placement.Vector {
			return placement.Vector{world[index].x, world[index].y, world[index].z}
		}
		direction := placement.Vector{}
		if next, ok := indices[records[host].OutputObjIdx]; ok && surfaceIsBelt(records[next].ItemID) {
			direction = direction.Add(surfaceUnit(point(next).Sub(point(host))))
		}
		if before := previous[records[host].Index]; len(before) == 1 {
			direction = direction.Add(surfaceUnit(point(host).Sub(point(before[0]))))
		}
		frame, err := encoding.recordPose(world[host], 0, 0, 0)
		if err != nil {
			return err
		}
		x, y := direction.Dot(frame.axes[0]), direction.Dot(frame.axes[2])
		if math.Hypot(x, y) < 1e-6 {
			return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "监测器宿主没有可识别的输送方向"}
		}
		yaw := float32(math.Mod(math.Atan2(x, y)*180/math.Pi+360, 360))
		for _, j := range []int{i, host} {
			records[j].Yaw, records[j].Yaw2 = yaw, yaw
			if updatePlaced {
				b.all[j].record.Yaw, b.all[j].record.Yaw2 = yaw, yaw
			}
		}
	}
	if encoding.plan != nil {
		encoding.plan.CollisionCoverage = "none"
	}
	return nil
}
