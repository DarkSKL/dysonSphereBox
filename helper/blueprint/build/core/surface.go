package build

// 文件职责：维护当前游戏的纬圈分段规则和布局请求边界。
// 使用：有限纬带装箱及原生 Area 编码前校验网格。
// 边界：不拥有装箱、碰撞或放置验收；未知网格在请求入口拒绝。

import (
	"DysonSphereBox/helper/blueprint"
	buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"
)

// defaultSurfaceGridSegment 职责：指定原版半径 200 行星的主网格；使用：请求缺省；边界：不推断未知行星。
const defaultSurfaceGridSegment = 200

// NormalizeSurfaceLayout 职责：公开同一套球面请求校验；使用：方案导出复用；边界：复制配置，拒绝未知选区，不执行生成。
func NormalizeSurfaceLayout(layout *SurfaceLayout) (*SurfaceLayout, error) {
	return normalizeSurfaceLayout(layout)
}

// normalizeSurfaceLayout 职责：在布局前校验并复制纬圈配置；使用：Generate 的入口；边界：旧请求返回 nil，不改写调用方配置。
func normalizeSurfaceLayout(layout *SurfaceLayout) (*SurfaceLayout, error) {
	if layout != nil && layout.LatitudeZone != "" && (layout.Mode == "" || layout.Mode == "planar") {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "建造区域选择需要球面模式"}
	}
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
	if normalized.LatitudeZone != "" {
		band, _, err := densityLayoutBand(&normalized)
		if err != nil {
			return nil, err
		}
		normalized.Mode, normalized.LatitudeGridIndex = "explicitTropic", bandCenter(band)
	}
	if normalized.Mode == "autoTropic" {
		normalized.LatitudeGridIndex = 0
	}
	if normalized.LatitudeGridIndex < -normalized.GridSegment*5/4 || normalized.LatitudeGridIndex > normalized.GridSegment*5/4 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬圈索引超出行星范围"}
	}
	return &normalized, nil
}

// longitudeSegmentCount 复刻当前游戏 PlanetGrid.DetermineLongitudeSegmentCount。
// 职责：按纬向细分格索引估算一整圈的离散经度分段数。
// 使用：球面布局容量选择和单 Area 纬圈段稳定性检查。
// 边界：映射规则基于 build 25955035；不能替代不同游戏版本的回归测试。
func longitudeSegmentCount(latitudeGrid, segment int) int {
	if segment != defaultSurfaceGridSegment {
		// 非法外部网格由normalizeSurfaceLayout返回error，本私有调用不得绕过该事实校验。
		panic("未经验证的内部星球网格")
	}
	value, err := buildgeometry.LongitudeSegmentsAtGrid(latitudeGrid, buildgeometry.Vanilla200)
	if err != nil {
		panic(err)
	}
	return value
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
