package generator_test

// 文件职责：固定种子六种真实大厂的性能基线；使用：授权目录中的冻结二进制；边界：不改生产预算，不重抽失败案例。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/quant"
)

type largeFactoryCase struct {
	ID             string
	Name           string
	TargetMachines int
	BaseMachines   int
	Quant          quant.Request
	Request        core.Request
}

// largeFactoryDir 职责：限制本任务证据目录；使用：清单和单例；边界：不写其他验收目录。
func largeFactoryDir(t *testing.T) string {
	t.Helper()
	root := filepath.FromSlash("D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/large-factories")
	dir, err := filepath.Abs(os.Getenv("LARGE_FACTORY_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Fatal("证据目录越界")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestLargeFactoryManifest 职责：先冻结筛选规则和六例输入；使用：只执行真实量化标定，不生成蓝图；边界：失败不换产品或档位。
func TestLargeFactoryManifest(t *testing.T) {
	if os.Getenv("LARGE_FACTORY_DIR") == "" {
		t.Skip("需要授权证据目录")
	}
	dir := largeFactoryDir(t)
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"电动机", "处理器", "推进器", "电磁涡轮", "石墨烯", "氘核燃料棒"}
	tiers := []int{256, 384, 640, 960, 1408, 1920}
	rng := rand.New(rand.NewSource(20261116))
	rng.Shuffle(len(tiers), func(i, j int) { tiers[i], tiers[j] = tiers[j], tiers[i] })
	rules := map[string]any{"Seed": 20261116, "Names": names, "Tiers": tiers, "Selection": "固定产品顺序；Dataset要求有制造配方；先Solve 60/min；按种子洗牌机器档位；rate=60*tier/baseMachines，再Solve，不按生成结果换例", "Settings": "UI belt1/inserter1/maxLays3/powertrue; quant StackResearchLab3 MiningSpeedMultiple1", "Surface": "原选区equator，允许expand；第6例ring2-north-center；不增加生产预算", "TimeoutSeconds": 300}
	randomAcceptanceJSON(t, filepath.Join(dir, "selection-rules.json"), rules)
	cases := []largeFactoryCase{}
	for i, name := range names {
		item, ok := dataset.ByName[name]
		if !ok || len(dataset.Choices[name]) == 0 {
			t.Fatalf("未核实制造产品%s", name)
		}
		q := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: 60}}}
		q.Settings.StackResearchLab = 3
		q.Settings.MiningSpeedMultiple = 1
		base, err := quant.Solve(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		q.Targets[0].RatePerMinute = math.Round(60*float64(tiers[i])/float64(base.MachineCount)*1e6) / 1e6
		plan, err := quant.Solve(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		enabled := true
		density := "dense"
		if i == 3 || i == 4 {
			density = "standard"
		}
		zone := "equator"
		if i == 5 {
			zone = "ring2-north-center"
		}
		id := fmt.Sprintf("case-%02d", i+1)
		req := core.Request{Plan: plan, TaskID: "large-factory-" + id, Name: name + "固定种子大厂", Mode: []int{1, 3, 4, 1, 3, 1}[i], Density: density, CollectPerformance: true, EmitPreviewDeltas: true, PreviewProtocolVersion: 4, Setting: core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, NeedPower: &enabled}, SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: zone}}
		cases = append(cases, largeFactoryCase{ID: id, Name: name, TargetMachines: tiers[i], BaseMachines: base.MachineCount, Quant: q, Request: req})
		t.Logf("%s %s item=%d rate=%.6f machines=%d productions=%d %s/%d %s", id, name, item.ID, q.Targets[0].RatePerMinute, plan.MachineCount, len(plan.Productions), density, req.Mode, zone)
	}
	randomAcceptanceJSON(t, filepath.Join(dir, "manifest.json"), cases)
}

// TestLargeFactoryProfileCase 职责：完整生成并保留失败与独立验收；使用：冻结程序每次只跑一例；边界：外部300秒截止，内部YAML预算原样保留。
func TestLargeFactoryProfileCase(t *testing.T) {
	if os.Getenv("LARGE_FACTORY_CASE") == "" {
		t.Skip("只供本任务顺序运行器使用")
	}
	dir := largeFactoryDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []largeFactoryCase
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	index, err := strconv.Atoi(os.Getenv("LARGE_FACTORY_CASE"))
	if err != nil || index < 0 || index >= len(cases) {
		t.Fatal("invalid case")
	}
	c := cases[index]
	dir = filepath.Join(dir, c.ID)
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	evidence := map[string]any{"Case": c, "Request": c.Request, "QuantRequest": c.Quant, "Status": "started", "Started": started}
	defer func() {
		if p := recover(); p != nil {
			evidence["Status"] = "panic"
			evidence["Error"] = fmt.Sprint(p)
			evidence["PanicStack"] = string(debug.Stack())
			t.Error(p)
		}
		evidence["ElapsedSeconds"] = time.Since(started).Seconds()
		randomAcceptanceJSON(t, filepath.Join(dir, "result.json"), evidence)
	}()
	randomAcceptanceJSON(t, filepath.Join(dir, "request.json"), c)
	// 冻结Dataset中再次真实求解，防止把清单里的Plan当成人工构造产线。
	plan, err := quant.Solve(context.Background(), c.Quant)
	if err != nil {
		evidence["Status"] = "quant-error"
		evidence["Error"] = err.Error()
		t.Error(err)
		return
	}
	if plan.PlanID != c.Request.Plan.PlanID {
		t.Fatal("量化指纹漂移")
	}
	c.Request.Plan = plan
	profile, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	if err = pprof.StartCPUProfile(profile); err != nil {
		t.Fatal(err)
	}
	defer func() { pprof.StopCPUProfile(); _ = profile.Close() }()
	progress, err := os.Create(filepath.Join(dir, "progress.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer progress.Close()
	var mu sync.Mutex
	counts := map[string]int{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx = core.WithProgress(ctx, func(e core.Progress) {
		mu.Lock()
		defer mu.Unlock()
		counts[e.Kind]++
		_ = json.NewEncoder(progress).Encode(map[string]any{"seconds": time.Since(started).Seconds(), "kind": e.Kind, "stage": e.Stage, "detail": e.Detail, "message": e.Message, "current": e.Current, "total": e.Total, "candidateId": e.CandidateID, "candidateStatus": e.CandidateStatus, "sequence": e.Sequence, "operations": len(e.Operations)})
	})
	ctx = core.WithPerformance(ctx, func(p core.GenerationPerformance) { evidence["ObservedPerformance"] = p })
	events, err := os.Create(filepath.Join(dir, "workflow.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	ctx = generator.WithWorkflowTracking(ctx, func(e workflow.ExecutionEvent) error {
		mu.Lock()
		defer mu.Unlock()
		return json.NewEncoder(events).Encode(e)
	})
	result, trace, runErr := generator.GenerateDetailed(ctx, c.Request)
	evidence["Result"] = result
	evidence["Trace"] = trace
	evidence["ProgressCounts"] = counts
	if runErr != nil {
		evidence["Status"] = "generation-error"
		evidence["Error"] = runErr.Error()
		evidence["ErrorType"] = fmt.Sprintf("%T", runErr)
		t.Error(runErr)
		return
	}
	checks, summary := randomAcceptanceCheck(c.Request, result)
	if trace == nil || trace.Steps == 0 {
		checks = append(checks, "缺少真实workflow轨迹")
	}
	evidence["ChecksFailed"] = checks
	evidence["Summary"] = summary
	if len(checks) > 0 {
		evidence["Status"] = "acceptance-failed"
		t.Error(checks)
		return
	}
	evidence["Status"] = "passed"
	t.Logf("%s passed machines=%d buildings=%d seconds=%.3f", c.ID, plan.MachineCount, result.BuildingCount, time.Since(started).Seconds())
}
