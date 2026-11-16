package generator

// 文件职责：将三个纯结果步骤分别接入受限工作流节点。
// 使用：编码后的result/compose子流；边界：每个工具只执行一个核心，不访问调度、装配器或发布预览。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"context"
)

// resultTools 职责：只读持有目录键；使用：反射注册；边界：接收器没有任务状态或可变结果。
type resultTools struct {
	// keys 职责：持有受限产物目录；使用：各结果工具固定读写边界；边界：不保存任务值或工作区句柄。
	keys artifacts
}

// CalculateStatistics 职责：计算唯一获选编码的最终统计；使用：stats.calculate@1；边界：失败不暂存报告，不发布成功。
func (t *resultTools) CalculateStatistics(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	encoded, err := workflow.Read(s, t.keys.encoded)
	if err != nil {
		return err
	}
	report, err := core.CalculateResultStatistics(ctx, input, encoded)
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.resultStages.statistics, report)
}

// CalculateTransportUpgrade 职责：说明初始用户与最终实际档位差异；使用：result.transport-upgrade@1；边界：只消费规范输入，不重新校验原请求或改变设备。
func (t *resultTools) CalculateTransportUpgrade(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	base, err := workflow.Read(s, t.keys.base)
	if err != nil {
		return err
	}
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	encoded, err := workflow.Read(s, t.keys.encoded)
	if err != nil {
		return err
	}
	report, err := core.CalculateResultTransportUpgrade(ctx, base, input, encoded)
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.resultStages.transportUpgrade, report)
}

// Compose 职责：组合已有统计与说明为最终Result；使用：result.compose@1；边界：不补算、不编码、不发布预览。
func (t *resultTools) Compose(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	encoded, err := workflow.Read(s, t.keys.encoded)
	if err != nil {
		return err
	}
	statistics, err := workflow.Read(s, t.keys.resultStages.statistics)
	if err != nil {
		return err
	}
	upgrade, err := workflow.Read(s, t.keys.resultStages.transportUpgrade)
	if err != nil {
		return err
	}
	result, err := core.ComposeResult(ctx, encoded, statistics, upgrade)
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.resultStages.final, result)
}
