package industryplan

import (
	"context"
	"errors"
	"maps"
	"math"
	"slices"
	"sort"

	"DysonSphereBox/helper/quant"
)

// preferFactoryHost 优先同恒星系的落点，同星系内复用已开工星球，再按地表和距离选址。
func preferFactoryHost(from, a, b Planet, used map[int64]bool) bool {
	if (a.StarID == from.StarID) != (b.StarID == from.StarID) {
		return a.StarID == from.StarID
	}
	if used[a.ID] != used[b.ID] {
		return used[a.ID]
	}
	if a.DrySurface != b.DrySurface {
		return a.DrySurface
	}
	da, db := distance(from, a), distance(from, b)
	return da < db || da == db && a.ID < b.ID
}

// changeFactoryBalance 更新留在本星的制造净余量，负数表示本星还需要的上游物料。
func changeFactoryBalance(balance map[int]float64, line quant.Production) {
	for _, input := range line.Inputs {
		balance[input.ItemID] -= input.RatePerMinute
	}
	for _, output := range line.Outputs {
		balance[output.ItemID] += output.RatePerMinute
	}
}

// manufacturingBalance 给出本星各制造配方间可直接抵消的物料余缺。
func manufacturingBalance(site PlanetPlan) map[int]float64 {
	balance := map[int]float64{}
	for _, line := range site.Plan.Productions {
		changeFactoryBalance(balance, line)
	}
	return balance
}

// factoryHostBalance 只计本地生产、矿区与固定需求，跨星进出口会在新分工中重新计算。
func factoryHostBalance(site PlanetPlan) map[int]float64 {
	balance := manufacturingBalance(site)
	for _, port := range site.Ports {
		switch port.Kind {
		case "mining":
			balance[port.ItemID] += port.RatePerMinute
		case "delivery", "storage", "power-fuel":
			balance[port.ItemID] -= port.RatePerMinute
		}
	}
	return balance
}

// localFactoryFlow 计算新增制造块可与本星既有采集、上下游或交付直接匹配的件数/分。
func localFactoryFlow(balance map[int]float64, line quant.Production) float64 {
	var matched float64
	for _, input := range line.Inputs {
		matched += math.Min(input.RatePerMinute, max(0, balance[input.ItemID]))
	}
	for _, output := range line.Outputs {
		matched += math.Min(output.RatePerMinute, max(0, -balance[output.ItemID]))
	}
	return matched
}

// connectedFactoryDestination 为整批迁移的制造块选择选址。批内产耗先抵消，只比较建设星可就地接通的净进出量。
func connectedFactoryDestination(from int64, blocks []quant.Production, previous Result, used, full map[int64]bool, planets map[int64]Planet, buildable []int64) int64 {
	host := factoryDestination(from, used, full, planets, buildable)
	if host == 0 {
		return 0
	}
	net := map[int]float64{}
	for _, block := range blocks {
		changeFactoryBalance(net, block)
	}
	var batch quant.Production
	for _, id := range slices.Sorted(maps.Keys(net)) {
		if net[id] < 0 {
			batch.Inputs = append(batch.Inputs, quant.ItemRate{ItemID: id, RatePerMinute: -net[id]})
		} else if net[id] > 0 {
			batch.Outputs = append(batch.Outputs, quant.ItemRate{ItemID: id, RatePerMinute: net[id]})
		}
	}
	best := 0.0
	for _, site := range previous.Planets {
		id := site.PlanetID
		if id == from || !planets[id].Buildable || full[id] {
			continue
		}
		flow := localFactoryFlow(factoryHostBalance(site), batch)
		if flow > best || flow == best && preferFactoryHost(planets[from], planets[id], planets[host], used) {
			host, best = id, flow
		}
	}
	return host
}

// distributeFactoryOverflow 以原星实测容量估算首轮可承接量，优先合并高流量上下游；每颗承接星仍需逐一生成蓝图。
func distributeFactoryOverflow(single assignment, previous Result, from int64, planets map[int64]Planet) (assignment, bool) {
	if !single.fullFactories[from] {
		return single, false
	}
	var source *PlanetPlan
	var hosts []int64
	load := map[int64]int{}
	balances := map[int64]map[int]float64{}
	manufacturing := map[int64]map[int]float64{}
	used := map[int64]bool{}
	for i := range previous.Planets {
		site := &previous.Planets[i]
		used[site.PlanetID] = true
		if site.PlanetID == from {
			source = site
		} else if planets[site.PlanetID].Buildable && !single.fullFactories[site.PlanetID] {
			hosts = append(hosts, site.PlanetID)
			load[site.PlanetID] = site.Plan.MachineCount
			balances[site.PlanetID] = factoryHostBalance(*site)
			manufacturing[site.PlanetID] = manufacturingBalance(*site)
		}
	}
	if source == nil || len(source.BlockIDs) != len(source.Plan.Productions) {
		return single, false
	}
	first, capacity := 0, 0
	for first < len(source.BlockIDs) && single.moves[source.BlockIDs[first]] == from {
		capacity += source.Plan.Productions[first].MachineCount
		first++
	}
	if capacity <= 0 || first >= len(source.BlockIDs) {
		return single, false
	}
	dest := single.moves[source.BlockIDs[first]]
	if !used[dest] && planets[dest].Buildable {
		hosts = append(hosts, dest)
		balances[dest] = map[int]float64{}
		manufacturing[dest] = map[int]float64{}
	}
	if len(hosts) < 2 {
		return single, false
	}
	sort.Slice(hosts, func(i, j int) bool {
		return preferFactoryHost(planets[from], planets[hosts[i]], planets[hosts[j]], used)
	})
	distributed := single
	distributed.moves = maps.Clone(single.moves)
	// 拥挤原星的成功前缀只是该星的容量。剩余设备超过各承接星参考容量之和时，
	// 按已开工星数分担负荷，避免把几千台设备全塞回默认承接星，再整批迁移。
	total, largest := 0, 0
	for _, id := range hosts {
		total += load[id]
	}
	for _, p := range source.Plan.Productions[first:] {
		total += p.MachineCount
		largest = max(largest, p.MachineCount)
	}
	if total > capacity*len(hosts) {
		capacity = (total+len(hosts)-1)/len(hosts) + largest - 1
	}
	for i := first; i < len(source.BlockIDs); i++ {
		block := source.BlockIDs[i]
		line := source.Plan.Productions[i]
		// 尚有参考容量时，比较落在各星可省下的物料运输。流量相同则沿用上述选址顺序。
		host := single.moves[block]
		bestFlow := -1.0
		for _, id := range hosts {
			if load[id]+line.MachineCount <= capacity {
				if flow := localFactoryFlow(balances[id], line); flow > bestFlow {
					host, bestFlow = id, flow
				}
			}
		}
		distributed.moves[block] = host
		load[host] += line.MachineCount
		changeFactoryBalance(balances[host], line)
		changeFactoryBalance(manufacturing[host], line)
	}
	matchOverflowBlocks(source, first, distributed.moves, balances, manufacturing)
	return distributed, !maps.Equal(distributed.moves, single.moves)
}

// matchOverflowBlocks 在本轮待迁移的块之间交换同种同数设备；已落位前缀、每星机器数量与承接星集合保持固定。
func matchOverflowBlocks(site *PlanetPlan, first int, moves map[string]int64, balances, manufacturing map[int64]map[int]float64) {
	type block struct {
		index int
		flow  map[int]float64
	}
	blocks := make([]block, 0, len(site.BlockIDs)-first)
	for i := first; i < len(site.BlockIDs); i++ {
		flow := map[int]float64{}
		changeFactoryBalance(flow, site.Plan.Productions[i])
		blocks = append(blocks, block{i, flow})
	}
	// 制造同星供料和两地货运都不能恶化。优先保住上下游，收益相同时取净外运更少的一对。
	for len(blocks) > 1 {
		bestLocal, bestFreight, ai, bi := 0.0, 0.0, -1, -1
		for j, b := range blocks {
			for i, a := range blocks[:j] {
				left, right := site.Plan.Productions[a.index], site.Plan.Productions[b.index]
				x, y := moves[site.BlockIDs[a.index]], moves[site.BlockIDs[b.index]]
				if x == y || left.MachineID != right.MachineID || left.MachineCount != right.MachineCount || left.RecipeID == right.RecipeID {
					continue
				}
				gain := swapFreightGain(balances[x], balances[y], a.flow, b.flow)
				local := swapFreightGain(manufacturing[x], manufacturing[y], a.flow, b.flow)
				if gain >= -1e-6 && local >= -1e-6 && (local > bestLocal+1e-6 || math.Abs(local-bestLocal) <= 1e-6 && gain > bestFreight+1e-6) {
					bestLocal, bestFreight, ai, bi = local, gain, i, j
				}
			}
		}
		if ai < 0 {
			return
		}
		a, b := blocks[ai], blocks[bi]
		left, right := site.BlockIDs[a.index], site.BlockIDs[b.index]
		x, y := moves[left], moves[right]
		for _, net := range []map[int64]map[int]float64{balances, manufacturing} {
			for id, rate := range a.flow {
				net[x][id] -= rate
				net[y][id] += rate
			}
			for id, rate := range b.flow {
				net[x][id] += rate
				net[y][id] -= rate
			}
		}
		moves[left], moves[right] = y, x
		blocks = slices.Delete(blocks, bi, bi+1)
		blocks = slices.Delete(blocks, ai, ai+1)
	}
}

// swapFreightGain 固定两颗星上的其他物料来源与用途，按净差的变化折回每分钟单程货量。
func swapFreightGain(x, y, a, b map[int]float64) float64 {
	var gain float64
	add := func(id int, delta float64) {
		gain += math.Abs(x[id]) + math.Abs(y[id]) - math.Abs(x[id]+delta) - math.Abs(y[id]-delta)
	}
	for id, rate := range a {
		add(id, b[id]-rate)
	}
	for id, rate := range b {
		if _, ok := a[id]; !ok {
			add(id, rate)
		}
	}
	return gain / 2
}

// matchedFactoryPolicy 固定已施工方案的星球和每星设备规格，仅交换两地都能减少外运的配方块。
func matchedFactoryPolicy(incumbent Result, initial assignment, held map[int64]bool) (assignment, map[int64]bool) {
	policy := initial
	policy.moves = map[string]int64{}
	// 原合同需要完整选区的星球已经有可用图纸。可选换位先用快速试摆，未通过便保留原工序。
	policy.fullSearch = nil
	policy.canOpenHost = true
	var eligible PlanetPlan
	balances := map[int64]map[int]float64{}
	manufacturing := map[int64]map[int]float64{}
	for _, site := range incumbent.Planets {
		if len(site.BlockIDs) != len(site.Plan.Productions) {
			return initial, nil
		}
		for _, id := range site.BlockIDs {
			policy.moves[id] = site.PlanetID
		}
		if !held[site.PlanetID] {
			eligible.BlockIDs = append(eligible.BlockIDs, site.BlockIDs...)
			eligible.Plan.Productions = append(eligible.Plan.Productions, site.Plan.Productions...)
			balances[site.PlanetID] = factoryHostBalance(site)
			manufacturing[site.PlanetID] = manufacturingBalance(site)
		}
	}
	before := maps.Clone(policy.moves)
	matchOverflowBlocks(&eligible, 0, policy.moves, balances, manufacturing)
	changed := map[int64]bool{}
	for id, to := range policy.moves {
		if from := before[id]; from != to {
			changed[from], changed[to] = true, true
		}
	}
	return policy, changed
}

// freightAndLocalFlow 比较按真实星球航线满舱估算的翘曲器，以及留在同星制造工序间的件数。
func freightAndLocalFlow(r Result) (warpers, local float64) {
	for _, route := range r.Routes {
		warpers += route.WarperRatePerMinute
	}
	for _, site := range r.Planets {
		if graph := site.Plan.FlowGraph; graph != nil {
			for _, edge := range graph.Edges {
				if edge.FlowKind == "internal" {
					local += edge.RatePerMinute
				}
			}
		}
	}
	return warpers, local
}

// improveVerifiedFreight 用已通过的整线作后备。只复核同星上下游与玩家评分都合格的换位；某星放不下则保留它的原工序。
func improveVerifiedFreight(ctx context.Context, incumbent Result, initial assignment, estimate func(assignment) Result, check func(*Result, assignment) (int64, error), repower func(assignment, Result, int64, error) (assignment, bool)) Result {
	oldWarp, oldLocal := freightAndLocalFlow(incumbent)
	if oldWarp == 0 {
		return incumbent
	}
	held := map[int64]bool{}
	repairs := incumbent.Repairs
	// 刚完成整线核验，原分工的最后失败星已通过。换位从各星的实查记录重新排队。
	initial.verifyFirst = 0
	for ctx.Err() == nil {
		policy, changed := matchedFactoryPolicy(incumbent, initial, held)
		if len(changed) == 0 {
			break
		}
		powered := map[int64]bool{}
		for ctx.Err() == nil {
			next := estimate(policy)
			warp, local := freightAndLocalFlow(next)
			if rank(next.Status) > rank(incumbent.Status) || len(next.Planets) == 0 || len(next.Planets) > len(incumbent.Planets) ||
				warp >= oldWarp-1e-6 || local < oldLocal-quant.FlowTolerance(local, oldLocal) || next.Score > incumbent.Score+1e-6 {
				incumbent.Repairs = repairs
				return incumbent
			}
			failed, err := check(&next, policy)
			if err == nil {
				next.Repairs = repairs
				return next
			}
			repairs++
			if timing, _ := ctx.Value(layoutTimingKey{}).(*layoutTiming); timing != nil {
				timing.recordAdjustment(err)
			}
			var power *PowerLoadError
			if errors.As(err, &power) && power.AvailableMW == nil && !powered[failed] {
				if adjusted, ok := repower(policy, next, failed, err); ok {
					powered[failed] = true
					policy = adjusted
					policy.verifyFirst = failed
					continue
				}
			}
			if !changed[failed] {
				incumbent.Repairs = repairs
				return incumbent
			}
			held[failed] = true
			break
		}
	}
	incumbent.Repairs = repairs
	return incumbent
}
