package generator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/quant"
)

// 本轮临时验收：真实数据集和量化结果经过完整生产工作流；最终统一清理测试，保留证据文件。
func TestCoreRefactorMatrix60(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	item, ok := dataset.ByName["电磁矩阵"]
	if !ok {
		t.Fatal("dataset lacks 电磁矩阵")
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	request := core.Request{Plan: plan, Mode: 1, Density: "dense", CollectPerformance: true,
		Setting:       core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, MaxMachineInALine: 15, BlockWidthLimit: 50, MergeType: 2},
		SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: "equator"}}
	started := time.Now()
	ctx := context.Background()
	if os.Getenv("BLUEPRINT_TRACE_FAILURES") != "" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		seen := map[string]bool{}
		ctx = WithWorkflowTracking(ctx, func(event workflow.ExecutionEvent) error {
			if event.Type == "tool" && event.Status == "failed" && !seen[event.Failure] && len(seen) < 25 {
				seen[event.Failure] = true
				t.Log(event.Tool, event.Failure)
			}
			return nil
		})
	}
	result, trace, err := GenerateDetailed(ctx, request)
	if dir := os.Getenv("BLUEPRINT_EVIDENCE_DIR"); dir != "" {
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		errorText := ""
		if err != nil {
			errorText = err.Error()
		}
		raw, e := json.MarshalIndent(struct {
			Request core.Request
			Result  core.Result
			Trace   any
			Error   string
			Elapsed string
		}{request, result, trace, errorText, time.Since(started).String()}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "matrix60.json"), raw, 0644); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("item=%d machines=%d productions=%d buildings=%d elapsed=%s", item.ID, plan.MachineCount, len(plan.Productions), result.BuildingCount, time.Since(started))
	if err != nil {
		t.Fatal(err)
	}
	if result.Blueprint == "" || result.BuildingCount == 0 || result.SourcePlanID != plan.PlanID {
		t.Fatal("missing verified output")
	}
	if result.LogisticsSummary == nil || !core.LayoutShapeAccepted(result.LogisticsSummary.Metrics) { t.Fatal("matrix result is not near-square") }
}

// TestCoreRefactorReplay 重放已保存的真实失败请求，不改产量、喷涂或选区。
func TestCoreRefactorReplay(t *testing.T) {
	path := os.Getenv("BLUEPRINT_REPLAY_PATH")
	if path == "" {
		t.Skip("requires recorded request")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct{ Request core.Request }
	if err = json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if os.Getenv("BLUEPRINT_TRACE_FAILURES") != "" {
		seen:=map[string]bool{}
		ctx=WithWorkflowTracking(ctx,func(event workflow.ExecutionEvent)error{
			message:=event.Failure
			if at:=strings.Index(message,"GEN_");at>=0 {message=message[at:]}
			if event.Type=="tool"&&event.Status=="failed"&&!seen[message]&&len(seen)<30 {seen[message]=true;t.Logf("%s candidate=%d %s",event.Tool,event.Candidate,message)}
			return nil
		})
	}
	result, trace, err := GenerateDetailed(ctx, evidence.Request)
	if output := os.Getenv("BLUEPRINT_EVIDENCE_DIR"); output != "" {
		if e := os.MkdirAll(output, 0755); e != nil {
			t.Fatal(e)
		}
		errorText := ""
		if err != nil {
			errorText = err.Error()
		}
		payload, e := json.MarshalIndent(struct {
			Request core.Request
			Result  core.Result
			Trace   any
			Error   string
			Elapsed string
		}{evidence.Request, result, trace, errorText, time.Since(started).String()}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(output, filepath.Base(path)), payload, 0644); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("buildings=%d elapsed=%s", result.BuildingCount, time.Since(started))
	if err != nil {
		t.Fatal(err)
	}
	if result.BuildingCount == 0 || result.Blueprint == "" {
		t.Fatal("missing blueprint")
	}
	if result.LogisticsSummary == nil || !core.LayoutShapeAccepted(result.LogisticsSummary.Metrics) { t.Fatalf("shape not accepted: %+v", result.LogisticsSummary.Metrics) }
}
