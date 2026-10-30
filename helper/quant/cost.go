package quant

// 文件职责：前向历史足迹、成本和 LP 模型；使用：Solve；边界：变量仍是物品，不能改为每配方一个变量。
// 模型行为对照 dsp-calc global_state.jsx，独立 Go 实现，见 NOTICE.md。

import (
	"context"
	"math"
	"strings"
)

// price 职责：保存单位物品历史成本；使用：前向展开；边界：关键/多来源节点预置零值。
type price struct {
	// materials 职责：历史足迹；使用：需求展开；边界：副产物以负量记录。
	materials *rates
	// own 职责：本节点成本；使用：累计；边界：不重复累计历史。
	own float64
	// total 职责：累计成本；使用：LP 目标；边界：只计正原料足迹。
	total float64
}

// itemCost 职责：计算单位净产出的设施成本；使用：前向和 LP；边界：不自动选择增产或设施。
func (g *graph) itemCost(name string) float64 {
	n := g.nodes[name]
	if n.raw || n.invalid != "" {
		return 0
	}
	extra := g.scheme.CostWeight.ItemExtraCost[name]
	cost := 0.
	if extra.Enabled {
		cost = extra.Cost
		if !extra.Cumulative {
			return cost
		}
	}
	space := 1.
	if g.scheme.CostWeight.Space != nil {
		space = *g.scheme.CostWeight.Space
	}
	speed := n.factory.Speed
	if speed <= 0 {
		speed = 1
	}
	count := 1 / n.rate / speed
	layer := 1.
	if strings.HasSuffix(n.factory.Name, "研究站") {
		layer = g.settings.StackResearchLab
	}
	_, _, power := effects(n.scheme.IncreasePoints)
	return cost + count*(space*n.factory.Space/layer+g.scheme.CostWeight.Power*n.factory.WorkEnergyPerTick*.00006*power+g.scheme.CostWeight.BuildingCost[n.factory.Name])
}

// prices 职责：按剥离顺序累加历史；使用：求解前；边界：不递归穿过关键或多来源节点。
func (g *graph) prices() (map[string]*price, error) {
	out := map[string]*price{}
	special := map[string]bool{}
	for _, k := range g.keys {
		special[k] = true
	}
	for _, k := range g.multi.keys() {
		special[k] = true
	}
	for _, k := range g.order {
		if g.nodes[k].raw {
			special[k] = true
		}
		if special[k] {
			out[k] = &price{materials: newRates()}
		}
	}
	for _, name := range g.sequence {
		if special[name] {
			continue
		}
		p := &price{materials: newRates(), own: g.itemCost(name)}
		out[name] = p
		add := func(material string, amount float64) error {
			previous, ok := out[material]
			if !ok {
				return qerror("Q_E_INTERNAL", "历史产出顺序缺少: "+material)
			}
			p.materials.add(material, amount)
			for _, sub := range previous.materials.keys() {
				p.materials.add(sub, previous.materials.values[sub]*amount)
			}
			return nil
		}
		n := g.nodes[name]
		for _, k := range n.inputs.keys() {
			if err := add(k, n.inputs.values[k]); err != nil {
				return nil, err
			}
		}
		for _, k := range n.sides.keys() {
			if err := add(k, -n.sides.values[k]); err != nil {
				return nil, err
			}
		}
		p.total = p.own
		for _, k := range p.materials.keys() {
			if p.materials.values[k] > 0 {
				p.total += out[k].own * p.materials.values[k]
			}
		}
		if math.IsNaN(p.total) || math.IsInf(p.total, 0) {
			return nil, qerror("Q_E_BAD_REQUEST", "成本包含非有限数值")
		}
	}
	return out, nil
}

// balanceNatural 职责：合入已有固定产线；使用：求解前平衡；边界：不修改真实游戏资源。
func (g *graph) balanceNatural(needs *rates) error {
	for _, line := range g.settings.NaturalProductionLine {
		choices := g.dataset.Choices[line.TargetItem]
		if line.RecipeChoice < 1 || line.RecipeChoice > len(choices) || line.MachineCount < 0 || math.IsNaN(line.MachineCount) || math.IsInf(line.MachineCount, 0) {
			return qerror("Q_E_BAD_REQUEST", "已有产线的配方或数量无效")
		}
		r := g.dataset.Recipes[choices[line.RecipeChoice-1]]
		rs, err := normalizeRecipeScheme(g.dataset, r, RecipeScheme{Building: line.Building, IncreasePoints: line.IncreasePoints, ProductionMode: line.ProductionMode})
		if err != nil {
			return qerror("Q_E_BAD_REQUEST", "已有产线配置无效: "+err.Error())
		}
		factory := g.dataset.ByID[r.Factories[line.Building]]
		speed := factory.Speed
		if speed == 0 {
			speed = 1
		}
		times := 60 * line.MachineCount * speed / (r.TimeSpend / 60)
		// 固定产线与普通产线共享采集/特殊设施倍率，避免同一配方出现两套语义。
		lineItem := line.TargetItem
		if lineItem == "" && len(r.Results) > 0 {
			lineItem = g.dataset.ByID[r.Results[0]].Name
		}
		times = multiplier(times, factory, lineItem, g.settings)
		inScale, outScale := 1., 1.
		if rs.ProductionMode > 0 && rs.IncreasePoints > 0 {
			inc, acc, _ := effects(rs.IncreasePoints)
			switch rs.ProductionMode {
			case 1:
				inScale, outScale = acc, acc
			case 2:
				outScale = inc
			case 3:
				outScale = acc
			case 4:
				outScale = float64(rs.IncreasePoints) / 10
			}
			total := 0.
			for _, v := range r.ItemCounts {
				total += v
			}
			spray, cost, err := sprayCost(g.dataset, rs.IncreasePoints, *g.settings.ProliferateItself)
			if err != nil {
				return err
			}
			needs.add(spray, cost*total*times*inScale)
		}
		for i, id := range r.Items {
			needs.add(g.dataset.ByID[id].Name, r.ItemCounts[i]*times*inScale)
		}
		for i, id := range r.Results {
			needs.add(g.dataset.ByID[id].Name, -r.ResultCounts[i]*times*outScale)
		}
	}
	return nil
}

// calculate 职责：按参考管线展开、求解、回填；使用：Solve；边界：不可行或超时失败，不返回半成品。
func (g *graph) calculate(ctx context.Context, needs *rates) (*rates, *rates, float64, error) {
	if err := g.balanceNatural(needs); err != nil {
		return nil, nil, 0, err
	}
	prices, err := g.prices()
	if err != nil {
		return nil, nil, 0, err
	}
	result, lp, surplus := newRates(), newRates(), newRates()
	for _, name := range needs.keys() {
		v := needs.values[name]
		if v <= 0 {
			continue
		}
		p, ok := prices[name]
		if !ok {
			return nil, nil, 0, qerror("Q_E_NO_RECIPE", "目标没有来源: "+name)
		}
		result.add(name, v)
		for _, mat := range p.materials.keys() {
			result.add(mat, p.materials.values[mat]*v)
		}
	}
	for _, k := range g.multi.keys() {
		lp.set(k, result.values[k])
	}
	for _, k := range needs.keys() {
		supply := needs.values[k]
		if supply >= 0 {
			continue
		}
		if _, multi := g.multi.values[k]; multi {
			lp.add(k, supply)
		} else if amount, ok := result.values[k]; ok {
			p := prices[k]
			if amount+supply > 0 {
				for _, mat := range p.materials.keys() {
					result.add(mat, p.materials.values[mat]*supply)
				}
				result.add(k, supply)
			} else {
				for _, mat := range p.materials.keys() {
					result.add(mat, -p.materials.values[mat]*amount)
				}
				lp.set(k, amount+supply)
				result.set(k, 0)
			}
		} else {
			lp.set(k, supply)
		}
	}
	for _, k := range g.keys {
		if _, ok := lp.values[k]; !ok {
			lp.set(k, result.values[k])
		}
	}
	names := lp.keys()
	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	count := len(names)
	a := make([][]float64, count)
	b, c := make([]float64, count), make([]float64, count)
	for i, name := range names {
		a[i] = make([]float64, count)
		b[i] = lp.values[name]
	}
	for col, name := range names {
		n := g.nodes[name]
		coeff := newRates()
		coeff.set(name, 1)
		c[col] = g.itemCost(name) + g.scheme.CostWeight.ItemExtraCost[name].OverflowCost
		for _, side := range n.sides.keys() {
			coeff.add(side, n.sides.values[side])
			c[col] += n.sides.values[side] * g.scheme.CostWeight.ItemExtraCost[side].OverflowCost
		}
		for _, material := range n.inputs.keys() {
			amount := n.inputs.values[material]
			p := prices[material]
			c[col] += amount * p.total
			if _, ok := index[material]; ok {
				coeff.add(material, -amount)
			}
			for _, sub := range p.materials.keys() {
				history := p.materials.values[sub]
				if _, ok := index[sub]; ok {
					coeff.add(sub, -history*amount)
				} else {
					for _, side := range g.nodes[sub].sides.keys() {
						coeff.add(side, g.nodes[sub].sides.values[side]*amount*history)
					}
				}
			}
		}
		if math.IsNaN(c[col]) || math.IsInf(c[col], 0) {
			return nil, nil, 0, qerror("Q_E_BAD_REQUEST", "LP 成本无效")
		}
		for row, k := range names {
			a[row][col] = coeff.values[k]
		}
	}
	solved, cost, err := minimize(ctx, a, b, c)
	if err != nil {
		return nil, nil, 0, err
	}
	// 同一多产物配方可能对应成比例的物品变量；统一选择单位成本较低的基，避免同一机器列被重复表述。
	for i := range solved {
		if solved[i] <= 1e-10 || c[i] <= 0 {
			continue
		}
		for j := range solved {
			if c[j] <= 0 || c[j] >= c[i]-1e-10 {
				continue
			}
			ratio := c[i] / c[j]
			equivalent := true
			for row := range a {
				if math.Abs(a[row][i]-ratio*a[row][j]) > 1e-9*math.Max(1, math.Abs(a[row][i])) {
					equivalent = false
					break
				}
			}
			if equivalent {
				solved[j] += solved[i] * ratio
				solved[i] = 0
				break
			}
		}
	}
	for row, name := range names {
		v := -b[row]
		for col, x := range solved {
			v += a[row][col] * x
		}
		if v > 1e-8 {
			surplus.set(name, v)
		} else if v < -1e-6 {
			return nil, nil, 0, qerror("Q_E_INFEASIBLE", "LP 回代不能满足: "+name)
		}
		result.set(name, 0)
	}
	for col, name := range names {
		x := solved[col]
		if x <= 1e-10 {
			continue
		}
		result.add(name, x)
		for _, material := range g.nodes[name].inputs.keys() {
			if _, ok := index[material]; ok {
				continue
			}
			amount := g.nodes[name].inputs.values[material]
			result.add(material, x*amount)
			p := prices[material]
			for _, sub := range p.materials.keys() {
				if _, ok := index[sub]; !ok {
					result.add(sub, x*amount*p.materials.values[sub])
				}
			}
		}
	}
	return result, surplus, cost, nil
}
