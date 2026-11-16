package build

// 文件职责：补充冻结旧站点候选与具名规格之间的回流、多产物及原生上下文差分验收。
// 使用：复用 station_strategy_test.go 的真实量化输入和旧规格投影；边界：仅验证候选语义，不把枚举通过声明为实体终检或游戏验收。

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"DysonSphereBox/helper/quant"
)

// stationRefactorSolve 职责：按真实物品和可选配方生成量化计划；使用：构造回流与多目标验收数据；边界：不修改求解结果、设备数量或数据集缓存。
func stationRefactorSolve(t *testing.T, dataset *quant.Dataset, names []string, rate float64, recipeID int, external []string) quant.Result {
	t.Helper()
	request := quant.Request{Dataset: "Vanilla"}
	for _, name := range names {
		item, ok := dataset.ByName[name]
		if !ok {
			t.Fatalf("真实数据集缺少物品 %s", name)
		}
		request.Targets = append(request.Targets, quant.Target{ItemID: item.ID, RatePerMinute: rate})
	}
	if recipeID != 0 {
		request.Scheme.ItemRecipeChoices = map[string]int{}
		for choice, index := range dataset.Choices[names[0]] {
			if dataset.Recipes[index].ID == recipeID {
				request.Scheme.ItemRecipeChoices[names[0]] = choice + 1
			}
		}
		if request.Scheme.ItemRecipeChoices[names[0]] == 0 {
			t.Fatalf("目标 %s 没有配方 %d", names[0], recipeID)
		}
	}
	request.Settings.MineralizeList = map[string]bool{}
	for _, name := range external {
		request.Settings.MineralizeList[name] = true
	}
	plan, err := quant.Solve(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if recipeID != 0 {
		found := false
		for _, production := range plan.Productions {
			found = found || production.RecipeID == recipeID
		}
		if !found {
			t.Fatalf("真实求解未使用指定配方 %d", recipeID)
		}
	}
	return plan
}

// stationRefactorCompare 职责：核对原始枚举、有效规格顺序、字段和观察计数；使用：补充场景逐项验收；边界：不排序结果、不归一新输出以掩盖差异，不运行实体装配。
func stationRefactorCompare(t *testing.T, input *PreparedInput, coverage map[string]int) []StationCandidateSpec {
	t.Helper()
	ctx := input.algorithmContext(context.Background())
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareStationLayoutInput(ctx, input.request, input.dataset, b.setting, input.graph)
	if err != nil {
		t.Fatal(err)
	}
	oldPlans, oldFields, err := b.baselineEnumerateDenseStationVariants(ctx, input.request, input.dataset, prepared)
	if err != nil {
		t.Fatal(err)
	}
	newPlans, newFields, err := b.enumerateDenseStationPlans(ctx, input.request, input.dataset, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(newPlans) != len(oldPlans) || newFields != oldFields {
		t.Fatalf("原始候选数量或field预算不同: new=%d/%d old=%d/%d", len(newPlans), newFields, len(oldPlans), oldFields)
	}
	recipes := make([]quant.Recipe, len(input.request.Plan.Productions))
	recycle := false
	for index, production := range input.request.Plan.Productions {
		recipes[index], err = datasetRecipeForProduction(input.dataset, production)
		if err != nil {
			t.Fatal(err)
		}
		recycle = recycle || recipeHasMachineRecycle(recipes[index])
	}
	for index, variant := range oldPlans {
		old, err := b.baselineStationSpecForVariant(ctx, input.request, input.dataset, variant)
		if err != nil {
			t.Fatal(err)
		}
		got := b.resolveStationPlan(ctx, input.request, newPlans[index], recipes, recycle)
		want := stationBaselineNamedSpec(t, old)
		if got != want {
			t.Fatalf("原始候选%d（冻结旧编号%d）的实际几何或修饰不同\ngot  %+v\nwant %+v", index, variant, got, want)
		}
		stationRefactorCompareContext(t, ctx, input.request, old, got)
	}
	var all []StationCandidateSpec
	for _, family := range []string{"all", "blackbox", "field"} {
		oldRegistry, newRegistry := newCandidateRegistry(), newCandidateRegistry()
		options := StationSearchOptions{Family: family}
		want := stationBaselineCandidates(t, context.WithValue(ctx, candidateRegistryKey{}, oldRegistry), input, options)
		got, err := EnumerateStationCandidates(context.WithValue(ctx, candidateRegistryKey{}, newRegistry), input, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("%s有效候选数量不同: got=%d want=%d", family, len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("%s有效候选%d不同\ngot  %+v\nwant %+v", family, index, got[index], want[index])
			}
		}
		if !reflect.DeepEqual(oldRegistry.snapshot(), newRegistry.snapshot()) {
			t.Fatalf("%s观察统计不同: got=%+v want=%+v", family, newRegistry.snapshot(), oldRegistry.snapshot())
		}
		if family == "all" {
			all = got
		}
	}
	for _, spec := range all {
		if spec.Kind == "blackbox" {
			refined, err := RefineStationCandidate(spec)
			if err != nil {
				t.Fatal(err)
			}
			want := spec
			want.NearbyInputPorts = true
			if refined != want || spec.NearbyInputPorts {
				t.Fatal("真实候选精修丢失字段或修改原规格")
			}
		}
		for key, enabled := range map[string]bool{
			"recycle-refine":        recycle && spec.Refine,
			"recycle-span8":         recycle && !input.request.Setting.EnableIntermediateBuffer && spec.MaxSpan == 8,
			"buffer-span32":         recycle && input.request.Setting.EnableIntermediateBuffer && !spec.MaterialOrder && spec.MaxSpan == 32,
			"refinery-and-chemical": spec.RefineryCollinear && spec.ChemicalFour,
			"balanced":              spec.BalancedFrame, "ground": spec.GroundTracks,
			"narrow17":        spec.MaterialOrder && spec.LatitudeSpan == 17,
			"field-street":    spec.Kind == "field" && spec.Street,
			"field-narrow":    spec.Kind == "field" && spec.FieldPacking.NarrowColumns,
			"field-staggered": spec.Kind == "field" && spec.FieldPacking.Staggered,
		} {
			if enabled {
				coverage[key]++
			}
		}
	}
	return all
}

// stationRefactorCompareContext 职责：验证每项修饰落到相同的实际几何上下文；使用：原始候选差分；边界：包括父上下文继承和供电禁用，不以规格字段一致替代生效检查。
func stationRefactorCompareContext(t *testing.T, parent context.Context, request Request, old baselineStationCandidateSpec, spec StationCandidateSpec) {
	t.Helper()
	for _, nearby := range []bool{false, true} {
		old.NearbyInputPorts, spec.NearbyInputPorts = nearby, nearby
		want, got := old.baselineAlgorithmContext(parent, request), spec.algorithmContext(parent, request)
		for _, key := range []any{
			denseGroundTracksKey{}, independentStationPlacementKey{}, denseBentRowsKey{},
			denseProducerSplittersKey{}, denseShortPairedOutputsKey{}, denseSpatialCompactKey{},
			denseStationSharedOutputsKey{}, denseStationSharedInputsKey{}, refineryCollinearKey{},
			chemicalFourKey{}, denseMaterialNetworksKey{}, stationInputAssignmentKey{},
			denseSpatialHolesKey{}, splitterGateModelKey{}, denseBalancedFrameKey{},
			denseUnitPowerRequestedKey{}, densityPrototypeFootprintKey{}, densityNativeBandKey{},
		} {
			if !reflect.DeepEqual(got.Value(key), want.Value(key)) {
				t.Fatalf("修饰上下文 %T 不同: got=%v want=%v", key, got.Value(key), want.Value(key))
			}
		}
	}
}

// TestStationStrategyRefactorSpecialFlows 职责：补足真实回流、多目标、多产物与原生策略组合；使用：在既有960场景基础上做独立差分；边界：强制断言场景实际触发所声称的配方和策略。
func TestStationStrategyRefactorSpecialFlows(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	coverage, cases, candidates := map[string]int{}, 0, 0
	for _, scene := range []struct {
		name     string
		targets  []string
		rate     float64
		recipe   int
		external []string
	}{
		{"reforming", []string{"精炼油"}, 600, 121, []string{"氢", "煤矿"}},
		{"xray", []string{"高能石墨"}, 600, 58, []string{"精炼油"}},
		{"multi-product", []string{"精炼油"}, 600, 16, nil},
		{"multi-target", []string{"铁块", "齿轮", "电路板"}, 600, 0, nil},
		{"large-native", []string{"电磁矩阵"}, 6000, 0, nil},
	} {
		plan := stationRefactorSolve(t, dataset, scene.targets, scene.rate, scene.recipe, scene.external)
		if scene.name == "multi-target" && len(plan.Targets) != 3 {
			t.Fatal("多目标场景未保留三个目标")
		}
		for _, mode := range []int{3, 4} {
			for flags := 0; flags < 4; flags++ {
				for _, surface := range []string{"planar", "strict-highlat", "native-window", "native-band", "native-expand"} {
					t.Run(fmt.Sprintf("%s/%d/%d/%s", scene.name, mode, flags, surface), func(t *testing.T) {
						input := stationTestInput(t, dataset, plan, densityDense, mode, flags&1 != 0, flags&2 != 0)
						power := flags&2 == 0
						input.request.Setting.NeedPower = &power
						if surface != "planar" {
							input.request.SurfaceLayout = &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring3-north-center", ExpandAdjacent: surface == "native-expand"}
						}
						if surface != "planar" && surface != "strict-highlat" {
							options := DefaultSurfaceSearchOptions()
							options.ExpandAdjacent = surface == "native-expand"
							options.PreciseFootprint = true
							surfaces, err := EnumerateSurfaceCandidates(context.Background(), input, options)
							if err != nil {
								t.Fatal(err)
							}
							strategy := "native-window"
							if surface == "native-band" {
								strategy = "native-band"
							}
							found := false
							for _, spec := range surfaces {
								if spec.Strategy == strategy {
									input, err = PrepareSurfaceCandidate(context.Background(), input, spec)
									if err != nil {
										t.Fatal(err)
									}
									found = true
									break
								}
							}
							if !found {
								t.Fatalf("生产入口未枚举出所需球面策略 %s", strategy)
							}
						}
						got := stationRefactorCompare(t, input, coverage)
						if scene.recipe == 121 || scene.recipe == 58 {
							recycle, err := denseBlackboxHasMachineRecycle(plan, dataset)
							if err != nil || !recycle {
								t.Fatalf("真实回流场景不成立: recycle=%v err=%v", recycle, err)
							}
						}
						for _, spec := range got {
							if spec.Kind == "field" && (scene.recipe == 121 || scene.recipe == 58 || input.request.Setting.EnableIntermediateBuffer || input.request.SurfaceLayout != nil && input.request.SurfaceLayout.polarRoot) {
								t.Fatal("未排除不支持回流、缓存或原生根坐标的field候选")
							}
						}
						cases++
						candidates += len(got)
					})
				}
			}
		}
	}
	for _, key := range []string{"recycle-refine", "recycle-span8", "buffer-span32", "refinery-and-chemical", "balanced", "ground", "narrow17", "field-street", "field-narrow", "field-staggered"} {
		if coverage[key] == 0 {
			t.Errorf("补充场景未实际覆盖 %s", key)
		}
	}
	t.Logf("补充差分: %d个真实请求/球面组合，%d个有序有效候选，覆盖=%v", cases, candidates, coverage)
}

// TestStationStrategyRefactorProjectionCoverage 职责：防止新字段被JSON投影静默遗漏；使用：冻结旧规格转新规格的测试契约；边界：旧编号仅在测试内转换，生产代码不得增加兼容桥。
func TestStationStrategyRefactorProjectionCoverage(t *testing.T) {
	old, current := reflect.TypeOf(baselineStationCandidateSpec{}), reflect.TypeOf(StationCandidateSpec{})
	for index := 0; index < current.NumField(); index++ {
		field := current.Field(index)
		if field.Name == "RowPacking" {
			// 本轮新增的普通行阵无旧编号对应；旧策略必须保持关闭，显式新路径由真实生成验收覆盖。
			if stationBaselineNamedSpec(t, baselineStationCandidateSpec{}).RowPacking {
				t.Fatal("旧策略隐式启用了普通行阵")
			}
			continue
		}
		if field.Name == "FieldPacking" {
			for variant := 0; variant < 4; variant++ {
				got := stationBaselineNamedSpec(t, baselineStationCandidateSpec{Kind: "field", FieldVariant: variant})
				want := StationFieldPacking{Staggered: variant >= 2, NarrowColumns: variant%2 == 1}
				if got.FieldPacking != want {
					t.Fatalf("field%d放置转换不同: got=%+v want=%+v", variant, got.FieldPacking, want)
				}
			}
			continue
		}
		previous, exists := old.FieldByName(field.Name)
		if !exists || previous.Type != field.Type || previous.Tag.Get("json") != field.Tag.Get("json") {
			t.Errorf("投影未覆盖具名字段 %s，需要明确预期转换", field.Name)
		}
	}
}
