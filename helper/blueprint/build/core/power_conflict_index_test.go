package build

// 文件职责：以独立穷举窄相位核对补电细桶与缓存包围盒；使用：性能优化临时验收；边界：不使用新宽相位作为期望结果。
import (
	"context"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func TestPowerConflictIndexMatchesExhaustive(t *testing.T) {
	ctx := context.Background()
	input := connectedPreparedTestInput(t, ctx, 1101, 60, "dense")
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	index, err := newConnectedPowerIndex(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	consumers := []connectedPowerConsumer{}
	for _, point := range []vector{{x: -6, y: -5}, {x: 2, y: 3}, {x: 9, y: 1}, {x: -2, y: 8}} {
		world, err := index.world(point)
		if err != nil {
			t.Fatal(err)
		}
		consumers = append(consumers, connectedPowerConsumer{center: point, world: world})
	}
	solver, err := newConnectedPowerSolver(index, consumers, connectedPowerCandidateOffsets(index.coverRadius))
	if err != nil {
		t.Fatal(err)
	}
	legacyConflict := func(a, c int) bool {
		left, right := solver.choices[a], solver.choices[c]
		delta := left.world.Sub(right.world)
		if delta.Dot(delta) < solver.minimumSpacing*solver.minimumSpacing {
			return true
		}
		for _, x := range left.boxes {
			for _, y := range right.boxes {
				if x.Overlaps(y) {
					return true
				}
			}
		}
		return false
	}
	rng := rand.New(rand.NewSource(20261116))
	for n := 0; n < 128; n++ {
		choice := rng.Intn(len(solver.choices))
		if err := solver.prepare(choice); err != nil {
			t.Fatal(err)
		}
		actual := append([]int(nil), solver.choices[choice].conflicts...)
		sort.Ints(actual)
		expected := []int{}
		for other := range solver.choices {
			if legacyConflict(choice, other) {
				expected = append(expected, other)
			}
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("conflict mismatch choice=%d: %d/%d", choice, len(actual), len(expected))
		}
		covers := []int{}
		for d, domain := range solver.domains {
			delta := solver.choices[choice].world.Sub(domain.consumer.world)
			if delta.Dot(delta) <= (index.coverRadius-1e-4)*(index.coverRadius-1e-4) {
				covers = append(covers, d)
			}
		}
		if !reflect.DeepEqual(solver.choices[choice].domains, covers) {
			t.Fatalf("coverage reuse mismatch %d", choice)
		}
	}
}
