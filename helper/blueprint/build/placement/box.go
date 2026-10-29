package placement

// 文件职责：提供三维定向盒的独立几何运算。
// 使用：球面布局把游戏数值碰撞盒转换为世界坐标后执行窄相位。
// 边界：不认识建筑类别或合法接线豁免；调用方先验证盒及规则来源。

import (
	"math"
	"sort"
)

// Vector 职责：保存世界或原型局部三维向量；使用：盒几何；边界：下标依次为 x、y、z，不包含格网单位。
type Vector [3]float64

// Add 职责：相加向量；使用：坐标平移；边界：不修改操作数。
func (v Vector) Add(w Vector) Vector { return Vector{v[0] + w[0], v[1] + w[1], v[2] + w[2]} }

// Sub 职责：相减向量；使用：相对中心和间距；边界：不修改操作数。
func (v Vector) Sub(w Vector) Vector { return Vector{v[0] - w[0], v[1] - w[1], v[2] - w[2]} }

// Scale 职责：按标量缩放；使用：正交投影与基向量组合；边界：不归一化。
func (v Vector) Scale(s float64) Vector { return Vector{v[0] * s, v[1] * s, v[2] * s} }

// Dot 职责：计算内积；使用：分离轴投影；边界：不要求单位向量。
func (v Vector) Dot(w Vector) float64 { return v[0]*w[0] + v[1]*w[1] + v[2]*w[2] }

// Cross 职责：计算叉积；使用：定向盒边与边的分离轴；边界：平行向量结果为零。
func (v Vector) Cross(w Vector) Vector {
	return Vector{v[1]*w[2] - v[2]*w[1], v[2]*w[0] - v[0]*w[2], v[0]*w[1] - v[1]*w[0]}
}

// Box 职责：保存世界中心、半尺寸和正交基；使用：真实数值碰撞盒的宽窄相位；边界：须先通过 Valid，不描述扫掠或胶囊。
type Box struct {
	// Center 职责：保存世界坐标中心；使用：投影；边界：单位为游戏世界长度。
	Center Vector
	// HalfSize 职责：保存原型局部三个半尺寸；使用：投影半径；边界：严格正数。
	HalfSize Vector
	// Axes 职责：保存局部 x/y/z 的世界单位基；使用：旋转定向盒；边界：须相互正交。
	Axes [3]Vector
}

// Valid 职责：拒绝非有限或非正交的碰撞几何；使用：占位登记前；边界：容许 float32 旋转的微小归一化误差。
func (box Box) Valid() bool {
	for i := 0; i < 3; i++ {
		if math.IsNaN(box.Center[i]) || math.IsInf(box.Center[i], 0) || math.IsNaN(box.HalfSize[i]) || math.IsInf(box.HalfSize[i], 0) || box.HalfSize[i] <= 0 {
			return false
		}
		if length := box.Axes[i].Dot(box.Axes[i]); math.IsNaN(length) || math.Abs(length-1) > 1e-4 {
			return false
		}
		for j := 0; j < i; j++ {
			if math.Abs(box.Axes[i].Dot(box.Axes[j])) > 1e-4 {
				return false
			}
		}
	}
	return true
}

// Bounds 职责：计算世界轴对齐包围盒；使用：空间哈希宽相位；边界：输入应有效，不能用包围盒相交直接认定定向盒冲突。
func (box Box) Bounds() (Vector, Vector) {
	var radius Vector
	for i, axis := range box.Axes {
		for dimension := 0; dimension < 3; dimension++ {
			radius[dimension] += math.Abs(axis[dimension]) * box.HalfSize[i]
		}
	}
	return box.Center.Sub(radius), box.Center.Add(radius)
}

// SegmentDistanceSquared 职责：求闭线段到定向盒的精确最近距离；使用：带节点球和动态胶囊窄相位；边界：盒须有效，距离为零也包括边界接触。
func (box Box) SegmentDistanceSquared(first, second Vector) float64 {
	var origin, direction Vector
	breaks := []float64{0, 1}
	for axis := range origin {
		origin[axis] = first.Sub(box.Center).Dot(box.Axes[axis])
		direction[axis] = second.Sub(first).Dot(box.Axes[axis])
		if math.Abs(direction[axis]) > 1e-12 {
			for _, edge := range []float64{-box.HalfSize[axis], box.HalfSize[axis]} {
				if t := (edge - origin[axis]) / direction[axis]; t > 0 && t < 1 {
					breaks = append(breaks, t)
				}
			}
		}
	}
	sort.Float64s(breaks)
	distance := func(t float64) float64 {
		total := 0.
		for axis := range origin {
			gap := max(0, math.Abs(origin[axis]+direction[axis]*t)-box.HalfSize[axis])
			total += gap * gap
		}
		return total
	}
	minimum := math.Inf(1)
	for i := 1; i < len(breaks); i++ {
		low, high := breaks[i-1], breaks[i]
		middle, slope, constant := (low+high)*.5, 0., 0.
		for axis := range origin {
			value := origin[axis] + direction[axis]*middle
			if math.Abs(value) > box.HalfSize[axis] {
				edge := math.Copysign(box.HalfSize[axis], value)
				slope += direction[axis] * direction[axis]
				constant += direction[axis] * (origin[axis] - edge)
			}
		}
		candidate := low
		if slope > 1e-20 {
			candidate = max(low, min(high, -constant/slope))
		}
		minimum = min(minimum, distance(candidate), distance(low), distance(high))
	}
	return minimum
}

// Overlaps 职责：用十五个分离轴检查两个有效定向盒；使用：宽相位命中后；边界：相切不算正体积交叠，碰撞引擎的接触容差须另行核对。
func (box Box) Overlaps(other Box) bool {
	delta := other.Center.Sub(box.Center)
	separated := func(axis Vector) bool {
		if axis.Dot(axis) < 1e-20 {
			return false
		}
		radius := 0.
		for i := 0; i < 3; i++ {
			radius += math.Abs(axis.Dot(box.Axes[i]))*box.HalfSize[i] + math.Abs(axis.Dot(other.Axes[i]))*other.HalfSize[i]
		}
		return math.Abs(delta.Dot(axis)) >= radius
	}
	for i := 0; i < 3; i++ {
		if separated(box.Axes[i]) || separated(other.Axes[i]) {
			return false
		}
		for j := 0; j < 3; j++ {
			if separated(box.Axes[i].Cross(other.Axes[j])) {
				return false
			}
		}
	}
	return true
}
