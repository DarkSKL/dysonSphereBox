package build

// 文件职责：独立用完整运输行、真实装配与规则验证本轮语义去重。
// 使用：Go1.24.11小规模差分及枚举；边界：不改既有测试和基线，本轮完成后由主线统一清理。

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// semanticReferenceIdentity 职责：保存测试独立归约身份；使用：完整运输行参考；边界：不读取生产摘要或街道键。
type semanticReferenceIdentity struct {
	spec      ConnectedCandidateSpec
	execution string
}

// semanticReferenceRows 职责：以完整运输行和明确街道策略构造独立参考；使用：验证安全去重；边界：不调用生产去重键和街道身份帮助函数。
func semanticReferenceRows(t *testing.T, ctx context.Context, input *PreparedInput, spec ConnectedCandidateSpec) (semanticReferenceIdentity, bool) {
	t.Helper()
	id := semanticReferenceIdentity{spec: spec}
	if input.request.Density != densityDense && !spec.SpatialRows && input.geometry.Strategy != "native-window" {
		return id, true
	}
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.denseSpatial, b.denseMaxSpan, b.denseCapacityFill = true, spec.DenseSpan, spec.CapacityFill
	rows, err := b.planTransportRows(ctx, input.request, input.dataset)
	if err != nil {
		t.Fatal(err)
	}
	type rowValue struct{ Owner, Recipe, Machine, Count int }
	type streetValue struct {
		Owner  int
		Counts []int
		Limits []int
	}
	values := []rowValue{}
	streets := []streetValue{}
	byOwner := map[int][]*transportRow{}
	owners := []int{}
	for _, row := range rows {
		values = append(values, rowValue{row.owner, row.recipe.ID, row.info.MachineID, row.count})
		if len(byOwner[row.owner]) == 0 {
			owners = append(owners, row.owner)
		}
		byOwner[row.owner] = append(byOwner[row.owner], row)
	}
	multi := false
	for _, owner := range owners {
		group := byOwner[owner]
		multi = multi || len(group) >= 2
		if !spec.Street || !denseConnectedStreetRowsEligible(group, b.setting) {
			continue
		}
		value := streetValue{Owner: owner}
		for _, row := range group {
			value.Counts = append(value.Counts, row.count)
		}
		if spec.StreetGroupLimit != 0 {
			limit := spec.StreetGroupLimit
			if limit < 0 {
				limit = len(group)
			}
			value.Limits = []int{min(len(group), limit)}
		} else if len(group) == 1 {
			value.Limits = []int{1}
		} else {
			value.Limits = []int{0}
		}
		streets = append(streets, value)
	}
	holes, _ := ctx.Value(denseSpatialHolesKey{}).(float64)
	if spec.BentRows && input.request.Mode == 1 && input.request.Density == densityDense && !multi && holes <= 0 {
		return id, false
	}
	id.spec.DenseSpan = 0
	id.spec.Limit = denseLatitudeHeightLimit(ctx, spec.Limit)
	if !spec.Street {
		id.spec.CapacityFill = false
	} else if len(streets) > 0 {
		id.spec.CapacityFill = false
		id.spec.StreetGroupLimit = 0
	}
	raw, err := json.Marshal(struct {
		Rows    []rowValue
		Streets []streetValue
	}{values, streets})
	if err != nil {
		t.Fatal(err)
	}
	id.execution = string(raw)
	return id, true
}

// TestConnectedSemanticTransportDifferential 职责：对照真实planTransportRows验证首原规格、普通能力和大厂分组；使用：小规模枚举；边界：无几何或候选数硬编码取代覆盖证明。
func TestConnectedSemanticTransportDifferential(t *testing.T) {
	for _, test := range []struct {
		item    int
		rate    float64
		density string
	}{{1101, 60, "dense"}, {1101, 1860, "dense"}, {1201, 600, "dense"}, {6001, 60, "dense"}, {6001, 600, "dense"}, {1201, 600, "standard"}} {
		t.Run(fmt.Sprintf("%d/%g/%s", test.item, test.rate, test.density), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			input := connectedPreparedTestInput(t, ctx, test.item, test.rate, test.density)
			ctx = input.algorithmContext(ctx)
			options := DefaultConnectedSearchOptions()
			options.HeightLimits = []float64{20, 60}
			options.MeasureHeightLimits = false
			b, err := input.newAssembly(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			var keys, capacityKeys map[int]string
			paired := false
			if test.density == densityDense {
				keys, paired, err = b.denseConnectedCandidateFacts(ctx, input.request, input.dataset, false)
				if err != nil {
					t.Fatal(err)
				}
				capacityKeys, _, err = b.denseConnectedCandidateFacts(ctx, input.request, input.dataset, true)
				if err != nil {
					t.Fatal(err)
				}
			}
			raw := deduplicateConnectedCandidates(ctx, input.request, planConnectedCandidates(input.request, options.HeightLimits, options, paired), keys, capacityKeys)
			before := append([]ConnectedCandidateSpec(nil), raw...)
			seen := map[semanticReferenceIdentity]bool{}
			want := []ConnectedCandidateSpec{}
			for _, spec := range raw {
				id, ok := semanticReferenceRows(t, ctx, input, spec)
				if ok && !seen[id] {
					seen[id] = true
					want = append(want, spec)
				}
			}
			actual, err := deduplicateConnectedRows(ctx, input, raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("coverage or stable representative differs: actual=%d reference=%d", len(actual), len(want))
			}
			if !reflect.DeepEqual(raw, before) {
				t.Fatal("source specifications mutated")
			}
			enumerated, err := EnumerateConnectedCandidates(ctx, input, options)
			if err != nil {
				t.Fatal(err)
			}
			if len(enumerated) != 2*len(want) {
				t.Fatalf("entry groups lost: %d vs %d", len(enumerated), 2*len(want))
			}
			for group := 0; group < 2; group++ {
				for i, spec := range want {
					spec.LegacyInputs = group == 1
					if enumerated[group*len(want)+i] != spec {
						t.Fatal("enumerator changed representative or entry strategy")
					}
				}
			}
			t.Logf("specifications=%d retained=%d independently classified=%d", len(raw), len(actual), len(raw))
		})
	}
}

// TestConnectedSemanticRealStrategies 职责：保护确实不同的几何/路线策略、不同容量分行及折行能力；使用：负例；边界：只排除已证明缺必要条件的候选。
func TestConnectedSemanticRealStrategies(t *testing.T) {
	ctx := context.Background()
	input := connectedPreparedTestInput(t, ctx, 6001, 60, densityDense)
	ctx = input.algorithmContext(ctx)
	base := ConnectedCandidateSpec{Limit: 30, DenseSpan: 12}
	other := base
	other.CapacityFill = true
	got, err := deduplicateConnectedRows(ctx, input, []ConnectedCandidateSpec{other, base})
	if err != nil || len(got) != 1 || got[0] != other {
		t.Fatalf("first raw spec not retained: %+v %v", got, err)
	}
	for _, change := range []func(*ConnectedCandidateSpec){func(s *ConnectedCandidateSpec) { s.DenseOrder = 1 }, func(s *ConnectedCandidateSpec) { s.DenseOrder = 2 }, func(s *ConnectedCandidateSpec) { s.Refine = true }, func(s *ConnectedCandidateSpec) { s.TranslateCompact = true }, func(s *ConnectedCandidateSpec) { s.ProducerSplits = true; s.SplitterModel = 38 }, func(s *ConnectedCandidateSpec) { s.MaterialNetworks = true }, func(s *ConnectedCandidateSpec) { s.PairedOutputs = true }, func(s *ConnectedCandidateSpec) { s.LegacyInputs = true }} {
		other = base
		change(&other)
		got, err = deduplicateConnectedRows(ctx, input, []ConnectedCandidateSpec{base, other})
		if err != nil || len(got) != 2 {
			t.Fatalf("real strategy merged: %+v %v", other, err)
		}
	}
	bent := base
	bent.BentRows = true
	got, err = deduplicateConnectedRows(ctx, input, []ConnectedCandidateSpec{bent, base})
	if err != nil || len(got) != 1 || got[0] != base {
		t.Fatalf("impossible bent candidate retained: %v %v", got, err)
	}
	large := connectedPreparedTestInput(t, ctx, 1101, 1500, densityDense)
	largeCtx := large.algorithmContext(ctx)
	got, err = deduplicateConnectedRows(largeCtx, large, []ConnectedCandidateSpec{bent})
	if err != nil || len(got) != 1 {
		t.Fatalf("multi-row bent ability lost: %v %v", got, err)
	}
	street := base
	street.Street = true
	street.DenseSpan = 0
	other = street
	other.StreetGroupLimit = 2
	got, err = deduplicateConnectedRows(largeCtx, large, []ConnectedCandidateSpec{street, other})
	if err != nil || len(got) != 2 {
		t.Fatalf("multi-group auto merged with fixed street: %v %v", got, err)
	}
	other = base
	other.CapacityFill = true
	got, err = deduplicateConnectedRows(largeCtx, large, []ConnectedCandidateSpec{base, other})
	if err != nil || len(got) != 2 {
		t.Fatalf("different effective capacity groups merged: %v %v", got, err)
	}
	// 探针失败只能撤销合并证明，不能让一个候选的失败终止整组枚举。
	request, err := input.RequestSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	request.Plan.Productions[0].MachineCount = 0
	unknown := *input
	unknown.request = request
	got, err = deduplicateConnectedRows(ctx, &unknown, []ConnectedCandidateSpec{base, other})
	if err != nil || len(got) != 2 {
		t.Fatalf("unknown row plan discarded: %v %v", got, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err = deduplicateConnectedRows(canceled, input, []ConnectedCandidateSpec{base}); err != context.Canceled {
		t.Fatal("cancel lost", err)
	}
}

// TestConnectedSemanticGeometry 职责：直接装配两侧原规格并跑全规则，核对实体和物流；使用：容量与街道语义合并实证；边界：只忽略容量开关附加的说明性检查，不忽略路线、容量、shape或规则。
func TestConnectedSemanticGeometry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	input := connectedPreparedTestInput(t, ctx, 6001, 60, densityDense)
	for _, test := range []struct{ street, bent bool }{{false, false}, {true, false}, {false, true}} {
		street := test.street
		candidateInput := input
		base := ConnectedCandidateSpec{Limit: 30, DenseSpan: 12, Street: street}
		if test.bent {
			candidateInput = connectedPreparedTestInput(t, ctx, 1101, 1860, densityDense)
			base.Limit, base.BentRows, base.ProducerSplits, base.SplitterModel = 60, true, true, 38
		}
		if street {
			base.DenseSpan = 0
		}
		other := base
		other.CapacityFill = true
		if street {
			other.StreetGroupLimit = 2
		}
		a, err := connectedVerifiedTestCandidate(ctx, candidateInput, base)
		if err != nil {
			t.Fatal(err)
		}
		b, err := connectedVerifiedTestCandidate(ctx, candidateInput, other)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a.Geometry().Document(), b.Geometry().Document()) {
			t.Fatal("merged candidate geometry changed")
		}
		if !reflect.DeepEqual(a.Validation().Audit(), b.Validation().Audit()) {
			t.Fatal("merged candidate rules changed")
		}
		left, right := a.Validation().Logistics(), b.Validation().Logistics()
		if test.bent {
			found := false
			for _, check := range left.Checks {
				if check.RuleID == "dense-bent-rows" && check.Status == "passed" {
					found = true
					t.Log(check.Message)
				}
			}
			if !found {
				t.Fatal("bent fixture did not form real L/U units")
			}
		}
		filter := func(checks []LogisticsCheck) []LogisticsCheck {
			out := []LogisticsCheck{}
			for _, c := range checks {
				if c.RuleID != "dense-capacity-fill-grouping" {
					out = append(out, c)
				}
			}
			return out
		}
		left.Checks, right.Checks = filter(left.Checks), filter(right.Checks)
		if !reflect.DeepEqual(left, right) {
			t.Fatal("merged candidate logistics or shape changed")
		}
		t.Logf("street=%t bent=%t buildings=%d aspect=%.9f rules=%d identical", street, test.bent, len(a.Geometry().Document().Buildings), a.Metrics().EnvelopeAspectRatio, len(RequiredRuleKinds()))
	}
}
