package build

// 文件职责：在六个真实机器槽位内规划分拣器轨位、并联臂和研究站堆叠高度。
// 使用：机器行装配与独立能力预检共用；边界：不改变机器数、配方、流量或分拣器等级，无解时保留明确的未验证原因。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// machineInserterPort 职责：绑定一个逻辑物料口与同侧真实机器槽位；使用：生成直臂；边界：slots 为同侧三个槽的局部索引，不能重复占用。
type machineInserterPort struct {
	top   bool
	reach int
	slots []int
}

// machineInserterPlan 职责：保存有界搜索的完整机器端方案；使用：buildLine；边界：只有实际能力不足可结构导出，capacityError中的数值或拓扑错误必须硬拒绝。
type machineInserterPlan struct {
	bottom, layers int
	inputs         []machineInserterPort
	outputs        []machineInserterPort
	capacityError  error
}

// machineInserterRates 职责：按恢复毛流计算精确量化对应的单机保守需求；使用：规划与实体复核；边界：取整余量不代表已设降速，无法识别的循环或无效数值必须硬失败。
func machineInserterRates(p quant.Production, recipe quant.Recipe) ([]float64, []float64, error) {
	flows, err := productionMaterialFlows(p, recipe)
	if err != nil {
		return nil, nil, err
	}
	denominator := p.ExactMachineCount
	if denominator < 0 || math.IsNaN(denominator) || math.IsInf(denominator, 0) {
		return nil, nil, machineMaterialFlowError(recipe.ID, "精确机器数不是非负有限数")
	}
	if denominator == 0 {
		denominator = float64(p.MachineCount)
	}
	if !finitePositive(denominator) {
		return nil, nil, machineMaterialFlowError(recipe.ID, "缺少可用于单机能力核算的有效机器数")
	}
	for _, rates := range [][]float64{flows.inputs, flows.outputs} {
		for index := range rates {
			rate := rates[index] / denominator
			if math.IsNaN(rate) || math.IsInf(rate, 0) || rates[index] > 0 && rate <= 0 {
				return nil, nil, machineMaterialFlowError(recipe.ID, "单机毛流换算溢出或下溢")
			}
			rates[index] = rate
		}
	}
	return flows.inputs, flows.outputs, nil
}

// defaultMachineInserters 职责：优先保留原输入轨并按产物顺序增加独立单臂轨；使用：可行基线与容量无解时的结构回退；边界：输入和产物合计不超过六种，每侧最多三轨。
func defaultMachineInserters(recipe quant.Recipe, layers int) machineInserterPlan {
	count := len(recipe.Items) + len(recipe.Results)
	bottom := max(count-3, min(3, len(recipe.Items)))
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	return orderedMachineInserters(len(recipe.Items), len(recipe.Results), layers, bottom, order)
}

// orderedMachineInserters 职责：把逻辑物料口排列映射为两侧三轨；使用：有限轨位枚举；边界：输出切片始终保持recipe.Results顺序而不是物理轨位顺序。
func orderedMachineInserters(inputs, outputs, layers, bottom int, order []int) machineInserterPlan {
	plan := machineInserterPlan{bottom: bottom, layers: layers, inputs: make([]machineInserterPort, inputs), outputs: make([]machineInserterPort, outputs)}
	for position, index := range order {
		top, reach := position >= bottom, position+1
		if top {
			reach -= bottom
		}
		port := machineInserterPort{top: top, reach: reach, slots: []int{reach - 1}}
		if index < inputs {
			plan.inputs[index] = port
		} else {
			plan.outputs[index-inputs] = port
		}
	}
	return plan
}

// machineInserterPlanError 职责：按计划中真实并联臂累加保守能力；使用：有限方案筛选及严格纯函数；边界：零科技、未集装，不以传送带带宽替代分拣器吞吐。
func machineInserterPlanError(p quant.Production, recipe quant.Recipe, setting Setting, plan machineInserterPlan) error {
	inputs, outputs, err := machineInserterRates(p, recipe)
	if err != nil {
		return err
	}
	for index, port := range plan.inputs {
		rate := inputs[index] * float64(plan.layers)
		capacity := float64(len(port.slots)) * inserterCapacityPerMinute(setting.InserterLv, port.reach)
		if rate > capacity+1e-6 {
			return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 原料 %d 单端口 %.3f/min 超过 Mk.%d 跨 %d 格分拣器已验证能力 %.3f/min（实际 %d 臂）", recipe.ID, recipe.Items[index], rate, setting.InserterLv, port.reach, capacity, len(port.slots))}
		}
	}
	for index, port := range plan.outputs {
		rate := outputs[index] * float64(plan.layers)
		capacity := float64(len(port.slots)) * inserterCapacityPerMinute(setting.InserterLv, port.reach)
		if rate > capacity+1e-6 {
			return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 产物 %d 单端口 %.3f/min 超过 Mk.%d 跨 %d 格分拣器已验证能力 %.3f/min（实际 %d 臂）", recipe.ID, recipe.Results[index], rate, setting.InserterLv, port.reach, capacity, len(port.slots))}
		}
	}
	return nil
}

// planMachineInserters 职责：确定性搜索两侧轨位与有限槽位，并优先保持研究站紧凑堆叠；使用：所有基础机器行；边界：最多枚举六种物料口的排列与十五层，不制造虚拟端口。
func planMachineInserters(p quant.Production, recipe quant.Recipe, setting Setting, count int) machineInserterPlan {
	layers := 1
	if p.MachineType == 6 {
		layers = max(1, min(count, setting.MaxLays))
	}
	baseline := defaultMachineInserters(recipe, layers)
	if setting.UseMixed || len(recipe.Items)+len(recipe.Results) > 6 || len(recipe.Results) == 0 {
		baseline.capacityError = &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "循环混带端口尚无按所选分拣器等级逐物品验证的六槽位方案"}
		return baseline
	}
	inputs, outputs, err := machineInserterRates(p, recipe)
	if err != nil {
		baseline.capacityError = err
		return baseline
	}
	baseline.capacityError = machineInserterPlanError(p, recipe, setting, baseline)
	if baseline.capacityError == nil {
		return baseline
	}
	rates := append(append([]float64{}, inputs...), outputs...)
	for stack := layers; stack >= 1; stack-- {
		original := defaultMachineInserters(recipe, stack)
		if machineInserterPlanError(p, recipe, setting, original) == nil {
			return original
		}
		bestScore := math.MaxInt
		best := machineInserterPlan{}
		order := make([]int, len(rates))
		used := make([]bool, len(order))
		var visit func(int)
		visit = func(depth int) {
			if depth != len(order) {
				for index := range order {
					if !used[index] {
						used[index], order[depth] = true, index
						visit(depth + 1)
						used[index] = false
					}
				}
				return
			}
			for bottom := max(0, len(order)-3); bottom <= min(3, len(order)); bottom++ {
				candidate := orderedMachineInserters(len(inputs), len(outputs), stack, bottom, order)
				if !allocateMachineInserterSlots(&candidate, rates, setting.InserterLv) {
					continue
				}
				score := 0
				for index, port := range candidate.inputs {
					score += (len(port.slots) - 1) * 100
					if port.top != baseline.inputs[index].top {
						score += 10
					}
					score += absInt(port.reach - baseline.inputs[index].reach)
				}
				for index, port := range candidate.outputs {
					score += (len(port.slots) - 1) * 100
					if port.top != baseline.outputs[index].top {
						score += 10
					}
					score += absInt(port.reach - baseline.outputs[index].reach)
				}
				if score < bestScore {
					best, bestScore = candidate, score
				}
			}
		}
		visit(0)
		if bestScore != math.MaxInt {
			return best
		}
	}
	return baseline
}

// allocateMachineInserterSlots 职责：把实际需要的额外臂放入同侧空槽；使用：轨位候选；边界：同一槽仅有一根臂，单侧最多三根，端口不能跨侧借用隐形带。
func allocateMachineInserterSlots(plan *machineInserterPlan, rates []float64, level int) bool {
	ports := make([]*machineInserterPort, 0, len(plan.inputs)+len(plan.outputs))
	for index := range plan.inputs {
		ports = append(ports, &plan.inputs[index])
	}
	for index := range plan.outputs {
		ports = append(ports, &plan.outputs[index])
	}
	occupied := [2][3]bool{}
	for _, port := range ports {
		side := 0
		if port.top {
			side = 1
		}
		occupied[side][port.slots[0]] = true
	}
	for index, port := range ports {
		rate := rates[index]
		capacity := inserterCapacityPerMinute(level, port.reach)
		if capacity <= 0 {
			return false
		}
		required := rate * float64(plan.layers)
		if required > 3*capacity+1e-6 {
			return false
		}
		number := max(1, int(math.Ceil((required-1e-6)/capacity)))
		side := 0
		if port.top {
			side = 1
		}
		for slot := 0; len(port.slots) < number && slot < 3; slot++ {
			if !occupied[side][slot] {
				occupied[side][slot] = true
				port.slots = append(port.slots, slot)
			}
		}
		if len(port.slots) < number {
			return false
		}
	}
	return true
}

// validateMachineInserterCapacity 职责：严格核验六槽位物理规划能否覆盖机器端需求；使用：纯函数回归与预估；边界：无解仍返回容量错误，结构兼容由装配层明确标记为未验证。
func validateMachineInserterCapacity(p quant.Production, recipe quant.Recipe, setting Setting) error {
	return planMachineInserters(p, recipe, setting, p.MachineCount).capacityError
}
