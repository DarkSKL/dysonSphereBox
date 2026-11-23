package planet

// 文件职责：区分建筑高度、地表占地与物流塔专用间距；使用：空间利用率回归；边界：岸线为独立合成样例，碰撞及塔距由原生蓝图终检交叉验证。

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// TestTerrainSamplingUsesHorizontalFootprint 职责：防止高建筑把远处水面误当脚下液面；使用：星际塔和射线接收器沿岸落位；边界：八米外的海水不属于这两个原型的水平建造盒。
func TestTerrainSamplingUsesHorizontalFootprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		item, model int16
	}{{"station", 2104, 50}, {"receiver", 2208, 73}} {
		t.Run(tc.name, func(t *testing.T) {
			obj := Object{Building: blueprint.Building{ItemID: tc.item, ModelIndex: tc.model, InputObjIdx: -1, OutputObjIdx: -1}}
			coast := func(p Position) (bool, bool) {
				return 200*math.Hypot(p.Longitude, p.Latitude) > 8, true
			}
			if terrainConflict([]Object{obj}, coast, catalog) {
				t.Fatalf("原型 %d 的水平占地不足八米，建筑高度却导致远处岸线误拒", tc.item)
			}
			if !terrainConflict([]Object{obj}, func(Position) (bool, bool) { return true, true }, catalog) {
				t.Fatal("缩小水平采样范围后不能放过建筑脚下的真实液面")
			}
		})
	}
}

// TestStationOccupancyMatchesNativePlacement 职责：允许合法塔距及塔旁的真实机器占地；使用：候选筛选与原生模型一致性回归；边界：只调整已知新模块之间的规则，不放宽存档未知碰撞。
func TestStationOccupancyMatchesNativePlacement(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		item      int16
		model     int16
		distance  float64
		collision bool
	}{
		{"interstellar-30m", 2104, 50, 30, false},
		{"interstellar-28m", 2104, 50, 28, true},
		{"receiver-12m", 2208, 73, 12, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := stationObject(0, Position{}, 1208, 1, 0, false)
			p := Position{Longitude: tc.distance / 200}
			second := Object{First: p, Second: p, Building: blueprint.Building{Index: 1, ItemID: tc.item, ModelIndex: tc.model, InputObjIdx: -1, OutputObjIdx: -1, Parameters: []int32{0}}}
			if tc.item == 2104 {
				second = stationObject(1, p, 1209, 2, 0, false)
			}
			_, nativeErr := Build(context.Background(), Request{PlanetID: 101, Objects: []Object{first, second}})
			if (nativeErr != nil) != tc.collision {
				t.Fatalf("独立原生几何基线不符：冲突=%t，错误=%v", tc.collision, nativeErr)
			}
			occupied := newObstacleIndex(extendOccupancy(Request{Objects: []Object{first}}, catalog))
			if placementConflict([]Object{second}, occupied, catalog) != tc.collision {
				t.Fatalf("候选筛选与实际模型/29米塔距不符：%s，原生错误=%v", tc.name, nativeErr)
			}
		})
	}
}
