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

// TestSurfaceEncodingUsesMultipleNativeAreas 职责：验证扩展确实分散建筑而非整体移圈；使用：装箱编码回归；边界：用合成独立带块检验格网，不验证机器体积。
func TestSurfaceEncodingUsesMultipleNativeAreas(t *testing.T) {
	b, err := newBuilder(Setting{LocalStationHitRadius: 2})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 14; i++ {
		begin := len(b.all)
		first := b.make(2001, 0, 0, 0)
		last := b.make(2001, 699, 11, 0)
		connect(first, last)
		b.surfaceBlocks = append(b.surfaceBlocks, b.all[begin:])
	}
	encoding, err := b.prepareSurfaceEncoding(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoding.areas) < 2 {
		t.Fatalf("expected actual multi-band expansion, got %d areas", len(encoding.areas))
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
		if records[i].AreaIndex != records[i+1].AreaIndex || records[i].OutputObjIdx != int32(i+1) {
			t.Fatalf("packing severed block %d", i/2)
		}
	}
	for i, area := range encoding.areas {
		band := encoding.plan.Bands[i]
		if !candidateFitsRing(band.MinLatitudeGrid, 0, int(area.Height)-1, int(area.Width), 200) {
			t.Fatalf("area %d crosses a tropic or exceeds a ring: %+v", i, area)
		}
		if i != encoding.plan.PrimaryAreaIndex {
			parent := encoding.plan.Bands[area.ParentIndex]
			if int(area.AnchorLocalOffsetY) != band.MinLatitudeGrid-parent.MinLatitudeGrid {
				t.Fatalf("area %d latitude anchor mismatch", i)
			}
		}
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Name: "surface-multiarea-contract", GameVersion: "0.10.35.29145", PrimaryAreaIndex: int32(encoding.plan.PrimaryAreaIndex), CursorTargetArea: int32(encoding.plan.PrimaryAreaIndex), Areas: encoding.areas, Buildings: records}
	if _, err := blueprint.Encode(document, 1); err != nil {
		t.Fatal(err)
	}
	exportSurfaceContract(t, "multi-area", document, encoding.plan)
}

// TestSurfaceEncodingRejectsSeveredReferences 职责：保护块内拓扑；使用：回归跨块错误；边界：未适配跨圈路由不能凭坐标拼接放行。
func TestSurfaceEncodingRejectsSeveredReferences(t *testing.T) {
	b, err := newBuilder(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	first, second := b.make(2001, 0, 0, 0), b.make(2001, 1, 0, 0)
	connect(first, second)
	b.surfaceBlocks = [][]*placed{{first}, {second}}
	if _, err := b.prepareSurfaceEncoding(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200}); err == nil {
		t.Fatal("cross-block connection was silently severed")
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
		center := result.SurfacePlan.AnchorLatitudeGrid + (int(document.Areas[0].Height)-1)/2
		if absInt(center-target) > 1 {
			t.Fatalf("single-area content was not centered near target: target=%d center=%d", target, center)
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
