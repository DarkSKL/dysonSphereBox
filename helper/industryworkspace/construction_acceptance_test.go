package industryworkspace

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/saveanalysis"
)

// TestSavedBlueMatrixConstruction 用玩家原档复现精确交付及充分开发，从实际导出的 TXT 核验单星交付和上下游带路。
func TestSavedBlueMatrixConstruction(t *testing.T) {
	path, previous := os.Getenv("DSP_BLUE_MATRIX_SAVE"), os.Getenv("DSP_BLUE_MATRIX_PACK")
	if path == "" || previous == "" {
		t.Skip("未指定电磁矩阵 600 的存档和原规划包")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	selection, world := Describe("blue-matrix", snapshot)
	selection.SaveName = filepath.Base(path)
	tech := selection.Technology
	if tech == nil || tech.BlueprintLimit != 1048576 || tech.FoundationLimit != 45000 || tech.MiningSpeed != 1.5 || tech.ShipCargo != 600 || tech.CruiseSpeed != 1200 || tech.WarpSpeed != 240000 || !tech.WarpUnlocked || tech.DroneCargo != 70 || tech.DroneSpeed != 16 {
		t.Fatalf("与原版 GameHistoryData 读数不同：%+v", tech)
	}
	pack, err := zip.OpenReader(previous)
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Close()
	record, err := pack.Open("规划记录.json")
	if err != nil {
		t.Fatal(err)
	}
	var original struct{ Request Request }
	err = json.NewDecoder(record).Decode(&original)
	record.Close()
	if err != nil {
		t.Fatal(err)
	}
	r := original.Request
	r.SaveID, r.SnapshotID, r.Options.TechnologyMode = selection.SaveID, selection.SnapshotID, "save"
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		expand bool
	}{{"target", false}, {"expanded", true}} {
		t.Run(tc.name, func(t *testing.T) {
			req := r
			req.Planning.DevelopFully = tc.expand
			start := time.Now()
			b, err := Generate(t.Context(), req, selection, world, snapshot, nil)
			if err != nil {
				t.Fatal(err)
			}
			report := b.Report
			if !report.Exportable || len(report.Planets) != 1 || len(report.Routes) != 0 || len(b.Codes) != 1 || report.BlueprintCount != 1 || b.ExportExtension() != ".txt" || len(report.AchievedTargets) != 1 || report.AchievedTargets[0].RatePerMinute < 600-1e-6 {
				t.Fatalf("未得到预期单星净交付：%s，%v，星球 %d，航线 %d，图纸 %d，诊断 %v", report.Status, report.AchievedTargets, len(report.Planets), len(report.Routes), report.BlueprintCount, report.Issues)
			}
			if !tc.expand && (report.ExportName != "电磁矩阵-600每分钟" || report.AchievedTargets[0].RatePerMinute > 600+1e-6) {
				t.Fatalf("精确交付目标不符：%s，%v", report.ExportName, report.AchievedTargets)
			}
			sheet := report.Planets[0]
			if len(sheet.Sheets) != 1 {
				t.Fatalf("该星球应当直接导出一张图，实际 %d 张", len(sheet.Sheets))
			}
			root := os.Getenv("DSP_BLUE_MATRIX_EVIDENCE")
			dir := t.TempDir()
			if root != "" {
				dir = filepath.Join(root, tc.name)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			filename := filepath.Join(dir, report.ExportName+b.ExportExtension())
			if err := b.Export(filename); err != nil {
				t.Fatal(err)
			}
			code, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(string(code), true)
			if err != nil || doc.Name != report.ExportName || len(doc.Buildings) != sheet.Buildings {
				t.Fatal("游戏内标题或建筑不符", err)
			}
			paths := exportedRecipeBeltPaths(t, doc)
			if len(paths) == 0 {
				t.Fatal("图纸内没有能按实际物品和方向追通的跨配方带路")
			}
			if tc.expand {
				if report.AchievedTargets[0].RatePerMinute <= 600+1e-6 {
					t.Fatal("此存档的充分开发样本应有额外交付", report.AchievedTargets)
				}
			} else {
				for _, input := range []struct{ recipe, item int }{{6, 1202}, {50, 1301}} {
					if !slices.ContainsFunc(paths, func(p recipeBeltPath) bool {
						return int(p.FromRecipeID) == input.recipe && p.ToRecipeID == 9 && p.ItemID == input.item
					}) {
						t.Fatalf("矩阵研究站缺少从配方 %d 生产设备直接输送物品 %d 的有向带路", input.recipe, input.item)
					}
				}
			}
			t.Logf("耗时 %s，%s，星球 %d，建筑 %d，物流塔 %d，地基 %d，有向带路配方对 %d，布局 %+v", time.Since(start), report.ExportName, sheet.ID, sheet.Buildings, sheet.Stations, sheet.FoundationCells, len(paths), report.LayoutMetrics)
			if root != "" {
				for name, value := range map[string]any{"report.json": report, "belt-paths.json": paths} {
					data, err := json.MarshalIndent(value, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
