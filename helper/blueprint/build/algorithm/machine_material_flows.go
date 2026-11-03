package build

// 文件职责：由量化净流与配方计量恢复机器端毛流和本地回流。
// 使用：机器带拆行、分拣器能力和循环接驳共用；边界：不修改量化快照，不猜测丢失的自定义增产倍率，无法唯一恢复时硬失败。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// machineMaterialFlows 职责：保存整条量化产线的机器毛流；使用：按真实台数或精确台数换算局部需求；边界：切片遵循配方顺序，喷涂剂附加供给不占机器槽位。
type machineMaterialFlows struct {
	inputs, outputs []float64
	// recycle 职责：保存同物品出入可在本产线闭合的毛流份额；使用：优先回流规划；边界：不表示启动库存已提供或回流实体已建成。
	recycle map[int]float64
}

// datasetRecipeForProduction 职责：从数据集原配方得到本条产线的有效端口配方；使用：所有生成模式入口统一配方视图；边界：只读数据集并在调用端处理配方不存在。
func datasetRecipeForProduction(dataset *quant.Dataset, p quant.Production) (quant.Recipe, error) {
	index, ok := dataset.RecipeByID[p.RecipeID]
	if !ok {
		return quant.Recipe{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 不存在", p.RecipeID)}
	}
	return effectiveProductionRecipe(p, dataset.Recipes[index])
}

// effectiveProductionRecipe 职责：复制生成端实际需要的配方端口，并补入可信满负荷毛流中存在的额外产物；使用：机器端口布局；边界：不修改数据集缓存，缺少可信能力向量时保留旧快照语义。
func effectiveProductionRecipe(p quant.Production, recipe quant.Recipe) (quant.Recipe, error) {
	capacity := p.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || len(capacity.GrossOutputs) == 0 {
		return recipe, nil
	}
	if p.MachineCount <= 0 || !finitePositive(capacity.CyclesPerMinute) {
		return recipe, machineMaterialFlowError(recipe.ID, "满负荷能力缺少有效执行次数")
	}
	fullCycles := capacity.CyclesPerMinute * float64(p.MachineCount)
	if !finitePositive(fullCycles) {
		return recipe, machineMaterialFlowError(recipe.ID, "满负荷能力执行次数溢出")
	}
	known := map[int]bool{}
	for _, item := range recipe.Results {
		known[item] = true
	}
	for _, flow := range capacity.GrossOutputs {
		if flow.ItemID <= 0 || !finitePositive(flow.RatePerMinute) {
			return recipe, machineMaterialFlowError(recipe.ID, "满负荷额外产物编号或速率无效")
		}
		if known[flow.ItemID] {
			continue
		}
		count := flow.RatePerMinute / fullCycles
		if !finitePositive(count) {
			return recipe, machineMaterialFlowError(recipe.ID, "满负荷额外产物计量无效")
		}
		recipe.Results = append(recipe.Results, flow.ItemID)
		recipe.ResultCounts = append(recipe.ResultCounts, count)
		known[flow.ItemID] = true
	}
	return recipe, nil
}

// capacityMaterialFlows 职责：把量化器保存的配置满负荷毛流按期望主产物缩放到本次产线；使用：自循环和额外产物不再从净流反推；边界：必须复核缩放后的净差与可信期望流完全一致。
func capacityMaterialFlows(p quant.Production, recipe quant.Recipe) (machineMaterialFlows, bool, error) {
	out := machineMaterialFlows{inputs: make([]float64, len(recipe.Items)), outputs: make([]float64, len(recipe.Results)), recycle: map[int]float64{}}
	capacity := p.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || len(capacity.GrossInputs) == 0 && len(capacity.GrossOutputs) == 0 {
		return out, false, nil
	}
	if p.MachineCount <= 0 || !finitePositive(p.OutputCount) || !finitePositive(capacity.OutputPerMinute) {
		return out, true, machineMaterialFlowError(recipe.ID, "满负荷能力缺少有效主产物速率")
	}
	scale := p.OutputCount / capacity.OutputPerMinute
	if !finitePositive(scale) || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return out, true, machineMaterialFlowError(recipe.ID, "满负荷能力无法缩放到期望产量")
	}
	inputs, outputs := map[int]float64{}, map[int]float64{}
	add := func(dst map[int]float64, flow quant.ItemRate) error {
		if flow.ItemID <= 0 || flow.RatePerMinute < 0 || math.IsNaN(flow.RatePerMinute) || math.IsInf(flow.RatePerMinute, 0) {
			return machineMaterialFlowError(recipe.ID, "满负荷毛流编号或速率无效")
		}
		value := flow.RatePerMinute * scale
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return machineMaterialFlowError(recipe.ID, "满负荷毛流缩放溢出")
		}
		dst[flow.ItemID] += value
		if math.IsInf(dst[flow.ItemID], 0) {
			return machineMaterialFlowError(recipe.ID, "满负荷同物品毛流累计溢出")
		}
		return nil
	}
	for _, flow := range capacity.GrossInputs {
		if err := add(inputs, flow); err != nil {
			return out, true, err
		}
	}
	for _, flow := range capacity.GrossOutputs {
		if err := add(outputs, flow); err != nil {
			return out, true, err
		}
	}
	netIn, err := machineMaterialRateTotals(p.Inputs, recipe.ID)
	if err != nil {
		return out, true, err
	}
	netOut, err := machineMaterialRateTotals(p.Outputs, recipe.ID)
	if err != nil {
		return out, true, err
	}
	items := map[int]bool{}
	for item := range inputs {
		items[item] = true
	}
	for item := range outputs {
		items[item] = true
	}
	for item := range netIn {
		items[item] = true
	}
	for item := range netOut {
		items[item] = true
	}
	for item := range items {
		actual := outputs[item] - inputs[item]
		want := netOut[item] - netIn[item]
		if math.Abs(actual-want) > machineMaterialRateTolerance(actual, want, math.Max(inputs[item], outputs[item])) {
			return out, true, machineMaterialFlowError(recipe.ID, fmt.Sprintf("物品 %d 的配置毛流净差 %.9f/min 与期望净流 %.9f/min 不一致", item, actual, want))
		}
	}
	inputItems, outputItems := map[int]bool{}, map[int]bool{}
	for _, item := range recipe.Items {
		inputItems[item] = true
	}
	for _, item := range recipe.Results {
		outputItems[item] = true
	}
	spray := productionSprayItem(p)
	for item := range inputs {
		if !inputItems[item] && item != spray {
			return out, true, machineMaterialFlowError(recipe.ID, fmt.Sprintf("配置毛原料 %d 不在配方或增产剂中", item))
		}
	}
	for item := range outputs {
		if !outputItems[item] {
			return out, true, machineMaterialFlowError(recipe.ID, fmt.Sprintf("配置毛产物 %d 不在有效配方中", item))
		}
	}
	for index, item := range recipe.Items {
		if !finitePositive(inputs[item]) {
			return out, true, machineMaterialFlowError(recipe.ID, fmt.Sprintf("配置毛原料 %d 缺失", item))
		}
		out.inputs[index] = inputs[item]
	}
	for index, item := range recipe.Results {
		if !finitePositive(outputs[item]) {
			return out, true, machineMaterialFlowError(recipe.ID, fmt.Sprintf("配置毛产物 %d 缺失", item))
		}
		out.outputs[index] = outputs[item]
		if recycled := math.Min(inputs[item], outputs[item]); recycled > 0 {
			out.recycle[item] = recycled
		}
	}
	return out, true, nil
}

// machineMaterialEquation 职责：保存一个物品的规范净差方程；使用：两未知执行次数和产出次数消元；边界：归一化只改善数值条件，不改变配方计量。
type machineMaterialEquation struct {
	item              int
	input, output     float64
	inputCoefficient  float64
	outputCoefficient float64
	net               float64
}

// productionMaterialFlows 职责：恢复机器端逐物品毛流并核对全部净差方程；使用：拆行、分拣器及回流规划；边界：非循环保留匹配快照总量，循环必须有唯一且一致的正有限解。
// 逻辑流：1. 汇总快照净率 => 2. 检测同物品循环 => 3. 用净差=-原料数*执行次数+产物数*产出次数求唯一解 => 4. 核验全部方程并生成毛流和回流。
func productionMaterialFlows(p quant.Production, recipe quant.Recipe) (machineMaterialFlows, error) {
	var err error
	recipe, err = effectiveProductionRecipe(p, recipe)
	if err != nil {
		return machineMaterialFlows{}, err
	}
	out := machineMaterialFlows{inputs: make([]float64, len(recipe.Items)), outputs: make([]float64, len(recipe.Results)), recycle: map[int]float64{}}
	inputRates, err := machineMaterialRateTotals(p.Inputs, recipe.ID)
	if err != nil {
		return out, err
	}
	outputRates, err := machineMaterialRateTotals(p.Outputs, recipe.ID)
	if err != nil {
		return out, err
	}
	inputItems, outputItems := map[int]bool{}, map[int]bool{}
	cyclic := false
	for _, item := range recipe.Items {
		inputItems[item] = true
	}
	for _, item := range recipe.Results {
		outputItems[item] = true
		cyclic = cyclic || inputItems[item]
	}
	if !cyclic {
		if capacity, used, capacityErr := capacityMaterialFlows(p, recipe); used {
			if capacityErr != nil {
				return out, capacityErr
			}
			return capacity, nil
		}
		for index, item := range recipe.Items {
			out.inputs[index] = inputRates[item]
		}
		for index, item := range recipe.Results {
			out.outputs[index] = outputRates[item]
		}
		return out, nil
	}
	if len(recipe.Items) != len(recipe.ItemCounts) || len(recipe.Results) != len(recipe.ResultCounts) {
		return out, machineMaterialFlowError(recipe.ID, "循环毛流恢复缺少完整配方计量")
	}
	inputs, outputs := map[int]float64{}, map[int]float64{}
	items := make([]int, 0, len(recipe.Items)+len(recipe.Results))
	for index, item := range recipe.Items {
		if item <= 0 || !finitePositive(recipe.ItemCounts[index]) || inputs[item] > 0 {
			return out, machineMaterialFlowError(recipe.ID, "循环配方原料计量无效或重复")
		}
		inputs[item] = recipe.ItemCounts[index]
		items = append(items, item)
	}
	for index, item := range recipe.Results {
		if item <= 0 || !finitePositive(recipe.ResultCounts[index]) || outputs[item] > 0 {
			return out, machineMaterialFlowError(recipe.ID, "循环配方产物计量无效或重复")
		}
		outputs[item] = recipe.ResultCounts[index]
		if !inputItems[item] {
			items = append(items, item)
		}
	}
	mode := p.ProliferatorMode
	if mode < 0 || mode > 4 || mode > 0 && (p.IncreasePoints <= 0 || p.IncreasePoints > 4 || recipe.Proliferator&(1<<uint(mode-1)) == 0) {
		return out, machineMaterialFlowError(recipe.ID, "循环快照喷涂模式与配方契约不匹配")
	}
	spray := productionSprayItem(p)
	if spray > 0 && (inputItems[spray] || outputItems[spray]) {
		return out, machineMaterialFlowError(recipe.ID, "喷涂剂同时属于循环配方，快照无法区分机器毛流与附加供剂")
	}
	for _, flow := range p.Inputs {
		if !inputItems[flow.ItemID] && flow.ItemID != spray {
			return out, machineMaterialFlowError(recipe.ID, fmt.Sprintf("净原料 %d 不在循环配方或附加供剂中", flow.ItemID))
		}
	}
	for _, flow := range p.Outputs {
		if !outputItems[flow.ItemID] {
			return out, machineMaterialFlowError(recipe.ID, fmt.Sprintf("净产物 %d 不在循环配方中", flow.ItemID))
		}
	}
	for _, item := range items {
		if inputRates[item] > 0 && outputRates[item] > 0 {
			return out, machineMaterialFlowError(recipe.ID, fmt.Sprintf("循环物品 %d 同时出现在快照两侧，要求可信量化净流而非旧毛流快照", item))
		}
	}
	// 先验证循环计量和净流快照，再采用满负荷毛流；可信能力字段不能
	// 让缺失计量、错误喷涂模式或旧毛流快照绕过循环数据边界。
	if capacity, used, capacityErr := capacityMaterialFlows(p, recipe); used {
		if capacityErr != nil {
			return out, capacityErr
		}
		return capacity, nil
	}
	equations := make([]machineMaterialEquation, 0, len(items))
	for _, item := range items {
		a, b := inputs[item], outputs[item]
		scale := math.Max(a, b)
		equations = append(equations, machineMaterialEquation{item: item, input: a, output: b, inputCoefficient: -a / scale, outputCoefficient: b / scale, net: (outputRates[item] - inputRates[item]) / scale})
	}
	left, right, bestDeterminant := -1, -1, 0.
	for i := range equations {
		for j := i + 1; j < len(equations); j++ {
			determinant := equations[i].inputCoefficient*equations[j].outputCoefficient - equations[j].inputCoefficient*equations[i].outputCoefficient
			if math.Abs(determinant) > math.Abs(bestDeterminant) {
				left, right, bestDeterminant = i, j, determinant
			}
		}
	}
	// 只有两列满秩才可区分执行次数和产出倍率；极近共线也不能靠默认倍率补猜毛流。
	if left < 0 || math.Abs(bestDeterminant) <= 1e-10 {
		return out, machineMaterialFlowError(recipe.ID, "循环净流方程秩不足或近奇异，不能唯一恢复机器毛流，请使用保留完整计量的可识别方案")
	}
	a, b := equations[left], equations[right]
	runs := (a.net*b.outputCoefficient - b.net*a.outputCoefficient) / bestDeterminant
	producedRuns := (a.inputCoefficient*b.net - b.inputCoefficient*a.net) / bestDeterminant
	if !finitePositive(runs) || !finitePositive(producedRuns) {
		return out, machineMaterialFlowError(recipe.ID, "循环净流没有正有限的机器执行次数和产出次数")
	}
	// 量化原始模式 0/1/4 的产物计量不变；模式 2/3 的自定义倍率不在快照中，必须由方程本身恢复。
	if mode != 2 && mode != 3 && math.Abs(runs-producedRuns) > machineMaterialRateTolerance(runs, producedRuns, math.Max(runs, producedRuns)) {
		return out, machineMaterialFlowError(recipe.ID, "循环净流隐含的产出倍率与不增产模式不一致")
	}
	for _, equation := range equations {
		grossInput, grossOutput := equation.input*runs, equation.output*producedRuns
		if equation.input > 0 && !finitePositive(grossInput) || equation.output > 0 && !finitePositive(grossOutput) {
			return out, machineMaterialFlowError(recipe.ID, "循环机器毛流溢出或下溢")
		}
		actual := outputRates[equation.item] - inputRates[equation.item]
		expected := grossOutput - grossInput
		if math.Abs(actual-expected) > machineMaterialRateTolerance(actual, expected, math.Max(grossInput, grossOutput)) {
			return out, machineMaterialFlowError(recipe.ID, fmt.Sprintf("物品 %d 的净流 %.9f/min 与配方恢复值 %.9f/min 不一致", equation.item, actual, expected))
		}
		if recycled := math.Min(grossInput, grossOutput); recycled > 0 {
			out.recycle[equation.item] = recycled
		}
	}
	for index, count := range recipe.ItemCounts {
		out.inputs[index] = count * runs
	}
	for index, count := range recipe.ResultCounts {
		out.outputs[index] = count * producedRuns
	}
	return out, nil
}

// machineMaterialRateTolerance 职责：按净率精度与毛流消减误差限定残差；使用：唯一解复核；边界：没有固定绝对下限，微小正流也不能因通用物流容差被吞掉或变成任意倍率。
func machineMaterialRateTolerance(actual, expected, gross float64) float64 {
	relative := math.Max(math.Abs(actual), math.Abs(expected)) * 1e-8
	rounding := gross * (math.Nextafter(1, 2) - 1) * 32
	return math.Max(relative, rounding)
}

// machineMaterialRateTotals 职责：校验并合并快照同物品速率；使用：机器毛流恢复；边界：允许缺项和零率，不接受负数、非有限数或求和溢出。
func machineMaterialRateTotals(flows []quant.ItemRate, recipe int) (map[int]float64, error) {
	totals := map[int]float64{}
	for _, flow := range flows {
		if flow.ItemID <= 0 || flow.RatePerMinute < 0 || math.IsNaN(flow.RatePerMinute) || math.IsInf(flow.RatePerMinute, 0) {
			return nil, machineMaterialFlowError(recipe, "快照物料编号或速率无效")
		}
		totals[flow.ItemID] += flow.RatePerMinute
		if math.IsInf(totals[flow.ItemID], 0) {
			return nil, machineMaterialFlowError(recipe, "快照同物品累计速率溢出")
		}
	}
	return totals, nil
}

// machineMaterialFlowError 职责：统一毛流恢复数值错误的硬失败分类；使用：机器规划及预检；边界：不能降级为可兼容的分拣器容量不足。
func machineMaterialFlowError(recipe int, reason string) error {
	return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 机器毛流恢复失败：%s", recipe, reason)}
}
