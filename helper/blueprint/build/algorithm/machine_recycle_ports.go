package build

// 文件职责：将回收优先门连接到真实槽位并保留离壳直线。
// 使用：机器内部毛流装配；边界：不改变优先参数，外端两格后才允许全场路由转弯。

import "math"

// buildRecyclePort 职责：建立槽点、一格过渡及两格外端的有向带链；使用：回收门各独立物料口；边界：槽位来自原型目录，最终仍检查碰撞。
func (b *builder) buildRecyclePort(gate *placed, slot int8, input bool) ([]*placed, error) {
	profile, err := loadSplitterPortProfile(gate.record.ModelIndex)
	if err != nil {
		return nil, err
	}
	port := profile.ports[slot]
	point := splitterGateRotate(port.point, gate.record.Yaw)
	forward := splitterGateRotate(port.forward, gate.record.Yaw)
	dx, dy := math.Round(forward[0]), math.Round(forward[2])
	points := []vector{{gate.a.x + point[0]/planarGridLength, gate.a.y + point[2]/planarGridLength, gate.a.z + point[1]/1.3333333}, {gate.a.x + dx, gate.a.y + dy, gate.a.z}, {gate.a.x + 2*dx, gate.a.y + 2*dy, gate.a.z}}
	if input {
		points[0], points[2] = points[2], points[0]
	}
	path := b.byPath(points)
	if input {
		path[2].output, path[2].record.OutputToSlot = gate, slot
	} else {
		path[0].input, path[0].record.InputFromSlot = gate, slot
	}
	return path, nil
}

// recycleInputAttached 职责：核对外端与真实槽口的身份和方向；使用：最终回流连通复核；边界：中间实体和有向边另由通用带链检查。
func recycleInputAttached(path []*placed, outer, gate *placed, slot int8) bool {
	return len(path) >= 3 && path[0] == outer && path[len(path)-1].output == gate && path[len(path)-1].record.OutputToSlot == slot
}
