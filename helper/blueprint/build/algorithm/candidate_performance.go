package build

import (
	"context"
	"fmt"
	"time"
)

// PerformanceCandidate 职责：保存独立候选的阶段计时；使用：并发搜索性能诊断；边界：候选耗时不能相加解释为请求墙钟时间。
type PerformanceCandidate struct {
	ID         string             `json:"id"`
	DurationMs float64            `json:"durationMs"`
	Stages     []PerformanceStage `json:"stages"`
	Failed     bool               `json:"failed"`
}

func beginCandidatePerformance(ctx context.Context, scope string, index int) (context.Context, func(error)) {
	parent := performanceFromContext(ctx)
	if parent == nil {
		return ctx, func(error) {}
	}
	childContext, child := beginGenerationPerformance(ctx)
	return childContext, func(err error) {
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
	}
}
