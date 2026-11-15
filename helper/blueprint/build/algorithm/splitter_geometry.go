package build

// 文件职责：把三形态分流器的真实局部米制几何接入统一占位与短接头复核。
// 使用：排布、寻路及最终记录净空；边界：保守选择体积独立于原生建造盒，不改变任何原料或产物树拓扑。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// splitterPortGeometryTolerance 职责：容纳半径200米球面上原生短接头的切平面弦高和float32误差；使用：槽口独立核对；边界：1.35米处弦高小于0.005米，不允许用整格误差猜测槽位。
const splitterPortGeometryTolerance = .01

// splitterPortPose 职责：向共同构造器提供原生槽点、外向方向与独立过渡点；使用：模型驱动接头；边界：point才是原生槽点，transition不是强制首带节点。
type splitterPortPose struct {
	point, forward, transition placement.Vector
	// extensionMeters 职责：保存原生从槽口到过渡点的长度；使用：共同构造器；边界：不是建筑中心到首带的最小距离。
	extensionMeters float64
}

// splitterPortProfile 职责：适配共同分流器构造器的几何查询；使用：原料和产物树共享；边界：clearanceMeters是水平槽方向的保守终点距离，不是原生碰撞半径。
type splitterPortProfile struct {
	model           int16
	ports           [4]splitterPortPose
	clearanceMeters float64
}

// loadSplitterPortProfile 职责：从唯一原型目录适配三模型构造合同；使用：共同接头装配；边界：保持米制，槽点和过渡点分别给出，净空由选择半宽与带球半径推导。
func loadSplitterPortProfile(model int16) (splitterPortProfile, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return splitterPortProfile{}, err
	}
	profile, err := catalog.SplitterProfile(2020, model)
	if err != nil {
		return splitterPortProfile{}, err
	}
	result := splitterPortProfile{model: model, clearanceMeters: math.Max(float64(profile.Selection.Extents[0]), float64(profile.Selection.Extents[2])) + surfaceBeltNodeRadius + splitterPortGeometryTolerance}
	for slot, port := range profile.Ports {
		result.ports[slot] = splitterPortPose{point: port.Position, forward: port.Forward, transition: port.Position.Add(port.Forward.Scale(placement.SplitterTransitionLength)), extensionMeters: placement.SplitterTransitionLength}
	}
	return result, nil
}

// splitterClearanceShapes 职责：在原生细分盒之外追加明确的分流器保守选择体积；使用：排布与寻路索引；边界：其它建筑仍使用原生建造盒，最终原生检查不被替换。
func splitterClearanceShapes(catalog *placement.Catalog, item, model int16) ([]placement.Collider, error) {
	shapes, err := catalog.Shapes(item, model)
	if err != nil || item != 2020 {
		return shapes, err
	}
	profile, err := catalog.SplitterProfile(item, model)
	if err != nil {
		return nil, err
	}
	return append(shapes, profile.Selection), nil
}

// splitterWorldPort 职责：应用真实模型、建筑旋转及球面姿态取得槽位；使用：短接头独立复核；边界：返回米制实体原点，不附加带节点碰撞球的抬高。
func (encoding *surfaceEncoding) splitterWorldPort(record blueprint.Building, position vector, slot int8) (placement.Vector, placement.Vector, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return placement.Vector{}, placement.Vector{}, err
	}
	profile, err := catalog.SplitterProfile(record.ItemID, record.ModelIndex)
	if err != nil {
		return placement.Vector{}, placement.Vector{}, err
	}
	port, err := profile.Port(int(slot))
	if err != nil {
		return placement.Vector{}, placement.Vector{}, err
	}
	pose, err := encoding.recordPose(position, record.Yaw, 0, 0)
	if err != nil {
		return placement.Vector{}, placement.Vector{}, err
	}
	forward := pose.transform(port.Forward)
	forward = forward.Scale(1 / math.Sqrt(forward.Dot(forward)))
	return pose.position.Add(pose.transform(port.Position)), forward, nil
}

// splitterPointOnPortRay 职责：验证实体位于已核对槽口的向外直线上；使用：实际短接头及离开本体的同向带段；边界：short时仅允许原生1.1米过渡范围，不能容纳任意三跳绕行。
func splitterPointOnPortRay(point, port, forward placement.Vector, short bool) bool {
	delta := point.Sub(port)
	along := delta.Dot(forward)
	if along < -splitterPortGeometryTolerance || short && along > placement.SplitterTransitionLength+splitterPortGeometryTolerance {
		return false
	}
	perpendicular := delta.Sub(forward.Scale(along))
	return perpendicular.Dot(perpendicular) <= splitterPortGeometryTolerance*splitterPortGeometryTolerance
}

// splitterShortConnectionAllowed 职责：只为正确槽位上的真实短接头及其相邻同向过渡带保留占用例外；使用：原生带建筑检查及保守选择体积；边界：最多追溯两条真实边，首端和带段都必须按模型位置、方向、高度复核。
func (encoding *surfaceEncoding) splitterShortConnectionAllowed(belt, gate, end int, records []blueprint.Building, positions []vector, byIndex map[int32]int) bool {
	if belt < 0 || gate < 0 || belt >= len(records) || gate >= len(records) || records[gate].ItemID != 2020 {
		return false
	}
	world := func(index int) placement.Vector {
		point := positions[index]
		return placement.Vector{point.x, point.y, point.z}
	}
	anchors := []int{belt}
	if end >= 0 && end != belt {
		anchors = append(anchors, end)
	}
	for _, anchor := range anchors {
		for _, output := range []bool{false, true} {
			current := anchor
			for hops := 1; hops <= 2; hops++ {
				reference, slot := records[current].InputObjIdx, records[current].InputFromSlot
				if output {
					reference, slot = records[current].OutputObjIdx, records[current].OutputToSlot
				}
				next, exists := byIndex[reference]
				if !exists {
					break
				}
				if next == gate {
					port, forward, err := encoding.splitterWorldPort(records[gate], positions[gate], slot)
					if err != nil {
						break
					}
					probes := []splitterPortProbe{{world(current), true}, {world(anchor), true}, {world(belt), false}}
					if end >= 0 && end != belt {
						probes = append(probes, splitterPortProbe{world(end), false})
					}
					if splitterPortProbesAllowed(world(gate), port, forward, probes, !encoding.planar) {
						return true
					}
					break
				}
				if !surfaceIsBelt(records[next].ItemID) {
					break
				}
				current = next
			}
		}
	}
	return false
}
