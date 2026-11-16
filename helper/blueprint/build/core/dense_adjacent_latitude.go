package build

// 文件职责：以用户选定位置为基准，允许完整密铺矩形使用相邻原生纬带。
// 使用：显式ExpandAdjacent的独立原生候选；边界：保留严格单带结果，不换产量、设备或锚点语义，完整终检决定可行性。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// denseAdjacentLatitudeKey 职责：传递本候选的扩展许可；使用：球面装箱与编码；边界：请求私有，旧请求默认关闭。
type denseAdjacentLatitudeKey struct{}

// denseLatitudeAllowedBands 职责：按中心/边缘给出连续相邻原生带；使用：布局与终检共用。
// 边界：未授权时保留原选区；授权时将旧112格最小扩展与原选区外指定方向的首条邻带取并集，不递归扩带、不跨极点、不移动参考点。
func denseLatitudeAllowedBands(selection densityLatitudeSelection, expand bool) []surfacePackedBand {
	selected := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
	if !expand || len(selected) == 0 {
		return selected
	}
	all := surfaceLatitudeBands(200)
	first, last := -1, -1
	for i, band := range all {
		if band.low == selected[0].low {
			first = i
		}
		if band.high == selected[len(selected)-1].high {
			last = i
		}
	}
	if first < 0 || last < first {
		return selected
	}
	low, high := selected[0].low, selected[len(selected)-1].high
	switch selection.position {
	case "south":
		high = max(high, min(all[len(all)-1].high-2, low+111))
	case "north":
		low = min(low, max(all[0].low+2, high-111))
	default:
		center := (low + high) / 2
		low = min(low, max(all[0].low+2, center-56))
		high = max(high, min(all[len(all)-1].high-2, center+55))
	}
	// 只从原始first/last取邻带，不能从112格扩后的端点继续加带；极点余量只限制新增范围，不裁掉原选区。
	if selection.position != "south" && first > 0 {
		low = min(low, max(all[0].low+2, all[first-1].low))
	}
	if selection.position != "north" && last+1 < len(all) {
		high = max(high, min(all[len(all)-1].high-2, all[last+1].high))
	}
	out := []surfacePackedBand{}
	for _, band := range all {
		if band.high < low || band.low > high {
			continue
		}
		band.low = max(band.low, low)
		band.high = min(band.high, high)
		out = append(out, band)
	}
	return out
}

// denseLatitudeReferenceRange 职责：固定最初选择的区域边界；使用：窗口定位和边缘证明；边界：扩展不会把锚点移动到邻带中心。
func denseLatitudeReferenceRange(selection densityLatitudeSelection) (int, int) {
	bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
	return bands[0].low, bands[len(bands)-1].high
}

// denseAdjacentFrameExtents 职责：按规模比较有限近方形窗口；使用：跨纬带候选；边界：窗口始终能围绕所选参考点放入允许区域。
func denseAdjacentFrameExtents(selection densityLatitudeSelection, bands []surfacePackedBand, machines int) []int {
	south, north := denseLatitudeReferenceRange(selection)
	limit := bands[len(bands)-1].high - bands[0].low + 1
	switch selection.position {
	case "south":
		limit = bands[len(bands)-1].high - south + 1
	case "north":
		limit = north - bands[0].low + 1
	default:
		center := (south + north) / 2
		limit = 2*min(center-bands[0].low, bands[len(bands)-1].high-center) + 1
	}
	// 实体坐标边界外仍有真实模型半径，两端各留两格，不用跨带许可吞掉外侧碰撞盒。
	limit -= 4
	ideal := int(math.Ceil(math.Sqrt(float64(max(1, machines))*12)*1.25)) + 12
	out := []int{}
	seen := map[int]bool{}
	windows := []int{ideal, ideal * 4 / 5, ideal * 6 / 5}
	// 少量大型设施、监测器和缓存的占地不能由机器数准确代表；按比例补齐到区域上限的窗口，避免从小窗口直接跳到全带。
	for value := ideal * 3 / 2; value < limit; value += max(1, value/2) {
		windows = append(windows, value)
	}
	windows = append(windows, limit)
	for _, value := range windows {
		value = min(limit, max(8, value))
		if value > 0 && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// updateDenseRootShapeMetrics 职责：将历史root度量入口收敛到共同物理包络；使用：所有密度、单区和多区的最终度量；边界：历史函数名不再限定dense，不把根网格伸缩当作实际形状，不更改格网面积和真实带长。
func updateDenseRootShapeMetrics(ctx context.Context, surface *surfaceEncoding, records []blueprint.Building, metrics *LayoutMetrics) error {
	return updatePhysicalLayoutShapeMetrics(ctx, surface, records, metrics)
}
