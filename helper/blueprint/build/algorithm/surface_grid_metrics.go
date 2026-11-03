package build

// 文件职责：按游戏单精度运算顺序复现分拣器检查使用的格网度量。
// 使用：端口修正及跨纬带动态检查；边界：只支持已核对的 200 分段行星，不调用 Physics。

import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfaceMetricVector 职责：保留 Unity Vector3 的单精度算术边界；使用：格网度量；边界：不替换装配阶段的双精度坐标。
type surfaceMetricVector [3]float32

// surfaceMetricInput 职责：把检查输入量化到游戏 Vector3；使用：度量入口；边界：拒绝策略由外层坐标验证负责。
func surfaceMetricInput(v placement.Vector) surfaceMetricVector {
	return surfaceMetricVector{float32(v[0]), float32(v[1]), float32(v[2])}
}

// length 职责：保留 Vector3.magnitude 的乘加与开方舍入；使用：单精度归一化及距离；边界：不把单位向量模长强制设为一。
func (v surfaceMetricVector) length() float32 {
	square := float32(float32(v[0]*v[0])+float32(v[1]*v[1])) + float32(v[2]*v[2])
	return float32(math.Sqrt(float64(square)))
}

// unit 职责：复现 Vector3.Normalize 的阈值和逐分量除法；使用：度量参考方向；边界：极小向量返回零。
func (v surfaceMetricVector) unit() surfaceMetricVector {
	length := v.length()
	if length <= 1e-5 {
		return surfaceMetricVector{}
	}
	return surfaceMetricVector{v[0] / length, v[1] / length, v[2] / length}
}

// surfaceMetricAbs 职责：保留 Mathf.Abs 的浮点位宽；使用：格网纬度及角度；边界：不改变运算顺序。
func surfaceMetricAbs(v float32) float32 { return float32(math.Abs(float64(v))) }

// surfaceMetricSegments 职责：按纬度弧度选择游戏离散经度段数；使用：两种格网度量；边界：纬度商须先单精度取整，不能借用双精度细格查询跨过临界纬带。
func surfaceMetricSegments(latitude float32) float32 {
	index := float32(math.Floor(float64(max(float32(0), surfaceMetricAbs(float32(latitude/float32(2*float32(math.Pi)))*200)-float32(.1)))))
	angle := float32(float32(index/50)*float32(math.Pi)) * .5
	tableIndex := int(math.Ceil(float64(surfaceMetricAbs(float32(math.Cos(float64(angle)))) * 200)))
	for i, upper := range longitudeSegmentCountThresholds {
		if tableIndex <= upper {
			return float32(longitudeSegmentCountValues[i])
		}
	}
	return float32((tableIndex + 49) / 100 * 100)
}

// surfaceLocalGridSize 职责：复现 PlanetGrid.CalcLocalGridSize 的单精度分支和格长；使用：端口纠错量合法性；边界：保留游戏 0.7 双精度比较，不能以容差换错经纬分支。
func surfaceLocalGridSize(position, direction placement.Vector) float64 {
	p, d := surfaceMetricInput(position), surfaceMetricInput(direction)
	east := (surfaceMetricVector{-p[2], 0, p[0]}).unit()
	dot := float32(float32(east[0]*d[0])+float32(east[1]*d[1])) + float32(east[2]*d[2])
	radius := p.length()
	if float64(surfaceMetricAbs(dot)) < .7 {
		return float64(float32(float32(radius*float32(math.Pi))*2) / 1000)
	}
	latitude := float32(math.Asin(float64(p.unit()[1])))
	step := float32(float32(float32(math.Cos(float64(latitude)))*float32(math.Pi))*2) / (surfaceMetricSegments(latitude) * 5)
	return float64(radius * max(float32(.0031415927), step))
}

// surfaceSegmentsAcross 职责：保留游戏归一化、DeltaAngle 和经纬加权的单精度顺序；使用：分拣器跨度与最小距离；边界：只核验数学，不推断科技或地形。
func surfaceSegmentsAcross(reference, first, second placement.Vector) float64 {
	r, a, b := surfaceMetricInput(reference).unit(), surfaceMetricInput(first).unit(), surfaceMetricInput(second).unit()
	latitude := float32(math.Asin(float64(r[1])))
	longitudeStep := max(float32(.0048), float32(float32(float32(math.Cos(float64(latitude)))*float32(math.Pi))*2)/(surfaceMetricSegments(latitude)*5))
	latitudeStep := float32(2*float32(math.Pi)) / 1000
	firstAngle := float32(math.Atan2(float64(a[0]), float64(-a[2]))) * float32(57.29578)
	secondAngle := float32(math.Atan2(float64(b[0]), float64(-b[2]))) * float32(57.29578)
	// Mathf.DeltaAngle 先以度数 Repeat 到 [0,360]，跨经度接缝时不能直接使用双精度余数。
	angle := secondAngle - firstAngle
	angle -= float32(math.Floor(float64(angle/360))) * 360
	angle = max(float32(0), min(float32(360), angle))
	if angle > 180 {
		angle -= 360
	}
	longitudeGap := surfaceMetricAbs(angle * float32(float32(math.Pi)/180))
	latitudeGap := surfaceMetricAbs(float32(math.Asin(float64(a[1]))) - float32(math.Asin(float64(b[1]))))
	total := latitudeGap + longitudeGap
	longitudeWeight, latitudeWeight := float32(0), float32(1)
	if total > 0 {
		longitudeWeight, latitudeWeight = longitudeGap/total, latitudeGap/total
	}
	step := float32(longitudeStep*longitudeWeight) + float32(latitudeStep*latitudeWeight)
	delta := surfaceMetricVector{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
	return float64(delta.length() / step)
}
