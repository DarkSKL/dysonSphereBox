package build

// 文件职责：将多层原料边界中的独立地面输出接入实际主路，避免输出目标留在低层孤岛。
// 使用：mode1交付端点创建后、全图路由之前；边界：保留原地面交付头、监测宿主与份额，以真实短带替代跨合流身份的虚拟预留。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// prepareDenseOutputCorridors 职责：先为所有输出建立可独立校验的地面、升层和主路短带。
// 使用：原料错列升层已冻结时；边界：单层边界沿用原路由，多层错开半格越过原料8/10层后接入主路，不改变公共寻路预算。
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
	// 在三层平直接口之后立即升层，避免继续向东进入某些生产行外伸的静态模型盒。
	riserX := math.Floor(b.denseBoundary.levelLimitX) + 1
	for _, g := range groups {
		endpoint := g.head
		if g.entry != nil {
			endpoint = g.entry
		}
		points := []vector{}
		for x := endpoint.a.x + 1; x <= riserX+1e-6; x++ {
			points = append(points, vector{x, endpoint.a.y, 0})
		}
		// 输出与上层原料可以共享整数纬向槽；先在模型盒外错开半格，避免升层穿过其8/10层固定逃逸段。
		riserY := math.Max(b.denseBoundary.minimumY, endpoint.a.y-.5)
		points = append(points, vector{riserX, endpoint.a.y, 1})
		if riserY != endpoint.a.y {
			points = append(points, vector{riserX, riserY, 1})
		}
		for z := 2.; z <= 4*denseBoundaryLevels; z++ {
			points = append(points, vector{riserX, riserY, z})
		}
		for x := riserX + 1; x <= math.Ceil(b.denseBoundary.maximumX); x++ {
			points = append(points, vector{x, riserY, 4 * denseBoundaryLevels})
		}
		previous := endpoint
		for _, point := range points {
			if err := ctx.Err(); err != nil {
				return err
			}
			nodeClear, spanClear := index.nodeClear(point, nil), index.spanClear(previous.a, point)
			if !nodeClear || !spanClear {
				return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("独立交付物品%d固定接入短带 %v→%v 缺少真实净空（节点%t、带段%t）", g.item, previous.a, point, nodeClear, spanClear)}
			}
			next := b.make(2000+b.setting.BeltLv, point.x, point.y, point.z)
			connect(next, previous)
			index.addNode(next)
			previous = next
		}
		g.approachSteps = len(points)
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
