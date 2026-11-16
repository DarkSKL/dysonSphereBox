package workflow

// 文件职责：解释有限失败依赖组及受失败码限制的备用；使用：候选节点；边界：复用唯一叶执行器，不复制调度代码，不从前组 winner 继续修改。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

// CandidateGroupTrace 职责：保存条件组实际参与证据；使用：RunResult；边界：存在 winner 不等于全部组完成比较。
type CandidateGroupTrace struct {
	// Path 职责：记录完整条件组作用域；使用：关联叶轨迹；边界：包含稳定输入组序。
	Path string
	// ID 职责：保存方案组标识；使用：依赖解释；边界：不提供调度入口。
	ID string
	// Ordinal 职责：保存本组在冻结表中的序号；使用：稳定呈现；边界：不是完成顺序。
	Ordinal int
	// Status 职责：区分 enabled/skipped/empty/canceled/not-started；使用：解释备用与预算截止；边界：不把未开始组当作无解。
	Status string
	// Reason 职责：说明依赖或截止原因；使用：诊断；边界：不参与领域评分。
	Reason string
	// HasSuccess 职责：说明本组存在完整验证的叶结果；使用：条件依赖；边界：落选组也可能成功。
	HasSuccess bool
	// Complete 职责：说明本组许可的全部叶均完成评估；使用：区分局部截止；边界：跳过与取消组为 false。
	Complete bool
	// Failure 职责：保留本组全部真实失败树；使用：回退和诊断；边界：成功组也可保存被拒叶原因。
	Failure error
}

// decodeCandidateGroups 职责：冻结组运输表并验证全部依赖；使用：编译与运行；边界：未知/前向/自引用依赖和过量叶在任何工具执行前拒绝。
func decodeCandidateGroups(value any, max int) ([]CandidateGroup, error) {
	data, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var groups []CandidateGroup
	if e = decoder.Decode(&groups); e != nil {
		return nil, fmt.Errorf("candidate groups: %w", e)
	}
	if len(groups) > max {
		return nil, fmt.Errorf("candidate group table budget exceeded")
	}
	seen := map[string]bool{}
	total := 0
	for _, group := range groups {
		if group.ID == "" || seen[group.ID] {
			return nil, fmt.Errorf("empty or duplicate candidate group id %q", group.ID)
		}
		dependencies := map[string]bool{}
		for _, id := range group.AfterNoSuccess {
			if !seen[id] || dependencies[id] {
				return nil, fmt.Errorf("group %s: dependency %s must uniquely name an earlier group", group.ID, id)
			}
			dependencies[id] = true
		}
		for _, code := range group.ExcludeFailureCodes {
			if code == "" {
				return nil, fmt.Errorf("group %s: empty excluded failure code", group.ID)
			}
		}
		if len(group.Items) > max-total {
			return nil, fmt.Errorf("candidate group leaf budget exceeded")
		}
		total += len(group.Items)
		seen[group.ID] = true
	}
	return groups, nil
}

// bindCandidateItems 职责：一次性绑定全部叶参数；使用：普通列表和条件表；边界：在开始工具前拒绝未知、缺失或错误类型输入。
func bindCandidateItems(value any, rules map[string]Parameter) ([]map[string]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("candidate items must be array")
	}
	bound := make([]map[string]any, len(items))
	for j, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("candidate %d must be object", j)
		}
		var e error
		bound[j], e = checkInputs(m, rules)
		if e != nil {
			return nil, fmt.Errorf("candidate %d: %w", j, e)
		}
	}
	return bound, nil
}

// noCandidateError 职责：包装原始叶失败而不伪造算法失败；使用：无可行项出口；边界：空失败树仍不可触发有真实失败条件的后组。
func noCandidateError(path string, failures []error) error {
	return &ExecutionError{Node: path, Cause: &Failure{Code: "no-candidate", Message: "all permitted attempts have no solution", Cause: errors.Join(failures...), synthetic: true}, Recoverable: true}
}

// nonExcludedFailure 职责：寻找声明真实无解叶；使用：条件组触发；边界：内核 no-candidate 汇总、局部截止与空剪枝都不绕过排除列表。
func nonExcludedFailure(err error, excluded []string) bool {
	return approvedFailure(err, func(actual string) bool {
		for _, code := range excluded {
			if actual == code {
				return false
			}
		}
		return true
	}, false)
}

// approvedFailure 职责：只遍历被调用器确认的失败边界；使用：组条件与推进过滤；边界：已确认工具错误的原始 Cause 仅作诊断，不能夹带未声明类别触发调度。
func approvedFailure(err error, accept func(string) bool, includeTimeout bool) bool {
	if err == nil {
		return false
	}
	if execution, ok := err.(*ExecutionError); ok && execution.declaredCode != "" {
		return execution.Recoverable && execution.declaredCode != "no-candidate" && accept(execution.declaredCode)
	}
	if failure, ok := err.(*Failure); ok && includeTimeout && failure.synthetic && failure.Code != "no-candidate" && accept(failure.Code) {
		return true
	}
	switch value := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range value.Unwrap() {
			if approvedFailure(child, accept, includeTimeout) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return approvedFailure(value.Unwrap(), accept, includeTimeout)
	}
	return false
}

// matchesApprovedFailure 职责：匹配真实已声明类别及显式截止类别；使用：FallbackOn/AdvanceOn；边界：不以汇总包装码或未授权 Cause 作为触发依据。
func matchesApprovedFailure(err error, codes []string) bool {
	return approvedFailure(err, func(actual string) bool {
		for _, code := range codes {
			if code == actual {
				return true
			}
		}
		return false
	}, true)
}

// recordGroup 职责：并发保存嵌套组证据；使用：执行器；边界：最终按稳定路径排序。
func (e *executor) recordGroup(n *compiledNode, w *workspace, record CandidateGroupTrace, started time.Time) {
	e.mu.Lock()
	e.groups = append(e.groups, record)
	e.mu.Unlock()
	status := "completed"
	if record.Status == "skipped" || record.Status == "empty" || record.Status == "not-started" {
		status = "skipped"
	} else if record.Failure != nil && !record.HasSuccess {
		status = "failed"
	}
	event := e.eventFor(n, w, record.Path, strings.TrimSuffix(record.Path, "/"+record.Path[strings.LastIndex(record.Path, "/")+1:]), "group", status)
	event.Selection = record.Status + ":" + record.Reason
	event.Failure, event.FailureCode = executionFailure(record.Failure)
	if !started.IsZero() {
		event.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
	}
	e.emitExecution(event)
}

// preferCandidate 职责：复用节点真实比较规则归约完整候选；使用：组间选优；边界：相同指标保留先前组及其较早叶。
func (e *executor) preferCandidate(n *compiledNode, a, b *candidateResult) (preferred bool, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &ExecutionError{Node: a.path, Cause: fmt.Errorf("candidate comparator panic: %v", p), Stack: string(debug.Stack())}
		}
	}()
	if b == nil {
		return true, nil
	}
	if id := n.ast.Candidates.Comparator; id != "" {
		compare := e.options.Comparators[id]
		if compare == nil {
			return false, fmt.Errorf("comparator %q is not registered", id)
		}
		order, err := compare(a.snapshot, b.snapshot)
		return order < 0, err
	}
	if len(a.score) != len(b.score) {
		return false, fmt.Errorf("inconsistent score dimensions")
	}
	return better(a.score, b.score), nil
}

// adoptCandidate 职责：只发布最终叶胜者并整体采用；使用：普通或分组搜索出口；边界：selected 指向真实叶子树，父取消不能采用。
func (e *executor) adoptCandidate(ctx context.Context, n *compiledNode, parent *workspace, best *candidateResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.observeCandidate(n, parent, CandidateEvent{Kind: "selected", Path: best.path, Ordinal: best.index, Context: best.context, Snapshot: best.snapshot}, time.Time{}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if best.baseline {
		return nil
	}
	if err := parent.instance.adopt(ctx, best.work.instance); err != nil {
		return err
	}
	if parent.counted {
		// adopt转移同一完整状态的所有权；父旧图释放，胜者额度随图转交，不制造第三份领域图预算。
		_ = e.budget.resize(-parent.retained)
		parent.retained = best.work.retained
		best.work.retained = 0
		best.work.counted = false
	}
	parent.instance.mu.Lock()
	parent.instance.origin = resultOrigin{best.path, best.index}
	parent.instance.mu.Unlock()
	return nil
}

// candidateFallback 职责：按明确失败码执行一次备用并保留双侧原因；使用：无可行候选出口；边界：未知错误、panic 与父取消不能被恢复。
func (e *executor) candidateFallback(ctx context.Context, n *compiledNode, parent *workspace, inputs map[string]any, path string, primary error) error {
	if !recoverable(primary) || n.ast.Candidates.Fallback == nil {
		return primary
	}
	if len(n.ast.Candidates.FallbackOn) > 0 && !matchesApprovedFailure(primary, n.ast.Candidates.FallbackOn) {
		return primary
	}
	if err := ctx.Err(); err != nil {
		return &ExecutionError{Node: path, Cause: errors.Join(primary, err)}
	}
	backup := e.flow(ctx, n.other, parent, inputs, path+"/fallback", path)
	if backup == nil {
		return nil
	}
	var located *ExecutionError
	stack := ""
	if errors.As(backup, &located) {
		stack = located.Stack
	}
	return &ExecutionError{Node: path + "/fallback", Cause: errors.Join(primary, backup), Recoverable: recoverable(backup), Stack: stack}
}

// candidateGroups 职责：顺序启用失败依赖组并比较整体最佳叶；使用：配送等有条件策略；边界：每个组从原父工作区搜索，只有最后采用，局部截止明确记录未开始组。
func (e *executor) candidateGroups(ctx context.Context, n *compiledNode, parent *workspace, inputs map[string]any, path string) error {
	value, err := resolve(ctx, e.plan.registry, n.groups, inputs, parent.instance, parent.region, parent.candidate, path, nil)
	if err != nil {
		return err
	}
	groups, err := decodeCandidateGroups(value, e.plan.limits.Candidates)
	if err != nil {
		return err
	}
	bound := make([][]map[string]any, len(groups))
	for j, group := range groups {
		items, err := transport(group.Items)
		if err != nil {
			return err
		}
		bound[j], err = bindCandidateItems(items, n.inputs)
		if err != nil {
			return fmt.Errorf("group %s: %w", group.ID, err)
		}
	}
	states := map[string]CandidateGroupTrace{}
	defer func() {
		for index, group := range groups {
			if _, ok := states[group.ID]; ok {
				continue
			}
			reason := "prior-group-failed"
			if ctx.Err() != nil {
				reason = "parent-canceled"
			}
			e.recordGroup(n, parent, CandidateGroupTrace{Path: fmt.Sprintf("%s/group:%06d:%s", path, index, url.PathEscape(group.ID)), ID: group.ID, Ordinal: index, Status: "not-started", Reason: reason}, time.Time{})
		}
	}()
	var failures []error
	var best *candidateResult
	defer func() { e.releaseResult(best) }()
	stopped := false
	spec := n.ast.Candidates
	cutoff := candidateCutoff(ctx, spec, time.Now())
	for j, group := range groups {
		record := CandidateGroupTrace{Path: fmt.Sprintf("%s/group:%06d:%s", path, j, url.PathEscape(group.ID)), ID: group.ID, Ordinal: j, Status: "enabled", Reason: "unconditional"}
		if err = ctx.Err(); err != nil {
			return err
		}
		if !cutoff.IsZero() && !time.Now().Before(cutoff) {
			stopped = true
		}
		if stopped {
			record.Status = "not-started"
			record.Reason = "local-timeout"
			e.recordGroup(n, parent, record, time.Time{})
			states[group.ID] = record
			continue
		}
		enabled := len(group.AfterNoSuccess) == 0
		for _, dependency := range group.AfterNoSuccess {
			previous := states[dependency]
			if !previous.HasSuccess && nonExcludedFailure(previous.Failure, group.ExcludeFailureCodes) {
				enabled = true
				break
			}
		}
		if !enabled {
			record.Status = "skipped"
			record.Reason = "no-triggering-failure"
			e.recordGroup(n, parent, record, time.Time{})
			states[group.ID] = record
			continue
		}
		if len(bound[j]) == 0 {
			record.Status = "empty"
			record.Reason = "no-candidates"
			record.Complete = true
			e.recordGroup(n, parent, record, time.Time{})
			states[group.ID] = record
			continue
		}
		if len(group.AfterNoSuccess) > 0 {
			record.Reason = "earlier-group-failed"
		}
		leafNode := *n
		leafSpec := *spec
		leafSpec.Groups = nil
		leafSpec.Fallback = nil
		leafSpec.FallbackOn = nil
		leafNode.ast.Candidates = &leafSpec
		groupStarted := time.Now()
		e.emitExecution(e.eventFor(n, parent, record.Path, path, "group", "started"))
		candidate, searchErr := e.searchCandidates(ctx, &leafNode, parent, bound[j], record.Path, cutoff)
		if candidate != nil {
			defer e.releaseResult(candidate)
			record.HasSuccess = true
			record.Failure = candidate.failures
			record.Complete = !candidate.stopped
			if candidate.stopped {
				record.Status = "canceled"
				record.Reason = "local-timeout"
				stopped = true
			}
		}
		if searchErr != nil {
			record.Failure = searchErr
			record.Complete = recoverable(searchErr)
			if errors.Is(searchErr, localDeadline) {
				record.Status = "canceled"
				record.Reason = "local-timeout"
				record.Complete = false
				stopped = true
			} else if !recoverable(searchErr) {
				record.Status = "canceled"
				record.Reason = "fatal-or-parent-canceled"
			}
			failures = append(failures, searchErr)
		}
		e.recordGroup(n, parent, record, groupStarted)
		states[group.ID] = record
		if err = ctx.Err(); err != nil {
			return err
		}
		if searchErr != nil && !recoverable(searchErr) && !errors.Is(searchErr, localDeadline) {
			return searchErr
		}
		if candidate != nil {
			prefer, err := e.preferCandidate(n, candidate, best)
			if err != nil {
				return err
			}
			if prefer {
				e.releaseResult(best)
				best = candidate
			} else {
				e.releaseResult(candidate)
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if best == nil {
		if stopped && spec.TimeoutFailure == "" {
			return &ExecutionError{Node: path, Cause: context.DeadlineExceeded}
		}
		if stopped && !matchesApprovedFailure(errors.Join(failures...), []string{spec.TimeoutFailure}) {
			failures = append(failures, &Failure{Code: spec.TimeoutFailure, Message: "local optimization deadline", Cause: localDeadline, synthetic: true})
		}
		return e.candidateFallback(ctx, n, parent, inputs, path, noCandidateError(path, failures))
	}
	return e.adoptCandidate(ctx, n, parent, best)
}
