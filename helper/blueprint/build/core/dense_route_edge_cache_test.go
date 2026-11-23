package build

import (
	"testing"

	"DysonSphereBox/helper/blueprint"
)

func TestIntegerBeltEdgeRechecksNewCrossing(t *testing.T) {
	index := denseRoutePortIndex{
		strictBelts: true, integerEdges: newDenseIntegerEdgeCache(vector{-2, -2, 0}, vector{8, 8, 2}),
		beltEdges: map[[2]*placed]bool{}, beltCells: map[[3]int][]int{},
	}
	add := func(a, b vector) {
		record := blueprint.Building{ItemID: 2001}
		index.addBeltSpan(&placed{a: a, record: record, output: &placed{a: b, record: record}})
	}
	a, b := vector{}, vector{x: 1}
	add(vector{5, 5, 0}, vector{6, 5, 0})
	if !index.spanClear(a, b) || !index.spanClear(b, a) {
		t.Fatal("无交叉的正反向整数边不可用")
	}
	add(vector{.5, -.5, 0}, vector{.5, .5, 0})
	if index.spanClear(a, b) || index.spanClear(b, a) {
		t.Fatal("新增交叉带段后仍复用了旧净空")
	}
}

func TestTransportGoalProbeKeepsReachableOrUnresolvedComponents(t *testing.T) {
	env := &transportEnvironment{}
	for _, test := range []struct {
		width int
		start float64
		want  bool
	}{
		{4, 8, false}, {4, 3, true}, {64, 80, false}, {65, 80, true},
	} {
		inside := func(v vector) bool { return v.y == 0 && v.z == 0 && v.x >= 0 && v.x < float64(test.width) }
		if got := env.transportGoalHasApproach(t.Context(), vector{x: test.start}, vector{}, nil, nil, inside); got != test.want {
			t.Fatalf("封闭通道长度 %d，源点 %.0f：得到 %t", test.width, test.start, got)
		}
	}
}
