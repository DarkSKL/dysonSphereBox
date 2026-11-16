package build

// 文件职责：以独立全表距离扫描核对球面设施索引；使用：本轮性能验证；边界：随机包含负坐标、不同设施类别和阈值边界。
import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
	"math/rand"
	"testing"
)

func TestPolarStationIndexMatchesScan(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	index := &polarStationIndex{}
	points := []polarStationOccupancy{}
	items := []int16{2103, 2104, 2201, 2202, 2210}
	for i := 0; i < 1200; i++ {
		item := items[rng.Intn(len(items))]
		point := placement.Vector{rng.Float64()*150 - 75, rng.Float64()*20 - 10, rng.Float64()*150 - 75}
		index.add(item, point)
		points = append(points, polarStationOccupancy{point, item})
	}
	for i := 0; i < 10000; i++ {
		item := items[rng.Intn(len(items))]
		point := placement.Vector{rng.Float64()*160 - 80, rng.Float64()*24 - 12, rng.Float64()*160 - 80}
		if i%3 == 0 {
			a := points[rng.Intn(len(points))]
			point = a.point
			point[0] += math.Sqrt(polarStationDistanceSquared(item, a.item)) + float64(i%5-2)*1e-7
		}
		expected := false
		for _, other := range points {
			limit := 0.
			if surfaceIsStation(item) && surfaceIsStation(other.item) {
				limit = float64(surfaceStationDistanceLimit(item, other.item))
			}
			if surfaceIsPowerNode(item) && surfaceIsPowerNode(other.item) {
				d := math.Sqrt(float64(surfacePowerDistanceLimit(item, other.item))) + 1e-4
				limit = d * d
			}
			delta := point.Sub(other.point)
			if delta.Dot(delta) < limit {
				expected = true
				break
			}
		}
		if actual := index.conflicts(item, point); actual != expected {
			t.Fatalf("query%d actual%v expected%v", i, actual, expected)
		}
	}
}
