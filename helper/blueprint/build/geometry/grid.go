package geometry

// 文件职责：统一原版经度分段表和两种不同舍入语义的格距查询。
// 使用：布局容量、实际粘贴和游戏端口度量；边界：双精度整数纬格与float32度量算法分别具名，不能机械互换。

import (
	"fmt"
	"math"
)

// longitudeThresholds/longitudeValues 职责：保存唯一PlanetGrid分段事实；使用：整数网格及单精度度量共用；边界：不是建筑碰撞或区域可建证明。
var longitudeThresholds = [...]int{0, 7, 15, 19, 27, 39, 53, 72, 90, 113, 139, 176, 220, 270, 350, 450, 499}
var longitudeValues = [...]int{1, 4, 8, 16, 20, 32, 40, 60, 80, 100, 120, 160, 200, 240, 300, 400, 500}

// SegmentTableValue 职责：查询共同整数分段表；使用：保留不同上层舍入方法的纯事实读取；边界：负索引和超出已核对算法范围的输入失败。
func SegmentTableValue(index int) (int, error) {
	if index < 0 || index > 1000000 {
		return 0, fmt.Errorf("经度分段表索引无效")
	}
	if index >= 500 {
		return (index + 49) / 100 * 100, nil
	}
	for i, upper := range longitudeThresholds {
		if index <= upper {
			return longitudeValues[i], nil
		}
	}
	return 0, fmt.Errorf("经度分段表缺少匹配事实")
}

// LongitudeSegmentsAtGrid 职责：按整数纬向细格查询真实经度分段；使用：球面布局容量；边界：仅支持已核对星球和正负250格范围。
func LongitudeSegmentsAtGrid(latitude int, planet Planet) (int, error) {
	if planet != Vanilla200 || latitude < -250 || latitude > 250 {
		return 0, fmt.Errorf("未知星球或超界纬向细格")
	}
	if latitude < 0 {
		latitude = -latitude
	}
	if latitude > 0 {
		latitude--
	}
	index := int(math.Ceil(math.Abs(math.Cos(float64(latitude/5)/50*math.Pi/2)) * 200))
	return SegmentTableValue(index)
}

// LatitudeStep32 职责：提供游戏单精度纬向细格弧度；使用：正式Area和原生框架；边界：不在调用方复制π舍入顺序。
func LatitudeStep32(planet Planet) (Radians, error) {
	if planet != Vanilla200 {
		return 0, fmt.Errorf("未知星球纬向步长")
	}
	return Radians(float32(2*float32(math.Pi)) / 1000), nil
}

// PasteLongitudeStep32 职责：按实际粘贴起点纬度选择float32经向步长；使用：主/子Area正式映射；边界：先单精度除法再半值远离零取整，不借用度量算法。
func PasteLongitudeStep32(latitude Radians, planet Planet) (Radians, error) {
	step, err := LatitudeStep32(planet)
	if err != nil {
		return 0, err
	}
	if err := finite(float64(latitude)); err != nil {
		return 0, err
	}
	if math.Abs(float64(latitude)) > math.Pi/2+1e-7 {
		return 0, fmt.Errorf("粘贴纬度越界")
	}
	grid := float32(latitude) / float32(step)
	if grid > 0 {
		grid += .5
	} else {
		grid -= .5
	}
	segments, err := LongitudeSegmentsAtGrid(int(grid), planet)
	if err != nil {
		return 0, err
	}
	return Radians(float32(2*float32(math.Pi)) / float32(segments*5)), nil
}

// Direction 职责：以明确世界框架保存无量纲方向；使用：局部格距经纬选择；边界：零方向与非有限值拒绝，不借用WorldLength表示方向。
type Direction struct {
	// Frame 职责：绑定方向所属空间；使用：与位置匹配；边界：须为相同PlanetWorld。
	Frame Frame
	// X/Y/Z 职责：保存方向分量；使用：保持原有dot计算；边界：不自动归一化改变0.7分支。
	X, Y, Z float64
}

// LocalGridSize32 职责：以游戏单精度规则计算当前位置沿指定方向的真实格长；使用：分拣器端口修正；边界：球面身份、径向位置及方向无效时明确失败。
func LocalGridSize32(position WorldPoint, direction Direction) (WorldLength, error) {
	if err := position.Validate(); err != nil {
		return 0, err
	}
	if position.Frame.Space != PlanetWorld || direction.Frame != position.Frame {
		return 0, fmt.Errorf("球面格距的位置和方向框架不匹配")
	}
	if err := finite(direction.X, direction.Y, direction.Z); err != nil {
		return 0, err
	}
	if err := finite(float64(float32(direction.X)), float64(float32(direction.Y)), float64(float32(direction.Z))); err != nil {
		return 0, err
	}
	if err := checkMetricPoint32(position); err != nil {
		return 0, err
	}
	if position.X == 0 && position.Y == 0 && position.Z == 0 || direction.X == 0 && direction.Y == 0 && direction.Z == 0 {
		return 0, fmt.Errorf("球面格距缺少有效位置或方向")
	}
	value, err := localGridSize32([3]float64{float64(position.X), float64(position.Y), float64(position.Z)}, [3]float64{direction.X, direction.Y, direction.Z})
	if err != nil {
		return 0, err
	}
	result := WorldLength(value)
	if err := finite(float64(result)); err != nil {
		return 0, err
	}
	if result <= 0 {
		return 0, fmt.Errorf("球面格距未形成正长度")
	}
	return result, nil
}

// SegmentsAcross32 职责：保持游戏归一化、接缝DeltaAngle及经纬加权的细格跨度；使用：最终分拣器几何；边界：不能把球面跨度当作世界弦长。
func SegmentsAcross32(reference, first, second WorldPoint) (GridLength, error) {
	for _, point := range []WorldPoint{reference, first, second} {
		if err := point.Validate(); err != nil {
			return 0, err
		}
		if point.Frame.Space != PlanetWorld || point.Frame != reference.Frame {
			return 0, fmt.Errorf("球面跨度的框架不匹配")
		}
		if point.X == 0 && point.Y == 0 && point.Z == 0 {
			return 0, fmt.Errorf("球面跨度缺少径向位置")
		}
		if err := checkMetricPoint32(point); err != nil {
			return 0, err
		}
	}
	values := func(point WorldPoint) [3]float64 {
		return [3]float64{float64(point.X), float64(point.Y), float64(point.Z)}
	}
	value, err := segmentsAcross32(values(reference), values(first), values(second))
	if err != nil {
		return 0, err
	}
	result := GridLength(value)
	return result, finite(float64(result))
}

// checkMetricPoint32 职责：拒绝Vector3量化后溢出或不能归一化的位置；使用：游戏度量入口；边界：不让极小向量的零单位向量成为成功原点回退。
func checkMetricPoint32(point WorldPoint) error {
	length := metricInput([3]float64{float64(point.X), float64(point.Y), float64(point.Z)}).length()
	if err := finite(float64(length)); err != nil {
		return err
	}
	if length <= 1e-5 {
		return fmt.Errorf("球面度量位置不能在游戏单精度中归一化")
	}
	return nil
}
