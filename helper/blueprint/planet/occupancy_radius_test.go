package planet

// 文件职责：区分建筑高度、地表占地与物流塔专用间距；使用：空间利用率回归；边界：岸线为独立合成样例，碰撞及塔距由原生蓝图终检交叉验证。

import (
	"context"
	"errors"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestSpaceCapsuleAt104 用原版第 2721 号塔的落点核对选址和终检，并保留相邻可放格、高空和传送带。
func TestSpaceCapsuleAt104(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	point, err := savePosition(galaxy.Vector{X: -156.04660, Y: 26.33918, Z: -122.62030})
	if err != nil {
		t.Fatal(err)
	}
	birth := galaxy.Vector{X: -160.45245, Y: 19.09065, Z: -120.63697}
	tower := stationObject(2721, point, 1001, 1, 0, false)
	adjacent, elevated, belt := tower, tower, tower
	adjacent.First.Latitude += .72 * math.Pi / 180
	adjacent.Second = adjacent.First
	elevated.First.Height, elevated.Second.Height = 3, 3
	belt.Building.ItemID, belt.Building.ModelIndex = 2003, 37
	for _, tc := range []struct {
		name    string
		object  Object
		capsule *galaxy.Vector
		blocked bool
	}{
		{"原塔位", tower, &birth, true},
		{"相邻格", adjacent, &birth, false},
		{"高空", elevated, &birth, false},
		{"带端", belt, &birth, false},
		{"已回收", tower, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{PlanetID: 104, spaceCapsule: tc.capsule, Objects: []Object{tc.object}}
			if blocked := req.terrainConflict(req.Objects, catalog); blocked != tc.blocked {
				t.Errorf("候选落位拒绝=%t，需要 %t", blocked, tc.blocked)
			}
			err := validatePlacement(t.Context(), req, []blueprint.Building{tc.object.Building})
			var placementError *Error
			if tc.blocked {
				if !errors.As(err, &placementError) || placementError.Code != "PLANET_SPACE_CAPSULE" {
					t.Fatalf("终检未报告太空舱冲突：%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestStationSpacingAt3402 用实际失败的 338 号塔和存档 4611 号大型采矿机核对选址及终检。
func TestStationSpacingAt3402(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	miner, err := savePosition(galaxy.Vector{X: -118.761856, Y: -136.126663, Z: -86.285515})
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := savePosition(galaxy.Vector{X: -124.606873, Y: -115.630066, Z: -105.749008})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 3402, Obstacles: []Obstacle{{Position: miner, Radius: 8, stationItem: 2316}}}
	for _, tc := range []struct {
		offset   float64
		collides bool
	}{{0, true}, {.36 * math.Pi / 180, false}} {
		p := anchor
		p.Latitude += tc.offset
		tower := stationObject(338, p, 1001, 1, 0, false)
		req.Objects = []Object{tower}
		if hit := placementConflict(req.Objects, newObstacleIndex(req.Obstacles), catalog); hit != tc.collides {
			t.Errorf("原星塔距纬度偏移 %.6f，选址碰撞=%t", tc.offset, hit)
		}
		err := validatePlacement(t.Context(), req, []blueprint.Building{tower.Building})
		if (err != nil) != tc.collides {
			t.Errorf("原星塔距纬度偏移 %.6f，终检=%v", tc.offset, err)
		}
	}
}

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

// TestInserterCrosses1303Vein 回放 1303 星可燃冰 431 的原版施工预览。相邻两臂共用制造台，只有 28645 号臂身碰矿。
func TestInserterCrosses1303Vein(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	point := func(lon, lat float64) Position {
		return Position{Longitude: lon * math.Pi / 180, Latitude: lat * math.Pi / 180}
	}
	host := Object{Building: blueprint.Building{
		Index: 28630, ItemID: 2303, ModelIndex: 65, Yaw: 90, Yaw2: 90,
		InputObjIdx: -1, OutputObjIdx: -1, RecipeID: 36, Parameters: []int32{0},
	}}
	host.First = point(141.84001132, 4.31999903)
	host.Second = host.First
	for _, tc := range []struct {
		id, belt              int32
		slot                  int8
		span                  int32
		beltAt, first, second Position
		hit                   bool
	}{
		{28645, 28571, 6, 3, point(140.40001620, 3.96000025), point(140.40001620, 4.03199727), point(141.55202323, 4.03199727), true},
		{28644, 28590, 7, 2, point(140.76002181, 4.31999903), point(140.76002181, 4.31999903), point(141.55202323, 4.31999903), false},
	} {
		belt := Object{First: tc.beltAt, Second: tc.beltAt, Building: blueprint.Building{
			Index: tc.belt, ItemID: 2003, ModelIndex: 37, Yaw: 180, Yaw2: 180,
			InputObjIdx: -1, OutputObjIdx: -1, InputToSlot: 1,
		}}
		arm := Object{First: tc.first, Second: tc.second, Building: blueprint.Building{
			Index: tc.id, ItemID: 2013, ModelIndex: 43, Yaw: 90, Yaw2: 90,
			InputObjIdx: tc.belt, OutputObjIdx: host.Building.Index, InputFromSlot: -1,
			OutputToSlot: tc.slot, InputToSlot: 1, Parameters: []int32{tc.span},
		}}
		req := Request{PlanetID: 1303, TerrainKnown: true, Foundation: "construction", Objects: []Object{belt, host, arm}}
		req.protectVein(saveanalysis.PlanetVein{ID: 431, VeinType: 8, Position: galaxy.Vector{X: 126.2834, Y: 13.76171, Z: 154.5675}})
		occupied := newObstacleIndex(req.minerals)
		if placementConflict(req.Objects, occupied, catalog) {
			t.Fatalf("原图 %d 的端点和普通设备应均有净空", tc.id)
		}
		if req.siteOccupied(t.Context(), req.Objects, occupied, catalog) != tc.hit {
			t.Errorf("原图 %d 选址与原版施工结果不符", tc.id)
		}
		_, err := Build(t.Context(), req)
		if tc.hit {
			var issue *Error
			if !errors.As(err, &issue) || issue.Code != "PLANET_VEIN_OCCUPIED" {
				t.Errorf("原图 %d 终检应报告臂身碰矿，得到 %v", tc.id, err)
			}
		} else if err != nil {
			t.Errorf("原图 %d 的相邻分拣器应能施工：%v", tc.id, err)
		}
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
	req.groundPad = func(p Position) bool {
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

// TestDryGroundPadsAt2003 用旱星实测的悬空塔脚隔离地面支撑规则，同时从选址和整图导出验证塔与电杆。
func TestDryGroundPadsAt2003(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	towerSite := Position{Longitude: 133.2 * math.Pi / 180, Latitude: -41.4 * math.Pi / 180}
	// 原档右后脚半径 198.79868 米；同纬度西移到 130.5° 时，八脚最低 199.90139 米。
	lowPad := Position{Longitude: 2.3516448584, Latitude: -.7323765272}
	supported := towerSite
	supported.Longitude = 130.5 * math.Pi / 180
	tower := stationObject(0, towerSite, 1001, 1, 0, false)
	pole := Object{First: lowPad, Second: lowPad, Building: blueprint.Building{ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}}
	for _, tc := range []struct {
		name      string
		obj       Object
		obstacles []Obstacle
	}{{"地表终检中的塔", tower, []Obstacle{{Position: Position{}, Radius: 1}}}, {"无障碍的塔", tower, nil}, {"无障碍的电杆", pole, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{PlanetID: 2003, TerrainKnown: true, Objects: []Object{tc.obj}, Obstacles: tc.obstacles}
			req.groundPad = func(p Position) bool {
				return math.Abs(p.Latitude-lowPad.Latitude) > 1e-6 || math.Abs(p.Longitude-lowPad.Longitude) > 1e-6
			}
			if !req.terrainConflict(req.Objects, catalog) {
				t.Error("选址放过了缺少支撑的落点")
			}
			var issue *Error
			if _, err := Build(t.Context(), req); !errors.As(err, &issue) || issue.Code != "PLANET_WATER" {
				t.Errorf("终检没有报告地面缺少支撑：%v", err)
			}
			req.Objects[0].First, req.Objects[0].Second = supported, supported
			if req.terrainConflict(req.Objects, catalog) {
				t.Error("选址拒绝了相邻有支撑的落点")
			}
			if _, err := Build(t.Context(), req); err != nil {
				t.Errorf("有支撑的设施导出失败：%v", err)
			}
		})
	}
}

// TestTeslaPoleLandPad 回归 101 星 5880 号电杆。熔岩液面 197.5 米，落点虽无液体，
// 铺地后中心射线只命中半径 199.2649 米。附近有支撑的电杆以及普通传送带仍可通过。
func TestTeslaPoleLandPad(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	position := Position{Longitude: 4.442212105, Latitude: .043982267}
	pole := Object{First: position, Second: position, Building: blueprint.Building{ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}}
	req := Request{PlanetID: 101, TerrainKnown: true, wet: func(Position) (bool, bool) { return false, true }}
	req.groundPad = func(p Position) bool { return math.Abs(p.Longitude-position.Longitude) > .001 }
	if !req.terrainConflict([]Object{pole}, catalog) {
		t.Fatal("原版铺地后悬空的中心支点通过了供电选址")
	}
	req.Objects = []Object{pole}
	if _, err := Build(t.Context(), req); err == nil {
		t.Fatal("悬空电杆通过了整星终检")
	}
	pole.First.Longitude += .02
	pole.Second = pole.First
	req.Objects = []Object{pole}
	if _, err := Build(t.Context(), req); err != nil {
		t.Fatal("相邻有地落点不能施工", err)
	}
	belt := Object{First: position, Second: position, Building: blueprint.Building{ItemID: 2003, ModelIndex: 37}}
	if req.terrainConflict([]Object{belt}, catalog) {
		t.Fatal("电杆中心支点被套用到传送带")
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
