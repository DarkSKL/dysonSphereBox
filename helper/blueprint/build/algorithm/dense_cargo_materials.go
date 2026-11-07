package build

// 文件职责：用真实物料标签与带连通关系补齐带间、箱体带侧的固定窗口身份。
// 使用：范围验证的只读scope；边界：不改变Nearest白名单，不把仅空间接近或未知物料的带当成同一物料。

import (
	"DysonSphereBox/helper/blueprint"
	"fmt"
)

// denseCargoMaterialPath 职责：保存物理带连通分量及已声明物料；使用：只读窗口归属；边界：跨分拣器不合并分量，混料取货必须有实际过滤器。
type denseCargoMaterialPath struct {
	nodes []*placed
	items map[int]bool
}

// completeCargoWindowScopes 职责：覆盖全部带侧端点，带→带臂分别验证取货与放货；使用：机器scope完成之后；边界：未知来源、未过滤混料或与配方/过滤物品不符均拒绝。
func (b *builder) completeCargoWindowScopes(scopes []denseMachineCargoScope) ([]denseMachineCargoScope, error) {
	parents, order := map[*placed]*placed{}, map[*placed]int{}
	for i, node := range b.all {
		if surfaceIsBelt(node.record.ItemID) {
			parents[node] = node
			order[node] = i
		}
	}
	var root func(*placed) *placed
	root = func(node *placed) *placed {
		parent, ok := parents[node]
		if !ok {
			return nil
		}
		if parent != node {
			parents[node] = root(parent)
		}
		return parents[node]
	}
	for _, node := range b.all {
		a, c := root(node), root(node.output)
		if a != nil && c != nil && a != c {
			if order[a] > order[c] {
				a, c = c, a
			}
			parents[c] = a
		}
	}
	groups, paths := map[*placed]*denseCargoMaterialPath{}, map[*placed]*denseCargoMaterialPath{}
	for _, node := range b.all {
		if key := root(node); key != nil {
			group := groups[key]
			if group == nil {
				group = &denseCargoMaterialPath{items: map[int]bool{}}
				groups[key] = group
			}
			group.nodes = append(group.nodes, node)
			paths[node] = group
		}
	}
	mark := func(node *placed, item int) {
		if group := paths[node]; group != nil && item > 0 {
			group.items[item] = true
		}
	}
	for _, scope := range scopes {
		for _, node := range scope.track {
			mark(node, scope.item)
		}
	}
	for _, route := range append(append([]*transportRoute{}, b.transportRoutes...), b.denseStreetRoutes...) {
		if route == nil {
			continue
		}
		mark(route.start, route.edge.ItemID)
		mark(route.end, route.edge.ItemID)
		for _, node := range route.path {
			mark(node, route.edge.ItemID)
		}
		for _, node := range route.sharedPath {
			mark(node, route.edge.ItemID)
		}
	}
	for _, endpoint := range b.flyEndpoints {
		if endpoint == nil {
			continue
		}
		mark(endpoint.link, endpoint.item)
		for _, node := range endpoint.belts {
			mark(node, endpoint.item)
		}
	}
	for _, link := range b.stationLinks {
		mark(link.head, link.item)
	}
	for _, line := range b.machineLines {
		if line != nil {
			mark(line.coaterInput, productionSprayItem(line.info))
		}
	}
	type endpointKey struct {
		arm    *placed
		output bool
	}
	known := map[endpointKey]int{}
	for i, scope := range scopes {
		known[endpointKey{scope.arm, scope.output}] = i
	}
	fail := func(arm *placed, reason string) error {
		return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("分拣器%d的固定取放窗口身份未证实：%s", arm.record.Index, reason)}
	}
	for _, arm := range b.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
			continue
		}
		for side, node := range []*placed{arm.input, arm.output} {
			if node == nil || !surfaceIsBelt(node.record.ItemID) {
				continue
			}
			key := endpointKey{arm, side == 1}
			group := paths[node]
			if group == nil {
				return nil, fail(arm, "带端点已不在最终实体中")
			}
			if i, exists := known[key]; exists {
				item := scopes[i].item
				// 独立参考夹具没有完整生产元数据，保留明确提供的原track身份，不猜物品。
				if item == 0 {
					continue
				}
				if !group.items[item] || side == 0 && len(group.items) > 1 && int(arm.record.FilterID) != item {
					return nil, fail(arm, "机器物料与实际带分量/过滤器不符")
				}
				scopes[i].track = group.nodes
				continue
			}
			item := int(arm.record.FilterID)
			if item == 0 && len(group.items) == 1 {
				for candidate := range group.items {
					item = candidate
				}
			}
			if item == 0 || !group.items[item] {
				return nil, fail(arm, "带间或箱体端点没有唯一物料/实际过滤器")
			}
			if side == 0 && len(group.items) > 1 && int(arm.record.FilterID) != item {
				return nil, fail(arm, "未过滤的混料取货")
			}
			known[key] = len(scopes)
			scopes = append(scopes, denseMachineCargoScope{arm: arm, output: side == 1, track: group.nodes, item: item})
		}
		if paths[arm.input] != nil && paths[arm.output] != nil {
			input, hasInput := known[endpointKey{arm, false}]
			output, hasOutput := known[endpointKey{arm, true}]
			if !hasInput || !hasOutput || scopes[input].item <= 0 || scopes[input].item != scopes[output].item {
				return nil, fail(arm, "带间取货与放货物料不一致")
			}
		}
	}
	return scopes, nil
}
