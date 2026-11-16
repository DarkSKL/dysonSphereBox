package build

// 文件职责：为领域核心提供已有服务诊断、预览和统计的适配边界。
// 使用：新生成任务根持有旁路观察器；边界：不持有生产实体、不选择候选、不运行生成流程，缓存只保存可重建证明。

import (
	"context"
	"sync"
)

// GenerationObservation 职责：汇聚本任务的旁路性能及成功缓存计数；使用：任务根在开始/结束各调用一次；边界：不作为领域状态或调度权威。
type GenerationObservation struct {
	// performance 职责：保存任务计数器；使用：最终报告；边界：不作为候选领域状态。
	performance *generationPerformance
	// registry 职责：汇总请求内成功证明去重；使用：收尾统计；边界：不选择候选。
	registry *candidateRegistry
	// collect 职责：固定是否返回性能报告；使用：任务收尾；边界：不影响算法结果。
	collect bool
	// candidates 职责：持有旁路会话目录；使用：任务终结；边界：不保存实体或工作流状态。
	candidates *candidateObservations
	// once 职责：确保任务计数和失败终结只执行一次；使用：defer；边界：不保护领域操作。
	once sync.Once
}

// BeginObservation 职责：绑定已有观察上下文与任务身份；使用：工作流任务开始前；边界：不创建超时或启动候选，寿命由调用方和引擎决定。
func BeginObservation(ctx context.Context, req Request) (context.Context, *GenerationObservation) {
	ctx, performance := beginGenerationPerformance(ctx)
	ctx = context.WithValue(ctx, generationTaskIDKey{}, req.TaskID)
	ctx = context.WithValue(ctx, generationWorkloadKey{}, req.Plan.MachineCount)
	ctx = withGenerationPreview(ctx, req)
	candidates := &candidateObservations{entries: make(map[string]*observedCandidate), root: previewFromContext(ctx)}
	ctx = context.WithValue(ctx, candidateObservationKey{}, candidates)
	registry := newCandidateRegistry()
	ctx = context.WithValue(ctx, candidateRegistryKey{}, registry)
	return ctx, &GenerationObservation{performance: performance, registry: registry, collect: req.CollectPerformance, candidates: candidates}
}

// Finish 职责：收口旁路报告并清理失败预览；使用：任务根defer；边界：失败不会发布蓝图或成功事件。
func (o *GenerationObservation) Finish(ctx context.Context, result *Result, err error) {
	if o == nil || o.performance == nil || o.registry == nil {
		return
	}
	v4 := o.candidates != nil && o.candidates.root != nil && o.candidates.root.protocol == 4
	if result == nil {
		if !v4 {
			return
		}
		result = &Result{}
	}
	if err == nil && v4 {
		err = ctx.Err()
	}
	o.once.Do(func() {
		if o.candidates != nil {
			o.candidates.close(err)
		}
		if err == nil {
			report := o.registry.snapshot()
			result.CandidateDeduplication = &report
		}
		performance := o.performance.finish(ctx, *result, err)
		if err == nil && o.collect {
			result.Performance = &performance
		}
		if err != nil {
			clearGenerationPreview(ctx)
		}
	})
}

// CandidateSlots 职责：限制旧协议的共享画面为单计算槽；使用：主线设置RunOptions.Parallel；边界：v4与未启用预览保持请求值，零值仍表示引擎自动。
func (o *GenerationObservation) CandidateSlots(requested int) int {
	if o != nil && o.candidates != nil && o.candidates.root != nil && o.candidates.root.protocol != 4 {
		return 1
	}
	return requested
}

// PublishResult 职责：只发布已完成结果的获选观察及最终预览；使用：三个结果步骤完成后的任务出口；边界：不归一原请求、不计算统计或业务说明，不选择候选、不修改编码实体。
func PublishResult(ctx context.Context, _ Request, input *PreparedInput, result Result) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, previewContextError(err)
	}
	if input == nil {
		return Result{}, previewContextError(context.Canceled)
	}
	if result.Statistics == nil || len(result.statisticsRecords) != 0 {
		return Result{}, resultStepError("发布前尚未完成最终结果组合")
	}
	if err := publishObservationWinner(ctx, result); err != nil {
		return Result{}, err
	}
	return publishFinalBlueprintProgress(ctx, input.request, input.dataset, result, nil)
}

// BetterBusCandidate 职责：比较完整验证的总线几何；使用：引擎只读比较器；边界：总建筑数不能因优化增加，不用估计路线替代真实指标。
func BetterBusCandidate(a, b VerifiedSnapshot) bool {
	if a.geometry.data == nil || b.geometry.data == nil {
		return false
	}
	return len(a.geometry.data.document.Buildings) <= len(b.geometry.data.document.Buildings) && betterTransportCandidate(a.Metrics(), b.Metrics())
}
