package planet

// 文件职责：核对能源模块的原型模式、实际物流和地表落位；使用：特殊蓝图回归；边界：不把编解码验证替代游戏实贴。

import (
	"context"
	"fmt"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestEnergyPortBlockBlueprint 职责：验证接收器和枢纽的实装双向物流；使用：赤道及非赤道模式回归；边界：端口必须直接接带。
func TestEnergyPortBlockBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	cases := []struct {
		name                               string
		facility, mode, incoming, outgoing int
		model                              int16
		inputPort                          int8
	}{
		{"photon", 2208, 1208, 0, 1208, 73, 1},
		{"photon-lens", 2208, 1208, 1209, 1208, 73, 1},
		{"electric-lens", 2208, 0, 1209, 0, 73, 1},
		{"charge", 2209, 1, 2206, 2207, 45, 2},
		{"discharge", 2209, -1, 2207, 2206, 45, 2},
	}
	for _, tc := range cases {
		for _, row := range []int{0, 35} {
			t.Run(fmt.Sprintf("%s/row%d", tc.name, row), func(t *testing.T) {
				position := Position{Longitude: .3, Latitude: float64(float32(row) * float32(latitudeStep))}
				step, _ := geometry.PasteLongitudeStep32(geometry.Radians(position.Latitude), geometry.Vanilla200)
				plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
					{ItemID: tc.incoming, Kind: "import", PeerPlanetID: 102, RatePerMinute: 1},
					{ItemID: tc.outgoing, Kind: "export", PeerPlanetID: 102, RatePerMinute: 6},
				}}
				objects, err := energyPortBlock(11, position, float64(step), tc.facility, tc.mode, tc.incoming, tc.outgoing, plan, catalog)
				if err != nil {
					t.Fatal(err)
				}
				built, err := Build(context.Background(), Request{PlanetID: 101, Objects: objects})
				if err != nil {
					t.Fatalf("能源模块无法生成实体蓝图：%v", err)
				}
				doc, err := blueprint.Decode(built.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				device := doc.Buildings[0]
				if device.ItemID != int16(tc.facility) || device.ModelIndex != tc.model || len(device.Parameters) != 1 || device.Parameters[0] != int32(tc.mode) {
					t.Fatalf("能源设施的原型或工作模式不符：%+v", device)
				}
				stations, connected := 0, map[int32]bool{}
				for _, io := range []struct {
					item int
					in   bool
				}{{tc.incoming, true}, {tc.outgoing, false}} {
					if io.item == 0 {
						continue
					}
					stations++
					port := int8(0)
					if io.in {
						port = tc.inputPort
					}
					energyAssertBeltPath(t, doc.Buildings, device.Index, io.item, io.in, port, connected)
				}
				if doc.Summary.ItemCounts[int16(tc.facility)] != 1 || doc.Summary.ItemCounts[2104] != stations || doc.Summary.ItemCounts[2003] != len(connected) {
					t.Fatalf("蓝图含缺失或游离的能源物流设施：%+v，已接带 %d", doc.Summary.ItemCounts, len(connected))
				}
				for _, b := range doc.Buildings {
					if b.ItemID != int16(tc.facility) && b.ItemID != 2104 && b.ItemID != 2003 {
						t.Fatalf("能源直连模块混入非带口设施 %d", b.ItemID)
					}
				}
			})
		}
	}
}

// energyAssertBeltPath 职责：独立追踪编解码后的物品路径；使用：能源模块实连接断言；边界：不按靠近设备推定已经连接。
func energyAssertBeltPath(t *testing.T, buildings []blueprint.Building, device int32, item int, input bool, devicePort int8, visited map[int32]bool) {
	t.Helper()
	byIndex := map[int32]blueprint.Building{}
	tower := int32(-1)
	for _, b := range buildings {
		byIndex[b.Index] = b
		if b.ItemID == 2104 && len(b.Parameters) == 2048 && b.Parameters[0] == int32(item) {
			if tower >= 0 {
				t.Fatalf("物品 %d 重复使用物流塔", item)
			}
			tower = b.Index
			wantLocal, wantRemote, port, mode := int32(1), int32(1), 4, int32(2)
			if input {
				wantLocal, wantRemote, port, mode = 2, 2, 10, 1
			}
			if b.Parameters[1] != wantLocal || b.Parameters[2] != wantRemote || b.Parameters[192+port*4] != mode {
				t.Fatalf("物品 %d 的物流塔供需或带口方向不符", item)
			}
		}
	}
	if tower < 0 {
		t.Fatalf("物品 %d 缺少真实物流塔", item)
	}
	start, end := device, tower
	startPort, endPort := devicePort, int8(4)
	if input {
		start, end, startPort, endPort = tower, device, 10, devicePort
	}
	next := int32(-1)
	for _, b := range buildings {
		if b.ItemID == 2003 && b.InputObjIdx == start {
			if next >= 0 || b.InputFromSlot != startPort {
				t.Fatalf("物品 %d 的带路起点或原型端口不唯一", item)
			}
			next = b.Index
		}
	}
	if next < 0 {
		t.Fatalf("物品 %d 没有从设施 %d 引出的带", item, start)
	}
	filtered := false
	for next != end {
		b, ok := byIndex[next]
		if !ok || b.ItemID != 2003 || visited[next] {
			t.Fatalf("物品 %d 的带路断开、串线或循环于 %d", item, next)
		}
		visited[next] = true
		filtered = filtered || len(b.Parameters) > 0 && b.Parameters[0] == int32(item)
		if b.OutputObjIdx == end && b.OutputToSlot != endPort {
			t.Fatalf("物品 %d 的带路没有接到原型端口 %d", item, endPort)
		}
		next = b.OutputObjIdx
	}
	if !filtered {
		t.Fatalf("物品 %d 的塔带口缺少实际物品筛选", item)
	}
}

// TestEnergyUnitSpacingBlueprint 职责：验证无燃料模块实物排布和风机距离；使用：多台落位回归；边界：太阳能和蓄电器的碰撞交给原生几何校验。
func TestEnergyUnitSpacingBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, facility := range []int{2203, 2205, 2206} {
		t.Run(fmt.Sprint(facility), func(t *testing.T) {
			const count = 6
			req := Request{PlanetID: 101}
			input := ProductionRequest{}
			if facility == 2203 {
				p, _ := world(Position{})
				input.Resources.Buildings = []saveanalysis.PlanetBuilding{{ItemID: 2203, Position: galaxy.Vector{X: float64(p.X), Y: float64(p.Y), Z: float64(p.Z)}}}
			}
			if err := placeEnergyUnits(context.Background(), &req, input, catalog, facility, count, 0, 0, 0); err != nil {
				t.Fatal(err)
			}
			if len(req.Objects) != count {
				t.Fatalf("期望 %d 台实物，得到 %d", count, len(req.Objects))
			}
			for i, obj := range req.Objects {
				if obj.Building.ItemID != int16(facility) {
					t.Fatalf("布置了错误设施：%d", obj.Building.ItemID)
				}
				if facility == 2203 {
					if energyWorldDistance(obj.First, Position{}) < 10.5 {
						t.Fatal("新风机没有避让存档中的风机")
					}
					for _, old := range req.Objects[:i] {
						if energyWorldDistance(obj.First, old.First) < 10.5 {
							t.Fatal("新风机间距小于原版 10.5 米")
						}
					}
				}
			}
			built, err := Build(context.Background(), req)
			if err != nil {
				t.Fatalf("多台能源设施几何冲突：%v", err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil || doc.Summary.ItemCounts[int16(facility)] != count {
				t.Fatalf("能源设施蓝图不完整：%+v，%v", doc.Summary.ItemCounts, err)
			}
		})
	}
}

// energyWorldDistance 职责：用球面端点复核真实间隔；使用：能源布置回归；边界：只处理已知合法端点。
func energyWorldDistance(a, b Position) float64 {
	p, _ := world(a)
	q, _ := world(b)
	return math.Hypot(math.Hypot(float64(p.X-q.X), float64(p.Y-q.Y)), float64(p.Z-q.Z))
}

// energyLavaTerrain 职责：构造有真实高度网格的均匀熔岩地表；使用：地热采样和终检；边界：使用原版 200 精度，避免气态星球捷径。
func energyLavaTerrain(height uint16) saveanalysis.PlanetTerrain {
	water, level := int64(-1), 0.0
	heights := make([]uint16, 402*402)
	for i := range heights {
		heights[i] = height
	}
	return saveanalysis.PlanetTerrain{WaterItemID: &water, WaterHeight: &level, HeightSource: "generated", HeightPrecision: 200, HeightCentimeters: heights}
}

// TestEnergyGeothermalLavaBlueprint 职责：验证熔岩候选可通过最终蓝图校验；使用：地热真实地形回归；边界：已选落点仍须保留三处采样地表。
func TestEnergyGeothermalLavaBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := ProductionRequest{Resources: saveanalysis.PlanetResourceDetail{Terrain: energyLavaTerrain(19900)}}
	req := Request{PlanetID: 101, TerrainKnown: true}
	req.wet = func(p Position) (bool, bool) {
		v, err := world(p)
		if err != nil {
			return false, false
		}
		height, known := input.Resources.Terrain.HeightAt(galaxy.Vector{X: float64(v.X), Y: float64(v.Y), Z: float64(v.Z)})
		return height < *input.Resources.Terrain.WaterHeight, known
	}
	if err := placeEnergyUnits(context.Background(), &req, input, catalog, 2213, 2, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(req.Protected) != 6 || energyWorldDistance(req.Objects[0].First, req.Objects[1].First) < 12 {
		t.Fatal("地热模块缺少三点保护或未满足 12 米间距")
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatalf("合法熔岩上的地热模块被最终蓝图校验拒绝：%v", err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2213] != 2 {
		t.Fatalf("地热实物蓝图不完整：%+v，%v", doc.Summary.ItemCounts, err)
	}
}

// TestEnergyGeothermalSamplingAndFoundation 职责：核对地热三处原型点和地基保护；使用：浅熔岩与施工回归；边界：保护点不能因模块已落位而被填平。
func TestEnergyGeothermalSamplingAndFoundation(t *testing.T) {
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	for _, row := range []int{0, 35} {
		position := Position{Longitude: .3, Latitude: float64(float32(row) * float32(latitudeStep))}
		for _, tc := range []struct {
			height uint16
			valid  bool
		}{{19900, true}, {19967, true}, {19969, false}, {20000, false}} {
			input := ProductionRequest{Resources: saveanalysis.PlanetResourceDetail{Terrain: energyLavaTerrain(tc.height)}}
			if geothermalSite(input, position) != tc.valid {
				t.Errorf("纬格 %d 高程 %d 的地热水深条件错误", row, tc.height)
			}
		}
		points := geothermalPoints(position)
		if len(points) != 3 {
			t.Fatalf("原型应有三处熔岩采样点，实际 %d", len(points))
		}
		for i, want := range [][2]float64{{0, -1.3}, {.5, 1.3}, {-.5, 1.3}} {
			dx := (points[i].Longitude - position.Longitude) * 200 * math.Cos(position.Latitude)
			dz := (points[i].Latitude - position.Latitude) * 200
			if math.Abs(dx-want[0]) > 1e-6 || math.Abs(dz-want[1]) > 1e-6 {
				t.Fatalf("地热采样点 %d 偏离原型：%.6f, %.6f", i, dx, dz)
			}
		}
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := ProductionRequest{Resources: saveanalysis.PlanetResourceDetail{Terrain: energyLavaTerrain(19900)}}
	req := Request{PlanetID: 101, Foundation: "construction", TerrainKnown: true}
	if err := placeEnergyUnits(context.Background(), &req, input, catalog, 2213, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(req.Protected) != 3 {
		t.Fatalf("地热施工必须登记三处保护点，实际 %d", len(req.Protected))
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	unprotected := req
	unprotected.Protected = nil
	filled, err := Build(context.Background(), unprotected)
	if err != nil || filled.FoundationCells <= built.FoundationCells {
		t.Fatalf("三处采样点没有从施工地基扣除：保护 %d，无保护 %d，%v", built.FoundationCells, filled.FoundationCells, err)
	}
	if doc.Reform == nil {
		t.Fatal("没有生成可核对的施工地基")
	}
	for _, rect := range doc.Reform.Rects {
		area := doc.Areas[rect.AreaIndex]
		areaRow := int(math.Round(built.Anchor.Latitude / float64(latitudeStep)))
		for area.ParentIndex >= 0 {
			areaRow += int(area.AnchorLocalOffsetY)
			area = doc.Areas[area.ParentIndex]
		}
		for y := int(rect.Y); y < int(rect.Y)+int(rect.H); y++ {
			for x := int(rect.X); x < int(rect.X)+int(rect.W); x++ {
				_, p, known := reformGridPosition(areaRow, y, x, float64(latitudeStep), built.Anchor.Longitude)
				if !known {
					t.Fatal("编码后的地基格不能投影到星球")
				}
				for _, protected := range req.Protected {
					if energyWorldDistance(p, Position{Longitude: protected.Longitude, Latitude: protected.Latitude}) < 1.6 {
						t.Fatal("施工地基覆盖了地热熔岩采样点")
					}
				}
			}
		}
	}
}
