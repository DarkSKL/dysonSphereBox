package build

// 文件职责：核对纬带装箱与原生 Area 编码的连接、计数和坐标不变量。
// 使用：Go 回归测试及本机 DLL 验证样本导出。
// 边界：自动几何检查不等同于游戏场景中完成实际放置。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestSurfaceEncodingKeepsPlanarLayout 职责：验证球面策略只选择粘贴锚点而不重新排版；使用：平面兼容回归；边界：不验证游戏场景中的最终 Physics。
func TestSurfaceEncodingKeepsPlanarLayout(t *testing.T) {
	b, err := newBuilder(Setting{LocalStationHitRadius: 2})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 14; i++ {
		first := b.make(2001, 0, 0, 0)
		last := b.make(2001, 699, 11, 0)
		connect(first, last)
	}
	encoding, err := b.prepareSurfaceEncoding(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoding.areas) != 1 {
		t.Fatalf("surface anchor changed planar layout into %d areas", len(encoding.areas))
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	if err := encoding.applyRecords(b.all, records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 28 {
		t.Fatalf("packing changed building count: %d", len(records))
	}
	for i := 0; i < len(records); i += 2 {
		if records[i].AreaIndex != 0 || records[i+1].AreaIndex != 0 || records[i].OutputObjIdx != int32(i+1) {
			t.Fatalf("planar layout changed block %d", i/2)
		}
	}
	area := encoding.areas[0]
	if !candidateFitsRing(encoding.plan.AnchorLatitudeGrid, 0, int(area.Height)-1, int(area.Width), 200) {
		t.Fatalf("single area crosses a tropic or exceeds a ring: %+v", area)
	}
	if area.TropicAnchor != 0 || area.AnchorLocalOffsetX != 0 || area.AnchorLocalOffsetY != 0 {
		t.Fatalf("root area encoded an absolute latitude unexpectedly: %+v", area)
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Name: "surface-multiarea-contract", GameVersion: "0.10.35.29145", PrimaryAreaIndex: int32(encoding.plan.PrimaryAreaIndex), CursorTargetArea: int32(encoding.plan.PrimaryAreaIndex), Areas: encoding.areas, Buildings: records}
	if _, err := blueprint.Encode(document, 1); err != nil {
		t.Fatal(err)
	}
	exportSurfaceContract(t, "multi-area", document, encoding.plan)
}

// TestSurfaceEncodingDoesNotSplitReferences 职责：验证相邻块连接仍作为同一平面蓝图编码；使用：跨块连接回归；边界：不把旧块概念升级成多 Area 路由。
func TestSurfaceEncodingDoesNotSplitReferences(t *testing.T) {
	b, err := newBuilder(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	first, second := b.make(2001, 0, 0, 0), b.make(2001, 1, 0, 0)
	connect(first, second)
	encoding, err := b.prepareSurfaceEncoding(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200})
	if err != nil {
		t.Fatal(err)
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	if err := encoding.applyRecords(b.all, records); err != nil {
		t.Fatal(err)
	}
	if records[0].OutputObjIdx != 1 || records[0].AreaIndex != records[1].AreaIndex {
		t.Fatalf("connection was changed by surface anchor: %+v", records)
	}
}

// TestSurfaceAnchorPreservesLegacyRecords 职责：逐字段比较平面与赤道锚点结果；使用：证明球面开关只增加锚点预检，不重排建筑或连接；边界：仅覆盖可生成的同一数据集和模式。
func TestSurfaceAnchorPreservesLegacyRecords(t *testing.T) {
	request := surfaceGenerationRequest(t)
	request.Mode = 1
	legacy, err := Generate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.SurfaceLayout = &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: 0}
	surface, err := Generate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	legacyDocument, err := blueprint.Decode(legacy.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	surfaceDocument, err := blueprint.Decode(surface.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyDocument.Buildings) != len(surfaceDocument.Buildings) {
		t.Fatalf("surface anchor changed building count: planar=%d surface=%d", len(legacyDocument.Buildings), len(surfaceDocument.Buildings))
	}
	for i := range legacyDocument.Buildings {
		before, after := legacyDocument.Buildings[i], surfaceDocument.Buildings[i]
		before.AreaIndex, after.AreaIndex = 0, 0
		if fmt.Sprintf("%+v", before) != fmt.Sprintf("%+v", after) {
			t.Fatalf("surface anchor changed record %d:\nplanar=%+v\nsurface=%+v", i, before, after)
		}
	}
	if surface.SurfacePlan == nil || surface.SurfacePlan.AnchorLatitudeGrid != 0 || len(surfaceDocument.Areas) != 1 {
		t.Fatalf("surface anchor metadata is invalid: %+v", surface.SurfacePlan)
	}
}

// TestGenerateAllModesKeepMachineCount 职责：覆盖六种模式的原生编码出口；使用：版本回归；边界：通过不代表所有模式的碰撞覆盖完成。
func TestGenerateAllModesKeepMachineCount(t *testing.T) {
	for mode := 1; mode <= 6; mode++ {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			request := surfaceGenerationRequest(t)
			request.Mode = mode
			request.SurfaceLayout = &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}
			result, err := Generate(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			document, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, record := range document.Buildings {
				if record.RecipeID > 0 {
					count++
				}
			}
			if count != request.Plan.Productions[0].MachineCount || len(document.Areas) != result.Areas {
				t.Fatalf("count/areas changed: machines=%d areas=%d", count, len(document.Areas))
			}
			exportSurfaceContract(t, fmt.Sprintf("mode-%d", mode), document, result.SurfacePlan)
		})
	}
}

// TestSingleSurfaceAreaKeepsPlacementLatitudeRange 职责：防止小蓝图写出整带高度造成单行落点；使用：单区粘贴回归；边界：要求目标及相邻行都在同一原生纬带内。
func TestSingleSurfaceAreaKeepsPlacementLatitudeRange(t *testing.T) {
	for _, target := range []int{0, 100, -100} {
		request := surfaceGenerationRequest(t)
		request.SurfaceLayout = &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: target}
		result, err := Generate(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		document, err := blueprint.Decode(result.Blueprint, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(document.Areas) != 1 || document.Areas[0].Height >= 20 {
			t.Fatalf("small blueprint retained excessive area height: %+v", document.Areas)
		}
		for delta := -1; delta <= 1; delta++ {
			if !candidateFitsRing(result.SurfacePlan.AnchorLatitudeGrid+delta, 0, int(document.Areas[0].Height)-1, int(document.Areas[0].Width), 200) {
				t.Fatalf("small single-area blueprint only fits one row: target=%d delta=%d", target, delta)
			}
		}
		if result.SurfacePlan.AnchorLatitudeGrid != target {
			t.Fatalf("single-area anchor changed explicit target: target=%d anchor=%d", target, result.SurfacePlan.AnchorLatitudeGrid)
		}
	}
}

// TestSurfaceStationGroupingKeepsTowerCount 职责：保护适合目标纬带的原始塔式分组；使用：避免无理由增加物流塔；边界：未来为容量避碰拆组时应披露实际账单。
func TestSurfaceStationGroupingKeepsTowerCount(t *testing.T) {
	for _, mode := range []int{3, 4} {
		request := surfaceGenerationRequest(t)
		request.Mode = mode
		request.Plan.Productions[0].MachineCount = 45
		legacy, err := Generate(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.SurfaceLayout = &SurfaceLayout{Mode: "autoTropic", ExpandAdjacent: true}
		surface, err := Generate(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		count := func(code string) int {
			document, err := blueprint.Decode(code, true)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, record := range document.Buildings {
				if record.ItemID == 2103 || record.ItemID == 2104 {
					n++
				}
			}
			return n
		}
		if before, after := count(legacy.Blueprint), count(surface.Blueprint); before != after {
			t.Fatalf("mode %d introduced unnecessary towers: before=%d after=%d", mode, before, after)
		}
	}
}

// exportSurfaceContract 职责：按显式环境变量导出游戏 DLL 核对输入；使用：本机验收；边界：缺省不写仓库或游戏目录，输出仅含独立生成的蓝图数值。
func exportSurfaceContract(t *testing.T, name string, document blueprint.Document, plan *SurfacePlan) {
	t.Helper()
	directory := os.Getenv("DSBOX_SURFACE_EVIDENCE_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(map[string]interface{}{"document": document, "surfacePlan": plan}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
