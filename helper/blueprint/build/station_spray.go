package build

// 文件职责：验证增产剂生产线从自身供货塔库存回送的毛流与实体来源。
// 使用：mode3/4 最终塔链核验；边界：不改规范净流，不把启动库存或机队吞吐当作已验证。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// validateStationSprayRecycle 职责：独立恢复自产供剂率并核对同塔入货来源；使用：零净需求的供剂出带；边界：没有可信毛流、完整同组机器或真实产物带即失败，容量仍受所选带等级约束。
func (b *builder) validateStationSprayRecycle(link *stationTransportLink) error {
	l := link.sprayRecycle
	failure := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("产线 %d 自产增产剂 %d 的塔内回送无效：%s", link.owner, link.item, reason)}
	}
	if l == nil || link.output || link.rate != 0 || l.coaterInput != link.head || productionSprayItem(l.info) != link.item || l.info.MachineCount <= 0 || len(l.machines) == 0 || len(l.coaters) == 0 {
		return failure("供剂端点、喷涂配置或机器行不匹配")
	}
	capacity := l.info.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || !finitePositive(capacity.OutputPerMinute) || !finitePositive(l.info.OutputCount) {
		return failure("缺少可信配置毛流")
	}
	if productionItemRate(l.info.Inputs, link.item) != 0 || !finitePositive(productionItemRate(l.info.Outputs, link.item)) {
		return failure("自产物品必须只有规范净产出")
	}
	if _, err := productionMaterialFlows(l.info, l.recipe); err != nil {
		return err
	}
	share := float64(len(l.machines)) / float64(l.info.MachineCount)
	scale := l.info.OutputCount / capacity.OutputPerMinute * share
	input := productionItemRate(capacity.GrossInputs, link.item) * scale
	output := productionItemRate(capacity.GrossOutputs, link.item) * scale
	net := productionItemRate(l.info.Outputs, link.item) * share
	if !finitePositive(input) || !finitePositive(output) || math.Abs(output-input-net) > machineMaterialRateTolerance(output-input, net, output) {
		return failure("供剂毛输入与产物毛输出不满足净差")
	}
	if input > b.logistics.BeltCapacityPerMinute+1e-6 || output > b.logistics.BeltCapacityPerMinute+1e-6 {
		return failure("自产入塔或供剂出塔毛流超过带容量")
	}
	for _, machine := range l.machines {
		if machine == nil || machine.owner != link.owner {
			return failure("机器行归属不一致")
		}
	}
	head := l.outputForItem(link.item)
	if len(head) == 0 {
		return failure("缺少独立产物带")
	}
	source := false
	for _, other := range b.stationLinks {
		if other.output && other.owner == link.owner && other.item == link.item && other.tower == link.tower && other.head == head[0] && math.Abs(other.rate-net) <= machineMaterialRateTolerance(other.rate, net, net) {
			source = true
		}
	}
	if !source {
		return failure("没有本机器行抵达同一库存槽的真实产物入塔链")
	}
	message := fmt.Sprintf("产线 %d 的增产剂 %d 从本行产物入塔后经真实供货槽出带回送喷涂机；本行入塔毛流 %.6f/min、供剂 %.6f/min、规范净产出 %.6f/min。未新增外供；启动需种子增产剂，持续自用与供货时序未在游戏验证", link.owner, link.item, output, input, net)
	if !containsExact(b.warnings, message) {
		b.warnings = append(b.warnings, message)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "station-local-spray-recycle", Status: "passed", Message: message})
	return nil
}
