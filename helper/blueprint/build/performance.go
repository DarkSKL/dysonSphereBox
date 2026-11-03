package build

import (
	"context"

	algorithm "DysonSphereBox/helper/blueprint/build/algorithm"
)

// GenerationPerformance 职责：公开生成性能统计；使用：诊断订阅；边界：不参与生成决策。
type GenerationPerformance = algorithm.GenerationPerformance

// PerformanceStage 职责：公开阶段耗时；使用：性能分析；边界：不表示任务进度。
type PerformanceStage = algorithm.PerformanceStage

// PerformanceFlow 职责：公开生产流统计；使用：性能分析；边界：不改变生产流。
type PerformanceFlow = algorithm.PerformanceFlow

// PerformanceCandidates 职责：公开候选统计；使用：性能分析；边界：不控制搜索。
type PerformanceCandidates = algorithm.PerformanceCandidates

// PerformanceRouting 职责：公开寻路统计；使用：性能分析；边界：不执行寻路。
type PerformanceRouting = algorithm.PerformanceRouting

// PerformanceCollision 职责：公开碰撞统计；使用：性能分析；边界：不执行碰撞检查。
type PerformanceCollision = algorithm.PerformanceCollision

// WithPerformance 职责：注册生成性能观察者；使用：包装 Generate 的上下文；边界：保持引擎原有统计语义。
func WithPerformance(ctx context.Context, observer func(GenerationPerformance)) context.Context {
	return algorithm.WithPerformance(ctx, observer)
}
