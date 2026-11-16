package workflow

// 文件职责：核对共享冻结图的暂存额度与原子发布；使用：大厂验证汇合回归；边界：可变产物的双份克隆额度仍独立验证。

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// immutableBudgetValue 职责：模拟封闭只读图；使用：预算契约测试；边界：payload创建后不再修改。
type immutableBudgetValue struct{ payload *[1000]byte }

// immutableBudgetTool 职责：发布冻结句柄并注入失败；使用：真实引擎call；边界：不绕过Stage与commit。
type immutableBudgetTool struct {
	key    ArtifactKey[immutableBudgetValue]
	fail   bool
	cancel context.CancelFunc
}

// Produce 职责：暂存同一冻结图；使用：反射测试工具；边界：失败与取消必须零发布。
func (tool *immutableBudgetTool) Produce(_ context.Context, step *StepContext, _ struct{}) error {
	if err := Stage(step, tool.key, immutableBudgetValue{new([1000]byte)}); err != nil {
		return err
	}
	if tool.cancel != nil {
		tool.cancel()
	}
	if tool.fail {
		return errors.New("expected failure")
	}
	return nil
}

// TestImmutableStagingBudget 职责：证明共享图无需两份额度，同时超限、失败和取消不泄漏；使用：不可变构造器回归；边界：预算不退还既有工作区以骗过上限。
func TestImmutableStagingBudget(t *testing.T) {
	for _, scenario := range []string{"success", "too-large", "failure", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			key, err := NewImmutableArtifactKey("frozen", func(immutableBudgetValue) int64 { return 1000 }, ArtifactOptions{})
			if err != nil {
				t.Fatal(err)
			}
			tool := &immutableBudgetTool{key: key, fail: scenario == "failure"}
			if scenario == "cancel" {
				tool.cancel = cancel
			}
			registry, err := NewRegistry([]ToolSpec{{ID: "test.frozen@1", Receiver: tool, Method: "Produce", Implementation: "immutable-budget", Writes: []Artifact{key.Ref()}, Provides: []Artifact{key.Ref()}, Role: Transform}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Load([]byte("schemaVersion: 1\nid: budget\nflow:\n  - id: produce\n    tool: test.frozen@1\n"), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			limit := int64(1400)
			if scenario == "too-large" {
				limit = 1200
			}
			plan, err := Compile(doc, registry, CompileOptions{Limits: Limits{StateBytes: limit}})
			if err != nil {
				t.Fatal(err)
			}
			instance, err := NewInstance([]Artifact{key.Ref()})
			if err != nil {
				t.Fatal(err)
			}
			e := &executor{plan: plan, budget: &budget{limits: plan.limits, semaphore: make(chan struct{}, 1)}}
			w := &workspace{instance: instance, counted: true}
			err = e.call(ctx, plan.flow[0], w, nil, "budget/produce")
			if scenario != "success" {
				if err == nil || e.budget.bytes != 0 || w.retained != 0 || instance.revision != 0 {
					t.Fatalf("failed transaction published or leaked: %v budget=%d retained=%d", err, e.budget.bytes, w.retained)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if e.budget.bytes < 1000 || e.budget.bytes != w.retained {
				t.Fatalf("graph not fully accounted: %d/%d", e.budget.bytes, w.retained)
			}
			e.release(w)
			if e.budget.bytes != 0 {
				t.Fatalf("release leaked %d", e.budget.bytes)
			}
		})
	}
}

// TestImmutableNestedCandidateBudget 职责：验证多层完整候选采用时转交额度而不重复计量冻结图；使用：真实嵌套引擎；边界：同样大的可变产物仍必须为比较快照预留独立副本。
func TestImmutableNestedCandidateBudget(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			size := func(immutableBudgetValue) int64 { return 10000 }
			key, err := NewImmutableArtifactKey("frozen", size, ArtifactOptions{Diagnostic: true})
			if !shared {
				key, err = NewArtifactKey("copied", func(v immutableBudgetValue) (immutableBudgetValue, error) {
					copy := *v.payload
					return immutableBudgetValue{&copy}, nil
				}, size, ArtifactOptions{Diagnostic: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			tool := &immutableBudgetTool{key: key}
			registry, err := NewRegistry([]ToolSpec{{ID: "test.frozen@1", Receiver: tool, Method: "Produce", Implementation: "nested-budget", Writes: []Artifact{key.Ref()}, Provides: []Artifact{key.Ref()}, Role: Validate}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Load([]byte("schemaVersion: 1\nid: nested\nflow:\n  - id: outer\n    candidates:\n      selection: first\n      parallel: 1\n      items: [{}]\n      flow:\n        - id: inner\n          candidates:\n            selection: first\n            parallel: 1\n            items: [{}]\n            flow:\n              - id: produce\n                tool: test.frozen@1\n"), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Compile(doc, registry, CompileOptions{Limits: Limits{StateBytes: 15000}})
			if err != nil {
				t.Fatal(err)
			}
			instance, err := NewInstance([]Artifact{key.Ref()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = Run(context.Background(), plan, instance, RunOptions{Score: func(*FrozenSnapshot) ([]float64, error) { return []float64{0}, nil }})
			if !shared {
				if err == nil {
					t.Fatal("mutable clone bypassed budget")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := instance.Freeze()
			if err != nil {
				t.Fatal(err)
			}
			v, err := SnapshotRead(frozen, key, "")
			if err != nil || v.payload == nil {
				t.Fatalf("nested winner missing: %v", err)
			}
		})
	}
}
