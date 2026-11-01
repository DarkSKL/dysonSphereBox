package quant

// 文件职责：计算与期望净流分离的配置满负荷能力；使用：Solve 结果桥梁；边界：不重写供需图，不代替游戏运输和产能验收。

import (
	"fmt"
	"math"
)

// ProductionCapacityBasisConfiguredFullLoad 职责：标识配置推导的满负荷能力；使用：前后端能力来源说明；边界：不等于实测产量或整线已验证吞吐。
const ProductionCapacityBasisConfiguredFullLoad = "configured-full-load"

// machineOutputPerMinute 职责：取得所选设备的单机净产出；使用：精确台数和满负荷能力统一基准；边界：沿用无速度原型的倍率 1 兼容语义。
func machineOutputPerMinute(n *node) float64 {
	speed := n.factory.Speed
	if speed <= 0 {
		speed = 1
	}
	return n.rate * speed * 60
}

// requiredMachineCount 职责：按可证明覆盖需求的额定能力向上取整；使用：Solve 整数机器规划；边界：仅用整数能力乘积消除除法舍入噪声，不用显示精度或容差吞掉真实超额。
func requiredMachineCount(amount, singleOutput float64) (float64, int, error) {
	exact := amount / singleOutput
	if amount <= 0 || singleOutput <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.IsNaN(singleOutput) || math.IsInf(singleOutput, 0) || math.IsNaN(exact) || math.IsInf(exact, 0) {
		return 0, 0, qerror("Q_E_BAD_REQUEST", "设备配置满负荷速率超出计算范围")
	}
	if exact > 1000001 {
		return 0, 0, qerror("Q_E_BAD_REQUEST", "单条产线机器数超过 100 万，请拆分目标")
	}
	count := max(1, int(math.Ceil(exact)))
	if count > 1 && float64(count-1)*singleOutput >= amount {
		count--
	}
	if float64(count)*singleOutput < amount {
		count++
	}
	if count > 1000000 {
		return 0, 0, qerror("Q_E_BAD_REQUEST", "单条产线机器数超过 100 万，请拆分目标")
	}
	if exact > float64(count) {
		exact = float64(count)
	}
	return exact, count, nil
}

// productionCapacity 职责：从真实执行向量恢复整数机器毛流、净流和单机周期；使用：Solve 制造产线；边界：不从配方原始件数猜测自循环、Mod 或自定义喷涂效果。
func productionCapacity(d *Dataset, n *node, p Production) (*ProductionCapacity, error) {
	singleOutput := machineOutputPerMinute(n)
	count := float64(p.MachineCount)
	cycles := singleOutput / n.runNet
	power := 1.
	if n.scheme.ProductionMode > 0 {
		_, _, power = effects(n.scheme.IncreasePoints)
	}
	capacity := &ProductionCapacity{
		Basis:                     ProductionCapacityBasisConfiguredFullLoad,
		OutputPerMinute:           singleOutput * count,
		OutputPerMachinePerMinute: singleOutput,
		CyclesPerMinute:           cycles,
		GrossInputs:               []ItemRate{},
		GrossOutputs:              []ItemRate{},
		Utilization:               p.ExactMachineCount / count,
		PowerMW:                   count * n.factory.WorkEnergyPerTick * .00006 * power,
	}
	invalid := func() (*ProductionCapacity, error) {
		return nil, qerror("Q_E_BAD_REQUEST", fmt.Sprintf("配方 %d 的配置满负荷能力超出计算范围", p.RecipeID))
	}
	for _, value := range []float64{singleOutput, count, cycles, capacity.OutputPerMinute} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return invalid()
		}
	}
	for _, value := range []float64{capacity.Utilization, capacity.PowerMW} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return invalid()
		}
	}
	if capacity.Utilization > 1 {
		return invalid()
	}
	capacity.Inputs, capacity.Outputs = flows(d, n, capacity.OutputPerMinute)
	for _, vector := range []struct {
		rates *rates
		out   *[]ItemRate
	}{{n.runInputs, &capacity.GrossInputs}, {n.runOutputs, &capacity.GrossOutputs}} {
		for _, name := range vector.rates.keys() {
			rate := vector.rates.values[name] * cycles * count
			if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
				return invalid()
			}
			if rate > 0 {
				*vector.out = append(*vector.out, ItemRate{ItemID: d.ByName[name].ID, Name: name, RatePerMinute: rate})
			}
		}
	}
	for _, vector := range [][]ItemRate{capacity.Inputs, capacity.Outputs} {
		for _, item := range vector {
			if math.IsNaN(item.RatePerMinute) || math.IsInf(item.RatePerMinute, 0) {
				return invalid()
			}
		}
	}
	return capacity, nil
}
