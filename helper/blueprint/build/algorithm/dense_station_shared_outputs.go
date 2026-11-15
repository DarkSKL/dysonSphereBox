package build

// 文件职责：把同物品的对外交付带合流到满足满负荷能力的真实塔口。
// 使用：独立塔布局候选的库存规划、边界绑定及终检；边界：保留全部规范路线，不把合流当成普通带分流，不修改产量或设备等级。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseStationSharedOutputVariantOffset 职责：登记共享出口的独立候选；使用：与原布局组合；边界：不是公开运输模式编号。
const denseStationSharedOutputVariantOffset = 400

// denseStationSharedOutputsKey 职责：隔离当前候选的出口合流策略；使用：内部上下文；边界：原有候选保持原出口。
type denseStationSharedOutputsKey struct{}

// denseStationOutputGroup 职责：保存同物品出口的规范成员与真实塔口；使用：构造和复核共用；边界：maximum按不同生产行的配置满负荷累计，不按期望产量缩小能力需求。
type denseStationOutputGroup struct {
	item             int
	members          []*transportRoute
	representative   *transportRoute
	maximum, planned float64
	sources          map[*transportRow]bool
	head, tower      *placed
}

// denseStationOutputPlan 职责：维护规范出口到物理合流组的身份映射；使用：单个builder内部；边界：代表路线只参与塔口规划，不能替换真实生产者路线。
type denseStationOutputPlan struct {
	byRoute          map[*transportRoute]*denseStationOutputGroup
	byRepresentative map[*transportRoute]*denseStationOutputGroup
	groups           []*denseStationOutputGroup
}

// stationOutputMaximum 职责：计算来源生产行的真实满负荷输出上界；使用：合流分组和最终重新核算；边界：缺少可信能力、无效份额或超过单带能力均失败。
func (b *builder) stationOutputMaximum(route *transportRoute) (float64, error) {
	if route == nil || route.from == nil || route.to != nil || !finitePositive(route.rate) || route.edge.ItemID <= 0 || (route.edge.FlowKind != "target-output" && route.edge.FlowKind != "leftover") {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共享出口缺少真实生产者或有效对外用途"}
	}
	row, production := route.from, route.from.info
	if production.Capacity == nil || production.Capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || production.MachineCount <= 0 || row.count <= 0 || row.count > production.MachineCount {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共享出口缺少可信配置满负荷能力"}
	}
	maximum := productionItemRate(production.Capacity.GrossOutputs, route.edge.ItemID) * float64(row.count) / float64(production.MachineCount)
	if !finitePositive(maximum) || maximum+1e-6 < route.rate || maximum > b.logistics.BeltCapacityPerMinute+1e-6 {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "共享出口的生产行份额或配置满负荷输出超过所选单带能力"}
	}
	return maximum, nil
}

// planDenseStationSharedOutputs 职责：按物品和配置满负荷上界合并塔口需求；使用：创建库存实体前；边界：不改原始路线，不合并回流或自产供剂出口，无实际合并时淘汰等价候选。
// 逻辑流：1. 保留内部路线及原料入口 => 2. 对外输出按不同来源累计满负荷 => 3. 以独立代表登记塔口，保留每个真实生产者供后续寻路。
func (b *builder) planDenseStationSharedOutputs(ctx context.Context) ([]*transportRoute, error) {
	enabled, _ := ctx.Value(denseStationSharedOutputsKey{}).(bool)
	if !enabled {
		return b.transportRoutes, nil
	}
	plan := &denseStationOutputPlan{byRoute: map[*transportRoute]*denseStationOutputGroup{}, byRepresentative: map[*transportRoute]*denseStationOutputGroup{}}
	result := make([]*transportRoute, 0, len(b.transportRoutes))
	merged := false
	for _, route := range b.transportRoutes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if route.to != nil || route.from == nil || recipeHasMachineRecycle(route.from.recipe) || productionSprayItem(route.from.info) == route.edge.ItemID {
			result = append(result, route)
			continue
		}
		if plan.byRoute[route] != nil {
			return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: "共享出口重复登记同一规范份额"}
		}
		maximum, err := b.stationOutputMaximum(route)
		if err != nil {
			return nil, err
		}
		var selected *denseStationOutputGroup
		for _, group := range plan.groups {
			extra := maximum
			if group.sources[route.from] {
				extra = 0
			}
			if group.item == route.edge.ItemID && group.maximum+extra <= b.logistics.BeltCapacityPerMinute+1e-6 && group.planned+route.rate <= b.logistics.BeltCapacityPerMinute+1e-6 {
				selected = group
				break
			}
		}
		if selected == nil {
			copy := *route
			copy.rate = 0
			selected = &denseStationOutputGroup{item: route.edge.ItemID, representative: &copy, sources: map[*transportRow]bool{}}
			plan.groups = append(plan.groups, selected)
			plan.byRepresentative[&copy] = selected
			result = append(result, &copy)
		} else {
			merged = true
		}
		if !selected.sources[route.from] {
			selected.maximum += maximum
			selected.sources[route.from] = true
		}
		selected.planned += route.rate
		selected.representative.rate += route.rate
		selected.members = append(selected.members, route)
		plan.byRoute[route] = selected
	}
	if !merged {
		return nil, &blueprint.Error{Code: "GEN_I_DENSITY_EQUIVALENT", Message: "当前出口没有可合并的同物品带，保留既有候选"}
	}
	b.denseStationOutputs = plan
	return result, nil
}

// bindDenseStationSharedOutputs 职责：把每个实际生产者接到同一真实塔带头；使用：代表路线的塔口创建后；边界：不连接虚拟生产者，合流仍由公共寻路逐条生成。
func (b *builder) bindDenseStationSharedOutputs(route *transportRoute, head, tower *placed) {
	if b.denseStationOutputs == nil {
		return
	}
	if group := b.denseStationOutputs.byRepresentative[route]; group != nil {
		group.head, group.tower = head, tower
		for _, member := range group.members {
			member.end, member.boundary = head, tower
		}
	}
}

// denseStationSharedOutputGroup 职责：核对当前规范份额所属的物理输出组；使用：黑盒逐边验证；边界：不凭相同物品或相同位置猜测成员身份。
func (b *builder) denseStationSharedOutputGroup(route *transportRoute, link *stationTransportLink) (*denseStationOutputGroup, error) {
	if !link.output || b.denseStationOutputs == nil {
		return nil, nil
	}
	group := b.denseStationOutputs.byRoute[route]
	if group == nil {
		return nil, nil
	}
	for _, member := range group.members {
		if member == route {
			return group, nil
		}
	}
	return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: "共享出口引用了组外规范份额"}
}

// validateDenseStationOutputGroup 职责：从真实来源、实际有向带链和当前配置重新证明出口能力；使用：每个已绑定塔口一次；边界：不信任缓存的最大流量或成员计数，最终公共物流验证继续检查机器来源与全图负载。
func (b *builder) validateDenseStationOutputGroup(ctx context.Context, group *denseStationOutputGroup, link *stationTransportLink) error {
	fail := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共享出口验证失败：" + reason}
	}
	if group == nil || b.denseStationOutputs == nil || !link.output || group.head != link.head || group.tower != link.tower || group.item != link.item || len(group.members) == 0 || !finitePositive(group.maximum) || !finitePositive(group.planned) {
		return fail("缺少一致的真实塔口、物品或来源")
	}
	seen := map[*transportRoute]bool{}
	sources := map[*transportRow]bool{}
	planned, maximum := 0., 0.
	for _, member := range group.members {
		if err := ctx.Err(); err != nil {
			return err
		}
		if member == nil || seen[member] || b.denseStationOutputs.byRoute[member] != group || member.edge.ItemID != group.item || member.end != group.head || member.boundary != group.tower {
			return fail("成员重复、物品不符或真实出口被替换")
		}
		seen[member] = true
		full, err := b.stationOutputMaximum(member)
		if err != nil {
			return err
		}
		if !sources[member.from] {
			maximum += full
			sources[member.from] = true
		}
		planned += member.rate
		if len(member.path) == 0 || member.path[len(member.path)-1] != group.head {
			return fail("来源未由实际带链接到公共塔口")
		}
		containsStart := false
		for i, node := range member.path {
			if node == nil || !surfaceIsBelt(node.record.ItemID) {
				return fail("合流路径包含非传送带实体")
			}
			containsStart = containsStart || node == member.start
			if i > 0 && member.path[i-1] != node && member.path[i-1].output != node {
				return fail("合流路径断开或方向错误")
			}
		}
		if !containsStart {
			return fail("路径不包含实际生产者出口")
		}
	}
	if len(sources) != len(group.sources) || maximum > b.logistics.BeltCapacityPerMinute+1e-6 || planned > maximum+1e-6 || math.Abs(maximum-group.maximum) > 1e-6 || math.Abs(planned-group.planned) > 1e-6 || math.Abs(planned-link.rate) > 1e-6 {
		return fail("实际来源、配置满负荷上界或规范总额不守恒")
	}
	for source := range sources {
		if !group.sources[source] {
			return fail("来源身份与计划不一致")
		}
	}
	return nil
}

// recordDenseStationOutputSharing 职责：记录实际合流后的外部带口变化；使用：完整黑盒终检通过后；边界：只报告已证明的合流，不宣称全局最少线长或游戏实测。
func (b *builder) recordDenseStationOutputSharing() {
	if b.denseStationOutputs == nil {
		return
	}
	members := 0
	for _, group := range b.denseStationOutputs.groups {
		members += len(group.members)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-station-shared-outputs", Status: "passed", Message: fmt.Sprintf("%d条对外输出份额通过真实有向带链合流到%d个塔口；按不同生产行重新累计配置满负荷上界，保留全部规范份额与来源身份", members, len(b.denseStationOutputs.groups))})
}
