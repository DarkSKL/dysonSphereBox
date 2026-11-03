package build

// 文件职责：在水平机器行架中复用已开启行的真实空槽。
// 使用：自动球面的独立轮廓候选；边界：全局回填不得扩张当前包络，局部扩展仍受宽度和真实轮廓约束，路由与最终球面规则不省略。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// placeHorizontal 职责：比较有限水平靠拢点和包络内空槽；使用：跨行接线前逐行装箱；边界：只提交真实几何无冲突的刚体平移，不合并不同物料端口。
func (packing *connectedPacking) placeHorizontal(ctx context.Context, contour connectedBlockContour, low, high vector, shelfX, shelfY, widthLimit float64) (float64, float64, error) {
	width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
	candidates := []vector{}
	seen := map[vector]bool{}
	add := func(x, y float64, infill bool) {
		point := vector{math.Floor(x), math.Floor(y), 0}
		if point.x < 0 || point.y < 0 || point.x+width > math.Max(widthLimit, width) || seen[point] {
			return
		}
		if infill && (!packing.ready || point.x+width > packing.high.x || point.y+height > packing.high.y) {
			return
		}
		seen[point] = true
		candidates = append(candidates, point)
	}
	add(shelfX, shelfY, false)
	for retreat := 0.; retreat <= 18; retreat++ {
		for _, offset := range []float64{0, 1, 2, 4} {
			add(shelfX-retreat, shelfY+offset, false)
		}
	}
	// 换架时允许轮廓向前一架靠拢，才能利用稀疏电杆边带；少量横向错位用于避开其真实占位。
	for retreat := 1.; retreat <= 8; retreat++ {
		for _, offset := range []float64{0, 1, 2, 4, 8} {
			add(shelfX+offset, shelfY-retreat, false)
		}
	}
	// 保留全部局部候选；全局候选先按边缘位置排序，只追加至多64个不扩张包络的落点。
	anchors := append([]vector(nil), packing.infillAnchors...)
	sort.SliceStable(anchors, func(i, j int) bool {
		if anchors[i].y != anchors[j].y {
			return anchors[i].y < anchors[j].y
		}
		return anchors[i].x < anchors[j].x
	})
	base := len(candidates)
	for _, point := range anchors {
		if len(candidates)-base >= 64 {
			break
		}
		add(point.x, point.y, true)
		add(point.x-width, point.y, true)
	}
	best, score, found := vector{}, math.Inf(1), false
	for _, point := range candidates {
		if ctx.Err() != nil {
			return 0, 0, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "水平行架空槽回填已取消或超时"}
		}
		if !packing.fits(ctx, contour, point.x-low.x, point.y-low.y) {
			continue
		}
		area := math.Max(packing.high.x, point.x+width) * math.Max(packing.high.y, point.y+height)
		if !found || area < score || area == score && (point.y < best.y || point.y == best.y && point.x < best.x) {
			best, score, found = point, area, true
		}
	}
	if !found {
		return 0, 0, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "水平行架在有限靠拢和包络内回填候选中没有合法位置"}
	}
	for _, source := range contour.shapes {
		shape := shiftedConnectedShape(source, best.x-low.x, best.y-low.y)
		index := len(packing.shapes)
		packing.shapes = append(packing.shapes, shape)
		connectedShapeCells(shape, func(cell [3]int) bool {
			packing.cells[cell] = append(packing.cells[cell], index)
			return true
		})
	}
	packing.high.x, packing.high.y = math.Max(packing.high.x, best.x+width), math.Max(packing.high.y, best.y+height)
	packing.infillAnchors = append(packing.infillAnchors,
		vector{best.x + width + 1, best.y, 0},
		vector{best.x, best.y + height + 1, 0},
		vector{best.x, best.y + height - 2, 0},
		vector{0, best.y, 0})
	packing.ready = true
	return best.x, best.y, nil
}
