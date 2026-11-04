package build

// 文件职责：识别必须通过真实本地毛流闭环处理的密铺黑盒请求。
// 使用：配送/物流塔候选分派；边界：只读可信配方，不把净流供需当作循环已经接好，含回流请求不能落回旧远程端点候选。

import "DysonSphereBox/helper/quant"

// denseBlackboxHasMachineRecycle 职责：按规范生产配方查找机器内回流；使用：密铺模式2/3/4选择候选；边界：失败返回原错误，普通模式能力不在这里扩大。
func denseBlackboxHasMachineRecycle(plan quant.Result, dataset *quant.Dataset) (bool, error) {
	for _, production := range plan.Productions {
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return false, err
		}
		if recipeHasMachineRecycle(recipe) {
			return true, nil
		}
	}
	return false, nil
}
