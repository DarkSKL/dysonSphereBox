package build

// 文件职责：复核原生高纬整厂的生成与导出记录。
// 使用：按子测试筛选产线/位置/模式；边界：数值检查与示意图不能代替游戏实测。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLatitudeAcceptance(t *testing.T) {
	for _, scenario := range []struct {
		item, rate int
		monitor    bool
	}{{6001, 240, false}, {6001, 480, false}, {6001, 240, true}, {2304, 30, false}, {5002, 60, false}, {1301, 120, false}, {1202, 120, false}} {
		if scenario.item == 5002 && os.Getenv("DSBOX_EXTENDED_LATITUDE_ACCEPTANCE") != "1" {
			continue
		}
		rate, item := scenario.rate, scenario.item
		plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Settings: quant.Settings{MineralizeList: map[string]bool{"硅石": true}}, Targets: []quant.Target{{ItemID: item, RatePerMinute: float64(rate)}}})
		if err != nil {
			t.Fatal(err)
		}
		if item == 2304 || item == 5002 {
			siliconOre := false
			for _, input := range plan.RawInputs {
				siliconOre = siliconOre || input.ItemID == 1003
			}
			for _, production := range plan.Productions {
				if production.MainItemID == 1003 {
					t.Fatal("不允许从石矿制造硅石")
				}
			}
			if !siliconOre {
				t.Fatal("测试计划缺少硅石原矿输入")
			}
		}
		for _, zone := range []string{"ring4-north-south", "ring4-north-center", "ring4-north-north", "ring4-south-south", "ring4-south-center", "ring4-south-north"} {
			for _, mode := range []int{1, 2, 3, 4} {
				t.Run(fmt.Sprintf("item%d-rate%d-%s-mode%d-monitor%t", item, rate, zone, mode, scenario.monitor), func(t *testing.T) {
					result, err := Generate(WithProgress(context.Background(), func(p Progress) {
						if p.Stage == "layout-rejected" {
							t.Log(p.Message)
						}
					}), Request{Plan: plan, Mode: mode, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4, EnableFlowMonitor: scenario.monitor}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: zone}})
					if err != nil {
						t.Fatal(err)
					}
					doc, err := blueprint.Decode(result.Blueprint, true)
					if err != nil {
						t.Fatal(err)
					}
					band, _, err := densityLatitudeBand(zone, 200)
					if err != nil {
						t.Fatal(err)
					}
					p := result.SurfacePlan
					if p.LayoutHeight > 25 || p.AnchorLatitudeGrid < band.low || p.AnchorLatitudeGrid+p.LayoutHeight-1 > band.high {
						t.Fatal("超出纬带")
					}
					selection, _ := densityLatitudeSelectionFor(zone)
					gap := p.AnchorLatitudeGrid - band.low
					if selection.position == "north" {
						gap = band.high - p.AnchorLatitudeGrid - p.LayoutHeight + 1
					}
					if selection.position != "center" && gap > 4 {
						t.Fatalf("边缘选区偏离边界%d格", gap)
					}
					machines, facilities := 0, 0
					for _, r := range doc.Buildings {
						if r.RecipeID > 0 {
							machines++
							if math.Abs(math.Remainder(float64(r.Yaw), 90)) > 1e-3 || r.Tilt != 0 || r.Pitch != 0 {
								t.Fatal("机器倾斜")
							}
						}
						if surfaceIsStation(r.ItemID) || r.ItemID == 2107 {
							facilities++
						}
						if item == 6001 && surfaceIsStation(r.ItemID) {
							for slot := 0; slot < 5; slot++ {
								item := r.Parameters[slot*6]
								if item != 0 && item != 1001 && item != 1002 && item != 6001 {
									t.Fatal("中间品绕行物流塔")
								}
							}
						}
					}
					if machines != plan.MachineCount {
						t.Fatal("生产机器数量不符")
					}
					if item == 6001 && (mode >= 3 && facilities != 1 || mode == 2 && facilities != 3) {
						t.Fatalf("物流设施数量回退为%d", facilities)
					}
					if p.SafeLatitudeMinDegrees == nil || p.SafeLatitudeMaxDegrees == nil {
						t.Fatal("未给出可放置范围")
					}
					for _, degrees := range []float64{*p.SafeLatitudeMinDegrees, *p.SafeLatitudeMaxDegrees} {
						cloned := *p
						cloned.AnchorLatitudeGrid = int(math.Round(degrees / .36))
						cloned.CollisionCoverage = "none"
						encoding := &surfaceEncoding{plan: &cloned, areas: doc.Areas}
						if err := validateRelocatedSurface(context.WithValue(context.Background(), denseCollisionKey{}, true), Request{}, encoding, doc.Buildings); err != nil {
							t.Fatal(err)
						}
					}

					t.Logf("%dx%d entities=%d", result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight, len(doc.Buildings))
					if dir := os.Getenv("DSBOX_BLUEPRINT_TEST_OUTPUT"); dir != "" {
						if !filepath.IsAbs(dir) {
							t.Fatal("导出目录必须是绝对路径")
						}
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						name := fmt.Sprintf("item%d-rate%d-%s-mode%d-monitor%t", item, rate, zone, mode, scenario.monitor)
						data, err := json.MarshalIndent(struct {
							Document    blueprint.Document `json:"document"`
							SurfacePlan *SurfacePlan       `json:"surfacePlan"`
							Metrics     LayoutMetrics      `json:"metrics"`
						}{doc, result.SurfacePlan, result.LogisticsSummary.Metrics}, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(filepath.Join(dir, name+".json"), data, 0644); err != nil {
							t.Fatal(err)
						}
						if err = os.WriteFile(filepath.Join(dir, name+".txt"), []byte(result.Blueprint), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
