package build

// 文件职责：将同物品多去向的过滤支路交替放到共享主带两侧，减少单侧隔离长干线。
// 使用：独立密铺直连与塔黑盒候选；边界：保留全部净流去向、真实过滤臂及共享带容量，不用普通带T分流。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// denseShortPairedOutputsKey 职责：显式启用二、三去向行的紧凑双侧过滤；使用：独立密铺候选context；边界：缺省关闭，不改变既有四去向候选或普通布局。
type denseShortPairedOutputsKey struct{}

// usesDensePairedOutputs 职责：统一选择旧多去向与新增短分支双侧几何；使用：局部出货装配；边界：新增范围仅二、三去向、单产物且无机器回流，仍使用原所选等级和完整过滤数量。
func (b *builder) usesDensePairedOutputs(row *transportRow) bool {
	if b.densePairedOutputs && densePairedOutputEligible(row) {
		return true
	}
	if !b.denseSpatial || b.generationCtx == nil || b.adapter != nil && b.adapter.density != densityDense {
		return false
	}
	enabled, _ := b.generationCtx.Value(denseShortPairedOutputsKey{}).(bool)
	if !enabled || row == nil || len(row.outputs) < 2 || len(row.outputs) > 3 {
		return false
	}
	return densePairedOutputRangeEligible(row, 2)
}

// densePairedOutputEligible 职责：限制双侧过滤到有收益且无需原优先回收门的单产物组；使用：计划与实际装配同源判断；边界：少于四个去向不新增等价候选。
func densePairedOutputEligible(row *transportRow) bool {
	return densePairedOutputRangeEligible(row, 4)
}

// densePairedOutputRangeEligible 职责：共用单产物、去向与回流资格；使用：原四路及独立二三路候选；边界：只校验拓扑，实际臂容量和碰撞仍由装配及终检证明。
func densePairedOutputRangeEligible(row *transportRow, minimum int) bool {
	if row == nil || len(row.recipe.Results) != 1 || len(row.outputs) < minimum || recipeHasMachineRecycle(row.recipe) {
		return false
	}
	for _, route := range row.outputs {
		if route == nil || route.edge.ItemID != row.recipe.Results[0] {
			return false
		}
	}
	return true
}

// hasDensePairedOutputGroups 职责：先核对真实容量行是否存在可压缩过滤组；使用：仅在几何确会变化时增加四向候选；边界：不凭总目标数猜测分支数。
func (b *builder) hasDensePairedOutputGroups(ctx context.Context, req Request, dataset *quant.Dataset) (bool, error) {
	probe, err := b.newCandidateBuilder()
	if err != nil {
		return false, err
	}
	probe.denseSpatial, probe.denseMaxSpan = true, 0
	probe.flowGraph = b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	rows, err := probe.planTransportRows(ctx, req, dataset)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if densePairedOutputEligible(row) {
			return true, nil
		}
	}
	return false, nil
}

// attachDensePairedOutputs 职责：用不同主带节点交替向两侧真实抓取并保留有限出口间距；使用：单产物多去向独立候选；边界：同节点双反向臂不采用，实际臂盒冲突直接拒绝本候选。
func (b *builder) attachDensePairedOutputs(row *transportRow, belts []*placed, routes []*transportRoute, cursor float64) (float64, error) {
	begin := len(b.all)
	trunk := belts[0]
	shared := append([]*placed{}, belts...)
	for _, input := range row.allInputEndpoints() {
		cursor = math.Min(cursor, input.a.x-2)
	}
	low, _ := bounds(row.line.all)
	cursor = math.Min(cursor, math.Floor(low.x)-2)
	appendMain := func(x float64) *placed {
		node := b.make(2000+b.setting.BeltLv, x, trunk.a.y, trunk.a.z)
		connect(trunk, node)
		trunk = node
		shared = append(shared, node)
		return node
	}
	for trunk.a.x-1 > cursor {
		appendMain(trunk.a.x - 1)
	}
	direction := 1.
	if trunk.a.y < row.line.machines[0].a.y {
		direction = -1
	}
	capacity := inserterCapacityPerMinute(b.setting.InserterLv, 1)
	if !finitePositive(capacity) {
		return cursor, &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: "双侧过滤缺少所选分拣器已验证能力"}
	}
	armCount := 0
	for index, route := range routes {
		side := direction
		if index%2 == 1 {
			side = -side
		}
		count := max(1, int(math.Ceil((route.rate-1e-8)/capacity)))
		branch := []*placed{}
		for armIndex := 0; armIndex < count; armIndex++ {
			main := appendMain(cursor)
			pickup := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y+side, trunk.a.z)
			if len(branch) > 0 {
				connect(branch[len(branch)-1], pickup)
			}
			branch = append(branch, pickup)
			arm, err := b.beltTransfer(main, pickup, b.setting.InserterLv, route.edge.ItemID)
			if err != nil {
				return cursor, err
			}
			route.filters = append(route.filters, arm)
			armCount++
			cursor--
		}
		end := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y+side, trunk.a.z)
		connect(branch[len(branch)-1], end)
		branch = append(branch, end)
		route.start, route.path = end, branch
		route.sharedPath = append([]*placed{}, shared...)
		end.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		if route.to == nil {
			route.boundary = end
		}
		// 两路使用相邻不同抓取点；每对子后留一个真实主带格，使同侧出口前仍有空格，也保留回料臂可选位置。
		if index%2 == 1 || index == len(routes)-1 {
			appendMain(cursor)
			cursor--
		}
	}
	local := &builder{all: append(append([]*placed{}, row.line.all...), b.all[begin:]...)}
	records, _, _, err := local.records()
	if err != nil {
		return cursor, err
	}
	ctx := b.generationCtx
	if ctx == nil {
		ctx = context.Background()
	}
	boxes, err := newPlanarRuleEncoding().surfaceInserterBoxes(ctx, records)
	if err != nil {
		return cursor, err
	}
	first, second, err := placement.FirstConflict(ctx, boxes)
	if err != nil {
		return cursor, err
	}
	if first >= 0 {
		return cursor, &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("双侧过滤真实分拣器盒冲突：%d/%d", first, second)}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-paired-filtered-outputs", Status: "passed", Message: fmt.Sprintf("产线%d配方%d的%d个净流去向使用%d根所选Mk.%d过滤臂，主带抓取点互不重复、支路交替两侧；每两路保留一个真实主带出口间距，全部共享带仍按净流及自产剂回料毛流累计", row.owner, row.info.RecipeID, len(routes), armCount, b.setting.InserterLv)})
		if len(routes) >= 2 && len(routes) <= 3 {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-short-paired-filtered-outputs", Status: "passed", Message: fmt.Sprintf("产线%d配方%d的%d个净流去向复用真实双侧过滤，保留%d根所选Mk.%d臂、各支路流量和完整带链；仅独立候选启用，不以删臂或升级设备缩短接头", row.owner, row.info.RecipeID, len(routes), armCount, b.setting.InserterLv)})
		}
	}
	return cursor, nil
}
