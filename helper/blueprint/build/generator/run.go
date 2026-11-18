package generator

// 文件职责：装配生产方案族、唯一任务实例和反射工具执行入口。
// 使用：build公开门面及真实回归；边界：不包含模式候选算法，不调用旧Generate，不提交未校验结果。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// runtime 职责：只读保存工具及冻结执行计划；使用：任务共享；边界：不保存进行中的任务状态。
type runtime struct {
	// execution 与编译计划来自同一次文件装载。
	execution executionPolicy
	search    core.LayoutSearchPolicy
	keys      artifacts
	plan      *workflow.Plan
	registry  *workflow.Registry
}

// layoutComparison 职责：应用各领域真实的双方比较规则；使用：引擎稳定归约；边界：只消费已验证快照，同分保留原序。
func (r *runtime) layoutComparison(a, b *workflow.FrozenSnapshot) (int, error) {
	first, err := workflow.SnapshotRead(a, r.keys.verified, "")
	if err != nil {
		return 0, err
	}
	second, err := workflow.SnapshotRead(b, r.keys.verified, "")
	if err != nil {
		return 0, err
	}
	left, err := workflow.SnapshotRead(a, r.keys.choice, "")
	if err != nil {
		return 0, err
	}
	right, err := workflow.SnapshotRead(b, r.keys.choice, "")
	if err != nil {
		return 0, err
	}
	if left.Family != right.Family {
		return 0, fmt.Errorf("incomparable candidate families")
	}
	better := false
	switch left.Family {
	case "connected":
		input, e := workflow.SnapshotRead(a, r.keys.input, "")
		if e != nil {
			return 0, e
		}
		better, err = core.BetterConnectedCandidate(input, first.Validation(), second.Validation())
	case "station":
		n, e := core.StationCandidateTowerCount(first.Geometry())
		if e != nil {
			return 0, e
		}
		m, e := core.StationCandidateTowerCount(second.Geometry())
		if e != nil {
			return 0, e
		}
		better = core.BetterStationCandidate(left.Station, n, first.Metrics(), right.Station, m, second.Metrics())
	case "distributor":
		n, e := core.DistributorCandidateFacilityCount(first.Geometry())
		if e != nil {
			return 0, e
		}
		m, e := core.DistributorCandidateFacilityCount(second.Geometry())
		if e != nil {
			return 0, e
		}
		better = core.BetterDistributorCandidate(left.Distributor, n, first.Metrics(), right.Distributor, m, second.Metrics())
	case "bus":
		better = core.BetterBusCandidate(first, second)
	default:
		return 0, fmt.Errorf("missing candidate comparator")
	}
	if err != nil {
		return 0, err
	}
	if better {
		return -1, nil
	}
	return 0, nil
}

// Generate 职责：以唯一任务实例执行全部生产YAML；使用：公开生成门面；边界：任何失败只返回空领域结果，旧引擎不可回退。
func Generate(parent context.Context, req core.Request) (result core.Result, err error) {
	result, _, err = GenerateDetailed(parent, req)
	return
}

// GenerateDetailed 职责：执行并返回完整工具轨迹；使用：生成门面和架构回归；边界：轨迹不进入产品界面，不替代真实实体校验。
func GenerateDetailed(parent context.Context, req core.Request) (result core.Result, trace *workflow.RunResult, err error) {
	defer func() { FinishWorkflowTracking(parent, trace, err) }()
	if req.CandidateConcurrency < 0 || req.CandidateConcurrency > 10 {
		return result, nil, &blueprint.Error{Code: "GEN_E_CONCURRENCY", Message: "候选并发上限必须为自动或一到十"}
	}
	r, err := loadRuntime()
	if err != nil {
		return result, nil, &blueprint.Error{Code: "GEN_E_WORKFLOW_CONFIG", Message: err.Error()}
	}
	if HasWorkflowStart(parent) {
		if err = StartWorkflowTracking(parent, r.plan.Describe()); err != nil {
			return result, nil, err
		}
	}
	workBudget := r.execution.workBudget(req.Plan.MachineCount)
	ctx, cancel := context.WithTimeout(parent, workBudget)
	defer cancel()
	ctx, observer := core.BeginObservation(ctx, req)
	defer func() { observer.Finish(ctx, &result, err) }()
	instance, err := workflow.NewInstance(r.keys.all)
	if err != nil {
		return result, nil, err
	}
	if err = workflow.Seed(instance, r.keys.request, "", req.WithLayoutSearch(r.search)); err != nil {
		return result, nil, publicError(ctx, err)
	}
	// 自动模式为大型计划保留内存余量；显式并发选择仍由请求决定，不改变全局GOMAXPROCS。
	parallel := r.execution.parallel(req.CandidateConcurrency, req.Plan.MachineCount)
	trace, err = workflow.Run(ctx, r.plan, instance, workflow.RunOptions{
		Timeout:          workBudget,
		ObserveExecution: WorkflowExecutionObserver(ctx),
		Parallel:         observer.CandidateSlots(parallel),
		Score:            func(*workflow.FrozenSnapshot) ([]float64, error) { return []float64{0}, nil },
		Comparators:      map[string]workflow.Comparator{"layout": r.layoutComparison, "station-refinement": r.refinementComparison},
		Observe: func(event workflow.CandidateEvent) error {
			var verified *core.VerifiedSnapshot
			if event.Snapshot != nil {
				value, readErr := r.observationVerified(event.Snapshot)
				if readErr != nil {
					return readErr
				}
				verified = &value
			}
			return core.ObserveCandidate(ctx, event.Kind, event.Path, event.Ordinal, verified, event.Failure)
		},
	})
	if err != nil {
		return core.Result{}, trace, publicError(ctx, err)
	}
	frozen, err := instance.Freeze()
	if err != nil {
		return result, trace, err
	}
	completed, err := workflow.SnapshotRead(frozen, r.keys.resultStages.final, "")
	if err != nil {
		return result, trace, err
	}
	input, err := workflow.SnapshotRead(frozen, r.keys.input, "")
	if err != nil {
		return result, trace, err
	}
	result = completed
	attachWorkflowDiagnostics(&result, trace)
	result, err = core.PublishResult(ctx, req, input, result)
	return result, trace, err
}

// publicError 职责：保留既有公开错误类别；使用：服务边界；边界：不把内核失败当作布局无解。
func publicError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return &blueprint.Error{Code: "GEN_E_CANCELED", Message: "蓝图生成已取消"}
		}
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "蓝图生成超过总截止时间"}
	}
	return projectError(err)
}

// projectError 职责：保留嵌套候选的全部真实领域原因；使用：公开错误映射；边界：不把内部no-candidate汇总码冒充最终物理原因。
func projectError(err error) error {
	if err == nil {
		return nil
	}
	if domain, ok := err.(*blueprint.Error); ok {
		return domain
	}
	if group, ok := err.(interface{ Unwrap() []error }); ok {
		causes := group.Unwrap()
		out := make([]error, 0, len(causes))
		for index := len(causes) - 1; index >= 0; index-- {
			projected := projectError(causes[index])
			var domain *blueprint.Error
			if errors.As(projected, &domain) && strings.HasPrefix(domain.Code, "GEN_I_") {
				continue
			}
			out = append(out, projected)
		}
		if len(out) > 0 {
			return errors.Join(out...)
		}
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "全部候选已证明等价或不优于现有边界，没有独立可行方案"}
	}
	if failure, ok := err.(*workflow.Failure); ok {
		if failure.Code != "no-candidate" && workflow.IsCandidateDeadline(failure.Cause) {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "候选布局计算超过时间预算，尚未完成完整蓝图"}
		}
		if failure.Cause != nil {
			return projectError(failure.Cause)
		}
		code := failure.Code
		if code == "no-candidate" {
			code = "GEN_E_LAYOUT"
		}
		return &blueprint.Error{Code: code, Message: failure.Message}
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if cause := wrapped.Unwrap(); cause != nil {
			return projectError(cause)
		}
	}
	return &blueprint.Error{Code: "GEN_E_WORKFLOW", Message: err.Error()}
}
