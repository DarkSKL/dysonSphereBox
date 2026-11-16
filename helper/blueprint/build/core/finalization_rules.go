package build

// 文件职责：把终检所需的候选规则上下文固化为显式领域值。
// 使用：定稿捕获、独立校验重放；边界：不保存任务context或可执行装配回调，不允许调用者通过新context改变已定稿的规则语义。

import (
	"context"
)

// finalizationRuleContext 职责：保存终检真正使用的规则选择；使用：快照指纹与校验；边界：只有真实业务规则，没有测试替代工厂或闭包。
type finalizationRuleContext struct {
	// DenseCollisions 职责：固定整图单次粘贴碰撞口径；使用：记录规则；边界：不能由后续context覆盖。
	DenseCollisions bool
	// SplitterModel 职责：固定候选的真实分流器模型；使用：生产分流树核验；边界：仍由共同规则验证模型和端口。
	SplitterModel int16
}

// captureFinalizationRuleContext 职责：读取并验证候选规则值；使用：定稿冻结；边界：非法模型沿用真实规则失败，不猜测替代模型。
func captureFinalizationRuleContext(ctx context.Context) (finalizationRuleContext, error) {
	model, err := splitterGateModel(ctx)
	if err != nil {
		return finalizationRuleContext{}, err
	}
	return finalizationRuleContext{DenseCollisions: usesDenseBeltCollisions(ctx), SplitterModel: model}, nil
}

// validationContext 职责：把冻结规则值应用到本次取消上下文；使用：独立规则入口；边界：只传递领域值，不保留原任务context或覆盖本次截止时间。
func (rules finalizationRuleContext) validationContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, denseCollisionKey{}, rules.DenseCollisions)
	ctx = context.WithValue(ctx, splitterGateModelKey{}, rules.SplitterModel)
	return ctx
}
