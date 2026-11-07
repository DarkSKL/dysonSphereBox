package build

// 文件职责：把已真实接回的街道附件前缀提供给原料监测器选点。
// 使用：mode1全场接线后；边界：只复制路线视图，原合流、物品、宿主净空核验与规范净流端点不变。

// denseBoundaryMonitorRoutes 职责：延伸到同一物料附件的真实地面前缀；使用：复用已有流速监测器；边界：只对已提交附件生效，路径的实际连接仍由公共检查逐节点核验。
func (b *builder) denseBoundaryMonitorRoutes() []*transportRoute {
	if len(b.denseConnectedStreets) == 0 {
		return b.directFacilityBoundaryMonitorRoutes(b.denseStationSprayMonitorViews(b.transportRoutes))
	}
	type materialTail struct {
		item  int
		nodes []*placed
	}
	tails := map[*placed]materialTail{}
	for _, work := range b.denseConnectedStreets {
		if !work.assembly.committed {
			continue
		}
		for _, module := range work.assembly.accessories {
			if module.input {
				tails[module.head] = materialTail{module.item, module.material}
			}
		}
		for _, port := range work.assembly.sprayPorts {
			path := []*placed{}
			seen := map[*placed]bool{}
			for node := port.head; node != nil && surfaceIsBelt(node.record.ItemID) && !seen[node] && len(path) < 3; node = node.output {
				seen[node] = true
				path = append(path, node)
			}
			tails[port.head] = materialTail{port.item, path}
		}
	}
	routes := make([]*transportRoute, 0, len(b.transportRoutes))
	for _, route := range b.transportRoutes {
		declared := tails[route.end]
		tail := declared.nodes
		if route.from != nil || route.edge.FlowKind != "external-input" || declared.item != route.edge.ItemID || len(tail) < 2 || len(route.path) == 0 || route.path[len(route.path)-1] != tail[0] {
			routes = append(routes, route)
			continue
		}
		view := *route
		view.path = append(append([]*placed{}, route.path...), tail[1:]...)
		routes = append(routes, &view)
	}
	return b.directFacilityBoundaryMonitorRoutes(b.denseStationSprayMonitorViews(routes))
}
