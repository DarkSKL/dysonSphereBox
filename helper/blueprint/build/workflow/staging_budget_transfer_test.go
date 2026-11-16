package workflow

// 文件职责：验证暂存额度提交转移、失败归还及可变克隆边界；使用：大厂预算回归；边界：使用真实引擎call和预算，不用RSS代替保留计量。
import (
	"context"
	"errors"
	"testing"
)

type budgetTransferTool struct {
	key  ArtifactKey[[]byte]
	fail bool
}

func (t *budgetTransferTool) Produce(_ context.Context, s *StepContext, _ struct{}) error {
	v := make([]byte, 1000)
	v[0] = 7
	if err := Stage(s, t.key, v); err != nil {
		return err
	}
	v[0] = 99
	if t.fail {
		return errors.New("expected failure")
	}
	return nil
}

func TestStagingBudgetTransfersWithoutDoubleCharge(t *testing.T) {
	for _, scenario := range []string{"success", "tool-failure", "commit-cancel", "commit-clone-failure"} {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		clones := 0
		fail := scenario != "success"
		key, err := NewArtifactKey("payload", func(v []byte) ([]byte, error) {
			clones++
			if clones == 2 {
				if scenario == "commit-cancel" {
					cancel()
				}
				if scenario == "commit-clone-failure" {
					return nil, errors.New("commit clone failure")
				}
			}
			return append([]byte(nil), v...), nil
		}, func(v []byte) int64 { return int64(len(v)) }, ArtifactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		tool := &budgetTransferTool{key: key, fail: scenario == "tool-failure"}
		registry, err := NewRegistry([]ToolSpec{{ID: "test.produce@1", Receiver: tool, Method: "Produce", Implementation: "budget-regression", Writes: []Artifact{key.Ref()}, Provides: []Artifact{key.Ref()}, Role: Transform}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := Load([]byte("schemaVersion: 1\nid: budget\nflow:\n  - id: produce\n    tool: test.produce@1\n"), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := Compile(doc, registry, CompileOptions{Limits: Limits{StateBytes: 2400}})
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
		if fail {
			if err == nil || e.budget.bytes != 0 || w.retained != 0 || instance.revision != 0 {
				t.Fatalf("failure retained state: %v bytes%d", err, e.budget.bytes)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		actual := instance.slots[slotID{key.def, ""}].value.([]byte)
		if len(actual) != 1000 || actual[0] != 7 {
			t.Fatal("clone boundary lost")
		}
		if e.budget.bytes != w.retained {
			t.Fatalf("unbalanced transfer budget%d retained%d", e.budget.bytes, w.retained)
		}
		e.release(w)
		if e.budget.bytes != 0 {
			t.Fatalf("release leaked %d", e.budget.bytes)
		}
	}
}
