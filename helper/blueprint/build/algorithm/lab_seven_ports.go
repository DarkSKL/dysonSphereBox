package build

// 文件职责：为六原料研究站提供第三侧的独立产物口。
// 使用：宇宙矩阵的行预算、分拣器规划和装配；边界：每个单元只含一列堆叠，不以混带或虚拟槽位替代实体。

import (
	"DysonSphereBox/helper/quant"
)

// labSevenPorts 职责：限定已覆盖的原版研究站七物料结构；使用：端口门禁与规划；边界：其它多物料机器仍须独立实现。
func labSevenPorts(p quant.Production, r quant.Recipe) bool {
	return p.MachineType == 6 && (p.MachineID == 2901 || p.MachineID == 2902) && len(r.Items) == 6 && len(r.Results) == 1
}

// planLabSevenInserters 职责：保留南北六原料口并从西侧出货；使用：通用规划前；边界：逐层核算所选分拣器能力，不提升等级。
func planLabSevenInserters(p quant.Production, r quant.Recipe, s Setting, count int) machineInserterPlan {
	plan := orderedMachineInserters(6, 0, max(1, min(count, s.MaxLays)), 3, []int{0, 1, 2, 3, 4, 5})
	plan.outputs = []machineInserterPort{{reach: 1, slots: []int{1}, lateral: true}}
	for plan.layers > 1 && machineInserterPlanError(p, r, s, plan) != nil {
		plan.layers--
	}
	plan.capacityError = machineInserterPlanError(p, r, s, plan)
	return plan
}

// buildLabSideOutput 职责：将西侧中槽4接到独立的西向出货带；使用：单列研究站装配；边界：三维槽位与货物采样仍交统一终检。
func (b *builder) buildLabSideOutput(machine *placed, belt []*placed) {
	arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
	arm.a, arm.b = vector{machine.a.x - 1.8, machine.a.y, 0}, vector{machine.a.x - 3, machine.a.y, 0}
	arm.input, arm.output = machine, belt[len(belt)-2]
	arm.record.InputFromSlot, arm.record.OutputToSlot = 4, -1
	arm.record.Yaw, arm.record.Yaw2 = 270, 270
	arm.record.Parameters = []int32{1}
}
