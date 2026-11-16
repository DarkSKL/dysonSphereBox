package build

// 文件职责：验证显式授权下的有界真实邻带范围、窗口和原选区许可。
// 使用：独立定向回归，不运行整厂生成；边界：不替代完整实体碰撞、产量或游戏验收。

import (
	"context"
	"reflect"
	"testing"
)

// TestDenseAdjacentBoundedEquator 职责：固定赤道三种位置的真实带和窗口；使用：小规模范围回归；边界：6007仅参与窗口预算，不生成实体。
func TestDenseAdjacentBoundedEquator(t *testing.T) {
	for _, tc := range []struct {
		zone       string
		bands      []surfacePackedBand
		windowRows int
	}{
		{"equator", []surfacePackedBand{{low: -130, high: -81, segments: 160}, {low: -80, high: 80, segments: 200}, {low: 81, high: 130, segments: 160}}, 257},
		{"equator-south", []surfacePackedBand{{low: -80, high: 80, segments: 200}, {low: 81, high: 130, segments: 160}}, 207},
		{"equator-north", []surfacePackedBand{{low: -130, high: -81, segments: 160}, {low: -80, high: 80, segments: 200}}, 207},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			selection, ok := densityLatitudeSelectionFor(tc.zone)
			if !ok {
				t.Fatal("赤道选区无法解析")
			}
			bands := denseLatitudeAllowedBands(selection, true)
			if !reflect.DeepEqual(bands, tc.bands) {
				t.Fatalf("原生纬带=%+v，期望%+v", bands, tc.bands)
			}
			windows := denseAdjacentFrameExtents(selection, bands, 6007)
			if !reflect.DeepEqual(windows, []int{tc.windowRows}) {
				t.Fatalf("窗口=%v，期望[%d]", windows, tc.windowRows)
			}
			if low, high := denseLatitudeReferenceRange(selection); low != -80 || high != 80 {
				t.Fatalf("原选区参考点被改变：%d至%d", low, high)
			}
		})
	}
}

// TestDenseAdjacentBoundedSelections 职责：逐行证明所有选区保留原范围和旧112格扩展；使用：全选区边界回归；边界：只允许原选区外首条邻带贡献新增行，极点余量不能裁掉原选区。
func TestDenseAdjacentBoundedSelections(t *testing.T) {
	all := surfaceLatitudeBands(200)
	for _, option := range DensityLatitudeOptions() {
		t.Run(option.Value, func(t *testing.T) {
			selection, ok := densityLatitudeSelectionFor(option.Value)
			if !ok {
				t.Fatal("目录选区无法解析")
			}
			selected := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
			if strict := denseLatitudeAllowedBands(selection, false); !reflect.DeepEqual(strict, selected) {
				t.Fatalf("未扩展范围改变：%+v，原范围%+v", strict, selected)
			}
			originalLow, originalHigh := selected[0].low, selected[len(selected)-1].high
			// 旧最小覆盖仍是独立必须保留的范围；两侧展开延续原有整数中心舍入。
			oldLow, oldHigh := originalLow, originalHigh
			switch selection.position {
			case "south":
				oldHigh = max(oldHigh, min(247, originalLow+111))
			case "north":
				oldLow = min(oldLow, max(-247, originalHigh-111))
			default:
				center := (originalLow + originalHigh) / 2
				oldLow = min(oldLow, max(-247, center-56))
				oldHigh = max(oldHigh, min(247, center+55))
			}
			// 用逐行集合定义并集，邻带只相对原范围取，不从旧112格覆盖范围取。
			expected := map[int]int{}
			for _, native := range all {
				neighbor := selection.position != "south" && native.high == originalLow-1 ||
					selection.position != "north" && native.low == originalHigh+1
				for row := native.low; row <= native.high; row++ {
					if row >= oldLow && row <= oldHigh || neighbor && row >= -247 && row <= 247 {
						expected[row] = native.segments
					}
				}
			}
			bands := denseLatitudeAllowedBands(selection, true)
			actual := map[int]int{}
			for i, band := range bands {
				if band.low > band.high || i > 0 && bands[i-1].high+1 != band.low {
					t.Fatalf("纬带不连续或顺序错误：%+v", bands)
				}
				for row := band.low; row <= band.high; row++ {
					actual[row] = band.segments
				}
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("范围未保持有界并集：%+v，旧范围%d至%d", bands, oldLow, oldHigh)
			}
			for _, band := range selected {
				for row := band.low; row <= band.high; row++ {
					if actual[row] != band.segments {
						t.Fatalf("原选区行%d或原生段数丢失", row)
					}
				}
			}
			for row := range actual {
				if row < -249 || row > 249 || (row < originalLow || row > originalHigh) && (row < -247 || row > 247) {
					t.Fatalf("新增行%d越过极点余量", row)
				}
			}
			if low, high := denseLatitudeReferenceRange(selection); low != originalLow || high != originalHigh {
				t.Fatalf("原参考范围改变：%d至%d", low, high)
			}
		})
	}
}

// TestDenseAdjacentBoundedNonRecursive 职责：固定旧112格已跨多带时的非递归结果；使用：防止围绕扩后范围再加带；边界：旧覆盖超过一条邻带的部分仍保留。
func TestDenseAdjacentBoundedNonRecursive(t *testing.T) {
	for _, tc := range []struct {
		zone      string
		low, high int
	}{
		{"ring2-north-center", -80, 160},
		{"ring2-south-center", -161, 80},
		{"ring3-north-center", 81, 198},
		{"ring4-north-center", 112, 223},
		{"ring5-north-center", 156, 249},
		{"ring5-south-center", -249, -156},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			selection, _ := densityLatitudeSelectionFor(tc.zone)
			bands := denseLatitudeAllowedBands(selection, true)
			if bands[0].low != tc.low || bands[len(bands)-1].high != tc.high {
				t.Fatalf("范围=%+v，期望%d至%d", bands, tc.low, tc.high)
			}
		})
	}
}

// TestDenseAdjacentBoundedRequestPermission 职责：验证更大非极区窗口仍受显式许可和规格校验约束；使用：候选准备回归；边界：仅枚举与准备，不运行生产候选。
func TestDenseAdjacentBoundedRequestPermission(t *testing.T) {
	ctx := context.Background()
	for _, zone := range []string{"equator", "equator-north", "equator-south"} {
		t.Run(zone, func(t *testing.T) {
			input := &PreparedInput{request: Request{Mode: 1, Density: densityDense, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: zone, ExpandAdjacent: true}}, fingerprint: "bounded-adjacent-test"}
			input.request.Plan.MachineCount = 6007
			options := DefaultSurfaceSearchOptions()
			options.ExpandAdjacent = true
			specs, err := EnumerateSurfaceCandidates(ctx, input, options)
			if err != nil || len(specs) == 0 {
				t.Fatalf("枚举失败：%v，规格%+v", err, specs)
			}
			for _, spec := range specs {
				prepared, err := PrepareSurfaceCandidate(ctx, input, spec)
				if err != nil {
					t.Fatalf("合法规格%+v被拒绝：%v", spec, err)
				}
				if prepared.request.SurfaceLayout.LatitudeZone != zone || !prepared.request.SurfaceLayout.ExpandAdjacent {
					t.Fatal("准备候选改变原选区或扩展许可")
				}
			}
			bad := specs[0]
			bad.ExtentRows++
			if _, err := PrepareSurfaceCandidate(ctx, input, bad); err == nil {
				t.Fatal("接受了超过有界窗口的规格")
			}
			input.request.SurfaceLayout.ExpandAdjacent = false
			if _, err := EnumerateSurfaceCandidates(ctx, input, options); err == nil {
				t.Fatal("未授权请求可枚举跨带候选")
			}
			if _, err := PrepareSurfaceCandidate(ctx, input, specs[0]); err == nil {
				t.Fatal("未授权请求接受了跨带规格")
			}
		})
	}
}

// TestDenseAdjacentBoundedPlanValidation 职责：核对扩带后的范围、原生段数和原选区锚点仍由原终检拒绝非法结果；使用：合成区域元数据的定向验证；边界：不宣称实体物理可行。
func TestDenseAdjacentBoundedPlanValidation(t *testing.T) {
	for _, tc := range []struct {
		zone          string
		start, height int
	}{
		{"equator", -128, 257},
		{"equator-south", -78, 207},
		{"equator-north", -128, 207},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			layout := &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: tc.zone, ExpandAdjacent: true, polarRoot: true}
			plan := SurfacePlan{AnchorLatitudeGrid: tc.start, LayoutHeight: tc.height}
			for _, native := range surfaceLatitudeBands(200) {
				low, high := max(tc.start, native.low), min(tc.start+tc.height-1, native.high)
				if low <= high {
					plan.Bands = append(plan.Bands, SurfaceBandPlan{MinLatitudeGrid: low, MaxLatitudeGrid: high, LongitudeSegments: native.segments})
				}
			}
			if err := validateDensityLatitudePlan(layout, &plan); err != nil {
				t.Fatalf("保持原参考点的合法跨带范围被拒绝：%v", err)
			}
			layout.ExpandAdjacent = false
			if err := validateDensityLatitudePlan(layout, &plan); err == nil {
				t.Fatal("严格选区接受跨带结果")
			}
			layout.ExpandAdjacent = true
			plan.Bands[0].LongitudeSegments++
			if err := validateDensityLatitudePlan(layout, &plan); err == nil {
				t.Fatal("接受错误原生段数")
			}
			plan.Bands[0].LongitudeSegments--
			selection, _ := densityLatitudeSelectionFor(tc.zone)
			allowed := denseLatitudeAllowedBands(selection, true)
			firstRow := plan.Bands[0].MinLatitudeGrid
			plan.Bands[0].MinLatitudeGrid = allowed[0].low - 1
			if err := validateDensityLatitudePlan(layout, &plan); err == nil {
				t.Fatal("接受超出有界邻带范围的结果")
			}
			plan.Bands[0].MinLatitudeGrid = firstRow
			if selection.position == "north" {
				plan.AnchorLatitudeGrid -= 10
			} else {
				plan.AnchorLatitudeGrid += 10
			}
			if err := validateDensityLatitudePlan(layout, &plan); err == nil {
				t.Fatal("接受偏离原中心或原边缘的锚点")
			}
		})
	}
}

// TestDenseAdjacentBoundedStaticPolar 职责：确认相邻带策略不放宽静态极区参数；使用：方案配置边界回归；边界：保留6..112，独立于非极区大窗口。
func TestDenseAdjacentBoundedStaticPolar(t *testing.T) {
	for _, rows := range []int{6, 112} {
		options := DefaultSurfaceSearchOptions()
		options.PolarWindowRows = []int{rows}
		if err := ValidateSurfaceSearchOptions(options); err != nil {
			t.Fatalf("合法极区参数%d被拒绝：%v", rows, err)
		}
	}
	for _, rows := range []int{0, 5, 113, 157, 207, 257, 501} {
		options := DefaultSurfaceSearchOptions()
		options.PolarWindowRows = []int{rows}
		if err := ValidateSurfaceSearchOptions(options); err == nil {
			t.Fatalf("非法极区参数%d被接受", rows)
		}
	}
}
