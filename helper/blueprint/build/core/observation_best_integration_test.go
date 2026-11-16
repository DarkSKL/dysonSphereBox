package build_test

// 文件职责：以真实默认量化请求验收best及最终正式快照；使用：定向验收与race；边界：不修改生产算法或伪造候选事件。

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
	"DysonSphereBox/helper/blueprint/build/preview"
	"DysonSphereBox/helper/quant"
)

// TestObservationBestMatrix60 职责：验证真实默认dense生成的协议完整性和严格Decode；使用：固定1.24.11工具链；边界：取消例在首次best后取消，不降低工作流规则。
func TestObservationBestMatrix60(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	item := dataset.ByName["电磁矩阵"]
	qr := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: 60}}}
	qr.Settings.StackResearchLab = 3
	qr.Settings.MiningSpeedMultiple = 1
	plan, err := quant.Solve(context.Background(), qr)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"v4", "v3", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			version := 4
			if scenario == "v3" {
				version = 3
			}
			enabled := true
			req := core.Request{Plan: plan, TaskID: "best-" + scenario, Mode: 1, Density: "dense", EmitPreviewDeltas: true, PreviewProtocolVersion: version, CandidateConcurrency: 3,
				Setting:       core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, NeedPower: &enabled},
				SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: "equator"}}
			var mu sync.Mutex
			var events []core.Progress
			ctx = core.WithProgress(ctx, func(e core.Progress) {
				mu.Lock()
				events = append(events, e)
				mu.Unlock()
				if scenario == "cancel" && e.Kind == "candidate-best" {
					cancel()
				}
			})
			began := time.Now()
			result, err := generator.Generate(ctx, req)
			if scenario == "cancel" {
				if err == nil || result.Blueprint != "" {
					t.Fatalf("canceled generation returned success: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			// 回调已由任务退出收口；用独立记录重放前端可见状态，不依赖候选实体的存活。
			sequence := uint64(0)
			candidateSequences := map[string]uint64{}
			revisions := map[string]uint64{}
			chunks := map[string][]core.Progress{}
			statuses := map[string]string{}
			best, bestCount, cleared, completes := "", 0, 0, 0
			var final []preview.Object
			for _, e := range events {
				if e.Sequence != sequence+1 {
					t.Fatalf("global gap %d -> %d", sequence, e.Sequence)
				}
				sequence = e.Sequence
				if e.ProtocolVersion == 4 && e.CandidateID != "" {
					if e.CandidateSequence != candidateSequences[e.CandidateID]+1 {
						t.Fatalf("candidate gap: %+v", e)
					}
					candidateSequences[e.CandidateID] = e.CandidateSequence
				}
				key := fmt.Sprintf("%s/%d", e.CandidateID, e.SnapshotRevision)
				switch e.Kind {
				case "preview-snapshot":
					if e.SnapshotRevision != revisions[e.CandidateID] {
						if e.SnapshotRevision != revisions[e.CandidateID]+1 {
							t.Fatalf("revision gap: %+v", e)
						}
						revisions[e.CandidateID] = e.SnapshotRevision
					}
					if e.ChunkIndex != len(chunks[key]) || e.CoordinateSystem != preview.AreaCoordinates {
						t.Fatal("invalid snapshot chunk")
					}
					chunks[key] = append(chunks[key], e)
				case "candidate-best", "preview-complete":
					if version == 3 {
						if e.Kind == "candidate-best" {
							t.Fatal("v3 best")
						}
						continue
					}
					parts := chunks[key]
					if len(parts) != e.ChunkCount || e.SnapshotRevision == 0 || e.CoordinateSystem != preview.AreaCoordinates {
						t.Fatalf("incomplete marker: %+v", e)
					}
					objects := []preview.Object{}
					for _, part := range parts {
						if part.ChunkCount != e.ChunkCount || !reflect.DeepEqual(part.PreviewAreas, e.PreviewAreas) {
							t.Fatal("inconsistent frame")
						}
						for _, op := range part.Operations {
							if op.Type != preview.Add || op.Building == nil || op.Building.IsTemporary {
								t.Fatal("non-final object in snapshot")
							}
							objects = append(objects, preview.Object{ID: op.ObjectID, Building: *op.Building})
						}
					}
					if len(objects) != e.BuildingCount {
						t.Fatal("snapshot count mismatch")
					}
					delete(chunks, key)
					if e.Kind == "candidate-best" {
						if statuses[e.CandidateID] == "pruned" || statuses[e.CandidateID] == "rejected" {
							t.Fatal("revived withdrawn best")
						}
						best = e.CandidateID
						bestCount++
					} else {
						if completes != 0 || e.WinnerCandidateID != e.CandidateID {
							t.Fatal("invalid winner")
						}
						final = objects
						completes++
					}
				case "candidate-finish":
					if (e.CandidateStatus == "pruned" || e.CandidateStatus == "rejected") && best == e.CandidateID {
						t.Fatal("best withdrawn before correction")
					}
					statuses[e.CandidateID] = e.CandidateStatus
				case "candidate-best-cleared":
					if e.CandidateID != "" || e.CandidateSequence != 0 || best == "" {
						t.Fatal("invalid task clear")
					}
					best = ""
					cleared++
				}
			}
			if version == 4 && bestCount == 0 {
				t.Fatal("real generation produced no best")
			}
			if scenario == "cancel" {
				if best != "" || cleared == 0 || completes != 0 {
					t.Fatal("canceled best retained")
				}
			} else {
				document, decodeErr := blueprint.Decode(result.Blueprint, true)
				if decodeErr != nil || len(document.Buildings) != result.BuildingCount {
					t.Fatalf("Decode failed: %v", decodeErr)
				}
				if !core.LayoutShapeAccepted(result.LogisticsSummary.Metrics) {
					t.Fatal("unaccepted final shape")
				}
				if version == 4 {
					if completes != 1 || len(final) != len(document.Buildings) {
						t.Fatal("missing final snapshot")
					}
					for i, object := range final {
						b := document.Buildings[i]
						if object.Building.Index != b.Index || object.Building.ItemID != b.ItemID || object.Building.AreaIndex != b.AreaIndex || object.Building.LocalOffset != b.LocalOffset || object.Building.LocalOffset2 != b.LocalOffset2 {
							t.Fatalf("final snapshot differs at %d", i)
						}
					}
					if revisions[result.WinnerCandidateID] < 2 {
						t.Fatal("winner final reused best revision")
					}
				} else if bestCount != 0 || cleared != 0 {
					t.Fatal("v3 behavior changed")
				}
			}
			t.Logf("scenario=%s machines=%d buildings=%d events=%d best=%d clear=%d elapsed=%s", scenario, plan.MachineCount, result.BuildingCount, len(events), bestCount, cleared, time.Since(began))
		})
	}
}
