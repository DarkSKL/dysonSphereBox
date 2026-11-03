package build

// 文件职责：用剩余矩形回填闭合块纬带空洞，不丢弃已开启行的尾部容量。
// 使用：有限自动球面宽度候选；边界：完整块不旋转、不拆分，所有真实几何回调仍执行。

import (
	"cmp"
	"context"
	"slices"

	"DysonSphereBox/helper/blueprint"
)

// surfaceFreeRectangle 职责：表示带间距的未占用矩形；使用：完整块落点筛选；边界：可与其他剩余矩形重叠，但已放块必须从全部矩形扣除。
type surfaceFreeRectangle struct{ x, y, width, height int }

// surfaceSubtractRectangle 职责：从所有空洞扣除真实已提交块及间距；使用：矩形装箱；边界：只删除包含项，不丢弃尚可容纳较小块的旧行尾部。
func surfaceSubtractRectangle(free []surfaceFreeRectangle, occupied surfaceFreeRectangle) []surfaceFreeRectangle {
	result := make([]surfaceFreeRectangle, 0, len(free)+4)
	for _, cell := range free {
		if occupied.x >= cell.x+cell.width || occupied.x+occupied.width <= cell.x || occupied.y >= cell.y+cell.height || occupied.y+occupied.height <= cell.y {
			result = append(result, cell)
			continue
		}
		if occupied.x > cell.x {
			result = append(result, surfaceFreeRectangle{cell.x, cell.y, occupied.x - cell.x, cell.height})
		}
		if occupied.x+occupied.width < cell.x+cell.width {
			result = append(result, surfaceFreeRectangle{occupied.x + occupied.width, cell.y, cell.x + cell.width - occupied.x - occupied.width, cell.height})
		}
		if occupied.y > cell.y {
			result = append(result, surfaceFreeRectangle{cell.x, cell.y, cell.width, occupied.y - cell.y})
		}
		if occupied.y+occupied.height < cell.y+cell.height {
			result = append(result, surfaceFreeRectangle{cell.x, occupied.y + occupied.height, cell.width, cell.y + cell.height - occupied.y - occupied.height})
		}
	}
	for i := 0; i < len(result); i++ {
		for j := 0; j < len(result); j++ {
			if i == j {
				continue
			}
			a, b := result[i], result[j]
			if a.x >= b.x && a.y >= b.y && a.x+a.width <= b.x+b.width && a.y+a.height <= b.y+b.height {
				result = append(result[:i], result[i+1:]...)
				i--
				break
			}
		}
	}
	return result
}

// planSurfaceBlockRectangles 职责：按高块顺序回填可用矩形并返回原编号落点；使用：有限自动球面宽度；边界：固定宽度、真实纬带、同一取消及几何预算，不返回半份布局。
func planSurfaceBlockRectangles(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap, width int, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool) (*surfacePacking, error) {
	return planSurfaceBlockRectanglesBounded(ctx, layout, sizes, gap, width, width, accept, acceptPlacement)
}

// planSurfaceBlockRectanglesBounded 职责：为短侧带单独限制横向展开并回填赤道空洞；使用：闭合块全图包络候选；边界：侧带限宽只收紧原生容量，不影响通用矩形入口。
func planSurfaceBlockRectanglesBounded(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap, width, sideWidth int, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool) (*surfacePacking, error) {
	// 非自动布局与非法输入沿用统一入口的验证和显式纬带语义，避免专用装箱绕过既有硬规则。
	if layout == nil || layout.Mode != "autoTropic" || gap < 2 || len(sizes) == 0 {
		return planSurfaceBlocksWidthChecked(ctx, layout, sizes, gap, width, accept, acceptPlacement)
	}
	segment := packingSegment(layout)
	if segment < 40 || segment > 512 || segment%4 != 0 || slices.ContainsFunc(sizes, func(size surfaceBlockSize) bool {
		return size.width <= 0 || size.height <= 0
	}) {
		return planSurfaceBlocksWidthChecked(ctx, layout, sizes, gap, width, accept, acceptPlacement)
	}
	rules := newSurfacePackRules(gap)
	rules.widthLimit, rules.ctx, rules.accept, rules.acceptPlacement = width, ctx, accept, acceptPlacement
	bands := surfaceLatitudeBands(segment)
	center := -1
	for i, band := range bands {
		if band.low <= 0 && band.high >= 0 {
			center = i
			break
		}
	}
	if center < 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "闭合块矩形装箱缺少赤道纬带"}
	}
	order := make([]int, len(sizes))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(sizes[b].height, sizes[a].height), cmp.Compare(sizes[b].width, sizes[a].width))
	})
	free := make([][]surfaceFreeRectangle, len(bands))
	used := make([]bool, len(bands))
	for i, band := range bands {
		bandWidth := rules.usableWidth(band)
		if i != center && sideWidth > 0 {
			bandWidth = min(bandWidth, sideWidth)
		}
		free[i] = []surfaceFreeRectangle{{rules.ringMargin, rules.edgeMargin, bandWidth + gap, rules.usableHeight(band) + gap}}
	}
	blocks := make([]surfacePackedBlock, len(sizes))
	previous := make([]surfacePackPlacement, 0, len(sizes))
	bandOrder := surfaceBandOrder(bands, center, 0, layout.ExpandAdjacent)
	opened := 0
	for _, index := range order {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "闭合块球面矩形装箱已取消或超时"}
		}
		size, placed := sizes[index], false
		for _, bi := range bandOrder {
			if !used[bi] && opened >= surfaceMaxAreas {
				continue
			}
			band := bands[bi]
			cells := append([]surfaceFreeRectangle(nil), free[bi]...)
			slices.SortStableFunc(cells, func(a, b surfaceFreeRectangle) int {
				return cmp.Or(cmp.Compare(min(a.width-size.width-gap, a.height-size.height-gap), min(b.width-size.width-gap, b.height-size.height-gap)), cmp.Compare(a.y, b.y), cmp.Compare(a.x, b.x))
			})
			for _, cell := range cells {
				if cell.width < size.width+gap || cell.height < size.height+gap {
					continue
				}
				cellRules := rules
				cellRules.widthLimit = cell.x + cell.width - gap - rules.ringMargin
				for y := cell.y; y+size.height+gap <= cell.y+cell.height; y++ {
					if ctx.Err() != nil {
						return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "闭合块球面矩形装箱已取消或超时"}
					}
					x, ok := cellRules.acceptedX(index, band, cell.x, y, size, previous)
					if !ok || x+size.width+gap > cell.x+cell.width {
						continue
					}
					free[bi] = surfaceSubtractRectangle(free[bi], surfaceFreeRectangle{x, y, size.width + gap, size.height + gap})
					blocks[index] = surfacePackedBlock{band: bi, x: x, y: y}
					previous = append(previous, surfacePackPlacement{index: index, band: band, x: x, y: y})
					if !used[bi] {
						opened++
					}
					placed, used[bi] = true, true
					break
				}
				if placed {
					break
				}
			}
			if placed {
				break
			}
		}
		if !placed {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "闭合块球面矩形无法容纳完整块"}
		}
	}
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "闭合块球面矩形装箱已取消或超时"}
	}
	packing := &surfacePacking{segment: segment, blocks: blocks}
	remap := map[int]int{}
	for bi := range bands {
		if used[bi] {
			remap[bi] = len(packing.bands)
			packing.bands = append(packing.bands, bands[bi])
		}
	}
	for i := range packing.blocks {
		packing.blocks[i].band = remap[packing.blocks[i].band]
	}
	if len(packing.blocks) > 0 {
		packing.selected = bandCenter(packing.bands[packing.blocks[order[0]].band])
	}
	return packing, nil
}
