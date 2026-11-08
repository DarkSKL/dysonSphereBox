package build

// 文件职责：将直连模式的剩余副产物沿真实独立带送回原料接口区，并保留净流边界身份。
// 使用：全图寻路前建立西侧出货口，编码前核验完整带链并提示玩家取货。
// 边界：这里的“导回”是空间归集，不把副产物并入原料带，不伪造循环消费或削减外供需求。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// transportLeftoverPositions 职责：在原料入口所在西边界为剩余 lane 分配独立地面出口；使用：直连带路由准备；边界：同物品优先靠近对应入口，无对应物品时靠近最近原料口，所有边界端点至少相隔两格。
func transportLeftoverPositions(ctx context.Context, routes []*transportRoute, inputs []vector, minimum vector) ([]vector, error) {
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物原料区出口规划已取消或超时"}
	}
	x := math.Floor(minimum.x) - 5
	if len(inputs) > 0 {
		x = inputs[0].x
	}
	used := map[int]bool{}
	inputItems := make([]int, 0, len(inputs))
	for _, route := range routes {
		if route.from == nil {
			inputItems = append(inputItems, route.edge.ItemID)
		}
	}
	for _, input := range inputs {
		x = math.Min(x, input.x)
		used[int(math.Round(input.y))] = true
	}
	positions := []vector{}
	for _, route := range routes {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物原料区出口规划已取消或超时"}
		}
		if route.from == nil || route.to != nil || route.edge.FlowKind != "leftover" {
			continue
		}
		originY := minimum.y
		if route.start != nil {
			originY = route.start.a.y
		}
		desired := int(math.Round(originY))
		best, sameItem := math.Inf(1), false
		for i, input := range inputs {
			matches := i < len(inputItems) && inputItems[i] == route.edge.ItemID
			distance := math.Abs(input.y - originY)
			if matches && !sameItem || matches == sameItem && distance < best {
				desired, best, sameItem = int(math.Round(input.y)), distance, matches
			}
		}
		available := func(y int) bool { return !used[y-1] && !used[y] && !used[y+1] }
		chosen := desired
		for offset := 0; ; offset++ {
			// 同距时取较高纬槽，规则稳定且不会因 map 遍历改变端口顺序。
			if available(desired + offset) {
				chosen = desired + offset
				break
			}
			if available(desired - offset) {
				chosen = desired - offset
				break
			}
		}
		used[chosen] = true
		positions = append(positions, vector{x, float64(chosen), 0})
	}
	return positions, nil
}

// prepareLeftoverBoundaries 职责：建立与原料区同侧、但方向相反的真实副产物出口；使用：mode1 全图占位前；边界：仅创建端点，随后必须完成寻路及最终连接/容量核验，内部总线策略不在此改写。
func (b *builder) prepareLeftoverBoundaries(ctx context.Context, inputPositions []vector) error {
	if b.busFabric != nil || b.directBoundaryUnits != nil {
		return nil
	}
	minimum, _ := bounds(b.all)
	positions, err := transportLeftoverPositions(ctx, b.transportRoutes, inputPositions, minimum)
	if err != nil {
		return err
	}
	index := 0
	for _, route := range b.transportRoutes {
		if route.from == nil || route.to != nil || route.edge.FlowKind != "leftover" {
			continue
		}
		if err := b.checkGeneration("副产物原料区出口规划"); err != nil {
			return err
		}
		if route.start == nil || index >= len(positions) {
			return transportAllocationError(route.edge, route.rate, "副产物缺少真实出货起点或独立原料区出口")
		}
		position := positions[index]
		endpoint := b.make(2000+b.setting.BeltLv, position.x, position.y, position.z)
		endpoint.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		route.end, route.boundary = endpoint, endpoint
		index++
	}
	return nil
}

// validateLeftoverBoundaries 职责：核实副产物导回出口仍是可取货的独立终点并解释用途；使用：mode1 最终物流核验；边界：不因空间邻近把剩余边冒充原料输入，也不声称游戏内持续产能已经验收。
func (b *builder) validateLeftoverBoundaries() error {
	count, rate := 0, 0.
	inputs := []*placed{}
	for _, route := range b.transportRoutes {
		if route.from == nil && route.boundary != nil {
			inputs = append(inputs, route.boundary)
		}
	}
	for _, route := range b.transportRoutes {
		if route.edge.FlowKind != "leftover" {
			continue
		}
		if route.from == nil || route.to != nil || route.end == nil || route.end != route.boundary || route.end.output != nil || len(route.path) < 2 || route.path[len(route.path)-1] != route.end {
			return transportAllocationError(route.edge, route.rate, "副产物未通过完整独立带链导至原料区外部出口")
		}
		if route.end == route.start || len(route.end.record.Parameters) < 2 || int(route.end.record.Parameters[0]) != route.edge.ItemID {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("副产物 %d 的原料区出口缺少正确物品标记", route.edge.ItemID)}
		}
		previous := route.path[len(route.path)-2]
		step := 1.
		if b.nativeLatitudeScaled {
			step = b.nativeLongitudeScale
		}
		if previous == nil || previous.output != route.end || math.Abs(previous.a.x-route.end.a.x-step) > 1e-6 || previous.a.y != route.end.a.y || previous.a.z != route.end.a.z || route.end.a.z != 0 {
			return transportAllocationError(route.edge, route.rate, "副产物原料区出口必须沿真实地面带向西出货")
		}
		for _, input := range inputs {
			if math.Abs(input.a.x-route.end.a.x) > 1e-6 || math.Abs(input.a.y-route.end.a.y) < 2-1e-6 || input == route.end {
				return transportAllocationError(route.edge, route.rate, "副产物出口没有独立留在原料入口边界")
			}
		}
		count++
		rate += route.rate
	}
	if count > 0 {
		message := fmt.Sprintf("%d 条剩余副产物带（合计 %.3f/min）已沿独立实体路线导至原料接口区；带上保留物品与流量标记，方向为出货。请单独接入回收/储存或其它消费设施，不能把不同物品混入原料带；原料需求和剩余流量账不变", count, rate)
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "leftover-input-boundary", Status: "passed", Message: message})
		b.warnings = append(b.warnings, message)
	}
	return nil
}
