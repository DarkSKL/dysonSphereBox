package industryworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// TestFullPlanningPerformance 职责：测量实档加载和完整 Go 蓝图链路；使用：独立验收进程；边界：无明确测试路径时跳过。
func TestFullPlanningPerformance(t *testing.T) {
	path, name, profile := os.Getenv("DSP_FULL_PERF_SAVE"), os.Getenv("DSP_FULL_PERF_CASE"), os.Getenv("DSP_FULL_PERF_PROFILE")
	if path == "" {
		t.Skip("未指定真实快照性能样本")
	}
	request := fullPerformanceRequest(t, name, profile)
	emit := func(event string, data any) {
		value, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("DSP-PERF\t%s\t%s\n", event, value)
	}

	emit("load-start", map[string]any{"case": name, "profile": profile})
	mark := time.Now()
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	view, world := Describe("performance", snapshot)
	emit("load-end", map[string]any{"ms": time.Since(mark).Milliseconds(), "planets": len(view.Sites), "stars": view.Stars, "tick": view.Tick})
	request.SaveID, request.SnapshotID = view.SaveID, view.SnapshotID
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}

	emit("plan-start", map[string]any{"targets": request.Planning.Quant.Targets, "center": request.Planning.CenterPlanetID, "destinations": request.Planning.DeliveryPlanetIDs})
	mark = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var eventMu sync.Mutex
	active := map[string]bool{}
	peak, peakScopes, started := 0, 0, 0
	ctx = build.WithWorkflowTracking(ctx, func(event workflow.ExecutionEvent) error {
		if event.Type != "candidate" {
			return nil
		}
		eventMu.Lock()
		defer eventMu.Unlock()
		// 一个布局工作流运行结束后才开始下一星；同一运行的嵌套路径共享父候选。
		key := event.Path
		switch event.Status {
		case "started":
			active[key] = true
			started++
			peakScopes = max(peakScopes, len(active))
			leaves := len(active)
			for path := range active {
				for child := range active {
					if strings.HasPrefix(child, path+"/") {
						leaves--
						break
					}
				}
			}
			peak = max(peak, leaves)
		case "completed", "failed", "skipped":
			delete(active, key)
		}
		return nil
	})
	built, err := Generate(ctx, request, view, world, snapshot, nil)
	elapsed := time.Since(mark).Milliseconds()
	if err != nil {
		emit("plan-end", map[string]any{"ms": elapsed, "error": err.Error()})
		t.Fatal(err)
	}
	entities := 0
	for _, planet := range built.Report.Planets {
		entities += planet.Buildings
	}
	emit("plan-end", map[string]any{
		"ms": elapsed, "status": built.Report.Status, "exportable": built.Report.Exportable,
		"candidates": built.Report.Candidates, "layoutCandidates": built.Report.LayoutCandidates,
		"layoutWorkflowCandidates": started, "peakParallelLayoutBranches": peak, "peakOpenLayoutScopes": peakScopes,
		"resourcesInspected": built.Report.ResourcesInspected, "repairs": built.Report.Repairs,
		"blueprints": len(built.Codes), "entities": entities, "routes": len(built.Report.Routes),
		"warnings": built.Report.Warnings,
	})
}

// fullPerformanceRequest 职责：固定各规模的目标及交付星；使用：重复测量；边界：各规模采用同一外供矿和原版设备设置。
func fullPerformanceRequest(t *testing.T, name, profile string) Request {
	t.Helper()
	mining := false
	r := Request{Planning: industryplan.Request{
		CenterPlanetID: 301, Profile: profile, Preference: "logistics", EnableMining: &mining,
		DeliveryPlanetIDs: map[int]int64{},
	}}
	tg := func(id int, rate float64, planet int64) {
		r.Planning.Quant.Targets = append(r.Planning.Quant.Targets, quant.Target{ItemID: id, RatePerMinute: rate})
		if planet != 0 {
			r.Planning.DeliveryPlanetIDs[id] = planet
		}
	}
	switch name {
	case "small":
		tg(1101, 30, 301)
	case "medium":
		tg(1114, 30, 301)
		tg(1120, 15, 302)
		tg(1101, 30, 401)
	case "large":
		tg(1114, 120, 301)
		tg(1120, 60, 401)
		tg(1101, 120, 301)
		tg(1104, 120, 302)
		tg(1102, 60, 301)
		tg(1105, 60, 302)
		tg(1301, 48, 401)
		tg(1302, 48, 401)
	default:
		t.Fatalf("未知的性能案例 %q", name)
	}
	if profile != "low" && profile != "standard" {
		t.Fatalf("未知的性能档位 %q", profile)
	}
	return r
}
