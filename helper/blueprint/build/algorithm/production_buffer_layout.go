package build

// 文件职责：固定普通固体旁挂缓存，让超密度缓存及回流旁路独立填入空隙，并登记共享主带。
// 使用：完整物流装配；边界：独立落位仍须真实接线，不改变库存旁路语义。

import "DysonSphereBox/helper/blueprint"

// compactBufferLayout 职责：选择超密度缓存的紧凑模块；使用：模块与独立落位共同判断；边界：不改变物料流。
func (b *builder) compactBufferLayout() bool {
	return b.adapter != nil && b.adapter.density == densityDense
}

// detachProductionBuffers 职责：分离无法紧贴机器的流体或回流旁路；使用：空间候选；边界：普通箱体保持同产线短带相连。
func (b *builder) detachProductionBuffers(row *transportRow, block []*placed) ([]*placed, []denseSpatialUnit) {
	members := map[*placed]bool{}
	units := []denseSpatialUnit{}
	for _, item := range row.recipe.Results {
		buffer := row.buffers[item]
		if buffer == nil || !buffer.detached {
			continue
		}
		ports := []denseStreetPort{{item: item, input: true, head: buffer.intake[0], inner: buffer.intake[1]}}
		for _, route := range row.outputs {
			if route.edge.ItemID != item {
				continue
			}
			inner := buffer.outlet[len(buffer.outlet)-2]
			if len(route.path) > 1 {
				inner = route.path[len(route.path)-2]
			}
			ports = append(ports, denseStreetPort{item: item, head: route.start, inner: inner})
		}
		for _, entity := range buffer.entities {
			members[entity] = true
		}
		units = append(units, denseSpatialUnit{entities: buffer.entities, ports: ports, buffer: buffer})
	}
	kept := make([]*placed, 0, len(block))
	for _, entity := range block {
		if !members[entity] {
			kept = append(kept, entity)
		}
	}
	return kept, units
}

// productionBufferRoutes 职责：仅为独立旁路提供辅助供给线；使用：全场路由与距离评分；边界：固定箱体短带不重复寻路。
func (b *builder) productionBufferRoutes() []*transportRoute {
	routes := []*transportRoute{}
	for _, buffer := range b.productionBuffers {
		if buffer.detached {
			routes = append(routes, buffer.feed)
		}
	}
	return routes
}

// commitProductionBuffers 职责：将实际缓存供给链合入原规范份额的共享容量证据；使用：全部路由完成后；边界：每条实体带每份需求只登记一次，任何断链都拒绝。
func (b *builder) commitProductionBuffers() error {
	for _, buffer := range b.productionBuffers {
		path := buffer.feed.path
		if len(path) < 2 || path[0] != buffer.source || path[len(path)-1] != buffer.intake[0] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "缓存供给未连接实际机器与库存"}
		}
		for i := 1; i < len(path); i++ {
			if path[i-1].output != path[i] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "缓存供给带断开"}
			}
		}
		for _, route := range buffer.row.outputs {
			if route.edge.ItemID != buffer.item {
				continue
			}
			seen := map[*placed]bool{}
			for _, entity := range route.sharedPath {
				seen[entity] = true
			}
			for _, entity := range route.path {
				seen[entity] = true
			}
			for _, entity := range path {
				if !seen[entity] {
					route.sharedPath = append(route.sharedPath, entity)
					seen[entity] = true
				}
			}
		}
	}
	return nil
}
