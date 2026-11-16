package generator

// 文件职责：独立采集真实量化生成、预览和后台工作流证据；使用：本轮临时验收；边界：不修改生产算法，不合成候选事件。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/factory/workflowjournal"
	"DysonSphereBox/helper/quant"
)

// TestPreviewCoreRefactorEvidence 职责：保存真实v3/v4事件及严格解码；使用：固定工具链go test；边界：错误同样保留证据，journal使用公开默认预算。
func TestPreviewCoreRefactorEvidence(t *testing.T) {
	dir := os.Getenv("PREVIEW_EVIDENCE_DIR")
	if dir == "" {
		t.Fatal("PREVIEW_EVIDENCE_DIR required")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	item, ok := dataset.ByName["电磁矩阵"]
	if !ok {
		t.Fatal("missing target")
	}
	quantRequest := quant.Request{Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: 60}}}
	uiDefaults := os.Getenv("PREVIEW_UI_DEFAULTS") == "1"
	if uiDefaults {
		// UI默认研究站堆叠为3，量化库零值默认15；真实故障复现必须使用工作区请求。
		quantRequest.Dataset = "Vanilla"
		quantRequest.Settings.StackResearchLab = 3
		quantRequest.Settings.MiningSpeedMultiple = 1
	}
	plan, err := quant.Solve(context.Background(), quantRequest)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			request := core.Request{Plan: plan, TaskID: fmt.Sprintf("core-refactor-preview-v%d-%d", version, time.Now().UnixNano()), Mode: 1, Density: "dense", EmitPreviewDeltas: true, PreviewProtocolVersion: version, CollectPerformance: true,
				Setting:       core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, MaxMachineInALine: 15, BlockWidthLimit: 50, MergeType: 2},
				SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: "equator"}}
			if uiDefaults {
				enabled := true
				request.Setting = core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, NeedPower: &enabled}
				request.CollectPerformance = false
			}
			store, err := workflowjournal.New(workflowjournal.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			writer, err := store.Reserve(request.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			var events []json.RawMessage
			var eventMicros []int64
			var captureMarshalNanos int64
			var began time.Time
			var eventMu sync.Mutex
			var startCount, finishCount int
			var trackingErrors []string
			var livePages []workflowjournal.Page
			var liveCursor uint64
			var observed uint64
			readLive := func() {
				page := store.Read(request.TaskID, liveCursor, 128)
				livePages = append(livePages, page)
				if page.Code != "ok" || page.Gap {
					trackingErrors = append(trackingErrors, "live journal gap: "+page.Code)
				}
				liveCursor = page.Next
			}
			ctx := core.WithProgress(context.Background(), func(event core.Progress) {
				captured := time.Now()
				raw, e := json.Marshal(event)
				eventMu.Lock()
				defer eventMu.Unlock()
				if e != nil {
					trackingErrors = append(trackingErrors, e.Error())
					return
				}
				events = append(events, raw)
				eventMicros = append(eventMicros, captured.Sub(began).Microseconds())
				captureMarshalNanos += time.Since(captured).Nanoseconds()
			})
			ctx = WithWorkflowStart(ctx, func(graph workflow.PlanDescription) error { startCount++; return writer.BindPlan(graph) })
			ctx = WithWorkflowTracking(ctx, func(event workflow.ExecutionEvent) error {
				if e := writer.Append(event); e != nil {
					return e
				}
				observed++
				if observed%128 == 0 {
					readLive()
				}
				return nil
			})
			ctx = WithWorkflowFinish(ctx, func(trace *workflow.RunResult, runErr error) {
				finishCount++
				state := "succeeded"
				message := ""
				var lost uint64
				if trace != nil {
					lost = trace.ExecutionEventsLost
				}
				if runErr != nil {
					state = "failed"
					message = runErr.Error()
				}
				if e := writer.FinishWithLost(state, "", message, lost); e != nil {
					trackingErrors = append(trackingErrors, e.Error())
				}
			})
			began = time.Now()
			result, trace, runErr := GenerateDetailed(ctx, request)
			returnedMicros := time.Since(began).Microseconds()
			readLive()
			document, decodeErr := blueprint.Decode(result.Blueprint, true)
			snapshot := store.Snapshot(request.TaskID)
			pages := []workflowjournal.Page{}
			var cursor uint64
			for {
				page := store.Read(request.TaskID, cursor, 128)
				pages = append(pages, page)
				if page.Complete || page.Next <= cursor {
					break
				}
				cursor = page.Next
			}
			errorText := ""
			if runErr != nil {
				errorText = runErr.Error()
			}
			decodeText := ""
			if decodeErr != nil {
				decodeText = decodeErr.Error()
			}
			evidence := map[string]any{"request": request, "result": result, "decoded": document, "events": events, "journal": snapshot, "journalPages": pages, "liveJournalPages": livePages, "observedWorkflowEvents": observed, "startCount": startCount, "finishCount": finishCount, "trackingErrors": trackingErrors, "error": errorText, "decodeError": decodeText, "elapsed": time.Since(began).String(), "trace": trace}
			evidence["quantRequest"] = quantRequest
			evidence["eventMicros"] = eventMicros
			evidence["returnedMicros"] = returnedMicros
			evidence["captureMarshalNanos"] = captureMarshalNanos
			evidence["uiDefaults"] = uiDefaults
			raw, e := json.MarshalIndent(evidence, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, fmt.Sprintf("v%d.json", version)), raw, 0644); e != nil {
				t.Fatal(e)
			}
			t.Logf("v%d buildings=%d progress=%d journal=%d dropped=%d lost=%d engineLost=%d start=%d finish=%d elapsed=%s", version, result.BuildingCount, len(events), snapshot.EventCount, snapshot.DroppedEvents, snapshot.LostEvents, snapshot.EngineEventsLost, startCount, finishCount, time.Since(began))
			if runErr != nil {
				t.Fatal(runErr)
			}
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if len(document.Buildings) != result.BuildingCount {
				t.Error("Decode count mismatch")
			}
			if startCount != 1 || finishCount != 1 || snapshot.State != "succeeded" || !snapshot.TrackingAvailable || liveCursor != observed || snapshot.LostEvents != 0 || snapshot.EngineEventsLost != 0 || len(trackingErrors) != 0 {
				t.Error("workflow lifecycle or journal incomplete")
			}
		})
	}
}
