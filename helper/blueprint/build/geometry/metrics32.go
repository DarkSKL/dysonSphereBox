package geometry

// 文件职责：保留已核对的游戏单精度格距数值内核。
// 使用：由检查过身份和有限值的公共入口调用；边界：不导出无单位入口，不以双精度改写游戏的舍入分支。

import "math"

// metricVector 职责：保留 Unity Vector3 的单精度算术边界；使用：格网度量；边界：不替换装配阶段的双精度坐标。
type metricVector [3]float32

// metricInput 职责：把检查输入量化到游戏 Vector3；使用：度量入口；边界：拒绝策略由外层坐标验证负责。
func metricInput(v [3]float64) metricVector {
	return metricVector{float32(v[0]), float32(v[1]), float32(v[2])}
}

// length 职责：保留 Vector3.magnitude 的乘加与开方舍入；使用：单精度归一化及距离；边界：不把单位向量模长强制设为一。
func (v metricVector) length() float32 {
	square := float32(float32(v[0]*v[0])+float32(v[1]*v[1])) + float32(v[2]*v[2])
	return float32(math.Sqrt(float64(square)))
}

// unit 职责：复现 Vector3.Normalize 的阈值和逐分量除法；使用：度量参考方向；边界：极小向量返回零。
func (v metricVector) unit() metricVector {
	length := v.length()
	if length <= 1e-5 {
		return metricVector{}
	}
	return metricVector{v[0] / length, v[1] / length, v[2] / length}
}

// metricAbs 职责：保留 Mathf.Abs 的浮点位宽；使用：格网纬度及角度；边界：不改变运算顺序。
func metricAbs(v float32) float32 { return float32(math.Abs(float64(v))) }

// metricSegments 职责：按纬度弧度选择游戏离散经度段数；使用：两种格网度量；边界：纬度商须先单精度取整，不能借用双精度细格查询跨过临界纬带。
func metricSegments(latitude float32) (float32, error) {
	index := float32(math.Floor(float64(max(float32(0), metricAbs(float32(latitude/float32(2*float32(math.Pi)))*200)-float32(.1)))))
	angle := float32(float32(index/50)*float32(math.Pi)) * .5
	tableIndex := int(math.Ceil(float64(metricAbs(float32(math.Cos(float64(angle)))) * 200)))
	value, err := SegmentTableValue(tableIndex)
	return float32(value), err
}

// localGridSize32 职责：复现 PlanetGrid.CalcLocalGridSize 的单精度分支和格长；使用：端口纠错量合法性；边界：保留游戏 0.7 双精度比较，不能以容差换错经纬分支。
func localGridSize32(position, direction [3]float64) (float64, error) {
	p, d := metricInput(position), metricInput(direction)
	east := (metricVector{-p[2], 0, p[0]}).unit()
	dot := float32(float32(east[0]*d[0])+float32(east[1]*d[1])) + float32(east[2]*d[2])
	radius := p.length()
	if float64(metricAbs(dot)) < .7 {
		return float64(float32(float32(radius*float32(math.Pi))*2) / 1000), nil
	}
	latitude := float32(math.Asin(float64(p.unit()[1])))
	segments, err := metricSegments(latitude)
	if err != nil {
		return 0, err
	}
	step := float32(float32(float32(math.Cos(float64(latitude)))*float32(math.Pi))*2) / (segments * 5)
	return float64(radius * max(float32(.0031415927), step)), nil
}

// segmentsAcross32 职责：保留游戏归一化、DeltaAngle 和经纬加权的单精度顺序；使用：分拣器跨度与最小距离；边界：只核验数学，不推断科技或地形。
func segmentsAcross32(reference, first, second [3]float64) (float64, error) {
	r, a, b := metricInput(reference).unit(), metricInput(first).unit(), metricInput(second).unit()
	latitude := float32(math.Asin(float64(r[1])))
	segments, err := metricSegments(latitude)
	if err != nil {
		return 0, err
	}
	longitudeStep := max(float32(.0048), float32(float32(float32(math.Cos(float64(latitude)))*float32(math.Pi))*2)/(segments*5))
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
	longitudeGap := metricAbs(angle * float32(float32(math.Pi)/180))
	latitudeGap := metricAbs(float32(math.Asin(float64(a[1]))) - float32(math.Asin(float64(b[1]))))
	total := latitudeGap + longitudeGap
	longitudeWeight, latitudeWeight := float32(0), float32(1)
	if total > 0 {
		longitudeWeight, latitudeWeight = longitudeGap/total, latitudeGap/total
	}
	step := float32(longitudeStep*longitudeWeight) + float32(latitudeStep*latitudeWeight)
	delta := metricVector{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
	return float64(delta.length() / step), nil
}
