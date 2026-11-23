package planet

// 文件职责：验证共享塔接收器模块的真实端口、带链和碰撞；使用：光子及发电模式回归；边界：独立校验原版蓝图，不推定游戏外来料已经存在。

import (
	"context"
	"fmt"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// TestEnergyReceiverLastBatchCanUseEarlierSite 验证较小的尾批能使用同纬度之前被三台机组排除的空位。
func TestEnergyReceiverLastBatchCanUseEarlierSite(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	step, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	req := Request{PlanetID: 101}
	// 在三台机组的北侧接收器行布置一段障碍；赤道第一格仍容得下一台。
	for x := 0; x <= 100; x += 2 {
		req.Obstacles = append(req.Obstacles, Obstacle{Position: Position{Longitude: float64(x) * float64(step), Latitude: 10 * float64(step)}, Radius: 1.2})
	}
	if err := placeEnergyUnits(context.Background(), &req, ProductionRequest{}, catalog, 2208, 4, 1208, 0, 1208); err != nil {
		t.Fatal(err)
	}
	var centers []Position
	for _, o := range req.Objects {
		if o.Building.ItemID == 2208 {
			centers = append(centers, o.First)
		}
	}
	if len(centers) != 4 || centers[0].Longitude == 0 || centers[3] != (Position{}) {
		t.Fatalf("先摆三台后，尾批单台应回到赤道首格：%+v", centers)
	}
}

// TestEnergyReceiverBatchBlueprint 职责：核对三台机组共享塔的每条实体带路；使用：多纬带和工作模式回归；边界：一条带只能服务一个设备端口。
func TestEnergyReceiverBatchBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	for _, tc := range []struct {
		name                     string
		mode, incoming, outgoing int
	}{{"photon-lens", 1208, 1209, 1208}, {"photon", 1208, 0, 1208}, {"electric-lens", 0, 1209, 0}, {"electric", 0, 0, 0}} {
		for _, row := range []int{0, 35, -35, 65, 130, 200, 210, -210} {
			t.Run(fmt.Sprintf("%s/row%d", tc.name, row), func(t *testing.T) {
				p := Position{Longitude: .3, Latitude: float64(float32(row) * float32(latitudeStep))}
				step, _ := geometry.PasteLongitudeStep32(geometry.Radians(p.Latitude), geometry.Vanilla200)
				plan := industryplan.PlanetPlan{Ports: []industryplan.Port{{ItemID: 1209, Kind: "import"}, {ItemID: 1208, Kind: "export"}}}
				objects, err := energyReceiverBatch(17, p, float64(step), 3, tc.mode, tc.incoming, tc.outgoing, plan, catalog)
				if err != nil {
					t.Fatal(err)
				}
				built, err := Build(context.Background(), Request{PlanetID: 101, Objects: objects})
				if err != nil {
					t.Fatalf("共享接收器模块不满足原生几何：%v", err)
				}
				doc, err := blueprint.Decode(built.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				towers := 0
				if tc.incoming != 0 {
					towers++
				}
				if tc.outgoing != 0 {
					towers++
				}
				if doc.Summary.ItemCounts[2208] != 3 || doc.Summary.ItemCounts[2104] != towers {
					t.Fatalf("三台接收器的共享塔数量不符：%+v", doc.Summary.ItemCounts)
				}
				for _, b := range doc.Buildings[:3] {
					if b.ItemID != 2208 || b.ModelIndex != 73 || len(b.Parameters) != 1 || b.Parameters[0] != int32(tc.mode) {
						t.Fatalf("接收器原型或工作模式错误：%+v", b)
					}
				}
				visited := map[int32]bool{}
				if tc.incoming != 0 {
					assertReceiverBatchPaths(t, doc.Buildings, 1209, true, [3]int8{10, 11, 9}, visited)
				}
				if tc.outgoing != 0 {
					assertReceiverBatchPaths(t, doc.Buildings, 1208, false, [3]int8{4, 3, 5}, visited)
				}
				if len(visited) != doc.Summary.ItemCounts[2003] {
					t.Fatalf("存在没有接通到接收器的传送带：已接 %d / 总数 %d", len(visited), doc.Summary.ItemCounts[2003])
				}
			})
		}
	}
}

// TestEnergyReceiverBatchRemainder 职责：保持最后一组的真实台数及塔口数；使用：总数不能被三整除的光子计划；边界：不为未装设备打开多余侧口。
func TestEnergyReceiverBatchRemainder(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	step, _ := geometry.PasteLongitudeStep32(0, geometry.Vanilla200)
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			objects, err := energyReceiverBatch(11, Position{}, float64(step), count, 1208, 1209, 1208, industryplan.PlanetPlan{}, catalog)
			if err != nil {
				t.Fatal(err)
			}
			built, err := Build(context.Background(), Request{PlanetID: 101, Objects: objects})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Summary.ItemCounts[2208] != count || doc.Summary.ItemCounts[2104] != 2 {
				t.Fatalf("余数组装多建或漏建设备：%+v", doc.Summary.ItemCounts)
			}
			for _, b := range doc.Buildings {
				if b.ItemID != 2104 {
					continue
				}
				ports := 0
				for p := 0; p < 12; p++ {
					if b.Parameters[192+p*4] != 0 {
						ports++
					}
				}
				if ports != count || b.Parameters[2] != 0 {
					t.Fatalf("余数模块的塔口数或纯本地角色错误：%d 台，%d 口，远程 %d", count, ports, b.Parameters[2])
				}
			}
		})
	}
}

// assertReceiverBatchPaths 职责：从每个原型口追踪不共享的带链；使用：共享塔模块验证；边界：物流塔的不同原生端口才允许分流。
func assertReceiverBatchPaths(t *testing.T, buildings []blueprint.Building, item int32, input bool, ports [3]int8, visited map[int32]bool) {
	t.Helper()
	tower := int32(-1)
	for _, b := range buildings {
		if b.ItemID == 2104 && b.Parameters[0] == item {
			if tower >= 0 {
				t.Fatal("同方向没有共享唯一一座物流塔")
			}
			tower = b.Index
			wantLocal, wantRemote, portMode := int32(1), int32(1), int32(2)
			if input {
				wantLocal, wantRemote, portMode = 2, 2, 1
			}
			if b.Parameters[1] != wantLocal || b.Parameters[2] != wantRemote {
				t.Fatal("共享塔的本地/远程供需角色不符")
			}
			for _, port := range ports {
				if b.Parameters[192+int(port)*4] != portMode || input && b.Parameters[193+int(port)*4] != 1 {
					t.Fatalf("共享塔侧口 %d 没有配置到对应物品槽", port)
				}
			}
		}
	}
	if tower < 0 {
		t.Fatalf("物品 %d 缺少共享塔", item)
	}
	for lane, port := range ports {
		from, to, fromPort, toPort := int32(lane), tower, int8(0), port
		if input {
			from, to, fromPort, toPort = tower, int32(lane), port, 1
		}
		next := int32(-1)
		for _, b := range buildings {
			if b.ItemID == 2003 && b.InputObjIdx == from && b.InputFromSlot == fromPort {
				if next >= 0 {
					t.Fatal("同一个设施端口被虚构为多路输出")
				}
				next = b.Index
			}
		}
		filtered := false
		for next != to {
			if next < 0 || int(next) >= len(buildings) || visited[next] || buildings[next].ItemID != 2003 {
				t.Fatalf("第 %d 条带链断开、循环或与别路共享：%d", lane, next)
			}
			visited[next] = true
			b := buildings[next]
			filtered = filtered || len(b.Parameters) > 0 && b.Parameters[0] == item
			if b.OutputObjIdx == to && b.OutputToSlot != toPort {
				t.Fatalf("第 %d 条带链接错设备端口", lane)
			}
			next = b.OutputObjIdx
		}
		if !filtered {
			t.Fatalf("第 %d 条带链缺少物流物品筛选", lane)
		}
	}
}

// TestEnergyReceiverBatchRejectsModelOverlap 职责：确认模型碰撞真正进入模块校验；使用：接收器间距证据；边界：仅改变一台设备位置，合法模块由正例覆盖。
func TestEnergyReceiverBatchRejectsModelOverlap(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	step, _ := geometry.PasteLongitudeStep32(0, geometry.Vanilla200)
	objects, err := energyReceiverBatch(0, Position{}, float64(step), 3, 0, 0, 0, industryplan.PlanetPlan{}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	objects[1].First, objects[1].Second = objects[0].First, objects[0].Second
	if _, err := Build(context.Background(), Request{PlanetID: 101, Objects: objects}); err == nil {
		t.Fatal("重叠接收器未被真实模型碰撞校验拒绝")
	}
}
