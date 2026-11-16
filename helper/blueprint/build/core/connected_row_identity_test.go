package build

import (
	"context"
	"fmt"
	"testing"
)

// assertConnectedRowCoverage 用完整运输规划独立验证被合并规格的执行行与高度，不复用生产去重键。
func assertConnectedRowCoverage(t *testing.T, ctx context.Context, input *PreparedInput, actual, original []ConnectedCandidateSpec) {
	t.Helper()
	ctx = input.algorithmContext(ctx)
	type identity struct {
		spec ConnectedCandidateSpec
		rows string
	}
	key := func(spec ConnectedCandidateSpec) identity {
		k := identity{spec: spec}
		if input.request.Density != densityDense && !spec.SpatialRows && input.geometry.Strategy != "native-window" {
			return k
		}
		b, err := input.newAssembly(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		b.denseSpatial, b.denseCapacityFill, b.denseMaxSpan = true, spec.CapacityFill, spec.DenseSpan
		rows, err := b.planTransportRows(ctx, input.request, input.dataset)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			k.rows += fmt.Sprintf("%d:%d:%d;", row.owner, row.info.RecipeID, row.count)
		}
		k.spec.DenseSpan = 0
		k.spec.Limit = denseLatitudeHeightLimit(ctx, spec.Limit)
		return k
	}
	want := make([]ConnectedCandidateSpec, 0, len(original))
	seen := map[identity]bool{}
	for _, spec := range original {
		id := key(spec)
		if !seen[id] {
			seen[id] = true
			want = append(want, spec)
		}
	}
	if len(actual) != len(want) {
		t.Fatalf("row execution coverage differs: %d vs %d", len(actual), len(want))
	}
	for i, spec := range actual {
		if spec != want[i] {
			t.Fatalf("stable row representative differs at %d", i)
		}
	}
}
