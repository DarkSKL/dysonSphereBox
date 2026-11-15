package build

// 文件职责：在原料轨继续串供之前保护末端参考分拣器的真实货物曲线。
// 使用：共享塔原料候选装箱前；边界：仅为仍需下游连接且取料点靠近带尾的轨道增加一个同向节点，不放宽采样误差或改动机器姿态。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
)

// extendDenseMaterialTail 职责：将串供转弯移到参考取料邻域以外；使用：已按真实output排序的未接线输入轨；边界：新增节点随原刚性单元落位，原物料身份、设备等级和完整几何终检均保留。
// 逻辑流：1. 判断带尾取料邻域 => 2. 确认延伸位置和单元归属 => 3. 增加同向节点 => 4. 同步实体集合、输入轨与货物轨引用。
func (b *builder) extendDenseMaterialTail(units []denseSpatialUnit, row *transportRow, input int, track []*placed) ([]*placed, error) {
	last, previous := track[len(track)-1], track[len(track)-2]
	needed := false
	for _, arm := range row.line.all {
		if !arm.densityCargo.enabled || arm.densityCargo.output {
			continue
		}
		member := false
		for _, node := range arm.densityCargo.track {
			member = member || node == last
		}
		if !member {
			continue
		}
		target, err := denseCargoLocalTarget(arm)
		if err != nil {
			return nil, err
		}
		distance := math.Sqrt(math.Pow((target.x-last.a.x)*planarGridLength, 2) + math.Pow((target.y-last.a.y)*planarGridLength, 2) + math.Pow((target.z-last.a.z)*1.3333333, 2))
		needed = needed || distance <= referenceCargoAnchorRadius
	}
	if !needed {
		return track, nil
	}
	point := vector{2*last.a.x - previous.a.x, 2*last.a.y - previous.a.y, 2*last.a.z - previous.a.z}
	if point.z < 0 {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "原料轨末端无法沿当前方向保留地面以上的串供采样邻域"}
	}
	unitIndex := -1
	for i, unit := range units {
		for _, node := range unit.entities {
			if node == last {
				unitIndex = i
				break
			}
		}
		if unitIndex >= 0 {
			break
		}
	}
	if unitIndex < 0 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "串供采样保护节点缺少实际落位单元"}
	}
	owner := b.owner
	b.owner = row.owner
	tail := b.make(int(last.record.ItemID), point.x, point.y, point.z)
	b.owner = owner
	tail.record.Yaw, tail.record.Yaw2 = last.record.Yaw, last.record.Yaw2
	connect(last, tail)
	units[unitIndex].entities = append(append([]*placed(nil), units[unitIndex].entities...), tail)
	for _, arm := range row.line.all {
		if !arm.densityCargo.enabled || arm.densityCargo.output {
			continue
		}
		for _, node := range arm.densityCargo.track {
			if node == last {
				arm.densityCargo.track = append(append([]*placed(nil), arm.densityCargo.track...), tail)
				break
			}
		}
	}
	row.line.all = append(append([]*placed(nil), row.line.all...), tail)
	track = append(append([]*placed(nil), track...), tail)
	row.line.inputs[input] = track
	return track, nil
}
