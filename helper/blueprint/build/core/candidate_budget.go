package build

// 文件职责：在全局任务中共享实际布局候选预算；使用：整星调用链；边界：旧局部调用没有上限变化。

import (
	"context"
	"sync/atomic"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/industryplan/search"
)

// CandidateBudget 职责：统计父任务已启动求解的候选；使用：多个逐星调用共用；边界：达到上限后拒绝新增。
type CandidateBudget struct {
	limit  int64
	used   atomic.Int64
	denied atomic.Bool
}

// candidateBudgetKey 职责：绑定任务共享计数；使用：候选工作区；边界：不与预览或工作流状态共用键。
type candidateBudgetKey struct{}

// WithCandidateBudget 职责：创建父任务预算；使用：调用方把返回的上下文传给全部逐星布局；边界：不替换已有预算。
func WithCandidateBudget(ctx context.Context, limit int) (context.Context, *CandidateBudget) {
	if existing, ok := ctx.Value(candidateBudgetKey{}).(*CandidateBudget); ok {
		return ctx, existing
	}
	budget := &CandidateBudget{limit: int64(max(0, limit))}
	return context.WithValue(ctx, candidateBudgetKey{}, budget), budget
}

// Used 职责：回报已开始的候选数；使用：父任务进度；边界：并发失败的候选仍计入。
func (b *CandidateBudget) Used() int { return int(b.used.Load()) }

// Exhausted 职责：确认至少一次候选因限额被拒；使用：报告全局预算缺口；边界：刚好用完但没有拒绝不算耗尽失败。
func (b *CandidateBudget) Exhausted() bool { return b.denied.Load() }

// CandidateBudgetLimited 报告本次任务是否限制候选数量。有限预算的规划保持原有派发顺序。
func CandidateBudgetLimited(ctx context.Context) bool {
	b, _ := ctx.Value(candidateBudgetKey{}).(*CandidateBudget)
	return b != nil && b.limit > 0
}

// takeCandidate 职责：原子保留一个候选位；使用：实际布局求解前；边界：没有父任务预算时沿用原先行为。
func takeCandidate(ctx context.Context) error {
	if err := search.Checkpoint(ctx); err != nil {
		return err
	}
	b, ok := ctx.Value(candidateBudgetKey{}).(*CandidateBudget)
	if !ok {
		return nil
	}
	if b.limit == 0 {
		b.used.Add(1)
		return nil
	}
	for used := b.used.Load(); used < b.limit; used = b.used.Load() {
		if b.used.CompareAndSwap(used, used+1) {
			return nil
		}
	}
	b.denied.Store(true)
	return &blueprint.Error{Code: "GEN_E_GLOBAL_CANDIDATE_BUDGET", Message: "全局布局候选预算已用尽"}
}
