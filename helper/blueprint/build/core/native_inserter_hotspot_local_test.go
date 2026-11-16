package build

// 文件职责：独立验证原生端点热点优化与冻结实现等价并测量局部成本。
// 使用：本轮并行性能任务；边界：不运行6007整厂，不替代主线完整生成验收，临时测试由主线统一清理。

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// nativeHotspotBox 职责：构造独立随机正交盒；使用：精确分离轴差分；边界：随机种子由调用方固定，不缩小盒以规避碰撞。
func nativeHotspotBox(rng *rand.Rand) placement.Box {
	u := surfaceUnit(placement.Vector{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()})
	v := surfaceUnit(u.Cross(placement.Vector{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()}))
	return placement.Box{Center: placement.Vector{rng.Float64()*8 - 4, rng.Float64()*8 - 4, rng.Float64()*8 - 4}, HalfSize: placement.Vector{.1 + rng.Float64(), .1 + rng.Float64(), .1 + rng.Float64()*4}, Axes: [3]placement.Vector{u, v, u.Cross(v)}}
}

// TestNativeHotspotSATDifferential 职责：逐对验证轴顺序优化与原始SAT一致；使用：随机、相切、退化和极近边界；边界：不以宽相位或容差代替原函数。
func TestNativeHotspotSATDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	for i := 0; i < 200000; i++ {
		a, b := nativeHotspotBox(rng), nativeHotspotBox(rng)
		if i%3 == 0 {
			b = a
			axis := i % 3
			gap := []float64{0, 1e-12, -1e-12, 1e-6, -1e-6}[i%5]
			b.Center = a.Center.Add(a.Axes[axis].Scale(2*a.HalfSize[axis] + gap))
		}
		if placement.BoxesOverlap(&a, &b) != nativeHotspotFrozenSAT(a, b) || a.Overlaps(b) != nativeHotspotFrozenSAT(a, b) {
			t.Fatalf("pair %d changed", i)
		}
	}
}

// nativeHotspotFrozenSAT 职责：冻结优化前交错访问十五轴的独立基线；使用：共同SAT回归；边界：不可随生产实现改写。
func nativeHotspotFrozenSAT(box, other placement.Box) bool {
	delta := other.Center.Sub(box.Center)
	separated := func(axis placement.Vector) bool {
		if axis.Dot(axis) < 1e-20 {
			return false
		}
		radius := 0.
		for i := 0; i < 3; i++ {
			radius += math.Abs(axis.Dot(box.Axes[i]))*box.HalfSize[i] + math.Abs(axis.Dot(other.Axes[i]))*other.HalfSize[i]
		}
		return math.Abs(delta.Dot(axis)) >= radius
	}
	for i := 0; i < 3; i++ {
		if separated(box.Axes[i]) || separated(other.Axes[i]) {
			return false
		}
		for j := 0; j < 3; j++ {
			if separated(box.Axes[i].Cross(other.Axes[j])) {
				return false
			}
		}
	}
	return true
}

// nativeHotspotDomains 职责：构造密集、稀疏、重复几何和多盒候选域；使用：冻结图逐位差分及基准；边界：记录差异保留，不将几何去重当端口去重。
func nativeHotspotDomains(count, choices, duplicates int, spacing float64) []nativeInserterDomain {
	rng := rand.New(rand.NewSource(89))
	domains := make([]nativeInserterDomain, count)
	for i := range domains {
		domains[i].index = i
		for j := 0; j < choices; j++ {
			var boxes []placement.OwnedBox
			if duplicates > 1 && j%duplicates != 0 {
				boxes = domains[i].choices[j-j%duplicates].boxes
			} else {
				box := nativeHotspotBox(rng)
				box.Center[0] = float64(i%8)*spacing + float64(j%3)*.05
				box.Center[1] = 200 + float64(i/8)*spacing
				box.Center[2] = rng.Float64() * .5
				boxes = []placement.OwnedBox{{Owner: i, Geometry: box}}
				if j%7 == 0 {
					box.Center[0] += .2
					boxes = append(boxes, placement.OwnedBox{Owner: i, Geometry: box})
				}
			}
			domains[i].choices = append(domains[i].choices, nativeInserterChoice{record: blueprint.Building{Index: int32(i), Yaw: float32(j)}, boxes: boxes})
		}
	}
	return domains
}

// TestNativeHotspotGraphDifferential 职责：逐边逐位及最终赋值对照冻结算法；使用：不同密度与重复率；边界：包括失败信息、空域和取消，不更改20000次预算。
func TestNativeHotspotGraphDifferential(t *testing.T) {
	for _, spacing := range []float64{.5, 3, 20} {
		for _, duplicates := range []int{1, 4} {
			domains := nativeHotspotDomains(24, 16, duplicates, spacing)
			ctx := context.Background()
			old, oe := frozenNativeInserterConflictGraph(ctx, domains)
			got, ge := nativeInserterConflictGraph(ctx, domains)
			if fmt.Sprint(oe) != fmt.Sprint(ge) || !reflect.DeepEqual(got, old) {
				t.Fatalf("graph changed: spacing=%g duplicates=%d", spacing, duplicates)
			}
			want, we := frozenSolveNativeInserterDomains(ctx, domains)
			actual, ae := solveNativeInserterDomains(ctx, domains)
			if fmt.Sprint(we) != fmt.Sprint(ae) || !reflect.DeepEqual(actual, want) {
				t.Fatalf("selection changed: spacing=%g duplicates=%d: %v / %v", spacing, duplicates, we, ae)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nativeInserterConflictGraph(ctx, nativeHotspotDomains(1, 1, 1, 3)); err != context.Canceled {
		t.Fatal(err)
	}
	for _, domains := range [][]nativeInserterDomain{nil, {{index: 0}}, {{index: 0, choices: []nativeInserterChoice{{}}}}} {
		old, oe := frozenNativeInserterConflictGraph(context.Background(), domains)
		got, ge := nativeInserterConflictGraph(context.Background(), domains)
		if fmt.Sprint(oe) != fmt.Sprint(ge) || !reflect.DeepEqual(got, old) {
			t.Fatal("empty geometry changed")
		}
	}
}

// TestNativeHotspotProgress 职责：核验真实构图计数及通知取消；使用：局部观察者；边界：完成数只在实际构图结束后等于总数。
func TestNativeHotspotProgress(t *testing.T) {
	domains := nativeHotspotDomains(12, 8, 2, 3)
	var events []Progress
	ctx := WithProgress(context.Background(), func(p Progress) { events = append(events, p) })
	if _, err := nativeInserterConflictGraph(ctx, domains); err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 || events[0].Current != 0 || events[len(events)-1].Current != len(domains) {
		t.Fatalf("events=%+v", events)
	}
	for _, p := range events {
		if p.Stage != "native-inserter-conflicts" || p.Total != len(domains) {
			t.Fatal(p)
		}
	}
	parent, cancel := context.WithCancel(context.Background())
	ctx = WithProgress(parent, func(p Progress) { cancel() })
	if _, err := nativeInserterConflictGraph(ctx, domains); err != context.Canceled {
		t.Fatal(err)
	}
}

// BenchmarkNativeHotspotGraph 职责：对相同内存域比较原图与优化图；使用：本地CPU基准；边界：合成负载收益不能冒充6007真实端到端收益。
func BenchmarkNativeHotspotGraph(b *testing.B) {
	for _, duplicates := range []int{1, 4} {
		domains := nativeHotspotDomains(96, 48, duplicates, 3)
		for _, frozen := range []bool{true, false} {
			b.Run(fmt.Sprintf("duplicates=%d/frozen=%t", duplicates, frozen), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var err error
					if frozen {
						_, err = frozenNativeInserterConflictGraph(context.Background(), domains)
					} else {
						_, err = nativeInserterConflictGraph(context.Background(), domains)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
