package build

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestDenseEquatorEdgeOrientation 职责：验证赤道两侧的建筑朝向、真实物流与紧凑包络；使用：边缘适配阶段回归；边界：完整生成和离线碰撞校验不等价于游戏拍建。
func TestDenseEquatorEdgeOrientation(t *testing.T) {
	for _, rate := range []float64{240, 480} {
		plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: rate}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, zone := range []string{"equator-north", "equator-south"} {
			for _, mode := range []int{1, 2, 3, 4} {
				t.Run(fmt.Sprintf("%.0f/%s/mode%d", rate, zone, mode), func(t *testing.T) {
					req := Request{Plan: plan, Mode: mode, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, LatitudeZone: zone}, Name: "赤道边缘朝向回归"}
					result, err := Generate(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					document, err := blueprint.Decode(result.Blueprint, true)
					if err != nil || !document.ChecksumValid {
						t.Fatalf("严格解码失败：%v", err)
					}
					deviation, towers, machines := 0., 0, 0
					for _, record := range document.Buildings {
						if record.RecipeID > 0 {
							machines++
						}
						if record.RecipeID > 0 || surfaceIsStation(record.ItemID) {
							deviation = math.Max(deviation, math.Abs(math.Remainder(float64(record.Yaw), 90)))
						}
						if surfaceIsStation(record.ItemID) {
							towers++
							for slot := 0; slot < 5; slot++ {
								if len(record.Parameters) <= slot*6 {
									t.Fatal("塔库存参数不完整")
								}
								item := record.Parameters[slot*6]
								if item != 0 && item != 1001 && item != 1002 && item != 6001 {
									t.Errorf("中间材料%d改由塔转发", item)
								}
							}
						}
					}
					t.Logf("footprint=%dx%d towers=%d yawDeviation=%.3f anchor=%d", result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight, towers, deviation, result.SurfacePlan.AnchorLatitudeGrid)
					if deviation > .001 {
						t.Errorf("建筑偏转%.3f度，仍未保持正交朝向", deviation)
					}
					if machines != plan.MachineCount {
						t.Errorf("产线不完整：预期%d台，实际%d台", plan.MachineCount, machines)
					}
					if mode >= 3 && towers != 1 {
						t.Errorf("一塔外部物流被回退为%d塔", towers)
					}
					metrics := result.LogisticsSummary.Metrics
					if result.SurfacePlan.LayoutWidth > 55 || result.SurfacePlan.LayoutHeight > 55 || metrics.EnvelopeArea > 1800 || metrics.EnvelopeAspectRatio > 2 {
						t.Errorf("边缘适配失去紧凑矩形：%+v", metrics)
					}
					if !result.SurfacePlan.StaticCollidersChecked || !result.SurfacePlan.BeltConnectionsChecked {
						t.Error("缺少完整球面几何校验")
					}
					band, _, err := densityLatitudeBand(zone, 200)
					if err != nil {
						t.Fatal(err)
					}
					gap := result.SurfacePlan.AnchorLatitudeGrid - band.low
					if zone == "equator-north" {
						gap = band.high - result.SurfacePlan.AnchorLatitudeGrid - result.SurfacePlan.LayoutHeight + 1
					}
					if gap < 0 || gap > 4 {
						t.Errorf("结果没有落在所选边缘：距边界%d格", gap)
					}
					// 使用解码后的实际记录，在建议点和宣称的范围端点再次复验，避免仅检查内存中的未导出实体。
					if result.SurfacePlan.SafeLatitudeMinDegrees == nil || result.SurfacePlan.SafeLatitudeMaxDegrees == nil {
						t.Fatal("缺少已验证的放置范围")
					}
					for _, anchor := range []int{result.SurfacePlan.AnchorLatitudeGrid, int(math.Round(*result.SurfacePlan.SafeLatitudeMinDegrees / .36)), int(math.Round(*result.SurfacePlan.SafeLatitudeMaxDegrees / .36))} {
						if anchor < band.low || anchor+result.SurfacePlan.LayoutHeight-1 > band.high {
							t.Fatalf("已声明放置范围超出纬带：%d", anchor)
						}
						planCopy := *result.SurfacePlan
						planCopy.AnchorLatitudeGrid = anchor
						encoding := &surfaceEncoding{areas: document.Areas, plan: &planCopy}
						if err := validateRelocatedSurface(context.WithValue(context.Background(), denseCollisionKey{}, true), req, encoding, document.Buildings); err != nil {
							t.Fatalf("导出蓝图在声明起点%d复验失败：%v", anchor, err)
						}
					}
					if towers > 1 {
						for _, diagnostic := range result.LogisticsSummary.SearchCandidates {
							if diagnostic.Scope == "dense-station-blackbox" {
								t.Logf("%s: %s", diagnostic.Parameters, diagnostic.Reason)
							}
						}
					}
					if directory := os.Getenv("DSBOX_BLUEPRINT_TEST_OUTPUT"); directory != "" {
						if !filepath.IsAbs(directory) {
							t.Fatal("导出目录必须是绝对路径")
						}
						if err := os.MkdirAll(directory, 0755); err != nil {
							t.Fatal(err)
						}
						stem := fmt.Sprintf("matrix%.0f-%s-mode%d", rate, zone, mode)
						data, err := json.MarshalIndent(struct {
							Document    blueprint.Document `json:"document"`
							SurfacePlan *SurfacePlan       `json:"surfacePlan"`
							Metrics     LayoutMetrics      `json:"metrics"`
						}{document, result.SurfacePlan, result.LogisticsSummary.Metrics}, "", "  ")
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(directory, stem+".json"), data, 0644); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(directory, stem+".txt"), []byte(result.Blueprint), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
