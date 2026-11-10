package placement

// 文件职责：提供当前原版三种四向分流器的真实槽位及不同用途的体积。
// 使用：构造器读取米制槽位，排布与寻路读取明确的保守选择体积。
// 边界：选择体积不是原生碰撞盒，不在此决定支路拓扑或把米直接当蓝图格。

import (
	"fmt"
	"math"
)

// SplitterTransitionLength 职责：保存原生铺带从槽口向外计算过渡点的米制长度；使用：模型驱动短接头；边界：来自当前BuildTool_Path，不表示首节点必须距离建筑中心这么远。
const SplitterTransitionLength = 1.1

// SplitterPort 职责：保存一个原生分流器槽位的位置与外向单位方向；使用：接头装配及独立复核；边界：坐标为模型局部米制，尚未应用建筑旋转和球面变换。
type SplitterPort struct {
	// Position 职责：保存槽位局部米制位置；使用：生成原生接头；边界：不同形态的槽位高度不同。
	Position Vector
	// Forward 职责：保存槽位外向单位方向；使用：向外短接头；边界：不是物料输送方向。
	Forward Vector
}

// SplitterProfile 职责：区分三形态的原生槽位、整体建造包络及保守选择净空；使用：分流器生成与验证；边界：不改变原生细分建造盒，禁止只替换模型而沿用其它形态槽位。
type SplitterProfile struct {
	// ModelIndex 职责：标识已经核对的形态；使用：构造记录；边界：仅38、39、40。
	ModelIndex int16
	// Ports 职责：按原生slot顺序保存四个真实接头；使用：连接及旋转；边界：不保证四口都在地面。
	Ports [4]SplitterPort
	// BuildEnvelope 职责：保存原生整体建造包络；使用：解释模型范围；边界：原生窄相位仍用Catalog.Shapes的细分盒。
	BuildEnvelope Collider
	// Selection 职责：保存选择体积对应的定向盒；使用：明确的保守排布和寻路净空；边界：不是原生Physics建造判定。
	Selection Collider
}

// Port 职责：按真实槽编号读取分流器接头；使用：构造与验证；边界：越界返回错误，不按最近方向猜测槽位。
func (profile SplitterProfile) Port(slot int) (SplitterPort, error) {
	if slot < 0 || slot >= len(profile.Ports) {
		return SplitterPort{}, fmt.Errorf("分流器模型%d没有槽位%d", profile.ModelIndex, slot)
	}
	return profile.Ports[slot], nil
}

// SplitterProfile 职责：查询当前锁定原型的三形态分流器几何；使用：物品2020构造、净空及接头复核；边界：缺少真实槽位或选择体积时失败，不回退成一格盒。
func (catalog *Catalog) SplitterProfile(item, model int16) (SplitterProfile, error) {
	profile := SplitterProfile{ModelIndex: model}
	if item != 2020 || model < 38 || model > 40 {
		return profile, fmt.Errorf("物品%d/模型%d不是已核对的三形态分流器", item, model)
	}
	if _, err := catalog.Shapes(item, model); err != nil {
		return profile, err
	}
	geometry, err := catalog.Geometry(item, model)
	if err != nil || len(geometry.PortPoses) != 4 {
		return profile, fmt.Errorf("分流器模型%d缺少四个真实槽位", model)
	}
	profile.BuildEnvelope = catalog.models[int(model)].BuildEnvelope
	profile.Selection = Collider{Position: geometry.SelectCenter, Rotation: [4]float32{0, 0, 0, 1}}
	for axis, size := range geometry.SelectSize {
		if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
			return profile, fmt.Errorf("分流器模型%d缺少真实选择体积", model)
		}
		profile.Selection.Extents[axis] = size / 2
	}
	for slot, pose := range geometry.PortPoses {
		q := Vector{float64(pose.Rotation[0]), float64(pose.Rotation[1]), float64(pose.Rotation[2])}
		w := float64(pose.Rotation[3])
		if math.Abs(q.Dot(q)+w*w-1) > 1e-4 {
			return profile, fmt.Errorf("分流器模型%d槽位%d旋转无效", model, slot)
		}
		forward := Vector{0, 0, 1}
		cross := q.Cross(forward)
		forward = forward.Add(cross.Scale(2 * w)).Add(q.Cross(cross).Scale(2))
		profile.Ports[slot] = SplitterPort{Position: Vector{float64(pose.Position[0]), float64(pose.Position[1]), float64(pose.Position[2])}, Forward: forward}
	}
	return profile, nil
}
