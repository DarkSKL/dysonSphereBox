package build

// 文件职责：在最终存活记录上证明所请求的机器物料监测及直连外供监测均有真实宿主。
// 使用：前四种模式编码前共同终检；边界：均不启用报警，普通监测器保持默认，mode1独立交付监测另核整组净产量期望，允许合法同段复用。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// validateFinalMonitorIntegrity 职责：拒绝缺失监测器、错误宿主/朝向、物料串带和未请求的过滤或阈值配置；使用：物料守恒及最终记录完成之后；边界：不补建实体，零需求不制造监测要求。
func (b *builder) validateFinalMonitorIntegrity(ctx context.Context, req Request, records []blueprint.Building) error {
	// 以原请求为准；局部构造临时关闭开关不能撤销用户已经请求的最终监测证明。
	if !req.Setting.EnableFlowMonitor && !b.setting.EnableFlowMonitor || req.Mode < 1 || req.Mode > 4 {
		return nil
	}
	fail := func(message string) error {
		return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: message}
	}
	if len(records) != len(b.all) {
		return fail("最终监测核验的实体与记录数量不一致")
	}
	present := map[*placed]bool{}
	indices := map[*placed]int{}
	beltsAt := map[vector][]*placed{}
	monitors := []*placed{}
	incoming := map[*placed]int{}
	for i, entity := range b.all {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终监测器宿主核验已取消或超时"}
		}
		present[entity], indices[entity] = true, i
		if surfaceIsBelt(entity.record.ItemID) {
			beltsAt[entity.a] = append(beltsAt[entity.a], entity)
			if entity.output != nil {
				incoming[entity.output]++
			}
		}
		if entity.record.ItemID == 2030 {
			monitors = append(monitors, entity)
		}
	}
	covered := map[*placed]bool{}
	for _, monitor := range monitors {
		record := records[indices[monitor]]
		// 普通监测保持默认；交付组仅允许独立重算的一分钟净产量期望，不能借此夹带筛选、报警或沙盒配置。
		if record.FilterID != 0 || record.RecipeID != 0 || monitor.deliveryMonitor == nil && len(record.Parameters) != 0 || record.InputObjIdx >= 0 || record.OutputObjIdx >= 0 {
			return fail(fmt.Sprintf("监测器%d的物料过滤、速率参数或连接与请求的默认监测配置不符", record.Index))
		}
		if monitor.deliveryMonitor != nil {
			if err := b.validateMode1DeliveryMonitorRecord(monitor, record); err != nil {
				return err
			}
		}
		var selected *placed
		for _, host := range beltsAt[monitor.a] {
			if !boundaryFlowMonitorMatches(host, monitor) {
				continue
			}
			belt := records[indices[host]]
			if record.AreaIndex != belt.AreaIndex || record.LocalOffset != belt.LocalOffset || record.LocalOffset2 != belt.LocalOffset || math.Abs(math.Remainder(float64(record.Yaw-belt.Yaw), 360)) >= 1e-3 || math.Abs(math.Remainder(float64(record.Yaw2-belt.Yaw), 360)) >= 1e-3 {
				continue
			}
			if selected != nil {
				return fail(fmt.Sprintf("监测器%d对应多个实际带节点，无法证明宿主身份", record.Index))
			}
			selected = host
		}
		if selected == nil {
			return fail(fmt.Sprintf("监测器%d缺少最终同区、同点、同向且无额外连接的真实宿主带", record.Index))
		}
		if monitor.flowMonitorHost != nil && monitor.flowMonitorHost != selected {
			return fail(fmt.Sprintf("监测器%d的最终宿主与原始物料带不一致", record.Index))
		}
		if covered[selected] {
			return fail(fmt.Sprintf("监测器%d与另一监测器重复占据同一真实宿主", record.Index))
		}
		covered[selected] = true
	}
	type requirement struct {
		hosts []*placed
		item  int
		rate  float64
		label string
	}
	requirements := []requirement{}
	items := map[*placed]map[int]bool{}
	mark := func(nodes []*placed, item int) {
		for _, node := range nodes {
			if !present[node] || !surfaceIsBelt(node.record.ItemID) {
				continue
			}
			if items[node] == nil {
				items[node] = map[int]bool{}
			}
			items[node][item] = true
		}
	}
	for _, l := range b.machineLines {
		if l == nil || len(l.machines) == 0 {
			continue
		}
		flows, err := productionMaterialFlows(l.info, l.recipe)
		if err != nil {
			return err
		}
		share := float64(len(l.machines)) / float64(l.info.MachineCount)
		for direction, rows := range [][][]*placed{l.inputs, l.outputRows()} {
			ids, rates, purpose := l.recipe.Items, flows.inputs, "原料"
			if direction == 1 {
				ids, rates, purpose = l.recipe.Results, flows.outputs, "产物"
			}
			for i, row := range rows {
				if i >= len(ids) || i >= len(rates) {
					return fail("最终机器监测的物料轨道与配方不一致")
				}
				rate := rates[i] * share
				if rate <= 0 {
					continue
				}
				if !finitePositive(rate) {
					return fail("最终机器监测缺少有限物料速率")
				}
				mark(row, ids[i])
				requirements = append(requirements, requirement{hosts: row, item: ids[i], rate: rate, label: fmt.Sprintf("配方%d%s", l.recipe.ID, purpose)})
			}
		}
	}
	for _, route := range b.transportRoutes {
		if route != nil {
			mark(route.path, route.edge.ItemID)
			mark(route.sharedPath, route.edge.ItemID)
		}
	}
	if req.Mode == 1 || b.directBoundaryUnits != nil {
		routes := b.uniqueMaterialBoundaryRoutes(b.denseBoundaryMonitorRoutes())
		for _, route := range routes {
			if route != nil {
				mark(route.path, route.edge.ItemID)
			}
		}
		for _, route := range routes {
			if route == nil || route.edge.FlowKind != "external-input" || route.rate <= 0 {
				continue
			}
			hosts, err := boundaryFlowMonitorHosts(ctx, route, present, incoming, items)
			if err != nil {
				return err
			}
			requirements = append(requirements, requirement{hosts: hosts, item: route.edge.ItemID, rate: route.rate, label: "独立外供入口"})
		}
	}
	if b.mode1Deliveries != nil {
		for _, group := range b.mode1Deliveries.groups {
			count := 0
			for _, monitor := range monitors {
				if monitor.flowMonitorHost == group.head || monitor.flowMonitorHost == group.end {
					count++
					if monitor.deliveryMonitor != group || monitor != group.monitor {
						return fail("合并交付尾带的监测器没有绑定完整交付组")
					}
				}
			}
			if count != 1 {
				return fail("每根合并交付尾带必须且只能有一个聚合期望监测器")
			}
			requirements = append(requirements, requirement{hosts: []*placed{group.head, group.end}, item: group.item, rate: group.planned, label: "独立合并交付口"})
		}
	}
	for _, demand := range requirements {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终监测物料覆盖检查已取消或超时"}
		}
		found := false
		for _, host := range demand.hosts {
			if present[host] && covered[host] && len(items[host]) == 1 && items[host][demand.item] {
				found = true
				break
			}
		}
		if !found {
			return fail(fmt.Sprintf("%s物品%d（计划%.6f/min）缺少存活且绑定正确物料宿主的流速监测器", demand.label, demand.item, demand.rate))
		}
	}
	if b.logistics != nil && len(requirements) > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-flow-monitor-coverage", Status: "passed", Message: fmt.Sprintf("最终%d条机器物料轨道/独立边界需求由存活监测器覆盖；已核对真实同区宿主、朝向及单物料身份，允许合法同段复用；合并交付另核整组净产量期望，均未启用报警", len(requirements))})
	}
	return nil
}
