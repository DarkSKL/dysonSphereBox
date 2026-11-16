package generator

// 文件职责：保存安全去重前后的完整默认案例及执行身份，供本轮独立比较。
// 使用：Go1.24.11定向验收；边界：仅写指定临时证据，不替换旧基线或修改生产输入。

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// TestConnectedSemanticDefault 职责：重放真实UI默认请求并保存完整结果及叶候选规格；使用：去重前后独立进程；边界：不由测试截断候选或放宽任何规则。
func TestConnectedSemanticDefault(t *testing.T) {
	dir := os.Getenv("CONNECTED_SEMANTIC_EVIDENCE")
	if dir == "" {
		t.Skip("requires evidence directory")
	}
	fixture := filepath.Join("..", "..", "..", "..", "..", ".local", "blueprint-core-refactor", "preview", "final-current", "v4.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct{ Request core.Request }
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	request := saved.Request
	request.TaskID = "connected-semantic-default"
	var mu sync.Mutex
	var started, prepared []workflow.ExecutionEvent
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = core.WithProgress(ctx, func(core.Progress) {})
	ctx = WithWorkflowTracking(ctx, func(e workflow.ExecutionEvent) error {
		if e.Status == "started" && (e.Tool == "connected.assemble@1" || e.Tool == "surface.prepare@1") {
			mu.Lock()
			if e.Tool == "connected.assemble@1" {
				started = append(started, e)
			} else {
				prepared = append(prepared, e)
			}
			mu.Unlock()
		}
		return nil
	})
	begin := time.Now()
	result, trace, err := GenerateDetailed(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Buildings) != result.BuildingCount || !core.LayoutShapeAccepted(result.LogisticsSummary.Metrics) {
		t.Fatal("invalid count or shape")
	}
	specs := []workflow.Trace{}
	for _, step := range trace.Trace {
		if step.Tool == "connected.assemble@1" || step.Tool == "surface.prepare@1" || step.Tool == "surface.enumerate@1" {
			specs = append(specs, step)
		}
	}
	paths := map[string]bool{}
	for _, step := range append(append([]workflow.ExecutionEvent(nil), started...), prepared...) {
		if paths[step.Path] {
			t.Fatalf("duplicate dispatch: %s", step.Path)
		}
		paths[step.Path] = true
	}
	identities := map[string]bool{}
	for _, step := range specs {
		if step.Tool != "connected.assemble@1" {
			continue
		}
		at := strings.LastIndex(step.Node, "/candidate:")
		if at < 0 {
			t.Fatal("candidate missing scope")
		}
		for _, d := range step.Diagnostics {
			if !strings.HasPrefix(d, "specification:connected:") {
				continue
			}
			id := step.Node[:at] + d
			if identities[id] {
				t.Fatal("same scope dispatched identical specification twice")
			}
			identities[id] = true
		}
	}
	if baseline := os.Getenv("CONNECTED_SEMANTIC_BASELINE"); baseline != "" {
		raw, err := os.ReadFile(baseline)
		if err != nil {
			t.Fatal(err)
		}
		var previous struct {
			Result   core.Result
			Document blueprint.Document
		}
		if err = json.Unmarshal(raw, &previous); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(previous.Document, doc) || !reflect.DeepEqual(previous.Result.Bindings, result.Bindings) || !reflect.DeepEqual(previous.Result.SurfacePlan, result.SurfacePlan) || !reflect.DeepEqual(previous.Result.RuleAudit, result.RuleAudit) {
			t.Fatal("default document, bindings, surface or rules changed")
		}
		left, right := previous.Result.LogisticsSummary, result.LogisticsSummary
		left.Metrics.CandidateBudget, right.Metrics.CandidateBudget = 0, 0
		left.Metrics.EvaluatedCandidates, right.Metrics.EvaluatedCandidates = 0, 0
		left.Metrics.RejectedCandidates, right.Metrics.RejectedCandidates = 0, 0
		left.SearchCandidates, right.SearchCandidates = nil, nil
		strip := func(checks []core.LogisticsCheck) []core.LogisticsCheck {
			out := []core.LogisticsCheck{}
			for _, c := range checks {
				if c.RuleID != "workflow-candidate-accounting" {
					out = append(out, c)
				}
			}
			return out
		}
		left.Checks, right.Checks = strip(left.Checks), strip(right.Checks)
		if !reflect.DeepEqual(left, right) {
			t.Fatal("default physical logistics or shape changed")
		}
		t.Log("baseline document, bindings, surface, all rules, physical logistics and shape identical; no repeated dispatch path or same-scope raw spec")
	}
	payload := struct {
		Result            core.Result
		Document          blueprint.Document
		Specs             []workflow.Trace
		Started, Prepared []workflow.ExecutionEvent
		Elapsed           string
	}{result, doc, specs, started, prepared, time.Since(begin).String()}
	output, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "default.json"), output, 0644); err != nil {
		t.Fatal(err)
	}
	buildings, _ := json.Marshal(doc.Buildings)
	t.Logf("candidates=%d buildings=%d aspect=%.9f buildingsSHA256=%x elapsed=%s", len(started), result.BuildingCount, result.LogisticsSummary.Metrics.EnvelopeAspectRatio, sha256.Sum256(buildings), time.Since(begin))
	for _, s := range result.CandidateDeduplication.Stages {
		t.Log(fmt.Sprintf("dedup=%+v", s))
	}
}
