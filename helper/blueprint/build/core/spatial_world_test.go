package build

// 文件职责：固定种子验证公共世界索引的候选、顺序、去重缓存和极区兼容性。
// 使用：本轮临时验收及性能对照；边界：与全扫描及冻结实现分别比较，不据此声称游戏内实测，主线验收后清理。

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"testing"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// spatialTestBounds 职责：构造有负坐标、跨桶和退化对象的确定性数据；使用：差分与基准；边界：全部为世界单位。
func spatialTestBounds(count int, seed int64) [][2]placement.Vector {
	rng := rand.New(rand.NewSource(seed))
	result := make([][2]placement.Vector, count)
	for i := range result {
		for axis := 0; axis < 3; axis++ {
			low := float64(rng.Intn(64)-32)*8 + rng.Float64()
			high := low + rng.Float64()*24
			if i%7 == 0 {
				low, high = float64(rng.Intn(16)-8)*8, float64(rng.Intn(16)-8)*8
				low, high = min(low, high), max(low, high)
			}
			if i%11 == 0 {
				high = low
			}
			result[i][0][axis], result[i][1][axis] = low, high
		}
	}
	return result
}

// spatialTestOverlap 职责：以独立全扫描判断闭合世界包围盒相交；使用：差分真值；边界：不调用生产桶换算。
func spatialTestOverlap(a, b [2]placement.Vector) bool {
	for axis := 0; axis < 3; axis++ {
		if a[1][axis] < b[0][axis] || a[0][axis] > b[1][axis] {
			return false
		}
	}
	return true
}

// TestSpatialWorldDifferential 职责：验证多种桶尺寸下全扫描命中、原枚举顺序及动态增长；使用：固定种子回归；边界：允许宽相位多选，禁止漏选或重访。
func TestSpatialWorldDifferential(t *testing.T) {
	objects, probes := spatialTestBounds(300, 6101), spatialTestBounds(180, 6102)
	for _, size := range []float64{8, 12, 29, 17.5} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			index := worldSpatialIndex{}
			for i, box := range objects {
				index.add(worldSpatialBounds(box[0], box[1], size), i*2)
				if i%31 == 0 {
					index.any(worldSpatialBounds(box[0], box[1], size), func(int) bool { return false })
				}
			}
			for _, probe := range probes {
				var got, want, order []int
				seen := map[int]bool{}
				cells := worldSpatialBounds(probe[0], probe[1], size)
				// 真值按旧三层循环读桶，验证首命中顺序而非仅集合。
				for x := int(math.Floor(probe[0][0] / size)); x <= int(math.Floor(probe[1][0]/size)); x++ {
					for y := int(math.Floor(probe[0][1] / size)); y <= int(math.Floor(probe[1][1]/size)); y++ {
						for z := int(math.Floor(probe[0][2] / size)); z <= int(math.Floor(probe[1][2]/size)); z++ {
							for _, id := range index.cells[[3]int{x, y, z}] {
								if !seen[id] {
									order = append(order, id)
									seen[id] = true
								}
							}
						}
					}
				}
				var visited []int
				index.any(cells, func(id int) bool {
					visited = append(visited, id)
					if spatialTestOverlap(probe, objects[id/2]) {
						got = append(got, id)
					}
					return false
				})
				if !slices.Equal(visited, order) {
					t.Fatal("候选顺序或去重变化")
				}
				for i, box := range objects {
					if spatialTestOverlap(probe, box) {
						want = append(want, i*2)
					}
				}
				slices.Sort(got)
				if !slices.Equal(got, want) {
					t.Fatalf("全扫描不符：got=%v want=%v", got, want)
				}
				calls := 0
				found := index.any(cells, func(id int) bool {
					calls++
					return true
				})
				if found != (len(order) > 0) || calls != min(1, len(order)) {
					t.Fatal("提前终止变化")
				}
			}
			index.query.serial = math.MaxUint32
			probe := objects[0]
			if !index.any(worldSpatialBounds(probe[0], probe[1], size), func(id int) bool { return id == 0 }) || index.query.serial != 1 {
				t.Fatal("查询序号回绕丢失对象")
			}
		})
	}
}

// TestSpatialPolarDifferential 职责：验证毫米外扩、过滤顺序与旧极区索引一致；使用：冻结基线和全扫描双对照；边界：不更改窄相位阈值。
func TestSpatialPolarDifferential(t *testing.T) {
	index, legacy := polarSpatialIndex{}, spatialLegacyPolarSpatialIndex{}
	objects := spatialTestBounds(512, 6111)
	for _, box := range objects {
		index.add(box[0], box[1])
		legacy.spatialLegacyAdd(box[0], box[1])
	}
	for _, probe := range append(spatialTestBounds(200, 6112), objects...) {
		var got, want, brute []int
		index.any(probe[0], probe[1], func(id int) bool { got = append(got, id); return false })
		legacy.spatialLegacyAny(probe[0], probe[1], func(id int) bool { want = append(want, id); return false })
		if !slices.Equal(got, want) {
			t.Fatal("极区查询顺序变化")
		}
		margin := placement.Vector{.001, .001, .001}
		expanded := [2]placement.Vector{probe[0].Sub(margin), probe[1].Add(margin)}
		for id, box := range objects {
			if spatialTestOverlap(expanded, box) {
				brute = append(brute, id)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, brute) {
			t.Fatal("极区全扫描不符")
		}
	}
}

// BenchmarkSpatialPolarQueries 职责：对比实际极区索引的热查询；使用：固定对象和探针、计入窄相位回调；边界：构建在计时外，另由生产基准覆盖构建开销。
func BenchmarkSpatialPolarQueries(b *testing.B) {
	objects, probes := spatialTestBounds(4096, 6121), spatialTestBounds(256, 6122)
	index, legacy := polarSpatialIndex{}, spatialLegacyPolarSpatialIndex{}
	for _, box := range objects {
		index.add(box[0], box[1])
		legacy.spatialLegacyAdd(box[0], box[1])
	}
	for _, old := range []bool{true, false} {
		b.Run(fmt.Sprintf("legacy=%t", old), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				probe := probes[i%len(probes)]
				if old {
					legacy.spatialLegacyAny(probe[0], probe[1], func(int) bool { return false })
				} else {
					index.any(probe[0], probe[1], func(int) bool { return false })
				}
			}
		})
	}
}
