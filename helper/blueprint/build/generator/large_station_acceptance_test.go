package generator_test

// 文件职责：复现默认超密度生产行星内物流运输站60/min的大蓝图；使用：本轮性能与生成验收；边界：真实量化、完整工作流及严格解码，证据只写本机指定目录。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/quant"
)

func TestLargeStation60(t *testing.T) {
	dir := os.Getenv("LARGE_STATION_EVIDENCE")
	if dir == "" {
		t.Skip("requires evidence directory")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	item, ok := dataset.ByName["行星内物流运输站"]
	if !ok {
		t.Fatal("dataset lacks requested target")
	}
	q := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: 60}}}
	q.Settings.StackResearchLab = 3
	q.Settings.MiningSpeedMultiple = 1
	plan, err := quant.Solve(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	req := core.Request{Plan: plan, TaskID: "large-station-60", Mode: 1, Density: "dense", CollectPerformance: true, EmitPreviewDeltas: true, PreviewProtocolVersion: 4,
		Setting:       core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, NeedPower: &enabled},
		SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: "equator"}}
	if os.Getenv("LARGE_STATION_DIAGNOSTIC_MK3") == "1" {
		req.Setting.BeltLv = 3
	}
	raw, _ := json.MarshalIndent(map[string]any{"quantRequest": q, "request": req}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "request.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("item=%d machines=%d productions=%d", item.ID, plan.MachineCount, len(plan.Productions))
	profile, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	if err := pprof.StartCPUProfile(profile); err != nil {
		t.Fatal(err)
	}
	defer func() { pprof.StopCPUProfile(); _ = profile.Close() }()
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	var mu sync.Mutex
	counts := map[string]int{}
	timeline := []map[string]any{}
	lastLog := time.Time{}
	ctx = core.WithProgress(ctx, func(e core.Progress) {
		mu.Lock()
		defer mu.Unlock()
		counts[e.Kind]++
		if e.Kind == "candidate-start" || e.Kind == "candidate-finish" || e.Kind == "candidate-best" || time.Since(lastLog) > 5*time.Second {
			timeline = append(timeline, map[string]any{"seconds": time.Since(started).Seconds(), "kind": e.Kind, "stage": e.Stage, "message": e.Message, "current": e.Current, "total": e.Total, "candidate": e.CandidateID, "status": e.CandidateStatus})
			t.Logf("%.2fs %s %s %d/%d %s", time.Since(started).Seconds(), e.Kind, e.Stage, e.Current, e.Total, e.Message)
			lastLog = time.Now()
		}
	})
	ctx = generator.WithWorkflowTracking(ctx, func(e workflow.ExecutionEvent) error {
		if e.Status == "failed" && e.Type == "tool" {
			message := e.Failure
			if len(message) > 1600 {
				message = message[:1600]
			}
			t.Logf("FAIL %.2fs %s %s", time.Since(started).Seconds(), e.Tool, message)
		}
		return nil
	})
	var peakHeap, peakSys atomic.Uint64
	stopMemory := make(chan struct{})
	memoryDone := make(chan struct{})
	go func() {
		defer close(memoryDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopMemory:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peakHeap.Load() {
					peakHeap.Store(m.HeapAlloc)
				}
				if m.Sys > peakSys.Load() {
					peakSys.Store(m.Sys)
				}
			}
		}
	}()
	result, trace, runErr := generator.GenerateDetailed(ctx, req)
	close(stopMemory)
	<-memoryDone
	t.Logf("peak Go heap=%d sys=%d", peakHeap.Load(), peakSys.Load())
	var failed []string
	var summary map[string]any
	if runErr == nil {
		failed, summary = randomAcceptanceCheck(req, result)
	}
	errorText := ""
	if runErr != nil {
		errorText = runErr.Error()
	}
	raw, err = json.MarshalIndent(map[string]any{"request": req, "result": result, "trace": trace, "error": errorText, "elapsed": time.Since(started).String(), "timeline": timeline, "counts": counts, "checksFailed": failed, "summary": summary, "peakGoHeap": peakHeap.Load(), "peakGoSys": peakSys.Load()}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "result.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatal(runErr)
	}
	if len(failed) > 0 {
		t.Fatal(failed)
	}
	t.Logf("SUCCESS buildings=%d elapsed=%s", result.BuildingCount, time.Since(started))
}
