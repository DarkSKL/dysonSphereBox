package build

// 文件职责：删去直连机器行中超出实际分拣器接点的无用带尾，并就近安排工厂西侧原料入口。
// 使用：mode1 全局寻路前安排入口、寻路后裁剪带尾；边界：不删机器、分拣器或有效带链，不改变流量、等级及外部入口数量。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// trimConnectedTails 职责：在完整实体路由形成后裁除各行闲置延伸；使用：直连带候选收尾；边界：不因局部宽度或占位变化改变已验证的装箱和路径选择，最终规则仍须重新执行。
func (b *builder) trimConnectedTails() error {
	seen := map[*line]bool{}
	lines := []*line{}
	required, removed := map[*placed]bool{}, map[*placed]bool{}
	for _, entity := range b.all {
		required[entity.input] = true
		if !surfaceIsBelt(entity.record.ItemID) {
			required[entity.input], required[entity.output] = true, true
		}
	}
	for _, route := range b.transportRoutes {
		for _, endpoint := range []*placed{route.start, route.end, route.boundary} {
			required[endpoint] = true
		}
		for _, entity := range route.path {
			required[entity] = true
		}
		for _, row := range []*transportRow{route.from, route.to} {
			if row == nil || row.line == nil || seen[row.line] {
				continue
			}
			seen[row.line] = true
			lines = append(lines, row.line)
		}
	}
	for _, recycle := range b.machineRecycles {
		for _, entities := range [][]*placed{recycle.route.path, recycle.source, recycle.intake, recycle.surplus, {recycle.fresh, recycle.returned}} {
			for _, entity := range entities {
				required[entity] = true
			}
		}
		if !seen[recycle.row.line] {
			seen[recycle.row.line] = true
			lines = append(lines, recycle.row.line)
		}
	}
	trim := func(l *line, belts []*placed) ([]*placed, error) {
		last := -1
		for index, belt := range belts {
			if required[belt] {
				last = index
			}
		}
		if last < 0 {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("配方 %d 的物料带没有实际机器分拣器接点", l.recipe.ID)}
		}
		for _, belt := range belts[last+1:] {
			removed[belt] = true
		}
		return belts[:last+1], nil
	}
	for _, l := range lines {
		if err := b.checkGeneration("直连带尾裁剪"); err != nil {
			return err
		}
		for index, belts := range l.inputs {
			row, err := trim(l, belts)
			if err != nil {
				return err
			}
			l.inputs[index] = row
		}
		for index, belts := range l.outputRows() {
			output, err := trim(l, belts)
			if err != nil {
				return err
			}
			if len(l.outputs) > 0 {
				l.outputs[index] = output
			}
			if index == 0 {
				l.output = output
			}
		}
	}
	for _, route := range b.transportRoutes {
		if route.from == nil || len(route.filters) == 0 {
			continue
		}
		lastForItem := true
		found := false
		for _, sibling := range route.from.outputs {
			if sibling == route {
				found = true
			} else if found && sibling.edge.ItemID == route.edge.ItemID {
				lastForItem = false
				break
			}
		}
		if !lastForItem {
			continue
		}
		last := route.filters[len(route.filters)-1].input
		for tail := last.output; tail != nil; tail = tail.output {
			if !surfaceIsBelt(tail.record.ItemID) || required[tail] || removed[tail] {
				return transportAllocationError(route.edge, route.rate, "过滤干线末端仍被真实路线或设施引用，不能裁剪")
			}
			removed[tail] = true
		}
	}
	for _, entity := range b.all {
		if !removed[entity] && removed[entity.output] {
			entity.output = nil
			entity.record.OutputToSlot = 0
		}
	}
	keep := func(entities []*placed) []*placed {
		kept := make([]*placed, 0, len(entities))
		for _, entity := range entities {
			if !removed[entity] {
				kept = append(kept, entity)
			}
		}
		return kept
	}
	// 行实体仍可能引用 builder 旧底层数组；各自重建切片，避免压缩时覆盖尚未处理的其它行。
	for _, l := range lines {
		l.all = keep(l.all)
	}
	for _, route := range b.transportRoutes {
		route.sharedPath = keep(route.sharedPath)
	}
	b.all = keep(b.all)
	return nil
}

// transportInputPositions 职责：为每条外供 lane 安排同一西边界上的独立入口；使用：routeTransport 创建原料端点；边界：就近方案只调整纬向位置，至少隔一格且不将接口藏入机器区，legacy 用于环路及回归对照。
func transportInputPositions(routes []*transportRoute, minimum, maximum vector, nearby bool) []vector {
	// input 职责：缓存原路线序和消费者纬向位置；使用：稳定分配有限边界槽；边界：排序不改变实际寻路顺序。
	type input struct {
		index int
		y     int
	}
	inputs := []input{}
	x, low := math.Floor(minimum.x)-5, int(math.Floor(minimum.y))
	for _, route := range routes {
		if route.from != nil {
			continue
		}
		y := low + len(inputs)*2
		if nearby && route.end != nil {
			y = int(math.Round(route.end.a.y))
		}
		inputs = append(inputs, input{index: len(inputs), y: y})
	}
	positions := make([]vector, len(inputs))
	if !nearby {
		for _, entry := range inputs {
			positions[entry.index] = vector{x, float64(entry.y), 0}
		}
		return positions
	}
	high := max(int(math.Ceil(maximum.y)), low+2*(len(inputs)-1))
	sort.SliceStable(inputs, func(i, j int) bool { return inputs[i].y < inputs[j].y })
	previous := low - 2
	for index, entry := range inputs {
		// 保持消费者纬向顺序，并提前给剩余入口留下两格槽位；不能为一次就近放置把接口挤到工厂包络外。
		chosen := max(previous+2, min(high-2*(len(inputs)-1-index), entry.y))
		positions[entry.index] = vector{x, float64(chosen), 0}
		previous = chosen
	}
	return positions
}
