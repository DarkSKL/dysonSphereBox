package build

// 文件职责：解释蓝图期望物流与整数设备配置能力的不同口径。
// 使用：所有模式在最终出口登记速率诊断；边界：不改规范流边，不把条件满负荷能力当作实测或整线物流保证。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// normalizeProductionRateBasis 职责：在生成入口以原型重算值替换客户端能力及精确台数；使用：规范图和机器规划前；边界：复制产线切片，不修改调用方快照或期望物料账，旧快照无能力则不生成额定标记。
func normalizeProductionRateBasis(ctx context.Context, plan quant.Result, dataset *quant.Dataset) (quant.Result, error) {
	plan.Productions = append([]quant.Production(nil), plan.Productions...)
	for index, production := range plan.Productions {
		if ctx.Err() != nil {
			return quant.Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配置能力重算已取消或超时"}
		}
		capacity, err := quant.ValidatedProductionCapacity(dataset, production)
		if err != nil {
			return quant.Result{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: err.Error()}
		}
		if production.Capacity != nil {
			plan.Productions[index].Capacity = capacity
		}
		plan.Productions[index].ExactMachineCount = production.OutputCount / capacity.OutputPerMachinePerMinute
		plan.Productions[index].RoundingMargin = float64(production.MachineCount) - plan.Productions[index].ExactMachineCount
	}
	return plan, nil
}

// annotateLineProductionRates 职责：把机器行的配置毛流绑定到不可裁除的真实分拣器接点；使用：基础机器行装配收尾；边界：旧快照无能力时不伪造速率，不改期望供需或机器数。
func (b *builder) annotateLineProductionRates(l *line) error {
	p := l.info
	if p.Capacity == nil || p.Capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad {
		return nil
	}
	flows, err := productionMaterialFlows(p, l.recipe)
	if err != nil {
		return err
	}
	capacity, err := transport.BeltCapacityPerMinute(b.setting.BeltLv)
	if err != nil {
		return err
	}
	share := float64(len(l.machines)) / float64(p.MachineCount)
	for _, side := range []struct {
		output     bool
		items      []int
		rows       [][]*placed
		gross      []float64
		configured []quant.ItemRate
		net        []quant.ItemRate
	}{{false, l.recipe.Items, l.inputs, flows.inputs, p.Capacity.GrossInputs, p.Inputs}, {true, l.recipe.Results, l.outputs, flows.outputs, p.Capacity.GrossOutputs, p.Outputs}} {
		for index, item := range side.items {
			// 额定值直接从原型重算的满负荷毛流拆行，避免期望流先缩放再除精确台数把225舍入成225.00000000000003。
			configuredTotal, plannedNet := productionItemRate(side.configured, item), productionItemRate(side.net, item)
			if !side.output {
				configuredTotal, err = configuredRoleInputRate(p, l.recipe, item, recipeInputRole)
				if err != nil {
					return err
				}
				plannedNet, err = productionRoleInputRate(p, l.recipe, item, recipeInputRole)
				if err != nil {
					return err
				}
			}
			configured := configuredTotal / float64(p.MachineCount) * float64(len(l.machines))
			if !finitePositive(configured) || configured > capacity+1e-6 || configured > math.MaxInt32 {
				return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("配方 %d 物品 %d 机器带配置满负荷 %.6f/min 超过等级 %d 单带能力 %.6f/min，不能仅缩小标注", p.RecipeID, item, configured, b.setting.BeltLv, capacity)}
			}
			anchor := productionRateAnchor(l, side.rows[index], side.output)
			if anchor == nil || anchor.productionRate != nil {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 物品 %d 缺少独立真实机器接带格", p.RecipeID, item)}
			}
			direction := "input"
			if side.output {
				direction = "output"
			}
			anchor.productionRate = &ProductionRateAnnotation{
				ProductionIndex: anchor.owner, RecipeID: p.RecipeID, ItemID: item, Direction: direction,
				MachineCount: len(l.machines), BuildingIndex: -1,
				PlannedRatePerMinute: side.gross[index] * share, PlannedNetRatePerMinute: plannedNet * share,
				ConfiguredRatePerMinute: configured, DisplayedRatePerMinute: int(math.Ceil(configured)), Basis: quant.ProductionCapacityBasisConfiguredFullLoad,
			}
		}
	}
	return nil
}

// productionRateAnchor 职责：选择实际机器臂所连的整行带流入口或汇集端；使用：双速率标注；边界：不选可裁除空尾，不把中间局部流量误作整行接口。
func productionRateAnchor(l *line, row []*placed, output bool) *placed {
	if len(row) == 0 {
		return nil
	}
	machines := make(map[*placed]bool, len(l.machines))
	for _, machine := range l.machines {
		machines[machine] = true
	}
	connected := map[*placed]bool{}
	for _, entity := range l.all {
		if entity.record.ItemID < 2011 || entity.record.ItemID > 2014 {
			continue
		}
		if output && machines[entity.input] {
			connected[entity.output] = true
		} else if !output && machines[entity.output] {
			connected[entity.input] = true
		}
	}
	ascending := len(row) < 2 || row[0].output == row[1]
	var anchor *placed
	for _, belt := range row {
		if !connected[belt] {
			continue
		}
		anchor = belt
		if !output || !ascending {
			break
		}
	}
	return anchor
}

// finalizeProductionRateAnnotations 职责：在最终重排编号后写入真实带图标数字并导出双速率；使用：finish 编码前；边界：只改标记参数，规范 lane/外供/目标流量保持不变。
func (b *builder) finalizeProductionRateAnnotations(records []blueprint.Building) error {
	if b.logistics == nil || len(records) != len(b.all) {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "机器速率标注缺少最终物流摘要或编号表"}
	}
	b.logistics.RateAnnotations = []ProductionRateAnnotation{}
	for index, entity := range b.all {
		annotation := entity.productionRate
		if annotation == nil {
			continue
		}
		if !surfaceIsBelt(entity.record.ItemID) || !finitePositive(annotation.ConfiguredRatePerMinute) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器速率标注没有关联有效真实传送带"}
		}
		value := *annotation
		value.BuildingIndex = index
		entity.record.Parameters = []int32{int32(value.ItemID), int32(value.DisplayedRatePerMinute)}
		records[index].Parameters = append([]int32(nil), entity.record.Parameters...)
		b.logistics.RateAnnotations = append(b.logistics.RateAnnotations, value)
	}
	if len(b.logistics.RateAnnotations) > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "machine-rate-annotations", Status: "passed", Message: fmt.Sprintf("%d 个真实机器带接点按本行整数设备配置满负荷毛流标注，向上取整仅用于游戏数字显示；期望毛流与净流另列，外部边界仍按分配流量标注，不宣称全厂同时满负荷或实测产量", len(b.logistics.RateAnnotations))})
	}
	return nil
}

// recordProductionRateBasis 职责：在生成结果中保留双速率及未降速边界；使用：finish；边界：旧快照缺少能力时明确要求重新量化，不从期望值伪造满负荷数据。
func (b *builder) recordProductionRateBasis(plan quant.Result) {
	if b.logistics == nil || len(plan.Productions) == 0 {
		return
	}
	checks := b.logistics.Checks[:0]
	for _, check := range b.logistics.Checks {
		if check.RuleID != "machine-throttle" && check.RuleID != "configured-full-load" {
			checks = append(checks, check)
		}
	}
	b.logistics.Checks = checks
	missing := 0
	for _, production := range plan.Productions {
		capacity := production.Capacity
		if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad {
			missing++
			continue
		}
		if capacity.Utilization < 1 {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{
				RuleID: "machine-throttle", Status: "unverified",
				Message: fmt.Sprintf("配方 %d 主产物期望净速率 %.3f/min，%d 台设备的配置满负荷净能力 %.3f/min；设备未写入降速，实际产量取决于供料、电力和出货，不能由期望值或满负荷值代替游戏测产", production.RecipeID, production.OutputCount, production.MachineCount, capacity.OutputPerMinute),
			})
		}
	}
	message := "机器行按单机满负荷毛流拆分带宽、核验分拣器并标注机器带配置速率；跨行与跨节点线路及外部边界按期望净流量守恒规划，未证明所有节点可同时满负荷，也未提供实测产量"
	if missing > 0 {
		message += fmt.Sprintf("；%d 条旧产线缺少配置能力字段，需重新量化后查看双速率", missing)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "configured-full-load", Status: "unverified", Message: message})
}
