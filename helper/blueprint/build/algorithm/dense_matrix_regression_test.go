package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestDenseMatrix480Equator 职责：重放用户480/min赤道矩阵的包络和塔转发回退；使用：完整生成后核对实际实体；边界：可选导出供逐层图片检查，不能代替游戏实测。
func TestDenseMatrix480Equator(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 480}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{1, 3, 4} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			result, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: "equator"}, Name: fmt.Sprintf("480矩阵赤道复核-模式%d", mode)})
			if err != nil {
				t.Fatal(err)
			}
			document, err := blueprint.Decode(result.Blueprint, true)
			if err != nil || !document.ChecksumValid {
				t.Fatalf("严格解码失败：%v", err)
			}
			if result.SurfacePlan == nil || result.LogisticsSummary == nil {
				t.Fatal("缺少球面布局或物流校验结果")
			}
			towers, machines := 0, 0
			for _, record := range document.Buildings {
				if record.RecipeID > 0 {
					machines++
				}
				if surfaceIsStation(record.ItemID) {
					towers++
					// 原料铁矿、铜矿和成品矩阵才使用塔槽；铁块、铜块、磁铁与线圈等中间材料必须直接输送。
					if len(record.Parameters) < 30 {
						t.Fatal("塔库存参数不完整")
					}
					for slot := 0; slot < 5; slot++ {
						item := record.Parameters[slot*6]
						if item != 0 && item != 1001 && item != 1002 && item != 6001 {
							t.Fatalf("中间材料%d绕行塔库存", item)
						}
					}
				}
			}
			if machines != 64 {
				t.Fatalf("480/min的完整产线应有64台机器，实际%d", machines)
			}
			if mode > 1 && towers != 1 {
				t.Fatalf("当前外部物料和流量一塔足够，实际%d塔", towers)
			}
			metrics := result.LogisticsSummary.Metrics
			// 保留布局搜索的变化空间，同时拦截原料入口长尾和占地翻倍回退。
			if result.SurfacePlan.LayoutWidth > 50 || result.SurfacePlan.LayoutHeight > 50 || metrics.EnvelopeArea > 1500 || metrics.EnvelopeAspectRatio > 2 {
				t.Fatalf("矩阵布局丢失紧凑矩形：%+v", metrics)
			}
			t.Logf("machines=%d buildings=%d towers=%d footprint=%dx%d belts=%d meters=%.1f", plan.MachineCount, result.BuildingCount, towers, result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight, result.LogisticsSummary.Metrics.BeltCount, result.LogisticsSummary.Metrics.TotalBeltLengthMeters)
			for _, diagnostic := range result.LogisticsSummary.SearchCandidates {
				if diagnostic.Scope == "dense-station-blackbox" {
					t.Logf("blackbox %s %s %s", diagnostic.Parameters, diagnostic.Status, diagnostic.Reason)
				}
			}
			if directory := os.Getenv("DSBOX_BLUEPRINT_TEST_OUTPUT"); directory != "" {
				if !filepath.IsAbs(directory) {
					t.Fatal("导出目录必须是绝对路径")
				}
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				stem := fmt.Sprintf("matrix480-mode%d", mode)
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
