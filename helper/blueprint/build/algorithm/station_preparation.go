package build

// 文件职责：把已规范化请求转换为物流塔布局阶段的显式输入。
// 使用：统一阶段编排在调用模式目录的布局实现前执行；边界：只验证配方、设备和单行容量，不决定模式分组、不操作建筑。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// prepareStationLayoutInput 职责：准备布局需要的规范配方和容量拆列；使用：物流塔公共准备；边界：模式包只得到数据，不得到 Builder 或私有函数回调。
func prepareStationLayoutInput(ctx context.Context, request Request, dataset *quant.Dataset, setting Setting, graph *quant.FlowGraph) (contract.StationLayoutInput, error) {
	if graph == nil {
		return contract.StationLayoutInput{}, &blueprint.Error{Code: "GEN_E_FLOW", Message: "物流塔生成缺少规范物料图"}
	}
	reportProgress(ctx, "zones", "正在准备物流塔布局数据", 0, 0)
	input := contract.StationLayoutInput{Plan: request.Plan, FlowGraph: graph, Productions: make([]contract.PreparedStationProduction, len(request.Plan.Productions))}
	for index, production := range request.Plan.Productions {
		if ctx.Err() != nil {
			return contract.StationLayoutInput{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔准备已取消或超时"}
		}
		if production.MachineCount <= 0 || production.MachineCount > 10000 || production.IncreaseLv < 0 || production.IncreaseLv > 3 {
			return contract.StationLayoutInput{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "方案机器数量或等级无效"}
		}
		recipeIndex, ok := dataset.RecipeByID[production.RecipeID]
		if !ok {
			return contract.StationLayoutInput{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "配方已变化"}
		}
		recipe, err := effectiveProductionRecipe(production, dataset.Recipes[recipeIndex])
		if err != nil {
			return contract.StationLayoutInput{}, err
		}
		validMachine := false
		for _, machineID := range recipe.Factories {
			if production.MachineID == machineID {
				validMachine = true
				break
			}
		}
		if !validMachine {
			return contract.StationLayoutInput{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "设施与配方不匹配"}
		}
		counts, err := lineCounts(production, recipe, setting, dataset.ByID[production.MachineID].Speed)
		if err != nil {
			return contract.StationLayoutInput{}, err
		}
		input.Productions[index] = contract.PreparedStationProduction{Recipe: recipe, Counts: counts, SprayItemID: productionSprayItem(production)}
		reportProgress(ctx, "production", "正在核验"+dataset.ByID[production.MainItemID].Name+"物流塔产线", index+1, len(request.Plan.Productions))
	}
	return input, nil
}
