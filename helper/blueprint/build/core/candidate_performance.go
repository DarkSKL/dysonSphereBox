package build

// 文件职责：聚合引擎绑定的完整候选旁路计时；使用：固定路径作用域；边界：只记录计数，不选择或重复执行候选。

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PerformanceCandidate 职责：保存独立候选的阶段计时；使用：并发搜索性能诊断；边界：候选耗时不能相加解释为请求墙钟时间。
type PerformanceCandidate struct {
	// ID 职责：记录固定作用域与输入序号；使用：稳定排序；边界：不是完成顺序。
	ID string `json:"id"`
	// DurationMs 职责：保存候选实际耗时；使用：诊断；边界：不可作为并行任务墙钟总和。
	DurationMs float64 `json:"durationMs"`
	// Stages 职责：保存粗粒度阶段计时；使用：定位成本；边界：不保存实体或参数。
	Stages []PerformanceStage `json:"stages"`
	// Failed 职责：标记候选失败；使用：诊断；边界：可行落选不当作领域错误。
	Failed bool `json:"failed"`
}

// beginCandidatePerformance 职责：隔离候选计数并在结束时合入任务；使用：引擎路径绑定；边界：结束幂等，不能重复聚合。
func beginCandidatePerformance(ctx context.Context, scope string, index int) (context.Context, func(error)) {
	parent := performanceFromContext(ctx)
	if parent == nil {
		return ctx, func(error) {}
	}
	childContext, child := beginGenerationPerformance(ctx)
	var once sync.Once
	return childContext, func(err error) {
		once.Do(func() {
			child.mu.Lock()
			now := time.Now()
			child.closeStage(now)
			data := child.data
			duration := float64(now.Sub(child.started)) / float64(time.Millisecond)
			child.mu.Unlock()
			parent.mu.Lock()
			defer parent.mu.Unlock()
			parent.data.CandidateDetails = append(parent.data.CandidateDetails, PerformanceCandidate{ID: fmt.Sprintf("%s/%d", scope, index), DurationMs: duration, Stages: data.Stages, Failed: err != nil})
			parent.data.Routing.SearchCalls += data.Routing.SearchCalls
			parent.data.Routing.CacheHits += data.Routing.CacheHits
			parent.data.Routing.TotalExpanded += data.Routing.TotalExpanded
			parent.data.Routing.MaxExpanded = max(parent.data.Routing.MaxExpanded, data.Routing.MaxExpanded)
			for _, candidate := range data.CandidateStages {
				found := false
				for position := range parent.data.CandidateStages {
					value := &parent.data.CandidateStages[position]
					if value.Stage == candidate.Stage {
						value.Attempts += candidate.Attempts
						value.MaxGroupBudget = max(value.MaxGroupBudget, candidate.MaxGroupBudget)
						found = true
						break
					}
				}
				if !found {
					parent.data.CandidateStages = append(parent.data.CandidateStages, candidate)
				}
			}
			for _, collision := range data.Collisions {
				found := false
				for position := range parent.data.Collisions {
					value := &parent.data.Collisions[position]
					if value.Rule == collision.Rule {
						value.Invocations += collision.Invocations
						value.CandidatePairs += collision.CandidatePairs
						value.NarrowPhaseChecks += collision.NarrowPhaseChecks
						found = true
						break
					}
				}
				if !found {
					parent.data.Collisions = append(parent.data.Collisions, collision)
				}
			}
		})
	}
}
