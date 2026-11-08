package build

// 文件职责：让共享外供消费轨沿用唯一真实边界与监测身份。
// 使用：完整物流提交后的入口监测；边界：规范份额仍逐条保留，只有相同已证明物理供给可以去重。

// uniqueMaterialBoundaryRoutes 职责：将共享入口的规范份额合成原物理入口视图；使用：避免为同一带头重复监测；边界：不按位置或物品猜测身份。
func (b *builder) uniqueMaterialBoundaryRoutes(routes []*transportRoute) []*transportRoute {
	plan := b.denseMaterialNetworks
	if plan == nil || !plan.committed {
		return routes
	}
	result := []*transportRoute{}
	seen := map[*transportRoute]bool{}
	for _, route := range routes {
		if binding, ok := plan.bindings[route]; ok && route.from == nil {
			if seen[binding.source] {
				continue
			}
			seen[binding.source] = true
			view := *binding.source
			view.path = append([]*placed(nil), binding.source.path...)
			result = append(result, &view)
		} else {
			result = append(result, route)
		}
	}
	return result
}
