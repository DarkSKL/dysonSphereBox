package build

// 文件职责：按真实单带毛容量把mode1同物品对外份额归集到原料同侧的独立交付尾带。
// 使用：所有机器和边界接口落位后、全图路由之前；边界：canonical份额不变，不接塔/配送/混带，不将成品或副产物接入原料带。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// mode1DeliveryIdentity 职责：冻结规范份额及实际生产起点；使用：路由后独立证明；边界：消费者和流量不能被交付聚合改写。
type mode1DeliveryIdentity struct {
	edge         quant.FlowEdge
	rate         float64
	from, to     *transportRow
	role         productionInputRole
	first, start *placed
}

// mode1DeliveryGroup 职责：登记同物料容量组、独立共享尾带和唯一监测；使用：路由及终检；边界：head/end在创建时固定身份，不能从成员路径末节点反推。
type mode1DeliveryGroup struct {
	item    int
	members []*transportRoute
	// maximum/planned职责：区分配置毛能力与规范净产量；使用：实际带宽和监测期望；边界：不得互相替代。
	maximum, planned float64
	// sources职责：按真实生产行去重毛贡献；使用：多个规范份额共享上游时；边界：不同生产行不得互相抵消。
	sources map[*transportRow]float64
	// head/end职责：固定真实尾链身份；使用：路由、裁尾和终检；边界：不能从可变path反推。
	head, end *placed
	// monitor职责：绑定本组唯一边界监测；使用：幂等构造和最终数量复核；边界：机器轨道监测不能冒充该交付监测。
	monitor *placed
}

// mode1DeliveryPlan 职责：保存全部左侧交付的规范身份和已独立证明的合流边；使用：公共物流、producer保护和裁尾；边界：edges只有完整终检成功后才有效。
type mode1DeliveryPlan struct {
	groups              []*mode1DeliveryGroup
	byRoute             map[*transportRoute]*mode1DeliveryGroup
	identities          map[*transportRoute]mode1DeliveryIdentity
	inputs              []*placed
	committed, verified bool
	edges               map[[2]*placed]bool
}

// mode1DeliveryGroup 职责：按真实路线身份取得交付组；使用：普通/密铺共用路由；边界：组外路线不靠相同物品猜身份。
func (b *builder) mode1DeliveryGroup(route *transportRoute) *mode1DeliveryGroup {
	if b.mode1Deliveries == nil {
		return nil
	}
	return b.mode1Deliveries.byRoute[route]
}

// planMode1DeliveryGroups 职责：按物品、生产行毛能力及净份额计算最终交付组；使用：原料口预留与交付实体创建共用；边界：不创建实体、不登记builder计划、不修改路线或要求边界已创建。
func (b *builder) planMode1DeliveryGroups(ctx context.Context) ([]*mode1DeliveryGroup, error) {
	groups := []*mode1DeliveryGroup{}
	for _, route := range b.transportRoutes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if route.from == nil || route.to != nil {
			continue
		}
		full, e := b.stationOutputMaximum(route)
		if e != nil {
			return nil, e
		}
		var chosen *mode1DeliveryGroup
		for _, g := range groups {
			extra := full
			if _, known := g.sources[route.from]; known {
				extra = 0
			}
			if g.item == route.edge.ItemID && g.maximum+extra <= b.logistics.BeltCapacityPerMinute+1e-6 && g.planned+route.rate <= b.logistics.BeltCapacityPerMinute+1e-6 {
				chosen = g
				break
			}
		}
		if chosen == nil {
			chosen = &mode1DeliveryGroup{item: route.edge.ItemID, sources: map[*transportRow]float64{}}
			groups = append(groups, chosen)
		}
		if _, known := chosen.sources[route.from]; !known {
			chosen.sources[route.from] = full
			chosen.maximum += full
		}
		chosen.planned += route.rate
		chosen.members = append(chosen.members, route)
	}
	return groups, nil
}

// mode1DeliveryPositions 职责：分配原料同侧的独立地面交付槽；使用：交付实体创建之前；边界：密铺读取联合边界计划，其它布局保留原有独立槽，不把高层入口当成地面占位。
func (b *builder) mode1DeliveryPositions(ctx context.Context, groups []*mode1DeliveryGroup, inputs []vector) ([]vector, error) {
	if b.denseBoundary != nil {
		positions := make([]vector, len(groups))
		if len(b.denseBoundary.outputs) != len(groups) {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "交付组与联合边界规划不一致"}
		}
		for i, group := range groups {
			position, ok := b.denseBoundary.outputs[group.members[0]]
			if !ok {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "交付组与联合边界规划不一致"}
			}
			positions[i] = position
		}
		return positions, nil
	}
	low, _ := bounds(b.all)
	x := math.Floor(low.x) - 5
	if len(inputs) > 0 {
		x = inputs[0].x
		for _, p := range inputs {
			x = math.Min(x, p.x)
		}
	}
	used := append([]vector(nil), inputs...)
	positions := make([]vector, 0, len(groups))
	for i, g := range groups {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		desired := int(math.Round(g.members[0].start.a.y))
		free := func(y int) bool {
			for _, p := range used {
				if b.polarFrame != nil && math.Abs(p.z) > 1e-6 {
					continue
				}
				if math.Abs(p.y-float64(y)) < 2-1e-6 {
					return false
				}
			}
			return true
		}
		chosen := desired
		if b.polarFrame != nil {
			chosen = i * 2
			if chosen >= b.polarFrame.height || !free(chosen) {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选原生球面缺少独立地面成品或副产物交付槽"}
			}
		} else {
			for offset := 0; ; offset++ {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				if free(desired + offset) {
					chosen = desired + offset
					break
				}
				if free(desired - offset) {
					chosen = desired - offset
					break
				}
			}
		}
		position := vector{x, float64(chosen), 0}
		positions = append(positions, position)
		used = append(used, position)
	}
	return positions, nil
}

// prepareMode1Deliveries 职责：按物品和不同来源毛能力分组并建立独立西向尾带；使用：原料接口已生成之后；边界：先完整证明槽位容量再创建实体，所有成员仍独立真实寻路。
func (b *builder) prepareMode1Deliveries(ctx context.Context, inputs []vector) error {
	if b.mode1Deliveries != nil {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "同一候选重复创建交付边界"}
	}
	groups, err := b.planMode1DeliveryGroups(ctx)
	if err != nil {
		return err
	}
	plan := &mode1DeliveryPlan{groups: groups, byRoute: map[*transportRoute]*mode1DeliveryGroup{}, identities: map[*transportRoute]mode1DeliveryIdentity{}}
	for _, group := range groups {
		for _, route := range group.members {
			plan.byRoute[route] = group
		}
	}
	for _, route := range b.transportRoutes {
		if route.from == nil && route.boundary != nil {
			plan.inputs = append(plan.inputs, route.boundary)
		}
		if plan.byRoute[route] == nil {
			continue
		}
		if route.start == nil || len(route.path) == 0 {
			return transportAllocationError(route.edge, route.rate, "交付份额缺少真实生产接带")
		}
		plan.identities[route] = mode1DeliveryIdentity{edge: route.edge, rate: route.rate, from: route.from, to: route.to, role: route.inputRole, first: route.path[0], start: route.start}
	}
	positions, err := b.mode1DeliveryPositions(ctx, plan.groups, inputs)
	if err != nil {
		return err
	}
	for i, g := range plan.groups {
		position := positions[i]
		g.end = b.make(2000+b.setting.BeltLv, position.x, position.y, 0)
		g.head = b.make(2000+b.setting.BeltLv, position.x+1, position.y, 0)
		connect(g.head, g.end)
		g.end.record.Parameters = []int32{int32(g.item), int32(math.Ceil(g.planned))}
		g.head.record.Parameters = append([]int32{}, g.end.record.Parameters...)
		for _, route := range g.members {
			route.end = g.head
			route.boundary = g.end
		}
	}
	b.mode1Deliveries = plan
	return nil
}

// commitMode1Deliveries 职责：把已真实路由到共享头的所有份额接到预先登记交付末端；使用：公共路由完成后、producer/canonical展开前；边界：不制造额外实体或猜测路径末端。
func (b *builder) commitMode1Deliveries() error {
	p := b.mode1Deliveries
	if p == nil || p.committed {
		return nil
	}
	for _, g := range p.groups {
		if g.head.output != g.end || g.end.output != nil {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "预先登记交付尾带已改变"}
		}
		for _, r := range g.members {
			if r.end != g.head || r.boundary != g.end || len(r.path) == 0 || r.path[len(r.path)-1] != g.head {
				return transportAllocationError(r.edge, r.rate, "交付份额没有抵达预先登记的真实合流头")
			}
			r.path = append(r.path, g.end)
			r.end = g.end
		}
	}
	p.committed = true
	return nil
}

// preserveMode1Deliveries 职责：在任何机器带裁尾前保护预先登记的交付头/末端；使用：公共裁尾；边界：不依赖可能被篡改的成员path保护边界身份。
func (b *builder) preserveMode1Deliveries(required map[*placed]bool) {
	if p := b.mode1Deliveries; p != nil {
		for _, g := range p.groups {
			required[g.head], required[g.end] = true, true
		}
	}
}

// mode1DeliveryAllowsEdge 职责：只向producer保护规则开放已完整证明的同物品交付合流边；使用：交付终检之后；边界：未验证计划或新增无关来源一律不放行。
func (b *builder) mode1DeliveryAllowsEdge(from, to *placed) bool {
	p := b.mode1Deliveries
	return p != nil && p.verified && p.edges[[2]*placed{from, to}]
}

// aggregateMode1DeliveryPorts 职责：将同一实际交付口同一规范边的展示速率合计；使用：lanes逐份额记账后；边界：保留不同规范边身份，物理共享靠相同BuildingIndex表达。
func (b *builder) aggregateMode1DeliveryPorts(summary *LogisticsSummary) {
	if b.mode1Deliveries == nil {
		return
	}
	ends := map[int]bool{}
	index := map[*placed]int{}
	for i, p := range b.all {
		index[p] = i
	}
	for _, g := range b.mode1Deliveries.groups {
		ends[index[g.end]] = true
	}
	type key struct {
		index, item int
		edge, kind  string
	}
	seen := map[key]int{}
	out := []BoundaryPort{}
	for _, port := range summary.ExternalPorts {
		if !ends[port.BuildingIndex] {
			out = append(out, port)
			continue
		}
		k := key{port.BuildingIndex, port.ItemID, port.EdgeID, port.Kind}
		if i, ok := seen[k]; ok {
			out[i].RatePerMinute += port.RatePerMinute
		} else {
			seen[k] = len(out)
			out = append(out, port)
		}
	}
	summary.ExternalPorts = out
}
