package workflow

// 文件职责：独立审计不可变共享、可变深复制和候选额度所有权的对抗性边界。
// 使用：本轮审验定向运行及race检查；边界：只使用本文件夹具，不修改生产实现或既有测试，不运行大型蓝图生成。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ownershipAuditMutable 职责：覆盖嵌套可写引用；使用：复制隔离夹具；边界：大小按实际保留元素估算，闭包可注入失败。
type ownershipAuditMutable struct {
	data []byte
	rows map[string][]int
}

// ownershipAuditClone 职责：深复制每层可写引用；使用：普通产物键；边界：不共享切片后备数组或map。
func ownershipAuditClone(v ownershipAuditMutable) (ownershipAuditMutable, error) {
	out := ownershipAuditMutable{data: append([]byte(nil), v.data...), rows: make(map[string][]int, len(v.rows))}
	for k, row := range v.rows {
		out.rows[k] = append([]int(nil), row...)
	}
	return out, nil
}

// ownershipAuditSize 职责：计算夹具领域负载；使用：独立预算预期；边界：不包含内核固定槽位管理成本。
func ownershipAuditSize(v ownershipAuditMutable) int64 {
	size := int64(len(v.data))
	for k, row := range v.rows {
		size += int64(len(k) + 8*len(row))
	}
	return size
}

// ownershipAuditValue 职责：创建可辨识的嵌套负载；使用：不同候选及旧图；边界：创建后只经普通键克隆进入引擎。
func ownershipAuditValue(n int, tag byte) ownershipAuditMutable {
	v := ownershipAuditMutable{data: make([]byte, n), rows: map[string][]int{"r": {int(tag), 7}}}
	if n > 0 {
		v.data[0] = tag
	}
	return v
}

// ownershipAuditFrozen 职责：模拟封闭不可变图句柄；使用：共享克隆夹具；边界：payload发布后不再修改。
type ownershipAuditFrozen struct{ payload *[1000]byte }

// ownershipAuditTool 职责：通过真实反射调用注入暂存与故障；使用：审计计划；边界：action由测试在启动前固定且保证并发安全。
type ownershipAuditTool struct {
	action func(context.Context, *StepContext) error
}

// Execute 职责：执行审计夹具动作；使用：真实工具注册器；边界：不访问调度器或生产领域对象。
func (tool *ownershipAuditTool) Execute(ctx context.Context, step *StepContext, _ struct{}) error {
	return tool.action(ctx, step)
}

// ownershipAuditPlan 职责：编译仅含审计工具的真实流程；使用：事务和嵌套搜索；边界：保留生产契约校验，不构造假已编译工具。
func ownershipAuditPlan(t *testing.T, spec ToolSpec, flow string, limit int64) *Plan {
	t.Helper()
	spec.ID, spec.Method, spec.Implementation = "audit.ownership@1", "Execute", "ownership-audit-20261116"
	registry, err := NewRegistry([]ToolSpec{spec}, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Load([]byte("schemaVersion: 1\nid: ownership\nflow:\n"+flow), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Compile(doc, registry, CompileOptions{Initial: spec.Requires, Limits: Limits{StateBytes: limit, Parallel: 4, Candidates: 4096}})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// ownershipAuditExecutor 职责：创建可读取计量的真实执行器；使用：精确预算审验；边界：root是否计量由每个测试显式确定。
func ownershipAuditExecutor(plan *Plan, options RunOptions) *executor {
	return &executor{plan: plan, options: options, budget: &budget{limits: plan.limits, semaphore: make(chan struct{}, 4)}}
}

// ownershipAuditBytes 职责：在预算锁内读取余额；使用：并发审验；边界：不写入或纠正被审计计数。
func ownershipAuditBytes(e *executor) int64 {
	e.budget.mu.Lock()
	defer e.budget.mu.Unlock()
	return e.budget.bytes
}

// ownershipAuditCheck 职责：比对真实槽位和工作区额度；使用：每个成功或失败出口；边界：不把计量错误自动归零。
func ownershipAuditCheck(t *testing.T, e *executor, w *workspace, expected int64) {
	t.Helper()
	if actual := ownershipAuditBytes(e); actual != expected || actual < 0 {
		t.Fatalf("budget=%d expected=%d retained=%d counted=%t", actual, expected, w.retained, w.counted)
	}
	if w.counted && w.retained != expected {
		t.Fatalf("retained=%d expected=%d", w.retained, expected)
	}
}

// TestOwnershipAuditStateCloneSize 职责：逐槽核验共享、可变及已消费状态的大小模型；使用：混合快照预算；边界：同时覆盖负值、panic及总和溢出拒绝。
func TestOwnershipAuditStateCloneSize(t *testing.T) {
	mutable, _ := NewArtifactKey("audit.mutable", ownershipAuditClone, ownershipAuditSize, ArtifactOptions{})
	frozen, _ := NewImmutableArtifactKey("audit.frozen", func(ownershipAuditFrozen) int64 { return 1000 }, ArtifactOptions{})
	v := ownershipAuditValue(40, 3)
	slots := map[slotID]slot{{def: mutable.def}: {value: v}, {def: frozen.def}: {value: ownershipAuditFrozen{new([1000]byte)}}}
	full, err := stateSize(slots)
	if err != nil {
		t.Fatal(err)
	}
	extra, err := stateCloneSize(slots)
	if err != nil || full != 128+256+ownershipAuditSize(v)+1000 || extra != full-1000 {
		t.Fatalf("full=%d extra=%d err=%v", full, extra, err)
	}
	slots[slotID{def: mutable.def}] = slot{consumed: true}
	extra, err = stateCloneSize(slots)
	if err != nil || extra != 384 {
		t.Fatalf("consumed extra=%d err=%v", extra, err)
	}
	for _, scenario := range []string{"negative", "panic", "overflow"} {
		key, _ := NewArtifactKey("audit.bad", func(v int) (int, error) { return v, nil }, func(int) int64 {
			if scenario == "panic" {
				panic("size injection")
			}
			if scenario == "negative" {
				return -1
			}
			return math.MaxInt64
		}, ArtifactOptions{})
		for _, measure := range []func(map[slotID]slot) (int64, error){stateSize, stateCloneSize} {
			if _, err := measure(map[slotID]slot{{def: key.def}: {value: 1}}); err == nil {
				t.Fatalf("accepted %s", scenario)
			}
		}
	}
}

// TestOwnershipAuditStageFailureMatrix 职责：检查暂存、提交及退还的故障矩阵；使用：真实call；边界：精确额度、失败零发布、重复expire/release不得重复归还。
func TestOwnershipAuditStageFailureMatrix(t *testing.T) {
	for _, scenario := range []string{"success", "stage-clone-error", "stage-clone-panic", "commit-clone-error", "commit-clone-panic", "tool-error", "tool-panic", "tool-cancel", "commit-cancel", "commit-growth-budget", "size-negative", "size-panic", "size-overflow"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var clones atomic.Int32
			key, err := NewArtifactKey("audit.payload", func(v ownershipAuditMutable) (ownershipAuditMutable, error) {
				n := clones.Add(1)
				if (n == 1 && strings.HasPrefix(scenario, "stage-clone")) || (n == 2 && strings.HasPrefix(scenario, "commit-clone")) {
					if strings.HasSuffix(scenario, "panic") {
						panic("clone injection")
					}
					return ownershipAuditMutable{}, errors.New("clone injection")
				}
				if n == 2 && scenario == "commit-cancel" {
					cancel()
				}
				out, err := ownershipAuditClone(v)
				if scenario == "commit-growth-budget" {
					out.data = make([]byte, len(v.data)*4)
				}
				return out, err
			}, func(v ownershipAuditMutable) int64 {
				switch scenario {
				case "size-negative":
					return -1
				case "size-panic":
					panic("size injection")
				case "size-overflow":
					return math.MaxInt64
				}
				return ownershipAuditSize(v)
			}, ArtifactOptions{Diagnostic: true})
			if err != nil {
				t.Fatal(err)
			}
			tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error {
				v := ownershipAuditValue(1000, 7)
				if err := Stage(step, key, v); err != nil {
					return err
				}
				v.data[0], v.rows["r"][0] = 99, 99
				switch scenario {
				case "tool-error":
					return errors.New("tool injection")
				case "tool-panic":
					panic("tool injection")
				case "tool-cancel":
					cancel()
				}
				return nil
			}}
			plan := ownershipAuditPlan(t, ToolSpec{Receiver: tool, Writes: []Artifact{key.Ref()}, Provides: []Artifact{key.Ref()}, Role: Validate}, "  - id: publish\n    tool: audit.ownership@1\n", 3000)
			instance, _ := NewInstance([]Artifact{key.Ref()})
			e := ownershipAuditExecutor(plan, RunOptions{})
			w := &workspace{instance: instance, candidate: -1, counted: true, retained: 128}
			if err := e.budget.resize(128); err != nil {
				t.Fatal(err)
			}
			err = e.call(ctx, plan.flow[0], w, nil, "ownership/publish")
			if scenario != "success" {
				if err == nil || len(instance.slots) != 0 || instance.hasValidation {
					t.Fatalf("fault published: %v", err)
				}
				ownershipAuditCheck(t, e, w, 128)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				want := int64(256) + ownershipAuditSize(ownershipAuditValue(1000, 7))
				ownershipAuditCheck(t, e, w, want)
				actual := instance.slots[slotID{def: key.def}].value.(ownershipAuditMutable)
				if actual.data[0] != 7 || actual.rows["r"][0] != 7 {
					t.Fatal("staging mutable alias escaped")
				}
			}
			e.release(w)
			e.release(w)
			ownershipAuditCheck(t, e, w, 0)
		})
	}
}

// TestOwnershipAuditExactStageReserve 职责：在一字节边界核验不可变单份和可变双份预留；使用：真实暂存工具；边界：重复暂存时旧副本仍存活，不提前归还旧输入额度。
func TestOwnershipAuditExactStageReserve(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("shared=%t/short=%t", shared, short), func(t *testing.T) {
				key, _ := NewArtifactKey("audit.frozen", func(v ownershipAuditFrozen) (ownershipAuditFrozen, error) {
					out := *v.payload
					return ownershipAuditFrozen{&out}, nil
				}, func(ownershipAuditFrozen) int64 { return 1000 }, ArtifactOptions{Diagnostic: true})
				if shared {
					key, _ = NewImmutableArtifactKey("audit.frozen", func(ownershipAuditFrozen) int64 { return 1000 }, ArtifactOptions{Diagnostic: true})
				}
				reserved := int64(2256)
				if shared {
					reserved = 1256
				}
				limit := int64(128) + reserved
				if short {
					limit--
				}
				value := ownershipAuditFrozen{new([1000]byte)}
				tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error { return Stage(step, key, value) }}
				plan := ownershipAuditPlan(t, ToolSpec{Receiver: tool, Writes: []Artifact{key.Ref()}, Provides: []Artifact{key.Ref()}, Role: Validate}, "  - id: publish\n    tool: audit.ownership@1\n", limit)
				instance, _ := NewInstance([]Artifact{key.Ref()})
				e := ownershipAuditExecutor(plan, RunOptions{})
				w := &workspace{instance: instance, counted: true, retained: 128}
				_ = e.budget.resize(128)
				err := e.call(context.Background(), plan.flow[0], w, nil, "ownership/publish")
				if short {
					if err == nil {
						t.Fatal("one-byte over-limit accepted")
					}
					ownershipAuditCheck(t, e, w, 128)
				} else {
					if err != nil {
						t.Fatal(err)
					}
					ownershipAuditCheck(t, e, w, 1256)
					actual := instance.slots[slotID{def: key.def}].value.(ownershipAuditFrozen)
					if (actual.payload == value.payload) != shared {
						t.Fatal("incorrect shared/mutable clone identity")
					}
				}
				e.release(w)
				ownershipAuditCheck(t, e, w, 0)
			})
		}
	}
}

// ownershipAuditValidatedParent 职责：经真实工具建立带来源的完整父验证；使用：baseline与采用审计；边界：计量状态明确，不直接伪造验证标志。
func ownershipAuditValidatedParent(t *testing.T, counted bool) (*executor, *workspace, ArtifactKey[ownershipAuditMutable], ArtifactKey[ownershipAuditFrozen], *compiledNode) {
	t.Helper()
	mutable, _ := NewArtifactKey("audit.mutable", ownershipAuditClone, ownershipAuditSize, ArtifactOptions{Diagnostic: true})
	frozen, _ := NewImmutableArtifactKey("audit.frozen", func(ownershipAuditFrozen) int64 { return 1000 }, ArtifactOptions{Diagnostic: true})
	tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error {
		if err := Stage(step, mutable, ownershipAuditValue(64, byte(step.Candidate()+3))); err != nil {
			return err
		}
		return Stage(step, frozen, ownershipAuditFrozen{new([1000]byte)})
	}}
	plan := ownershipAuditPlan(t, ToolSpec{Receiver: tool, Writes: []Artifact{mutable.Ref(), frozen.Ref()}, Provides: []Artifact{mutable.Ref(), frozen.Ref()}, Role: Validate, FailureCodes: []string{"audit-no-solution"}}, "  - id: publish\n    tool: audit.ownership@1\n", 1<<20)
	i, _ := NewInstance([]Artifact{mutable.Ref(), frozen.Ref()})
	e := ownershipAuditExecutor(plan, RunOptions{Score: func(s *FrozenSnapshot) ([]float64, error) {
		v, err := SnapshotRead(s, mutable, "")
		if err != nil {
			return nil, err
		}
		return []float64{float64(v.data[0])}, nil
	}})
	w := &workspace{instance: i, candidate: -1, counted: counted}
	if counted {
		w.retained = 128
		_ = e.budget.resize(128)
	}
	if err := e.call(context.Background(), plan.flow[0], w, nil, "ownership/publish"); err != nil {
		t.Fatal(err)
	}
	node := &compiledNode{definitionID: "ownership/pick", ast: Node{ID: "pick", Candidates: &Candidates{Selection: "best", Parallel: 3, IncludeCurrent: true}}, children: plan.flow}
	return e, w, mutable, frozen, node
}

// TestOwnershipAuditBaselineLifetime 职责：核验includeCurrent基线保留、失败退款和可变读取隔离；使用：计量父及外部root两种所有者；边界：基线生存期间其共享图的父所有者始终存活。
func TestOwnershipAuditBaselineLifetime(t *testing.T) {
	for _, counted := range []bool{false, true} {
		t.Run(fmt.Sprint(counted), func(t *testing.T) {
			e, parent, mutable, frozen, node := ownershipAuditValidatedParent(t, counted)
			initial := ownershipAuditBytes(e)
			full, _ := stateSize(parent.instance.slots)
			extra, _ := stateCloneSize(parent.instance.slots)
			want := full
			if counted {
				want = extra
			}
			baseline, err := e.currentCandidate(context.Background(), parent, e.options.Score, nil)
			if err != nil {
				t.Fatal(err)
			}
			if baseline.snapshotBytes != want || ownershipAuditBytes(e) != initial+want || !baseline.baseline {
				t.Fatalf("baseline not owned: %+v", baseline)
			}
			copy, _ := SnapshotRead(baseline.snapshot, mutable, "")
			copy.data[0], copy.rows["r"][0] = 99, 99
			again, _ := SnapshotRead(baseline.snapshot, mutable, "")
			if again.data[0] == 99 || again.rows["r"][0] == 99 {
				t.Fatal("baseline mutable read aliases")
			}
			shared, _ := SnapshotRead(baseline.snapshot, frozen, "")
			if shared.payload != parent.instance.slots[slotID{def: frozen.def}].value.(ownershipAuditFrozen).payload {
				t.Fatal("immutable graph copied")
			}
			e.releaseResult(baseline)
			e.releaseResult(baseline)
			ownershipAuditCheck(t, e, parent, initial)
			for _, scenario := range []string{"score-error", "nonfinite", "cancel", "missing-origin", "tight-budget"} {
				ctx, cancel := context.WithCancel(context.Background())
				score := ScoreFunc(func(*FrozenSnapshot) ([]float64, error) {
					if scenario == "score-error" {
						return nil, errors.New("score injection")
					}
					return []float64{math.NaN()}, nil
				})
				origin, limit := parent.instance.origin, e.budget.limits.StateBytes
				if scenario == "cancel" {
					cancel()
				}
				if scenario == "missing-origin" {
					parent.instance.origin = resultOrigin{}
				}
				if scenario == "tight-budget" {
					e.budget.limits.StateBytes = initial + want - 1
				}
				if _, err := e.currentCandidate(ctx, parent, score, nil); err == nil {
					t.Fatalf("accepted baseline failure %s", scenario)
				}
				cancel()
				parent.instance.origin = origin
				e.budget.limits.StateBytes = limit
				ownershipAuditCheck(t, e, parent, initial)
			}
			winner, err := e.searchCandidates(context.Background(), node, parent, nil, "ownership/empty-refine", time.Time{})
			if err != nil || !winner.baseline || winner.path != parent.instance.origin.path {
				t.Fatalf("empty includeCurrent: %v", err)
			}
			if err := e.adoptCandidate(context.Background(), node, parent, winner); err != nil {
				t.Fatal(err)
			}
			e.releaseResult(winner)
			ownershipAuditCheck(t, e, parent, initial)
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}

// TestOwnershipAuditAdoptTransfer 职责：核验成功额度转交和采用失败的旧图保留；使用：真实搜索工作区；边界：成功后父仍全额计量，胜者工作区不再重复释放。
func TestOwnershipAuditAdoptTransfer(t *testing.T) {
	for _, scenario := range []string{"success", "parent-cancel", "observer-error", "observer-cancel", "observer-panic", "wrong-task"} {
		t.Run(scenario, func(t *testing.T) {
			e, parent, mutable, _, node := ownershipAuditValidatedParent(t, true)
			node.ast.Candidates.IncludeCurrent = false
			initial, original := ownershipAuditBytes(e), parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0]
			best, err := e.searchCandidates(context.Background(), node, parent, []map[string]any{{}, {}, {}}, "ownership/refine", time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			child, retained, extra := best.work, best.work.retained, best.snapshotBytes
			if ownershipAuditBytes(e) != initial+retained+extra {
				t.Fatal("search retained unmatched ownership")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "parent-cancel" {
				cancel()
			}
			if scenario == "wrong-task" {
				best.work.instance.identity = &taskIdentity{id: "foreign"}
			}
			e.options.Observe = func(CandidateEvent) error {
				switch scenario {
				case "observer-error":
					return errors.New("observer injection")
				case "observer-cancel":
					cancel()
				case "observer-panic":
					panic("observer injection")
				}
				return nil
			}
			err = e.adoptCandidate(ctx, node, parent, best)
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
				if parent.retained != retained || child.retained != 0 || child.counted || !parent.counted {
					t.Fatal("ownership not transferred")
				}
				e.releaseResult(best)
				ownershipAuditCheck(t, e, parent, retained)
				if parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0] != 3 {
					t.Fatal("wrong adopted result")
				}
			} else {
				if err == nil || parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0] != original {
					t.Fatalf("adopt failure published: %v", err)
				}
				e.releaseResult(best)
				ownershipAuditCheck(t, e, parent, initial)
			}
			e.releaseResult(best)
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}

// TestOwnershipAuditParallelMutableIsolation 职责：并发候选读写相同初始嵌套负载并核验快照独立；使用：race压力审验；边界：观察者仅经公开复制API读取，全部worker退出后再释放所有者。
func TestOwnershipAuditParallelMutableIsolation(t *testing.T) {
	for iteration := 0; iteration < 20; iteration++ {
		e, parent, mutable, _, node := ownershipAuditValidatedParent(t, true)
		node.ast.Candidates.IncludeCurrent = false
		initial := ownershipAuditBytes(e)
		var observations atomic.Int32
		e.options.Observe = func(event CandidateEvent) error {
			if event.Kind == "succeeded" {
				v, err := SnapshotRead(event.Snapshot, mutable, "")
				if err != nil {
					return err
				}
				v.data[0], v.rows["r"][0] = 200, 200
				observations.Add(1)
			}
			return nil
		}
		best, err := e.searchCandidates(context.Background(), node, parent, []map[string]any{{}, {}, {}, {}, {}, {}, {}, {}}, "ownership/parallel", time.Time{})
		if err != nil || observations.Load() != 8 {
			t.Fatalf("parallel search: %v, observations=%d", err, observations.Load())
		}
		actual, _ := SnapshotRead(best.snapshot, mutable, "")
		if actual.data[0] != 3 || actual.rows["r"][0] != 3 {
			t.Fatal("parallel snapshot alias")
		}
		e.releaseResult(best)
		ownershipAuditCheck(t, e, parent, initial)
		e.release(parent)
		ownershipAuditCheck(t, e, parent, 0)
	}
}

// TestOwnershipAuditAdoptCancellationDuringTransfer 职责：确定性触发采用进入候选锁后父取消；使用：线性化边界对抗；边界：只锁测试私有停止候选，不修改生产同步设施。
func TestOwnershipAuditAdoptCancellationDuringTransfer(t *testing.T) {
	e, parent, mutable, _, node := ownershipAuditValidatedParent(t, true)
	node.ast.Candidates.IncludeCurrent = false
	best, err := e.searchCandidates(context.Background(), node, parent, []map[string]any{{}}, "ownership/adopt-cancel", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { e.releaseResult(best); e.release(parent) }()
	original := parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	selected := make(chan struct{})
	e.options.Observe = func(CandidateEvent) error { close(selected); return nil }
	best.work.instance.mu.Lock()
	done := make(chan error, 1)
	go func() { done <- e.adoptCandidate(ctx, node, parent, best) }()
	<-selected
	// 等待采用goroutine阻塞在instance.adopt的候选锁；避免用睡眠猜测时序。
	deadline := time.Now().Add(3 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		stack := make([]byte, 1<<16)
		n := runtime.Stack(stack, true)
		if strings.Contains(string(stack[:n]), "(*BlueprintInstance).adopt(") {
			blocked = true
			break
		}
		runtime.Gosched()
	}
	cancel()
	best.work.instance.mu.Unlock()
	err = <-done
	if !blocked {
		t.Fatal("could not synchronize adopt lock boundary")
	}
	actual := parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0]
	if err == nil || actual != original {
		t.Fatalf("canceled adopt published: err=%v before=%d after=%d budget=%d retained=%d", err, original, actual, ownershipAuditBytes(e), parent.retained)
	}
}

// TestOwnershipAuditNestedFailureCleanup 职责：验证三层并行搜索在评分、观察和取消故障下全部归还；使用：真实嵌套flow与竞态检查；边界：累计候选/步骤计数不退还，仅活跃状态额度可归还。
func TestOwnershipAuditNestedFailureCleanup(t *testing.T) {
	for _, scenario := range []string{"success", "score-error", "score-panic", "succeeded-error", "selected-error", "selected-cancel", "all-no-solution", "byte-limit"} {
		t.Run(scenario, func(t *testing.T) {
			e, parent, mutable, frozen, _ := ownershipAuditValidatedParent(t, true)
			initial := ownershipAuditBytes(e)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var active atomic.Int32
			tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error {
				active.Add(1)
				defer active.Add(-1)
				if err := Stage(step, mutable, ownershipAuditValue(64, byte(step.Candidate()+3))); err != nil {
					return err
				}
				if err := Stage(step, frozen, ownershipAuditFrozen{new([1000]byte)}); err != nil {
					return err
				}
				if scenario == "all-no-solution" {
					return &Failure{Code: "audit-no-solution", Message: "injected no solution"}
				}
				runtime.Gosched()
				return nil
			}}
			flow := "  - id: outer\n    candidates:\n      parallel: 2\n      items: [{}, {}]\n      flow:\n        - id: middle\n          candidates:\n            parallel: 2\n            items: [{}, {}]\n            flow:\n              - id: inner\n                candidates:\n                  parallel: 2\n                  items: [{}, {}]\n                  flow:\n                    - id: publish\n                      tool: audit.ownership@1\n"
			plan := ownershipAuditPlan(t, ToolSpec{Receiver: tool, Writes: []Artifact{mutable.Ref(), frozen.Ref()}, Provides: []Artifact{mutable.Ref(), frozen.Ref()}, Role: Validate, FailureCodes: []string{"audit-no-solution"}}, flow, 1<<20)
			e.plan, e.budget.limits = plan, plan.limits
			if scenario == "byte-limit" {
				e.budget.limits.StateBytes = initial + 128
			}
			if scenario == "score-error" {
				e.options.Score = func(*FrozenSnapshot) ([]float64, error) { return nil, errors.New("nested score injection") }
			}
			if scenario == "score-panic" {
				e.options.Score = func(*FrozenSnapshot) ([]float64, error) { panic("nested score injection") }
			}
			e.options.Observe = func(event CandidateEvent) error {
				if event.Kind == "succeeded" && scenario == "succeeded-error" {
					return errors.New("nested succeeded injection")
				}
				if event.Kind == "selected" {
					if scenario == "selected-error" {
						return errors.New("nested selected injection")
					}
					if scenario == "selected-cancel" {
						cancel()
					}
				}
				return nil
			}
			err := e.flow(ctx, plan.flow, parent, nil, "ownership/nested")
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("injected nested failure succeeded")
			}
			if active.Load() != 0 {
				t.Fatal("nested workers not joined")
			}
			ownershipAuditCheck(t, e, parent, initial)
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}

// TestOwnershipAuditBaselineReplacement 职责：比较baseline保留与被更优候选替代时的存活计量；使用：真实includeCurrent搜索；边界：外部持有的观察快照不归引擎计量。
func TestOwnershipAuditBaselineReplacement(t *testing.T) {
	for _, improve := range []bool{false, true} {
		t.Run(fmt.Sprint(improve), func(t *testing.T) {
			e, parent, mutable, _, node := ownershipAuditValidatedParent(t, true)
			initial, originalPath := ownershipAuditBytes(e), parent.instance.origin.path
			if improve {
				e.options.Score = func(s *FrozenSnapshot) ([]float64, error) {
					v, err := SnapshotRead(s, mutable, "")
					if err != nil {
						return nil, err
					}
					return []float64{-float64(v.data[0])}, nil
				}
			}
			best, err := e.searchCandidates(context.Background(), node, parent, []map[string]any{{}, {}, {}}, "ownership/optional-refine", time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if best.baseline == improve {
				t.Fatal("wrong baseline selection")
			}
			if best.baseline && best.path != originalPath {
				t.Fatal("baseline received fake candidate identity")
			}
			if err := e.adoptCandidate(context.Background(), node, parent, best); err != nil {
				t.Fatal(err)
			}
			e.releaseResult(best)
			ownershipAuditCheck(t, e, parent, initial)
			value := parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable)
			want := byte(2)
			if improve {
				want = 5
			}
			if value.data[0] != want {
				t.Fatalf("selected=%d want=%d", value.data[0], want)
			}
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}

// TestOwnershipAuditCloneReservationFailure 职责：注入候选克隆及baseline冻结失败；使用：既有夹具键的私有测试克隆器；边界：失败之后完整恢复注册定义，不修改生产键。
func TestOwnershipAuditCloneReservationFailure(t *testing.T) {
	for _, scenario := range []string{"candidate-clone-error", "candidate-clone-panic", "candidate-clone-growth", "baseline-clone-error", "baseline-clone-panic", "baseline-score-panic"} {
		t.Run(scenario, func(t *testing.T) {
			e, parent, mutable, _, _ := ownershipAuditValidatedParent(t, true)
			initial := ownershipAuditBytes(e)
			old := mutable.def.clone
			defer func() { mutable.def.clone = old; e.release(parent); ownershipAuditCheck(t, e, parent, 0) }()
			if scenario != "baseline-score-panic" {
				mutable.def.clone = func(v any) (any, error) {
					if strings.HasSuffix(scenario, "panic") {
						panic("clone audit")
					}
					if scenario == "candidate-clone-growth" {
						value, _ := ownershipAuditClone(v.(ownershipAuditMutable))
						value.data = make([]byte, 100000)
						return value, nil
					}
					return nil, errors.New("clone audit")
				}
			}
			if scenario == "candidate-clone-growth" {
				e.budget.limits.StateBytes = initial * 2
			}
			if strings.HasPrefix(scenario, "candidate-") {
				if work, err := e.candidateWork(parent, 0); err == nil || work != nil {
					t.Fatal("candidate clone fault accepted")
				}
			} else if scenario == "baseline-score-panic" {
				panicked := false
				func() {
					defer func() { panicked = recover() != nil }()
					_, _ = e.currentCandidate(context.Background(), parent, func(*FrozenSnapshot) ([]float64, error) { panic("baseline score audit") }, nil)
				}()
				if !panicked {
					t.Fatal("injection did not panic")
				}
			} else {
				if _, err := e.currentCandidate(context.Background(), parent, e.options.Score, nil); err == nil {
					t.Fatal("baseline clone fault accepted")
				}
			}
			ownershipAuditCheck(t, e, parent, initial)
		})
	}
}

// TestOwnershipAuditRestageAndConsume 职责：核验同槽再次暂存、消费收缩及失败回滚；使用：真实事务所有权链；边界：旧暂存直到新副本创建前仍需额度，消费必须与新状态一同提交。
func TestOwnershipAuditRestageAndConsume(t *testing.T) {
	for _, scenario := range []string{"restage", "restage-limit", "restage-error", "consume", "consume-error", "consume-cancel"} {
		t.Run(scenario, func(t *testing.T) {
			e, parent, mutable, frozen, _ := ownershipAuditValidatedParent(t, true)
			initial := ownershipAuditBytes(e)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error {
				if strings.HasPrefix(scenario, "consume") {
					if _, err := Read(step, frozen); err != nil {
						return err
					}
				}
				if err := Stage(step, mutable, ownershipAuditValue(1000, 8)); err != nil {
					return err
				}
				if err := Stage(step, mutable, ownershipAuditValue(32, 9)); err != nil {
					return err
				}
				// 两次暂存后旧暂存额度已归还，尚未提交的第二份值仍完整预留。
				if got := ownershipAuditBytes(e); got != initial+2*(ownershipAuditSize(ownershipAuditValue(32, 9))+128) {
					return fmt.Errorf("restage budget=%d", got)
				}
				if strings.HasSuffix(scenario, "error") {
					return errors.New("restage/consume injection")
				}
				if scenario == "consume-cancel" {
					cancel()
				}
				return nil
			}}
			spec := ToolSpec{Receiver: tool, Writes: []Artifact{mutable.Ref()}, Provides: []Artifact{mutable.Ref()}, Role: Validate}
			if strings.HasPrefix(scenario, "consume") {
				spec.Reads, spec.Requires, spec.Consumes = []Artifact{frozen.Ref()}, []Artifact{frozen.Ref()}, []Artifact{frozen.Ref()}
			}
			plan := ownershipAuditPlan(t, spec, "  - id: update\n    tool: audit.ownership@1\n", 1<<20)
			e.plan, e.budget.limits = plan, plan.limits
			if scenario == "restage-limit" {
				e.budget.limits.StateBytes = initial + 2*(ownershipAuditSize(ownershipAuditValue(1000, 8))+128) + 2*(ownershipAuditSize(ownershipAuditValue(32, 9))+128) - 1
			}
			err := e.call(ctx, plan.flow[0], parent, nil, "ownership/update")
			failure := strings.HasSuffix(scenario, "error") || strings.HasSuffix(scenario, "cancel") || scenario == "restage-limit"
			if failure {
				if err == nil {
					t.Fatal("fault succeeded")
				}
				ownershipAuditCheck(t, e, parent, initial)
				if parent.instance.slots[slotID{def: frozen.def}].consumed || parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0] != 2 {
					t.Fatal("failed restage/consume published")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				want := int64(384) + ownershipAuditSize(ownershipAuditValue(32, 9))
				if scenario != "consume" {
					want += 1000
				}
				ownershipAuditCheck(t, e, parent, want)
				consumed := parent.instance.slots[slotID{def: frozen.def}]
				if scenario == "consume" && (!consumed.consumed || consumed.value != nil) {
					t.Fatal("consumed graph still owned by slot")
				}
			}
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}

// TestOwnershipAuditGroupedCandidateCleanup 职责：检查跨组best存活及重复defer释放的额度平衡；使用：条件组实际引擎；边界：后组失败不得采用前组临时胜者，比较故障必须整体退出。
func TestOwnershipAuditGroupedCandidateCleanup(t *testing.T) {
	for _, scenario := range []string{"success", "second-group-error", "comparator-error", "comparator-panic", "second-group-budget"} {
		t.Run(scenario, func(t *testing.T) {
			e, parent, mutable, frozen, _ := ownershipAuditValidatedParent(t, true)
			initial := ownershipAuditBytes(e)
			tool := &ownershipAuditTool{action: func(_ context.Context, step *StepContext) error {
				second := strings.Contains(step.Node(), "group:000001:second")
				if second && scenario == "second-group-error" {
					return errors.New("group audit failure")
				}
				tag := byte(9)
				if second {
					tag = 1
				}
				if err := Stage(step, mutable, ownershipAuditValue(64, tag)); err != nil {
					return err
				}
				return Stage(step, frozen, ownershipAuditFrozen{new([1000]byte)})
			}}
			comparator := ""
			if strings.HasPrefix(scenario, "comparator-") {
				comparator = "      comparator: audit\n"
			}
			flow := "  - id: grouped\n    candidates:\n      parallel: 1\n" + comparator + "      groups:\n        - id: first\n          items: [{}]\n        - id: second\n          items: [{}]\n      flow:\n        - id: publish\n          tool: audit.ownership@1\n"
			plan := ownershipAuditPlan(t, ToolSpec{Receiver: tool, Writes: []Artifact{mutable.Ref(), frozen.Ref()}, Provides: []Artifact{mutable.Ref(), frozen.Ref()}, Role: Validate}, flow, 1<<20)
			e.plan, e.budget.limits = plan, plan.limits
			if comparator != "" {
				e.options.Comparators = map[string]Comparator{"audit": func(a, b *FrozenSnapshot) (int, error) {
					if scenario == "comparator-panic" {
						panic("group comparator audit")
					}
					return 0, errors.New("group comparator audit")
				}}
			}
			if scenario == "second-group-budget" {
				e.options.Observe = func(event CandidateEvent) error {
					if event.Kind == "succeeded" {
						e.budget.mu.Lock()
						e.budget.limits.StateBytes = e.budget.bytes
						e.budget.mu.Unlock()
					}
					return nil
				}
			}
			err := e.flow(context.Background(), plan.flow, parent, nil, "ownership/groups")
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("group fault accepted")
			}
			ownershipAuditCheck(t, e, parent, initial)
			actual := parent.instance.slots[slotID{def: mutable.def}].value.(ownershipAuditMutable).data[0]
			want := byte(2)
			if scenario == "success" {
				want = 1
			}
			if actual != want {
				t.Fatalf("group result=%d want=%d", actual, want)
			}
			e.release(parent)
			ownershipAuditCheck(t, e, parent, 0)
		})
	}
}
