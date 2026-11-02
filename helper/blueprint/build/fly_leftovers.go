package build

// 文件职责：把配送模式的规范剩余份额分离为真实独立过滤支路和原料区回收箱。
// 使用：完整机器块装配后、球面编码前；边界：不借配送机队伪装实体导流，不合并外供账，失败或取消恢复全部已有实体。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// flyLeftoverMove 职责：保存一份已经过规范守恒分配的回收迁移；使用：事务式装配；边界：部分份额必须新建独立过滤支路，不能搬走仍服务其它边的箱体。
type flyLeftoverMove struct {
	endpoint *flyTransportEndpoint
	edge     quant.FlowEdge
	rate     float64
}

// flyEndpointsForEdge 职责：隔离已指定规范边的真实回收端点；使用：普通配送边分配；边界：空标识兼容普通端点，不能把回收箱分给内部消费。
func flyEndpointsForEdge(endpoints []*flyTransportEndpoint, edgeID string) []*flyTransportEndpoint {
	eligible := make([]*flyTransportEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.flowEdgeID == edgeID {
			eligible = append(eligible, endpoint)
		}
	}
	for _, endpoint := range endpoints {
		if endpoint.flowEdgeID == "" {
			eligible = append(eligible, endpoint)
		}
	}
	return eligible
}

// prepareFlyLeftovers 职责：按规范剩余边迁移或拆分本组真实配送出口；使用：mode2 全图装配收尾；边界：保留闭合块，取消和失败恢复所有实体与表项，模式5不宣称原料侧回收。
func (b *builder) prepareFlyLeftovers(ctx context.Context, mode int) (err error) {
	if mode != 2 || b.flowGraph == nil {
		return nil
	}
	if err := b.checkGeneration("配送副产物导流"); err != nil {
		return err
	}
	moves, err := b.planFlyLeftovers(ctx)
	if err != nil || len(moves) == 0 {
		return err
	}
	// 保存指针身份和记录参数的深副本；失败不让路由、旧链拆除或容量份额
	// 泄露到下一候选。原计划与规范图从始至终只读。
	originalAll := append([]*placed(nil), b.all...)
	originalEndpoints := append([]*flyTransportEndpoint(nil), b.flyEndpoints...)
	placedSnapshots := map[*placed]placed{}
	endpointSnapshots := map[*flyTransportEndpoint]flyTransportEndpoint{}
	for _, entity := range originalAll {
		snapshot := *entity
		snapshot.record.Parameters = append([]int32(nil), entity.record.Parameters...)
		placedSnapshots[entity] = snapshot
	}
	for _, endpoint := range originalEndpoints {
		endpointSnapshots[endpoint] = *endpoint
	}
	owner, surfaceBlocks, layoutBlocks := b.owner, b.surfaceBlocks, b.flyLayoutBlocks
	zoneRanges := append([]zoneRange(nil), b.zoneRanges...)
	defer func() {
		b.owner = owner
		if err == nil {
			return
		}
		for entity, snapshot := range placedSnapshots {
			*entity = snapshot
		}
		for endpoint, snapshot := range endpointSnapshots {
			*endpoint = snapshot
		}
		b.all, b.flyEndpoints, b.surfaceBlocks = originalAll, originalEndpoints, surfaceBlocks
		b.flyLayoutBlocks, b.zoneRanges = layoutBlocks, zoneRanges
	}()
	blocks := layoutBlocks
	if len(blocks) == 0 {
		blocks = []surfaceBlockRange{{begin: 0, end: len(originalAll)}}
	}
	combined, rebuilt := []*placed{}, []surfaceBlockRange{}
	newZones := append([]zoneRange(nil), zoneRanges...)
	processed := 0
	for _, block := range blocks {
		b.all = append([]*placed(nil), originalAll[block.begin:block.end]...)
		inside := map[*placed]bool{}
		for _, p := range b.all {
			inside[p] = true
		}
		changed := false
		for _, move := range moves {
			if !inside[move.endpoint.link] {
				continue
			}
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送副产物导流已取消或超时"}
			}
			if err = b.moveFlyLeftover(ctx, move); err != nil {
				return err
			}
			changed = true
			processed++
		}
		if changed {
			if err = b.completeFlyPower(0); err != nil {
				return err
			}
		}
		begin := len(combined)
		combined = append(combined, b.all...)
		rebuilt = append(rebuilt, surfaceBlockRange{begin: begin, end: len(combined)})
		for index, zone := range zoneRanges {
			if block.begin == zone.begin {
				newZones[index].begin = begin
			}
			if block.end == zone.end {
				newZones[index].end = len(combined)
			}
		}
	}
	if processed != len(moves) {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "副产物来源未完整包含在真实闭合配送块中"}
	}
	b.all, b.flyLayoutBlocks, b.zoneRanges = combined, rebuilt, newZones
	if len(surfaceBlocks) > 0 {
		b.surfaceBlocks = rebuilt
	}
	// 新回收箱会扩展轮廓，按闭合块重新排布，不能沿用旧小矩形而压住下一行。
	zoneX, nextX, nextY, columnWidth := 0., 0., 0., 0.
	heightLimit := b.flyLayoutHeight
	if heightLimit <= 0 {
		heightLimit = b.setting.BlockWidthLimit
	}
	for _, zone := range newZones {
		nextX, nextY, columnWidth = zoneX, 0, 0
		packing := &connectedPacking{cells: map[[3]int][]int{}}
		for _, block := range rebuilt {
			if block.begin < zone.begin || block.end > zone.end {
				continue
			}
			entities := b.all[block.begin:block.end]
			low, high := bounds(entities)
			width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
			if nextY > 0 && nextY+height > heightLimit {
				nextX, nextY, columnWidth = nextX+columnWidth+2, 0, 0
			}
			x, y := nextX, nextY
			if b.flyCompact {
				contour, shapeErr := b.connectedBlockShape(ctx, &transportRow{}, entities)
				if shapeErr != nil {
					return shapeErr
				}
				x, y, err = packing.place(ctx, contour, low, high, nextX, nextY, heightLimit)
				if err != nil {
					return err
				}
			}
			shift(entities, x-low.x, y-low.y, 0)
			nextY = math.Max(nextY, y+height+2)
			columnWidth = math.Max(columnWidth, x-nextX+width)
		}
		_, high := bounds(b.all[zone.begin:zone.end])
		zoneX = math.Ceil(high.x) + 3
	}
	if ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送副产物导流提交已取消或超时"}
	}
	return b.validateFlyEndpointTopology(mode)
}

// planFlyLeftovers 职责：复用真实配送份额分配顺序取得规范剩余边来源；使用：任何实体变更之前；边界：不改端点速率或份额，错误不返回部分计划。
func (b *builder) planFlyLeftovers(ctx context.Context) ([]flyLeftoverMove, error) {
	owners := map[string]int{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			owners[node.ID] = node.ProductionIndex
		}
	}
	outputs, remaining := map[int]map[int][]*flyTransportEndpoint{}, map[*flyTransportEndpoint]float64{}
	for _, endpoint := range b.flyEndpoints {
		if endpoint == nil {
			return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送回收规划遇到空端点、已分配端点或无效速率"}
		}
		if endpoint.flowEdgeID != "" || !finitePositive(endpoint.rate) {
			return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: fmt.Sprintf("配送回收规划遇到已分配端点或无效速率：产线%d、物品%d、输入%t、速率%.9g/min", endpoint.owner, endpoint.item, endpoint.input, endpoint.rate)}
		}
		if endpoint.input {
			continue
		}
		if outputs[endpoint.owner] == nil {
			outputs[endpoint.owner] = map[int][]*flyTransportEndpoint{}
		}
		outputs[endpoint.owner][endpoint.item] = append(outputs[endpoint.owner][endpoint.item], endpoint)
		remaining[endpoint] = endpoint.rate
	}
	moves := []flyLeftoverMove{}
	for _, edge := range b.flowGraph.Edges {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送副产物份额规划已取消或超时"}
		}
		owner, ok := owners[edge.SourceNodeID]
		if !ok {
			continue
		}
		uses, err := allocateFlyEndpoints(outputs[owner][edge.ItemID], remaining, edge.RatePerMinute)
		if err != nil {
			return nil, transportAllocationError(edge, edge.RatePerMinute, "配送回收无法分配实际生产输出")
		}
		if edge.FlowKind == "leftover" {
			for _, use := range uses {
				moves = append(moves, flyLeftoverMove{endpoint: use.endpoint, edge: edge, rate: use.rate})
			}
		}
	}
	return moves, nil
}

// flyLeftoverBoundaryCheck 职责：核实剩余边只绑定回收专属实体并解释原料区归集；使用：普通配送诊断提交之前；边界：缺少实物证据硬失败，不宣称配送机队或库存吞吐。
func (b *builder) flyLeftoverBoundaryCheck(mode int, routes []*transportRoute) (*LogisticsCheck, string, error) {
	if mode != 2 {
		return nil, "", nil
	}
	byDelivery := map[*placed]*flyTransportEndpoint{}
	for _, endpoint := range b.flyEndpoints {
		byDelivery[endpoint.delivery] = endpoint
	}
	count, matchedCount, rate := 0, 0, 0.
	for _, route := range routes {
		if route.edge.FlowKind != "leftover" {
			continue
		}
		endpoint := byDelivery[route.boundary]
		if endpoint == nil {
			return nil, "", transportAllocationError(route.edge, route.rate, "配送副产物没有独立的原料区回收箱")
		}
		var rawAnchor *placed
		anchorMatched := false
		for _, block := range b.flyLayoutBlocks {
			for _, p := range b.all[block.begin:block.end] {
				if endpoint != nil && p == endpoint.box {
					rawAnchor, anchorMatched = b.flyLocalInputAnchorForItem(b.all[block.begin:block.end], endpoint.item, endpoint.owner)
					break
				}
			}
		}
		if len(b.flyLayoutBlocks) == 0 {
			rawAnchor, anchorMatched = b.flyLocalInputAnchorForItem(b.all, endpoint.item, endpoint.owner)
		}
		if endpoint.flowEdgeID != route.edge.ID || endpoint.leftoverAnchor == nil || endpoint.leftoverAnchor != rawAnchor || endpoint.leftoverAnchorMatched != anchorMatched || endpoint.input || endpoint.leftoverAnchor == endpoint.box || endpoint.leftoverAnchor.record.ItemID != 2101 {
			return nil, "", transportAllocationError(route.edge, route.rate, "配送副产物没有独立的原料区回收箱")
		}
		dx, dy := endpoint.box.a.x-endpoint.leftoverAnchor.a.x, endpoint.box.a.y-endpoint.leftoverAnchor.a.y
		if math.Abs(dx) > 1e-6 || dy < 4 || len(endpoint.belts) < 2 {
			return nil, "", transportAllocationError(route.edge, route.rate, "配送副产物未沿真实带导至所标原料接口区")
		}
		count++
		if anchorMatched {
			matchedCount++
		}
		rate += route.rate
	}
	if count == 0 {
		return nil, "", nil
	}
	message := fmt.Sprintf("%d 条剩余副产物 lane（合计 %.3f/min，其中 %d 条靠近同物品真实原料箱，%d 条仅归集到本组原料区）已沿独立实体带导至开放回收箱；回收箱不连接原料供给箱，闭合配送块保持完整，规范剩余边及外供账不变。请单独接入回收/储存或其它消费设施；配送机队和箱体库存的运行时吞吐仍未验证", count, rate, matchedCount, count-matchedCount)
	return &LogisticsCheck{RuleID: "leftover-input-boundary", Status: "passed", Message: message}, message, nil
}
