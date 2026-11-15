package quant

// 文件职责：将可信量化结果投影为独立统计契约；使用：Solve及蓝图统计重建；边界：不信任客户端Statistics，不修改原物料或能力。

import (
	"DysonSphereBox/helper/factory/statistics"
	"DysonSphereBox/helper/factory/statistics/facts"
	"fmt"
	"math"
)

// statisticsRates 职责：复制速率并按期望负载缩放；使用：净流/毛流适配；边界：不共用量化切片。
func statisticsRates(input []ItemRate, scale float64) []statistics.Rate {
	out := make([]statistics.Rate, 0, len(input))
	for _, v := range input {
		out = append(out, statistics.Rate{ItemID: v.ItemID, Name: v.Name, RatePerMinute: v.RatePerMinute * scale})
	}
	return out
}

// BuildStatistics 职责：从已验证量化字段重算分析报告；使用：算法数据接口；边界：缺少能力时按原型恢复，失败不返回伪造的零统计。
func BuildStatistics(plan Result) (*statistics.Report, error) {
	d, err := LoadDataset(plan.Dataset)
	if err != nil {
		return nil, err
	}
	c, err := facts.Load()
	if err != nil {
		return nil, err
	}
	lines := make([]statistics.Line, 0, len(plan.Productions))
	for index, p := range plan.Productions {
		capacity := p.Capacity
		if capacity == nil {
			capacity, err = ValidatedProductionCapacity(d, p)
			if err != nil {
				return nil, err
			}
		}
		if p.MachineCount <= 0 || capacity.OutputPerMinute <= 0 || p.OutputCount <= 0 {
			return nil, fmt.Errorf("产线%d缺少统计能力", index)
		}
		load := p.OutputCount / capacity.OutputPerMinute
		if math.IsNaN(load) || math.IsInf(load, 0) || load < 0 || load > 1+1e-10 || math.IsNaN(capacity.PowerMW) || math.IsInf(capacity.PowerMW, 0) || capacity.PowerMW < 0 {
			return nil, fmt.Errorf("产线%d统计能力包含无效数值", index)
		}
		item := d.ByID[p.MachineID]
		line := statistics.Line{ProductionIndex: index, RecipeID: p.RecipeID, MachineID: p.MachineID, MachineName: item.Name, Count: p.MachineCount, ExactCount: p.ExactMachineCount,
			Utilization: p.OutputCount / capacity.OutputPerMinute, ProliferatorMode: p.ProliferatorMode, Points: p.IncreasePoints, PowerMultiplier: 1,
			Inputs: statisticsRates(capacity.GrossInputs, p.OutputCount/capacity.OutputPerMinute), Outputs: statisticsRates(capacity.GrossOutputs, p.OutputCount/capacity.OutputPerMinute),
			NetInputs: statisticsRates(p.Inputs, 1), NetOutputs: statisticsRates(p.Outputs, 1), ConfiguredInputs: statisticsRates(capacity.GrossInputs, 1), ConfiguredOutputs: statisticsRates(capacity.GrossOutputs, 1), JointProducts: []int{}}
		line.Power = statistics.LinePower(c, p.MachineID, p.MachineCount, line.Utilization, capacity.PowerMW)
		if line.Power.BaseWorkMW > 0 {
			line.PowerMultiplier = capacity.PowerMW / line.Power.BaseWorkMW
		}
		if ri, ok := d.RecipeByID[p.RecipeID]; ok && len(d.Recipes[ri].Results) > 1 {
			line.JointProducts = append(line.JointProducts, d.Recipes[ri].Results...)
		}
		lines = append(lines, line)
	}
	targets := make([]statistics.Rate, 0, len(plan.Targets))
	for _, t := range plan.Targets {
		targets = append(targets, statistics.Rate{ItemID: t.ItemID, Name: d.ByID[t.ItemID].Name, RatePerMinute: t.RatePerMinute})
	}
	report := statistics.Sum(c, lines, statisticsRates(plan.RawInputs, 1), targets, statisticsRates(plan.Leftovers, 1))
	report.PlanID = plan.PlanID
	report.DataVersion = plan.DataVersion
	report.GameVersion = plan.GameVersion
	return &report, nil
}
