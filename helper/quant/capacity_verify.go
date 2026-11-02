package quant

// 文件职责：从内置原型与实体配置复核请求携带的满负荷能力。
// 使用：蓝图纯引擎的防伪边界；边界：不信任Capacity或精确台数自证，不把自定义量化倍率冒充原版设备能力。

import (
	"fmt"
	"math"
)

// VerifyProductionCapacity 职责：独立重算并逐项复核蓝图配置能力；使用：生成端接收量化快照；边界：旧快照可缺少Capacity，供剂一律不假定未生成的自喷设施。
func VerifyProductionCapacity(d *Dataset, p Production) error {
	_, err := ValidatedProductionCapacity(d, p)
	return err
}

// ValidatedProductionCapacity 职责：校验请求并返回原型重算的规范能力而非请求字段；使用：生成前规范化额定流量及精确台数；边界：旧快照也复核精确台数，但调用方不得据此补写原快照缺失的额定展示。
func ValidatedProductionCapacity(d *Dataset, p Production) (*ProductionCapacity, error) {
	invalid := func() error {
		return qerror("Q_E_BAD_REQUEST", fmt.Sprintf("配方 %d 的物料流、满负荷能力或精确台数与原版配置不符，请重新量化；增产剂须按未自喷消耗计算，不支持把自定义倍率当作实体能力", p.RecipeID))
	}
	if p.Capacity != nil && p.Capacity.Basis != ProductionCapacityBasisConfiguredFullLoad || !capacityFinitePositive(p.OutputCount) || !capacityFinitePositive(p.ExactMachineCount) || p.MachineCount <= 0 {
		return nil, invalid()
	}
	expected, err := configuredProductionCapacity(d, p)
	if err != nil {
		return nil, invalid()
	}
	exact := p.OutputCount / expected.OutputPerMachinePerMinute
	if !capacityNumberMatches(p.ExactMachineCount, exact) || !capacityNumberMatches(p.RoundingMargin, float64(p.MachineCount)-exact) {
		return nil, invalid()
	}
	scale := p.OutputCount / expected.OutputPerMinute
	scaled := func(rates []ItemRate) []ItemRate {
		out := append([]ItemRate(nil), rates...)
		for i := range out {
			out[i].RatePerMinute *= scale
		}
		return out
	}
	inputs, outputs := netFlowRates(p.Inputs, p.Outputs)
	if !capacityRatesMatch(inputs, scaled(expected.Inputs)) || !capacityRatesMatch(outputs, scaled(expected.Outputs)) {
		return nil, invalid()
	}
	actual := p.Capacity
	if actual != nil && !(capacityNumberMatches(actual.OutputPerMinute, expected.OutputPerMinute) && capacityNumberMatches(actual.OutputPerMachinePerMinute, expected.OutputPerMachinePerMinute) && capacityNumberMatches(actual.CyclesPerMinute, expected.CyclesPerMinute) && capacityNumberMatches(actual.Utilization, expected.Utilization) && capacityNumberMatches(actual.PowerMW, expected.PowerMW) && capacityRatesMatch(actual.Inputs, expected.Inputs) && capacityRatesMatch(actual.Outputs, expected.Outputs) && capacityRatesMatch(actual.GrossInputs, expected.GrossInputs) && capacityRatesMatch(actual.GrossOutputs, expected.GrossOutputs)) {
		return nil, invalid()
	}
	return expected, nil
}

// configuredProductionCapacity 职责：复用量化执行向量从原版数据恢复单条制造线配置能力；使用：能力防伪；边界：原料全部外供仅为隔离求解，不使用请求的净流、毛流、Capacity或ExactMachineCount参与计算。
func configuredProductionCapacity(d *Dataset, p Production) (*ProductionCapacity, error) {
	if d == nil || !IsVanillaDatasetKey(d.Info.Key) {
		return nil, qerror("Q_E_DATASET", "配置能力复核缺少原版数据")
	}
	index, exists := d.RecipeByID[p.RecipeID]
	item, known := d.ByID[p.MainItemID]
	if !exists || !known || p.ProliferatorMode < 0 || p.ProliferatorMode > 4 || p.ProductionMode != blueprintMode(p.ProliferatorMode) {
		return nil, qerror("Q_E_BAD_REQUEST", "配置能力复核的配方、物品或喷涂模式无效")
	}
	building, choice := -1, 0
	for candidate, id := range d.Recipes[index].Factories {
		if id == p.MachineID {
			building = candidate
		}
	}
	for candidate, recipeIndex := range d.Choices[item.Name] {
		if recipeIndex == index {
			choice = candidate + 1
		}
	}
	typ, level := MachineKind(p.MachineID)
	pointsLevel := 0
	if p.IncreasePoints > 0 {
		pointsLevel = 1
		if p.IncreasePoints >= 2 {
			pointsLevel = 2
		}
		if p.IncreasePoints >= 4 {
			pointsLevel = 3
		}
	}
	if building < 0 || choice == 0 || typ < 0 || p.MachineType != typ || p.MachineLv != level || p.IncreaseLv != pointsLevel {
		return nil, qerror("Q_E_BAD_REQUEST", "配置能力复核的机器或增产等级无效")
	}
	scheme := Scheme{ItemRecipeChoices: map[string]int{item.Name: choice}, SchemeForRecipe: make([]RecipeScheme, len(d.Recipes))}
	scheme.SchemeForRecipe[index] = RecipeScheme{Building: building, IncreasePoints: p.IncreasePoints, ProductionMode: p.ProliferatorMode}
	external := make(map[string]bool, len(d.Items))
	for _, material := range d.Items {
		if material.ID != p.MainItemID {
			external[material.Name] = true
		}
	}
	g, err := makeGraph(d, scheme, Settings{MineralizeList: external})
	if err != nil {
		return nil, err
	}
	n := g.nodes[item.Name]
	if n == nil || n.invalid != "" || n.recipe != index || n.scheme.ProductionMode != p.ProliferatorMode || n.scheme.IncreasePoints != p.IncreasePoints {
		return nil, qerror("Q_E_BAD_REQUEST", "配置能力复核未能恢复一致的执行向量")
	}
	p.ExactMachineCount = p.OutputCount / machineOutputPerMinute(n)
	return productionCapacity(d, n, p)
}

// capacityFinitePositive 职责：排除非正或非有限的配置数值；使用：能力可信性复核；边界：不做取整或修复。
func capacityFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// capacityNumberMatches 职责：只允许原型计算的浮点舍入误差；使用：能力标量及物料向量比较；边界：不是游戏物理误差或显示精度容差。
func capacityNumberMatches(actual, expected float64) bool {
	return !math.IsNaN(actual) && !math.IsInf(actual, 0) && !math.IsNaN(expected) && !math.IsInf(expected, 0) && math.Abs(actual-expected) <= 1e-8*math.Max(1, math.Max(math.Abs(actual), math.Abs(expected)))
}

// capacityRatesMatch 职责：逐物品复核配置物料向量而不依赖显示名称或排序；使用：满负荷净流和毛流；边界：拒绝负数、非有限数、额外物品或缺失物品。
func capacityRatesMatch(actual, expected []ItemRate) bool {
	totals := func(rates []ItemRate) (map[int]float64, bool) {
		out := make(map[int]float64, len(rates))
		for _, rate := range rates {
			if rate.ItemID <= 0 || !capacityFinitePositive(rate.RatePerMinute) {
				return nil, false
			}
			out[rate.ItemID] += rate.RatePerMinute
		}
		return out, true
	}
	left, valid := totals(actual)
	right, expectedValid := totals(expected)
	if !valid || !expectedValid || len(left) != len(right) {
		return false
	}
	for item, rate := range right {
		actualRate, exists := left[item]
		if !exists || !capacityNumberMatches(actualRate, rate) {
			return false
		}
	}
	return true
}
