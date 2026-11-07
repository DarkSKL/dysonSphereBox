package build

// 文件职责：把同物料的多个消费者组织为真实单向串供轨，保留规范净流与物理供给的分别登记。
// 使用：独立密铺内部物流候选；边界：无喷涂原料串轨、喷涂原料保留原喷涂前入口并用真实过滤取料，禁止普通带T分支及替代回流，整组配置毛需求不得超过一条带。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// denseMaterialNetworksKey 职责：显式启用独立共享消费轨候选；使用：候选context；边界：不修改普通模式或未启用的密铺拓扑。
type denseMaterialNetworksKey struct{}

// denseMaterialNetworkPlan 职责：保留规范份额、聚合后的真实供给及串供连接；使用：装配和终检交接；边界：物理聚合路线不得代替最终规范边账。
type denseMaterialNetworkPlan struct {
	canonical, physical []*transportRoute
	groups              []*denseMaterialNetwork
	bindings            map[*transportRoute]denseMaterialNetworkBinding
	members             map[*transportRoute][]*transportRoute
	originalOutputs     map[*transportRow][]*transportRoute
	physicalOutputs     map[*transportRow][]*transportRoute
	joins               []*transportRoute
	committed           bool
}

// denseMaterialNetwork 职责：表示同物料的一条连续消费干线；使用：输入轨串接和保守毛流上界；边界：每台机器仍属于原产线，消费者只在本行轨上取料。
type denseMaterialNetwork struct {
	id, item                     int
	rows                         []*transportRow
	planned, configured          []float64
	entries, exits, predecessors []*placed
	tracks                       [][]*placed
	joins                        []*transportRoute
	// intake 职责：把多来源汇流留在无机器臂的上游段；使用：保护cargo曲线邻域；边界：节点及接线均进入实际占位、负载和路径核验。
	intake      []*placed
	intakeRoute *transportRoute
	supplyHead  *placed
	// tapped 职责：为需要保留各行喷涂的消费者使用共享原料主干及真实末端过滤；使用：不改供剂角色的共享候选；边界：不会把更多物料直接穿过首个消费者的喷涂机。
	tapped bool
	taps   []*denseMaterialTap
}

// denseMaterialTap 职责：保留原料主干到一个消费者的真实过滤组；使用：喷涂兼容共享；边界：所有并联臂均使用所选等级，并按消费者配置满负荷需求规划。
type denseMaterialTap struct {
	route      *transportRoute
	filters    []*placed
	mainPrefix []*placed
}

// denseMaterialNetworkBinding 职责：将一个规范份额对应到实际聚合供给及消费者位置；使用：真实接线后的路径展开；边界：不改变原edge、rate或to归属。
type denseMaterialNetworkBinding struct {
	group  *denseMaterialNetwork
	source *transportRoute
	target int
}

// denseMaterialInputKey 职责：唯一标识一个消费者原料轨；使用：按物品收集全部真实供给；边界：供剂用途不能混入配方原料。
type denseMaterialInputKey struct {
	row  *transportRow
	item int
}

// planDenseMaterialNetworks 职责：按真实配置毛需求组合有限共享组，并合并同来源同组的物理出货；使用：机器行几何创建之前；边界：外部边及机器回流保持原拓扑，喷涂组仅共享喷涂前的原料供给。
func (b *builder) planDenseMaterialNetworks(ctx context.Context, rows []*transportRow) error {
	enabled, _ := ctx.Value(denseMaterialNetworksKey{}).(bool)
	if !enabled || !b.denseSpatial || b.adapter == nil || b.adapter.mode < 1 || b.adapter.mode > 4 {
		return nil
	}
	type demand struct {
		row              *transportRow
		planned, maximum float64
		order            int
	}
	incoming := map[denseMaterialInputKey][]*transportRoute{}
	for _, route := range b.transportRoutes {
		if route.to != nil && route.inputRole == recipeInputRole {
			key := denseMaterialInputKey{route.to, route.edge.ItemID}
			incoming[key] = append(incoming[key], route)
		}
	}
	byItem := map[int][]demand{}
	capacity := b.logistics.BeltCapacityPerMinute
	for order, row := range rows {
		if recipeHasMachineRecycle(row.recipe) {
			continue
		}
		for _, item := range row.recipe.Items {
			flows := incoming[denseMaterialInputKey{row, item}]
			valid, planned := len(flows) > 0, 0.
			for _, route := range flows {
				if route.from == nil || route.edge.FlowKind != "internal" || recipeHasMachineRecycle(route.from.recipe) || productionSprayItem(route.from.info) == item {
					valid = false
					break
				}
				planned += route.rate
			}
			if !valid {
				continue
			}
			full, err := configuredRoleInputRate(row.info, row.recipe, item, recipeInputRole)
			if err != nil {
				return err
			}
			maximum := full * float64(row.count) / float64(row.info.MachineCount)
			if !finitePositive(maximum) || maximum+1e-6 < planned || maximum > capacity+1e-6 {
				continue
			}
			byItem[item] = append(byItem[item], demand{row, planned, maximum, order})
		}
	}
	plan := &denseMaterialNetworkPlan{canonical: append([]*transportRoute{}, b.transportRoutes...), bindings: map[*transportRoute]denseMaterialNetworkBinding{}, members: map[*transportRoute][]*transportRoute{}, originalOutputs: map[*transportRow][]*transportRoute{}, physicalOutputs: map[*transportRow][]*transportRoute{}}
	items := make([]int, 0, len(byItem))
	for item := range byItem {
		items = append(items, item)
	}
	sort.Ints(items)
	targets := map[denseMaterialInputKey]denseMaterialNetworkBinding{}
	for _, item := range items {
		pending := byItem[item]
		sort.SliceStable(pending, func(i, j int) bool {
			if pending[i].maximum != pending[j].maximum {
				return pending[i].maximum > pending[j].maximum
			}
			return pending[i].order < pending[j].order
		})
		for len(pending) > 0 {
			chosen := []demand{pending[0]}
			pending = pending[1:]
			load := chosen[0].maximum
			for len(chosen) < 4 {
				best := -1
				for i, candidate := range pending {
					if load+candidate.maximum > capacity+1e-6 {
						continue
					}
					if best < 0 || pending[best].row.recipe.ID == chosen[0].row.recipe.ID && candidate.row.recipe.ID != chosen[0].row.recipe.ID {
						best = i
					}
				}
				if best < 0 {
					break
				}
				chosen = append(chosen, pending[best])
				load += pending[best].maximum
				pending = append(pending[:best], pending[best+1:]...)
			}
			if len(chosen) < 2 {
				continue
			}
			group := &denseMaterialNetwork{id: len(plan.groups), item: item}
			for _, consumer := range chosen {
				group.tapped = group.tapped || productionSprayItem(consumer.row.info) != 0
				targets[denseMaterialInputKey{consumer.row, item}] = denseMaterialNetworkBinding{group: group, target: len(group.rows)}
				group.rows = append(group.rows, consumer.row)
				group.planned = append(group.planned, consumer.planned)
				group.configured = append(group.configured, consumer.maximum)
			}
			plan.groups = append(plan.groups, group)
		}
	}
	if len(plan.groups) == 0 {
		return nil
	}
	type supplyKey struct {
		row   *transportRow
		group *denseMaterialNetwork
	}
	aggregates := map[supplyKey]*transportRoute{}
	for _, route := range plan.canonical {
		binding, grouped := targets[denseMaterialInputKey{route.to, route.edge.ItemID}]
		if !grouped || route.inputRole != recipeInputRole {
			plan.physical = append(plan.physical, route)
			continue
		}
		key := supplyKey{route.from, binding.group}
		source := aggregates[key]
		if source == nil {
			copy := *route
			copy.edge.ID = fmt.Sprintf("material-network:%d:%d", binding.group.id, len(plan.physical))
			copy.edge.TargetNodeID = fmt.Sprintf("material-network:%d", binding.group.id)
			copy.rate, copy.to = 0, binding.group.rows[0]
			source = &copy
			aggregates[key] = source
			plan.physical = append(plan.physical, source)
		}
		source.rate += route.rate
		binding.source = source
		plan.bindings[route] = binding
		plan.members[source] = append(plan.members[source], route)
	}
	for _, row := range rows {
		plan.originalOutputs[row] = append([]*transportRoute{}, row.outputs...)
		row.outputs = nil
	}
	for _, route := range plan.physical {
		if route.rate > capacity+math.Max(1e-6, capacity*1e-9) {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "共享消费轨聚合供给超过所选带宽"}
		}
		if route.from != nil {
			route.from.outputs = append(route.from.outputs, route)
		}
	}
	for _, row := range rows {
		plan.physicalOutputs[row] = append([]*transportRoute{}, row.outputs...)
	}
	b.denseMaterialNetworks, b.transportRoutes = plan, plan.physical
	return nil
}

// denseMaterialNetworkRoutes 职责：返回未写入规范净流账的真实串供连接；使用：占位距离、路由和裁尾保留；边界：普通候选返回空。
func (b *builder) denseMaterialNetworkRoutes() []*transportRoute {
	if b.denseMaterialNetworks == nil {
		return nil
	}
	return b.denseMaterialNetworks.joins
}
