package build

// 文件职责：固定电力、物流塔和最终引用规则的安全侧、边界和违规侧样本。
// 使用：离线规则回归；边界：仅验证版本化数值条件，不假冒游戏放置或实际产能。

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestStationSpacingBoundaries 职责：校验行星塔与星际塔的严格最小距离；使用：特殊规则回归；边界：相等合法，低于阈值冲突。
func TestStationSpacingBoundaries(t *testing.T) {
	for _, pair := range [][2]int16{{2103, 2103}, {2103, 2104}, {2104, 2104}} {
		limit := math.Sqrt(float64(surfaceStationDistanceLimit(pair[0], pair[1])))
		for _, delta := range []float64{-.01, 0, .01} {
			records := []blueprint.Building{{ItemID: pair[0]}, {ItemID: pair[1]}}
			left, right, err := surfaceStationConflict(context.Background(), records, []vector{{}, {limit + delta, 0, 0}})
			if err != nil || (left >= 0) != (delta < 0) || left >= 0 && right != 1 {
				t.Fatalf("station pair=%v delta=%g conflict=%d/%d err=%v", pair, delta, left, right, err)
			}
		}
	}
}

// TestPowerSpacingBoundaries 职责：校验普通节点、风机与地热阈值；使用：数值矩阵；边界：不把所有原型误用感应塔间距。
func TestPowerSpacingBoundaries(t *testing.T) {
	for _, pair := range [][2]int16{{2201, 2201}, {2203, 2203}, {2213, 2213}, {2206, 2207}} {
		limit := math.Sqrt(float64(surfacePowerDistanceLimit(pair[0], pair[1])))
		for _, delta := range []float64{-.01, 0, .01} {
			distance := math.Max(0, limit+delta)
			left, _, err := surfacePowerNodeConflict(context.Background(), []blueprint.Building{{ItemID: pair[0]}, {ItemID: pair[1]}}, []vector{{}, {distance, 0, 0}})
			if err != nil || (left >= 0) != (limit > 0 && delta < 0) {
				t.Fatalf("power pair=%v delta=%g conflict=%d err=%v", pair, delta, left, err)
			}
		}
	}
}

// TestPowerCoverageBoundary 职责：检查实际感应塔覆盖和网络连接的闭边界；使用：平面适配回归；边界：坐标按适配器转换到米。
func TestPowerCoverageBoundary(t *testing.T) {
	for _, delta := range []float64{-.1, .1} {
		records := []blueprint.Building{
			{Index: 0, ItemID: 2201, InputObjIdx: -1, OutputObjIdx: -1},
			{Index: 1, ItemID: 2301, RecipeID: 1, InputObjIdx: -1, OutputObjIdx: -1, LocalOffset: blueprint.Point{X: float32((10.5 + delta) / planarGridLength)}},
		}
		_, err := validateFinalPowerCoverage(context.Background(), newPlanarRuleEncoding(), records)
		if (err != nil) != (delta > 0) {
			t.Fatalf("cover delta=%g err=%v", delta, err)
		}
	}
	for _, delta := range []float64{-.1, .1} {
		records := []blueprint.Building{{Index: 0, ItemID: 2201}, {Index: 1, ItemID: 2201, LocalOffset: blueprint.Point{X: float32((22.5 + delta) / planarGridLength)}}}
		networks, err := validateFinalPowerCoverage(context.Background(), newPlanarRuleEncoding(), records)
		want := 2
		if delta < 0 {
			want = 1
		}
		if err != nil || networks != want {
			t.Fatalf("connect delta=%g networks=%d want=%d err=%v", delta, networks, want, err)
		}
	}
}

// TestStationPortFailureClosed 职责：覆盖出塔筛选、入塔标记、重复端口及槽容量；使用：最终参数校验；边界：缺失物品不允许 panic 或按无限容量放行。
func TestStationPortFailureClosed(t *testing.T) {
	station := blueprint.Building{Index: 0, ItemID: 2103, InputObjIdx: -1, OutputObjIdx: -1, Parameters: make([]int32, 2048)}
	station.Parameters[0], station.Parameters[1], station.Parameters[3] = 1001, 2, 5000
	station.Parameters[192], station.Parameters[193] = 1, 1
	belt := blueprint.Building{Index: 1, ItemID: 2001, InputObjIdx: 0, OutputObjIdx: -1, InputFromSlot: 0, Parameters: []int32{1001, 0}}
	if err := validateStationPorts(context.Background(), []blueprint.Building{station, belt}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"missing-item", "wrong-item", "bad-filter", "wrong-direction", "over-capacity", "duplicate-port"} {
		t.Run(invalid, func(t *testing.T) {
			tower := station
			tower.Parameters = append([]int32(nil), station.Parameters...)
			link := belt
			link.Parameters = append([]int32(nil), belt.Parameters...)
			records := []blueprint.Building{tower, link}
			switch invalid {
			case "missing-item":
				records[1].Parameters = nil
			case "wrong-item":
				records[1].Parameters[0] = 1002
			case "bad-filter":
				records[0].Parameters[193] = 5
			case "wrong-direction":
				records[0].Parameters[192] = 2
			case "over-capacity":
				records[0].Parameters[3] = 5001
			case "duplicate-port":
				duplicate := link
				duplicate.Index = 2
				records = append(records, duplicate)
			}
			if err := validateStationPorts(context.Background(), records); err == nil {
				t.Fatal("无效塔端口未拒绝")
			}
		})
	}
}

// TestFinalReferenceFailureClosed 职责：拒绝非有限坐标和悬空连接；使用：编码前检查回归；边界：合法开放外部端口允许 -1。
func TestFinalReferenceFailureClosed(t *testing.T) {
	valid := blueprint.Building{Index: 0, ItemID: 2001, InputObjIdx: -1, OutputObjIdx: -1}
	if err := validateFinalReferences(context.Background(), []blueprint.Building{valid}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []blueprint.Building{
		{Index: 0, InputObjIdx: -1, OutputObjIdx: 0},
		{Index: 0, InputObjIdx: -1, OutputObjIdx: 9},
		{Index: 0, InputObjIdx: -1, OutputObjIdx: -1, LocalOffset: blueprint.Point{X: float32(math.NaN())}},
	} {
		if err := validateFinalReferences(context.Background(), []blueprint.Building{bad}); err == nil {
			t.Fatalf("invalid reference allowed: %+v", bad)
		}
	}
}
