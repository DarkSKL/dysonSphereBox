package planet

// 文件职责：回归原版文档兼容、定位与实际接驳；使用：逐星生成证明；边界：不能代替游戏 UI 实贴。

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestBuildGeometryAndFoundation 职责：核对跨纬带装配、独立投影与保护地基；使用：装配回归；边界：样例是两台原版风机。
func TestBuildGeometryAndFoundation(t *testing.T) {
	ctx := context.Background()
	lat, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	first := Position{Longitude: 6.19, Latitude: -100 * float64(lat)}
	second := Position{Longitude: .21, Latitude: 151 * float64(lat)}
	objects := []Object{}
	for i, p := range []Position{first, second} {
		objects = append(objects, Object{First: p, Second: p, Building: blueprint.Building{Index: int32(i + 5), ItemID: 2203, ModelIndex: 53, InputObjIdx: -1, OutputObjIdx: -1}})
	}
	req := Request{PlanetID: 101, GameVersion: "0.10.35.29151", Foundation: "construction", TerrainKnown: true, Objects: objects,
		Protected: []Cell{{Longitude: first.Longitude, Latitude: first.Latitude}}}
	built, err := Build(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || doc.Reform == nil || len(doc.Reform.Rects) == 0 || built.AreaCount < 2 || built.FoundationCells == 0 {
		t.Fatalf("跨纬带或保护格不符：Area %d，地基格 %d，%v", built.AreaCount, built.FoundationCells, err)
	}
	for _, rect := range doc.Reform.Rects {
		if rect.Data != 224 {
			t.Fatalf("地基未使用原版无装饰类型：%+v", rect)
		}
	}
	unprotected := req
	unprotected.Protected = nil
	unmasked, err := Build(ctx, unprotected)
	if err != nil || unmasked.FoundationCells <= built.FoundationCells {
		t.Fatalf("保护格未从建设地基中扣除：无遮罩 %d，有遮罩 %d，%v", unmasked.FoundationCells, built.FoundationCells, err)
	}
	// 独立用行星角位移计算每个 Area 的原点，核对压缩后的绝对经纬。
	for i, want := range []Position{first, second} {
		b := doc.Buildings[i]
		row := built.Anchor.Latitude / float64(lat)
		for area := doc.Areas[b.AreaIndex]; area.ParentIndex >= 0; area = doc.Areas[area.ParentIndex] {
			row += float64(area.AnchorLocalOffsetY)
		}
		area := doc.Areas[b.AreaIndex]
		gotLat := (row + float64(b.LocalOffset.Y)) * float64(lat)
		gotLon := built.Anchor.Longitude + float64(b.LocalOffset.X)*2*math.Pi/float64(area.AreaSegments*5)
		if math.Abs(gotLat-want.Latitude) > 1e-5 || math.Abs(math.Remainder(gotLon-want.Longitude, 2*math.Pi)) > 1e-5 {
			t.Fatalf("建筑 %d 绝对坐标不符：(%f,%f)，期望 %+v", i, gotLon, gotLat, want)
		}
	}
	req.Obstacles = []Obstacle{{Position: first, Radius: 1}}
	_, err = Build(ctx, req)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_OCCUPIED" {
		t.Fatalf("未发现已有占用冲突：%v", err)
	}
}

// TestExistingReformAndSoil 职责：验证现成地基及原始高程不重复计费；使用：施工清单回归；边界：使用独立的顶点样本。
func TestExistingReformAndSoil(t *testing.T) {
	req := Request{}
	offsets := make([]int64, 500)
	offsets[10] = 4
	if err := configureTerrainReform(&req, saveanalysis.PlanetTerrain{ReformOffsets: offsets,
		ReformData: []byte{0, 0, 0, 0, 0, 224}, HeightCentimeters: []uint16{19990, 20040, 20000},
		ModData: []byte{0, 3},
	}); err != nil {
		t.Fatal(err)
	}
	if !req.isReformed(1, 10) || req.isReformed(0, 10) || req.soilUpper == nil || *req.soilUpper != 30 {
		t.Fatalf("原版旧地基或沙土折算不符")
	}
}

// TestWholePlanetReformGrid 职责：核对整星模式是否覆盖游戏全部纬格；使用：新版地基投影；边界：独立求和不包含需保留的已有设施。
func TestWholePlanetReformGrid(t *testing.T) {
	ctx := context.Background()
	placed := Object{Building: blueprint.Building{ItemID: 2203, ModelIndex: 53, InputObjIdx: -1, OutputObjIdx: -1}}
	pack, err := Build(ctx, Request{PlanetID: 101, Foundation: "planet", TerrainKnown: true, Objects: []Object{placed}})
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for row := 0; row < 250; row++ {
		segments, _ := geometry.LongitudeSegmentsAtGrid(row+1, geometry.Vanilla200)
		want += 2 * segments * 5
	}
	if pack.FoundationCells != want {
		t.Fatalf("整星地基格数量 %d，原版纬格总数 %d", pack.FoundationCells, want)
	}
	// 南纬 80 格的下边界属于经格更窄的相邻区；即使地基总数正确，放在本区仍会令原版整区标红。
	step, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	bands := latitudeBands(-249, 249)
	for i, b := range bands {
		if b.low != -80 {
			continue
		}
		if _, _, ok := reformGridPosition(b.low, 0, 0, float64(step), 0); ok {
			t.Fatal("跨经格宽度的地基被登记到错误的纬区")
		}
		south := bands[i-1]
		key, _, ok := reformGridPosition(south.low, south.high-south.low+1, 0, float64(step), 0)
		if !ok || key[1] != 330 {
			t.Fatalf("相邻纬区未接管南纬边界格：%v", key)
		}
		return
	}
	t.Fatal("缺少用于原版交界回归的南纬 80 格")
}

// TestAreaAnchorNeedsSharedGrid 职责：防止主经度在子纬带产生原版拒绝；使用：真实矿区失效回归；边界：只核对共用零旋转原点。
func TestAreaAnchorNeedsSharedGrid(t *testing.T) {
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	bands := latitudeBands(3, 132)
	anchor := Position{Latitude: float64(3 * float32(latitudeStep)), Longitude: .289026529}
	if err := validateAreaAnchor(anchor, bands, float64(latitudeStep)); err == nil {
		t.Fatal("真实旧矿区中的跨 Area 经度对齐缺陷未检出")
	}
	anchor.Longitude = 0
	if err := validateAreaAnchor(anchor, bands, float64(latitudeStep)); err != nil {
		t.Fatalf("真实共用经度格应可通过：%v", err)
	}
}

// TestUncertainCoastBlocksConstruction 职责：避免把岸线误差带当作可施工陆面；使用：地形边界回归；边界：不访问游戏。
func TestUncertainCoastBlocksConstruction(t *testing.T) {
	tower := stationObject(0, Position{}, 1001, 0, 2, false)
	_, err := Build(context.Background(), Request{PlanetID: 101, TerrainKnown: true, Objects: []Object{tower},
		wet: func(Position) (bool, bool) { return false, false }})
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_WATER" {
		t.Fatalf("未拒绝不确定岸线：%v", err)
	}
}

// TestRelayZoneFootprint 职责：检查黑雾区按游戏米保护占地边缘和线路中间；使用：候选及发布终检回归；边界：同星区域外仍可建造。
func TestRelayZoneFootprint(t *testing.T) {
	const radius = 40.0
	zone := Obstacle{Position: Position{}, Radius: radius, surfaceOnly: true}
	index := newObstacleIndex([]Obstacle{zone})
	longitude := func(meters float64) float64 { return 2 * math.Asin(meters/(2*200.2)) }
	high, _ := world(Position{Longitude: longitude(40.25), Height: 30})
	if !index.conflicts(high, .5) {
		t.Fatal("建筑中心在区域外但实际占地边缘相交")
	}
	clear, _ := world(Position{Longitude: longitude(41), Height: 30})
	if index.conflicts(clear, .5) {
		t.Fatal("区域外的独立地块不应被屏蔽")
	}
	left := Position{Longitude: longitude(39.6), Latitude: -.035}
	right := Position{Longitude: left.Longitude, Latitude: .035}
	leftWorld, _ := world(left)
	rightWorld, _ := world(right)
	if index.conflicts(leftWorld, .01) || index.conflicts(rightWorld, .01) {
		t.Fatal("线路样本端点应在黑雾保护区域之外")
	}
	if !index.surfaceSpanConflict(left, right, .01) {
		t.Fatal("遗漏中间跨越受保护区的线路")
	}
	tower := stationObject(0, Position{}, 1001, 0, 2, false)
	request := Request{PlanetID: 101, Objects: []Object{tower}, Obstacles: []Obstacle{zone}}
	_, err := Build(context.Background(), request)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_RELAY_OCCUPIED" {
		t.Fatalf("最终蓝图未报告已知基地占地：%v", err)
	}
	request.Objects[0].First = Position{Longitude: longitude(65)}
	request.Objects[0].Second = request.Objects[0].First
	if _, err := Build(context.Background(), request); err != nil {
		t.Fatalf("同星安全地块应可完成蓝图：%v", err)
	}
}

// TestRelayZoneProtectsFoundation 职责：验证两档地基都能保留黑雾已占区域；使用：地基打包回归；边界：区域外地基仍写出。
func TestRelayZoneProtectsFoundation(t *testing.T) {
	ctx := context.Background()
	latStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	bands := latitudeBands(-40, 40)
	areas := make([]blueprint.Area, len(bands))
	for i, band := range bands {
		areas[i].Width = int16(band.segments * 5)
	}
	tower := stationObject(0, Position{}, 1001, 0, 2, false)
	clear := stationObject(1, Position{Longitude: .4}, 1002, 0, 2, false)
	for _, mode := range []string{"construction", "planet"} {
		req := Request{PlanetID: 101, Foundation: mode, TerrainKnown: true, Objects: []Object{tower, clear}}
		_, all, err := foundation(ctx, req, bands, areas, float64(latStep), 0)
		if err != nil {
			t.Fatal(err)
		}
		req.Obstacles = []Obstacle{{Radius: 40, surfaceOnly: true}}
		_, remaining, err := foundation(ctx, req, bands, areas, float64(latStep), 0)
		if err != nil || remaining <= 0 || remaining >= all {
			t.Fatalf("地基模式 %s 未仅去掉基地附近格：未避让%d，已避让%d，%v", mode, all, remaining, err)
		}
	}
}

// TestRelayZoneMovesReceivingTower 职责：证明既有虚拟区域阻挡原选址但允许按真实落点重选；使用：逐星端口集成回归；边界：只读快照。
func TestRelayZoneMovesReceivingTower(t *testing.T) {
	ctx := context.Background()
	input := ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
			{Kind: "import", ItemID: 1101, PeerPlanetID: 102, RatePerMinute: 30},
			{Kind: "delivery", ItemID: 1101, RatePerMinute: 30},
		}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
	}
	control, err := Generate(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Resources.RelayZones = []saveanalysis.RelayZone{{PlanetID: 101, HiveAstroID: 1000001, RelayID: 1, Radius: 40, Depth: 60, Position: galaxy.Vector{Z: -200}}}
	protected, err := Generate(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if protected.Anchor == control.Anchor || protected.StationCount != 1 {
		t.Fatalf("预留区域未重新选址或漏掉收货塔：原定 %+v，新锚点 %+v", control.Anchor, protected.Anchor)
	}
	doc, err := blueprint.Decode(protected.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := ProjectDocument(ctx, doc, protected.Anchor, "核对收货塔")
	if err != nil {
		t.Fatal(err)
	}
	tower, _ := world(objects[0].First)
	zone := newObstacleIndex([]Obstacle{{Radius: 40, surfaceOnly: true}})
	if zone.surfaceConflict(tower, 1) {
		t.Fatal("重选收货塔仍落在黑雾区域")
	}
}

// TestLocalDroneItems 职责：仅报告实际本地供需对；使用：施工准备；边界：远程需求不能冒充蓝图内无人机流。
func TestLocalDroneItems(t *testing.T) {
	out := stationObject(0, Position{}, 1001, 1, 0, false)
	in := stationObject(1, Position{}, 1001, 2, 0, false)
	remoteSupply := stationObject(2, Position{}, 1002, 1, 0, false)
	remoteDemand := stationObject(3, Position{}, 1002, 0, 2, false)
	collector := blueprint.Building{Index: 4, ItemID: 2316, Parameters: []int32{1006, 1, 0, 10000, 0, 0}}
	localFuel := stationObject(5, Position{}, 1006, 2, 0, false)
	got, modes := localLogistics([]blueprint.Building{out.Building, in.Building, remoteSupply.Building, remoteDemand.Building, collector, localFuel.Building})
	if len(got) != 2 || got[0] != 1001 || got[1] != 1006 || modes[1001] != 3 || modes[1002] != 1 || modes[1006] != 3 {
		t.Fatalf("同星真实本地供需计算不符：%v、%v", got, modes)
	}
	configureRemoteGroup(&out.Building, 4)
	configureRemoteGroup(&remoteDemand.Building, 0)
	if out.Building.Parameters[334] != 0 || remoteDemand.Building.Parameters[332] != 0 || remoteDemand.Building.Parameters[334] != 4 {
		t.Fatal("纯本地塔保持原样，远程塔无专组也必须取指定模式")
	}
}

// TestMineOnlyBlueprint 职责：验证真实矿点驱动矿机到塔的端口链；使用：大小矿机回归；边界：采矿功率和速率按额定。
func TestMineOnlyBlueprint(t *testing.T) {
	for _, facility := range []int{2301, 2316} {
		ctx := context.Background()
		mine := industryplan.Mine{ItemID: 1001, FacilityID: facility, Kind: "solid", RatePerMinute: 30}
		if facility == 2316 {
			mine.SpeedMultiplier, mine.FacilityMultiplier = 1.2, 1.7
		}
		req := ProductionRequest{
			Plan: industryplan.PlanetPlan{PlanetID: 101,
				Mines: []industryplan.Mine{mine},
				Ports: []industryplan.Port{{ItemID: 1001, Kind: "mining", RatePerMinute: 30}, {ItemID: 1001, Kind: "export", PeerPlanetID: 102, RatePerMinute: 30}}},
			Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain(), Veins: []saveanalysis.PlanetVein{
				{ID: 1, ItemID: 1001, VeinType: 1, Amount: 5000, Position: galaxy.Vector{Z: -200}},
			}},
			Options: ProductionOptions{RemoteGroup: (1 << 45) | (1 << 3)},
		}
		got, err := Generate(ctx, req)
		if err != nil {
			t.Fatalf("原型 %d：%v", facility, err)
		}
		doc, err := blueprint.Decode(got.Blueprint, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Actual.Ports) != 2 || doc.Buildings[0].ItemID != int16(facility) || doc.Buildings[1].ItemID != 2104 ||
			doc.Buildings[1].Parameters[2] != 1 || doc.Buildings[2].InputObjIdx != doc.Buildings[0].Index ||
			doc.Buildings[len(doc.Buildings)-1].OutputObjIdx != doc.Buildings[1].Index {
			t.Fatalf("原型 %d 端口链缺失", facility)
		}
		if facility == 2316 && (doc.Buildings[0].Parameters[329] != 17000 || math.Abs(got.Mining[0].RatePerMinute-122.4) > 1e-8) {
			t.Fatalf("大型采矿机的原版运行档位或实际科技速率不符")
		}
		if got.Mining[0].CoveredItems == nil || *got.Mining[0].CoveredItems != 5000 {
			t.Fatal("未按实际矿点提供有限固矿剩余件数")
		}
		station := doc.Buildings[1]
		if station.Parameters[331] != 0 {
			t.Fatal("新建矿区物流塔不应从背包自动补满飞船")
		}
		if station.Parameters[332] != 8 || station.Parameters[333] != 8192 || station.Parameters[334] != 4 {
			t.Fatalf("原版塔的远程专组或指定优先字段不符")
		}
	}
}

// TestMineOnlyLocalDelivery 职责：核对直接采集硅石的本星交付；使用：整星生成回归；边界：从矿机沿编码后的输送带走到塔槽。
func TestMineOnlyLocalDelivery(t *testing.T) {
	const item = 1003
	delivery := industryplan.Port{ItemID: item, Kind: "delivery", RatePerMinute: 30}
	got, err := Generate(context.Background(), ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101,
			Mines: []industryplan.Mine{{ItemID: item, FacilityID: 2301, Kind: "solid", RatePerMinute: 30}},
			Ports: []industryplan.Port{{ItemID: item, Kind: "mining", RatePerMinute: 30}, delivery}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101},
			Terrain: dryTerrain(), Veins: []saveanalysis.PlanetVein{
				{ID: 1, ItemID: item, VeinType: 3, Amount: 5000, Position: galaxy.Vector{Z: -200}},
			}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Actual.Ports) != 2 || got.Actual.Ports[1] != delivery {
		t.Fatalf("缺少本星硅石交付：%+v", got.Actual.Ports)
	}
	doc, err := blueprint.Decode(got.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	miner, first := doc.Buildings[0], doc.Buildings[2]
	if miner.ItemID != 2301 || first.InputObjIdx != miner.Index {
		t.Fatal("硅石出矿带没有接至矿机")
	}
	belt := first
	for range len(doc.Buildings) {
		if belt.ItemID < 2001 || belt.ItemID > 2003 || belt.OutputObjIdx < 0 || int(belt.OutputObjIdx) >= len(doc.Buildings) {
			t.Fatal("硅石交付输送链断开")
		}
		next := doc.Buildings[belt.OutputObjIdx]
		if next.ItemID == 2104 {
			if next.Parameters[0] != item || next.Parameters[1] != 1 || next.Parameters[2] != 0 ||
				next.Parameters[3] <= 0 || next.Parameters[192+4*int(belt.OutputToSlot)] != 2 {
				t.Fatal("硅石输送链未接入本地物流塔的有效库存槽")
			}
			return
		}
		belt = next
	}
	t.Fatal("硅石输送链未连接本地物流塔")
}

// TestOilAndWaterTransport 职责：证明原油和液体原型的真实带链；使用：资源装配回归；边界：液面样例用独立半球岸线函数。
func TestOilAndWaterTransport(t *testing.T) {
	ctx := context.Background()
	oilRate := 2.0
	buriedRate := 20.0
	oil, err := Generate(ctx, ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Mines: []industryplan.Mine{{ItemID: 1007, FacilityID: 2307, Kind: "oil", RatePerMinute: 60}},
			Ports: []industryplan.Port{{ItemID: 1007, Kind: "mining", RatePerMinute: 60}}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain(),
			Veins: []saveanalysis.PlanetVein{
				{ID: 4, ItemID: 1007, VeinType: 7, Position: galaxy.Vector{Z: -150}, Amount: 14400000, Rate: &buriedRate},
				{ID: 15, ItemID: 1007, VeinType: 7, Position: galaxy.Vector{Z: -200}, Amount: 144000, Rate: &oilRate},
			}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oil.ItemCounts[2307] != 1 || oil.Mining[0].VeinIDs[0] != 15 || oil.Mining[0].RatePerMinute != 120 {
		t.Fatalf("原油萃取站缺少真实井口速率：%+v", oil.Mining)
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	water := int64(1116)
	req := Request{PlanetID: 101, TerrainKnown: true, wet: func(p Position) (bool, bool) { return math.Sin(p.Longitude) < 0, true }}
	liquid := ProductionRequest{Resources: saveanalysis.PlanetResourceDetail{Terrain: saveanalysis.PlanetTerrain{WaterItemID: &water}}}
	mine := industryplan.Mine{ItemID: 1116, FacilityID: 2306, SpeedMultiplier: 1}
	var shore liquidSiteCursor
	pump, rate, err := findLiquid(ctx, req, liquid, mine, catalog, true, &shore)
	if err != nil || len(pump) < 3 || rate != 50 {
		t.Fatalf("硫酸泵找不到可核对岸线：数量 %d，速率 %f，错误 %v", len(pump), rate, err)
	}
	req.Objects = pump
	result, err := Build(ctx, req)
	if err != nil || result.ItemCounts[2306] != 1 {
		t.Fatalf("硫酸泵没有完整原版输出：%v", err)
	}
	// 在第一台泵、转运塔和带路已经占地后，续查岸线须与从头扫描得到完全相同的第二台。
	next, nextRate, err := findLiquid(ctx, req, liquid, mine, catalog, true, &shore)
	if err != nil || len(next) == 0 {
		t.Fatalf("找不到第二台硫酸泵：%v", err)
	}
	fromStart, fullRate, err := findLiquid(ctx, req, liquid, mine, catalog, true, &liquidSiteCursor{})
	if err != nil || nextRate != fullRate || !reflect.DeepEqual(next, fromStart) {
		t.Fatalf("续查与全查得到不同的泵、带路或速率：%v", err)
	}
}

// TestOilWellThreeDimensionalGate 职责：核对同经纬深埋井及原版横向容差；使用：油井选址回归；边界：不启动游戏。
func TestOilWellThreeDimensionalGate(t *testing.T) {
	center, err := world(Position{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		point placement.Vector
		want  int
	}{
		{"surface", placement.Vector{0, 0, -200}, 0},
		{"within-transverse-threshold", placement.Vector{.31, 0, -200}, 0},
		{"outside-transverse-threshold", placement.Vector{.32, 0, -200}, -1},
		{"within-neighborhood", placement.Vector{0, 0, -209.9}, 0},
		{"outside-neighborhood", placement.Vector{0, 0, -210.3}, -1},
		{"same-direction-buried", placement.Vector{0, 0, -150}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := oilWellAt(center, []veinPoint{{kind: 7, v: tc.point}}); got != tc.want {
				t.Fatalf("原版井口三维验证得到 %d，预期 %d", got, tc.want)
			}
		})
	}
}

// TestPowerBlueprint 职责：验证火电煤路原版输出；使用：能源端口回归；边界：暂不启动游戏。
func TestPowerBlueprint(t *testing.T) {
	ctx := context.Background()
	got, err := Generate(ctx, ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Power: &industryplan.PowerPlan{
			FacilityID: 2204, FuelItemID: 1006, Count: 1, FuelRatePerMinute: 60, PowerMW: 2.16,
		}, Ports: []industryplan.Port{{ItemID: 1006, Kind: "power-fuel", RatePerMinute: 60}, {ItemID: 1006, Kind: "external", RatePerMinute: 60}}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(got.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2204] != 1 || doc.Summary.ItemCounts[2013] != 1 || doc.Summary.ItemCounts[2201] < 1 {
		t.Fatalf("火电建筑、进煤分拣器及配电塔不完整：%+v，%v", doc.Summary, err)
	}
}

// TestProductionBlueprint 职责：运行现有量化制造链和原版跨球角色；使用：整星制造接入回归；边界：样例为单熔炉铁锭。
func TestProductionBlueprint(t *testing.T) {
	ctx := context.Background()
	plan, err := quant.Solve(ctx, quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Generate(ctx, ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Plan: plan,
			Ports: []industryplan.Port{{ItemID: 1001, Kind: "import", RatePerMinute: 30, PeerPlanetID: 102}, {ItemID: 1101, Kind: "delivery", RatePerMinute: 30}}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
		Options:   ProductionOptions{Profile: "low"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.StationCount == 0 || len(got.Actual.Ports) != 2 {
		t.Fatalf("缺少实物物流证据：%+v", got.Actual)
	}
}

// TestProductionPowerWithManualCoal 职责：确认只为火电直通的煤不会阻断制造；使用：独立火电与制造集成回归；边界：煤由玩家供给。
func TestProductionPowerWithManualCoal(t *testing.T) {
	ctx := context.Background()
	plan, err := quant.Solve(ctx, quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}})
	if err != nil {
		t.Fatal(err)
	}
	plan.RawInputs = append(plan.RawInputs, quant.ItemRate{ItemID: 1006, RatePerMinute: 60})
	plan.Targets = append(plan.Targets, quant.Target{ItemID: 1006, RatePerMinute: 60})
	plan.ExternalInputItemIDs = append(plan.ExternalInputItemIDs, 1006)
	plan.FlowGraph = nil
	result, err := Generate(ctx, ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Plan: plan, Power: &industryplan.PowerPlan{
			FacilityID: 2204, FuelItemID: 1006, Count: 1, FuelRatePerMinute: 60, PowerMW: 2.16,
		}, Ports: []industryplan.Port{
			{ItemID: 1001, Kind: "external", RatePerMinute: 30}, {ItemID: 1101, Kind: "delivery", RatePerMinute: 30},
			{ItemID: 1006, Kind: "external", RatePerMinute: 60}, {ItemID: 1006, Kind: "power-fuel", RatePerMinute: 60},
		}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Actual.Power == nil || result.Actual.Power.Count != 1 || result.StationCount < 2 || len(result.Actual.Ports) != 4 {
		t.Fatalf("制造和手动供煤没有闭合：%+v", result.Actual)
	}
}

// dryTerrain 职责：提供明确无液面的合成星；使用：几何回归；边界：独立于存档地形解析。
func dryTerrain() saveanalysis.PlanetTerrain {
	water, height := int64(0), float64(0)
	return saveanalysis.PlanetTerrain{WaterSource: "generated", WaterItemID: &water, WaterHeight: &height}
}
