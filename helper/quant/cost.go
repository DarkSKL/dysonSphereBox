package quant

// 文件职责：计算单条制造执行的成本与已有产线供需。
// 使用：直接物料平衡求解器；边界：不递归累计副产物历史，配方选择仍由用户指定。

import "strings"

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

// balanceNatural 职责：合入已有固定产线；使用：求解前平衡；边界：不修改真实游戏资源。
func (g *graph) balanceNatural(needs *rates) error {
	for _, line := range g.settings.NaturalProductionLine {
		choices := g.dataset.Choices[line.TargetItem]
		if line.RecipeChoice < 1 || line.RecipeChoice > len(choices) || line.MachineCount < 0 {
			return qerror("Q_E_BAD_REQUEST", "已有产线的配方或数量无效")
		}
		r := g.dataset.Recipes[choices[line.RecipeChoice-1]]
		if line.Building < 0 || line.Building >= len(r.Factories) || line.IncreasePoints < 0 || line.IncreasePoints > 4 || line.ProductionMode < 0 || line.ProductionMode > 4 {
			return qerror("Q_E_BAD_REQUEST", "已有产线配置无效")
		}
		factory := g.dataset.ByID[r.Factories[line.Building]]
		speed := factory.Speed
		if speed == 0 {
			speed = 1
		}
		times := 60 * line.MachineCount * speed / (r.TimeSpend / 60)
		inScale, outScale := 1., 1.
		if line.ProductionMode > 0 && line.IncreasePoints > 0 {
			inc, acc, _ := effects(line.IncreasePoints)
			switch line.ProductionMode {
			case 1:
				inScale, outScale = acc, acc
			case 2:
				outScale = inc
			case 3:
				outScale = acc
			case 4:
				outScale = float64(line.IncreasePoints) / 10
			}
			total := 0.
			for _, v := range r.ItemCounts {
				total += v
			}
			spray, cost, err := sprayCost(g.dataset, line.IncreasePoints)
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
