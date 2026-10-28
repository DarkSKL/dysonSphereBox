package build

// 文件职责：维护当前游戏的纬圈分段规则和布局请求边界。
// 使用：有限纬带装箱及原生 Area 编码前校验网格。
// 边界：不拥有装箱、碰撞或放置验收；未知网格在请求入口拒绝。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
)

// defaultSurfaceGridSegment 职责：指定原版半径 200 行星的主网格；使用：请求缺省；边界：不推断未知行星。
const defaultSurfaceGridSegment = 200

// normalizeSurfaceLayout 职责：在布局前校验并复制纬圈配置；使用：Generate 的入口；边界：旧请求返回 nil，不改写调用方配置。
func normalizeSurfaceLayout(layout *SurfaceLayout) (*SurfaceLayout, error) {
	if layout == nil || layout.Mode == "" || layout.Mode == "planar" {
		return nil, nil
	}
	normalized := *layout
	if normalized.Mode != "autoTropic" && normalized.Mode != "explicitTropic" {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "未知行星布局模式"}
	}
	if normalized.GridSegment == 0 {
		normalized.GridSegment = defaultSurfaceGridSegment
	}
	if normalized.GridSegment < 40 || normalized.GridSegment > 512 || normalized.GridSegment%4 != 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "目标行星主网格分段数必须为 40 到 512 的 4 的倍数"}
	}
	if normalized.GridSegment != defaultSurfaceGridSegment {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "当前仅支持已核对的原版 200 分段行星"}
	}
	if normalized.Mode == "autoTropic" {
		normalized.LatitudeGridIndex = 0
	}
	if normalized.LatitudeGridIndex < -normalized.GridSegment*5/4 || normalized.LatitudeGridIndex > normalized.GridSegment*5/4 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬圈索引超出行星范围"}
	}
	return &normalized, nil
}

// longitudeSegmentCountThresholds 保存游戏 segmentTable 的连续值区间上界。
// 职责：用紧凑阈值表复刻 PlanetGrid.segmentTable 的 0..499 查询范围。
// 使用：longitudeSegmentCount 按离赤道指数查当前纬圈经度段数。
// 边界：500 及以上由游戏规则使用 (index+49)/100*100，不使用此表。
var longitudeSegmentCountThresholds = [...]int{0, 7, 15, 19, 27, 39, 53, 72, 90, 113, 139, 176, 220, 270, 350, 450, 499}

// longitudeSegmentCountValues 与阈值表逐项对应，来源为当前游戏 PlanetGrid.segmentTable。
// 职责：提供纬圈经度段数离散值。
// 使用：longitudeSegmentCount 查询。
// 边界：不推导建筑碰撞或可建区域。
var longitudeSegmentCountValues = [...]int{1, 4, 8, 16, 20, 32, 40, 60, 80, 100, 120, 160, 200, 240, 300, 400, 500}

// longitudeSegmentCount 复刻当前游戏 PlanetGrid.DetermineLongitudeSegmentCount。
// 职责：按纬向细分格索引估算一整圈的离散经度分段数。
// 使用：球面布局容量选择和单 Area 纬圈段稳定性检查。
// 边界：映射规则基于 build 25955035；不能替代不同游戏版本的回归测试。
func longitudeSegmentCount(latitudeGrid, segment int) int {
	latitudeGrid = absInt(latitudeGrid)
	if latitudeGrid > 0 {
		latitudeGrid--
	}
	latitudeIndex := latitudeGrid / 5
	denominator := float64(segment) / 4
	index := int(math.Ceil(math.Abs(math.Cos(float64(latitudeIndex)/denominator*math.Pi/2)) * float64(segment)))
	if index >= 500 {
		return (index + 49) / 100 * 100
	}
	for i, upper := range longitudeSegmentCountThresholds {
		if index <= upper {
			return longitudeSegmentCountValues[i]
		}
	}
	return 500
}

// absInt 返回整数绝对值。
// 职责：统一纬圈南北半球索引折算。
// 使用：网格分段计算。
// 边界：输入为受请求范围限制的普通纬圈整数，不处理最小整数溢出。
func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// candidateFitsRing 验证完整区域纵向范围处于同一纬圈经度段，且横向未超过整圈。
// 职责：复刻游戏多 Area 跨 Tropic 与经度周期容量的关键拒绝条件。
// 使用：prepareSurfaceEncoding 独立验证所用区域。
// 边界：不检查具体建筑 collider，容量格不额外包含建筑间距余量。
func candidateFitsRing(center, lowOffset, highOffset, width, segment int) bool {
	maxLatitudeGrid := segment * 5 / 4
	low, high := center-lowOffset, center+highOffset
	if low < -maxLatitudeGrid || high > maxLatitudeGrid {
		return false
	}
	longitudeSegments := longitudeSegmentCount(center, segment)
	if width > longitudeSegments*5 {
		return false
	}
	for row := low; row <= high; row++ {
		if longitudeSegmentCount(row, segment) != longitudeSegments {
			return false
		}
	}
	return true
}

// makeSurfacePlan 创建可解释的纬圈建议结果。
// 职责：汇总目标、最终纬圈、圈容量和高度范围。
// 使用：prepareSurfaceEncoding 在候选通过后调用。
// 边界：碰撞覆盖由实际验证器在运行成功后设置。
func makeSurfacePlan(mode string, segment, preferred, selected, width, height int) *SurfacePlan {
	longitudeSegments := longitudeSegmentCount(selected, segment)
	return &SurfacePlan{
		Mode: mode, GridSegment: segment, PreferredLatitudeGrid: preferred, SelectedLatitudeGrid: selected,
		LatitudeDegrees: float64(selected) * 360 / float64(segment*5), LongitudeSegments: longitudeSegments,
		RingGridCells: longitudeSegments * 5, LayoutWidth: width, LayoutHeight: height,
		ShiftedGridRows:   selected - preferred,
		CollisionCoverage: "none", PlacementStatus: "partial",
	}
}
