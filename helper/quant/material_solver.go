package quant

// 文件职责：直接按配方执行向量建立守恒模型，避免循环历史足迹重复计算副产物。
// 使用：Solve的统一量化入口；边界：变量仍是所选物品配方的净产出，不自动换配方、不把缓存作为消耗。

import (
	"context"
	"fmt"
	"math"
	"math/big"
)

// calculateMaterials 职责：联立求解目标、全部原料及副产物；使用：量化主流程；边界：只扩展目标连通的已选配方，所有回代残差必须满足守恒。
// 逻辑流：1. 展开已选配方依赖及联产 => 2. 每物品建立净供需约束 => 3. 有界LP => 4. 外供只补真实缺口 => 5. 回代剩余。
func (g *graph) calculateMaterials(ctx context.Context, requested *rates) (*rates, *rates, float64, error) {
	needs := requested.clone()
	if err := g.balanceNatural(needs); err != nil {
		return nil, nil, 0, err
	}
	active := map[string]bool{}
	queue := append([]string(nil), needs.keys()...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if active[name] {
			continue
		}
		n := g.nodes[name]
		if n == nil {
			return nil, nil, 0, qerror("Q_E_NO_RECIPE", "物料没有来源: "+name)
		}
		active[name] = true
		if n.raw || n.invalid != "" {
			continue
		}
		queue = append(queue, n.runInputs.keys()...)
		queue = append(queue, n.runOutputs.keys()...)
	}
	names, variables := []string{}, []string{}
	rows := map[string]int{}
	for _, name := range g.order {
		if !active[name] {
			continue
		}
		rows[name] = len(names)
		names = append(names, name)
		if n := g.nodes[name]; n.raw || n.invalid == "" && n.runNet > 0 {
			variables = append(variables, name)
		}
	}
	a := make([][]float64, len(names))
	exactColumns := make([]map[string]*big.Rat, len(variables))
	demand, costs := make([]float64, len(names)), make([]float64, len(variables))
	for i, name := range names {
		a[i] = make([]float64, len(variables))
		demand[i] = needs.values[name]
	}
	for column, name := range variables {
		exactColumns[column] = g.exactMaterialColumn(name)
		for item, value := range exactColumns[column] {
			a[rows[item]][column], _ = value.Float64()
		}
		costs[column] = g.itemCost(name)
		// 过剩成本作用于最终净余量；常数项不影响最优解，输入抵扣必须与输出一起计入。
		for row, item := range names {
			costs[column] += a[row][column] * g.scheme.CostWeight.ItemExtraCost[item].OverflowCost
		}
		if math.IsNaN(costs[column]) || math.IsInf(costs[column], 0) {
			return nil, nil, 0, qerror("Q_E_BAD_REQUEST", "物料平衡成本包含非有限值")
		}
	}
	var basis []int
	_, cost, err := minimize(ctx, a, demand, costs, &basis)
	if err != nil {
		return nil, nil, 0, err
	}
	solved, err := exactMaterialSolution(ctx, exactColumns, names, needs, basis)
	if err != nil {
		return nil, nil, 0, err
	}
	// 同一配方的多个主产物只是相同执行向量的不同单位；统一到单位成本最低的表述，避免重复制造行及浮点择基改变几何。
	for i, name := range variables {
		first := g.nodes[name]
		if first.raw || solved[i].Sign() == 0 {
			continue
		}
		for j, other := range variables {
			second := g.nodes[other]
			if second.raw || first.recipe != second.recipe || first.scheme != second.scheme || costs[j] >= costs[i]-1e-10 {
				continue
			}
			ratio := exactColumns[i][other]
			if ratio == nil || ratio.Sign() <= 0 {
				continue
			}
			equivalent := true
			for _, item := range names {
				left, right := exactColumns[i][item], exactColumns[j][item]
				if left == nil {
					left = new(big.Rat)
				}
				if right == nil {
					right = new(big.Rat)
				}
				if left.Cmp(new(big.Rat).Mul(right, ratio)) != 0 {
					equivalent = false
					break
				}
			}
			if equivalent {
				solved[j].Add(solved[j], new(big.Rat).Mul(solved[i], ratio))
				solved[i].SetInt64(0)
				break
			}
		}
	}
	// 外部原料为可控供给，零成本退化解中也只能进口真实缺口，不能同时进口和丢弃同物料。
	for column, name := range variables {
		if !g.nodes[name].raw {
			continue
		}
		deficit := materialNumber(needs.values[name])
		for other, rate := range solved {
			if coefficient := exactColumns[other][name]; other != column && coefficient != nil {
				deficit.Sub(deficit, new(big.Rat).Mul(coefficient, rate))
			}
		}
		if deficit.Sign() < 0 {
			deficit.SetInt64(0)
		}
		solved[column] = deficit
	}
	result, surplus := newRates(), newRates()
	for column, name := range variables {
		if solved[column].Sign() > 0 {
			value, _ := solved[column].Float64()
			result.set(name, value)
		}
	}
	for _, name := range names {
		remaining := new(big.Rat).Neg(materialNumber(needs.values[name]))
		for column, rate := range solved {
			if coefficient := exactColumns[column][name]; coefficient != nil {
				remaining.Add(remaining, new(big.Rat).Mul(coefficient, rate))
			}
		}
		if remaining.Sign() < 0 {
			return nil, nil, 0, qerror("Q_E_INFEASIBLE", fmt.Sprintf("物料平衡回代缺少%s %s/min", name, new(big.Rat).Neg(remaining).FloatString(9)))
		}
		if remaining.Sign() > 0 {
			value, _ := remaining.Float64()
			surplus.set(name, value)
		}
	}
	return result, surplus, cost, nil
}
