package build

// 文件职责：在路由/附件/尾带裁剪完成后，用机器阵列快速预筛固定区域的静态冲突。
// 使用：固定包络的单区域密铺候选；边界：不得提前到裁剪前限制锚点，通过仍需原尺寸完整终检。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
)

// preflightDenseMachineSurface 职责：在昂贵完整几何/补电前筛出固定包络下的稳健机器冲突；使用：全部路由和裁剪后；边界：精确旧单点请求和可多区重排流程不据此剪枝。
func (b *builder) preflightDenseMachineSurface(ctx context.Context, req Request) error {
	// 原生高纬随后才补偿经向格距；不能用尚未适配的平面间距提前淘汰，完整原生检查仍必跑。
	if denseNativeLatitude(req) {
		return nil
	}
	if !b.denseSpatial || !b.connectedGlobalPower || req.SurfaceLayout == nil || req.SurfaceLayout.Mode == "planar" {
		return nil
	}
	layout := req.SurfaceLayout
	if layout.Mode == "explicitTropic" && layout.LatitudeZone == "" {
		return nil
	}
	low, high := bounds(b.all)
	width, height := int(math.Ceil(high.x-low.x))+1, int(math.Ceil(high.y-low.y))+1
	if width <= 0 || height <= 0 || width > 32767 || height > 32767 {
		return nil
	}
	anchors := surfaceLayoutAnchorCandidates(layout, width, height)
	if len(anchors) == 0 {
		return nil
	}
	records := []blueprint.Building{}
	for i, entity := range b.all {
		if entity.record.RecipeID <= 0 {
			continue
		}
		r := entity.record
		r.Index = int32(i)
		r.AreaIndex = 0
		r.LocalOffset = blueprint.Point{X: float32(entity.a.x - low.x), Y: float32(entity.a.y - low.y), Z: float32(entity.a.z)}
		records = append(records, r)
	}
	if len(records) < 2 {
		return nil
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	shapes := make([][]placement.Collider, len(records))
	for i, record := range records {
		shapes[i], err = catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return err
		}
	}
	reportProgress(ctx, "dense-static-probe", "正在预检机器阵列的可用球面位置", 0, len(anchors))
	var left, right int
	for _, anchor := range anchors {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺阵列预检已取消或超时"}
		}
		encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(longitudeSegmentCount(anchor, layout.GridSegment)), Width: int16(width), Height: int16(height)}}, plan: &SurfacePlan{GridSegment: layout.GridSegment, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor}}
		positions, err := encoding.surfaceWorldPositions(records)
		if err != nil {
			return err
		}
		boxes := []placement.OwnedBox{}
		for i, record := range records {
			for _, shape := range shapes[i] {
				box, err := encoding.colliderBox(positions[i], record.Yaw, shape)
				if err != nil {
					return err
				}
				// 仅预筛选时去掉一毫米边缘，避免后续整体平移的float32舍入造成临界误拒。
				// 这使拒绝条件更严格；不会将该临时尺寸用于最终校验或宣称布局通过。
				for axis := range box.HalfSize {
					box.HalfSize[axis] = math.Max(box.HalfSize[axis]-.001, box.HalfSize[axis]*.99)
				}
				boxes = append(boxes, placement.OwnedBox{Owner: int(record.Index), Geometry: box})
			}
		}
		var statistics placement.ConflictStatistics
		left, right, statistics, err = placement.FirstConflictWithStatistics(ctx, boxes)
		recordPerformanceCollision(ctx, "denseMachinePreflight", statistics.CandidatePairs, statistics.NarrowPhaseChecks)
		if err != nil {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺阵列预检已取消或超时"}
			}
			return err
		}
		if left < 0 {
			return nil
		}
	}
	return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("机器阵列在最终固定包络的全部%d个可用锚点均有静态冲突（末次建筑%d/%d）", len(anchors), left, right)}
}
