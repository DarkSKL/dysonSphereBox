package build

// 文件职责：把玩家可理解的建造区域转换为严格的原生纬带范围。
// 使用：超密度选区、候选锚点及最终结果核验；边界：仅原版200分段星球，不为失败结果自动换区。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
	"sort"
)

// DensityLatitudeOption 职责：提供简单放置区域选项；使用：数据集元信息与界面下拉；边界：选项不承诺任意产量均可放置。
type DensityLatitudeOption struct {
	// Value 职责：保存稳定区域键；使用：SurfaceLayout；边界：未知键拒绝。
	Value string `json:"value"`
	// Label 职责：保存玩家可读名称；使用：下拉；边界：不要求输入纬圈索引。
	Label string `json:"label"`
	// Description 职责：说明区域位置及容量差异；使用：选项提示；边界：不是游戏放置保证。
	Description string `json:"description"`
}

// DensityLatitudeOptions 职责：返回支持的原版建造区域；使用：界面发现能力；边界：每次返回独立切片。
func DensityLatitudeOptions() []DensityLatitudeOption {
	return []DensityLatitudeOption{
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
}

// densityLatitudeBand 职责：通过当前网格表解析选区；使用：请求与候选核验；边界：不硬编码纬带边界，不支持任意字符串或其它网格。
func densityLatitudeBand(zone string, segment int) (surfacePackedBand, string, error) {
	selection, ok := densityLatitudeSelectionFor(zone)
	if !ok || segment != defaultSurfaceGridSegment {
		return surfacePackedBand{}, "", &blueprint.Error{Code: "GEN_E_SURFACE", Message: "未知密铺建造区域或不支持的星球网格"}
	}
	// 逐档接入生成：尚未接入的第4档及极区键不能误当作已验证单Area生成。
	if selection.tier > 3 {
		return surfacePackedBand{}, "", &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "该档位尚未接入对应的纬带生成方案"}
	}
	label := ""
	for _, option := range DensityLatitudeOptions() {
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
	band, _, err := densityLatitudeBand(layout.LatitudeZone, layout.GridSegment)
	if err != nil {
		return nil
	}
	preferred := bandCenter(band) - (height-1)/2
	anchors := []int{}
	margin := 2
	selection, _ := densityLatitudeSelectionFor(layout.LatitudeZone)
	if selection.tier == 3 {
		// 狭窄原生带可使用全部格行；完整碰撞检查决定落点，不能再重复扣除装箱已预留的线路余量。
		margin = 0
	}
	for anchor := band.low + margin; anchor+height-1 <= band.high-margin; anchor++ {
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
	band, _, err := densityLatitudeBand(zone, defaultSurfaceGridSegment)
	if err != nil {
		return limit
	}
	margin := 8
	selection, _ := densityLatitudeSelectionFor(zone)
	dense, _ := ctx.Value(denseCollisionKey{}).(bool)
	if selection.tier == 3 && dense {
		// 塔可利用21格内部空间；直连、配送器及保守塔候选在装箱时另保留更长接头的余量。
		margin = 4
	}
	return math.Min(limit, math.Max(1, float64(band.high-band.low+1-margin)))
}

// validateDensityLatitudePlan 职责：证明最终编码仍处于用户选择的区域；使用：所有密铺模式的共同出口；边界：缺少球面结果、越区或跨带均拒绝，防止适配器静默回退。
func validateDensityLatitudePlan(layout *SurfaceLayout, plan *SurfacePlan) error {
	if layout == nil || layout.LatitudeZone == "" {
		return nil
	}
	band, label, err := densityLatitudeBand(layout.LatitudeZone, layout.GridSegment)
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
	hint := fmt.Sprintf("；建造位置：%s，建议放置起点：%s，请以默认方向整张粘贴", plan.LatitudeZoneLabel, latitude)
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
		hint += fmt.Sprintf("；默认方向已验证起点范围：%s至%s；%s，整图保持在同一纬带内", format(low), format(high), placement)
	}
	return hint
}
