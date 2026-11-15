package quant

// 文件职责：以原配方有理数复核浮点LP的获选基，消除舍入引起的多建机器。
// 使用：物料求解器在确定最优基后调用；边界：不放宽真实需求、不按显示精度取整，不改变LP选择的配方。

import (
	"context"
	"math/big"
	"strconv"
)

// materialNumber 职责：把输入十进制数转换为精确有理数；使用：原型和请求边界；边界：调用前已排除NaN/Inf。
func materialNumber(value float64) *big.Rat {
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(value, 'g', -1, 64))
	return r
}

// exactMaterialColumn 职责：从原配方重建单位主产物净向量；使用：物料矩阵；边界：喷涂消耗保留1/12、1/24、1/60的精确分母。
func (g *graph) exactMaterialColumn(name string) map[string]*big.Rat {
	n := g.nodes[name]
	if n.raw {
		return map[string]*big.Rat{name: big.NewRat(1, 1)}
	}
	r := g.dataset.Recipes[n.recipe]
	values := map[string]*big.Rat{}
	add := func(item string, value *big.Rat) {
		if values[item] == nil {
			values[item] = new(big.Rat)
		}
		values[item].Add(values[item], value)
	}
	produce := big.NewRat(1, 1)
	if n.scheme.ProductionMode == 2 {
		inc, _, _ := effects(n.scheme.IncreasePoints)
		produce.Mul(materialNumber(inc), materialNumber(g.settings.IncRate))
	} else if n.scheme.ProductionMode == 3 {
		_, speed, _ := effects(n.scheme.IncreasePoints)
		produce.Mul(materialNumber(speed), materialNumber(g.settings.AccRate))
	}
	total := new(big.Rat)
	for i, id := range r.Items {
		count := materialNumber(r.ItemCounts[i])
		total.Add(total, count)
		add(g.dataset.ByID[id].Name, new(big.Rat).Neg(count))
	}
	if n.scheme.ProductionMode > 0 && n.scheme.IncreasePoints > 0 {
		item, times := 1141, int64(12)
		if n.scheme.IncreasePoints == 2 {
			item, times = 1142, 24
		}
		if n.scheme.IncreasePoints >= 4 {
			item, times = 1143, 60
		}
		add(g.dataset.ByID[item].Name, new(big.Rat).Neg(new(big.Rat).Quo(total, big.NewRat(times, 1))))
	}
	for i, id := range r.Results {
		add(g.dataset.ByID[id].Name, new(big.Rat).Mul(materialNumber(r.ResultCounts[i]), produce))
	}
	net := new(big.Rat).Set(values[name])
	for _, value := range values {
		value.Quo(value, net)
	}
	return values
}

// exactMaterialSolution 职责：用原始有理数重算获选基变量；使用：浮点优化之后；边界：只求当前基的精确解，矛盾或负值必须失败，不把超额需求吞成浮点容差。
func exactMaterialSolution(ctx context.Context, columns []map[string]*big.Rat, names []string, demand *rates, basis []int) ([]*big.Rat, error) {
	count := len(columns)
	variables, slack := []int{}, map[int]bool{}
	for _, index := range basis {
		if index >= count {
			slack[index-count] = true
		} else if index >= 0 {
			variables = append(variables, index)
		}
	}
	matrix := [][]*big.Rat{}
	for row, name := range names {
		if slack[row] {
			continue
		}
		values := make([]*big.Rat, len(variables)+1)
		for i, index := range variables {
			values[i] = new(big.Rat)
			if value := columns[index][name]; value != nil {
				values[i].Set(value)
			}
		}
		values[len(variables)] = materialNumber(demand.values[name])
		matrix = append(matrix, values)
	}
	for column := range variables {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pivot := column
		for pivot < len(matrix) && matrix[pivot][column].Sign() == 0 {
			pivot++
		}
		if pivot == len(matrix) {
			return nil, qerror("Q_E_INTERNAL", "物料平衡基无法独立复核")
		}
		matrix[column], matrix[pivot] = matrix[pivot], matrix[column]
		divisor := new(big.Rat).Set(matrix[column][column])
		for j := column; j <= len(variables); j++ {
			matrix[column][j].Quo(matrix[column][j], divisor)
		}
		for i := range matrix {
			if i == column || matrix[i][column].Sign() == 0 {
				continue
			}
			factor := new(big.Rat).Set(matrix[i][column])
			for j := column; j <= len(variables); j++ {
				matrix[i][j].Sub(matrix[i][j], new(big.Rat).Mul(factor, matrix[column][j]))
			}
		}
	}
	for i := len(variables); i < len(matrix); i++ {
		if matrix[i][len(variables)].Sign() != 0 {
			return nil, qerror("Q_E_INFEASIBLE", "物料平衡基不满足精确需求")
		}
	}
	out := make([]*big.Rat, count)
	for i := range out {
		out[i] = new(big.Rat)
	}
	for i, index := range variables {
		if matrix[i][len(variables)].Sign() < 0 {
			return nil, qerror("Q_E_INFEASIBLE", "物料平衡基出现负执行量")
		}
		out[index].Set(matrix[i][len(variables)])
	}
	return out, nil
}
