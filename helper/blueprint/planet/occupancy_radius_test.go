package planet

// 文件职责：区分建筑高度、地表占地与物流塔专用间距；使用：空间利用率回归；边界：岸线为独立合成样例，碰撞及塔距由原生蓝图终检交叉验证。

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestRaisedBeltClears1202Vein 用原版报告的 285 号硅矿核对 3976 号高架带和有净空的相邻格。
func TestRaisedBeltClears1202Vein(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 1202, Foundation: "construction"}
	req.protectVein(saveanalysis.PlanetVein{ID: 285, VeinType: 3, Position: galaxy.Vector{X: -193.41850, Y: -2.87963, Z: 55.27808}})
	belt := blueprint.Building{ItemID: 2003, ModelIndex: 37}
	for _, tc := range []struct {
		name  string
		point galaxy.Vector
		hit   bool
	}{
		{"原版碰矿的第3976节", galaxy.Vector{X: -193.86534, Y: -2.53248, Z: 55.00459}, true},
		{"相邻有净空的第3975节", galaxy.Vector{X: -193.84621, Y: -3.79859, Z: 54.99915}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := savePosition(tc.point)
			if err != nil {
				t.Fatal(err)
			}
			req.Objects = []Object{{First: p, Second: p, Building: belt}}
			if hit := placementConflict(req.Objects, newObstacleIndex(req.minerals), catalog); hit != tc.hit {
				t.Errorf("候选位置碰撞=%t，原版=%t", hit, tc.hit)
			}
			if err := validatePlacement(t.Context(), req, []blueprint.Building{belt}); (err != nil) != tc.hit {
				t.Errorf("终检结果=%v，原版碰撞=%t", err, tc.hit)
			}
		})
	}
}

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
			if (&Request{wet: coast}).terrainConflict([]Object{obj}, catalog) {
				t.Fatalf("原型 %d 的水平占地不足八米，建筑高度却导致远处岸线误拒", tc.item)
			}
			if !(&Request{wet: func(Position) (bool, bool) { return true, true }}).terrainConflict([]Object{obj}, catalog) {
				t.Fatal("缩小水平采样范围后不能放过建筑脚下的真实液面")
			}
		})
	}
}

// TestStationPadsAt2401Shore 用原版 368 号塔的斜向支脚复现岸线缺地，核对候选和整图终检共用的规则。
func TestStationPadsAt2401Shore(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	anchor := Position{Longitude: 32.76 * math.Pi / 180, Latitude: 16.56 * math.Pi / 180}
	tower := stationObject(0, anchor, 1004, 1, 1, false)
	req := Request{PlanetID: 2401, Objects: []Object{tower}, wet: func(Position) (bool, bool) { return false, true }}
	// 铺地后实测只有右后方这只脚落在半径 199.3742 米处；塔心及其余支脚均有地。
	req.stationPad = func(p Position) bool {
		return math.Abs(math.Sin(p.Latitude)-.294521) > .0001 ||
			math.Abs(math.Cos(p.Latitude)*math.Sin(p.Longitude)-.533805) > .0001
	}
	if !req.terrainConflict(req.Objects, catalog) {
		t.Fatal("选址放过了 368 号塔悬空的斜向支脚")
	}
	if _, err := Build(context.Background(), req); err == nil {
		t.Fatal("含悬空塔脚的蓝图通过了终检")
	}
	req.Objects[0].First.Longitude += .02
	req.Objects[0].Second = req.Objects[0].First
	if _, err := Build(context.Background(), req); err != nil {
		t.Fatalf("旁边八脚均有地的塔位应可施工：%v", err)
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
