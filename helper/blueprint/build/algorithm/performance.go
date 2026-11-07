package build

// 文件职责：记录一次生成调用的结构化性能事实。
// 使用：Generate 创建请求级观察器，阶段、寻路和碰撞入口只追加实测计数。
// 边界：不参与候选选择、预算或硬规则；未测量的内存峰值和取消响应延迟明确保留为缺口。

import (
	"context"
	"sort"
	"sync"
	"time"

	"DysonSphereBox/helper/quant"
)

// GenerationPerformance 职责：汇总单次 Generate 的全部候选工作；使用：结果诊断与性能回归；边界：耗时不是确定性输出或游戏验收证据。
type GenerationPerformance struct {
	CandidateDetails []PerformanceCandidate `json:"candidateDetails"`
	// Scope 职责：标识计数作用域；使用：区分请求与获选候选；边界：固定为 request-all-candidates。
	Scope string `json:"scope"`
	// Outcome 职责：记录 passed、failed、canceled 或 timed-out；使用：失败观察回调；边界：不替代返回错误。
	Outcome string `json:"outcome"`
	// TotalDurationMs 职责：记录从进入 Generate 到形成结果的墙钟毫秒；使用：效率对比；边界：包含同步阶段回调，不包含性能回调自身。
	TotalDurationMs float64 `json:"totalDurationMs"`
	// Stages 职责：累计按阶段通知划分的墙钟时间；使用：定位耗时；边界：阶段可重入，顺序为首次到达顺序。
	Stages []PerformanceStage `json:"stages"`
	// Flow 职责：记录规范物料图和最终 lane 数；使用：固定规模基线；边界：lane 只在成功后记录最终实体数量。
	Flow PerformanceFlow `json:"flow"`
	// CandidateStages 职责：分别累计现有候选阶段的实际尝试次数；使用：识别嵌套布局工作；边界：不同阶段不能相加当作唯一布局数。
	CandidateStages []PerformanceCandidates `json:"candidateStages"`
	// Routing 职责：累计所有成功、失败与放弃候选的运输 A* 调用；使用：效率诊断；边界：不包含二维走廊势场预计算。
	Routing PerformanceRouting `json:"routing"`
	// Collisions 职责：分规则累计已接入的空间索引与精确几何工作；使用：核对宽窄相位；边界：不是整座生成器全部几何调用的总数。
	Collisions []PerformanceCollision `json:"collisions"`
	// Unmeasured 职责：明示本次未测量的性能项；使用：验收证据边界；边界：不得把缺失项解释为零。
	Unmeasured []string `json:"unmeasured"`
}

// PerformanceStage 职责：承载可重入阶段的累计墙钟；使用：请求耗时诊断；边界：按报告阶段划分，不推断内部子函数耗时。
type PerformanceStage struct {
	// Stage 职责：保存实际阶段键；使用：关联进度协议；边界：initialization 表示首条通知前的时间。
	Stage string `json:"stage"`
	// DurationMs 职责：累计该阶段的墙钟毫秒；使用：热点诊断；边界：不参与算法决策。
	DurationMs float64 `json:"durationMs"`
	// Visits 职责：统计进入该阶段的次数；使用：解释候选重入；边界：同阶段连续报告只计一次进入。
	Visits int `json:"visits"`
}

// PerformanceFlow 职责：保存请求的规范图规模；使用：比较相同输入性能；边界：不修改守恒或 SCC 结果。
type PerformanceFlow struct {
	// NodeCount 职责：统计含边界的规范节点；使用：规模记录；边界：不是机器数。
	NodeCount int `json:"nodeCount"`
	// EdgeCount 职责：统计规范供需边；使用：规模记录；边界：不是实体带段数。
	EdgeCount int `json:"edgeCount"`
	// SCCCount 职责：统计强连通分量；使用：拓扑规模；边界：包含非循环单节点分量。
	SCCCount int `json:"sccCount"`
	// CyclicSCCCount 职责：统计实际循环分量；使用：循环复杂度对比；边界：不替代保边验证。
	CyclicSCCCount int `json:"cyclicSccCount"`
	// LaneCount 职责：记录成功结果的实体 lane 数；使用：容量规模；边界：失败时不发布部分 lane。
	LaneCount int `json:"laneCount"`
}

// PerformanceCandidates 职责：记录一种现有候选阶段的请求累计工作；使用：比较完整生成成本；边界：预算为该阶段单组已报告的最大值。
type PerformanceCandidates struct {
	// Stage 职责：标识候选类型；使用：区分布局和球面宽度；边界：不混合不同搜索层级。
	Stage string `json:"stage"`
	// Attempts 职责：累计实际报告的候选尝试；使用：请求成本统计；边界：只覆盖已标记阶段。
	Attempts int `json:"attempts"`
	// MaxGroupBudget 职责：保存报告过的单组最大预算；使用：解释嵌套候选；边界：不代表请求累计预算。
	MaxGroupBudget int `json:"maxGroupBudget"`
}

// PerformanceRouting 职责：保存运输三维 A* 的累计计数；使用：核对缓存和展开效率；边界：缓存命中也算调用但展开为零。
type PerformanceRouting struct {
	// SearchCalls 职责：累计实际寻路函数调用；使用：包含被剪枝、取消和缓存命中的请求；边界：不是获选路线条数。
	SearchCalls int `json:"searchCalls"`
	// CacheHits 职责：累计实际路径缓存命中；使用：缓存效率；边界：多源与多终点搜索不使用单终点缓存。
	CacheHits int `json:"cacheHits"`
	// TotalExpanded 职责：累计实际弹出并展开的节点；使用：请求成本；边界：未展开的队列项不计入。
	TotalExpanded int `json:"totalExpanded"`
	// MaxExpanded 职责：保存单次最高展开数；使用：核对固定预算；边界：不增加原上限。
	MaxExpanded int `json:"maxExpanded"`
	// AverageExpanded 职责：以调用数为分母计算平均展开；使用：效率对比；边界：包含缓存与零展开调用。
	AverageExpanded float64 `json:"averageExpanded"`
	// CacheHitRate 职责：计算命中调用比例；使用：效率对比；边界：零调用时为零。
	CacheHitRate float64 `json:"cacheHitRate"`
}

// PerformanceCollision 职责：记录一种已接入碰撞规则的真实工作；使用：比较宽窄相位；边界：次数按形状对或几何调用统计，不是建筑对全局去重。
type PerformanceCollision struct {
	// Rule 职责：标记计数所属规则；使用：解释覆盖；边界：不同规则不能视为同一去重集合。
	Rule string `json:"rule"`
	// Invocations 职责：累计规则检查调用；使用：包含失败候选；边界：零候选调用仍计一次。
	Invocations int `json:"invocations"`
	// CandidatePairs 职责：累计空间索引返回并局部去重的形状对；使用：宽相位诊断；边界：同建筑或有连接豁免是否计入由该规则约定。
	CandidatePairs uint64 `json:"candidatePairs"`
	// NarrowPhaseChecks 职责：累计真实执行的 OBB 或距离计算；使用：窄相位诊断；边界：一个候选可能调用两次数学检查。
	NarrowPhaseChecks uint64 `json:"narrowPhaseChecks"`
}

// performanceKey 职责：隔离请求计数器键；使用：生成上下文传播；边界：不保存全局状态。
type performanceKey struct{}

// performanceObserverKey 职责：隔离可选诊断回调键；使用：WithPerformance；边界：与进度回调独立。
type performanceObserverKey struct{}

// generationPerformance 职责：维护单请求计时与确定性计数；使用：观测入口；边界：锁只保护统计，不包围生成算法。
type generationPerformance struct {
	// mu 职责：串行保护观测数据；使用：所有追加入口；边界：不持锁执行引擎或外部回调。
	mu sync.Mutex
	// started/stageStarted 职责：保存单调墙钟起点；使用：阶段与请求收尾；边界：不参与布局排序。
	started, stageStarted time.Time
	// stage 职责：保存当前阶段；使用：累加连续区间；边界：同阶段重复报告不新开区间。
	stage string
	// data 职责：保存本请求可序列化快照；使用：结束时交付；边界：不引用 builder 实体。
	data GenerationPerformance
}

// WithPerformance 职责：订阅成功、失败或取消调用的最终性能快照；使用：服务或性能测试包装父上下文；边界：回调同步且应快速返回，不允许修改引擎，失败仍返回空 Result。
func WithPerformance(ctx context.Context, observer func(GenerationPerformance)) context.Context {
	return context.WithValue(ctx, performanceObserverKey{}, observer)
}

// beginGenerationPerformance 职责：为每次 Generate 独立创建计数器；使用：函数入口；边界：即使复用父上下文也不混入上一次请求。
func beginGenerationPerformance(ctx context.Context) (context.Context, *generationPerformance) {
	now := time.Now()
	p := &generationPerformance{
		started: now, stageStarted: now, stage: "initialization",
		data: GenerationPerformance{
			Scope: "request-all-candidates", Stages: []PerformanceStage{{Stage: "initialization", Visits: 1}},
			CandidateStages: []PerformanceCandidates{}, Collisions: []PerformanceCollision{},
			Unmeasured: []string{"peak-memory", "cancellation-response-latency", "uninstrumented-geometry", "corridor-potential-expansions", "uninstrumented-layout-candidates"},
		},
	}
	return context.WithValue(ctx, performanceKey{}, p), p
}

// performanceFromContext 职责：取得当前请求计数器；使用：各观测入口；边界：未通过 Generate 的调用安全地不记录。
func performanceFromContext(ctx context.Context) *generationPerformance {
	p, _ := ctx.Value(performanceKey{}).(*generationPerformance)
	return p
}

// recordPerformanceStage 职责：闭合上一阶段并记录实际候选尝试；使用：reportProgress；边界：不依赖是否有进度订阅者，不发出新事件。
func recordPerformanceStage(ctx context.Context, stage string, total int) {
	p := performanceFromContext(ctx)
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if stage != p.stage {
		now := time.Now()
		p.closeStage(now)
		p.stage, p.stageStarted = stage, now
		found := false
		for i := range p.data.Stages {
			if p.data.Stages[i].Stage == stage {
				p.data.Stages[i].Visits++
				found = true
				break
			}
		}
		if !found {
			p.data.Stages = append(p.data.Stages, PerformanceStage{Stage: stage, Visits: 1})
		}
	}
	switch stage {
	case "flow-layout", "fly-layout", "fly-surface-widths", "station-surface-widths":
		for i := range p.data.CandidateStages {
			if p.data.CandidateStages[i].Stage == stage {
				p.data.CandidateStages[i].Attempts++
				p.data.CandidateStages[i].MaxGroupBudget = max(p.data.CandidateStages[i].MaxGroupBudget, total)
				return
			}
		}
		p.data.CandidateStages = append(p.data.CandidateStages, PerformanceCandidates{Stage: stage, Attempts: 1, MaxGroupBudget: total})
	}
}

// closeStage 职责：把阶段起点到指定时刻累计到已存在条目；使用：持锁阶段转换与收尾；边界：调用方负责不重复闭合。
func (p *generationPerformance) closeStage(now time.Time) {
	for i := range p.data.Stages {
		if p.data.Stages[i].Stage == p.stage {
			p.data.Stages[i].DurationMs += float64(now.Sub(p.stageStarted)) / float64(time.Millisecond)
			return
		}
	}
}

// recordPerformanceFlow 职责：记录唯一规范图的规模；使用：Generate 完成 BuildFlowGraph 后；边界：不累计各候选重复引用。
func recordPerformanceFlow(ctx context.Context, graph quant.FlowGraph) {
	p := performanceFromContext(ctx)
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data.Flow = PerformanceFlow{NodeCount: len(graph.Nodes), EdgeCount: len(graph.Edges), SCCCount: len(graph.Components)}
	for _, component := range graph.Components {
		if component.Cyclic {
			p.data.Flow.CyclicSCCCount++
		}
	}
}

// recordPerformanceSearch 职责：累计一次已完成或提前退出的运输搜索；使用：所有运输 A* 的 defer；边界：沿用原展开数，不增加搜索。
func recordPerformanceSearch(ctx context.Context, expanded int, cacheHit bool) {
	p := performanceFromContext(ctx)
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	r := &p.data.Routing
	r.SearchCalls++
	r.TotalExpanded += expanded
	r.MaxExpanded = max(r.MaxExpanded, expanded)
	if cacheHit {
		r.CacheHits++
	}
}

// recordPerformanceCollision 职责：累计一类已执行的宽窄相位工作；使用：碰撞检查收尾；边界：只登记真实计数，不将未接入规则默认为零工作。
func recordPerformanceCollision(ctx context.Context, rule string, candidates, narrow uint64) {
	p := performanceFromContext(ctx)
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.data.Collisions {
		if p.data.Collisions[i].Rule == rule {
			p.data.Collisions[i].Invocations++
			p.data.Collisions[i].CandidatePairs += candidates
			p.data.Collisions[i].NarrowPhaseChecks += narrow
			return
		}
	}
	p.data.Collisions = append(p.data.Collisions, PerformanceCollision{Rule: rule, Invocations: 1, CandidatePairs: candidates, NarrowPhaseChecks: narrow})
}

// finish 职责：闭合计时、派生均值并通知可选观察者；使用：Generate 的 defer；边界：失败快照不携带部分蓝图或建筑，未提供内存与取消延迟假值。
func (p *generationPerformance) finish(ctx context.Context, result Result, err error) GenerationPerformance {
	p.mu.Lock()
	now := time.Now()
	p.closeStage(now)
	p.data.TotalDurationMs = float64(now.Sub(p.started)) / float64(time.Millisecond)
	p.data.Outcome = "passed"
	if err != nil {
		p.data.Outcome = "failed"
		if ctx.Err() == context.Canceled {
			p.data.Outcome = "canceled"
		}
		if ctx.Err() == context.DeadlineExceeded {
			p.data.Outcome = "timed-out"
		}
	}
	if err == nil && result.LogisticsSummary != nil {
		p.data.Flow.LaneCount = len(result.LogisticsSummary.Lanes)
	}
	if r := &p.data.Routing; r.SearchCalls > 0 {
		r.AverageExpanded = float64(r.TotalExpanded) / float64(r.SearchCalls)
		r.CacheHitRate = float64(r.CacheHits) / float64(r.SearchCalls)
	}
	snapshot := p.data
	sort.Slice(snapshot.Collisions, func(first, second int) bool {
		return snapshot.Collisions[first].Rule < snapshot.Collisions[second].Rule
	})
	sort.Slice(snapshot.CandidateStages, func(first, second int) bool {
		return snapshot.CandidateStages[first].Stage < snapshot.CandidateStages[second].Stage
	})
	sort.Slice(snapshot.CandidateDetails, func(first, second int) bool {
		return snapshot.CandidateDetails[first].ID < snapshot.CandidateDetails[second].ID
	})
	p.mu.Unlock()
	if observer, ok := ctx.Value(performanceObserverKey{}).(func(GenerationPerformance)); ok && observer != nil {
		observer(snapshot)
	}
	return snapshot
}
