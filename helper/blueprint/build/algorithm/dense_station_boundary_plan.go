package build

// 文件职责：仅为黑盒对外边界分配共享塔库存和真实带口，计算可说明的设施下界。
// 使用：内部直连已规划、实际塔尚未创建时；边界：每条独立带保留一个带口，不假设堆叠科技或把相反供需角色混入同槽。

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"DysonSphereBox/helper/blueprint"
)

// denseStationBoundaryPlan 职责：保存对外角色的最少塔分配及两个不同下界；使用：构造与结果说明；边界：物料理论下界不把当前独立lane数量当成不可降低的全局下界。
type denseStationBoundaryPlan struct {
	// routes/towers 职责：保存规范外部lane及其真实库存归属；使用：一次构造；边界：不包含任何内部中间流。
	routes []*transportRoute
	towers []denseStationTower
	// 下界字段职责：区分物料必需能力与当前独立带口数量；使用：可核对的设施报告；边界：不宣称现有lane数不可进一步合并。
	roleCount, minimumPorts, materialLowerBound, laneLowerBound int
	// proven 职责：标识当前lane约束下是否已证明最少塔数；使用：报告搜索结果；边界：有界搜索未完成时不得假装全局最优。
	proven bool
}

// planDenseStationBoundaries 职责：在真实库存槽与十二口内搜索外部角色的最少塔数；使用：黑盒边界；边界：搜索有明确预算，未证明最优时保留实际可行分配并如实报告。
func planDenseStationBoundaries(ctx context.Context, routes []*transportRoute, slots int, capacity float64) (*denseStationBoundaryPlan, error) {
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒最少塔规划已取消"}
	}
	if slots <= 0 || !finitePositive(capacity) {
		return nil, &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "黑盒塔缺少有效库存槽或带容量"}
	}
	plan := &denseStationBoundaryPlan{}
	roles := []stationItem{}
	roleRates, rolePorts := map[stationItem]float64{}, map[stationItem]int{}
	for index, route := range routes {
		if index%64 == 0 && ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒对外角色规划已取消"}
		}
		if route == nil || route.edge.ItemID <= 0 || !finitePositive(route.rate) || route.rate > capacity+1e-6 {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "黑盒规范路线缺少真实单带份额"}
		}
		if route.from != nil && route.to != nil {
			continue
		}
		if route.from == nil && route.to == nil {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "黑盒外部直通物料须同时规划供货和交付设施，当前候选不冒充已接通"}
		}
		output := route.to == nil
		if !output && route.edge.FlowKind != "external-input" || output && route.edge.FlowKind != "target-output" && route.edge.FlowKind != "leftover" {
			return nil, transportAllocationError(route.edge, route.rate, "黑盒边界用途与实际供需端不一致")
		}
		role := stationItem{id: route.edge.ItemID, provide: output}
		plan.routes = append(plan.routes, route)
		roles = append(roles, role)
		roleRates[role] += route.rate
		rolePorts[role]++
	}
	if len(plan.routes) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "黑盒塔没有实际对外物料边界"}
	}
	plan.roleCount = len(rolePorts)
	for _, rate := range roleRates {
		plan.minimumPorts += max(1, int(math.Ceil((rate-1e-8)/capacity)))
	}
	plan.materialLowerBound = max((plan.roleCount+slots-1)/slots, (plan.minimumPorts+11)/12)
	for role := range rolePorts {
		if _, ok := rolePorts[stationItem{id: role.id, provide: !role.provide}]; ok {
			plan.materialLowerBound = max(plan.materialLowerBound, 2)
		}
	}
	plan.laneLowerBound = max(plan.materialLowerBound, (len(plan.routes)+11)/12)
	order := make([]int, len(roles))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, c := roles[order[i]], roles[order[j]]
		if rolePorts[a] != rolePorts[c] {
			return rolePorts[a] > rolePorts[c]
		}
		if a.id != c.id {
			return a.id < c.id
		}
		return !a.provide && c.provide
	})
	type bin struct {
		used  int
		kinds map[stationItem]int
	}
	fits := func(box bin, role stationItem) bool {
		return box.used < 12 && box.kinds[stationItem{id: role.id, provide: !role.provide}] == 0 && (box.kinds[role] > 0 || len(box.kinds) < slots)
	}
	greedy, assignments := []bin{}, make([]int, len(roles))
	for _, index := range order {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒共享库存规划已取消"}
		}
		chosen := -1
		for i, box := range greedy {
			if fits(box, roles[index]) && (chosen < 0 || box.kinds[roles[index]] > 0 && greedy[chosen].kinds[roles[index]] == 0) {
				chosen = i
			}
		}
		if chosen < 0 {
			chosen = len(greedy)
			greedy = append(greedy, bin{kinds: map[stationItem]int{}})
		}
		greedy[chosen].used++
		greedy[chosen].kinds[roles[index]]++
		assignments[index] = chosen
	}
	steps, exhausted := 0, false
	for count := plan.laneLowerBound; count < len(greedy); count++ {
		bins := make([]bin, count)
		for i := range bins {
			bins[i].kinds = map[stationItem]int{}
		}
		trial := make([]int, len(roles))
		var search func(int) bool
		search = func(at int) bool {
			steps++
			if steps > 65536 || len(order) > 256 {
				exhausted = true
				return false
			}
			if ctx.Err() != nil {
				return false
			}
			if at == len(order) {
				return true
			}
			index, seen := order[at], map[string]bool{}
			for i := range bins {
				box := &bins[i]
				if !fits(*box, roles[index]) {
					continue
				}
				keys := []string{}
				for role, n := range box.kinds {
					keys = append(keys, fmt.Sprintf("%d:%t:%d", role.id, role.provide, n))
				}
				sort.Strings(keys)
				key := strings.Join(keys, ",")
				if seen[key] {
					continue
				}
				seen[key] = true
				box.used++
				box.kinds[roles[index]]++
				trial[index] = i
				if search(at + 1) {
					return true
				}
				box.used--
				box.kinds[roles[index]]--
				if box.kinds[roles[index]] == 0 {
					delete(box.kinds, roles[index])
				}
				if exhausted || ctx.Err() != nil {
					return false
				}
			}
			return false
		}
		if search(0) {
			assignments = trial
			plan.proven = true
			break
		}
		if exhausted || ctx.Err() != nil {
			break
		}
	}
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒最少塔库存搜索已取消"}
	}
	if !exhausted {
		plan.proven = true
	}
	for index, tower := range assignments {
		for len(plan.towers) <= tower {
			plan.towers = append(plan.towers, denseStationTower{})
		}
		stock := &plan.towers[tower]
		found := false
		for _, role := range stock.items {
			found = found || role == roles[index]
		}
		if !found {
			stock.items = append(stock.items, roles[index])
		}
		stock.routes = append(stock.routes, index)
	}
	return plan, nil
}
