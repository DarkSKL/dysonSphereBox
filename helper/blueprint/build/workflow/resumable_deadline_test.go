package workflow

// 文件职责：验证可恢复父任务与普通工作流的时限边界；使用：暂停续算回归；边界：以短时窗和真实候选调度验证，不等待产品中的分钟级时限。

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"DysonSphereBox/helper/industryplan/search"
)

// deadlineProbe 职责：提供反射工具中的等待点；使用：时限回归；边界：不创建领域产物。
type deadlineProbe struct {
	run func(context.Context, bool) error
}

// deadlineArgs 职责：区分立即验证与暂停验证；使用：候选参数；边界：仅测试工具使用。
type deadlineArgs struct {
	Pause bool `json:"pause"`
}

// Verify 职责：执行测试等待并提交验证；使用：工作流白名单；边界：等待必须响应传入的取消信号。
func (p *deadlineProbe) Verify(ctx context.Context, _ *StepContext, args deadlineArgs) error {
	return p.run(ctx, args.Pause)
}

// TestWorkflowResumableDeadlines 职责：确认暂停不会耗尽子时限或遗失后续候选；使用：总时限、候选与两条 settle 路径回归；边界：普通调用仍按原时限退出。
func TestWorkflowResumableDeadlines(t *testing.T) {
	for _, mode := range []string{"total", "items", "groups", "settle", "current"} {
		for _, controlled := range []bool{false, true} {
			name := mode + "/ordinary"
			if controlled {
				name = mode + "/resumable"
			}
			t.Run(name, func(t *testing.T) { testResumableDeadline(t, mode, controlled, false) })
		}
	}
	t.Run("paused_parent_cancellation", func(t *testing.T) { testResumableDeadline(t, "items", true, true) })
}

// TestFirstLayoutTrialCutoffs 职责：核对短父时窗能进入邻带；使用：以同一起点分别计算单带和收尾截止；边界：普通工作台与可暂停任务保留各自的时限语义。
func TestFirstLayoutTrialCutoffs(t *testing.T) {
	start := time.Now()
	short, cancelShort := context.WithDeadline(t.Context(), start.Add(12*time.Second))
	defer cancelShort()
	long, cancelLong := context.WithDeadline(t.Context(), start.Add(time.Minute))
	defer cancelLong()
	for _, tc := range []struct {
		name           string
		ctx            context.Context
		options        RunOptions
		single, finish time.Time
	}{
		{"short", short, RunOptions{FirstVerifiedLayout: true}, start.Add(4 * time.Second), start.Add(10500 * time.Millisecond)},
		{"long", long, RunOptions{FirstVerifiedLayout: true}, start.Add(20 * time.Second), start.Add(40 * time.Second)},
		{"workbench", short, RunOptions{}, start.Add(20 * time.Second), start.Add(-8 * time.Second)},
		{"resumable", short, RunOptions{FirstVerifiedLayout: true, UseParentDeadline: true}, time.Time{}, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := executor{options: tc.options}
			single := e.candidateCutoff(tc.ctx, &Candidates{Selection: "first", TimeoutMs: 20000}, start)
			finish := e.candidateCutoff(tc.ctx, &Candidates{Selection: "first", ReserveMs: 20000}, start)
			if !single.Equal(tc.single) || !finish.Equal(tc.finish) {
				t.Fatalf("单带截止 %v，邻带截止 %v，预期 %v / %v", single, finish, tc.single, tc.finish)
			}
		})
	}
}

// TestFirstVerifiedLayout 检查跨星装配的首个合规模板，以及普通工作台仍然执行完整的模板比较和塔位优化。
func TestFirstVerifiedLayout(t *testing.T) {
	for _, quick := range []bool{false, true} {
		calls := atomic.Int32{}
		probe := &deadlineProbe{run: func(context.Context, bool) error { calls.Add(1); return nil }}
		registry, err := NewRegistry([]ToolSpec{{ID: "test.verify@1", Receiver: probe, Method: "Verify", Implementation: "verified-layout-test", Role: Validate}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		validate := Node{ID: "validate", Tool: "test.verify@1", With: map[string]any{"pause": false}}
		options := func(name string, baseline bool) *Candidates {
			return &Candidates{Comparator: name, IncludeCurrent: baseline, SettleMs: 5000, Parallel: 1,
				Items: []map[string]any{{}, {}}, Flow: []Node{validate}}
		}
		doc := &Document{SchemaVersion: 1, ID: "verified-layout-test", Flow: []Node{
			{ID: "layout", Candidates: options("layout", false)},
			{ID: "refine", Candidates: options("station-refinement", true)},
		}}
		plan, err := Compile(doc, registry, CompileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		instance, err := NewInstance(nil)
		if err != nil {
			t.Fatal(err)
		}
		equal := func(*FrozenSnapshot, *FrozenSnapshot) (int, error) { return 0, nil }
		if _, err := Run(t.Context(), plan, instance, RunOptions{FirstVerifiedLayout: quick,
			Comparators: map[string]Comparator{"layout": equal, "station-refinement": equal}}); err != nil {
			t.Fatal(err)
		}
		want := int32(4)
		if quick {
			want = 1
		}
		if calls.Load() != want {
			t.Fatalf("优先有效模板=%t，完整布局验证调用 %d 次，预期 %d 次", quick, calls.Load(), want)
		}
	}
}

// testResumableDeadline 职责：运行一次暂停或超时场景；使用：表驱动回归；边界：以父取消清理全部工作，不使用轮询。
func testResumableDeadline(t *testing.T, mode string, controlled, cancelPaused bool) {
	t.Helper()
	entered := make(chan struct{}, 1)
	var started, completed atomic.Int32
	probe := &deadlineProbe{run: func(ctx context.Context, pause bool) error {
		started.Add(1)
		if pause {
			if controlled {
				if !search.Controlled(ctx) {
					return errors.New("子工具遗失可恢复任务上下文")
				}
				if err := search.Checkpoint(ctx); err != nil {
					return err
				}
			} else {
				entered <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			}
		}
		completed.Add(1)
		return nil
	}}
	registry, err := NewRegistry([]ToolSpec{{ID: "test.verify@1", Receiver: probe, Method: "Verify", Implementation: "deadline-test", Role: Validate}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait := Node{ID: "wait", Tool: "test.verify@1", With: map[string]any{"pause": true}}
	flow := []Node{wait}
	expected := int32(1)
	options := RunOptions{Timeout: time.Second, UseParentDeadline: controlled, Score: func(*FrozenSnapshot) ([]float64, error) { return []float64{0}, nil }}
	if mode == "total" {
		options.Timeout = 20 * time.Millisecond
	} else {
		wait.With = map[string]any{"pause": map[string]any{"$input": "pause"}}
		spec := &Candidates{Items: []map[string]any{{"pause": true}}, Inputs: map[string]Parameter{"pause": {Type: "boolean", Required: true}}, Flow: []Node{wait}, Parallel: 1}
		switch mode {
		case "items":
			spec.TimeoutMs, spec.ReserveMs = 20, 1
		case "groups":
			spec.TimeoutMs, spec.ReserveMs = 20, 1
			spec.Items = nil
			spec.Groups = []CandidateGroup{{ID: "first", Items: []map[string]any{{"pause": true}}}, {ID: "later", Items: []map[string]any{{"pause": false}}}}
			expected = 2
		case "settle":
			spec.Items = []map[string]any{{"pause": false}, {"pause": true}}
			spec.SettleMs = 20
			expected = 2
		case "current":
			spec.IncludeCurrent, spec.SettleMs = true, 20
			expected = 2
		}
		flow = []Node{{ID: "candidates", Candidates: spec}}
		if mode == "current" {
			flow = append([]Node{{ID: "baseline", Tool: "test.verify@1", With: map[string]any{"pause": false}}}, flow...)
		}
	}
	plan, err := Compile(&Document{SchemaVersion: 1, ID: "deadline-test", Flow: flow}, registry, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	control := search.New(time.Millisecond, func() { entered <- struct{}{} })
	if controlled {
		// 先让时窗明确到期，避免 Windows 同一计时刻度内尚未越过截止点。
		timer := time.NewTimer(2 * time.Millisecond)
		<-timer.C
		ctx = search.WithControl(ctx, control)
	}
	finished := make(chan error, 1)
	go func() { _, err := Run(ctx, plan, instance, options); finished <- err }()
	select {
	case <-entered:
	case err := <-finished:
		t.Fatalf("等待点之前任务已退出：%v", err)
	case <-ctx.Done():
		t.Fatal("任务未进入等待点")
	}
	if controlled && !cancelPaused {
		// 暂停超过全部 20ms 子时限；只有显式 Continue 能释放当前调用栈。
		timer := time.NewTimer(60 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-finished:
			t.Fatalf("暂停中的任务被子时限销毁：%v", err)
		case <-timer.C:
		}
		if !control.Continue() {
			t.Fatal("原检查点未保留暂停状态")
		}
	} else if cancelPaused {
		cancel()
	}
	select {
	case err = <-finished:
	case <-time.After(time.Second):
		t.Fatal("任务没有响应继续、超时或父取消")
	}
	switch {
	case cancelPaused:
		if !errors.Is(err, context.Canceled) || completed.Load() != 0 {
			t.Fatalf("暂停任务没有响应父取消：%v，已完成 %d", err, completed.Load())
		}
	case controlled:
		if err != nil || started.Load() != expected || completed.Load() != expected {
			t.Fatalf("继续没有完成原候选及后续候选：%v，启动 %d，完成 %d，预期 %d", err, started.Load(), completed.Load(), expected)
		}
	case mode == "settle" || mode == "current":
		if err != nil || completed.Load() != 1 {
			t.Fatalf("普通 settle 应保留先前成功候选：%v，完成 %d", err, completed.Load())
		}
	default:
		if !errors.Is(err, context.DeadlineExceeded) || completed.Load() != 0 {
			t.Fatalf("普通调用没有执行原时限：%v，完成 %d", err, completed.Load())
		}
	}
}
