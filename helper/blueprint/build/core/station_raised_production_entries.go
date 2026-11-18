package build

// 文件职责：为普通原生大厂的独立生产出口构造真实升层短带，避免从拥挤地面反复搜索主路。
// 边界：只改变已有带的实体接驳，生产量、机器、带容量及最大高度不变，沿用原portal、整链和最终规则。
import (
	"context"
	"math"
)

// raiseStationProductionEntries 职责：为普通原生塔大厂的独立生产出口和真实塔供料源口构造通过原规则的实体升层接驳。
// 使用：由prepareStationTransportEntries在全场布局固定后、共同路由前明确调用；依次准备交付端、生产源及配方入口的真实带链。
// 边界：不改变输入分配、制造实体、配方、产量、容量及高度规则；共享或已连接源保持原样，未找到合法portal时不提交升层链，后续仍须完整终检。
func (b *builder) raiseStationProductionEntries(ctx context.Context) error {
	machines, _ := ctx.Value(generationWorkloadKey{}).(int)
	if b.policy == nil || b.policy.density != densityStandard || b.policy.station == nil || b.directBoundaryUnits == nil || b.polarFrame == nil || machines < b.policy.search.LargeLayoutThreshold {
		return nil
	}
	idx, err := b.newDenseRoutePortIndex(ctx)
	if err != nil {
		return err
	}
	low, high := bounds(b.all)
	env := &transportEnvironment{densePorts: idx, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}, minimum: vector{math.Max(-8, math.Floor(low.x)-8), math.Max(0, math.Floor(low.y)-8), 0}, maximum: vector{math.Min(math.Ceil(high.x)+8, b.polarFrame.longitudeLimit()+8), float64(b.polarFrame.height - 1), transportMaxHeight}, beltCorridors: map[[2]int]bool{}}
	for _, p := range b.all {
		if surfaceIsBelt(p.record.ItemID) {
			idx.blockNode(env.blocked, p.a)
		}
	}
	routes := b.transportRoutingRoutes()
	env.reserveDenseBoundaryEscapes(routes)
	if err := b.raiseStationDeliveryEntries(ctx, env, routes); err != nil {
		return err
	}
	uses := map[*placed]int{}
	for _, r := range routes {
		if r.start != nil {
			uses[r.transportSource()]++
		}
	}
	owner := b.owner
	defer func() { b.owner = owner }()
	b.owner = -1
	for _, r := range routes {
		if err := ctx.Err(); err != nil {
			return err
		}
		producer := r.from != nil && r.to != nil && r.from.line != nil
		supply := r.from == nil && r.to != nil && r.boundary != nil && surfaceIsStation(r.boundary.record.ItemID)
		if (!producer && !supply) || r.start == nil || r.end == nil || r.routingEntry != nil || uses[r.start] != 1 || r.start.output != nil || len(r.path) == 0 || r.path[len(r.path)-1] != r.start {
			continue
		}
		source := r.start
		if source.a.z >= 8 {
			continue
		}
		found := false
		preferred := denseExternalStep(r)
		if producer {
			preferred = denseRotateVector(vector{x: -1}, r.from.quarterTurns)
		}
		for _, height := range []float64{8, 16, transportMaxHeight} {
			for _, portal := range env.densePortals(source, source, r.end, preferred, true, false, height) {
				// 接驳必须包含原源点及至少一个新点，才可读取末点并提交非空实体链。
				if len(portal.points) < 2 || portal.points[0] != source.a {
					continue
				}
				last := portal.points[len(portal.points)-1]
				if portal.height < 8 || last.z < 8 {
					continue
				}
				virtualEnd := &placed{a: last, record: source.record}
				if !env.densePathClear(portal.points, source, virtualEnd) {
					continue
				}
				if len(b.all)+len(portal.points)-1 > generationBuildingBudget(ctx) {
					continue
				}
				begin := len(b.all)
				nodes := b.byPath(portal.points[1:])
				connect(source, nodes[0])
				r.routingEntry = nodes[len(nodes)-1]
				r.path = append(r.path, nodes...)
				idx.addNode(source)
				for _, p := range nodes {
					idx.blockNode(env.blocked, p.a)
					idx.addNode(p)
				}
				b.previewRouteAppend(begin, source)
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	return b.raiseStationConsumerEntries(ctx, env, routes)
}
