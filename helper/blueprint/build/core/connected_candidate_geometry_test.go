package build

// 文件职责：用真实输入、装配及完整规则链验证候选归一化等价。
// 使用：直连重构临时行为验收；边界：不修改实体规则，不使用旧报告冒充当前证据，主线最终清理。

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// connectedPreparedTestInput 职责：逐项执行生产输入阶段；使用：真实候选行为测试；边界：任何前置规则失败立即失败，不手造已验证产物。
func connectedPreparedTestInput(t *testing.T, ctx context.Context, item int, rate float64, density string) *PreparedInput {
	t.Helper()
	plan, err := quant.Solve(ctx, quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item, RatePerMinute: rate}}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := NormalizeInputRequest(ctx, Request{Plan: plan, Mode: 1, Density: density})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := LoadInputDatasetFacts(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	source, err := ValidateInputPlanSource(ctx, request, facts)
	if err != nil {
		t.Fatal(err)
	}
	production, err := NormalizeInputProductions(ctx, request, facts, source)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := BuildInputFlowFacts(ctx, production)
	if err != nil {
		t.Fatal(err)
	}
	capacity, err := ValidateInputProductionCapacity(ctx, production)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := PlanMinimumInputTransport(ctx, production, capacity)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := ValidateInputTransportCapacity(ctx, transport)
	if err != nil {
		t.Fatal(err)
	}
	workload, err := EstimateInputWorkload(ctx, transport)
	if err != nil {
		t.Fatal(err)
	}
	input, err := ComposePreparedInput(ctx, flow, capacity, transport, proof, workload)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

// connectedVerifiedTestCandidate 职责：逐步定位并完成全部最终规则；使用：候选行为差分；边界：无省略或放宽规则，无隐式重试。
func connectedVerifiedTestCandidate(ctx context.Context, input *PreparedInput, spec ConnectedCandidateSpec) (VerifiedSnapshot, error) {
	assembly, err := AssembleConnectedCandidate(ctx, input, spec)
	if err != nil {
		return VerifiedSnapshot{}, err
	}
	draft, err := PositionGeometry(ctx, assembly)
	if err != nil {
		return VerifiedSnapshot{}, err
	}
	for _, step := range []func(context.Context, GeometryDraft) (GeometryDraft, error){RepairGeometryPower, PruneGeometryPower, RelocateGeometry, AlignGeometryMonitors, BindGeometryCargo, ComputeGeometryRange, BindGeometryTransport, CommitGeometryPriorities, AnnotateGeometryRates} {
		draft, err = step(ctx, draft)
		if err != nil {
			return VerifiedSnapshot{}, err
		}
	}
	geometry, err := FreezeGeometry(ctx, draft)
	if err != nil {
		return VerifiedSnapshot{}, err
	}
	proofs := make([]RuleProof, 0, len(RequiredRuleKinds()))
	byKind := map[RuleKind]RuleProof{}
	for _, kind := range RequiredRuleKinds() {
		parents := []RuleProof{}
		for _, dependency := range RuleDependencies(kind) {
			parents = append(parents, byKind[dependency])
		}
		proof, err := CheckRule(ctx, geometry, kind, parents...)
		if err != nil {
			return VerifiedSnapshot{}, fmt.Errorf("%s: %w", kind, err)
		}
		proofs, byKind[kind] = append(proofs, proof), proof
	}
	return CompleteValidation(ctx, geometry, proofs...)
}

// TestConnectedNormalizationGeometry 职责：证明密铺Compact和Horizontal四种组合编码前实体完全相同；使用：真实装配与完整规则链；边界：保留原始规格直接求解，不能由测试先归一掩盖差异。
func TestConnectedNormalizationGeometry(t *testing.T) {
	for _, item := range []int{1101, 1201} {
		t.Run(fmt.Sprint(item), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			input := connectedPreparedTestInput(t, ctx, item, 60, densityDense)
			base := ConnectedCandidateSpec{Limit: 30, DenseSpan: 12}
			verified, err := connectedVerifiedTestCandidate(ctx, input, base)
			if err != nil {
				t.Fatal(err)
			}
			for flags := 1; flags < 4; flags++ {
				spec := base
				spec.Compact, spec.Horizontal = flags&1 != 0, flags&2 != 0
				other, err := connectedVerifiedTestCandidate(ctx, input, spec)
				if err != nil {
					t.Fatalf("flags=%d: %v", flags, err)
				}
				if !reflect.DeepEqual(verified.Geometry().Document(), other.Geometry().Document()) {
					t.Fatalf("flags=%d changed geometry", flags)
				}
				if !reflect.DeepEqual(verified.Validation().Logistics(), other.Validation().Logistics()) {
					t.Fatalf("flags=%d changed logistics", flags)
				}
			}
			t.Logf("item=%d: four raw parameter combinations, identical geometry/logistics; %d mandatory rules each", item, len(RequiredRuleKinds()))
		})
	}
}

// TestConnectedProbeReuse 职责：核对共用容量行后的街道键和双侧资格与旧探针完全相同；使用：真实生产图；边界：均衡和容量优先分别比较，不跨策略复用行。
func TestConnectedProbeReuse(t *testing.T) {
	for _, item := range []int{1101, 1201, 6001} {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		input := connectedPreparedTestInput(t, ctx, item, 600, densityDense)
		assembly, err := input.newAssembly(ctx, nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		for _, fill := range []bool{false, true} {
			keys, paired, err := assembly.denseConnectedCandidateFacts(ctx, input.request, input.dataset, fill)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			oldKeys, err := assembly.referenceConnectedPlanKeys(ctx, input.request, input.dataset, fill)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			if !reflect.DeepEqual(keys, oldKeys) {
				cancel()
				t.Fatalf("item=%d fill=%t keys changed", item, fill)
			}
			if !fill {
				oldPaired, err := assembly.referenceConnectedPairedGroups(ctx, input.request, input.dataset)
				if err != nil || paired != oldPaired {
					cancel()
					t.Fatalf("item=%d paired changed: %v", item, err)
				}
			}
		}
		cancel()
	}
}

// TestConnectedEnumerationIntegration 职责：用真实设备和容量键验证公开枚举两组规格、数量及取消行为；使用：接口级差分；边界：显式固定高度，避免把主线高度修复混入本子任务证据。
func TestConnectedEnumerationIntegration(t *testing.T) {
	for _, item := range []int{1101, 1201, 6001} {
		for _, density := range []string{"standard", densityDense} {
			t.Run(fmt.Sprintf("%d/%s", item, density), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				input := connectedPreparedTestInput(t, ctx, item, 600, density)
				options := DefaultConnectedSearchOptions()
				options.HeightLimits, options.MeasureHeightLimits = []float64{20, 16, 25, 60}, false
				assembly, err := input.newAssembly(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				var keys, capacityKeys map[int]string
				paired := false
				if density == densityDense {
					keys, err = assembly.referenceConnectedPlanKeys(ctx, input.request, input.dataset)
					if err != nil {
						t.Fatal(err)
					}
					capacityKeys, err = assembly.referenceConnectedPlanKeys(ctx, input.request, input.dataset, true)
					if err != nil {
						t.Fatal(err)
					}
					paired, err = assembly.referenceConnectedPairedGroups(ctx, input.request, input.dataset)
					if err != nil {
						t.Fatal(err)
					}
				}
				old := referenceConnectedCandidates(input.request, options.HeightLimits, options, paired, keys, capacityKeys)
				canonical := deduplicateConnectedCandidates(ctx, input.request, old, keys, capacityKeys)
				want := []ConnectedCandidateSpec{}
				for _, legacy := range []bool{false, true} {
					for _, spec := range canonical {
						spec.LegacyInputs = legacy
						want = append(want, spec)
					}
				}
				actual, err := EnumerateConnectedCandidates(ctx, input, options)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, want) {
					assertConnectedRowCoverage(t, ctx, input, actual, want)
				}
				t.Logf("actual capacities: two-groups %d -> %d, paired=%t", len(old)*2, len(actual), paired)
				cancelled, stop := context.WithCancel(ctx)
				stop()
				if _, err := EnumerateConnectedCandidates(cancelled, input, options); err != context.Canceled {
					t.Fatalf("cancel cause lost: %v", err)
				}
			})
		}
	}
}
