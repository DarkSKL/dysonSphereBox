package build

// 文件职责：逐段核验喷涂兼容共享主干与真实末端过滤支路。
// 使用：最终物流出口；边界：分别检查上游聚合与下游过滤能力，不把串联两组臂的能力相加，不改变原喷涂/供剂角色。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
)

// validateDenseMaterialTapNetwork 职责：核验真实主干、配置毛需求、独立过滤支口及逐份额账；使用：含喷涂的共享原料组；边界：原喷涂机及每个供剂链仍由共同最终逐机检查证明。
func (b *builder) validateDenseMaterialTapNetwork(group *denseMaterialNetwork, alive map[*placed]bool) (float64, error) {
	fail := func(reason string) (float64, error) {
		return 0, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享原料主干不完整：" + reason}
	}
	plan := b.denseMaterialNetworks
	if len(group.rows) < 2 || len(group.taps) != len(group.rows) || len(group.tracks) != len(group.rows) || len(group.intake) < 4 {
		return fail("缺少真实主干或消费者取料模块")
	}
	main, err := denseMaterialDirectedPath(group.supplyHead, group.intake[len(group.intake)-1], alive)
	if err != nil || !sameDenseMaterialNodes(main, group.intake) {
		return fail("共享主干已断开")
	}
	mainNodes, allowed := map[*placed]bool{}, map[*placed]bool{}
	for _, node := range main {
		mainNodes[node], allowed[node] = true, true
	}
	configured := 0.
	for i, row := range group.rows {
		if recipeHasMachineRecycle(row.recipe) {
			return fail("不能以普通原料取料模块替代回流")
		}
		full, err := configuredRoleInputRate(row.info, row.recipe, group.item, recipeInputRole)
		if err != nil {
			return 0, err
		}
		full *= float64(row.count) / float64(row.info.MachineCount)
		if !finitePositive(full) || math.Abs(full-group.configured[i]) > math.Max(1e-6, full*1e-9) {
			return fail("配置毛输入与实际消费者不符")
		}
		configured += full
		track, err := denseMaterialDirectedPath(group.entries[i], group.exits[i], alive)
		if err != nil || !sameDenseMaterialNodes(track, group.tracks[i]) {
			return fail("原机器输入轨或喷涂前入口已改变")
		}
		tap := group.taps[i]
		if tap == nil || tap.route == nil || len(tap.route.path) == 0 {
			return fail("缺少真实过滤支路")
		}
		path, err := denseMaterialDirectedPath(tap.route.path[0], group.entries[i], alive)
		if err != nil || !sameDenseMaterialNodes(path, tap.route.path) {
			return fail("过滤支路未实际到达原消费者入口")
		}
		armCapacity := 0.
		seenArms := map[*placed]bool{}
		for _, arm := range tap.filters {
			if !alive[arm] || seenArms[arm] || !mainNodes[arm.input] || arm.output == nil || !surfaceIsBelt(arm.output.record.ItemID) || int(arm.record.ItemID) != 2010+b.setting.InserterLv || int(arm.record.FilterID) != group.item || arm.record.InputFromSlot != -1 || arm.record.OutputToSlot != -1 || !flyBeltReaches(arm.output, tap.route.start) {
				return fail("分流臂的物料、等级、端点或有向支路错误")
			}
			seenArms[arm] = true
			if err := flyInserterSpanMatchesGeometry(arm); err != nil {
				return fail(err.Error())
			}
			capacity, err := flyEntityInserterCapacity(arm)
			if err != nil {
				return 0, err
			}
			armCapacity += capacity
		}
		if armCapacity+1e-6 < full {
			return 0, &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: "共享原料取料臂不能覆盖本行配置满负荷毛需求"}
		}
	}
	if configured > b.logistics.BeltCapacityPerMinute+1e-6 {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "共享原料主干的整组配置毛需求超过所选单带"}
	}
	graph := finalMachineMaterialGraph(b.all)
	for source, members := range plan.members {
		if len(members) == 0 || plan.bindings[members[0]].group != group {
			continue
		}
		if source == nil || source.from == nil || source.from.line == nil || len(source.path) == 0 {
			return fail("缺少聚合供给的真实机器来源")
		}
		producer := source.from.line.outputForItem(group.item)
		if len(producer) == 0 || !finalMaterialReaches(graph, producer[0], source.start, group.item) {
			return fail("聚合来源没有接到实际独立机器出货带")
		}
		path, err := denseMaterialDirectedPath(source.path[0], group.supplyHead, alive)
		if err != nil || !sameDenseMaterialNodes(path, source.path) {
			return fail("来源没有通过真实带接入共享主干")
		}
		for _, node := range source.path {
			allowed[node] = true
		}
		total, capacity := 0., 0.
		for _, arm := range source.filters {
			if !alive[arm] || int(arm.record.ItemID) != 2010+b.setting.InserterLv || int(arm.record.FilterID) != group.item || !flyBeltReaches(producer[0], arm.input) || !flyBeltReaches(arm.output, source.start) {
				return fail("上游独立过滤分支失去真实来源或物料身份")
			}
			if err := flyInserterSpanMatchesGeometry(arm); err != nil {
				return fail(err.Error())
			}
			value, err := flyEntityInserterCapacity(arm)
			if err != nil {
				return 0, err
			}
			capacity += value
		}
		if len(source.filters) > 0 && capacity+1e-6 < source.rate {
			return 0, &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: "上游聚合过滤能力不足，不能与下游取料臂相加"}
		}
		for _, route := range members {
			binding := plan.bindings[route]
			if binding.target < 0 || binding.target >= len(group.rows) || route.from != source.from || route.to != group.rows[binding.target] || route.edge.ItemID != group.item || route.inputRole != recipeInputRole || !finitePositive(route.rate) {
				return fail("原规范份额的来源、消费者或用途被改写")
			}
			tap := group.taps[binding.target]
			filters := append(append([]*placed{}, source.filters...), tap.filters...)
			if route.start != tap.route.start || route.end != group.entries[binding.target] || !sameDenseMaterialNodes(route.path, tap.route.path) || !sameDenseMaterialNodes(route.filters, filters) || !sameDenseMaterialNodes(route.sharedPath, denseMaterialTapSharedPath(source, group, binding.target)) {
				return fail("规范份额遗漏来源/共享前缀或本行实际消费轨")
			}
			total += route.rate
		}
		if math.Abs(total-source.rate) > math.Max(1e-6, source.rate*1e-9) || total > b.logistics.BeltCapacityPerMinute+1e-6 {
			return fail("聚合物理流与规范份额不守恒或超带宽")
		}
	}
	for _, entity := range b.all {
		if mainNodes[entity.output] && surfaceIsBelt(entity.record.ItemID) && !allowed[entity] {
			return fail("共享主干被未登记的物料带汇入")
		}
	}
	return configured, nil
}
