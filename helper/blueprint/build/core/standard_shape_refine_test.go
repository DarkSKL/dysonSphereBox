package build

// 文件职责：从原始普通请求诊断有限装箱策略，保留全部几何和规则检查。
// 使用：STANDARD_SHAPE_PROBE指定真实请求；边界：临时诊断，不改用户请求，不将装配成功当成形状成功。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	distributor "DysonSphereBox/helper/blueprint/build/planning/distributor"
	"DysonSphereBox/helper/quant"
)

// TestStandardShapeRefinePreservesCandidates 职责：证明新参数只追加普通候选；使用：生产默认变更回归；边界：保持原规格顺序和参数，不按比较结果弱化对照。
func TestStandardShapeRefinePreservesCandidates(t *testing.T) {
	ctx := context.Background()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := quant.Solve(ctx, quant.Request{Targets: []quant.Target{{ItemID: dataset.ByName["齿轮"].ID, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{2, 3, 4} {
		input := stationTestInput(t, dataset, plan, densityStandard, mode, true, true)
		if mode == 2 {
			options := DefaultDistributorSearchOptions()
			old, e := EnumerateDistributorCandidates(ctx, input, options)
			if e != nil {
				t.Fatal(e)
			}
			options.StandardRowSpans = []float64{8, 12}
			options.StandardRowPacking = true
			options.StandardRows = []distributor.FieldStrategy{{Scale: 1.2, Order: 0}, {Scale: 1.2, Order: 2}}
			added, e := EnumerateDistributorCandidates(ctx, input, options)
			if e != nil {
				t.Fatal(e)
			}
			if len(added) <= len(old) {
				t.Fatal("配送没有追加候选")
			}
			for i := range old {
				before, _ := json.Marshal(old[i])
				after, _ := json.Marshal(added[i])
				if string(before) != string(after) {
					t.Fatalf("配送原候选%d变更", i)
				}
			}
		} else {
			old, e := EnumerateStationCandidates(ctx, input, DefaultStationSearchOptions())
			if e != nil {
				t.Fatal(e)
			}
			added, e := EnumerateStationCandidates(ctx, input, StationSearchOptions{StandardRowSpans: []float64{8, 12}, StandardRowPacking: true})
			if e != nil {
				t.Fatal(e)
			}
			if len(added) <= len(old) {
				t.Fatalf("mode%d没有追加候选", mode)
			}
			for i := range old {
				if old[i] != added[i] {
					t.Fatalf("mode%d原候选%d变更", mode, i)
				}
			}
		}
	}
}

// TestStandardShapeRefineParameters 职责：防止显式行策略被无效参数、去重或缓存默认吞掉；使用：边界回归；边界：不以参数测试替代真实生成。
func TestStandardShapeRefineParameters(t *testing.T) {
	for _, spans := range [][]float64{{0}, {-1}, {math.NaN()}, {math.Inf(1)}, {10001}, {8, 8}, {4, 8, 12, 16, 24}} {
		if ValidateDistributorSearchOptions(DistributorSearchOptions{StandardRowSpans: spans}) == nil {
			t.Fatalf("配送接受非法跨度%v", spans)
		}
		if ValidateStationSearchOptions(StationSearchOptions{StandardRowSpans: spans}) == nil {
			t.Fatalf("塔接受非法跨度%v", spans)
		}
	}
	base := DistributorCandidateSpec{Topology: DistributorStandardField, Scale: 1.2, Order: 2, RowSpan: 8, RowPacking: true}
	if err := validateDistributorCandidateParameters(base); err != nil {
		t.Fatal(err)
	}
	other := base
	other.RowSpan = 12
	if distributorIdentity(base, Request{}) == distributorIdentity(other, Request{}) {
		t.Fatal("不同普通行跨度被合并")
	}
	other = base
	other.RowPacking = false
	if distributorIdentity(base, Request{}) == distributorIdentity(other, Request{}) {
		t.Fatal("不同普通装箱策略被合并")
	}
	for _, topology := range []DistributorTopology{DistributorDenseBlackbox, DistributorDenseDetached, DistributorInternalMode5} {
		bad := base
		bad.Topology = topology
		if validateDistributorCandidateParameters(bad) == nil {
			t.Fatalf("非普通拓扑%v接受普通行策略", topology)
		}
	}
	b := &builder{setting: Setting{EnableIntermediateBuffer: true}}
	old := b.resolveStationPlan(context.Background(), Request{}, stationRowPlans()[0], nil, false)
	if old.MaxSpan != 32 {
		t.Fatalf("旧缓存默认变更%g", old.MaxSpan)
	}
	for _, plan := range standardStationRowPlans([]float64{8, 12}) {
		spec := b.resolveStationPlan(context.Background(), Request{}, plan, nil, false)
		if spec.MaxSpan != plan.explicitRowSpan {
			t.Fatalf("显式跨度被缓存默认覆盖%g", spec.MaxSpan)
		}
	}
}

// standardShapeInput 职责：逐项完成原始请求准备；使用：真实候选探针；边界：不重算量化和运输设备。
func standardShapeInput(t *testing.T, ctx context.Context, req Request) *PreparedInput {
	t.Helper()
	r, e := NormalizeInputRequest(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	f, e := LoadInputDatasetFacts(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	s, e := ValidateInputPlanSource(ctx, r, f)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NormalizeInputProductions(ctx, r, f, s)
	if e != nil {
		t.Fatal(e)
	}
	g, e := BuildInputFlowFacts(ctx, p)
	if e != nil {
		t.Fatal(e)
	}
	c, e := ValidateInputProductionCapacity(ctx, p)
	if e != nil {
		t.Fatal(e)
	}
	tr, e := PlanMinimumInputTransport(ctx, p, c)
	if e != nil {
		t.Fatal(e)
	}
	pr, e := ValidateInputTransportCapacity(ctx, tr)
	if e != nil {
		t.Fatal(e)
	}
	w, e := EstimateInputWorkload(ctx, tr)
	if e != nil {
		t.Fatal(e)
	}
	i, e := ComposePreparedInput(ctx, g, c, tr, pr, w)
	if e != nil {
		t.Fatal(e)
	}
	return i
}

// standardShapeVerify 职责：完成装配后的完整生产规则链；使用：单候选探针；边界：不省略实体检查、不编码失败结果。
func standardShapeVerify(ctx context.Context, a AssemblySnapshot) (LayoutMetrics, error) {
	d, e := PositionGeometry(ctx, a)
	if e != nil {
		return LayoutMetrics{}, e
	}
	for _, step := range []func(context.Context, GeometryDraft) (GeometryDraft, error){RepairGeometryPower, PruneGeometryPower, RelocateGeometry, AlignGeometryMonitors, BindGeometryCargo, ComputeGeometryRange, BindGeometryTransport, CommitGeometryPriorities, AnnotateGeometryRates} {
		d, e = step(ctx, d)
		if e != nil {
			return LayoutMetrics{}, e
		}
	}
	g, e := FreezeGeometry(ctx, d)
	if e != nil {
		return LayoutMetrics{}, e
	}
	proofs := []RuleProof{}
	byKind := map[RuleKind]RuleProof{}
	for _, k := range RequiredRuleKinds() {
		parents := []RuleProof{}
		for _, dep := range RuleDependencies(k) {
			parents = append(parents, byKind[dep])
		}
		p, err := CheckRule(ctx, g, k, parents...)
		if err != nil {
			return LayoutMetrics{}, fmt.Errorf("%s: %w", k, err)
		}
		proofs = append(proofs, p)
		byKind[k] = p
	}
	_, e = CompleteValidation(ctx, g, proofs...)
	return byKind[RuleLayoutMetrics].Metrics(), e
}

// TestStandardShapeRefineProbe 职责：有限比较真实请求的排序与附件策略；使用：定位生产候选缺口；边界：不构造案例专属生产分支。
func TestStandardShapeRefineProbe(t *testing.T) {
	path := os.Getenv("STANDARD_SHAPE_PROBE")
	if path == "" {
		t.Skip("指定真实请求后执行")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var evidence struct{ Request Request }
	if e = json.Unmarshal(raw, &evidence); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	input := standardShapeInput(t, ctx, evidence.Request)
	surfaces, e := EnumerateSurfaceCandidates(ctx, input, SurfaceSearchOptions{Strategy: "native", ExpandAdjacent: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, surface := range surfaces {
		if surface.ExtentRows < 60 || surface.ExtentRows > 110 || surface.Mirror {
			continue
		}
		in, e := PrepareSurfaceCandidate(ctx, input, surface)
		if e != nil {
			t.Fatal(e)
		}
		for order := 0; order < 4; order++ {
			for variant := 0; variant < 4; variant++ {
				started := time.Now()
				run, cancel := context.WithTimeout(ctx, 20*time.Second)
				var a AssemblySnapshot
				var err error
				if input.request.Mode == 2 {
					spec := DistributorCandidateSpec{Topology: DistributorStandardField, Scale: 1.2, Order: order, RowSpan: 8}
					baseContext := context.WithValue(in.algorithmContext(run), denseBalancedFrameKey{}, false)
					b, actx, er := newDistributorAssembly(baseContext, in, spec)
					if er == nil {
						b.denseUnitOrder = func(c context.Context, u []denseSpatialUnit) ([]denseSpatialUnit, error) {
							return b.standardSpatialRowUnits(c, u, 2)
						}
						er = assembleDistributorSpec(actx, b, in, spec)
						if er == nil {
							a, er = freezeAssembly(b.generationCtx, b, in.request, in.dataset)
						}
					}
					err = er
				} else {
					specs, er := EnumerateStationCandidates(run, in, StationSearchOptions{Family: "blackbox", StandardRowSpans: []float64{[]float64{8, 12, 18, 24}[variant]}})
					if er != nil {
						t.Fatal(er)
					}
					spec := specs[len(specs)-4+order]
					config, er := checkStationInput(in)
					if er != nil {
						t.Fatal(er)
					}
					actx := context.WithValue(in.algorithmContext(run), denseBalancedFrameKey{}, false)
					b, er := in.newAssembly(actx, nil)
					if er == nil {
						b.directBoundaryExtentScale = 1.25
						b.denseUnitOrder = func(c context.Context, u []denseSpatialUnit) ([]denseSpatialUnit, error) {
							return b.standardSpatialRowUnits(c, u, 2)
						}
						er = b.assembleStationBlackbox(actx, in.request, in.dataset, config, spec)
						if er == nil {
							a, er = freezeAssembly(b.generationCtx, b, in.request, in.dataset)
						}
					}
					err = er
				}
				m := LayoutMetrics{}
				if err == nil {
					m, err = standardShapeVerify(run, a)
				}
				cancel()
				t.Logf("window=%d order=%d variant=%d ratio=%.6f area=%.0f elapsed=%s error=%v", surface.ExtentRows, order, variant, m.EnvelopeAspectRatio, m.EnvelopeArea, time.Since(started), err)
			}
		}
	}
}
