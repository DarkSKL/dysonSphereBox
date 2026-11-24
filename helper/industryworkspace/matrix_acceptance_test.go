package industryworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/planet"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// TestMatrixSavedScale 职责：对同一真实存档复现白矩阵的不同规模；使用：明确指定样本时运行；边界：只读游戏存档，诊断写到指定验收目录。
func TestMatrixSavedScale(t *testing.T) {
	path := os.Getenv("DSP_MATRIX_SAVE")
	if path == "" {
		t.Skip("未指定白矩阵验收存档")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	selection, world := Describe("matrix-acceptance", snapshot)
	selection.SaveName = filepath.Base(path)
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	for _, rate := range []float64{2000, 5000, 10000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			q := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6006, RatePerMinute: rate}}}
			for _, recipe := range dataset.Recipes {
				mode := 0
				if recipe.Proliferator&2 != 0 {
					mode = 2
				}
				points := 0
				if mode > 0 {
					points = 4
				}
				q.Scheme.SchemeForRecipe = append(q.Scheme.SchemeForRecipe, quant.RecipeScheme{ProductionMode: mode, IncreasePoints: points})
			}
			var receiverStar int64
			var receivers []int64
			for _, site := range selection.Sites {
				if !site.Gas {
					if receiverStar == 0 {
						receiverStar = site.StarID
					}
					if site.StarID == receiverStar {
						receivers = append(receivers, site.ID)
					}
				}
			}
			r := Request{SaveID: selection.SaveID, SnapshotID: selection.SnapshotID, Planning: industryplan.Request{Quant: q, Profile: "standard", Mining: industryplan.MiningSettings{FacilityID: 2301, SpeedMultiplier: 1.8},
				Energy: industryplan.EnergySettings{Default: industryplan.EnergyChoice{Mode: "auto"}}, Dyson: industryplan.DysonSettings{Mode: "sphere", Lenses: true, Supplies: []industryplan.DysonSupply{{StarID: receiverStar, PlanetIDs: receivers, AvailableMW: 300000, Efficiency: .8, Uptime: 1}}}},
				Options: Options{TechnologyMode: "save", Power: true, Fuel: true, Spray: true, Foundation: "construction", BeltLevel: 3, InserterLevel: 3}}
			if os.Getenv("DSP_MATRIX_PREFERENCE") != "" {
				r.Planning.Preference = os.Getenv("DSP_MATRIX_PREFERENCE")
			}
			if os.Getenv("DSP_MATRIX_ENERGY") == "star" {
				r.Planning.Energy.Default = industryplan.EnergyChoice{Mode: "fixed", FacilityID: 2210, FuelItemID: 1803}
			}
			if err := r.Validate(); err != nil {
				t.Fatal(err)
			}
			fullLayout := os.Getenv("DSP_MATRIX_LAYOUT") == "1" && rate == 2000
			ctx := t.Context()
			if deadline, ok := t.Deadline(); ok || !fullLayout {
				if fullLayout {
					// 真存档整星施工沿用 go test -timeout，结束前留出写回验收结果的时间。
					deadline = deadline.Add(-5 * time.Second)
				} else {
					deadline = time.Now().Add(8 * time.Minute)
				}
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline)
				defer cancel()
			}
			// 与桌面共用可恢复时窗；验收自动继续，整体仍受测试 context 限制。
			var control *search.Control
			control = search.New(3*time.Minute, func() { control.Continue() })
			ctx = search.WithControl(ctx, control)
			start := time.Now()
			phase := ""
			var lastPlanet atomic.Int64
			lastUpdate := start
			var stall *time.Timer
			if file := os.Getenv("DSP_MATRIX_STALL_STACK"); fullLayout && file != "" {
				// 只在诊断运行中采样长时间未完成一个模块的调用栈，正常桌面不创建文件。
				stall = time.AfterFunc(90*time.Second, func() {
					if f, err := os.Create(file); err == nil {
						_ = pprof.Lookup("goroutine").WriteTo(f, 2)
						_ = f.Close()
					}
				})
				defer stall.Stop()
			}
			progress := func(p industryplan.Progress) {
				if p.Phase != phase || p.PlanetID != lastPlanet.Load() || time.Since(lastUpdate) >= time.Minute {
					lastUpdate = time.Now()
					if stall != nil {
						stall.Reset(90 * time.Second)
					}
					phase = p.Phase
					lastPlanet.Store(p.PlanetID)
					t.Logf("%s %.1fs 星球=%d 工作量=%d/%d", p.Phase, time.Since(start).Seconds(), p.PlanetID, p.WorkDone, p.WorkTotal)
					if p.RepairReason != "" {
						t.Logf("调整原因 %s", p.RepairReason)
					}
				}
			}
			var value any
			if fullLayout {
				calls, totalMS := 0, 0.0
				failed1504, slow1504 := 0, 0
				var failedMS1504, slowMS1504 float64
				var sampleMu sync.Mutex
				ctx = planet.WithTemplatePerformance(ctx, func(planetID int64, sample build.GenerationPerformance) {
					sampleMu.Lock()
					defer sampleMu.Unlock()
					calls++
					totalMS += sample.TotalDurationMs
					if planetID == 1504 && sample.Outcome == "failed" {
						failed1504++
						failedMS1504 += sample.TotalDurationMs
						if sample.TotalDurationMs >= 25000 {
							slow1504++
							slowMS1504 += sample.TotalDurationMs
						}
					}
					if sample.TotalDurationMs >= 25000 {
						t.Logf("单个制造模板 %.1fs 星球=%d 结果=%s", sample.TotalDurationMs/1000, planetID, sample.Outcome)
					}
				})
				defer func() {
					t.Logf("制造模板生成共 %d 次，合计 %.1fs", calls, totalMS/1000)
					t.Logf("星球 1504 失败模板共 %d 次、%.1fs，其中至少 25 秒的 %d 次、%.1fs", failed1504, failedMS1504/1000, slow1504, slowMS1504/1000)
				}()
				bundle, err := Generate(ctx, r, selection, world, snapshot, progress)
				if err != nil {
					t.Fatal(err)
				}
				value = bundle.Report
				for _, warning := range bundle.Report.Warnings {
					if diagnosticCode.MatchString(warning) {
						t.Fatal("界面泄露内部诊断", warning)
					}
				}
				t.Logf("状态=%s 可导出=%t 星球=%d 耗时=%s", bundle.Report.Status, bundle.Report.Exportable, len(bundle.Report.Planets), time.Since(start))
				if !bundle.Report.Exportable {
					t.Errorf("2000/min 完整施工验收未通过：%s", bundle.Report.Status)
				} else if dir := os.Getenv("DSP_MATRIX_EVIDENCE"); dir != "" {
					if err := bundle.Export(filepath.Join(dir, "宇宙矩阵-2000每分钟.zip")); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				if _, err := applyTechnology(&r, selection.Technology); err != nil {
					t.Fatal(err)
				}
				result, err := industryplan.Plan(ctx, r.Planning, world, nil, progress)
				if err != nil {
					t.Fatal(err)
				}
				value = result
				t.Logf("量化/分工 状态=%s 星球=%d 机器=%d 耗时=%s", result.Status, len(result.Planets), result.Plan.MachineCount, time.Since(start))
				if result.LimitStage != "" {
					t.Fatalf("需求规模仍受到固定预算限制：%s", result.LimitStage)
				}
			}
			if dir := os.Getenv("DSP_MATRIX_EVIDENCE"); dir != "" {
				data, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("matrix-%g.json", rate)), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
