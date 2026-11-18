package build

// 文件职责：将多层原料边界中的独立地面输出接入实际主路，避免输出目标留在低层孤岛。
// 使用：mode1交付端点创建后、全图路由之前；边界：保留原地面交付头、监测宿主与份额，以真实短带替代跨合流身份的虚拟预留。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// prepareDenseOutputCorridors 将多层边界的地面交付短带延伸到允许转向和升降的交接列。
// 后续路线由公共寻路器按实际占位选择高度，交付口和监测宿主保持原位。
func (b *builder) prepareDenseOutputCorridors(ctx context.Context, groups []*mode1DeliveryGroup) error {
	if b.denseBoundary == nil || !b.denseBoundary.staggerInputs {
		return nil
	}
	index, err := b.newDenseRoutePortIndex(ctx)
	if err != nil {
		return err
	}
	index.boundary = b.denseBoundary
	for _, p := range b.all {
		index.addBeltSpan(p)
	}
	handoff := math.Floor(b.denseBoundary.levelLimitX) + 1
	for _, g := range groups {
		endpoint := g.head
		if g.entry != nil {
			endpoint = g.entry
		}
		previous := endpoint
		for x := endpoint.a.x + 1; x <= handoff; x++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			point := vector{x, endpoint.a.y, endpoint.a.z}
			nodeClear, spanClear := index.nodeClear(point, nil), index.spanClear(previous.a, point)
			if !nodeClear || !spanClear {
				return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("独立交付物品%d固定接入短带 %v→%v 缺少真实净空（节点%t、带段%t）", g.item, previous.a, point, nodeClear, spanClear)}
			}
			next := b.make(2000+b.setting.BeltLv, point.x, point.y, point.z)
			connect(next, previous)
			index.addNode(next)
			previous = next
			g.approachSteps++
		}
		for _, r := range g.members {
			r.end = previous
		}
	}
	return nil
}

// mode1DeliveryApproach 职责：按冻结步数核实路由接入头仍连续指向预定交付头；使用：交付提交；边界：只沿真实output指针，环路、断带、改端点及不同成员偷换接入头均拒绝。
func mode1DeliveryApproach(g *mode1DeliveryGroup, source, target *placed) ([]*placed, error) {
	path := []*placed{}
	seen := map[*placed]bool{}
	current := source
	for step := 0; step < g.approachSteps; step++ {
		if current == nil || seen[current] || !surfaceIsBelt(current.record.ItemID) || current.output == nil {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "固定交付接入短带断开或成环"}
		}
		seen[current] = true
		current = current.output
		path = append(path, current)
	}
	if current != target {
		return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "固定交付接入短带未抵达预定交付头"}
	}
	return path, nil
}
