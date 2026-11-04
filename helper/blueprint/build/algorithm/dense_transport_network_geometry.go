package build

// 文件职责：从真实output拓扑提取消费轨末端，创建串供接线并在全场占位时保护接驳口。
// 使用：所有机器行装配后；边界：先按原规则裁尾再取末端，不保留无用长尾，也不把slice顺序或同坐标当成连线。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseOrderedMaterialTrack 职责：将独立物料轨按实际output排序；使用：获得进出端与方向；边界：断链、分叉、重复或环路一律拒绝。
func denseOrderedMaterialTrack(nodes []*placed) ([]*placed, error) {
	fail := func() ([]*placed, error) {
		return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享消费轨缺少唯一完整的实际有向输入带"}
	}
	members, incoming := map[*placed]bool{}, map[*placed]int{}
	for _, node := range nodes {
		if node == nil || !surfaceIsBelt(node.record.ItemID) || members[node] {
			return fail()
		}
		members[node] = true
	}
	for _, node := range nodes {
		if members[node.output] {
			incoming[node.output]++
		}
	}
	var entry *placed
	for _, node := range nodes {
		if incoming[node] > 1 {
			return fail()
		}
		if incoming[node] == 0 {
			if entry != nil {
				return fail()
			}
			entry = node
		}
	}
	ordered, seen := []*placed{}, map[*placed]bool{}
	for node := entry; members[node] && !seen[node]; node = node.output {
		ordered = append(ordered, node)
		seen[node] = true
		if members[node.output] && node.record.OutputToSlot != 1 {
			return fail()
		}
	}
	if len(ordered) != len(nodes) || len(ordered) < 2 {
		return fail()
	}
	return ordered, nil
}

// denseMaterialInputIndex 职责：按配方物品找唯一原料轨；使用：跨配方共享；边界：未知或重复物料不猜轨位。
func denseMaterialInputIndex(row *transportRow, item int) int {
	index := -1
	for i, current := range row.recipe.Items {
		if current == item {
			if index >= 0 {
				return -1
			}
			index = i
		}
	}
	return index
}

// prepareDenseMaterialNetworkGeometry 职责：把共享消费轨的尾→头关系交给同一真实路由器；使用：机器及边界设施落位前；边界：不直接跨空隙connect，不改变原物料track成员。
func (b *builder) prepareDenseMaterialNetworkGeometry(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	plan := b.denseMaterialNetworks
	if plan == nil {
		return units, nil
	}
	if err := b.trimConnectedTails(); err != nil {
		return nil, err
	}
	for _, group := range plan.groups {
		for _, row := range group.rows {
			index := denseMaterialInputIndex(row, group.item)
			if row.line == nil || index < 0 || index >= len(row.line.inputs) || row.inputEndpoint(group.item, recipeInputRole) == nil {
				return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共享消费轨没有实际输入口"}
			}
			track, err := denseOrderedMaterialTrack(row.line.inputs[index])
			if err != nil {
				return nil, err
			}
			if track[len(track)-1].output != nil {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享消费轨末端已被其它物理线路占用"}
			}
			group.entries = append(group.entries, row.inputEndpoint(group.item, recipeInputRole))
			group.exits = append(group.exits, track[len(track)-1])
			group.predecessors = append(group.predecessors, track[len(track)-2])
		}
		if group.tapped {
			var err error
			units, err = b.prepareDenseMaterialTaps(group, units)
			if err != nil {
				return nil, err
			}
			continue
		}
		group.supplyHead = group.entries[0]
		sources := []*transportRoute{}
		for _, source := range plan.physical {
			members := plan.members[source]
			if len(members) > 0 && plan.bindings[members[0]].group == group {
				sources = append(sources, source)
			}
		}
		if len(sources) > 1 {
			owner := b.owner
			b.owner = -1
			group.intake = b.byPath([]vector{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}})
			b.owner = owner
			group.supplyHead = group.intake[0]
			proxy := &transportRow{owner: -1, inputs: map[int]*placed{group.item: group.supplyHead}}
			rate := 0.
			for _, part := range group.planned {
				rate += part
			}
			group.intakeRoute = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("material-intake:%d", group.id), ItemID: group.item, FlowKind: "internal"}, rate: rate, from: proxy, to: group.rows[0], start: group.intake[3], end: group.entries[0], path: []*placed{group.intake[3]}}
			plan.joins = append(plan.joins, group.intakeRoute)
			for _, source := range sources {
				source.to = proxy
			}
			units = append(units, denseSpatialUnit{entities: group.intake, ports: []denseStreetPort{{item: group.item, input: true, head: group.intake[0], inner: group.intake[1]}, {item: group.item, head: group.intake[3], inner: group.intake[2]}}})
		}
		for i := 0; i+1 < len(group.rows); i++ {
			rate := 0.
			for _, part := range group.planned[i+1:] {
				rate += part
			}
			join := &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("material-chain:%d:%d", group.id, i), ItemID: group.item, FlowKind: "internal"}, rate: rate, from: group.rows[i], to: group.rows[i+1], start: group.exits[i], end: group.entries[i+1], path: []*placed{group.exits[i]}}
			group.joins = append(group.joins, join)
			plan.joins = append(plan.joins, join)
			found := false
			for unitIndex := range units {
				for _, entity := range units[unitIndex].entities {
					if entity == join.start {
						units[unitIndex].ports = append(units[unitIndex].ports, denseStreetPort{item: group.item, head: join.start, inner: group.predecessors[i]})
						found = true
						break
					}
				}
			}
			if !found {
				return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "串供出口没有进入实际全场占位集合"}
			}
		}
	}
	for source := range plan.members {
		binding := plan.bindings[plan.members[source][0]]
		source.end = binding.group.supplyHead
	}
	for row := range plan.physicalOutputs {
		plan.physicalOutputs[row] = append([]*transportRoute{}, row.outputs...)
	}
	return units, b.checkGeneration("共享消费轨端口准备")
}

// preserveDenseMaterialNetworkTracks 职责：保留真实串供出口、接线与已展开输入轨；使用：所有后续裁尾；边界：只保留构造前已正常裁短的必要轨道，不保护原无用长尾。
func (b *builder) preserveDenseMaterialNetworkTracks(required map[*placed]bool) {
	if b.denseMaterialNetworks == nil {
		return
	}
	for _, group := range b.denseMaterialNetworks.groups {
		for _, node := range group.intake {
			required[node] = true
		}
		for _, node := range append(append([]*placed{}, group.entries...), group.exits...) {
			required[node] = true
		}
		for _, track := range group.tracks {
			for _, node := range track {
				required[node] = true
			}
		}
	}
	for _, route := range b.denseMaterialNetworkRoutes() {
		required[route.start], required[route.end] = true, true
		for _, node := range route.path {
			required[node] = true
		}
	}
}
