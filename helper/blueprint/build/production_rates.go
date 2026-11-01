package build

// 文件职责：解释蓝图期望物流与整数设备配置能力的不同口径。
// 使用：所有模式在最终出口登记速率诊断；边界：不改规范流边，不把条件满负荷能力当作实测或整线物流保证。

import (
	"fmt"

	"DysonSphereBox/helper/quant"
)

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
	message := "机器行按单机满负荷毛流拆分带宽并核验分拣器；跨行与跨节点线路按期望净流量守恒规划，未证明所有节点可同时满负荷，也未提供实测产量"
	if missing > 0 {
		message += fmt.Sprintf("；%d 条旧产线缺少配置能力字段，需重新量化后查看双速率", missing)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "configured-full-load", Status: "unverified", Message: message})
}
