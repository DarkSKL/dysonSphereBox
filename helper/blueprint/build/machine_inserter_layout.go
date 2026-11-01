package build

// 文件职责：把机器端口方案落实为分拣器实体，并从实际连接重新核验吞吐。
// 使用：buildLine 收尾与实体回归；边界：容量不足、错误连接、重用槽位和错误等级均失败关闭。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// buildMachineInserterPort 职责：将同一物料的每根并联臂连接到不同真实机器槽；使用：基础机器行；边界：所有臂使用用户选定等级和同一物料带，不复制流量或借用其它物料的槽。
func (b *builder) buildMachineInserterPort(machine *placed, belt []*placed, port machineInserterPort, conf geometry, machineIndex int, spacing, pre float64, output bool, filter int) {
	for _, slot := range port.slots {
		arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
		arm.record.InputFromSlot = -1
		arm.record.OutputToSlot = -1
		arm.record.Parameters = []int32{int32(port.reach)}
		arm.record.FilterID = int16(filter)
		px := machine.a.x - conf.Left + float64(slot)*(1-conf.Outside) + conf.Outside
		beltPoint := belt[machineIndex*int(spacing)+int(conf.Start)+slot+int(pre)]
		machinePoint := vector{px, machine.a.y - conf.Bottom + conf.Lack, 0}
		outsidePoint := vector{px, machine.a.y - conf.Bottom - float64(port.reach), 0}
		machineSlot := slot
		arm.record.Yaw, arm.record.Yaw2 = 0, 0
		if port.top {
			machineSlot += 3
			machinePoint.y = machine.a.y + conf.Top - conf.Lack
			outsidePoint.y = machine.a.y + conf.Top + float64(port.reach)
			arm.record.Yaw, arm.record.Yaw2 = 180, 180
		}
		arm.input, arm.output = beltPoint, machine
		arm.record.OutputToSlot = conf.Slots[machineSlot]
		arm.a, arm.b = outsidePoint, machinePoint
		if output {
			arm.input, arm.output = machine, beltPoint
			arm.record.InputFromSlot, arm.record.OutputToSlot = conf.Slots[machineSlot], -1
			arm.a, arm.b = machinePoint, outsidePoint
			arm.record.Yaw, arm.record.Yaw2 = 0, 0
			if !port.top {
				arm.record.Yaw, arm.record.Yaw2 = 180, 180
			}
		}
	}
}

// validateLineMachineInserters 职责：独立从已创建臂、实际机器堆叠和物料带重建容量；使用：装配收尾与测试；边界：不把纯规划结果当作实体证据，最终槽位姿态和动态碰撞仍由统一规则核验。
func validateLineMachineInserters(l *line, setting Setting) error {
	rows := l.outputRows()
	if len(l.inputs) != len(l.recipe.Items) || len(rows) != len(l.recipe.Results) || len(l.machines) == 0 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器端口缺少配方物料带或真实机器"}
	}
	machineSet := make(map[*placed]bool, len(l.machines))
	for _, machine := range l.machines {
		machineSet[machine] = true
	}
	stacks := map[*placed]int{}
	for _, machine := range l.machines {
		root := machine
		for steps := 0; root.input != nil && machineSet[root.input]; steps++ {
			if steps >= len(l.machines) {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "研究站堆叠连接形成循环"}
			}
			root = root.input
		}
		stacks[root]++
	}
	inputBelts := map[*placed]int{}
	for index, row := range l.inputs {
		if len(row) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器端口缺少真实原料带"}
		}
		for _, belt := range row {
			if _, exists := inputBelts[belt]; exists {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "多个配方原料引用同一条物料带"}
			}
			inputBelts[belt] = index
		}
	}
	outputBelts := map[*placed]int{}
	for index, row := range rows {
		if len(row) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器端口缺少逐产物独立出货带"}
		}
		for _, belt := range row {
			_, reusedOutput := outputBelts[belt]
			_, reusedInput := inputBelts[belt]
			if reusedOutput || reusedInput {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "不同产物或原料端口引用了同一条实体带"}
			}
			outputBelts[belt] = index
		}
	}
	capacities := map[*placed][]float64{}
	occupied := map[*placed]map[int8]bool{}
	for machine := range stacks {
		capacities[machine] = make([]float64, len(l.inputs)+len(rows))
		occupied[machine] = map[int8]bool{}
	}
	for _, arm := range l.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
			continue
		}
		inputMachine, outputMachine := machineSet[arm.input], machineSet[arm.output]
		if !inputMachine && !outputMachine {
			continue
		}
		if inputMachine && outputMachine || arm.record.ItemID != int16(2010+setting.InserterLv) || len(arm.record.Parameters) != 1 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器分拣器方向、等级或跨度参数与所选设置不符"}
		}
		machine, slot, port := arm.output, arm.record.OutputToSlot, 0
		if inputMachine {
			index, exists := outputBelts[arm.output]
			machine, slot, port = arm.input, arm.record.InputFromSlot, len(l.inputs)+index
			if !exists || len(rows) > 1 && arm.record.FilterID != int16(l.recipe.Results[index]) || len(rows) == 1 && arm.record.FilterID != 0 && arm.record.FilterID != int16(l.recipe.Results[index]) {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器出货分拣器没有连接本行实际出货带"}
			}
		} else {
			var exists bool
			port, exists = inputBelts[arm.input]
			if !exists || arm.record.FilterID != 0 && arm.record.FilterID != int16(l.recipe.Items[port]) {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器原料分拣器连接了错误物料带或过滤物品"}
			}
		}
		if stacks[machine] == 0 || slot < 0 || occupied[machine][slot] {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器分拣器使用非地面机器、无效或重复的槽位"}
		}
		occupied[machine][slot] = true
		reach := int(arm.record.Parameters[0])
		observedReach := int(math.Round(math.Hypot(arm.b.x-arm.a.x, arm.b.y-arm.a.y)))
		if reach != observedReach || reach < 1 || reach > 3 || math.Abs(arm.a.z-arm.b.z) > 1e-6 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器分拣器跨度参数与实际端点不一致"}
		}
		capacities[machine][port] += inserterCapacityPerMinute(setting.InserterLv, reach)
	}
	inputs, outputs, err := machineInserterRates(l.info, l.recipe)
	if err != nil {
		return err
	}
	for _, machine := range l.machines {
		layers := stacks[machine]
		if layers == 0 {
			continue
		}
		for _, capacity := range capacities[machine] {
			if capacity <= 0 {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器缺少某种配方物料与实际独立带之间的实体分拣器"}
			}
		}
		for port, input := range inputs {
			rate := input * float64(layers)
			if rate > capacities[machine][port]+1e-6 {
				return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 原料 %d 实际 %d 层机器端口需求 %.3f/min，所选 Mk.%d 实体分拣器合计能力 %.3f/min", l.recipe.ID, l.recipe.Items[port], layers, rate, setting.InserterLv, capacities[machine][port])}
			}
		}
		for index, output := range outputs {
			port := len(inputs) + index
			if rate := output * float64(layers); rate > capacities[machine][port]+1e-6 {
				return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 产物 %d 实际 %d 层机器输出需求 %.3f/min，所选 Mk.%d 实体分拣器合计能力 %.3f/min", l.recipe.ID, l.recipe.Results[index], layers, rate, setting.InserterLv, capacities[machine][port])}
			}
		}
	}
	return nil
}

// checkLineMachineInserters 职责：要求机器行实际分拣能力覆盖物料毛流；使用：每个布局候选内的实际机器行；边界：容量不足硬失败，不降级为可导出告警，成功检查仍不代表游戏内连续产量。
func (b *builder) checkLineMachineInserters(l *line) error {
	if err := validateLineMachineInserters(l, b.setting); err != nil {
		return err
	}
	if b.logistics == nil {
		return nil
	}
	message := "已按真实机器堆叠、独立物料带、不同机器槽位和实际并联分拣器累计保守能力；未升级所选等级，不假定集装或科技加成"
	for _, rule := range []string{"machine-inserter-capacity", "inserter-capacity"} {
		found := false
		for index := range b.logistics.Checks {
			check := &b.logistics.Checks[index]
			if check.RuleID != rule {
				continue
			}
			found = true
			check.Status, check.Message = "passed", message
		}
		if !found {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: rule, Status: "passed", Message: message})
		}
	}
	return nil
}

// containsExact 职责：判断兼容诊断是否已记录；使用：机器回收和布局告警去重；边界：只匹配完整文本，不吞并不同原因。
func containsExact(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
