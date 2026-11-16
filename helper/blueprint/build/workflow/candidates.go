package workflow

// 文件职责：调度嵌套有界候选、局部优化截止和确定性归约；使用：引擎候选节点；边界：只有声明无解允许推进，父取消及致命失败必须取消并等待整波。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"time"
)

// candidateResult 职责：保存一次隔离尝试；使用：按输入序号归约；边界：不拼接不同候选的状态。
type candidateResult struct {
	work          *workspace
	snapshot      *FrozenSnapshot
	score         []float64
	err           error
	index         int
	snapshotBytes int64
	path          string
	context       context.Context
	failures      error
	stopped       bool
	baseline      bool
	cancel        context.CancelCauseFunc
}

// localDeadline 职责：标记本组显式截止；使用：允许保留完整 winner；边界：与父任务取消区分。
var localDeadline = errors.New("candidate local optimization deadline")

// localSettle 职责：标记成功后的优化宽限结束；使用：允许保留完整 winner；边界：不属于领域算法失败。
var localSettle = errors.New("candidate settle period elapsed")

// release 职责：归还候选状态预算；使用：淘汰或采用后；边界：每个工作区仅释放一次。
func (e *executor) release(w *workspace) {
	if w != nil && w.counted {
		_ = e.budget.resize(-w.retained)
		w.counted = false
		w.retained = 0
		w.instance = nil
	}
}

// releaseResult 职责：释放候选及其比较快照；使用：确定性归约；边界：冻结快照由内核拥有，观察者保留时自行负责外部内存。
func (e *executor) releaseResult(r *candidateResult) {
	if r == nil {
		return
	}
	if r.cancel != nil {
		r.cancel(nil)
		r.cancel = nil
	}
	e.release(r.work)
	if r.snapshotBytes > 0 {
		_ = e.budget.resize(-r.snapshotBytes)
		r.snapshotBytes = 0
	}
	r.snapshot = nil
	r.work = nil
	r.score = nil
}

// candidateWork 职责：预留预算再克隆候选；使用：调度波次；边界：任一领域克隆失败致命且释放预留。
func (e *executor) candidateWork(parent *workspace, index int) (*workspace, error) {
	parent.instance.mu.Lock()
	expected, err := stateSize(parent.instance.slots)
	parent.instance.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err = e.budget.resize(expected); err != nil {
		return nil, err
	}
	instance, size, err := parent.instance.clone()
	if err != nil {
		_ = e.budget.resize(-expected)
		return nil, err
	}
	if err = e.budget.resize(size - expected); err != nil {
		_ = e.budget.resize(-expected)
		return nil, err
	}
	return &workspace{instance: instance, region: parent.region, candidate: index, retained: size, counted: true}, nil
}

// better 职责：比较有限有序指标；使用：按输入顺序归约；边界：完全相同保留较早输入。
func better(a, b []float64) bool {
	for j := range a {
		if a[j] < b[j] {
			return true
		}
		if a[j] > b[j] {
			return false
		}
	}
	return false
}

// recoverable 职责：识别调用器确认的无解；使用：候选回退；边界：取消、panic、预算和未知错误不能推进。
func recoverable(err error) bool { var x *ExecutionError; return errors.As(err, &x) && x.Recoverable }

// matchesFailure 职责：遍历完整原始包装及聚合错误树；使用：诊断与回归；边界：不代替调度器对已声明类别的权限过滤。
func matchesFailure(err error, codes []string) bool {
	if err == nil {
		return false
	}
	if failure, ok := err.(*Failure); ok {
		for _, code := range codes {
			if failure.Code == code {
				return true
			}
		}
	}
	switch x := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range x.Unwrap() {
			if matchesFailure(child, codes) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return matchesFailure(x.Unwrap(), codes)
	}
	return false
}

// observe 职责：发送调度生命周期；使用：外部预览适配；边界：回调不能获得主实例或调度设施。
func (e *executor) observe(event CandidateEvent) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = &ExecutionError{Node: event.Path, Cause: fmt.Errorf("candidate observer panic: %v", x), Stack: string(debug.Stack())}
		}
	}()
	if e.options.Observe == nil {
		return nil
	}
	return e.options.Observe(event)
}

// candidates 职责：冻结候选列表或条件组并采用完整结果；使用：编译节点；边界：所有叶调度复用同一执行器，工具不能自行遍历候选。
func (e *executor) candidates(ctx context.Context, n *compiledNode, parent *workspace, inputs map[string]any, path string) error {
	if n.ast.Candidates.IncludeCurrent {
		parent.instance.mu.Lock()
		valid := parent.region == "" && parent.instance.validationValid("")
		parent.instance.mu.Unlock()
		if !valid {
			return fmt.Errorf("%s: includeCurrent parent is not fully validated", path)
		}
	}
	if n.groups != nil {
		return e.candidateGroups(ctx, n, parent, inputs, path)
	}
	values, err := resolve(ctx, e.plan.registry, n.items, inputs, parent.instance, parent.region, parent.candidate, path, nil)
	if err != nil {
		return err
	}
	bound, err := bindCandidateItems(values, n.inputs)
	if err != nil {
		return err
	}
	if len(bound) == 0 && !n.ast.Candidates.IncludeCurrent {
		return fmt.Errorf("%s: candidate items must be nonempty array", path)
	}
	best, err := e.searchCandidates(ctx, n, parent, bound, path, time.Time{})
	if err != nil {
		return e.candidateFallback(ctx, n, parent, inputs, path, err)
	}
	defer e.releaseResult(best)
	return e.adoptCandidate(ctx, n, parent, best)
}

// searchCandidates 职责：执行一组有界叶候选并交回获选工作区所有权；使用：items 与条件 groups 共用；边界：不执行备用、不发布 selected，也不修改原父工作区。
func (e *executor) searchCandidates(ctx context.Context, n *compiledNode, parent *workspace, bound []map[string]any, path string, cutoff time.Time) (winner *candidateResult, err error) {
	spec := n.ast.Candidates
	if err = e.budget.addCandidates(len(bound)); err != nil {
		return nil, err
	}
	items := bound
	first := spec.Selection == "first"
	score := e.options.Score
	if n.scorer != "" {
		score = e.options.Scorers[n.scorer]
	}
	var compare Comparator
	if spec.Comparator != "" {
		compare = e.options.Comparators[spec.Comparator]
		if compare == nil {
			return nil, fmt.Errorf("%s: comparator %q is not registered", path, spec.Comparator)
		}
	}
	if !first && compare == nil && score == nil && !(spec.IncludeCurrent && len(bound) == 0 && n.scorer == "") {
		return nil, fmt.Errorf("%s: candidate scorer %q is not registered", path, n.scorer)
	}
	parallel := spec.Parallel
	if parallel == 0 {
		parallel = e.plan.limits.Parallel
	}
	// 请求并发同时限制工作区派发，避免只限工具槽却预留整波大型图。
	parallel = min(parallel, cap(e.budget.semaphore))
	groupCtx, cancel := context.WithCancelCause(ctx)
	scheduled := 0
	defer func() {
		for index := scheduled; index < len(bound); index++ {
			event := e.eventFor(n, parent, fmt.Sprintf("%s/candidate:%06d", path, index), path, "candidate", "skipped")
			event.Candidate = index
			event.Tool = ""
			event.Selection = "not-scheduled"
			if ctx.Err() != nil {
				event.Selection = "parent-canceled"
			} else if groupCtx.Err() != nil {
				event.Selection = "local-search-stopped"
			}
			e.emitExecution(event)
		}
	}()
	defer func() {
		if winner != nil {
			winner.cancel = cancel
		} else {
			cancel(nil)
		}
	}()
	var timer *time.Timer
	deadline := candidateCutoff(ctx, spec, time.Now())
	if !cutoff.IsZero() && (deadline.IsZero() || cutoff.Before(deadline)) {
		deadline = cutoff
	}
	if !deadline.IsZero() {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			cancel(localDeadline)
		} else {
			timer = time.AfterFunc(remaining, func() { cancel(localDeadline) })
			defer timer.Stop()
		}
	}
	var settleOnce sync.Once
	var settleTimer *time.Timer
	defer func() {
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()
	var fatalMu sync.Mutex
	var fatal error
	fatalError := func(failure error) {
		if failure == nil {
			return
		}
		fatalMu.Lock()
		if fatal == nil {
			fatal = failure
		}
		fatalMu.Unlock()
		cancel(failure)
	}
	var best *candidateResult
	defer func() {
		if winner != best {
			e.releaseResult(best)
		}
	}()
	if spec.IncludeCurrent {
		best, err = e.currentCandidate(ctx, parent, score, compare)
		if err != nil {
			return nil, err
		}
		best.context = groupCtx
		if spec.SettleMs > 0 {
			settleOnce.Do(func() {
				settleTimer = time.AfterFunc(time.Duration(spec.SettleMs)*time.Millisecond, func() { cancel(localSettle) })
			})
		}
	}
	var failures []error
	stop := false
	localStopped := false
	for start := 0; start < len(items) && !stop; start += parallel {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if groupCtx.Err() != nil {
			localStopped = true
			break
		}
		end := start + parallel
		if end > len(items) {
			end = len(items)
		}
		wave := make([]candidateResult, end-start)
		scheduled = end
		var wait sync.WaitGroup
		for index := start; index < end; index++ {
			offset := index - start
			wave[offset].index = index
			wait.Add(1)
			go func(index, offset int) {
				defer wait.Done()
				r := &wave[offset]
				candidatePath := fmt.Sprintf("%s/candidate:%06d", path, index)
				r.path = candidatePath
				r.context = groupCtx
				candidateStarted := time.Now()
				defer func() {
					if x := recover(); x != nil {
						r.err = &ExecutionError{Node: candidatePath, Cause: fmt.Errorf("candidate panic: %v", x), Stack: string(debug.Stack())}
					}
					if r.err != nil {
						if !recoverable(r.err) && !((errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded)) && groupCtx.Err() != nil) {
							fatalError(r.err)
						}
						if x := e.observeCandidate(n, parent, CandidateEvent{Kind: "rejected", Path: candidatePath, Ordinal: index, Context: groupCtx, Failure: r.err}, candidateStarted); x != nil {
							fatalError(x)
						}
					}
				}()
				if r.err = groupCtx.Err(); r.err != nil {
					return
				}
				if r.err = e.observeCandidate(n, parent, CandidateEvent{Kind: "started", Path: candidatePath, Ordinal: index, Context: groupCtx}, time.Time{}); r.err != nil {
					return
				}
				r.work, r.err = e.candidateWork(parent, index)
				if r.err != nil {
					return
				}
				r.work.scope = candidatePath
				r.err = e.flow(groupCtx, n.children, r.work, bound[index], candidatePath)
				if r.err != nil {
					return
				}
				// 工作区与比较快照共同存活；封闭图已经由工作区计量，只有可变克隆和容器新增空间。
				r.work.instance.mu.Lock()
				extra, sizeErr := stateCloneSize(r.work.instance.slots)
				r.work.instance.mu.Unlock()
				if sizeErr != nil {
					r.err = sizeErr
					return
				}
				if r.err = e.budget.resize(extra); r.err != nil {
					return
				}
				r.snapshotBytes = extra
				r.snapshot, r.err = r.work.instance.freezeFor(r.work.region)
				if r.err != nil {
					return
				}
				if !first && compare == nil {
					r.score, r.err = score(r.snapshot)
					if r.err != nil {
						return
					}
					r.score = append([]float64(nil), r.score...)
					for _, v := range r.score {
						if math.IsNaN(v) || math.IsInf(v, 0) {
							r.err = fmt.Errorf("nonfinite candidate score")
							return
						}
					}
				}
				if r.err = e.observeCandidate(n, parent, CandidateEvent{Kind: "succeeded", Path: candidatePath, Ordinal: index, Context: groupCtx, Snapshot: r.snapshot}, candidateStarted); r.err != nil {
					return
				}
				if spec.SettleMs > 0 {
					settleOnce.Do(func() {
						settleTimer = time.AfterFunc(time.Duration(spec.SettleMs)*time.Millisecond, func() { cancel(localSettle) })
					})
				}
			}(index, offset)
		}
		wait.Wait()
		fatalMu.Lock()
		waveFatal := fatal
		fatalMu.Unlock()
		if ctx.Err() != nil {
			waveFatal = ctx.Err()
		}
		if waveFatal != nil {
			for j := range wave {
				e.releaseResult(&wave[j])
			}
			return nil, waveFatal
		}
		if groupCtx.Err() != nil {
			cause := context.Cause(groupCtx)
			if cause != localDeadline && cause != localSettle {
				for j := range wave {
					e.releaseResult(&wave[j])
				}
				return nil, cause
			}
			localStopped = true
		}
		for j := range wave {
			r := &wave[j]
			if r.err != nil {
				if recoverable(r.err) {
					failures = append(failures, r.err)
					if first && len(spec.AdvanceOn) > 0 && !matchesApprovedFailure(r.err, spec.AdvanceOn) {
						stop = true
					}
				}
				e.releaseResult(r)
				continue
			}
			if stop {
				e.releaseResult(r)
				continue
			}
			choose := best == nil
			if best != nil && !first {
				var x error
				choose, x = e.preferCandidate(n, r, best)
				if x != nil {
					for k := j; k < len(wave); k++ {
						e.releaseResult(&wave[k])
					}
					return nil, x
				}
			}
			if choose {
				e.releaseResult(best)
				best = r
			} else {
				e.releaseResult(r)
			}
			if first {
				stop = true
			}
		}
		if localStopped {
			break
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if best == nil {
		if localStopped {
			if spec.TimeoutFailure == "" {
				return nil, &ExecutionError{Node: path, Cause: errors.Join(context.DeadlineExceeded, localDeadline)}
			}
			failures = append(failures, &Failure{Code: spec.TimeoutFailure, Message: "local optimization deadline", Cause: localDeadline, synthetic: true})
		}
		return nil, noCandidateError(path, failures)
	}
	if !best.baseline {
		best.path = fmt.Sprintf("%s/candidate:%06d", path, best.index)
	}
	best.context = groupCtx
	best.failures = errors.Join(failures...)
	best.stopped = localStopped
	return best, nil
}
