package generator_test

// 文件职责：固定种子真实量化与蓝图集成验收，逐例保留完整请求、结果及失败轨迹。
// 使用：独立验收运行器启动24个隔离子进程；边界：只写指定证据目录，不改生产数据，不把离线预检当作游戏实测。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
	"DysonSphereBox/helper/quant"
)

// randomAcceptanceSeed 职责：锁定抽样；使用：首次生成案例清单；边界：重跑不重抽失败输入。
const randomAcceptanceSeed int64 = 20261116

// randomAcceptanceCase 职责：保存可重放案例维度；使用：父子验收进程交换；边界：完整生成Request在真实Solve之后另存。
type randomAcceptanceCase struct {
	// ID 职责：标识证据文件；使用：恢复和精确报告；边界：固定两位编号。
	ID string
	// Names 职责：保存经Dataset核实的目标名；使用：报告；边界：求解仍按真实ID。
	Names []string
	// Quant 职责：保存完整量化输入；使用：子进程真实求解；边界：不使用伪造产线。
	Quant quant.Request
	// Request 职责：保存生成配置；使用：Solve后填充Plan；边界：清单中的Plan为空。
	Request core.Request
}

// randomAcceptanceDir 职责：限制证据落点；使用：所有临时输出；边界：拒绝授权目录之外的路径。
func randomAcceptanceDir(t *testing.T) string {
	t.Helper()
	root := "D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/random"
	dir := os.Getenv("BLUEPRINT_RANDOM_DIR")
	if dir == "" {
		t.Fatal("请用授权目录中的run.ps1启动，必须指定BLUEPRINT_RANDOM_DIR")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(filepath.FromSlash(root), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Fatalf("证据目录越界: %q", dir)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		t.Fatal(err)
	}
	return abs
}

// randomAcceptanceJSON 职责：原子保存完整JSON；使用：每阶段与每例收尾；边界：写入失败直接终止验收，不隐藏缺失证据。
func randomAcceptanceJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".tmp", raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
}

// randomAcceptanceCases 职责：分层随机抽样24例；使用：覆盖两密度四模式及三档速率；边界：保留已知矩阵回归点，未失败前不筛选案例。
func randomAcceptanceCases(t *testing.T) []randomAcceptanceCase {
	t.Helper()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"铁块", "电磁矩阵", "电路板", "齿轮", "精炼油", "石墨烯", "电动机", "推进器"}
	rng := rand.New(rand.NewSource(randomAcceptanceSeed))
	order := rng.Perm(len(names))
	for i, n := range order {
		if names[n] == "电磁矩阵" {
			order[0], order[i] = order[i], order[0]
			break
		}
	}
	zones := []string{"equator", "equator-south", "equator-north"}
	for tier := 2; tier <= 5; tier++ {
		for _, hemisphere := range []string{"north", "south"} {
			zones = append(zones, fmt.Sprintf("ring%d-%s-center", tier, hemisphere))
		}
	}
	for tier := 2; tier <= 4; tier++ {
		for _, hemisphere := range []string{"north", "south"} {
			for _, edge := range []string{"north", "south"} {
				zones = append(zones, fmt.Sprintf("ring%d-%s-%s", tier, hemisphere, edge))
			}
		}
	}
	zones = append(zones, "ring5-south-south")
	rng.Shuffle(len(zones), func(i, j int) { zones[i], zones[j] = zones[j], zones[i] })
	for i, zone := range zones {
		if zone == "equator" {
			zones[0], zones[i] = zones[i], zones[0]
			break
		}
	}
	out := make([]randomAcceptanceCase, 24)
	for i := range out {
		id := fmt.Sprintf("case-%02d", i+1)
		rate := []float64{30, 60, 120}[i%3]
		if i == 0 {
			rate = 60
		}
		if i == 1 {
			rate = 30
		}
		selected := []string{names[order[i%8]]}
		if i%5 == 4 {
			selected = append(selected, names[order[(i+1)%8]])
		}
		q := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{}}
		for j, name := range selected {
			item, ok := dataset.ByName[name]
			if !ok || len(dataset.Choices[name]) == 0 {
				t.Fatalf("未核实可制造目标: %s", name)
			}
			targetRate := rate
			if j > 0 {
				targetRate = []float64{30, 60, 120}[rng.Intn(3)]
			}
			q.Targets = append(q.Targets, quant.Target{ItemID: item.ID, RatePerMinute: targetRate})
		}
		spray := i%3 == 2
		if spray {
			q.Scheme.SchemeForRecipe = make([]quant.RecipeScheme, len(dataset.Recipes))
			for j, recipe := range dataset.Recipes {
				mode := 0
				if recipe.Proliferator&1 != 0 {
					mode = 1
				}
				if i%2 == 0 && recipe.Proliferator&2 != 0 {
					mode = 2
				}
				if mode > 0 {
					q.Scheme.SchemeForRecipe[j] = quant.RecipeScheme{ProductionMode: mode, IncreasePoints: []int{1, 2, 4}[(i/3)%3]}
				}
			}
		}
		density := []string{"dense", "standard"}[i/12]
		out[i] = randomAcceptanceCase{ID: id, Names: selected, Quant: q, Request: core.Request{
			Mode: (i%12)/3 + 1, Density: density, Name: fmt.Sprintf("随机验收%d-%s", randomAcceptanceSeed, id), CollectPerformance: true,
			TaskID: fmt.Sprintf("random-%d-%s", randomAcceptanceSeed, id),
			Setting: core.Setting{BeltLv: 1, InserterLv: 1, MaxLays: 3, MaxMachineInALine: 15, BlockWidthLimit: 50, MergeType: 2,
				EnableFlowMonitor: i%4 == 1 || i%4 == 3, EnableIntermediateBuffer: i%4 >= 2, NeedCoaterNet: spray},
			SurfaceLayout: &core.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, ExpandAdjacent: true, LatitudeZone: zones[i]},
		}}
	}
	return out
}

// randomAcceptanceSource 职责：对当前生产输入作内容指纹；使用：标明并行源码漂移；边界：不读取依赖目录，不将计时当性能结论。
func randomAcceptanceSource(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	h := sha256.New()
	for _, domain := range []string{"helper/blueprint", "helper/quant"} {
		err := filepath.WalkDir(filepath.Join(root, domain), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if strings.HasSuffix(path, "_test.go") || (ext != ".go" && ext != ".yaml" && ext != ".json") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
			h.Write(raw)
			h.Write([]byte{0})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestRandomCoreRefactor24 职责：逐例隔离执行、记录失败并继续；使用：验收主入口；边界：每例内部截止60秒，进程65秒兜底只用于取超时栈与退出。
func TestRandomCoreRefactor24(t *testing.T) {
	if os.Getenv("BLUEPRINT_RANDOM_DIR") == "" {
		t.Skip("仅在指定授权证据目录后运行随机集成验收")
	}
	dir := randomAcceptanceDir(t)
	cases := randomAcceptanceCases(t)
	manifestPath := filepath.Join(dir, "manifest.json")
	if raw, err := os.ReadFile(manifestPath); err == nil {
		var saved []randomAcceptanceCase
		if err := json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved, cases) {
			t.Fatal("清单与当前固定种子输入不符；请使用新的run目录")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	} else {
		randomAcceptanceJSON(t, manifestPath, cases)
	}
	started := time.Now()
	source := randomAcceptanceSource(t)
	randomAcceptanceJSON(t, filepath.Join(dir, "run-start.json"), map[string]any{"Seed": randomAcceptanceSeed, "Started": started, "Go": runtime.Version(), "SourceSHA256": source, "Cases": 24, "CaseTimeoutSeconds": 60})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{}
	for i, c := range cases {
		path := filepath.Join(dir, c.ID+".json")
		var evidence map[string]any
		if raw, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(raw, &evidence); err != nil {
				t.Fatal(err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		status, _ := evidence["Status"].(string)
		if status != "passed" && status != "failed" && status != "panic" && status != "timeout" {
			fmt.Printf("RANDOM_START %s names=%v mode=%d density=%s zone=%s targets=%v\n", c.ID, c.Names, c.Request.Mode, c.Request.Density, c.Request.SurfaceLayout.LatitudeZone, c.Quant.Targets)
			ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestRandomCoreRefactorCase$", "-test.v", "-test.timeout=60s")
			cmd.Env = append(os.Environ(), "BLUEPRINT_RANDOM_CASE="+strconv.Itoa(i))
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			runErr := cmd.Run()
			cancel()
			if err := os.WriteFile(filepath.Join(dir, c.ID+".log"), output.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			raw, readErr := os.ReadFile(path)
			if readErr == nil {
				if err := json.Unmarshal(raw, &evidence); err != nil {
					t.Fatal(err)
				}
			}
			status, _ = evidence["Status"].(string)
			if runErr != nil && (status != "failed" && status != "panic") {
				if evidence == nil {
					evidence = map[string]any{"Case": c}
				}
				status = "failed"
				if bytes.Contains(output.Bytes(), []byte("test timed out")) || ctx.Err() == context.DeadlineExceeded {
					status = "timeout"
				}
				evidence["Status"] = status
				evidence["ProcessError"] = runErr.Error()
				evidence["CompleteProcessOutput"] = output.String()
				randomAcceptanceJSON(t, path, evidence)
			}
			if runErr == nil && status != "passed" {
				t.Errorf("%s子进程没有发布明确成功证据", c.ID)
			}
		}
		row := map[string]any{"ID": c.ID, "Names": c.Names, "Mode": c.Request.Mode, "Density": c.Request.Density, "Zone": c.Request.SurfaceLayout.LatitudeZone, "Targets": c.Quant.Targets, "Status": status, "Error": evidence["Error"], "Summary": evidence["Summary"], "ChecksFailed": evidence["ChecksFailed"], "ProcessError": evidence["ProcessError"]}
		rows = append(rows, row)
		randomAcceptanceJSON(t, filepath.Join(dir, "summary.json"), map[string]any{"Seed": randomAcceptanceSeed, "Started": started, "Completed": len(rows), "Cases": rows, "SourceSHA256": source, "Elapsed": time.Since(started).String()})
		if status != "passed" {
			fmt.Printf("RANDOM_FAIL %s evidence=%s error=%v checks=%v process=%v\n", c.ID, path, evidence["Error"], evidence["ChecksFailed"], evidence["ProcessError"])
			t.Errorf("%s真实验收未通过，完整错误与trace见%s", c.ID, path)
		} else {
			fmt.Printf("RANDOM_PASS %s summary=%v\n", c.ID, evidence["Summary"])
		}
	}
	finalSource := randomAcceptanceSource(t)
	randomAcceptanceJSON(t, filepath.Join(dir, "run-end.json"), map[string]any{"Seed": randomAcceptanceSeed, "Finished": time.Now(), "SourceSHA256": finalSource, "SourceChangedDuringRun": source != finalSource, "TimingUse": "diagnostic-only, no concurrency performance claim"})
}

// TestRandomCoreRefactorCase 职责：执行单例真实LoadDataset→Solve→GenerateDetailed→Decode；使用：父进程隔离入口；边界：外部没有案例编号时跳过，失败完整保存而不截断前缀。
func TestRandomCoreRefactorCase(t *testing.T) {
	indexText := os.Getenv("BLUEPRINT_RANDOM_CASE")
	if indexText == "" {
		t.Skip("仅供随机验收父进程调用")
	}
	dir := randomAcceptanceDir(t)
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || index >= 24 {
		t.Fatal("invalid case index")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []randomAcceptanceCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	c := cases[index]
	path := filepath.Join(dir, c.ID+".json")
	started := time.Now()
	evidence := map[string]any{"Seed": randomAcceptanceSeed, "Case": c, "QuantRequest": c.Quant, "Status": "started", "Started": started}
	defer func() {
		if p := recover(); p != nil {
			evidence["Status"] = "panic"
			evidence["Error"] = fmt.Sprint(p)
			evidence["PanicStack"] = string(debug.Stack())
			t.Errorf("panic: %v", p)
		}
		evidence["Elapsed"] = time.Since(started).String()
		evidence["Finished"] = time.Now()
		randomAcceptanceJSON(t, path, evidence)
	}()
	randomAcceptanceJSON(t, path, evidence)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dataset, err := quant.LoadDataset(c.Quant.Dataset)
	if err != nil {
		evidence["Status"] = "failed"
		evidence["Error"] = err.Error()
		t.Error(err)
		return
	}
	evidence["Dataset"] = dataset.Info
	plan, err := quant.Solve(ctx, c.Quant)
	if err != nil {
		evidence["Status"] = "failed"
		evidence["Error"] = err.Error()
		t.Error(err)
		return
	}
	c.Request.Plan = plan
	evidence["Request"] = c.Request
	evidence["Status"] = "generating"
	randomAcceptanceJSON(t, path, evidence)
	ctx = core.WithPerformance(ctx, func(p core.GenerationPerformance) { evidence["ObservedPerformance"] = p })
	result, trace, err := generator.GenerateDetailed(ctx, c.Request)
	evidence["Result"] = result
	evidence["Trace"] = trace
	if err != nil {
		evidence["Status"] = "failed"
		evidence["Error"] = err.Error()
		evidence["ErrorType"] = fmt.Sprintf("%T", err)
		t.Error(err)
		return
	}
	checks, summary := randomAcceptanceCheck(c.Request, result)
	evidence["ChecksFailed"] = checks
	evidence["Summary"] = summary
	if trace == nil || trace.Steps == 0 {
		checks = append(checks, "缺少真实工作流执行轨迹")
	}
	if ctx.Err() != nil {
		checks = append(checks, "单例60秒截止已到: "+ctx.Err().Error())
	}
	evidence["ChecksFailed"] = checks
	if len(checks) > 0 {
		evidence["Status"] = "failed"
		evidence["Error"] = strings.Join(checks, "\n")
		t.Error(evidence["Error"])
		return
	}
	evidence["Status"] = "passed"
}

// randomAcceptanceCheck 职责：独立解码并核实最终产线、绑定、规则及球面报告；使用：仅真实生成成功后；边界：累积全部不一致，不豁免失败，不宣称游戏内放置或满产。
func randomAcceptanceCheck(req core.Request, result core.Result) ([]string, map[string]any) {
	failed := []string{}
	check := func(ok bool, format string, args ...any) {
		if !ok {
			failed = append(failed, fmt.Sprintf(format, args...))
		}
	}
	summary := map[string]any{"PlanID": req.Plan.PlanID, "Productions": len(req.Plan.Productions), "ExpectedMachines": req.Plan.MachineCount, "Buildings": result.BuildingCount, "Areas": result.Areas, "Warnings": result.Warnings}
	check(req.Plan.PlanID != "" && result.SourcePlanID == req.Plan.PlanID, "SourcePlanID不匹配")
	check(result.Blueprint != "" && result.BuildingCount > 0, "没有真实编码产物")
	doc, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		failed = append(failed, "严格Decode失败: "+err.Error())
		return failed, summary
	}
	check(doc.ChecksumValid, "MD5F未通过")
	check(len(doc.Buildings) == result.BuildingCount && len(doc.Areas) == result.Areas, "Result与Decode建筑/区域数不符")
	areaByID := map[int8]blueprint.Area{}
	for _, a := range doc.Areas {
		_, exists := areaByID[a.Index]
		check(!exists, "Area编号重复%d", a.Index)
		areaByID[a.Index] = a
	}
	_, primaryExists := areaByID[int8(doc.PrimaryAreaIndex)]
	check(primaryExists, "主Area不存在")
	for _, a := range doc.Areas {
		seen := map[int8]bool{}
		cursor := a.Index
		for cursor >= 0 {
			if seen[cursor] {
				check(false, "Area父链循环%d", a.Index)
				break
			}
			seen[cursor] = true
			parent, ok := areaByID[cursor]
			if !ok {
				check(false, "Area父链悬空%d", cursor)
				break
			}
			cursor = parent.ParentIndex
		}
	}
	indices := map[int32]bool{}
	counts := map[int]int{}
	areaCounts := map[int]int{}
	actualMachines := 0
	for _, b := range doc.Buildings {
		check(b.Index >= 0 && !indices[b.Index], "建筑编号重复/负数%d", b.Index)
		indices[b.Index] = true
		_, exists := areaByID[b.AreaIndex]
		check(exists, "建筑%d引用不存在Area%d", b.Index, b.AreaIndex)
		areaCounts[int(b.AreaIndex)]++
		counts[int(b.ItemID)]++
		if kind, _ := quant.MachineKind(int(b.ItemID)); kind > 0 {
			actualMachines++
			check(b.RecipeID > 0, "制造机器%d没有配方", b.Index)
		}
		for _, n := range []float32{b.LocalOffset.X, b.LocalOffset.Y, b.LocalOffset.Z, b.LocalOffset2.X, b.LocalOffset2.Y, b.LocalOffset2.Z, b.Yaw, b.Yaw2, b.Pitch, b.Pitch2, b.Tilt, b.Tilt2} {
			check(!math.IsNaN(float64(n)) && !math.IsInf(float64(n), 0), "建筑%d坐标非有限", b.Index)
		}
	}
	for _, b := range doc.Buildings {
		for _, target := range []int32{b.InputObjIdx, b.OutputObjIdx} {
			check(target == -1 || target >= 0 && indices[target] && target != b.Index, "建筑%d连接引用%d无效", b.Index, target)
		}
	}
	check(actualMachines == req.Plan.MachineCount, "解码制造机器%d != 量化%d", actualMachines, req.Plan.MachineCount)
	seenProductions := map[int]bool{}
	machineOwners := map[int]bool{}
	accounted := map[int]bool{}
	for _, binding := range result.Bindings {
		p := binding.ProductionIndex
		if p < 0 || p >= len(req.Plan.Productions) {
			check(false, "绑定产线越界%d", p)
			continue
		}
		check(!seenProductions[p], "绑定产线重复%d", p)
		seenProductions[p] = true
		production := req.Plan.Productions[p]
		members := map[int]bool{}
		boundMachines := map[int]bool{}
		for _, n := range binding.BuildingIndices {
			check(n >= 0 && n < len(doc.Buildings) && !members[n], "产线%d建筑绑定越界/重复%d", p, n)
			if n < 0 || n >= len(doc.Buildings) {
				continue
			}
			members[n] = true
			accounted[n] = true
			b := doc.Buildings[n]
			if kind, _ := quant.MachineKind(int(b.ItemID)); kind > 0 {
				boundMachines[n] = true
				check(int(b.ItemID) == production.MachineID && int(b.RecipeID) == production.RecipeID, "产线%d机器%d原型/配方不符", p, n)
			}
		}
		check(len(boundMachines) == production.MachineCount, "产线%d解码机器%d != 计划%d", p, len(boundMachines), production.MachineCount)
		check(len(binding.MachineIndices) == production.MachineCount, "产线%dMachineIndices数量不符", p)
		local := map[int]bool{}
		for _, n := range binding.MachineIndices {
			check(boundMachines[n] && !machineOwners[n] && !local[n], "产线%d机器绑定非成员/重复%d", p, n)
			local[n] = true
			machineOwners[n] = true
		}
	}
	check(len(seenProductions) == len(req.Plan.Productions), "产线Bindings覆盖不全")
	for _, n := range result.SharedBuildingIndices {
		check(n >= 0 && n < len(doc.Buildings), "共享建筑越界%d", n)
		accounted[n] = true
	}
	check(len(accounted) == len(doc.Buildings), "建筑归属覆盖%d/%d", len(accounted), len(doc.Buildings))
	if audit := result.RuleAudit; audit == nil {
		check(false, "缺少RuleAudit")
	} else {
		check(audit.QuantDataVersion == req.Plan.DataVersion && audit.Profile.Version != "" && audit.Profile.AssemblySHA256 != "" && audit.Profile.ResourcesSHA256 != "", "规则来源与量化版本不完整")
		check(audit.Coverage == "partial-numeric" && audit.CoordinateSpace == "planet-surface", "规则覆盖/坐标声明不符")
		passed := map[string]bool{}
		for _, c := range audit.Checks {
			passed[c.RuleID] = c.Status == "passed"
			check(c.Status == "passed" || c.Status == "unverified", "规则%s状态%s", c.RuleID, c.Status)
		}
		for _, id := range []string{"final-references", "pasted-connection-slots", "static-build-colliders", "belt-distance-occupancy", "inserter-slot-geometry", "station-spacing-ports", "power-node-spacing", "power-coverage"} {
			check(passed[id], "规则%s未通过", id)
		}
	}
	if logistics := result.LogisticsSummary; logistics == nil {
		check(false, "缺少LogisticsSummary")
	} else {
		check(logistics.FlowGraphDigest != "", "物流流图指纹为空")
		check(logistics.Metrics.RouteCrossings == 0, "仍有路线交叉%d", logistics.Metrics.RouteCrossings)
		check(core.LayoutShapeAccepted(logistics.Metrics), "实际物理包络长宽比%.6f超出近方形上限1.8或度量无效", logistics.Metrics.EnvelopeAspectRatio)
		summary["PhysicalAspectRatio"] = logistics.Metrics.EnvelopeAspectRatio
		summary["ShapeAccepted"] = core.LayoutShapeAccepted(logistics.Metrics)
		for _, c := range logistics.Checks {
			check(c.Status == "passed" || c.Status == "unverified", "物流规则%s状态%s", c.RuleID, c.Status)
		}
	}
	if surface := result.SurfacePlan; surface == nil {
		check(false, "缺少SurfacePlan")
	} else {
		check(surface.GridSegment == 200 && surface.Mode == "explicitTropic", "球面网格/模式不符")
		check(surface.BeltConnectionsChecked && surface.StaticCollidersChecked && surface.CollisionCoverage != "none" && surface.CollisionCoverage != "", "球面终检覆盖缺失")
		check(surface.PrimaryAreaIndex == int(doc.PrimaryAreaIndex), "球面主Area不符")
		check(surface.AnchorLatitudeGrid == surface.SelectedLatitudeGrid, "球面锚点不符")
		seen := map[int]bool{}
		total := 0
		for _, band := range surface.Bands {
			a, exists := areaByID[int8(band.AreaIndex)]
			check(exists && !seen[band.AreaIndex], "球面band重复或Area不存在%d", band.AreaIndex)
			seen[band.AreaIndex] = true
			total += band.BuildingCount
			check(band.BuildingCount == areaCounts[band.AreaIndex], "Area%d建筑数%d != 解码%d", band.AreaIndex, band.BuildingCount, areaCounts[band.AreaIndex])
			check(int(a.AreaSegments) == band.LongitudeSegments && band.LongitudeSegments > 0, "Area%d经度段不符", band.AreaIndex)
			check(band.MinLatitudeGrid <= band.MaxLatitudeGrid, "Area%d纬度边界倒置", band.AreaIndex)
		}
		check(len(seen) == len(doc.Areas) && total == len(doc.Buildings), "SurfacePlan区域/建筑覆盖不全")
		if surface.SafeLatitudeMinDegrees != nil && surface.SafeLatitudeMaxDegrees != nil {
			check(*surface.SafeLatitudeMinDegrees <= *surface.SafeLatitudeMaxDegrees, "安全纬度范围倒置")
		}
		summary["SurfacePlan"] = surface
	}
	digest := sha256.Sum256([]byte(result.Blueprint))
	if density := result.DensityReport; density == nil {
		check(false, "缺少DensityReport")
	} else {
		check(density.Requested == req.Density && density.BlueprintSHA256 == hex.EncodeToString(digest[:]), "密度请求或蓝图指纹不符")
		seen := map[int]bool{}
		for _, p := range density.Productions {
			if p.ProductionIndex < 0 || p.ProductionIndex >= len(req.Plan.Productions) {
				check(false, "密度产线越界")
				continue
			}
			expected := req.Plan.Productions[p.ProductionIndex]
			check(!seen[p.ProductionIndex] && p.MachineID == expected.MachineID && p.MachineCount == expected.MachineCount, "密度产线%d计数/原型不符", p.ProductionIndex)
			seen[p.ProductionIndex] = true
			if req.Density == "dense" {
				check(p.PairedMachines+p.RowMachines+p.SingleMachines == p.MachineCount, "密度产线%d布局来源覆盖不全", p.ProductionIndex)
			}
		}
		check(len(seen) == len(req.Plan.Productions), "密度产线覆盖不全")
	}
	if req.Setting.EnableFlowMonitor {
		check(counts[2030] > 0, "请求监测但没有真实流速监测器")
	}
	if req.Setting.EnableIntermediateBuffer {
		check(counts[2101]+counts[2102]+counts[2106] > 0, "请求缓存但没有真实仓/罐")
	}
	sprayed := false
	for _, p := range req.Plan.Productions {
		sprayed = sprayed || p.IncreasePoints > 0 && p.ProductionMode > 0
	}
	if sprayed {
		check(counts[2313] > 0, "量化请求喷涂但没有真实喷涂机")
	}
	summary["DecodedMachines"] = actualMachines
	summary["ItemCounts"] = counts
	summary["BlueprintSHA256"] = hex.EncodeToString(digest[:])
	summary["Bindings"] = len(result.Bindings)
	return failed, summary
}
