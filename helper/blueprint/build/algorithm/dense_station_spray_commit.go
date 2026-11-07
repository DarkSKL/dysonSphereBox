package build

// 文件职责：在真实供剂串供接通后恢复逐owner规范路径，并在最终库存上重新核对同物品连续供给。
// 使用：塔黑盒的boundary commit与最终核验；边界：不重复供给总额、不恢复过时原料网络切片、不借用幽灵带或未声明入口。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// commitDenseStationSprayNetwork 职责：沿实际输出指针展开每行供剂的源到入口路径；使用：共享原料网络与附件提交之后；边界：只更新本网络的原始份额对象。
func (b *builder) commitDenseStationSprayNetwork(ctx context.Context) error {
	plan := b.denseStationSprayNetwork
	if plan == nil {
		return nil
	}
	if plan.committed {
		return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "塔供剂网络重复提交"}
	}
	alive := map[*placed]bool{}
	for _, p := range b.all {
		alive[p] = true
	}
	aggregates := map[*transportRoute]*denseStationSprayGroup{}
	for _, group := range plan.groups {
		physical := group.physical
		last := group.members[len(group.members)-1].tail
		chain := []*placed{}
		seen := map[*placed]bool{}
		cursor := physical.start
		for cursor != nil && alive[cursor] && surfaceIsBelt(cursor.record.ItemID) && !seen[cursor] {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "塔供剂真实链展开已取消"}
			}
			seen[cursor] = true
			chain = append(chain, cursor)
			if cursor == last {
				break
			}
			cursor = cursor.output
		}
		if len(chain) == 0 || chain[len(chain)-1] != last {
			return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "塔供剂没有沿真实带串接到全部消费者尾端"}
		}
		positions := map[*placed]int{}
		for i, p := range chain {
			positions[p] = i
		}
		for _, member := range group.members {
			end, ok := positions[member.entry]
			if !ok {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂串供链遗漏实际行入口"}
			}
			route := member.route
			route.start, route.end, route.boundary = physical.start, member.entry, physical.boundary
			route.path = append([]*placed(nil), chain[:end+1]...)
			route.sharedPath = nil
			route.filters = nil
		}
		group.chain = chain
		aggregates[physical] = group
	}
	restored := []*transportRoute{}
	seen := map[*transportRoute]bool{}
	appendRoute := func(route *transportRoute) error {
		if seen[route] {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "塔供剂规范份额重复恢复"}
		}
		seen[route] = true
		restored = append(restored, route)
		return nil
	}
	for _, route := range b.transportRoutes {
		if group := aggregates[route]; group != nil {
			for _, member := range group.members {
				if err := appendRoute(member.route); err != nil {
					return err
				}
			}
		} else if err := appendRoute(route); err != nil {
			return err
		}
	}
	for route := range plan.byRoute {
		if !seen[route] {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "塔供剂原始规范份额在共享网络恢复后丢失"}
		}
	}
	b.transportRoutes = restored
	plan.committed = true
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-station-spray-network", Status: "passed", Message: fmt.Sprintf("%d条原始外供剂份额由%d个真实塔供剂口串接；所有喷涂机保留，逐行规范rate和owner不变，整组配置毛需求限制在所选单带内，不采用自喷节省", len(plan.byRoute), len(plan.groups))})
	return nil
}

// validateDenseStationSprayNetwork 职责：在最终存活实体上拒绝断链、隐藏喂料及错误供剂份额；使用：最终塔记录检查；边界：只允许本组塔端唯一入口及真实喷涂通道消费。
func (b *builder) validateDenseStationSprayNetwork(ctx context.Context) error {
	plan := b.denseStationSprayNetwork
	if plan == nil {
		return nil
	}
	if !plan.committed {
		return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "塔供剂网络尚未提交实际路径"}
	}
	alive := map[*placed]bool{}
	for _, p := range b.all {
		alive[p] = true
	}
	for _, group := range plan.groups {
		members := map[*placed]bool{}
		var allowedSource *placed
		for _, link := range b.stationLinks {
			if !link.output && link.head == group.physical.start && link.tower == group.physical.boundary && len(link.path) > 0 {
				allowedSource = link.path[0]
			}
		}
		if allowedSource == nil {
			return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂串供没有唯一已登记的真实塔源"}
		}
		positions := map[*placed]int{}
		for i, node := range group.chain {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终塔供剂网络核验已取消"}
			}
			if !alive[node] || !surfaceIsBelt(node.record.ItemID) || members[node] || i > 0 && group.chain[i-1].output != node {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "最终塔供剂串供链中断、重复或含库存外节点"}
			}
			members[node] = true
			positions[node] = i
		}
		for _, node := range b.all {
			if node.output != nil && members[node.output] && !members[node] && node != allowedSource {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂串供链出现未声明的额外喂料"}
			}
		}
		rate, maximum := 0., 0.
		for _, member := range group.members {
			route, row := member.route, member.route.to
			full, err := configuredRoleInputRate(row.info, row.recipe, route.edge.ItemID, sprayInputRole)
			if err != nil {
				return err
			}
			maximum += full * float64(len(row.line.machines)) / float64(row.info.MachineCount)
			rate += route.rate
			end, exists := positions[member.entry]
			if !exists || route.inputRole != sprayInputRole || route.from != nil || route.start != group.physical.start || route.end != member.entry || route.boundary != group.physical.boundary || len(route.path) != end+1 {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "最终供剂份额没有保留实际行入口和唯一源路径"}
			}
			for i, node := range route.path {
				if group.chain[i] != node {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "供剂份额路径与真实串供链不一致"}
				}
			}
		}
		if math.Abs(rate-group.physical.rate) > 1e-6 || math.Abs(maximum-group.maximum) > 1e-6 || maximum > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "串供剂的计划份额或配置毛流超过已验证物理单带能力"}
		}
	}
	return nil
}

// denseStationSprayMonitorViews 职责：把已证明共用同一物理入口的供剂份额作为一条真实监测边界；使用：设施监测视图转换之前；边界：只改临时视图，缺失首成员或身份不符时保持原视图让严格检查拒绝。
func (b *builder) denseStationSprayMonitorViews(routes []*transportRoute) []*transportRoute {
	plan := b.denseStationSprayNetwork
	if plan == nil || !plan.committed {
		return routes
	}
	match := func(view *transportRoute, member *denseStationSprayMember, group *denseStationSprayGroup) bool {
		return view != nil && view.from == nil && view.inputRole == sprayInputRole && view.edge.ID == member.route.edge.ID && view.edge.ItemID == member.route.edge.ItemID && view.to == member.route.to && view.end == member.entry && view.start == group.physical.start && view.boundary == group.physical.boundary
	}
	firstViews := map[*denseStationSprayGroup]*transportRoute{}
	for _, group := range plan.groups {
		for _, view := range routes {
			if match(view, group.members[0], group) {
				copy := *view
				copy.rate = group.physical.rate
				copy.edge = group.physical.edge
				firstViews[group] = &copy
				break
			}
		}
	}
	views := []*transportRoute{}
	seen := map[*denseStationSprayGroup]bool{}
	for _, view := range routes {
		var shared *denseStationSprayGroup
		for _, group := range plan.groups {
			if firstViews[group] == nil {
				continue
			}
			for _, member := range group.members {
				if match(view, member, group) {
					shared = group
					break
				}
			}
			if shared != nil {
				break
			}
		}
		if shared == nil {
			views = append(views, view)
		} else if !seen[shared] {
			views = append(views, firstViews[shared])
			seen[shared] = true
		}
	}
	return views
}
