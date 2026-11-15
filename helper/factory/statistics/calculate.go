package statistics

// 文件职责：从已守恒量化数据汇总物料、建筑和能量；使用：量化适配器；边界：不求解配方、不反写原方案、不假设燃料全部燃烧。

import (
	"DysonSphereBox/helper/factory/statistics/facts"
	"sort"
)

// Number 职责：为可知数值建立独立指针；使用：区分零与未知；边界：调用方不共享可变缓存。
func Number(v float64) *float64 { return &v }

// AddPower 职责：累加同口径功率并维护覆盖；使用：分类和全厂；边界：任一负载或原型未知则不提供总预测。
func AddPower(total *Power, value Power) {
	total.IdleMW += value.IdleMW
	total.BaseWorkMW += value.BaseWorkMW
	total.ConfiguredWorkMW += value.ConfiguredWorkMW
	total.KnownExpectedMW += value.KnownExpectedMW
	total.UnknownCount += value.UnknownCount
	total.MissingCount += value.MissingCount
	total.ExpectedMW = nil
	if total.UnknownCount == 0 && total.MissingCount == 0 {
		total.ExpectedMW = Number(total.KnownExpectedMW)
	}
}

// Category 职责：按原版原型划分设施种类；使用：分类图表；边界：未知保留other，不能误算成制造设施。
func Category(id int) string {
	switch {
	case id >= 2001 && id <= 2003:
		return "belt"
	case id >= 2011 && id <= 2014:
		return "inserter"
	case id == 2020:
		return "splitter"
	case id == 2030:
		return "monitor"
	case id == 2040:
		return "piler"
	case id == 2313:
		return "spray-coater"
	case id == 2101 || id == 2102 || id == 2106:
		return "storage"
	case id == 2103 || id == 2104 || id == 2105 || id == 2107:
		return "logistics"
	case id >= 2201 && id <= 2213:
		return "power"
	case id == 2901 || id == 2902:
		return "lab"
	case id >= 2302 && id <= 2319:
		return "manufacturing"
	default:
		return "other"
	}
}

// LinePower 职责：计算按平均制造负载的工作/待机混合；使用：量化产线；边界：满载功率已包含增产，不再重复加成。
func LinePower(c *facts.Catalog, id, count int, utilization, configured float64) Power {
	p := Power{ConfiguredWorkMW: configured}
	f, ok := c.Lookup(id)
	if !ok || !f.PowerKnown || f.Power == nil {
		p.MissingCount = count
		return p
	}
	p.IdleMW = float64(f.Power.Idle) * .00006 * float64(count)
	p.BaseWorkMW = float64(f.Power.Work) * .00006 * float64(count)
	p.KnownExpectedMW = configured*utilization + p.IdleMW*(1-utilization)
	p.ExpectedMW = Number(p.KnownExpectedMW)
	return p
}

// Sum 职责：按物品稳定汇总已求解数据；使用：Solve完成和实体统计前；边界：只计算已有口径，不把配置满产当期望外供。
// 逻辑流：1. 汇总净边界 => 2. 累加毛流与同线回流 => 3. 热值换算和余量标记 => 4. 制造设备分类。
func Sum(c *facts.Catalog, lines []Line, imports, targets, surplus []Rate) Report {
	r := Report{SchemaVersion: "factory-statistics/v1", Scope: "planned-manufacturing", FactsVersion: c.Version, Lines: append([]Line{}, lines...), Buildings: []BuildingGroup{}, Materials: []Material{}, Heatmap: []HeatmapEntry{}, ProductionLineCount: len(lines),
		Notes: []string{"所有速率单位为件/min，功率为MW；能力与能量均为静态预测。", "量化范围只包含制造设备；完整物流、缓存、监测和供电设施在生成后统计。", "制造设备按产线平均负载运行，平均功率=增产后工作功率×负载率+待机功率×(1-负载率)。", "逐线满载能力不代表全厂可同时满载；增产剂已计入毛输入，不能重复加成。", "热值通量以未喷涂物品热值换算，表示能量载体的进出，不表示已经燃烧或建筑散热；散热数据为空。"}}
	materials := map[int]*Material{}
	get := func(id int, name string) *Material {
		if m := materials[id]; m != nil {
			return m
		}
		m := &Material{ItemID: id, Name: name}
		if f, ok := c.Lookup(id); ok {
			m.HeatValueMJ = Number(float64(f.HeatValue) / 1e6)
		}
		materials[id] = m
		return m
	}
	for _, v := range imports {
		get(v.ItemID, v.Name).Imported += v.RatePerMinute
	}
	for _, v := range targets {
		get(v.ItemID, v.Name).Target += v.RatePerMinute
	}
	for _, v := range surplus {
		get(v.ItemID, v.Name).Surplus += v.RatePerMinute
	}
	groups := map[int]*BuildingGroup{}
	for i := range r.Lines {
		line := &r.Lines[i]
		line.Thermal = Thermal{}
		inputs := map[int]float64{}
		for _, v := range line.NetInputs {
			get(v.ItemID, v.Name).Consumed += v.RatePerMinute
		}
		for _, v := range line.NetOutputs {
			get(v.ItemID, v.Name).Produced += v.RatePerMinute
		}
		for _, v := range line.Inputs {
			m := get(v.ItemID, v.Name)
			m.GrossInput += v.RatePerMinute
			inputs[v.ItemID] += v.RatePerMinute
			if m.HeatValueMJ != nil {
				line.Thermal.InputPotentialMW += v.RatePerMinute * *m.HeatValueMJ / 60
			}
		}
		for _, v := range line.Outputs {
			m := get(v.ItemID, v.Name)
			m.GrossOutput += v.RatePerMinute
			m.LocalRecycle += min(inputs[v.ItemID], v.RatePerMinute)
			if m.HeatValueMJ != nil {
				line.Thermal.OutputPotentialMW += v.RatePerMinute * *m.HeatValueMJ / 60
			}
		}
		for _, id := range line.JointProducts {
			get(id, "").JointProduct = true
		}
		g := groups[line.MachineID]
		if g == nil {
			g = &BuildingGroup{ItemID: line.MachineID, Name: line.MachineName, Category: Category(line.MachineID), Utilization: Number(0)}
			groups[line.MachineID] = g
		}
		g.Count += line.Count
		g.Inputs = append(g.Inputs, line.Inputs...)
		g.Outputs = append(g.Outputs, line.Outputs...)
		g.ConfiguredInputs = append(g.ConfiguredInputs, line.ConfiguredInputs...)
		g.ConfiguredOutputs = append(g.ConfiguredOutputs, line.ConfiguredOutputs...)
		*g.Utilization += line.Utilization * float64(line.Count)
		AddPower(&g.Power, line.Power)
		AddPower(&r.Power, line.Power)
		g.Thermal.InputPotentialMW += line.Thermal.InputPotentialMW
		g.Thermal.OutputPotentialMW += line.Thermal.OutputPotentialMW
		r.ProductionMachineCount += line.Count
	}
	r.BuildingCount = r.ProductionMachineCount
	for _, m := range materials {
		m.BalanceResidual = m.Imported + m.Produced - m.Consumed - m.Target - m.Surplus
		m.HasSurplus = m.Surplus > 1e-8
		if m.HasSurplus {
			r.HasSurplus = true
			r.SurplusItemCount++
		}
		if m.HeatValueMJ != nil {
			m.ImportPotentialMW = m.Imported * *m.HeatValueMJ / 60
			m.ExportPotentialMW = (m.Target + m.Surplus) * *m.HeatValueMJ / 60
		}
		r.Thermal.InputPotentialMW += m.ImportPotentialMW
		r.Thermal.OutputPotentialMW += m.ExportPotentialMW
		r.Materials = append(r.Materials, *m)
	}
	for _, g := range groups {
		*g.Utilization /= float64(g.Count)
		g.Inputs = MergeRates(g.Inputs)
		g.Outputs = MergeRates(g.Outputs)
		g.ConfiguredInputs = MergeRates(g.ConfiguredInputs)
		g.ConfiguredOutputs = MergeRates(g.ConfiguredOutputs)
		r.Buildings = append(r.Buildings, *g)
	}
	sort.Slice(r.Buildings, func(i, j int) bool { return r.Buildings[i].ItemID < r.Buildings[j].ItemID })
	sort.Slice(r.Materials, func(i, j int) bool { return r.Materials[i].ItemID < r.Materials[j].ItemID })
	if r.Power.ExpectedMW == nil && r.Power.MissingCount == 0 && r.Power.UnknownCount == 0 {
		r.Power.ExpectedMW = Number(0)
	}
	return r
}

// MergeRates 职责：按ID稳定合并多条产线毛流；使用：建筑类型产能；边界：不抵消同名输入和输出。
func MergeRates(input []Rate) []Rate {
	values := map[int]Rate{}
	for _, v := range input {
		current := values[v.ItemID]
		current.ItemID = v.ItemID
		current.Name = v.Name
		current.RatePerMinute += v.RatePerMinute
		values[v.ItemID] = current
	}
	out := make([]Rate, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}
