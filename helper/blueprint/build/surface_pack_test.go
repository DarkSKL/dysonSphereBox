package build

// 文件职责：核对有限容量纬带装箱的容量边界、扩展策略、南纬、确定性与区域上限。
// 使用：go test ./helper/blueprint/build。
// 边界：只验证纬圈分段表上的整数几何不变量，不验证游戏 collider 或真实粘贴结果。

import (
	"errors"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// equatorBand200 职责：返回 200 分段赤道纬带；使用：容量用例计算刚好装满数量；边界：依赖当前段表，赤道为 -80..80。
func equatorBand200(t *testing.T) surfacePackedBand {
	t.Helper()
	for _, band := range surfaceLatitudeBands(200) {
		if band.low <= 0 && band.high >= 0 {
			return band
		}
	}
	t.Fatal("equator band missing")
	return surfacePackedBand{}
}

// repeatBlocks 职责：构造同尺寸块序列；使用：容量用例；边界：仅测试。
func repeatBlocks(count int, size surfaceBlockSize) []surfaceBlockSize {
	blocks := make([]surfaceBlockSize, count)
	for i := range blocks {
		blocks[i] = size
	}
	return blocks
}

// requireCapacityError 职责：断言返回容量类错误；使用：超容量用例；边界：不匹配中文文案。
func requireCapacityError(t *testing.T, err error) {
	t.Helper()
	var target *blueprint.Error
	if !errors.As(err, &target) || target.Code != "GEN_E_SURFACE_CAPACITY" {
		t.Fatalf("expected capacity error, got %v", err)
	}
}

// assertPackingGeometry 校验装箱输出的全部几何不变量。
// 职责：检查纬带南到北且互不重叠、纬带内段数恒定、块位于余量内、同纬带块按间距互不重叠。
// 使用：各装箱用例的公共断言。
// 边界：只检查整数占地矩形，不检查块内建筑。
func assertPackingGeometry(t *testing.T, packing *surfacePacking, sizes []surfaceBlockSize, gap int) {
	t.Helper()
	rules := newSurfacePackRules(gap)
	if len(packing.blocks) != len(sizes) || len(packing.bands) == 0 || len(packing.bands) > surfaceMaxAreas {
		t.Fatalf("unexpected packing shape: bands=%d blocks=%d", len(packing.bands), len(packing.blocks))
	}
	maxLatitudeGrid := packing.segment * 5 / 4
	for i, band := range packing.bands {
		if band.low <= -maxLatitudeGrid || band.high >= maxLatitudeGrid {
			t.Fatalf("band %d includes a pole: %+v", i, band)
		}
		if i > 0 && band.low <= packing.bands[i-1].high {
			t.Fatalf("bands not sorted south to north: %+v", packing.bands)
		}
		for row := band.low; row <= band.high; row++ {
			if longitudeSegmentCount(row, packing.segment) != band.segments {
				t.Fatalf("band %d crosses a tropic at row %d", i, row)
			}
		}
	}
	for i, block := range packing.blocks {
		if block.band < 0 || block.band >= len(packing.bands) {
			t.Fatalf("block %d references band %d", i, block.band)
		}
		band, size := packing.bands[block.band], sizes[i]
		if block.x < 0 || block.x+size.width > band.segments*5-rules.ringMargin || block.y < rules.edgeMargin || block.y+size.height > band.high-band.low+1-rules.edgeMargin {
			t.Fatalf("block %d violates band margins: %+v size=%+v band=%+v", i, block, size, band)
		}
		for j := 0; j < i; j++ {
			other := packing.blocks[j]
			if other.band != block.band {
				continue
			}
			otherSize := sizes[j]
			separatedX := block.x >= other.x+otherSize.width+gap || other.x >= block.x+size.width+gap
			separatedY := block.y >= other.y+otherSize.height+gap || other.y >= block.y+size.height+gap
			if !separatedX && !separatedY {
				t.Fatalf("blocks %d and %d overlap or violate gap: %+v %+v", j, i, other, block)
			}
		}
	}
}

// TestPlanSurfaceBlocksExactCapacity 职责：验证目标纬带刚好装满成功、多一块且禁止扩展时失败；使用：容量边界回归；边界：以赤道纬带为例。
func TestPlanSurfaceBlocksExactCapacity(t *testing.T) {
	equator, gap := equatorBand200(t), 2
	rules := newSurfacePackRules(gap)
	size := surfaceBlockSize{width: 100, height: 10}
	perShelf := (rules.usableWidth(equator) + gap) / (size.width + gap)
	shelves := (rules.usableHeight(equator) + gap) / (size.height + gap)
	exact := perShelf * shelves
	layout := &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: 0}
	packing, err := planSurfaceBlocks(layout, repeatBlocks(exact, size), gap)
	if err != nil {
		t.Fatalf("exact capacity %d rejected: %v", exact, err)
	}
	if len(packing.bands) != 1 || packing.bands[0] != equator || packing.selected != 0 || packing.preferred != 0 {
		t.Fatalf("exact capacity left the target band: %+v", packing)
	}
	assertPackingGeometry(t, packing, repeatBlocks(exact, size), gap)
	_, err = planSurfaceBlocks(layout, repeatBlocks(exact+1, size), gap)
	requireCapacityError(t, err)

	// 单块恰好等于安全可用区成功，超出一格在任何模式下都明确失败。
	whole := surfaceBlockSize{width: rules.usableWidth(equator), height: rules.usableHeight(equator)}
	if _, err := planSurfaceBlocks(layout, []surfaceBlockSize{whole}, gap); err != nil {
		t.Fatalf("block equal to usable area rejected: %v", err)
	}
	wide := surfaceBlockSize{width: whole.width + 1, height: 1}
	for _, expand := range []bool{false, true} {
		_, err := planSurfaceBlocks(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: expand}, []surfaceBlockSize{wide}, gap)
		requireCapacityError(t, err)
	}
}

// TestPlanSurfaceBlocksStrictWithoutExpansion 职责：验证禁止扩展时不迁移到其他纬带；使用：explicit 策略回归；边界：高纬目标容量不足也不得回退。
func TestPlanSurfaceBlocksStrictWithoutExpansion(t *testing.T) {
	// 纬向 236..240 为 16 段 80 格环，宽 100 的块只能放到更低纬度。
	layout := &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: 238}
	blocks := []surfaceBlockSize{{width: 100, height: 1}}
	_, err := planSurfaceBlocks(layout, blocks, 2)
	requireCapacityError(t, err)

	layout.ExpandAdjacent = true
	packing, err := planSurfaceBlocks(layout, blocks, 2)
	if err != nil {
		t.Fatal(err)
	}
	band := packing.bands[packing.blocks[0].band]
	if packing.preferred != 238 || band.segments*5-4 < 100 || band.high >= 236 || packing.selected != bandCenter(band) {
		t.Fatalf("high latitude expansion picked %+v selected=%d", band, packing.selected)
	}
	assertPackingGeometry(t, packing, blocks, 2)
}

// TestPlanSurfaceBlocksSpreadsAcrossBands 职责：验证扩展是逐块分散而不是整体迁移；使用：多纬带回归；边界：14 块 700x12 对应父级编码集成样本。
func TestPlanSurfaceBlocksSpreadsAcrossBands(t *testing.T) {
	blocks := repeatBlocks(14, surfaceBlockSize{width: 700, height: 12})
	for _, layout := range []*SurfaceLayout{
		{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: 10, ExpandAdjacent: true},
		{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true},
	} {
		packing, err := planSurfaceBlocks(layout, blocks, 2)
		if err != nil {
			t.Fatalf("%s: %v", layout.Mode, err)
		}
		assertPackingGeometry(t, packing, blocks, 2)
		if len(packing.bands) < 2 {
			t.Fatalf("%s: expected multiple bands, got %+v", layout.Mode, packing.bands)
		}
		primary := packing.bands[packing.blocks[0].band]
		counts := map[int]int{}
		for _, block := range packing.blocks {
			counts[block.band]++
		}
		// 赤道纬带 157 可用行、每块占 12+2 行，应先装满 11 块再扩展。
		if primary.segments != 200 || counts[packing.blocks[0].band] != 11 {
			t.Fatalf("%s: target band was not filled first: %+v counts=%v", layout.Mode, primary, counts)
		}
		if layout.Mode == "explicitTropic" && packing.selected != 10 {
			t.Fatalf("explicit selected should keep preferred: %d", packing.selected)
		}
		if layout.Mode == "autoTropic" && (packing.selected != 0 || packing.preferred != 0) {
			t.Fatalf("auto selected/preferred: %d/%d", packing.selected, packing.preferred)
		}
	}
	_, err := planSurfaceBlocks(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200}, blocks, 2)
	requireCapacityError(t, err)
}

// TestPlanSurfaceBlocksSouthernLatitude 职责：验证南纬负索引被正常选择与扩展；使用：南半球回归；边界：南北对称纬带同分时自动选南侧。
func TestPlanSurfaceBlocksSouthernLatitude(t *testing.T) {
	layout := &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: -100}
	blocks := []surfaceBlockSize{{width: 300, height: 20}, {width: 300, height: 20}}
	packing, err := planSurfaceBlocks(layout, blocks, 2)
	if err != nil {
		t.Fatal(err)
	}
	band := packing.bands[packing.blocks[0].band]
	if band.low > -100 || band.high < -100 || packing.selected != -100 || packing.preferred != -100 {
		t.Fatalf("southern target misplaced: %+v selected=%d", band, packing.selected)
	}
	assertPackingGeometry(t, packing, blocks, 2)

	// 赤道装满后，南北 160 段纬带与赤道距离相同，自动扩展必须能选中南纬负索引纬带。
	overflow := repeatBlocks(12, surfaceBlockSize{width: 700, height: 12})
	auto, err := planSurfaceBlocks(&SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}, overflow, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := auto.bands[auto.blocks[11].band]; got.high >= 0 || got.segments != 160 {
		t.Fatalf("auto overflow did not use southern band: %+v", got)
	}
	assertPackingGeometry(t, auto, overflow, 2)
	south := []surfaceBlockSize{{width: 595, height: 21}}
	_, err = planSurfaceBlocks(&SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: -200}, south, 2)
	requireCapacityError(t, err)
	explicitSouth, err := planSurfaceBlocks(&SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: -200, ExpandAdjacent: true}, south, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := explicitSouth.bands[explicitSouth.blocks[0].band]; got.high >= 0 || explicitSouth.selected >= 0 {
		t.Fatalf("southern expansion crossed to the north: %+v selected=%d", got, explicitSouth.selected)
	}
}

// TestPlanSurfaceBlocksDeterministicOrder 职责：验证同输入同输出且同纬带内保持输入顺序；使用：确定性回归；边界：混合尺寸序列固定生成。
func TestPlanSurfaceBlocksDeterministicOrder(t *testing.T) {
	blocks := make([]surfaceBlockSize, 60)
	for i := range blocks {
		blocks[i] = surfaceBlockSize{width: 40 + (i*37)%260, height: 3 + (i*11)%17}
	}
	layout := &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}
	first, err := planSurfaceBlocks(layout, blocks, 3)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 5; run++ {
		again, err := planSurfaceBlocks(layout, blocks, 3)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatal("packing is not deterministic")
		}
	}
	assertPackingGeometry(t, first, blocks, 3)
	last := map[int]surfacePackedBlock{}
	for i, block := range first.blocks {
		if previous, ok := last[block.band]; ok && (block.y < previous.y || block.y == previous.y && block.x <= previous.x) {
			t.Fatalf("block %d placed before an earlier block in band %d", i, block.band)
		}
		last[block.band] = block
	}
}

// TestPackSurfaceLayoutAreaLimit 职责：验证纬带数不超过 64；使用：区域上限回归；边界：真实网格最多约 31 段，使用合成纬带触发上限。
func TestPackSurfaceLayoutAreaLimit(t *testing.T) {
	bands := make([]surfacePackedBand, 70)
	for i := range bands {
		// 每条合成纬带 10 行、可用 6 行，只能容纳一个高 6 的块。
		bands[i] = surfacePackedBand{low: i * 10, high: i*10 + 9, segments: 4}
	}
	rules := newSurfacePackRules(2)
	size := surfaceBlockSize{width: 16, height: 6}
	packing, err := packSurfaceLayout("explicitTropic", 200, 0, true, bands, repeatBlocks(surfaceMaxAreas, size), rules, surfaceMaxAreas)
	if err != nil {
		t.Fatalf("64 areas rejected: %v", err)
	}
	if len(packing.bands) != surfaceMaxAreas {
		t.Fatalf("expected 64 bands, got %d", len(packing.bands))
	}
	_, err = packSurfaceLayout("explicitTropic", 200, 0, true, bands, repeatBlocks(surfaceMaxAreas+1, size), rules, surfaceMaxAreas)
	requireCapacityError(t, err)
	_, err = packSurfaceLayout("autoTropic", 200, 0, true, bands, repeatBlocks(surfaceMaxAreas+1, size), rules, surfaceMaxAreas)
	requireCapacityError(t, err)
}

// TestPlanSurfaceBlocksValidation 职责：验证非法布局与输入被拒绝；使用：参数校验回归；边界：只检查错误存在。
func TestPlanSurfaceBlocksValidation(t *testing.T) {
	ok := []surfaceBlockSize{{width: 10, height: 5}}
	cases := []struct {
		name   string
		layout *SurfaceLayout
		blocks []surfaceBlockSize
		gap    int
	}{
		{"nil", nil, ok, 2},
		{"planar", &SurfaceLayout{Mode: "planar"}, ok, 2},
		{"unknown", &SurfaceLayout{Mode: "spiral", GridSegment: 200}, ok, 2},
		{"segment-small", &SurfaceLayout{Mode: "autoTropic", GridSegment: 36}, ok, 2},
		{"segment-large", &SurfaceLayout{Mode: "autoTropic", GridSegment: 516}, ok, 2},
		{"segment-odd", &SurfaceLayout{Mode: "autoTropic", GridSegment: 202}, ok, 2},
		{"latitude", &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeGridIndex: -251}, ok, 2},
		{"gap", &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}, ok, 1},
		{"empty", &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}, nil, 2},
		{"zero-size", &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}, []surfaceBlockSize{{width: 0, height: 5}}, 2},
	}
	for _, tc := range cases {
		if _, err := planSurfaceBlocks(tc.layout, tc.blocks, tc.gap); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
	}
	// 南极点不属于任何纬带：禁止扩展时极区纬带无可用行而失败，允许扩展时向赤道方向寻找。
	_, err := planSurfaceBlocks(&SurfaceLayout{Mode: "explicitTropic", LatitudeGridIndex: -250}, ok, 2)
	requireCapacityError(t, err)
	packing, err := planSurfaceBlocks(&SurfaceLayout{Mode: "explicitTropic", LatitudeGridIndex: -250, ExpandAdjacent: true}, ok, 2)
	if err != nil {
		t.Fatalf("default segment / southern pole expansion rejected: %v", err)
	}
	if packing.segment != 200 || packing.preferred != -250 || packing.bands[0].high >= 0 {
		t.Fatalf("southern pole expansion: %+v", packing)
	}
}
