package statistics

// 文件职责：用最终实体补足设施数、电耗及热力图定位；使用：蓝图获选后一次调用；边界：不遍历搜索候选、不把排序器忙碌程度从制造负载强行推断。

import (
	"DysonSphereBox/helper/factory/statistics/facts"
	"context"
	"fmt"
	"sort"
)

// Entity 职责：承载统计所需的最小实体投影；使用：生成器适配；边界：生产关联必须来自真实机器绑定，其余为-1。
type Entity struct {
	Index, ItemID, ProductionIndex, AreaIndex int
	X, Y, Z                                   float64
}

// EnrichBlueprint 职责：替换制造预估数量为最终实体数量；使用：正式蓝图出口；边界：只修改本次新报告，不修改量化快照，保留未知功耗。
// 逻辑流：1. 核验机器绑定 => 2. 按原型计算额定设施功率 => 3. 填充已知负载和坐标 => 4. 汇总完整设施清单。
func EnrichBlueprint(ctx context.Context, r *Report, entities []Entity, space Space) error {
	c, err := facts.Load()
	if err != nil {
		return err
	}
	planned := map[int]BuildingGroup{}
	for _, g := range r.Buildings {
		planned[g.ItemID] = g
	}
	groups := map[int]*BuildingGroup{}
	machineCounts := make([]int, len(r.Lines))
	seen := map[int]bool{}
	r.Power = Power{}
	r.Thermal.RatedGenerationMW = 0
	r.Thermal.RatedFuelUseMW = 0
	r.Buildings = []BuildingGroup{}
	r.Heatmap = []HeatmapEntry{}
	for _, e := range entities {
		if err := ctx.Err(); err != nil {
			return err
		}
		if seen[e.Index] {
			return fmt.Errorf("统计建筑编号%d重复", e.Index)
		}
		seen[e.Index] = true
		f, known := c.Lookup(e.ItemID)
		known = known && f.PowerKnown
		g := groups[e.ItemID]
		if g == nil {
			g = &BuildingGroup{ItemID: e.ItemID, Name: f.Name, Category: Category(e.ItemID), Inputs: []Rate{}, Outputs: []Rate{}, ConfiguredInputs: []Rate{}, ConfiguredOutputs: []Rate{}}
			if p, ok := planned[e.ItemID]; ok {
				g.Name = p.Name
				g.Inputs = p.Inputs
				g.Outputs = p.Outputs
				g.ConfiguredInputs = p.ConfiguredInputs
				g.ConfiguredOutputs = p.ConfiguredOutputs
				g.Utilization = p.Utilization
			}
			groups[e.ItemID] = g
			if g.Name == "" {
				g.Name = fmt.Sprintf("物品 %d", e.ItemID)
			}
		}
		g.Count++
		power := Power{ExpectedMW: Number(0)}
		heat := Thermal{}
		var load *float64
		if !known {
			power.MissingCount = 1
			power.ExpectedMW = nil
		} else if f.Power != nil {
			if f.Power.Consumer {
				power.IdleMW = float64(f.Power.Idle) * .00006
				power.BaseWorkMW = float64(f.Power.Work) * .00006
				power.ConfiguredWorkMW = power.BaseWorkMW
				if power.BaseWorkMW > 0 || power.IdleMW > 0 {
					power.UnknownCount = 1
					power.ExpectedMW = nil
				}
			}
			heat.RatedGenerationMW = float64(f.Power.Generation) * .00006
			heat.RatedFuelUseMW = float64(f.Power.FuelUse) * .00006
		}
		if e.ProductionIndex >= 0 {
			if e.ProductionIndex >= len(r.Lines) {
				return fmt.Errorf("统计产线下标越界")
			}
			line := r.Lines[e.ProductionIndex]
			if line.MachineID != e.ItemID {
				return fmt.Errorf("统计制造设备绑定与产线不符")
			}
			machineCounts[e.ProductionIndex]++
			power = LinePower(c, e.ItemID, 1, line.Utilization, line.Power.ConfiguredWorkMW/float64(line.Count))
			load = Number(line.Utilization)
			heat.InputPotentialMW = line.Thermal.InputPotentialMW / float64(line.Count)
			heat.OutputPotentialMW = line.Thermal.OutputPotentialMW / float64(line.Count)
		} else if g.Utilization != nil {
			g.Utilization = nil
		}
		AddPower(&g.Power, power)
		AddPower(&r.Power, power)
		g.Thermal.InputPotentialMW += heat.InputPotentialMW
		g.Thermal.OutputPotentialMW += heat.OutputPotentialMW
		g.Thermal.RatedGenerationMW += heat.RatedGenerationMW
		g.Thermal.RatedFuelUseMW += heat.RatedFuelUseMW
		if e.ProductionIndex >= 0 || !known || power.ConfiguredWorkMW > 0 || power.IdleMW > 0 || heat.RatedGenerationMW > 0 {
			r.Heatmap = append(r.Heatmap, HeatmapEntry{BuildingIndex: e.Index, ItemID: e.ItemID, ProductionIndex: e.ProductionIndex, AreaIndex: e.AreaIndex, X: e.X, Y: e.Y, Z: e.Z,
				Utilization: load, ExpectedMW: power.ExpectedMW, ConfiguredWorkMW: power.ConfiguredWorkMW, Thermal: heat})
		}
	}
	for i, count := range machineCounts {
		if count != r.Lines[i].Count {
			return fmt.Errorf("产线%d统计机器数%d与量化%d不符", i, count, r.Lines[i].Count)
		}
	}
	for _, g := range groups {
		r.Buildings = append(r.Buildings, *g)
		r.Thermal.RatedGenerationMW += g.Thermal.RatedGenerationMW
		r.Thermal.RatedFuelUseMW += g.Thermal.RatedFuelUseMW
	}
	sort.Slice(r.Buildings, func(i, j int) bool { return r.Buildings[i].ItemID < r.Buildings[j].ItemID })
	sort.Slice(r.Heatmap, func(i, j int) bool { return r.Heatmap[i].BuildingIndex < r.Heatmap[j].BuildingIndex })
	r.BuildingCount = len(entities)
	r.Space = &space
	r.Scope = "generated-blueprint"
	r.Notes = append(r.Notes, "实体清单按实际生成原型计数，包括自动升级后的传送带和分拣器；不把产线产出的建筑物品算作已建设施。",
		"分拣器实际忙碌率、物流塔及配送器充电和运送功耗依赖运行状态/科技；提供原型额定值，未知运行负载的预测为null，KnownExpectedMW只含可估算部分。",
		"占地为最终Area局部格网的保守投影并集，不是碰撞体积或地表平方米；建筑占地排除带和分拣器，叠层只算一次。",
		"热力图制造负载按同产线各台均匀分配；其余用电设施负载未知。无耗电传送带不生成无意义热力图点。")
	return nil
}
