package build

// 文件职责：按生产规模协调候选工作集和最终实体预算。
// 使用：生成入口登记已校验计划的设备数，候选调度和通用收尾读取；边界：不增加时限、不改产量或碰撞规则，显式并发配置由原入口校验。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
)

// generationWorkloadKey 职责：隔离单请求的生产设备规模；使用：从 Generate 传递到独立候选；边界：不跨请求共享或参与蓝图编码。
type generationWorkloadKey struct{}

// largeFactoryWorkload 职责：识别需要控制同时在途工作集的大型工厂；使用：默认并发及已落位单元的局部修复；边界：只按生产设备数量划分，传送带数量不反向改变该请求。
func largeFactoryWorkload(ctx context.Context) bool {
	machines, _ := ctx.Value(generationWorkloadKey{}).(int)
	return machines >= 2048
}

// generationBuildingBudget 职责：保留常规工厂的既有实体预算，给大型工厂使用已验证的扩展预算；使用：路线提交及最终收尾；边界：始终受公共编解码上限及独立文本、解压大小限制。
func generationBuildingBudget(ctx context.Context) int {
	if largeFactoryWorkload(ctx) {
		return blueprint.MaxBuildingCount
	}
	return min(200000, blueprint.MaxBuildingCount)
}
