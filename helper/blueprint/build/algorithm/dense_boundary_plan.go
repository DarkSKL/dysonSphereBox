package build

// 文件职责：在生产实体的实际纬向范围内联合安排密铺输入与输出，最多使用三层入口。
// 使用：mode1 边界实体创建前；边界：不扩大工厂高度、不合并不同物料、不改变流量或实际带宽。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// denseBoundaryLayout 职责：冻结共同入口边、直入交接点和交付组位置；使用：输入创建、输出创建和边界避碰共用；边界：仅属于当前装配候选，组身份取真实路线。
type denseBoundaryLayout struct {
	handoffX, maximumX float64
	// levelLimitX 职责：划定三层平直接口末端；使用：接口之后允许在模型盒外升降接入厂内线路；边界：整个 maximumX 以西仍禁止独立带段交叉。
	levelLimitX                  float64
	minimumX, minimumY, maximumY float64
	outputs                      map[*transportRoute]vector
}

// planDenseBoundaries 职责：先保留就近地面输出，再按消费者纬向位置匹配三层输入槽；使用：原料位置草案完成后；边界：不使用已拉长的旧入口 y 作为目标，容量不足明确失败。
func (b *builder) planDenseBoundaries(ctx context.Context, positions []vector, low, high vector) ([]vector, error) {
	groups, err := b.planMode1DeliveryGroups(ctx)
	if err != nil {
		return nil, err
	}
	bottom, top := math.Ceil(low.y), math.Floor(high.y)
	if b.polarFrame != nil {
		bottom, top = max(0, bottom), min(float64(b.polarFrame.height-1), top)
	}
	if top < bottom {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "生产实体范围内没有独立的输入输出槽"}
	}
	pitch := 2.
	rows := int((top-bottom)/pitch) + 1
	if !b.setting.EnableFlowMonitor && 3*rows-len(groups) < len(positions) {
		pitch = 1
		rows = int(top-bottom) + 1
	}
	if len(groups) > rows || len(positions) > 3*rows-len(groups) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("当前工厂高度最多容纳%d条独立原料带（三层），实际需要%d条；请提高带等级或使用物流设施", max(0, 3*rows-len(groups)), len(positions))}
	}
	left := positions[0].x
	for _, p := range positions {
		left = min(left, p.x)
	}
	// 实体原点外仍有模型盒；在盒外结束三层接口，留下厂内接驳升降空间，不能将低层接口硬贴到机器侧面。
	handoff := math.Floor(low.x) - 4
	left = min(left, handoff-3)
	plan := &denseBoundaryLayout{handoffX: handoff, levelLimitX: handoff + .5, maximumX: math.Floor(low.x) - .5, minimumX: left, minimumY: bottom, maximumY: top, outputs: make(map[*transportRoute]vector)}
	reserved := make(map[int]bool)
	for _, group := range groups {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		y := make([]float64, 0, len(group.members))
		for _, member := range group.members {
			if member.start == nil {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "交付组缺少生产接带，无法安排边界"}
			}
			y = append(y, member.start.a.y)
		}
		sort.Float64s(y)
		desired, chosen, distance := y[len(y)/2], -1, math.Inf(1)
		for row := 0; row < rows; row++ {
			// 一格紧凑输入仍为地面交付保留两格间隔，不能破坏输出隔离契约。
			if pitch == 1 && row%2 != 0 {
				continue
			}
			if cost := math.Abs(bottom + float64(row)*pitch - desired); !reserved[row] && cost < distance {
				chosen, distance = row, cost
			}
		}
		if chosen < 0 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "生产实体范围内缺少独立地面交付槽"}
		}
		reserved[chosen] = true
		if pitch == 1 {
			if chosen > 0 {
				reserved[chosen-1] = true
			}
			if chosen+1 < rows {
				reserved[chosen+1] = true
			}
		}
		plan.outputs[group.members[0]] = vector{left, bottom + float64(chosen)*pitch, 0}
	}
	desired := append([]vector(nil), positions...)
	index := 0
	for _, route := range b.transportRoutes {
		if route.from != nil {
			continue
		}
		if index >= len(desired) {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "原料接口草案与真实路线数量不一致"}
		}
		desired[index].x = left
		if route.end != nil {
			desired[index].y = min(top, max(bottom, route.end.a.y))
		}
		index++
	}
	if index != len(desired) {
		return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "原料接口草案与真实路线数量不一致"}
	}
	result, err := matchDenseBoundarySlots(ctx, desired, bottom, pitch, rows, reserved)
	if err != nil {
		return nil, err
	}
	if err := b.compactDenseGroundBoundary(ctx, plan, result, low.x); err != nil {
		return nil, err
	}
	b.denseBoundary = plan
	return result, nil
}

// compactDenseGroundBoundary 职责：为可在地面接入的接口去掉大型分层接入预留；使用：联合槽位已确定、端口尚未创建；边界：先按真实静态盒及节点核验所有新短带和交付尾带，不可行时保持原三层方案。
func (b *builder) compactDenseGroundBoundary(ctx context.Context, plan *denseBoundaryLayout, inputs []vector, productionLeft float64) error {
	for _, p := range inputs {
		if p.z != 0 {
			return nil
		}
	}
	index, err := b.newDenseRoutePortIndex(ctx)
	if err != nil {
		return err
	}
	left := math.Floor(productionLeft) - 3
	positions := append([]vector(nil), inputs...)
	for _, p := range plan.outputs {
		positions = append(positions, p)
	}
	for _, p := range positions {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// 两格短接包含即将进入厂区的第一步；不因缩短接口而让源端埋入模型盒。
		previous := vector{left, p.y, 0}
		if !index.nodeClear(previous, nil) {
			return nil
		}
		for step := 1.; step <= 2; step++ {
			next := vector{left + step, p.y, 0}
			if !index.nodeClear(next, nil) || !index.spanClear(previous, next) {
				return nil
			}
			previous = next
		}
	}
	plan.minimumX, plan.handoffX, plan.levelLimitX = left, left+1, left+1.5
	for i := range inputs {
		inputs[i].x = left
	}
	for group, p := range plan.outputs {
		p.x = left
		plan.outputs[group] = p
	}
	return nil
}

// matchDenseBoundarySlots 职责：以保序动态规划最小化纬向偏移和升层代价；使用：输入端一维有序槽位匹配；边界：每个槽只使用一次，保留路线原顺序，最多三层且不越过地面输出预留。
func matchDenseBoundarySlots(ctx context.Context, desired []vector, bottom, pitch float64, rows int, reserved map[int]bool) ([]vector, error) {
	order := make([]int, len(desired))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return desired[order[i]].y < desired[order[j]].y })
	slots := make([]vector, 0, rows*3)
	for row := 0; row < rows; row++ {
		for level := 0; level < 3; level++ {
			if level == 0 && reserved[row] {
				continue
			}
			slots = append(slots, vector{0, bottom + float64(row)*pitch, float64(level * 2)})
		}
	}
	if len(desired) > len(slots) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "三层独立原料槽容量不足"}
	}
	width := len(slots) + 1
	take := make([]bool, (len(desired)+1)*width)
	previous, current := make([]float64, width), make([]float64, width)
	for i, id := range order {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		current[0] = math.Inf(1)
		for j, slot := range slots {
			match := previous[j] + math.Abs(desired[id].y-slot.y) + slot.z*2
			current[j+1] = current[j]
			if match < current[j+1] {
				current[j+1] = match
				take[(i+1)*width+j+1] = true
			}
		}
		previous, current = current, previous
	}
	if math.IsInf(previous[len(slots)], 1) || math.IsNaN(previous[len(slots)]) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "无法为全部原料分配独立三层接口"}
	}
	result := make([]vector, len(desired))
	for i, j := len(desired), len(slots); i > 0 && j > 0; j-- {
		if take[i*width+j] {
			id := order[i-1]
			result[id] = slots[j-1]
			result[id].x = desired[id].x
			i--
		}
	}
	return result, nil
}
