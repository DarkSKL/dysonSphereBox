package build

// 文件职责：为完整折行短run保留各自真实输入/出货口的局部接线柱。
// 使用：凹形单元四向排布及之后的平移压缩；边界：仅保留单元实际拥有的端点，不把独立生产分流附件的端口绑回原机器run。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
)

// denseBentPortShapes 职责：按每个run实际累计朝向预留原端口；使用：复合L/U轮廓；边界：只占端点接线柱，凹角内部仍可由其它真实单元回填。
func (b *builder) denseBentPortShapes(ctx context.Context, unit denseSpatialUnit, turns int) ([]connectedPackingShape, error) {
	owned := map[*placed]bool{}
	for _, entity := range unit.entities {
		owned[entity] = true
	}
	shapes := []connectedPackingShape{}
	for _, row := range unit.rows {
		if err := b.checkGeneration("折行原生端口轮廓"); err != nil {
			return nil, err
		}
		ports := row.allInputEndpoints()
		for _, route := range row.outputs {
			if route.to != nil && owned[route.start] {
				ports = append(ports, route.start)
			}
		}
		for _, output := range row.line.outputRows() {
			if len(output) > 0 && owned[output[0]] && output[0].output == nil {
				ports = append(ports, output[0])
			}
		}
		for _, recycle := range row.recycles {
			ports = append(ports, recycle.route.start, recycle.route.end)
		}
		offset := denseRotateVector(vector{x: -1}, (row.quarterTurns+turns)%4)
		seen := map[*placed]bool{}
		for _, port := range ports {
			if !owned[port] || seen[port] {
				continue
			}
			seen[port] = true
			point := denseRouteWorld(vector{port.a.x + offset.x, port.a.y + offset.y, 0})
			box := placement.Box{Center: point.Add(placement.Vector{0, transportMaxHeight * 1.3333333 / 2, 0}), HalfSize: placement.Vector{.31, transportMaxHeight*1.3333333/2 + .3, .31}, Axes: [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
			shape := connectedBoxShape("port", box)
			shape.gridPort = true
			shapes = append(shapes, shape)
		}
	}
	return shapes, nil
}
