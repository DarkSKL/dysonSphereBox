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
	domains := []nativeInserterDomain{}
	maxX, maxY := encoding.entityBounds()
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
		if encoding.rootCoordinates {
			// 跨子环记录直接使用真实机器槽位，避免游戏再把旧切平面端点的横向误差加到带端。
			for end, host := range []int{input, output} {
				if surfaceIsBelt(records[host].ItemID) || records[host].ItemID == 2101 {
					continue
				}
				slot := int(r.InputFromSlot)
				point := &r.LocalOffset
				if end == 1 {
					slot = int(r.OutputToSlot)
					point = &r.LocalOffset2
				}
				geometry, err := catalog.Geometry(records[host].ItemID, records[host].ModelIndex)
				if err != nil {
					return 0, err
				}
				if slot < 0 || slot >= len(geometry.SlotPoses) {
					return 0, fmt.Errorf("分拣器%d缺少原生机器槽位", r.Index)
				}
				base, err := encoding.recordPose(world[host], records[host].Yaw, 0, 0)
				if err != nil {
					return 0, err
				}
				port := surfaceSlotPose(base, geometry.SlotPoses[slot], end == 1)
				local := nativeSingleAreaPoint(encoding, port.position, float64(point.X))
				*point = blueprint.Point{X: float32(local.x), Y: float32(local.y), Z: float32((math.Sqrt(port.position.Dot(port.position)) - 200.2) / 1.3333333)}
			}
		}
		candidates := []blueprint.Building{r}
		if encoding.rootCoordinates && surfaceIsBelt(records[input].ItemID) && surfaceIsBelt(records[output].ItemID) {
			secondRecord := r
			secondRecord.LocalOffset = r.LocalOffset2
			second, err := encoding.surfaceWorldPositions([]blueprint.Building{secondRecord})
			if err != nil {
				return 0, err
			}
			direction := placement.Vector{second[0].x - world[i].x, second[0].y - world[i].y, second[0].z - world[i].z}
			candidate := r
			for end, point := range []vector{world[i], second[0]} {
				frame, err := encoding.recordPose(point, 0, 0, 0)
				if err != nil {
					return 0, err
				}
				yaw := float32(math.Mod(math.Atan2(direction.Dot(frame.axes[0]), direction.Dot(frame.axes[2]))*180/math.Pi+360, 360))
				if end == 0 {
					candidate.Yaw = yaw
				} else {
					candidate.Yaw2 = yaw
				}
			}
			candidates = append(candidates, candidate)
		}
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
		if arm.densityCargo.enabled || encoding.rootCoordinates && surfaceIsBelt(records[input].ItemID) != surfaceIsBelt(records[output].ItemID) {
			outputToBelt := surfaceIsBelt(records[output].ItemID)
			host, slot, endpoint := output, int(r.OutputToSlot), r.LocalOffset
			if outputToBelt {
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
			port := surfaceSlotPose(base, geometry.SlotPoses[slot], !outputToBelt)
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
				heights := []float32{0, -.01, .01, -.02, .02, -.04, .04, -.08, .08}
				if encoding.rootCoordinates {
					heights = append(heights, -.12, .12, -.16, .16, -.24, .24, -.32, .32)
				}
				for _, dz := range heights {
					candidate := r
					p := &candidate.LocalOffset
					if outputToBelt {
						p = &candidate.LocalOffset2
					}
					p.X += delta.X * fraction
					p.Y += delta.Y * fraction
					p.Z += dz
					if p.X < 0 || p.Y < 0 || p.X > maxX || p.Y > maxY {
						continue
					}
					candidates = append(candidates, candidate)
				}
			}
			distance := func(c blueprint.Building) float64 {
				p, q := c.LocalOffset, r.LocalOffset
				if outputToBelt {
					p, q = c.LocalOffset2, r.LocalOffset2
				}
				return math.Pow(float64(p.X-q.X), 2) + math.Pow(float64(p.Y-q.Y), 2) + math.Pow(float64(p.Z-q.Z), 2)
			}
			sort.SliceStable(candidates, func(i, j int) bool { return distance(candidates[i]) < distance(candidates[j]) })
		}
		var last error
		domain := nativeInserterDomain{index: i}
		seen := map[nativeInserterEndpoints]bool{}
		inspect := func(candidate blueprint.Building) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			inside := true
			for _, point := range []blueprint.Point{candidate.LocalOffset, candidate.LocalOffset2} {
				inside = inside && point.X >= 0 && point.Y >= 0 && point.X <= maxX && point.Y <= maxY
			}
			if !inside {
				last = &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "分拣器端点修正超出所选原生区域"}
				return nil
			}
			spans := map[int32]int32{}
			boxes, err := encoding.surfaceInserterBoxesMeasured(ctx, []blueprint.Building{candidate, records[input], records[output]}, spans)
			if err != nil {
				last = err
				return nil
			}
			if len(candidate.Parameters) == 0 {
				last = fmt.Errorf("分拣器%d缺少跨度参数", candidate.Index)
				return nil
			}
			candidate.Parameters = append([]int32(nil), candidate.Parameters...)
			candidate.Parameters[0] = spans[candidate.Index]
			key := nativeInserterEndpoints{candidate.LocalOffset, candidate.LocalOffset2, candidate.Yaw, candidate.Yaw2}
			if !seen[key] {
				domain.choices = append(domain.choices, nativeInserterChoice{record: candidate, boxes: boxes})
				seen[key] = true
			}
			return nil
		}
		inspectVariants := func(candidate blueprint.Building) error {
			if err := inspect(candidate); err != nil {
				return err
			}
			if encoding.rootCoordinates && surfaceIsBelt(records[input].ItemID) != surfaceIsBelt(records[output].ItemID) {
				aligned, err := nativeBeltEndHeading(encoding, candidate, surfaceIsBelt(records[output].ItemID))
				if err != nil {
					return err
				}
				return inspect(aligned)
			}
			return nil
		}
		for _, candidate := range candidates {
			if err := inspectVariants(candidate); err != nil {
				return 0, err
			}
		}
		if len(domain.choices) == 0 && encoding.rootCoordinates && (surfaceIsBelt(records[input].ItemID) != surfaceIsBelt(records[output].ItemID)) {
			second := surfaceIsBelt(records[output].ItemID)
			yaw := r.Yaw
			if second {
				yaw = r.Yaw2
			}
			angle := float64(yaw) * math.Pi / 180
			for _, step := range []float64{.05, -.05, .1, -.1, .2, -.2, .35, -.35, .5, -.5} {
				for _, candidate := range candidates {
					p := &candidate.LocalOffset
					if second {
						p = &candidate.LocalOffset2
					}
					p.X += float32(math.Sin(angle) * step)
					p.Y += float32(math.Cos(angle) * step)
					if err := inspectVariants(candidate); err != nil {
						return 0, err
					}
				}
				if len(domain.choices) > 0 {
					break
				}
			}
		}
		if len(domain.choices) == 0 {
			return 0, fmt.Errorf("原生端口 %d（宿主 %d→%d，端点 %v→%v，格距 %.3f）：%w", r.Index, records[input].ItemID, records[output].ItemID, r.LocalOffset, r.LocalOffset2, arm.nativeUnitScale, last)
		}
		domains = append(domains, domain)
	}
	return b.chooseNativeInserterEndpoints(ctx, records, domains)
}
