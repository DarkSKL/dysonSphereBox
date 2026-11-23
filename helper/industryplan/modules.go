package industryplan

// 文件职责：把已量化产线划为可独立落位的整数机器模块；使用：跨星分工前调用；边界：不增加产量、设备或原料，也不改变配方和目标。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/quant"
)

// productionBlockIDs 职责：按配方身份和块序号关联分工；使用：燃料闭环重新量化；边界：前序配方增加模块不会移动后序模块的身份。
func productionBlockIDs(productions []quant.Production) []string {
	counts := map[string]int{}
	ids := make([]string, len(productions))
	for i, p := range productions {
		identity := fmt.Sprintf("%d:%d:%d:%d:%d", p.RecipeID, p.MainItemID, p.MachineID, p.ProliferatorMode, p.IncreasePoints)
		ids[i] = fmt.Sprintf("%s:%d", identity, counts[identity])
		counts[identity]++
	}
	return ids
}

// SplitProductions 职责：按机器上限等负荷切分生产实例；使用：跨区域或跨星球布局；边界：非正上限采用 32，保持原方案边界和总量，取消或能力不符时不返回部分结果。
// 逻辑流：1. 确认需要切分 => 2. 验证原配置并分配整数机器和净流 => 3. 按整数机器数切分可信额定能力 => 4. 更新流图及已有统计的生产下标。
func SplitProductions(ctx context.Context, plan quant.Result, maxMachines int) (quant.Result, error) {
	if err := ctx.Err(); err != nil {
		return quant.Result{}, err
	}
	if maxMachines <= 0 {
		maxMachines = 32
	}
	changed := false
	for _, production := range plan.Productions {
		if production.MachineCount > maxMachines {
			changed = true
			break
		}
	}
	if !changed {
		return plan, nil
	}
	dataset, err := quant.LoadDataset(plan.Dataset)
	if err != nil {
		return quant.Result{}, err
	}
	result := plan
	result.Productions = nil
	for index, production := range plan.Productions {
		if err := ctx.Err(); err != nil {
			return quant.Result{}, err
		}
		if production.MachineCount <= maxMachines {
			result.Productions = append(result.Productions, production)
			continue
		}
		capacity, err := quant.ValidatedProductionCapacity(dataset, production)
		if err != nil {
			return quant.Result{}, fmt.Errorf("产线 %d 无法切分：%w", index, err)
		}
		var exact, output, power float64
		inputRates, outputRates := make([]float64, len(production.Inputs)), make([]float64, len(production.Outputs))
		for allocated := 0; allocated < production.MachineCount; {
			if err := ctx.Err(); err != nil {
				return quant.Result{}, err
			}
			count := min(maxMachines, production.MachineCount-allocated)
			last := allocated+count == production.MachineCount
			fraction := float64(count) / float64(production.MachineCount)
			// 尾块取原值减已分配值，避免多块重复乘除令原料或产量累计漂移。
			take := func(total float64, used *float64) float64 {
				value := total * fraction
				if last {
					value = total - *used
				}
				*used += value
				return value
			}
			part := production
			part.MachineCount = count
			part.ExactMachineCount = take(production.ExactMachineCount, &exact)
			part.OutputCount = take(production.OutputCount, &output)
			// 满载浮点乘除可能多出一个舍入位；不得让等比例切片超过原型整数能力。
			fullOutput := capacity.OutputPerMachinePerMinute * float64(count)
			if fullOutput/capacity.OutputPerMachinePerMinute > float64(count) {
				fullOutput = math.Nextafter(fullOutput, 0)
			}
			if part.OutputCount > fullOutput {
				output -= part.OutputCount - fullOutput
				part.OutputCount = fullOutput
			}
			part.PowerMW = take(production.PowerMW, &power)
			part.RoundingMargin = float64(count) - part.ExactMachineCount
			part.Inputs = append([]quant.ItemRate(nil), production.Inputs...)
			part.Outputs = append([]quant.ItemRate(nil), production.Outputs...)
			for i := range part.Inputs {
				part.Inputs[i].RatePerMinute = take(production.Inputs[i].RatePerMinute, &inputRates[i])
			}
			for i := range part.Outputs {
				part.Outputs[i].RatePerMinute = take(production.Outputs[i].RatePerMinute, &outputRates[i])
			}
			// 父线已经独立按原型验证；额定流只随整数台数缩放，不重复为每块构建整张量化图。
			scaleRates := func(rates []quant.ItemRate) []quant.ItemRate {
				out := append([]quant.ItemRate(nil), rates...)
				for i := range out {
					out[i].RatePerMinute *= fraction
				}
				return out
			}
			blockCapacity := *capacity
			blockCapacity.OutputPerMinute *= fraction
			blockCapacity.PowerMW *= fraction
			blockCapacity.Utilization = part.ExactMachineCount / float64(count)
			blockCapacity.Inputs = scaleRates(capacity.Inputs)
			blockCapacity.Outputs = scaleRates(capacity.Outputs)
			blockCapacity.GrossInputs = scaleRates(capacity.GrossInputs)
			blockCapacity.GrossOutputs = scaleRates(capacity.GrossOutputs)
			part.Capacity = &blockCapacity
			result.Productions = append(result.Productions, part)
			allocated += count
		}
	}
	graph, err := quant.BuildFlowGraphContext(ctx, result)
	if err != nil {
		return quant.Result{}, err
	}
	result.FlowGraph = &graph
	if plan.Statistics != nil {
		result.Statistics, err = quant.BuildStatistics(result)
		if err != nil {
			return quant.Result{}, err
		}
	}
	return result, nil
}
