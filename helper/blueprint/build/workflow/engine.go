package workflow

// 文件职责：拥有全部遍历、候选和反射调度；使用：生成服务运行冻结计划；边界：工具只能计算和暂存，取消等待全部已启动工作退出。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// ScoreFunc 职责：从已验证冻结快照读取有序指标；使用：候选归约；边界：指标越小越优、必须有限、相同长度，完全相同以输入序号破平。
type ScoreFunc func(*FrozenSnapshot) ([]float64, error)

// Comparator 职责：按真实领域双方规则比较冻结候选；使用：命名比较器；边界：负数表示 a 更优，零使用稳定输入序号破平，必须无副作用。
type Comparator func(a, b *FrozenSnapshot) (int, error)

// CandidateEvent 职责：向外部预览报告引擎拥有的生命周期；使用：Observe；边界：不提供调度权限或可变实例，回调须响应局部 Context。
type CandidateEvent struct {
	// Kind 职责：描述 started/succeeded/rejected/selected；使用：预览适配；边界：succeeded 不等于任务成功。
	Kind string
	// Path 职责：稳定完整候选路径；使用：嵌套预览分流；边界：不能当作调度入口。
	Path string
	// Ordinal 职责：本组稳定输入序号；使用：身份破平；边界：不是并发完成顺序。
	Ordinal int
	// Context 职责：传播本组局部取消；使用：预览子会话；边界：父取消和本组截止均能触发。
	Context context.Context
	// Snapshot 职责：提供成功候选的只读快照；使用：最终预览；边界：started/rejected 时为空。
	Snapshot *FrozenSnapshot
	// Failure 职责：保留拒绝原因；使用：诊断；边界：不改变回退分类。
	Failure error
}

// RunOptions 职责：提供服务层评分和时限；使用：Run；边界：不把调度权限交给工具。
type RunOptions struct {
	// ObserveExecution 职责：同步旁路投递深复制执行事件；使用：后台journal；边界：须快返，错误/panic仅计lost，不影响预览Observe或业务事务，不启动新线程。
	ObserveExecution func(ExecutionEvent) error
	// Parallel 职责：收紧全局工具并发及各候选组工作区派发；使用：请求级并发选择；边界：零沿用计划，显式值为一至十且不超过计划上限，YAML仍是每组上限。
	Parallel int
	// Score 职责：默认候选评分器；使用：未指定 scorer 的组；边界：有候选时不能为空。
	Score ScoreFunc
	// Scorers 职责：命名评分器；使用：嵌套候选的不同目标；边界：任务开始时复制目录。
	Scorers map[string]ScoreFunc
	// Comparators 职责：提供具名双方比较器；使用：复杂真实领域选优；边界：运行开始复制目录，工具无法访问。
	Comparators map[string]Comparator
	// Observe 职责：同步报告候选生命周期；使用：服务预览适配；边界：错误或 panic 致命，回调可能并发调用。
	Observe func(CandidateEvent) error
	// Timeout 职责：任务总时限；使用：服务取消联动；边界：零值四分钟，负值拒绝。
	Timeout time.Duration
}

// Trace 职责：记录一次粗粒度调用的可解释证据；使用：RunResult；边界：参数是独立 JSON 字节，不持有工具参数别名。
type Trace struct {
	// Fingerprint 职责：完整冻结依赖身份；使用：复现；边界：不是运行成功标志。
	Fingerprint string
	// Node 职责：含区域及候选的调用路径；使用：定位；边界：稳定顺序由路径排序。
	Node string
	// Tool 职责：业务契约 ID；使用：追踪；边界：与 Go 方法名无关。
	Tool string
	// Implementation 职责：实现身份；使用：诊断版本；边界：由注册者维护。
	Implementation string
	// EffectiveParameters 职责：本次生效参数；使用：解释策略；边界：精确整数以 JSON 原数值保留。
	EffectiveParameters json.RawMessage
	// ParameterSources 职责：记录顶层覆盖来源；使用：调试优先级；边界：复合字段整体覆盖。
	ParameterSources map[string]string
	// RevisionBefore 职责：输入内容版本；使用：事务证据；边界：不代表所有产物都存在。
	RevisionBefore uint64
	// RevisionAfter 职责：提交后内容版本；使用：事务证据；边界：失败时与前值相同。
	RevisionAfter uint64
	// Failure 职责：错误文本；使用：诊断；边界：空值仅代表该步骤成功。
	Failure string
	// Diagnostics 职责：工具显式诊断；使用：失败解释；边界：不推进内容版本。
	Diagnostics []string
}

// RunResult 职责：返回执行轨迹及最终版本；使用：服务适配；边界：失败时同时返回已有轨迹，不能作为可导出成功。
type RunResult struct {
	// ExecutionEventsLost 职责：最终旁路观察失败次数；使用：任务终态；边界：不影响业务成功与实例版本。
	ExecutionEventsLost uint64
	// CandidateGroups 职责：记录条件组启用、跳过及真实失败树；使用：解释备用触发；边界：成功仅表示该组存在完整叶候选。
	CandidateGroups []CandidateGroupTrace
	// Fingerprint 职责：计划身份；使用：日志；边界：不包含可变领域数据。
	Fingerprint string
	// Trace 职责：稳定排序的调用证据；使用：诊断；边界：并行完成顺序不影响呈现。
	Trace []Trace
	// Revision 职责：最终主实例内容版本；使用：冻结输出；边界：需另用 Freeze 验证可编码。
	Revision uint64
	// Steps 职责：实际已调度工具数；使用：预算审计；边界：失败调用也计数。
	Steps int
	// Candidates 职责：任务累计候选数；使用：嵌套预算审计；边界：不是并发数。
	Candidates int
}

// budget 职责：在所有嵌套工作区共享计数与保留字节；使用：引擎；边界：父候选等待子组不占工具并发槽。
type budget struct {
	mu                sync.Mutex
	limits            Limits
	steps, candidates int
	bytes             int64
	semaphore         chan struct{}
}

// addCandidates 职责：原子预占全组候选计数；使用：展开动态列表后；边界：超限整组拒绝，不能部分搜索后宣称最优。
func (b *budget) addCandidates(n int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > b.limits.Candidates-b.candidates {
		return fmt.Errorf("candidate total budget exceeded")
	}
	b.candidates += n
	return nil
}

// step 职责：占用累计执行预算；使用：工具进入前；边界：不因失败退还。
func (b *budget) step() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.steps >= b.limits.Steps {
		return fmt.Errorf("execution step budget exceeded")
	}
	b.steps++
	return nil
}

// resize 职责：原子调整活跃候选保留字节；使用：克隆和提交；边界：预留失败不改变计数。
func (b *budget) resize(delta int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if delta > 0 && delta > b.limits.StateBytes-b.bytes {
		return fmt.Errorf("candidate state byte budget exceeded: additional=%d retained=%d limit=%d", delta, b.bytes, b.limits.StateBytes)
	}
	b.bytes += delta
	return nil
}

// workspace 职责：绑定固定实例及其候选内存计量；使用：嵌套调度；边界：一个工作区内默认顺序执行。
type workspace struct {
	scope     string
	instance  *BlueprintInstance
	region    string
	candidate int
	retained  int64
	counted   bool
}

// executor 职责：保存一次任务调度设施；使用：Run；边界：不复用任务参数和可变领域状态。
type executor struct {
	executionMu       sync.Mutex
	executionSequence uint64
	executionLost     uint64
	plan              *Plan
	options           RunOptions
	budget            *budget
	mu                sync.Mutex
	trace             []Trace
	groups            []CandidateGroupTrace
}

// Run 职责：执行冻结方案并等待全部已启动计算；使用：生成服务；边界：不捕获已声明无解以外的错误进行回退，不自动导出。
func Run(ctx context.Context, p *Plan, i *BlueprintInstance, options RunOptions) (result *RunResult, err error) {
	if ctx == nil || p == nil || i == nil {
		return nil, fmt.Errorf("context, plan and instance required")
	}
	if options.Timeout < 0 {
		return nil, fmt.Errorf("negative timeout")
	}
	if options.Parallel < 0 || options.Parallel > 10 || options.Parallel > p.limits.Parallel {
		return nil, fmt.Errorf("parallel override must be zero or 1..10 within plan limit")
	}
	if options.Timeout == 0 {
		options.Timeout = 4 * time.Minute
	}
	scorers := map[string]ScoreFunc{}
	for k, v := range options.Scorers {
		scorers[k] = v
	}
	options.Scorers = scorers
	comparators := map[string]Comparator{}
	for k, v := range options.Comparators {
		comparators[k] = v
	}
	options.Comparators = comparators
	i.mu.Lock()
	if i.running {
		i.mu.Unlock()
		return nil, fmt.Errorf("instance is already running")
	}
	for _, d := range p.registry.artifacts {
		if !i.defs[d] {
			i.mu.Unlock()
			return nil, fmt.Errorf("instance lacks registered artifact %s", d.name)
		}
	}
	i.running = true
	i.mu.Unlock()
	defer func() { i.mu.Lock(); i.running = false; i.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	parallel := p.limits.Parallel
	if options.Parallel > 0 {
		parallel = options.Parallel
	}
	e := &executor{plan: p, options: options, budget: &budget{limits: p.limits, semaphore: make(chan struct{}, parallel)}}
	runStarted := time.Now()
	root := &workspace{scope: p.id, instance: i, candidate: -1}
	runEvent := e.eventFor(nil, root, p.id, "", "run", "started")
	e.emitExecution(runEvent)
	defer func() {
		if x := recover(); x != nil {
			err = &ExecutionError{Node: p.id, Cause: fmt.Errorf("kernel panic: %v", x), Stack: string(debug.Stack())}
		}
		runEvent.Status = "completed"
		if err != nil {
			runEvent.Status = "failed"
		}
		runEvent.Failure, runEvent.FailureCode = executionFailure(err)
		runEvent.RevisionAfter = i.Revision()
		runEvent.DurationMs = float64(time.Since(runStarted)) / float64(time.Millisecond)
		e.emitExecution(runEvent)
		sort.Slice(e.trace, func(a, b int) bool { return e.trace[a].Node < e.trace[b].Node })
		sort.Slice(e.groups, func(a, b int) bool { return e.groups[a].Path < e.groups[b].Path })
		result = &RunResult{Fingerprint: p.fingerprint, Trace: e.trace, Revision: i.Revision(), Steps: e.budget.steps, Candidates: e.budget.candidates, CandidateGroups: e.groups, ExecutionEventsLost: e.executionLost}
	}()
	err = e.flow(ctx, p.flow, root, nil, p.id)
	if err == nil {
		err = ctx.Err()
	}
	return
}

// flow 职责：顺序执行同一作用域并隔离意外 panic；使用：所有调度结构；边界：不吞取消或工具故障。
func (e *executor) flow(ctx context.Context, nodes []*compiledNode, w *workspace, inputs map[string]any, path string, parentOverride ...string) (err error) {
	parent := path
	if len(parentOverride) > 0 {
		parent = parentOverride[0]
	}
	defer func() {
		if x := recover(); x != nil {
			err = &ExecutionError{Node: path, Cause: fmt.Errorf("kernel panic: %v", x), Stack: string(debug.Stack())}
		}
	}()
	for index, n := range nodes {
		if x := ctx.Err(); x != nil {
			e.skipFlow(nodes[index:], w, path, "parent-canceled", parent)
			return x
		}
		nodePath := path + "/" + n.ast.ID
		if n.tool != nil {
			err = e.call(ctx, n, w, inputs, nodePath, parent)
		} else {
			err = e.trackScope(e.eventFor(n, w, nodePath, parent, nodeType(n), "started"), w, func() error { return e.node(ctx, n, w, inputs, nodePath) })
		}
		if err != nil {
			e.skipFlow(nodes[index+1:], w, path, "prior-node-failed", parent)
			var located *ExecutionError
			if !errors.As(err, &located) {
				return &ExecutionError{Node: nodePath, Tool: n.ast.Tool, Cause: err}
			}
			return err
		}
	}
	return nil
}

// node 职责：执行一个已编译动作；使用：flow；边界：调度只在此层发生。
func (e *executor) node(ctx context.Context, n *compiledNode, w *workspace, inputs map[string]any, path string) error {
	switch {
	case n.tool != nil:
		return e.call(ctx, n, w, inputs, path)
	case n.ast.Fail != "":
		return &ExecutionError{Node: path, Cause: fmt.Errorf("explicit failure: %s", n.ast.Fail)}
	case n.ast.Sequence != nil:
		return e.flow(ctx, n.children, w, inputs, path)
	case n.ast.Call != "":
		v, err := resolve(ctx, e.plan.registry, n.with, inputs, w.instance, w.region, w.candidate, path, nil)
		if err != nil {
			return err
		}
		bound, err := checkInputs(v.(map[string]any), n.inputs)
		if err != nil {
			return err
		}
		return e.flow(ctx, n.children, w, bound, path+"/call:"+n.ast.Call, path)
	case n.ast.Switch != nil:
		v, err := resolve(ctx, e.plan.registry, n.items, inputs, w.instance, w.region, w.candidate, path, nil)
		if err != nil {
			return err
		}
		var label string
		switch x := v.(type) {
		case string:
			label = x
		case bool:
			label = fmt.Sprint(x)
		default:
			return fmt.Errorf("%s: switch requires string/boolean", path)
		}
		if branch, ok := n.cases[label]; ok {
			event := e.eventFor(n, w, path, strings.TrimSuffix(path, "/"+n.ast.ID), "switch", "selected")
			event.Selection = label
			e.emitExecution(event)
			for name, unselected := range n.cases {
				if name != label {
					e.skipFlow(unselected, w, path+"/case:"+name, "branch-not-selected", path)
				}
			}
			e.skipFlow(n.other, w, path+"/otherwise", "branch-not-selected", path)
			return e.flow(ctx, branch, w, inputs, path+"/case:"+label, path)
		}
		event := e.eventFor(n, w, path, strings.TrimSuffix(path, "/"+n.ast.ID), "switch", "selected")
		event.Selection = "otherwise"
		e.emitExecution(event)
		for name, unselected := range n.cases {
			e.skipFlow(unselected, w, path+"/case:"+name, "branch-not-selected", path)
		}
		return e.flow(ctx, n.other, w, inputs, path+"/otherwise", path)
	case n.ast.Regions != nil:
		v, err := resolve(ctx, e.plan.registry, n.items, inputs, w.instance, w.region, w.candidate, path, nil)
		if err != nil {
			return err
		}
		regions, err := regionItems(v, e.plan.limits.Regions)
		if err != nil {
			return err
		}
		if err = w.instance.bindRegions(regions, n.ast.Regions.Reset); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for index, r := range regions {
			regionWork := &workspace{scope: w.scope, instance: w.instance, region: r, candidate: w.candidate, retained: w.retained, counted: w.counted}
			regionPath := path + "/region:" + r
			err = e.trackScope(e.eventFor(n, regionWork, regionPath, path, "region", "started"), regionWork, func() error { return e.flow(ctx, n.children, regionWork, inputs, regionPath) })
			w.retained = regionWork.retained
			if err != nil {
				for _, unstarted := range regions[index+1:] {
					skipped := e.eventFor(n, w, path+"/region:"+unstarted, path, "region", "skipped")
					skipped.Region = unstarted
					skipped.Selection = "prior-region-failed"
					e.emitExecution(skipped)
				}
				return err
			}
		}
		return nil
	case n.ast.Candidates != nil:
		return e.candidates(ctx, n, w, inputs, path)
	}
	return fmt.Errorf("%s: unknown compiled node", path)
}

// sameStrings 职责：检查固定区域集合；使用：后续遍历；边界：顺序不同也视为目标变化。
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for j := range a {
		if a[j] != b[j] {
			return false
		}
	}
	return true
}

// call 职责：解析参数、反射调用和原子提交；使用：工具节点；边界：panic 致命、取消后不提交，恢复点同时记录栈。
func (e *executor) call(ctx context.Context, n *compiledNode, w *workspace, inputs map[string]any, path string, parentOverride ...string) (err error) {
	t := n.tool
	parent := strings.TrimSuffix(path, "/"+n.ast.ID)
	if len(parentOverride) > 0 {
		parent = parentOverride[0]
	}
	event := e.eventFor(n, w, path, parent, "tool", "started")
	started := time.Now()
	sentStart := false
	sources := map[string]string{}
	for k, v := range n.sources {
		sources[k] = v
	}
	tr := Trace{Fingerprint: e.plan.fingerprint, Node: path, Tool: t.spec.ID, Implementation: t.spec.Implementation, RevisionBefore: w.instance.Revision(), ParameterSources: sources}
	tr.RevisionAfter = tr.RevisionBefore
	startEvent := func() {
		if sentStart {
			return
		}
		event.EffectiveParameters, event.ParameterSources, event.ParametersTruncated = traceParameters(t, tr.EffectiveParameters, tr.ParameterSources)
		e.emitExecution(event)
		sentStart = true
	}
	var step *StepContext
	defer func() {
		if x := recover(); x != nil {
			err = &ExecutionError{Node: path, Tool: t.spec.ID, Cause: fmt.Errorf("tool panic: %v", x), Stack: string(debug.Stack())}
		}
		if step != nil {
			step.expire()
			step.mu.Lock()
			tr.Diagnostics = append([]string(nil), step.diagnostics...)
			step.mu.Unlock()
		}
		if err != nil {
			tr.Failure = err.Error()
		}
		tr.RevisionAfter = w.instance.Revision()
		startEvent()
		event.Status = "completed"
		if err != nil {
			event.Status = "failed"
		}
		event.Failure, event.FailureCode = executionFailure(err)
		event.RevisionAfter = tr.RevisionAfter
		event.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		event.Diagnostics, event.DiagnosticsTruncated = boundedDiagnostics(tr.Diagnostics)
		e.emitExecution(event)
		e.mu.Lock()
		e.trace = append(e.trace, tr)
		e.mu.Unlock()
	}()
	if err = e.budget.step(); err != nil {
		return err
	}
	step, err = beginStep(w.instance, t.spec, w.region, w.candidate, path)
	if err != nil {
		return err
	}
	step.scope = w.scope
	if w.counted {
		step.reserve = e.budget.resize
	}
	values, err := resolve(ctx, e.plan.registry, n.args, inputs, w.instance, w.region, w.candidate, path, step)
	if err != nil {
		return err
	}
	tr.EffectiveParameters, _ = json.Marshal(values)
	args, err := t.bind(values.(map[string]any), path)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	startEvent()
	select {
	case e.budget.semaphore <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	var returned []reflect.Value
	// 逻辑流：只在算法计算期间占全局并发槽，父候选等待嵌套组时不会占槽造成死锁。
	func() {
		defer func() { <-e.budget.semaphore }()
		if ctx.Err() != nil {
			return
		}
		returned = t.method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(step), args})
	}()
	if err = ctx.Err(); err != nil {
		return err
	}
	if len(returned) != 1 {
		return fmt.Errorf("invalid reflected return")
	}
	if !returned[0].IsNil() {
		cause := returned[0].Interface().(error)
		var failure *Failure
		recoverable := false
		if errors.As(cause, &failure) {
			for _, code := range t.spec.FailureCodes {
				if failure.Code == code {
					recoverable = true
				}
			}
		}
		code := ""
		if recoverable {
			code = failure.Code
		}
		return &ExecutionError{Node: path, Tool: t.spec.ID, Cause: cause, Recoverable: recoverable, declaredCode: code}
	}
	var resize func(int64) error
	oldRetained, newRetained, reserved := w.retained, w.retained, int64(0)
	transferred := int64(0)
	if w.counted {
		resize = func(size int64) error {
			newRetained = size
			delta := size - oldRetained
			if delta > 0 {
				// Stage已为暂存和提交副本预留空间；提交时转移这笔额度，不把同一副本再计一次。
				// commit持有step.mu；失败时不转移，原暂存额度仍由expire统一归还。
				transferred = min(delta, step.stagedBytes)
				if x := e.budget.resize(delta - transferred); x != nil {
					return x
				}
				reserved = delta - transferred
			}
			return nil
		}
	}
	err = step.commit(ctx, resize)
	if w.counted {
		if err != nil {
			_ = e.budget.resize(-reserved)
		} else {
			step.mu.Lock()
			step.stagedBytes -= transferred
			step.mu.Unlock()
			if newRetained < oldRetained {
				_ = e.budget.resize(newRetained - oldRetained)
			}
			w.retained = newRetained
		}
	}
	return err
}
