package build

// 文件职责：证明回流优先门的单去向净产出可以直接出货。
// 使用：密铺省去重复过滤臂后的终检；边界：多去向仍需独立分流，不能使用优先回流口作为净出口。

import "math"

// compactRecycleOutputX 职责：按实际独立出货结构缩短优先门前的预留；使用：单一回流、零补料的密铺行；边界：任一产物需要分流或自产供剂时保留原过滤区预算，门与其它端点至少相隔三格。
func (b *builder) compactRecycleOutputX(row *transportRow, fallback float64) float64 {
	if !b.denseSpatial || b.adapter == nil || b.adapter.density != densityDense {
		return fallback
	}
	left := math.Inf(1)
	for _, item := range row.recipe.Results {
		count := 0
		for _, route := range row.outputs {
			if route.edge.ItemID == item {
				count++
			}
		}
		if count > 1 || productionSprayItem(row.info) == item {
			return fallback
		}
		belts := row.line.outputForItem(item)
		if len(belts) == 0 {
			return fallback
		}
		left = math.Min(left, belts[0].a.x)
	}
	for _, input := range row.allInputEndpoints() {
		left = math.Min(left, input.a.x)
	}
	return left - 3
}

// validateRecycleDirectOutput 职责：复核净出口、唯一消费份额和共同毛流记录；使用：无二次过滤的回流产物；边界：完整回流和机器过滤臂仍由公共终检逐项证明。
func (b *builder) validateRecycleDirectOutput(route *transportRoute, recycle *machineRecycle) error {
	fail := func(reason string) error {
		return transportAllocationError(route.edge, route.rate, "回流净产出直连无效："+reason)
	}
	if b.adapter == nil || b.adapter.density != densityDense || recycle.row != route.from || recycle.netOutput <= 1e-8 || len(recycle.surplus) == 0 || len(route.path) == 0 {
		return fail("缺少真实净产物出口")
	}
	count := 0
	for _, other := range route.from.outputs {
		if other.edge.ItemID == recycle.item {
			count++
			if other != route {
				return fail("同一净产物存在其它去向")
			}
		}
	}
	terminal := recycle.surplus[len(recycle.surplus)-1]
	if count != 1 || route.start != terminal || route.path[0] != terminal || recycle.surplus[0].input != recycle.outputGate || recycle.surplus[0].record.InputFromSlot == recycle.outputSlot {
		return fail("未从独立净出口直接出货")
	}
	for i := 1; i < len(recycle.surplus); i++ {
		if recycle.surplus[i-1].output != recycle.surplus[i] {
			return fail("净产物接头中断")
		}
	}
	expected := append(append(append([]*placed{}, route.from.line.outputForItem(recycle.item)...), recycle.source...), recycle.surplus[:len(recycle.surplus)-1]...)
	// 配送箱等边界设施可在末尾追加自己的共享短带；本行毛流前缀必须完整保留，边界尾链由各模式终检负责。
	if len(expected) > len(route.sharedPath) {
		return fail("共同毛流记录不完整")
	}
	for i, part := range expected {
		if route.sharedPath[i] != part {
			return fail("共同毛流带被替换")
		}
	}
	return nil
}
