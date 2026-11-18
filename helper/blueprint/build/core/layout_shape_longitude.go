package build

// 文件职责：区分局部近方形工厂与环绕行星的纬向长条；使用：正式物理包络度量之前。
// 边界：不改变实际坐标、物理规则或区域许可；只有全部实体足迹位于已核对的原生极区帽内时允许经度环绕。
import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// physicalLongitudeEnvelope 职责：以线性扫描限制非极区的局部经度包络；使用：共同球面形状检查。
// 边界：在连续半圈内时，相对任一成员的有符号角范围就是唯一连续包络；超出半圈只作拒绝，不把该范围冒充最小圆弧。
func physicalLongitudeEnvelope(points []placement.Vector) error {
	north := densityLatitudeTierBands(5, 1, defaultSurfaceGridSegment)
	south := densityLatitudeTierBands(5, -1, defaultSurfaceGridSegment)
	if len(north) == 0 || len(south) == 0 {
		return fmt.Errorf("缺少原生极区边界事实")
	}
	northY := math.Sin(float64(north[0].low) * 2 * math.Pi / 1000)
	southY := math.Sin(float64(south[len(south)-1].high) * 2 * math.Pi / 1000)
	var reference placement.Vector
	known := false
	minimum, maximum := 0., 0.
	northCap, southCap := true, true
	for _, point := range points {
		length := math.Sqrt(point.Dot(point))
		if !finitePositive(length) {
			return fmt.Errorf("经度包络缺少有效地表位置")
		}
		y := point[1] / length
		northCap = northCap && y >= northY
		southCap = southCap && y <= southY
		// float32在极点可能留下约1e-8的余弦尾差；极点的经度无定义，不让该尾差决定整厂展开方向。
		if point[0]*point[0]+point[2]*point[2] > length*length*1e-14 {
			if !known {
				reference, known = point, true
			}
			angle := math.Atan2(point[2]*reference[0]-point[0]*reference[2], point[0]*reference[0]+point[2]*reference[2])
			minimum, maximum = math.Min(minimum, angle), math.Max(maximum, angle)
		}
		if !northCap && !southCap && maximum-minimum > math.Pi+1e-9 {
			return &blueprint.Error{Code: "GEN_E_SHAPE", Message: "非极区工厂经度包络超过半圈，环绕长条不能作为近方形矩形；需缩短经向布局并利用实际纬带高度"}
		}
	}
	return nil
}
