package build

// 文件职责：在真实原生纬度内以最小端点改动修正参考分拣器。
// 使用：高纬补电前；边界：机器与带路固定，货物取放位置不变，仍核验真实跨度、角度及动态盒。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
	"sort"
)

// fitNativeInserterEndpoints 职责：为每根参考臂选择最小可建端点修正；使用：原生候选；边界：不扩大碰撞容差、不提升设备能力，整厂校验失败由调用方回滚。
func (b *builder) fitNativeInserterEndpoints(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (int, error) {
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return 0, err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return 0, err
	}
	indices := map[int32]int{}
	for i, r := range records {
		indices[r.Index] = i
	}
	occupied := []placement.OwnedBox{}
	adjusted := 0
	for i, arm := range b.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
			continue
		}
		r := records[i]
		input, ok1 := indices[r.InputObjIdx]
		output, ok2 := indices[r.OutputObjIdx]
		if !ok1 || !ok2 {
			return 0, fmt.Errorf("分拣器%d缺少原生连接宿主", r.Index)
		}
		candidates := []blueprint.Building{r}
		// 普通机器长臂保留实际带节点及货物offset，只调整动画带端在本格内的纵向落点。
		// 端口校正、跨度和真实货物链随后完整复验，不能把改变span当作增加能力。
		if !arm.densityCargo.enabled && (records[input].RecipeID > 0 && surfaceIsBelt(records[output].ItemID) || records[output].RecipeID > 0 && surfaceIsBelt(records[input].ItemID)) {
			isOutput := records[input].RecipeID > 0
			p, q := r.LocalOffset, r.LocalOffset2
			if isOutput {
				p, q = q, p
			}
			dx, dy := float64(q.X-p.X), float64(q.Y-p.Y)
			length := math.Hypot(dx, dy)
			if length > 0 {
				for _, step := range []float64{.025, -.025, .05, -.05, .1, -.1, .2, -.2, .3, -.3, .4, -.4, .5, -.5} {
					candidate := r
					endpoint := &candidate.LocalOffset
					if isOutput {
						endpoint = &candidate.LocalOffset2
					}
					endpoint.X += float32(dx / length * step)
					endpoint.Y += float32(dy / length * step)
					candidates = append(candidates, candidate)
				}
			}
		}
		if arm.densityCargo.enabled {
			host, slot, endpoint := output, int(r.OutputToSlot), r.LocalOffset
			if arm.densityCargo.output {
				host, slot, endpoint = input, int(r.InputFromSlot), r.LocalOffset2
			}
			geometry, err := catalog.Geometry(records[host].ItemID, records[host].ModelIndex)
			if err != nil {
				return 0, err
			}
			if slot < 0 || slot >= len(geometry.SlotPoses) {
				return 0, fmt.Errorf("分拣器%d槽位无效", r.Index)
			}
			base, err := encoding.recordPose(world[host], records[host].Yaw, 0, 0)
			if err != nil {
				return 0, err
			}
			port := surfaceSlotPose(base, geometry.SlotPoses[slot], !arm.densityCargo.output)
			north, err := encoding.recordPose(world[host], 0, 0, 0)
			if err != nil {
				return 0, err
			}
			point := nativeSingleAreaPoint(encoding, port.position, float64(endpoint.X))
			delta := blueprint.Point{}
			if math.Abs(port.axes[2].Dot(north.axes[2])) >= math.Abs(port.axes[2].Dot(north.axes[0])) {
				delta.X = float32(point.x) - endpoint.X
			} else {
				delta.Y = float32(point.y) - endpoint.Y
			}
			for _, fraction := range []float32{0, .1, .25, .5, .75, 1} {
				for _, dz := range []float32{0, -.01, .01, -.02, .02, -.04, .04, -.08, .08} {
					candidate := r
					p := &candidate.LocalOffset
					if arm.densityCargo.output {
						p = &candidate.LocalOffset2
					}
					p.X += delta.X * fraction
					p.Y += delta.Y * fraction
					p.Z += dz
					if p.X < 0 || p.Y < 0 || p.X > float32(encoding.areas[0].Width-1) || p.Y > float32(encoding.areas[0].Height-1) {
						continue
					}
					candidates = append(candidates, candidate)
				}
			}
			distance := func(c blueprint.Building) float64 {
				p, q := c.LocalOffset, r.LocalOffset
				if arm.densityCargo.output {
					p, q = c.LocalOffset2, r.LocalOffset2
				}
				return math.Pow(float64(p.X-q.X), 2) + math.Pow(float64(p.Y-q.Y), 2) + math.Pow(float64(p.Z-q.Z), 2)
			}
			sort.SliceStable(candidates, func(i, j int) bool { return distance(candidates[i]) < distance(candidates[j]) })
		}
		var last error
		accepted := false
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			inside := true
			for _, point := range []blueprint.Point{candidate.LocalOffset, candidate.LocalOffset2} {
				inside = inside && point.X >= 0 && point.Y >= 0 && point.X <= float32(encoding.areas[0].Width-1) && point.Y <= float32(encoding.areas[0].Height-1)
			}
			if !inside {
				last = &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "分拣器端点修正超出所选原生区域"}
				continue
			}
			spans := map[int32]int32{}
			boxes, err := encoding.surfaceInserterBoxesMeasured(ctx, []blueprint.Building{candidate, records[input], records[output]}, spans)
			if err != nil {
				last = err
				continue
			}
			if len(candidate.Parameters) == 0 {
				last = fmt.Errorf("分拣器%d缺少跨度参数", candidate.Index)
				continue
			}
			candidate.Parameters = append([]int32(nil), candidate.Parameters...)
			candidate.Parameters[0] = spans[candidate.Index]
			conflict := false
			for _, a := range boxes {
				for _, c := range occupied {
					if a.Geometry.Overlaps(c.Geometry) {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
			}
			if conflict {
				last = fmt.Errorf("分拣器%d修正后与已定位臂碰撞", candidate.Index)
				continue
			}
			occupied = append(occupied, boxes...)
			records[i] = candidate
			if candidate.LocalOffset != r.LocalOffset || candidate.LocalOffset2 != r.LocalOffset2 {
				adjusted++
			}
			if candidate.LocalOffset != r.LocalOffset {
				arm.a = vector{float64(candidate.LocalOffset.X), float64(candidate.LocalOffset.Y), float64(candidate.LocalOffset.Z)}
			}
			if candidate.LocalOffset2 != r.LocalOffset2 {
				arm.b = vector{float64(candidate.LocalOffset2.X), float64(candidate.LocalOffset2.Y), float64(candidate.LocalOffset2.Z)}
			}
			arm.record.Parameters = candidate.Parameters
			arm.nativeInserterSpan = spans[candidate.Index]
			accepted = true
			break
		}
		if !accepted {
			return 0, last
		}
	}
	return adjusted, nil
}
