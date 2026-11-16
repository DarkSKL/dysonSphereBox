package build

// 文件职责：按实际单机物料毛流和用户分拣器等级决定参考单臂拓扑是否有足够能力。
// 使用：选择参考密铺入口；边界：不改参考姿态、机器数量或等级，不能满足时交给现有真实并联臂布局。

import "DysonSphereBox/helper/quant"

// referenceSingleArmsSufficient 职责：验证参考单元每种物料的一根实际一格臂足以承载单机毛流；使用：参考布局选择；边界：来源三类臂的一格参数由独立回归锁定，不以净流或取整后平均流量缩小机器需求。
func referenceSingleArmsSufficient(production quant.Production, recipe quant.Recipe, level int) bool {
	inputs, outputs, err := machineInserterRates(production, recipe)
	if err != nil {
		return false
	}
	capacity := inserterCapacityPerMinute(level, 1)
	if !finitePositive(capacity) {
		return false
	}
	for _, rates := range [][]float64{inputs, outputs} {
		for _, rate := range rates {
			if rate > capacity+1e-6 {
				return false
			}
		}
	}
	return true
}
