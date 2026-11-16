package workflow

// 文件职责：导出有界纯值执行事件与隔离观察边界；使用：后台跟踪 journal；边界：不传实体、实例能力、快照或 context，不建立队列或后台线程。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"
)

// ExecutionEvent 职责：记录真实节点及控制作用域的增量活动；使用：ObserveExecution；边界：纯 JSON 深复制 DTO，观察失败不改变业务。
type ExecutionEvent struct {
	// Sequence 职责：单 Run 串行发射序号；使用：journal 排序；边界：包含观察失败造成的缺口。
	Sequence uint64 `json:"sequence"`
	// Timestamp 职责：记录发射时刻；使用：持续时间展示；边界：序号才是可靠顺序。
	Timestamp time.Time `json:"timestamp"`
	// Fingerprint 职责：关联冻结定义；使用：图关联；边界：不包含运行状态。
	Fingerprint string `json:"fingerprint"`
	// InstanceID 职责：关联唯一任务实例；使用：候选隔离解释；边界：不授予访问实例权限。
	InstanceID string `json:"instanceID"`
	// Path 职责：实际动态作用域路径；使用：区分候选/区域；边界：不能作为工具调用入口。
	Path string `json:"path"`
	// DefinitionPath 职责：关联 PlanDescription 节点；使用：动态活动投影；边界：控制活动可关联其父定义。
	DefinitionPath string `json:"definitionPath"`
	// ParentPath 职责：实际运行父作用域；使用：重建控制树；边界：根事件为空。
	ParentPath string `json:"parentPath,omitempty"`
	// Type 职责：区分 run/tool/sequence/call/switch/candidates/regions/candidate/region/group；使用：渲染节点；边界：不表达业务完成百分比。
	Type string `json:"type"`
	// Status 职责：区分 started/completed/failed/selected/skipped；使用：增量状态；边界：候选成功不等于主任务成功。
	Status string `json:"status"`
	// Tool 职责：稳定业务工具 ID；使用：定位；边界：控制节点可为空。
	Tool string `json:"tool,omitempty"`
	// Candidate 职责：固定候选序号；使用：运行身份；边界：根作用域为负一。
	Candidate int `json:"candidate"`
	// Region 职责：固定区域 ID；使用：区域汇合；边界：根作用域为空。
	Region string `json:"region,omitempty"`
	// RevisionBefore 职责：活动开始内容版本；使用：事务解释；边界：不包含领域值。
	RevisionBefore uint64 `json:"revisionBefore"`
	// RevisionAfter 职责：活动结束内容版本；使用：提交证据；边界：诊断更新不推进版本。
	RevisionAfter uint64 `json:"revisionAfter"`
	// EffectiveParameters 职责：只含工具白名单顶层策略参数；使用：解释真实调用；边界：最多16KiB，超限省略。
	EffectiveParameters json.RawMessage `json:"effectiveParameters,omitempty"`
	// ParameterSources 职责：白名单参数的覆盖来源；使用：解释优先级；边界：不输出其他字段名。
	ParameterSources map[string]string `json:"parameterSources,omitempty"`
	// DurationMs 职责：活动墙钟时长；使用：性能观察；边界：候选并行耗时不可相加为总时长。
	DurationMs float64 `json:"durationMs"`
	// Selection 职责：记录分支、区域或组状态选择；使用：控制活动；边界：不包含实体数据。
	Selection string `json:"selection,omitempty"`
	// Failure 职责：有界错误文本；使用：诊断；边界：不附 panic 栈或内部对象。
	Failure string `json:"failure,omitempty"`
	// FailureCode 职责：失败分类；使用：取消/领域/内核解释；边界：不改变实际恢复决策。
	FailureCode string `json:"failureCode,omitempty"`
	// ParametersTruncated 职责：表示参数超限未输出；使用：UI 明确缺口；边界：不截断为非法 JSON。
	ParametersTruncated bool `json:"parametersTruncated,omitempty"`
	// EventsLost 职责：此前观察错误或 panic 数量；使用：任务终态展示；边界：最终回调自身失败由 RunResult 兜底。
	EventsLost uint64 `json:"eventsLost,omitempty"`
	// Diagnostics 职责：有界记录工具显式实际参数与规则适用性；使用：终态解释；边界：最多64项及16KiB，工具不能放入领域实体。
	Diagnostics []string `json:"diagnostics,omitempty"`
	// DiagnosticsTruncated 职责：表示诊断达到输出上限；使用：明确展示缺口；边界：不改变实际规则结果。
	DiagnosticsTruncated bool `json:"diagnosticsTruncated,omitempty"`
}

// boundedText 职责：限制观察文本字节与保留合法 UTF-8；使用：错误诊断；边界：不改变原业务错误。
func boundedText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	text = text[:max]
	for !utf8.ValidString(text) && len(text) > 0 {
		text = text[:len(text)-1]
	}
	return text
}

// executionFailure 职责：投影安全失败码；使用：终态事件；边界：原始错误及栈仍由业务返回值保留。
func executionFailure(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	code := "kernel"
	if errors.Is(err, context.Canceled) {
		code = "canceled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
	} else {
		var failure *Failure
		if errors.As(err, &failure) {
			code = failure.Code
		}
	}
	return boundedText(err.Error(), 2048), code
}

// emitExecution 职责：串行分配序号并同步交付隔离观察；使用：引擎真实边界；边界：回调必须快返，不持实例/事务锁，错误和 panic 仅计丢失。
func (e *executor) emitExecution(event ExecutionEvent) {
	if e.options.ObserveExecution == nil {
		return
	}
	e.executionMu.Lock()
	defer e.executionMu.Unlock()
	e.executionSequence++
	event.Sequence = e.executionSequence
	event.Timestamp = time.Now().UTC()
	event.Fingerprint = e.plan.fingerprint
	event.EventsLost = e.executionLost
	defer func() {
		if recover() != nil {
			e.executionLost++
		}
	}()
	event.EffectiveParameters = append(json.RawMessage(nil), event.EffectiveParameters...)
	if event.ParameterSources != nil {
		sources := map[string]string{}
		for k, v := range event.ParameterSources {
			sources[k] = v
		}
		event.ParameterSources = sources
	}
	event.Diagnostics = append([]string(nil), event.Diagnostics...)
	if err := e.options.ObserveExecution(event); err != nil {
		e.executionLost++
	}
}

// eventFor 职责：构造安全节点身份快照；使用：实际运行边界；边界：只读取版本数字，不暴露实例引用。
func (e *executor) eventFor(n *compiledNode, w *workspace, path, parent, kind, status string) ExecutionEvent {
	revision := w.instance.Revision()
	event := ExecutionEvent{InstanceID: w.instance.ID(), Path: path, ParentPath: parent, Type: kind, Status: status, Candidate: w.candidate, Region: w.region, RevisionBefore: revision, RevisionAfter: revision}
	if n != nil {
		event.DefinitionPath = n.definitionID
		event.Tool = n.ast.Tool
	}
	return event
}

// trackScope 职责：成对发送真实控制作用域终态；使用：run、区域及控制节点；边界：panic 保持致命语义，旁路故障不会替换原始错误。
func (e *executor) trackScope(event ExecutionEvent, w *workspace, run func() error) (err error) {
	started := time.Now()
	e.emitExecution(event)
	defer func() {
		if p := recover(); p != nil {
			err = &ExecutionError{Node: event.Path, Cause: fmt.Errorf("kernel panic: %v", p), Stack: string(debug.Stack())}
		}
		event.Status = "completed"
		if err != nil {
			event.Status = "failed"
		}
		event.Failure, event.FailureCode = executionFailure(err)
		event.RevisionAfter = w.instance.Revision()
		event.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		e.emitExecution(event)
	}()
	return run()
}

// boundedDiagnostics 职责：限制旁路工具诊断；使用：完成及失败事件；边界：不复制无界诊断文本或截成非法UTF8。
func boundedDiagnostics(values []string) ([]string, bool) {
	out := make([]string, 0, min(64, len(values)))
	remaining := (16 << 10) - 130
	truncated := false
	for j, text := range values {
		if j >= 64 || remaining == 0 {
			truncated = true
			break
		}
		bounded := boundedText(text, remaining)
		encoded, _ := json.Marshal(bounded)
		for len(encoded) > remaining && len(bounded) > 0 {
			bounded = boundedText(bounded, len(bounded)/2)
			encoded, _ = json.Marshal(bounded)
		}
		if len(encoded) > remaining {
			truncated = true
			break
		}
		if len(bounded) < len(text) {
			truncated = true
		}
		out = append(out, bounded)
		remaining -= len(encoded)
	}
	return out, truncated
}

// skipFlow 职责：记录未运行节点明确跳过；使用：取消后的剩余顺序及未选分支；边界：不执行参数求值或工具。
func (e *executor) skipFlow(nodes []*compiledNode, w *workspace, path, reason string, parentOverride ...string) {
	parent := path
	if len(parentOverride) > 0 {
		parent = parentOverride[0]
	}
	for _, n := range nodes {
		event := e.eventFor(n, w, path+"/"+n.ast.ID, parent, nodeType(n), "skipped")
		event.Selection = reason
		e.emitExecution(event)
	}
}

// observeCandidate 职责：在既有预览回调之外独立发射纯值候选活动；使用：候选真实生命周期；边界：旁路故障不影响预览既有错误语义。
func (e *executor) observeCandidate(n *compiledNode, w *workspace, event CandidateEvent, started time.Time) error {
	parent := event.Path
	if index := strings.LastIndex(parent, "/candidate:"); index >= 0 {
		parent = parent[:index]
	}
	status := event.Kind
	switch status {
	case "succeeded":
		status = "completed"
	case "rejected":
		status = "failed"
	}
	activity := e.eventFor(n, w, event.Path, parent, "candidate", status)
	activity.Tool = ""
	activity.Candidate = event.Ordinal
	activity.Failure, activity.FailureCode = executionFailure(event.Failure)
	if event.Snapshot != nil {
		activity.RevisionAfter = event.Snapshot.Revision()
		activity.Region = event.Snapshot.Region()
	}
	if !started.IsZero() {
		activity.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
	}
	e.emitExecution(activity)
	return e.observe(event)
}
