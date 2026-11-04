package build

// 文件职责：区分同一物品的配方消耗与喷涂供剂，保留规范输入总量和配置满负荷毛流两个口径。
// 使用：全部模式的真实输入端口、附件及机器分拣能力；边界：不把聚合需求重复分给两个角色，不修改量化计划。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// productionInputRole 职责：标识同物品的实际消耗位置；使用：角色速率选择；边界：配方口进入机器，供剂口进入喷涂通道。
type productionInputRole uint8

// 输入角色常量职责：区分两种实际消耗位置；使用：端口份额及能力核验；边界：不改变规范总流量。
const (
	// recipeInputRole 职责：标识机器配方原料；使用：原料口与分拣器容量；边界：不含喷涂通道耗剂。
	recipeInputRole productionInputRole = iota
	// sprayInputRole 职责：标识喷涂通道供剂；使用：供剂口份额；边界：即使物品相同也不混入机器配方需求。
	sprayInputRole
)

// productionRecipeInputCount 职责：返回一次配方执行对指定物品的原始计量；使用：分离供剂附加消耗；边界：沿用有效配方顺序，不按净流推测原料系数。
func productionRecipeInputCount(recipe quant.Recipe, item int) float64 {
	count := 0.
	for i, id := range recipe.Items {
		if id == item && i < len(recipe.ItemCounts) {
			count += recipe.ItemCounts[i]
		}
	}
	return count
}

// productionInputRoleExists 职责：拒绝把一个角色套到不使用该物品的端点；使用：两种速率入口；边界：不存在的有效角色返回零，未知角色由调用入口报错。
func productionInputRoleExists(p quant.Production, recipe quant.Recipe, item int, role productionInputRole) bool {
	if role == sprayInputRole {
		return item > 0 && item == productionSprayItem(p)
	}
	return productionRecipeInputCount(recipe, item) > 0
}

// configuredRoleInputRate 职责：返回整条产线配置满负荷时单个输入角色的毛流；使用：机器能力及真实附件带宽；边界：自产剂仍有正供剂毛流，调用者另外按机器组份额拆分。
func configuredRoleInputRate(p quant.Production, recipe quant.Recipe, item int, role productionInputRole) (float64, error) {
	fail := func(reason string) (float64, error) {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方%d物品%d输入角色无法拆分：%s", recipe.ID, item, reason)}
	}
	if role != recipeInputRole && role != sprayInputRole {
		return fail("未知输入用途")
	}
	if !productionInputRoleExists(p, recipe, item, role) {
		return 0, nil
	}
	capacity := p.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || p.MachineCount <= 0 || !finitePositive(capacity.CyclesPerMinute) {
		return fail("缺少可信满负荷执行次数")
	}
	total := productionItemRate(capacity.GrossInputs, item)
	if !finitePositive(total) {
		return fail("缺少可信满负荷毛输入")
	}
	count := productionRecipeInputCount(recipe, item)
	if item != productionSprayItem(p) || count <= 0 {
		return total, nil
	}
	raw := count * capacity.CyclesPerMinute * float64(p.MachineCount)
	spray := total - raw
	if !finitePositive(raw) || !finitePositive(spray) {
		return fail("配方计量与供剂附加消耗不能组成两条正毛流")
	}
	if role == recipeInputRole {
		return raw, nil
	}
	return spray, nil
}

// productionRoleInputRate 职责：返回整条产线规范净输入中某个真实用途的份额；使用：塔、配送和直连规范入口；边界：双用途份额之和必须仍等于原净输入，自产供剂净需求为零时不伪造外供。
func productionRoleInputRate(p quant.Production, recipe quant.Recipe, item int, role productionInputRole) (float64, error) {
	fail := func(reason string) (float64, error) {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方%d物品%d输入角色无法绑定：%s", recipe.ID, item, reason)}
	}
	if role != recipeInputRole && role != sprayInputRole {
		return fail("未知输入用途")
	}
	if !productionInputRoleExists(p, recipe, item, role) {
		return 0, nil
	}
	total := productionItemRate(p.Inputs, item)
	if total < 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return fail("规范输入不是有限非负数")
	}
	if item != productionSprayItem(p) || productionRecipeInputCount(recipe, item) <= 0 {
		return total, nil
	}
	raw, err := configuredRoleInputRate(p, recipe, item, recipeInputRole)
	if err != nil {
		return 0, err
	}
	spray, err := configuredRoleInputRate(p, recipe, item, sprayInputRole)
	if err != nil {
		return 0, err
	}
	if !finitePositive(p.OutputCount) || !finitePositive(p.Capacity.OutputPerMinute) {
		return fail("缺少有效主产物缩放基准")
	}
	scale := p.OutputCount / p.Capacity.OutputPerMinute
	raw, spray = raw*scale, spray*scale
	if !finitePositive(raw) || !finitePositive(spray) || math.Abs(raw+spray-total) > machineMaterialRateTolerance(raw+spray, total, total) {
		return fail("配方与供剂份额不等于规范净输入，需要独立回流角色而非重复供给")
	}
	if role == recipeInputRole {
		return raw, nil
	}
	return spray, nil
}
