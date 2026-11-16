package build

// 文件职责：为每根合并交付带设置唯一监测器及整组净产量期望。
// 使用：mode1全部真实路线固定之后；边界：只对新交付口设置一分钟期望，其它机器/原料监测仍沿用默认合同，不开启报警或沙盒造物。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// mode1DeliveryMonitorParameters 职责：按已核对原生128参数协议配置一分钟整组期望；使用：创建及最终独立比对；边界：[2]以十分之一件、[3]以tick计，所有警告/沙盒/过滤字段保持零。
func mode1DeliveryMonitorParameters(group *mode1DeliveryGroup) []int32 {
	p := make([]int32, 128)
	p[2] = int32(math.Round(group.planned * 10))
	p[3] = 3600
	return p
}

// completeMode1DeliveryMonitors 职责：每根独立交付尾带只建立一个聚合期望监测器；使用：原料监测完成后、全场补电及最终规则之前；边界：候选点只使用预定尾带，不借其它来源的监测充数。
func (b *builder) completeMode1DeliveryMonitors(ctx context.Context, footprint *[2]vector) error {
	p := b.mode1Deliveries
	if p == nil || !b.setting.EnableFlowMonitor {
		return nil
	}
	for _, group := range p.groups {
		if e := ctx.Err(); e != nil {
			return e
		}
		if group.monitor != nil {
			if e := b.validateMode1DeliveryMonitorRecord(group.monitor, group.monitor.record); e != nil {
				return e
			}
			continue
		}
		selected := false
		for _, host := range []*placed{group.end, group.head} {
			if !b.flowMonitorNodeClear(host) {
				continue
			}
			begin := len(b.all)
			monitor := b.make(2030, host.a.x, host.a.y, host.a.z)
			monitor.flowMonitorHost = host
			monitor.deliveryMonitor = group
			monitor.layoutFootprint = footprint
			monitor.record.Yaw, monitor.record.Yaw2 = host.record.Yaw, host.record.Yaw
			monitor.record.Parameters = mode1DeliveryMonitorParameters(group)
			if e := b.coverBoundaryFlowMonitor(host); e != nil {
				b.all = b.all[:begin]
				continue
			}
			group.monitor = monitor
			selected = true
			break
		}
		if !selected {
			return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "独立合并交付尾带没有同时满足净空和供电的唯一监测位置"}
		}
	}
	return nil
}

// validateMode1DeliveryMonitorRecord 职责：证明监测器期望来自整个真实交付组而非首条lane；使用：最终记录和幂等创建核验；边界：任何过滤/告警/沙盒额外参数或错宿主均拒绝。
func (b *builder) validateMode1DeliveryMonitorRecord(monitor *placed, record blueprint.Building) error {
	fail := func() error {
		return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "合并交付监测器不是该组唯一实际宿主或聚合净产量参数不符"}
	}
	if monitor == nil || monitor.deliveryMonitor == nil || b.mode1Deliveries == nil {
		return fail()
	}
	g := monitor.deliveryMonitor
	known := false
	for _, group := range b.mode1Deliveries.groups {
		known = known || group == g
	}
	if !known || g.monitor != monitor || (monitor.flowMonitorHost != g.end && monitor.flowMonitorHost != g.head) || record.ItemID != 2030 || record.FilterID != 0 || record.RecipeID != 0 || record.InputObjIdx >= 0 || record.OutputObjIdx >= 0 {
		return fail()
	}
	expected := mode1DeliveryMonitorParameters(g)
	if len(record.Parameters) != len(expected) {
		return fail()
	}
	for i, v := range expected {
		if record.Parameters[i] != v {
			return fail()
		}
	}
	return nil
}
