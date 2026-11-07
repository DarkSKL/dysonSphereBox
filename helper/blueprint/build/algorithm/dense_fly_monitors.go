package build

// 文件职责：给黑盒边界设施的每个独立物料份额提供真实带侧监测视图。
// 使用：共享箱完成真实分流和全场路线之后；边界：不更改规范lane的配送器边界，原逐节点连通、物料、合流前宿主和净空检查继续执行。

// directFacilityBoundaryMonitorRoutes 职责：以设施外的真实出料头作为该份额的监测起点；使用：最终布设与终检采用同一视图；边界：塔端口需为path首带，配送分流从全部并联臂汇入后的route.start起监测，不能把部分臂流量冒充整路覆盖。
func (b *builder) directFacilityBoundaryMonitorRoutes(routes []*transportRoute) []*transportRoute {
	if b.directBoundaryUnits == nil {
		return routes
	}
	views := make([]*transportRoute, 0, len(routes))
	prefixes := map[*placed][]*placed{}
	starts := map[*placed]*placed{}
	if b.denseFlyBlackbox != nil {
		for _, facility := range b.denseFlyBlackbox.facilities {
			if !facility.endpoint.input || !b.setting.EnableFlowMonitor {
				continue
			}
			if len(facility.routes) == 1 && len(facility.endpoint.belts) >= 2 {
				prefixes[facility.routes[0].start] = []*placed{facility.endpoint.belts[1], facility.endpoint.belts[0]}
			}
			for route, head := range facility.monitorHeads {
				starts[route.start] = head
			}
		}
	}
	for _, route := range routes {
		if route == nil || route.from != nil || route.edge.FlowKind != "external-input" || len(route.path) == 0 || route.boundary == nil {
			views = append(views, route)
			continue
		}
		item := route.boundary.record.ItemID
		station := item == 2103 || item == 2104
		delivery := item == 2107 && b.denseFlyBlackbox != nil
		if !station && !delivery || station && route.path[0] != route.start {
			views = append(views, route)
			continue
		}
		desired := route.start
		if starts[route.start] != nil {
			desired = starts[route.start]
		}
		start := -1
		for i, node := range route.path {
			if node == desired {
				start = i
				break
			}
		}
		if start < 0 {
			views = append(views, route)
			continue
		}
		view := *route
		view.path = route.path[start:]
		if prefix := prefixes[route.start]; len(prefix) > 0 {
			view.path = append(append([]*placed{}, prefix...), view.path...)
		}
		view.start, view.boundary = view.path[0], view.path[0]
		views = append(views, &view)
	}
	return views
}
