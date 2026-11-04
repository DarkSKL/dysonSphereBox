package build

// 文件职责：将公共输入用途份额绑定到逐机器组的真实配送端口。
// 使用：普通与密铺配送共同调用；边界：不重复分配聚合物料净流，不自行推导喷涂消耗。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// flyPortRate 职责：给真实配送端点分配其角色的计划份额；使用：同物品配方/供剂两个入口；边界：输出仍沿用净出货逻辑，自产供剂另由真实回流处理。
func flyPortRate(p quant.Production, r quant.Recipe, port flyIndependentPort, count int) (float64, error) {
	if !port.input {
		return flyLineRate(p, port.item, false, count), nil
	}
	if p.MachineCount <= 0 || count < 1 || count > p.MachineCount {
		return 0, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "配送用途份额缺少可信机器数量"}
	}
	total, err := productionRoleInputRate(p, r, port.item, port.role)
	if err != nil {
		return 0, err
	}
	return total * float64(count) / float64(p.MachineCount), nil
}
