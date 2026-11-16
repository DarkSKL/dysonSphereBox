package build

// 文件职责：差分验收站点具名策略对旧候选语义的完整保留。
// 使用：临时站点重构验证；边界：冻结旧编码仅用于测试，主线统一删除测试文件。

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// stationBaselineNamedSpec 职责：把冻结基线的实际字段投影到新契约；使用：独立预期结果；边界：丢弃旧编号，不调用生产解析函数。
func stationBaselineNamedSpec(t *testing.T, old baselineStationCandidateSpec) StationCandidateSpec {
	t.Helper()
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var spec StationCandidateSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if old.Kind == "field" {
		spec.FieldPacking = StationFieldPacking{Staggered: old.FieldVariant >= 2, NarrowColumns: old.FieldVariant%2 == 1}
	}
	return spec
}

// stationBaselineCandidates 职责：重放冻结旧枚举并投影实际规格；使用：完整候选顺序与统计对照；边界：不使用新计划和新归一化推导预期。
func stationBaselineCandidates(t *testing.T, ctx context.Context, input *PreparedInput, options StationSearchOptions) []StationCandidateSpec {
	t.Helper()
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := input.request
	variants, fields := []int{0, 1, 2, 3}, 0
	if req.Density == densityDense {
		prepared, err := prepareStationLayoutInput(ctx, req, input.dataset, b.setting, input.graph)
		if err != nil {
			t.Fatal(err)
		}
		variants, fields, err = b.baselineEnumerateDenseStationVariants(ctx, req, input.dataset, prepared)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		for variant := 0; variant < 4; variant++ {
			variants = append(variants, baselineIndependentStationVariantOffset+variant)
		}
	}
	variants, err = b.baselineStationCandidateVariants(ctx, req, input.dataset, "station-"+req.Density, variants)
	if err != nil {
		t.Fatal(err)
	}
	result := []StationCandidateSpec{}
	if options.Family != "field" {
		for _, variant := range variants {
			old, err := b.baselineStationSpecForVariant(ctx, req, input.dataset, variant)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, stationBaselineNamedSpec(t, old))
		}
	}
	if options.Family != "blackbox" {
		for index := 0; index < fields; index++ {
			span := 24.
			if index%4%2 == 1 {
				span = 32
			}
			result = append(result, stationBaselineNamedSpec(t, baselineStationCandidateSpec{Kind: "field", FieldVariant: index % 4, Street: index >= 4, MaxSpan: span}))
		}
	}
	for index := range result {
		result[index].InputFingerprint, result[index].Density, result[index].StationID = input.fingerprint, req.Density, 2100+req.Mode
	}
	return result
}

// stationTestInput 职责：以真实量化计划构造独立枚举输入；使用：策略差分测试；边界：不宣称此辅助器签发完整输入工作流的规则证明。
func stationTestInput(t *testing.T, dataset *quant.Dataset, plan quant.Result, density string, mode int, buffer, monitor bool) *PreparedInput {
	t.Helper()
	setting, err := normalizeSetting(Setting{BeltLv: 3, InserterLv: 4, MaxMachineInALine: 15, MaxLays: 3, BlockWidthLimit: 50, MergeType: 2, EnableIntermediateBuffer: buffer, EnableFlowMonitor: monitor})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := newAssemblyPolicy(mode, density)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := quant.BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	return &PreparedInput{request: Request{Plan: plan, Mode: mode, Density: density, Setting: setting}, dataset: dataset, graph: &graph, policy: policy, fingerprint: "station-baseline-test"}
}

// TestStationStrategyBaseline 职责：逐项比较普通/密铺、两种塔及缓存/监测/纬带组合；使用：重构差分验收；边界：保留完整顺序、所有几何字段和观察计数，不对差异排序或放宽比较。
func TestStationStrategyBaseline(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	cases, total := 0, 0
	coverage := map[string]bool{}
	for _, name := range []string{"铁块", "电磁矩阵", "硫酸", "精炼油", "石墨烯", "卡西米尔晶体"} {
		item, ok := dataset.ByName[name]
		if !ok {
			t.Fatalf("缺少真实物品 %s", name)
		}
		for _, rate := range []float64{6, 6000} {
			plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: item.ID, RatePerMinute: rate}}})
			if err != nil {
				t.Fatal(err)
			}
			for _, density := range []string{densityStandard, densityDense} {
				for flags := 0; flags < 8; flags++ {
					mode := 3 + flags%2
					for _, region := range []string{"", "equator", "ring3-north-center", "ring4-north-center", "ring5-north-center"} {
						label := fmt.Sprintf("%s/%g/%s/%d/%s", name, rate, density, flags, region)
						t.Run(label, func(t *testing.T) {
							input := stationTestInput(t, dataset, plan, density, mode, flags&2 != 0, flags&4 != 0)
							if region != "" {
								input.request.SurfaceLayout = &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: region}
								if density == densityDense {
									// 原生资格由显式球面规格恢复；仅设置请求内部标志不会进入均衡、地面轨和同侧炼油分支。
									surfaces, err := EnumerateSurfaceCandidates(context.Background(), input, DefaultSurfaceSearchOptions())
									if err != nil || len(surfaces) == 0 {
										t.Fatalf("原生球面枚举失败: %v", err)
									}
									input, err = PrepareSurfaceCandidate(context.Background(), input, surfaces[0])
									if err != nil {
										t.Fatal(err)
									}
								}
							}
							ctx := input.algorithmContext(context.Background())
							oldRegistry, newRegistry := newCandidateRegistry(), newCandidateRegistry()
							oldCtx := context.WithValue(ctx, candidateRegistryKey{}, oldRegistry)
							newCtx := context.WithValue(ctx, candidateRegistryKey{}, newRegistry)
							want := stationBaselineCandidates(t, oldCtx, input, StationSearchOptions{})
							got, err := EnumerateStationCandidates(newCtx, input, StationSearchOptions{})
							if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, want) {
								for index := 0; index < min(len(got), len(want)); index++ {
									if got[index] != want[index] {
										t.Fatalf("第%d项不同\ngot  %+v\nwant %+v", index, got[index], want[index])
									}
								}
								t.Fatalf("候选数量 got=%d want=%d", len(got), len(want))
							}
							if !reflect.DeepEqual(oldRegistry.snapshot(), newRegistry.snapshot()) {
								t.Fatalf("候选统计不同: got=%+v want=%+v", newRegistry.snapshot(), oldRegistry.snapshot())
							}
							for _, spec := range got {
								for key, enabled := range map[string]bool{"chemical": spec.ChemicalFour, "refinery": spec.RefineryCollinear, "shared-input": spec.SharedInputs, "shared-output": spec.SharedOutputs, "balanced": spec.BalancedFrame, "ground": spec.GroundTracks, "paired": spec.PairedOutputs, "producer": spec.ProducerSplits, "independent": spec.IndependentStations, "hole": spec.HoleSpan == 20, "bent": spec.BentRows, "field": spec.Kind == "field", "street": spec.Street} {
									coverage[key] = coverage[key] || enabled
								}
							}
							cases++
							total += len(got)
						})
					}
				}
			}
		}
	}
	for _, key := range []string{"chemical", "refinery", "shared-input", "shared-output", "balanced", "ground", "paired", "producer", "independent", "hole", "bent", "field", "street"} {
		if !coverage[key] {
			t.Errorf("未覆盖策略 %s", key)
		}
	}
	t.Logf("完整差分: %d 个请求组合，%d 个顺序候选，策略覆盖=%v", cases, total, coverage)
}

// TestStationRefinementContract 职责：验证获选策略按值精修和原评分阈值；使用：保持就近塔口语义；边界：拒绝塔场或重复精修，不能以面积或塔数退步换线长。
func TestStationRefinementContract(t *testing.T) {
	base := StationCandidateSpec{Kind: "blackbox", MaxSpan: 24, SharedInputs: true, FieldPacking: StationFieldPacking{Staggered: true}}
	refined, err := RefineStationCandidate(base)
	if err != nil {
		t.Fatal(err)
	}
	want := base
	want.NearbyInputPorts = true
	if refined != want || base.NearbyInputPorts {
		t.Fatal("精修未按值保留完整策略")
	}
	for _, invalid := range []StationCandidateSpec{refined, {Kind: "field"}} {
		if _, err := RefineStationCandidate(invalid); err == nil {
			t.Fatal("非法精修未拒绝")
		}
	}
	metrics := LayoutMetrics{EnvelopeArea: 100, TotalBeltLengthMeters: 100}
	for _, test := range []struct {
		towers       int
		area, length float64
		want         bool
	}{{2, 100, 99, true}, {2, 99, 99, true}, {2, 101, 99, false}, {1, 100, 99, false}, {3, 100, 99, false}, {2, 100, 100, false}} {
		if got := BetterStationRefinement(test.towers, LayoutMetrics{EnvelopeArea: test.area, TotalBeltLengthMeters: test.length}, 2, metrics); got != test.want {
			t.Fatalf("精修判据错误: %+v", test)
		}
	}
}

// TestStationCandidateAssembly 职责：实际装配普通/密铺及两种塔；使用：确认具名规格贯穿实体入口；边界：此处只检查装配快照，不把装配成功宣称为完整终检或游戏验收。
func TestStationCandidateAssembly(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: dataset.ByName["铁块"].ID, RatePerMinute: 6}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, density := range []string{densityStandard, densityDense} {
		for _, mode := range []int{3, 4} {
			t.Run(fmt.Sprintf("%s/%d", density, mode), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				input := stationTestInput(t, dataset, plan, density, mode, false, false)
				specs, err := EnumerateStationCandidates(ctx, input, StationSearchOptions{Family: "blackbox"})
				if err != nil || len(specs) == 0 {
					t.Fatalf("枚举失败: %v", err)
				}
				snapshot, err := AssembleStationCandidate(ctx, input, specs[0])
				if err != nil {
					t.Fatal(err)
				}
				if snapshot.data == nil || snapshot.data.evidence == nil {
					t.Fatal("缺少实际装配快照")
				}
				bad := specs[0]
				bad.StationID = 2107
				if _, err := AssembleStationCandidate(ctx, input, bad); err == nil {
					t.Fatal("未拒绝跨塔事实规格")
				}
			})
		}
	}
}
