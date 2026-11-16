package build

// 文件职责：把低流量外供增产剂接成真实串供轨，减少重复塔口而不减少喷涂或修改规范需求。
// 使用：独立大厂黑盒候选；边界：只串接纯外供的同剂种，整组配置满载供剂不超过单带，内部自产剂仍沿原有闭环。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseStationSprayMember 职责：保留一行供剂份额和可真实延续的端点；使用：串接与规范路径恢复；边界：配方原料即使同item也不属于此成员。
type denseStationSprayMember struct {
	route             *transportRoute
	entry, tail, exit *placed
	maximum           float64
	module            *denseStreetAccessory
	assembly          *denseStreetAssembly
}

// denseStationSprayGroup 职责：保存一个真实塔供剂口服务的连续消费者；使用：完整路由前后交接；边界：canonical份额各自保留，不把总额重复给每行。
type denseStationSprayGroup struct {
	members  []*denseStationSprayMember
	physical *transportRoute
	chain    []*placed
	maximum  float64
}

// denseStationSprayPlan 职责：保存本候选独占供剂网络及待注入占位的真实出口；使用：共同装配的prepare/order/commit三个阶段；边界：不覆盖共享原料网络的恢复结果。
type denseStationSprayPlan struct {
	groups    []*denseStationSprayGroup
	pending   map[*placed]*placed
	byRoute   map[*transportRoute]*denseStationSprayGroup
	committed bool
}

// denseStationSprayTail 职责：从实际供剂链及尚待提交的附件契约找出末端；使用：串接准备；边界：不按Y排序猜测已旋转或分离附件的方向。
func (b *builder) denseStationSprayTail(row *transportRow, entry *placed) (*placed, *denseStreetAccessory, *denseStreetAssembly, error) {
	for _, work := range b.denseConnectedStreets {
		for group, member := range work.rows {
			if member == row {
				var last *denseStreetAccessory
				for _, module := range work.assembly.accessories {
					if module.group != group || module.supplyTail == nil {
						continue
					}
					used := false
					for _, link := range work.assembly.links {
						used = used || link.purpose == "spray" && link.start == module.supplyTail
					}
					if !used {
						if last != nil {
							return nil, nil, nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "独立附件供剂存在多个未解释的尾端"}
						}
						last = module
					}
				}
				if last != nil {
					return last.supplyTail, last, work.assembly, nil
				}
			}
		}
	}
	seen := map[*placed]bool{}
	tail := entry
	for tail != nil && surfaceIsBelt(tail.record.ItemID) && !seen[tail] {
		seen[tail] = true
		if tail.output == nil {
			return tail, nil, nil, nil
		}
		tail = tail.output
	}
	return nil, nil, nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "外供剂没有可确认的真实单向尾端"}
}

// prepareDenseStationSprayNetwork 职责：聚合真正同物品外供并建立消费者间的实际连接需求；使用：建塔之前；边界：只改变物理路线计划，原edge/rate/owner在commit中逐项恢复。
func (b *builder) prepareDenseStationSprayNetwork(ctx context.Context) error {
	byItem := map[int][]*transportRoute{}
	for _, route := range b.transportRoutes {
		if route == nil || route.from != nil || route.to == nil || route.inputRole != sprayInputRole || route.end == nil {
			continue
		}
		pure := true
		for _, other := range b.transportRoutes {
			if other != route && other.to == route.to && other.edge.ItemID == route.edge.ItemID && other.inputRole == sprayInputRole {
				pure = false
				break
			}
		}
		if pure {
			byItem[route.edge.ItemID] = append(byItem[route.edge.ItemID], route)
		}
	}
	items := []int{}
	for item := range byItem {
		items = append(items, item)
	}
	sort.Ints(items)
	plan := &denseStationSprayPlan{pending: map[*placed]*placed{}, byRoute: map[*transportRoute]*denseStationSprayGroup{}}
	capacity := b.logistics.BeltCapacityPerMinute
	for _, item := range items {
		pending := byItem[item]
		for len(pending) > 0 {
			group := &denseStationSprayGroup{}
			for len(pending) > 0 && len(group.members) < 32 {
				if ctx.Err() != nil {
					return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "塔供剂串供规划已取消"}
				}
				route := pending[0]
				row := route.to
				full, err := configuredRoleInputRate(row.info, row.recipe, item, sprayInputRole)
				if err != nil {
					return err
				}
				maximum := full * float64(row.count) / float64(row.info.MachineCount)
				if !finitePositive(maximum) || maximum > capacity+1e-6 {
					return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "单行真实供剂配置毛流超过带能力"}
				}
				if group.maximum+maximum > capacity+1e-6 {
					break
				}
				pending = pending[1:]
				tail, module, assembly, err := b.denseStationSprayTail(row, route.end)
				if err != nil {
					return err
				}
				group.members = append(group.members, &denseStationSprayMember{route: route, entry: route.end, tail: tail, maximum: maximum, module: module, assembly: assembly})
				group.maximum += maximum
			}
			if len(group.members) < 2 {
				continue
			}
			copy := *group.members[0].route
			copy.edge.ID = fmt.Sprintf("station-spray:%d:%d", item, len(plan.groups))
			copy.edge.TargetNodeID = copy.edge.ID
			copy.rate = 0
			for _, member := range group.members {
				copy.rate += member.route.rate
				plan.byRoute[member.route] = group
			}
			group.physical = &copy
			remaining := group.maximum
			for i, member := range group.members[:len(group.members)-1] {
				remaining -= member.maximum
				var previous *placed
				for _, entity := range b.all {
					if surfaceIsBelt(entity.record.ItemID) && entity.output == member.tail {
						if previous != nil {
							return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂末端存在多个前驱，不能猜测串供出口"}
						}
						previous = entity
					}
				}
				if previous == nil {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂末端缺少真实邻带"}
				}
				dx, dy := member.tail.a.x-previous.a.x, member.tail.a.y-previous.a.y
				length := math.Hypot(dx, dy)
				if length < 1e-6 {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂末端没有可延续的水平真实方向"}
				}
				owner := b.owner
				b.owner = member.tail.owner
				exit := b.make(2000+b.setting.BeltLv, member.tail.a.x+dx/length, member.tail.a.y+dy/length, member.tail.a.z)
				b.owner = owner
				connect(member.tail, exit)
				member.exit = exit
				plan.pending[member.tail] = exit
				if module := member.module; module != nil {
					module.supplyInner = member.tail
					module.supplyTail = exit
					module.entities = append(append([]*placed(nil), module.entities...), exit)
					member.assembly.all = append(append([]*placed(nil), member.assembly.all...), exit)
				}
				member.route.to.line.all = append(append([]*placed(nil), member.route.to.line.all...), exit)
				next := group.members[i+1]
				b.denseStreetRoutes = append(b.denseStreetRoutes, &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("station-spray-chain:%d:%d", len(plan.groups), i), ItemID: item, FlowKind: "station-spray-chain"}, rate: remaining, from: member.route.to, to: next.route.to, start: exit, end: next.entry, path: []*placed{exit}, inputRole: sprayInputRole})
			}
			plan.groups = append(plan.groups, group)
		}
	}
	if len(plan.groups) == 0 {
		return nil
	}
	physical := []*transportRoute{}
	added := map[*denseStationSprayGroup]bool{}
	for _, route := range b.transportRoutes {
		if group := plan.byRoute[route]; group != nil {
			if !added[group] {
				physical = append(physical, group.physical)
				added[group] = true
			}
		} else {
			physical = append(physical, route)
		}
	}
	b.transportRoutes, b.denseStationSprayNetwork = physical, plan
	return nil
}

// attachDenseStationSprayExits 职责：把真实供剂延伸节点归入其已有刚性单元；使用：全场落位之前；边界：出口不能遗留在未平移的局部原点。
func (b *builder) attachDenseStationSprayExits(units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	plan := b.denseStationSprayNetwork
	if plan == nil {
		return units, nil
	}
	seen := map[*placed]bool{}
	for i := range units {
		for _, entity := range append([]*placed(nil), units[i].entities...) {
			if exit := plan.pending[entity]; exit != nil {
				if seen[exit] {
					return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "供剂出口被分配到多个占位单元"}
				}
				seen[exit] = true
				// unit可能是b.all的子切片；扩容前独立复制，不能覆盖下一单元的首实体。
				units[i].entities = append(append([]*placed(nil), units[i].entities...), exit)
				found := false
				for p := range units[i].ports {
					if units[i].ports[p].head == entity {
						units[i].ports[p].head, units[i].ports[p].inner = exit, entity
						found = true
					}
				}
				if !found {
					units[i].ports = append(units[i].ports, denseStreetPort{head: exit, inner: entity})
				}
			}
		}
	}
	if len(seen) != len(plan.pending) {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "供剂出口没有归入真实占位单元"}
	}
	for i, unit := range units {
		members := map[*placed]bool{}
		for _, entity := range unit.entities {
			members[entity] = true
		}
		for _, entity := range unit.entities {
			if entity.output != nil && !members[entity.output] {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("供剂出口归属后单元%d仍有跨单元预连接：%d %+v -> %d %+v", i, entity.record.ItemID, entity.a, entity.output.record.ItemID, entity.output.a)}
			}
		}
	}
	return units, nil
}
