package build

// 文件职责：在生成上下文里隔离各模式共用的候选统计和终检成功缓存；使用：一次Generate；边界：不修改父上下文或复用失败证明。

import (
	"DysonSphereBox/helper/blueprint/build/candidates"
	"context"
	"sort"
	"sync"
)

// candidateRegistryKey 职责：隔离单次生成的缓存；使用：递归及并发候选继承；边界：不跨用户请求。
type candidateRegistryKey struct{}

// candidateRegistry 职责：汇总规划去重和终检复用；使用：全部模式；边界：缓存值不可变，锁不覆盖实际算法执行。
type candidateRegistry struct {
	mutex  sync.Mutex
	stages map[string]candidates.Stage
	rules  *candidates.Memo[finalRuleProof]
	// rowAllocations 在同源流图上按实际分行与带容量共享不可变的供需份额。
	rowAllocations *candidates.Memo[[]transportRowAllocation]
}

// newCandidateRegistry 职责：创建有界请求注册表；使用：Generate唯一入口；边界：私有实例不存储完整蓝图。
func newCandidateRegistry() *candidateRegistry {
	return &candidateRegistry{stages: map[string]candidates.Stage{}, rules: candidates.NewMemo[finalRuleProof](128),
		rowAllocations: candidates.NewMemo[[]transportRowAllocation](16)}
}

// recordCandidatePlans 职责：统一登记规范化效果；使用：所有模式候选列表生成后；边界：独立测试未提供注册表时不改变行为。
func recordCandidatePlans(ctx context.Context, scope string, proposed, unique, inactive int) {
	registry, _ := ctx.Value(candidateRegistryKey{}).(*candidateRegistry)
	if registry == nil {
		return
	}
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	stage := registry.stages[scope]
	stage.Scope = scope
	stage.Proposed += proposed
	stage.Unique += unique
	stage.Skipped += proposed - unique
	stage.InactiveModifiers += inactive
	registry.stages[scope] = stage
}

// snapshot 职责：返回不可共享的稳定计数；使用：成功结果或失败性能观察；边界：不暴露内部缓存。
func (r *candidateRegistry) snapshot() candidates.Report {
	out := candidates.Report{Stages: []candidates.Stage{}, FinalRules: r.rules.Counts()}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if work := r.rowAllocations.Counts(); work.Executed+work.Reused > 0 {
		stage := candidates.Stage{Scope: "transport-row-allocation", Proposed: work.Executed + work.Reused, Unique: work.Executed, Skipped: work.Reused}
		out.Stages = append(out.Stages, stage)
		out.Proposed, out.Unique, out.Skipped = stage.Proposed, stage.Unique, stage.Skipped
	}
	for _, s := range r.stages {
		out.Stages = append(out.Stages, s)
		out.Proposed += s.Proposed
		out.Unique += s.Unique
		out.Skipped += s.Skipped
		out.InactiveModifiers += s.InactiveModifiers
	}
	sort.Slice(out.Stages, func(i, j int) bool { return out.Stages[i].Scope < out.Stages[j].Scope })
	return out
}
