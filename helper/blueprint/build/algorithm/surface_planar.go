package build

// 文件职责：为兼容平面布局提供与球面检查共用的切平面坐标适配。
// 使用：最终规则沙箱为未指定 SurfaceLayout 的请求运行同一组占用规则。
// 边界：恒定格长不模拟纬圈收缩；结果必须披露为切平面预检，不能当作球面或游戏实测。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// planarGridLength 职责：保存原版赤道表面细格长度；使用：平面布局的米制适配；边界：不用于非原版行星或纬圈收缩。
const planarGridLength = 200.2 * 2 * math.Pi / 1000

// newPlanarRuleEncoding 职责：创建只供规则检查的平面适配器；使用：最终记录出口；边界：不改写蓝图 Area 或创建推荐纬圈。
func newPlanarRuleEncoding() *surfaceEncoding {
	return &surfaceEncoding{planar: true, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0}}
}

// colliderBox 职责：按坐标空间变换原型盒；使用：静态和带建筑占用；边界：平面采用固定切线基，球面保留实际径向基。
func (encoding *surfaceEncoding) colliderBox(origin vector, yaw float32, collider placement.Collider) (placement.Box, error) {
	if !encoding.planar {
		return surfaceColliderBox(origin, yaw, collider)
	}
	box, err := surfaceColliderBox(vector{0, 0, -200.2}, yaw, collider)
	if err != nil {
		return placement.Box{}, err
	}
	// 赤道参考基为 x 向东、y 向北、-z 向上；置换到平面 x 向东、z 向北、y 向上。
	convert := func(v placement.Vector) placement.Vector { return placement.Vector{v[0], -v[2], v[1]} }
	box.Center = convert(box.Center.Sub(placement.Vector{0, 0, -200.2})).Add(placement.Vector{origin.x, origin.y, origin.z})
	for axis := range box.Axes {
		box.Axes[axis] = convert(box.Axes[axis])
	}
	return box, nil
}

// recordPose 职责：按所选空间创建最终端口姿态；使用：分拣器和附加通道；边界：保留相同 Euler 旋转顺序。
func (encoding *surfaceEncoding) recordPose(position vector, yaw, pitch, tilt float32) (surfacePose, error) {
	box, err := encoding.colliderBox(position, yaw, placement.Collider{Extents: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}})
	if err != nil {
		return surfacePose{}, err
	}
	return surfacePoseFromBox(box, pitch, tilt), nil
}

// localUp 职责：返回坐标空间的局部竖直方向；使用：带球和胶囊；边界：平面不能用位置向量冒充法线。
func (encoding *surfaceEncoding) localUp(point placement.Vector) placement.Vector {
	if encoding.planar {
		return placement.Vector{0, 1, 0}
	}
	return surfaceUnit(point)
}

// localGridSize 职责：选择端口修正量的米格换算；使用：动态分拣器；边界：平面只使用原版赤道切线格长。
func (encoding *surfaceEncoding) localGridSize(position, direction placement.Vector) float64 {
	if encoding.planar {
		return planarGridLength
	}
	return surfaceLocalGridSize(position, direction)
}

// segmentsAcross 职责：计算对应空间的水平跨度；使用：分拣器硬边界；边界：切平面不引入虚假的球面径向变化。
func (encoding *surfaceEncoding) segmentsAcross(reference, first, second placement.Vector) float64 {
	if encoding.planar {
		return math.Hypot(first[0]-second[0], first[2]-second[2]) / planarGridLength
	}
	return surfaceSegmentsAcross(reference, first, second)
}

// endpointHeightGap 职责：计算分拣器两端竖直差；使用：最小跨度；边界：保留游戏使用的 .2 米单位。
func (encoding *surfaceEncoding) endpointHeightGap(first, second placement.Vector) float64 {
	if encoding.planar {
		return math.Abs(first[1]-second[1]) / .2
	}
	return math.Abs(math.Sqrt(first.Dot(first))-math.Sqrt(second.Dot(second))) / .2
}

// inserterBuried 职责：检查分拣器两端均埋入基准面的情形；使用：修正后姿态；边界：不模拟真实地形凹凸。
func (encoding *surfaceEncoding) inserterBuried(poses [2]surfacePose) bool {
	if encoding.planar {
		return poses[0].position[1]+poses[0].axes[1][1] < -.1 && poses[1].position[1]+poses[1].axes[1][1] < -.1
	}
	return surfaceInserterBuried(poses)
}
