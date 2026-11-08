package build

// 文件职责：把玩家可理解的建造区域转换为严格的原生纬带范围。
// 使用：超密度选区、候选锚点及最终结果核验；边界：仅原版200分段星球，不为失败结果自动换区。

import (
	"DysonSphereBox/helper/blueprint"
	"cmp"
	"context"
	"fmt"
	"math"
	"sort"
)

// compareDenseLatitudeEdge 职责：第3档选边缘时优先比较实际可建包络距边界的格数；使用：已通过完整规则的直连候选择优；边界：不让更小面积覆盖用户的放置位置选择。
func compareDenseLatitudeEdge(layout *SurfaceLayout, a, b *SurfacePlan) int {
	if layout == nil || a == nil || b == nil {
		return 0
	}
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	if !ok || selection.tier != 3 || selection.position == "center" {
		return 0
	}
	band, _, err := densityLayoutBand(layout)
	if err != nil {
		return 0
	}
	gap := func(plan *SurfacePlan) int {
		if selection.position == "south" {
			return plan.AnchorLatitudeGrid - band.low
		}
		return band.high - plan.AnchorLatitudeGrid - plan.LayoutHeight + 1
	}
	return cmp.Compare(gap(a), gap(b))
}

// DensityLatitudeOption 职责：提供简单放置区域选项；使用：数据集元信息与界面下拉；边界：选项不承诺任意产量均可放置。
type DensityLatitudeOption struct {
	// Value 职责：保存稳定区域键；使用：SurfaceLayout；边界：未知键拒绝。
	Value string `json:"value"`
	// Label 职责：保存玩家可读名称；使用：下拉；边界：不要求输入纬圈索引。
	Label string `json:"label"`
	// Description 职责：说明区域位置及容量差异；使用：选项提示；边界：不是游戏放置保证。
	Description string `json:"description"`
	// Tier 职责：标示从赤道向两极的档位；使用：界面球体分档着色；边界：仅展示，不参与生成判定。
	Tier int `json:"tier"`
	// Position 职责：标示南北边缘或中心（center/north/south）；使用：界面在纬带内划分子区；边界：南北按地理方向定义。
	Position string `json:"position"`
	// LatitudeLow 职责：所属原生纬带南界纬度（度，南负北正）；使用：界面球体绘制真实纬带；边界：按200分段网格折算，仅展示。
	LatitudeLow float64 `json:"latitudeLow"`
	// LatitudeHigh 职责：所属原生纬带北界纬度（度）；使用：同 LatitudeLow；边界：同 LatitudeLow。
	LatitudeHigh float64 `json:"latitudeHigh"`
}

// DensityLatitudeOptions 职责：返回支持的原版建造区域；使用：界面发现能力；边界：每次返回独立切片。
func DensityLatitudeOptions() []DensityLatitudeOption {
	options := densityLatitudeOptionLabels()
	// 纬度按网格行边界折算：每行占 90/(segment*5/4) 度，纬带上下界各外扩半行，使相邻档无缝衔接。
	degreesPerRow := 90 / float64(defaultSurfaceGridSegment*5/4)
	for i := range options {
		selection, ok := densityLatitudeSelectionFor(options[i].Value)
		if !ok {
			continue
		}
		options[i].Tier, options[i].Position = selection.tier, selection.position
		if bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, defaultSurfaceGridSegment); len(bands) == 1 {
			options[i].LatitudeLow = (float64(bands[0].low) - 0.5) * degreesPerRow
			options[i].LatitudeHigh = (float64(bands[0].high) + 0.5) * degreesPerRow
		}
	}
	return options
}

// densityLatitudeOptionLabels 职责：维护选区键与玩家可读文案；使用：DensityLatitudeOptions 补充几何前的基础表；边界：每次返回独立切片。
func densityLatitudeOptionLabels() []DensityLatitudeOption {
	options := []DensityLatitudeOption{
		{Value: "equator", Label: "第1档 · 赤道环 · 中心", Description: "最宽的赤道环中心；按最终已验证起点范围放置。"},
		{Value: "equator-south", Label: "第1档 · 赤道环 · 南边缘", Description: "靠赤道环南侧分界线放置；保持默认方向。"},
		{Value: "equator-north", Label: "第1档 · 赤道环 · 北边缘", Description: "靠赤道环北侧分界线放置；保持默认方向。"},
		{Value: "ring2-north-center", Label: "第2档 · 北半球 · 中心", Description: "赤道以北的下一圈，环宽50格；蓝图保持在本圈内。"},
		{Value: "ring2-north-south", Label: "第2档 · 北半球 · 南边缘", Description: "第2档靠近赤道的一侧；按已验证起点放置。"},
		{Value: "north-mid", Label: "第2档 · 北半球 · 北边缘", Description: "第2档远离赤道的一侧；兼容原北半球中部选项。"},
		{Value: "ring2-south-center", Label: "第2档 · 南半球 · 中心", Description: "赤道以南的下一圈，环宽50格；蓝图保持在本圈内。"},
		{Value: "south-mid", Label: "第2档 · 南半球 · 南边缘", Description: "第2档远离赤道的一侧；兼容原南半球中部选项。"},
		{Value: "ring2-south-north", Label: "第2档 · 南半球 · 北边缘", Description: "第2档靠近赤道的一侧；按已验证起点放置。"},
		{Value: "ring3-north-center", Label: "第3档 · 北半球 · 中心", Description: "第3档环宽25格；按最终已验证起点范围放置。"},
		{Value: "ring3-north-south", Label: "第3档 · 北半球 · 南边缘", Description: "第3档靠近赤道的一侧；整图保持在本圈内。"},
		{Value: "north-high", Label: "第3档 · 北半球 · 北边缘", Description: "第3档远离赤道的一侧；兼容原北半球高纬选项。"},
		{Value: "ring3-south-center", Label: "第3档 · 南半球 · 中心", Description: "第3档环宽25格；按最终已验证起点范围放置。"},
		{Value: "south-high", Label: "第3档 · 南半球 · 南边缘", Description: "第3档远离赤道的一侧；兼容原南半球高纬选项。"},
		{Value: "ring3-south-north", Label: "第3档 · 南半球 · 北边缘", Description: "第3档靠近赤道的一侧；整图保持在本圈内。"},
	}
	for _, hemisphere := range []string{"north", "south"} {
		for _, position := range []string{"center", "south", "north"} {
			key := "ring4-" + hemisphere + "-" + position
			selection, _ := densityLatitudeSelectionFor(key)
			options = append(options, DensityLatitudeOption{Value: key, Label: selection.label(), Description: "第4档环宽25格；按原生格距适配间距，按蓝图给出的光标范围放置。"})
		}
	}
	return options

}

// densityLatitudeBand 职责：通过当前网格表解析选区；使用：请求与候选核验；边界：不硬编码纬带边界，不支持任意字符串或其它网格。
func densityLatitudeBand(zone string, segment int) (surfacePackedBand, string, error) {
	selection, ok := densityLatitudeSelectionFor(zone)
	if !ok || segment != defaultSurfaceGridSegment {
		return surfacePackedBand{}, "", &blueprint.Error{Code: "GEN_E_SURFACE", Message: "未知密铺建造区域或不支持的星球网格"}
	}
	if selection.tier == 5 {
		bands := densityLatitudeTierBands(5, selection.hemisphere, segment)
		if len(bands) > 0 {
			return surfacePackedBand{low: bands[0].low, high: bands[len(bands)-1].high}, selection.label(), nil
		}
	}

	label := ""
	for _, option := range densityLatitudeOptionLabels() {
		if option.Value == zone {
			label = option.Label
		}
	}
	if label == "" {
		label = selection.label()
	}
	bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, segment)
	if len(bands) == 1 {
		return bands[0], label, nil
	}
	return surfacePackedBand{}, "", &blueprint.Error{Code: "GEN_E_SURFACE", Message: "所选建造区域没有对应的原生纬带"}
}

// densityLatitudeKey 职责：把本次选区的高度预算传到所有全场适配器；使用：共享空间搜索；边界：仅请求生命周期，普通请求不注入。
type densityLatitudeKey struct{}

// surfaceLayoutAnchorCandidates 职责：在选定区域内选择完整矩形锚点；使用：公共球面编码；边界：几何失败只在同一原生纬带内移动，绝不改到赤道或邻带。
func surfaceLayoutAnchorCandidates(layout *SurfaceLayout, width, height int) []int {
	if layout.LatitudeZone == "" {
		return surfaceAnchorCandidates(layout.Mode, layout.GridSegment, layout.LatitudeGridIndex, layout.ExpandAdjacent, width, height)
	}
	band, _, err := densityLayoutBand(layout)
	if err != nil {
		return nil
	}
	preferred := bandCenter(band) - (height-1)/2
	anchors := []int{}
	margin := 2
	selection, _ := densityLatitudeSelectionFor(layout.LatitudeZone)
	if selection.tier >= 3 {
		// 狭窄原生带可使用全部格行；完整碰撞检查决定落点，不能再重复扣除装箱已预留的线路余量。
		margin = 0
		if selection.position == "south" {
			preferred = band.low
		}
		if selection.position == "north" {
			preferred = band.high - height + 1
		}
	}
	for anchor := band.low + margin; anchor+height-1 <= band.high-margin; anchor++ {
		if selection.tier >= 4 && absInt(anchor-preferred) > 4 {
			continue
		}
		if candidateFitsRing(anchor, 0, height-1, width, layout.GridSegment) {
			anchors = append(anchors, anchor)
		}
	}
	sort.SliceStable(anchors, func(i, j int) bool { return absInt(anchors[i]-preferred) < absInt(anchors[j]-preferred) })
	return anchors
}

// denseLatitudeHeightLimit 职责：在选区内预留真实边界接线余量；使用：全场矩形候选；边界：最终范围由完整编码再次硬核验。
func denseLatitudeHeightLimit(ctx context.Context, limit float64) float64 {
	zone, _ := ctx.Value(densityLatitudeKey{}).(string)
	if zone == "" {
		return limit
	}
	band, _, err := densityContextBand(ctx, zone, defaultSurfaceGridSegment)
	if err != nil {
		return limit
	}
	margin := 8
	selection, _ := densityLatitudeSelectionFor(zone)
	dense, _ := ctx.Value(denseCollisionKey{}).(bool)
	if selection.tier >= 3 && dense {
		// 塔可利用21格内部空间；直连、配送器及保守塔候选在装箱时另保留更长接头的余量。
		margin = 4
		if selection.tier == 5 {
			margin = 0
		}
	}
	return math.Min(limit, math.Max(1, float64(band.high-band.low+1-margin)))
}

// validateDensityLatitudePlan 职责：证明最终编码仍处于用户选择的区域；使用：所有密铺模式的共同出口；边界：缺少球面结果、越区或跨带均拒绝，防止适配器静默回退。
func validateDensityLatitudePlan(layout *SurfaceLayout, plan *SurfacePlan) error {
	if layout == nil || layout.LatitudeZone == "" {
		return nil
	}
	band, label, err := densityLayoutBand(layout)
	if err != nil {
		return err
	}
	if plan == nil || len(plan.Bands) == 0 {
		return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "所选建造区域没有经过球面布局校验"}
	}
	for _, area := range plan.Bands {
		if area.MinLatitudeGrid < band.low || area.MaxLatitudeGrid > band.high || area.LongitudeSegments != band.segments {
			return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("完整蓝图超出%s，请降低产量或选择更宽的建造位置", label)}
		}
	}
	plan.LatitudeZone, plan.LatitudeZoneLabel = layout.LatitudeZone, label
	plan.PlacementHint = densityLatitudePositionHint(layout.LatitudeZone)
	return nil
}

// densityLatitudePositionHint 职责：统一中心与南北边缘的真实放置语义；使用：结果UI和导出蓝图共同复用；边界：位置名称不替代最终已验证起点区间。
func densityLatitudePositionHint(zone string) string {
	selection, _ := densityLatitudeSelectionFor(zone)
	switch selection.position {
	case "center":
		return "以本环中心为目标，按已验证起点范围放置"
	case "north":
		return "靠近本环北侧分界线放置"
	case "south":
		return "靠近本环南侧分界线放置"
	default:
		return "按已验证起点范围放置"
	}
}

// densityLatitudePlacementHint 职责：把已核验区域和起点保留到可复制的蓝图说明；使用：最终编码；边界：没有明确选区的旧结果不臆造地区，不把建议位置说成任意纬度均可放置。
func densityLatitudePlacementHint(plan *SurfacePlan) string {
	if plan == nil || plan.LatitudeZoneLabel == "" {
		return ""
	}
	latitude := "赤道（0°）"
	if math.Abs(plan.LatitudeDegrees) >= .005 {
		direction := "北纬"
		if plan.LatitudeDegrees < 0 {
			direction = "南纬"
		}
		latitude = fmt.Sprintf("%s%.2f°", direction, math.Abs(plan.LatitudeDegrees))
	}
	hint := fmt.Sprintf("；建造位置：%s，蓝图光标建议位置：%s，请以默认方向整张粘贴", plan.LatitudeZoneLabel, latitude)
	if plan.SafeLatitudeMinDegrees != nil && plan.SafeLatitudeMaxDegrees != nil {
		low, high := *plan.SafeLatitudeMinDegrees, *plan.SafeLatitudeMaxDegrees
		format := func(degrees float64) string {
			if math.Abs(degrees) < .005 {
				return "赤道0°"
			}
			if degrees < 0 {
				return fmt.Sprintf("南纬%.2f°", -degrees)
			}
			return fmt.Sprintf("北纬%.2f°", degrees)
		}
		if high < 0 {
			low, high = high, low
		}
		placement := plan.PlacementHint
		if placement == "" {
			placement = densityLatitudePositionHint(plan.LatitudeZone)
		}
		hint += fmt.Sprintf("；默认方向已验证光标范围：%s至%s；%s，整图保持在同一纬带内", format(low), format(high), placement)
	}
	return hint
}
