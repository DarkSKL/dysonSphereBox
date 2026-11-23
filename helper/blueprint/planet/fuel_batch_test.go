package planet

// 文件职责：验证共享燃料塔的实际机组、带链和负荷上限；使用：电厂占地回归；边界：不跳过原版几何、供电和端口校验。

import (
	"context"
	"fmt"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/transport"
)

// TestFuelBatchBlueprint 职责：核对多台电厂共享塔后的真实带路与原版几何；使用：不同机型和南北纬；边界：每个分拣器必须沿同一无分叉燃料带追溯到塔。
func TestFuelBatchBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	factCatalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	for _, tc := range []struct{ facility, fuel, count, level int }{{2210, 1803, 8, 3}, {2210, 1804, 3, 3}, {2211, 1802, 8, 3}, {2204, 1006, 6, 3}, {2204, 1006, 6, 1}} {
		for _, row := range []int{0, 65, -65, 160, -160} {
			t.Run(fmt.Sprintf("%d-%d-Mk%d/%d", tc.facility, tc.fuel, tc.level, row), func(t *testing.T) {
				facility, _ := factCatalog.Lookup(tc.facility)
				fuel, _ := factCatalog.Lookup(tc.fuel)
				_, perPlant, err := fuelGeneratorRating(facility, fuel)
				if err != nil {
					t.Fatal(err)
				}
				armID := int16(2010 + tc.level)
				start := Position{Longitude: .3, Latitude: float64(float32(row) * float32(latitudeStep))}
				step, _ := geometry.PasteLongitudeStep32(geometry.Radians(start.Latitude), geometry.Vanilla200)
				objects, err := fuelBatch(17, start, float64(step), tc.count, facility, int32(tc.fuel), tc.level, 2, false, catalog)
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
				if doc.Summary.ItemCounts[int16(tc.facility)] != tc.count || doc.Summary.ItemCounts[2104] != 1 || doc.Summary.ItemCounts[armID] != tc.count {
					t.Fatalf("共享燃料模块实体错误：%v", doc.Summary.ItemCounts)
				}
				byIndex := map[int32]blueprint.Building{}
				var tower blueprint.Building
				for _, b := range doc.Buildings {
					byIndex[b.Index] = b
					if b.ItemID == 2104 {
						tower = b
					}
				}
				if tower.Parameters[0] != int32(tc.fuel) || tower.Parameters[1] != 2 || tower.Parameters[2] != 2 || tower.Parameters[192+10*4] != 1 || tower.Parameters[193+10*4] != 1 {
					t.Fatal("燃料塔的物品、供需或侧口错误")
				}
				connected := map[int32]bool{}
				next := int32(-1)
				for _, b := range doc.Buildings {
					if b.ItemID == 2001 && b.InputObjIdx == tower.Index {
						if next >= 0 {
							t.Fatal("燃料塔伪造同口分叉")
						}
						next = b.Index
						if b.InputFromSlot != 10 || len(b.Parameters) == 0 || b.Parameters[0] != int32(tc.fuel) {
							t.Fatal("真实侧口或筛选缺失")
						}
					}
				}
				for next >= 0 {
					b, ok := byIndex[next]
					if !ok || b.ItemID != 2001 || connected[next] {
						t.Fatal("燃料带断开或成环")
					}
					connected[next] = true
					next = b.OutputObjIdx
				}
				if len(connected) != doc.Summary.ItemCounts[2001] {
					t.Fatal("存在未接通的燃料带")
				}
				fed := map[int32]bool{}
				for _, b := range doc.Buildings {
					if b.ItemID != armID {
						continue
					}
					target := byIndex[b.OutputObjIdx]
					if !connected[b.InputObjIdx] || target.ItemID != int16(tc.facility) || fed[target.Index] || b.FilterID != int16(tc.fuel) || b.Parameters[0] < 1 || b.Parameters[0] > 3 {
						t.Fatal("电厂分拣器没有独立接到实际燃料带")
					}
					center, _ := world(objects[target.Index].First)
					first, _ := world(objects[b.Index].First)
					second, _ := world(objects[b.Index].Second)
					across, err := geometry.SegmentsAcross32(center, first, second)
					if err != nil {
						t.Fatal(err)
					}
					span := max(1, min(3, int(math.RoundToEven(float64(across)))))
					rate, err := transport.InserterCapacityPerMinute(tc.level, span)
					if err != nil || int(b.Parameters[0]) != span || rate < perPlant {
						t.Fatalf("分拣器 %d 的真实跨度 %.3f 格、记录档位 %d、额定 %.1f 件/分钟，机组需要 %.1f", b.Index, across, b.Parameters[0], rate, perPlant)
					}
					fed[target.Index] = true
				}
				if len(fed) != tc.count {
					t.Fatal("有电厂缺少实际燃料入口")
				}
			})
		}
	}
}

// TestFuelBatchGeneration 职责：通过完整入口核对批量供电与真实物流吞吐；使用：五十二台恒星和两种爪速的煤带上限；边界：塔数减少不能虚增燃料或额定电力。
func TestFuelBatchGeneration(t *testing.T) {
	factCatalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                                 string
		facility, fuel, count, level, towers int
	}{{"stars-52", 2210, 1803, 52, 3, 7}, {"coal-belt-limit", 2204, 1006, 7, 3, 2}, {"coal-mk1", 2204, 1006, 7, 1, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			facility, _ := factCatalog.Lookup(tc.facility)
			fuel, _ := factCatalog.Lookup(tc.fuel)
			powerMW, rate, err := fuelGeneratorRating(facility, fuel)
			if err != nil {
				t.Fatal(err)
			}
			power := industryplan.PowerPlan{FacilityID: tc.facility, FuelItemID: tc.fuel, Count: tc.count, PowerMW: powerMW * float64(tc.count), FuelRatePerMinute: rate * float64(tc.count)}
			ports := []industryplan.Port{{ItemID: tc.fuel, Kind: "import", PeerPlanetID: 102, RatePerMinute: power.FuelRatePerMinute}, {ItemID: tc.fuel, Kind: "power-fuel", RatePerMinute: power.FuelRatePerMinute}}
			result, err := Generate(context.Background(), ProductionRequest{
				Plan:      industryplan.PlanetPlan{PlanetID: 101, Power: &power, Ports: ports},
				Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
				Options:   ProductionOptions{Setting: build.Setting{InserterLv: tc.level}},
			})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Summary.ItemCounts[int16(tc.facility)] != tc.count || doc.Summary.ItemCounts[2104] != tc.towers {
				t.Fatalf("真实机组或共享塔数不符：%v", doc.Summary.ItemCounts)
			}
			if result.Actual.Power == nil || result.Actual.Power.Count != tc.count || math.Abs(result.Actual.Power.PowerMW-power.PowerMW) > 1e-6 || result.Actual.Power.RequiredMW > power.PowerMW || math.Abs(result.Actual.Power.FuelRatePerMinute-power.FuelRatePerMinute) > 1e-6 {
				t.Fatalf("批量机组改变供电或燃料合同：%+v", result.Actual.Power)
			}
			predecessor := map[int32]int32{}
			for _, b := range doc.Buildings {
				if b.ItemID == 2001 {
					if b.InputObjIdx >= 0 {
						predecessor[b.Index] = b.InputObjIdx
					}
					if b.OutputObjIdx >= 0 {
						predecessor[b.OutputObjIdx] = b.Index
					}
				}
			}
			byIndex := map[int32]blueprint.Building{}
			for _, b := range doc.Buildings {
				byIndex[b.Index] = b
			}
			served := map[int32]int{}
			for _, b := range doc.Buildings {
				if b.ItemID != int16(2010+tc.level) {
					continue
				}
				capacity, err := transport.InserterCapacityPerMinute(tc.level, int(b.Parameters[0]))
				if err != nil || capacity+1e-6 < rate {
					t.Fatal("实际跨度分拣器无法承担单台燃料")
				}
				next := b.InputObjIdx
				for steps := 0; byIndex[next].ItemID == 2001 && steps < len(doc.Buildings); steps++ {
					previous, ok := predecessor[next]
					if !ok {
						t.Fatal("燃料输入带未接到塔")
					}
					next = previous
				}
				if byIndex[next].ItemID != 2104 {
					t.Fatal("机组没有真实共享塔来源")
				}
				served[next]++
			}
			beltCapacity, _ := transport.BeltCapacityPerMinute(1)
			for _, count := range served {
				if float64(count)*rate > beltCapacity+1e-6 {
					t.Fatal("共享主带燃料流超过实际带速")
				}
			}
			for _, want := range ports {
				found := false
				for _, got := range result.Actual.Ports {
					found = found || got == want
				}
				if !found {
					t.Fatalf("缺少真实燃料端口回执：%+v", want)
				}
			}
		})
	}
}

// TestFuelBatchFallsBackToSingleSite 职责：狭窄干燥纬带容不下双列组时沿用单台模块；使用：碎片空地回退；边界：每台仍独立通过地形、电网和原版蓝图终检。
func TestFuelBatchFallsBackToSingleSite(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	req := Request{PlanetID: 101, TerrainKnown: true, wet: func(p Position) (bool, bool) { return math.Abs(p.Latitude) > 5*float64(latitudeStep), true }}
	power := industryplan.PowerPlan{FacilityID: 2210, FuelItemID: 1803, Count: 3, PowerMW: 216, FuelRatePerMinute: 1.8}
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Power: &power, Ports: []industryplan.Port{{ItemID: 1803, Kind: "import", RatePerMinute: 1.8}}}}
	actual := industryplan.PlanetActual{}
	if err := appendPower(context.Background(), &req, input, &actual, catalog); err != nil {
		t.Fatal(err)
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Summary.ItemCounts[2210] != 3 || doc.Summary.ItemCounts[2104] != 3 || actual.Power == nil || actual.Power.Count != 3 {
		t.Fatalf("窄地带未保留原单台落位能力：%v / %+v", doc.Summary.ItemCounts, actual.Power)
	}
}
