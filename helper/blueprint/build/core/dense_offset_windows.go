package build

// 文件职责：在已授权的同一原生bands中规划显式偏移窗口并维护原参考纬度。
// 使用：非极区中心dense mode1大厂的可选布局候选；边界：不扩大depth/bands，不改格距、边缘/镜像语义、物理规则或最终shape阈值。

import (
	"context"
	"fmt"
	"sort"
)

// denseOffsetWindowKey 职责：传递本候选的显式偏移标记；使用：装配/几何快照；边界：默认false，不允许外部ctx覆盖已冻结值。
type denseOffsetWindowKey struct{}

// denseOffsetWindowEligible 职责：统一可选偏移窗的核心权限；使用：枚举和Prepare；边界：原用户ExpandAdjacent、非极区中心及大dense1缺一不可，镜像仍沿旧行为。
func denseOffsetWindowEligible(req Request, selection densityLatitudeSelection, expand, mirror bool) bool {
	return req.Mode == 1 && req.Density == densityDense && req.Plan.MachineCount >= denseNativeLargeMachineThreshold && req.SurfaceLayout != nil && req.SurfaceLayout.ExpandAdjacent && expand && !mirror && selection.tier < 5 && selection.position == "center"
}

// denseOffsetFrameStart 职责：把原居中起点夹入已有bands扣两端原边距后的范围。
// 使用：显式偏移frame；边界：不能跨极点、增许可或丢掉原参考中心，太大或不合法窗口明确拒绝。
func denseOffsetFrameStart(selection densityLatitudeSelection, bands []surfacePackedBand, height, centeredStart int) (int, error) {
	if selection.tier >= 5 || selection.position != "center" || len(bands) == 0 || height < 6 {
		return 0, fmt.Errorf("偏移窗口只支持非极区中心原生窗口")
	}
	low, high := bands[0].low+2, bands[len(bands)-1].high-2
	if height > high-low+1 {
		return 0, fmt.Errorf("偏移窗口%d行超过当前许可内%d行", height, high-low+1)
	}
	start := max(low, min(centeredStart, high-height+1))
	south, north := denseLatitudeReferenceRange(selection)
	reference := float64((south + north) / 2)
	if float64(start) > reference || float64(start+height-1) < reference {
		return 0, fmt.Errorf("偏移窗口未包含原选区参考纬度")
	}
	return start, nil
}

// denseOffsetSurfaceSpecs 职责：为同一深度添加有限、实际有平移的窗口规格。
// 使用：显式AllowOffsetWindows激活之后；边界：无需移动的Offset规格不输出，不执行装箱或改变原请求。
func denseOffsetSurfaceSpecs(req Request, selection densityLatitudeSelection, bands []surfacePackedBand, options SurfaceSearchOptions, depth int) ([][]SurfaceCandidateSpec, error) {
	if !options.AllowOffsetWindows || !denseOffsetWindowEligible(req, selection, options.ExpandAdjacent, false) {
		return nil, nil
	}
	extents := denseAdjacentFrameExtentsAtLimit(req.Plan.MachineCount, bands[len(bands)-1].high-bands[0].low+1-4)
	if options.LargestOffsetFirst {
		sort.Sort(sort.Reverse(sort.IntSlice(extents)))
	}
	groups := [][]SurfaceCandidateSpec{}
	for _, height := range extents {
		frame, err := newDensePolarLayoutFrame(selection, bands, height, true, false, true, true)
		if err != nil {
			return nil, err
		}
		if !frame.offsetWindow {
			continue
		}
		base := SurfaceCandidateSpec{Strategy: "native-window", ExtentRows: height, AdjacentDepth: depth, OffsetWindow: true, PreciseFootprint: options.PreciseFootprint, ExpandAdjacent: true}
		group := denseSurfaceWindowModifiers(req, selection, options, base)
		groups = append(groups, group)
	}
	return groups, nil
}

// denseInterleaveOffsetSpecs 职责：保留每depth内原规格相对顺序，并让各新增窗口的主规格及后续变体轮转。
// 使用：每depth候选末端；边界：选项关闭时原列表不变，不把新大窗埋在所有旧变体之后。
func denseInterleaveOffsetSpecs(old []SurfaceCandidateSpec, groups [][]SurfaceCandidateSpec, preferIdeal bool) []SurfaceCandidateSpec {
	if len(groups) == 0 {
		return old
	}
	if !preferIdeal {
		out := append([]SurfaceCandidateSpec(nil), old...)
		for _, g := range groups {
			out = append(out, g...)
		}
		return out
	}
	groups = append([][]SurfaceCandidateSpec{groups[0], old}, groups[1:]...)
	out := []SurfaceCandidateSpec{}
	for n := 0; ; n++ {
		added := false
		for _, g := range groups {
			if n < len(g) {
				out = append(out, g[n])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out
}

// denseOffsetWindowPlanValid 职责：对显式偏移候选验证原参考点与相同depth许可。
// 使用：原中心最终计划检查的必要分支；边界：旧false返回原判定，Offset不能豁免原band/模型/半圈/shape等独立规则，裁掉空行后的实际范围也须包含参考纬度。
func denseOffsetWindowPlanValid(layout *SurfaceLayout, selection densityLatitudeSelection, first, last int, oldValid bool) bool {
	if !layout.offsetWindow {
		return oldValid
	}
	if !layout.ExpandAdjacent || !layout.polarRoot || layout.nativeBand != nil || layout.GridSegment != 200 || layout.adjacentDepth < 0 || layout.adjacentDepth > 3 || selection.tier >= 5 || selection.position != "center" {
		return false
	}
	bands := denseLatitudeAllowedBands(selection, true, layout.adjacentDepth)
	south, north := denseLatitudeReferenceRange(selection)
	reference := float64((south + north) / 2)
	return len(bands) > 0 && first >= bands[0].low+2 && last <= bands[len(bands)-1].high-2 && float64(first) <= reference && float64(last) >= reference
}

// denseOffsetContext 职责：读取本候选的偏移标记；使用：无完整request的监测器布局重建；边界：缺省false。
func denseOffsetContext(ctx context.Context) bool {
	value, _ := ctx.Value(denseOffsetWindowKey{}).(bool)
	return value
}

// denseSurfaceWindowModifiers 职责：从基础窗口按旧次序派生sameSide/aspect/turn正交修饰。
// 使用：原居中窗口和显式Offset窗口共同消费；边界：不授予跨带/偏移权限、不修改base，其它完整字段原样保留。
func denseSurfaceWindowModifiers(req Request, selection densityLatitudeSelection, options SurfaceSearchOptions, base SurfaceCandidateSpec) []SurfaceCandidateSpec {
	out := []SurfaceCandidateSpec{}
	for _, sameSide := range refineryLayoutVariants(req) {
		for _, aspect := range denseLocalRectangleAspects(req, selection, options) {
			turns := []bool{false}
			if aspect > 0 && options.CompareTurnedRows {
				turns = append(turns, true)
			}
			for _, turn := range turns {
				spec := base
				spec.SameSideRefinery = sameSide
				spec.LocalRectangleAspect = aspect
				spec.TurnLongRows = turn
				out = append(out, spec)
			}
		}
	}
	return out
}
