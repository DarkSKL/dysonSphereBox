package build

// 文件职责：临时差分验收直连候选的策略覆盖、稳定顺序和可证明的归一化等价。
// 使用：与冻结旧实现对照；边界：不跳过实体规则，主线完成总验收后统一清理。

import (
	"context"
	"math/rand"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/blueprint/build/planning/connected"
	"DysonSphereBox/helper/quant"
)

// referenceConnectedCandidates 职责：重放重构前完整枚举顺序；使用：差分测试；边界：仅在测试中保留旧展开，不供生产调用。
func referenceConnectedCandidates(req Request, limits []float64, options ConnectedSearchOptions, paired bool, keys, capacityKeys map[int]string) []ConnectedCandidateSpec {
	variants := referenceConnectedBase(req, limits, options)
	variants = referenceConnectedExpand(req, limits, variants, options)
	if req.Density == densityDense {
		variants, _ = referenceConnectedStreetUnique(variants, keys, capacityKeys)
		if options.PairedOutputs && paired {
			for _, variant := range append([]referenceConnectedVariant{}, variants...) {
				if variant.street && variant.streetGroupLimit == 0 && !variant.capacityFill {
					variant.pairedOutputs = true
					variants = append(variants, variant)
				}
			}
		}
		if options.TranslateCompact {
			for _, variant := range append([]referenceConnectedVariant{}, variants...) {
				if variant.Compact && !variant.street && !variant.refine && !variant.capacityFill && !variant.producerSplits && !variant.bentRows {
					variant.translateCompact = true
					variants = append(variants, variant)
				}
			}
		}
	}
	result := make([]ConnectedCandidateSpec, 0, len(variants))
	for _, variant := range variants {
		result = append(result, referenceConnectedSpec(req, variant, false))
	}
	return candidates.StableUnique(result, func(spec ConnectedCandidateSpec) ConnectedCandidateSpec { return spec })
}

// connectedTestRequest 职责：提供纯枚举所需最小资格输入；使用：候选差分；边界：不冒充通过输入准备和实体规则的完整生产请求。
func connectedTestRequest(density, mode string) Request {
	req := Request{Mode: 1, Density: density, Plan: quant.Result{Productions: []quant.Production{{MachineCount: 12, MachineType: 1}}}}
	if mode != "" {
		req.SurfaceLayout = &SurfaceLayout{Mode: mode}
	}
	return req
}

// TestConnectedLayoutConstruction 职责：穷举替换优先级与短高度列表；使用：直接构造替代回写后；边界：完整布局种子顺序必须与冻结基线相同。
func TestConnectedLayoutConstruction(t *testing.T) {
	options := DefaultConnectedSearchOptions().Layout
	for size := 0; size <= 6; size++ {
		limits := []float64{10, 100, 25, 60, 9, 200}[:size]
		for flags := 0; flags < 4; flags++ {
			options.Horizontal, options.GlobalPower = flags&1 != 0, flags&2 != 0
			for _, mode := range []connected.SurfaceMode{connected.SurfacePlanar, connected.SurfaceAutoTropic} {
				actual := connected.Variants(mode, limits, options)
				want := referenceConnectedLayouts(mode, limits, options)
				if !reflect.DeepEqual(actual, want) {
					t.Fatalf("size=%d flags=%d mode=%s: got %+v want %+v", size, flags, mode, actual, want)
				}
			}
		}
	}
}

// TestConnectedCandidateCoverage 职责：固定种子差分覆盖默认策略及不同高度；使用：验证归一化后无策略损失和顺序变化；边界：只将明确证明的无效字段和实际容量键归一。
func TestConnectedCandidateCoverage(t *testing.T) {
	random := rand.New(rand.NewSource(20261116))
	for round := 0; round < 150; round++ {
		options := DefaultConnectedSearchOptions()
		if round%3 == 0 {
			options.DenseSpans = []float64{12}
		}
		limits := make([]float64, 1+random.Intn(6))
		for i := range limits {
			limits[i] = float64(8 + random.Intn(100))
		}
		for _, density := range []string{"standard", densityDense} {
			for _, mode := range []string{"", "planar", "autoTropic", "tropic"} {
				req := connectedTestRequest(density, mode)
				for _, keys := range []map[int]string{nil, {0: "single", 2: "single", -1: "single"}, {0: "auto", 2: "two", -1: "all"}, {0: "", 2: "", -1: ""}} {
					old := referenceConnectedCandidates(req, limits, options, true, keys, keys)
					want := deduplicateConnectedCandidates(context.Background(), req, old, keys, keys)
					planned := planConnectedCandidates(req, limits, options, true)
					actual := deduplicateConnectedCandidates(context.Background(), req, planned, keys, keys)
					if !reflect.DeepEqual(actual, want) {
						t.Fatalf("round=%d density=%s mode=%s limits=%v keys=%v: candidate sequence differs (%d vs %d)", round, density, mode, limits, keys, len(actual), len(want))
					}
				}
			}
		}
	}
}

// TestConnectedDistinctStrategies 职责：逐字段证明真实策略不能误合并；使用：保护完整规格身份；边界：Compact与Horizontal密铺无效字段另测。
func TestConnectedDistinctStrategies(t *testing.T) {
	req := connectedTestRequest(densityDense, "planar")
	base := ConnectedCandidateSpec{Limit: 20, Street: true, DenseSpan: 12}
	variants := []ConnectedCandidateSpec{base}
	for _, change := range []func(*ConnectedCandidateSpec){
		func(s *ConnectedCandidateSpec) { s.Limit++ },
		func(s *ConnectedCandidateSpec) { s.DenseOrder = 1 },
		func(s *ConnectedCandidateSpec) { s.DenseOrder = 2 },
		func(s *ConnectedCandidateSpec) { s.DenseOrder = 3 },
		func(s *ConnectedCandidateSpec) { s.DenseSpan++ },
		func(s *ConnectedCandidateSpec) { s.Refine = true },
		func(s *ConnectedCandidateSpec) { s.StreetRotation = 1 },
		func(s *ConnectedCandidateSpec) { s.StreetGroupLimit = 2 },
		func(s *ConnectedCandidateSpec) { s.PairedOutputs = true },
		func(s *ConnectedCandidateSpec) { s.CapacityFill = true },
		func(s *ConnectedCandidateSpec) { s.ProducerSplits = true },
		func(s *ConnectedCandidateSpec) { s.SplitterModel = 38 },
		func(s *ConnectedCandidateSpec) { s.SplitterModel = 39 },
		func(s *ConnectedCandidateSpec) { s.SplitterModel = 40 },
		func(s *ConnectedCandidateSpec) { s.BentRows = true },
		func(s *ConnectedCandidateSpec) { s.TranslateCompact = true },
		func(s *ConnectedCandidateSpec) { s.MaterialNetworks = true },
		func(s *ConnectedCandidateSpec) { s.LegacyInputs = true },
		func(s *ConnectedCandidateSpec) { s.ObjectiveBound.Enabled = true },
	} {
		spec := base
		change(&spec)
		variants = append(variants, spec)
	}
	if actual := deduplicateConnectedCandidates(context.Background(), req, variants, nil, nil); len(actual) != len(variants) {
		t.Fatalf("lost real strategy: %d vs %d", len(actual), len(variants))
	}
	for _, mode := range []string{"", "autoTropic"} {
		standard := connectedTestRequest("standard", mode)
		variants := []ConnectedCandidateSpec{{Limit: 20}, {Limit: 20, Compact: true}, {Limit: 20, Compact: true, Horizontal: true}, {Limit: 20, GlobalPower: true}}
		want := 3
		if mode == "autoTropic" {
			want = 4
		}
		if actual := deduplicateConnectedCandidates(context.Background(), standard, variants, nil, nil); len(actual) != want {
			t.Fatalf("standard mode=%s: got %d want %d", mode, len(actual), want)
		}
	}
}

// TestConnectedStreetKeysAndDerivation 职责：证明双侧和移近策略在等价组提前合并时不会丢失；使用：完整派生后统一去重；边界：跨度不匹配探针时保留分组差异。
func TestConnectedStreetKeysAndDerivation(t *testing.T) {
	req := connectedTestRequest(densityDense, "planar")
	options := DefaultConnectedSearchOptions()
	options.StreetGroups = []int{2, 0, -1}
	keys := map[int]string{0: "same", 2: "same", -1: "same"}
	actual := deduplicateConnectedCandidates(context.Background(), req, planConnectedCandidates(req, []float64{20, 16, 25, 60}, options, true), keys, keys)
	paired, translated := 0, 0
	for _, spec := range actual {
		if spec.PairedOutputs {
			paired++
		}
		if spec.TranslateCompact {
			translated++
		}
	}
	if paired != 4 || translated != 4 {
		t.Fatalf("paired=%d translated=%d", paired, translated)
	}
	base := ConnectedCandidateSpec{Limit: 20, Street: true}
	other := base
	other.StreetGroupLimit = 2
	for _, entry := range []struct {
		keys map[int]string
		span float64
		want int
	}{
		{keys, 0, 1}, {nil, 0, 2}, {map[int]string{0: ""}, 0, 1}, {keys, 12, 2},
	} {
		base.DenseSpan, other.DenseSpan = entry.span, entry.span
		if got := deduplicateConnectedCandidates(context.Background(), req, []ConnectedCandidateSpec{base, other}, entry.keys, nil); len(got) != entry.want {
			t.Fatalf("keys=%v span=%v got=%d want=%d", entry.keys, entry.span, len(got), entry.want)
		}
	}
}

// TestConnectedCandidateCounts 职责：记录默认及重复配置的实际候选数量前后；使用：性能与覆盖证据；边界：不为减少数量而删除独立策略。
func TestConnectedCandidateCounts(t *testing.T) {
	for _, density := range []string{"standard", densityDense} {
		for _, mode := range []string{"planar", "autoTropic"} {
			for _, duplicate := range []bool{false, true} {
				req := connectedTestRequest(density, mode)
				options := DefaultConnectedSearchOptions()
				if duplicate {
					options.DenseSpans = []float64{12}
				}
				keys := map[int]string{0: "same", 2: "same", -1: "same"}
				limits := []float64{20, 16, 25, 60}
				old := referenceConnectedCandidates(req, limits, options, true, keys, keys)
				planned := planConnectedCandidates(req, limits, options, true)
				actual := deduplicateConnectedCandidates(context.Background(), req, planned, keys, keys)
				t.Logf("density=%s surface=%s duplicate=%t per-group old=%d generated=%d final=%d; two-groups %d -> %d", density, mode, duplicate, len(old), len(planned), len(actual), len(old)*2, len(actual)*2)
				if len(actual) > len(old) {
					t.Fatal("unexpected growth")
				}
			}
		}
	}
}

// BenchmarkConnectedCandidatePlanning 职责：量测相同候选准备工作的时间及分配；使用：冻结旧实现与当前实现对照；边界：不包含实体求解，不据此宣称整厂生成加速。
func BenchmarkConnectedCandidatePlanning(b *testing.B) {
	req := connectedTestRequest(densityDense, "planar")
	limits := []float64{20, 16, 25, 60}
	options := DefaultConnectedSearchOptions()
	keys := map[int]string{0: "same", 2: "same", -1: "same"}
	b.Run("before", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			referenceConnectedCandidates(req, limits, options, true, keys, keys)
		}
	})
	b.Run("after", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			planned := planConnectedCandidates(req, limits, options, true)
			deduplicateConnectedCandidates(context.Background(), req, planned, keys, keys)
		}
	})
}
