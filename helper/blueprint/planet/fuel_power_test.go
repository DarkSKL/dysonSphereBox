package planet

// 文件职责：验证燃料电厂的额定合同与真实输入模块；使用：火电、聚变和人造恒星回归；边界：不模拟燃料喷涂或游戏运行。

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
)

// TestFuelGeneratorRating 职责：验证不同燃料的功率和热耗；使用：单厂额定回归；边界：金色棒提高功率但不会把热值再翻倍。
func TestFuelGeneratorRating(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		facility, fuel int
		power, rate    float64
	}{
		{2204, 1006, 2.16, 60},
		{2204, 1109, 2.16, 24},
		{2204, 1120, 2.16, 18},
		{2204, 1801, 2.16, 3},
		{2204, 1123, 2.16, 1687.5},
		{2211, 1802, 15, 1.5},
		{2210, 1803, 72, .6},
		{2210, 1804, 144, .12},
	}
	for _, tc := range cases {
		facility, _ := catalog.Lookup(tc.facility)
		fuel, _ := catalog.Lookup(tc.fuel)
		power, rate, err := fuelGeneratorRating(facility, fuel)
		if err != nil || math.Abs(power-tc.power) > 1e-9 || math.Abs(rate-tc.rate) > 1e-9 {
			t.Errorf("设施 %d / 燃料 %d: %.9f MW / %.9f 件每分, %v；期望 %.9f / %.9f", tc.facility, tc.fuel, power, rate, err, tc.power, tc.rate)
		}
	}
	for _, pair := range [][2]int{{2204, 1802}, {2204, 2207}, {2211, 1006}, {2210, 1802}, {2204, 1001}, {2205, 1006}} {
		facility, _ := catalog.Lookup(pair[0])
		fuel, _ := catalog.Lookup(pair[1])
		if _, _, err := fuelGeneratorRating(facility, fuel); err == nil {
			t.Errorf("设施 %d 不应接收燃料 %d", pair[0], pair[1])
		}
	}
}

// TestFuelBlockBlueprint 职责：验证燃料塔、带、分拣器与原型插口实接；使用：不同电厂的球面蓝图回归；边界：覆盖赤道和非赤道落点，游戏实贴仍独立验收。
func TestFuelBlockBlueprint(t *testing.T) {
	factCatalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	latStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	cases := []struct {
		facility, fuel int
		model          int16
		slot           int8
		west, height   float64
	}{
		{2204, 1006, 54, 1, -2.12, -.01},
		{2204, 1120, 54, 1, -2.12, -.01},
		{2211, 1802, 118, 1, -2.12, -.01},
		{2210, 1803, 56, 3, -2.25, -.016},
		{2210, 1804, 56, 3, -2.25, -.016},
	}
	for _, tc := range cases {
		for _, row := range []int{0, 65} {
			start := Position{Longitude: .3, Latitude: float64(float32(row) * float32(latStep))}
			step, _ := geometry.PasteLongitudeStep32(geometry.Radians(start.Latitude), geometry.Vanilla200)
			facility, _ := factCatalog.Lookup(tc.facility)
			objects, err := fuelBlock(0, start, float64(step), facility, int32(tc.fuel), 3, 2, false, catalog)
			if err != nil {
				t.Fatalf("设施 %d 纬格 %d: %v", tc.facility, row, err)
			}
			generator := objects[1]
			var arm Object
			for _, object := range objects {
				if object.Building.ItemID == 2013 {
					arm = object
				}
			}
			if generator.Building.ModelIndex != tc.model || generator.Building.Yaw != 0 || arm.Building.OutputObjIdx != generator.Building.Index || arm.Building.OutputToSlot != tc.slot || arm.Building.FilterID != int16(tc.fuel) {
				t.Fatalf("设施 %d 的原型或燃料插口错误: %+v / %+v", tc.facility, generator.Building, arm.Building)
			}
			center, _ := world(generator.First)
			end, _ := world(arm.Second)
			dx, dy, dz := float64(end.X-center.X), float64(end.Y-center.Y), float64(end.Z-center.Z)
			sinLon, cosLon := math.Sincos(generator.First.Longitude)
			sinLat, cosLat := math.Sincos(generator.First.Latitude)
			west := dx*cosLon + dz*sinLon
			height := dx*cosLat*sinLon + dy*sinLat - dz*cosLat*cosLon
			if math.Abs(west-tc.west) > 1e-4 || math.Abs(height-tc.height) > 1e-4 {
				t.Fatalf("设施 %d 的分拣器终点没有落在原型插口: x=%.6f y=%.6f", tc.facility, west, height)
			}
			if objects[0].Building.Parameters[0] != int32(tc.fuel) || objects[2].Building.InputObjIdx != objects[0].Building.Index || objects[2].Building.InputFromSlot != 10 || objects[2].Building.Parameters[0] != int32(tc.fuel) {
				t.Fatalf("设施 %d 缺少匹配的燃料塔出带路径", tc.facility)
			}
			built, err := Build(context.Background(), Request{PlanetID: 101, Objects: objects})
			if err != nil {
				t.Fatalf("设施 %d 纬格 %d 蓝图几何不成立: %v", tc.facility, row, err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Summary.ItemCounts[int16(tc.facility)] != 1 || doc.Summary.ItemCounts[2104] != 1 || doc.Summary.ItemCounts[2001] != 8 || doc.Summary.ItemCounts[2013] != 1 {
				t.Fatalf("设施 %d 缺少真实燃料物流设备: %+v", tc.facility, doc.Summary.ItemCounts)
			}
		}
	}
}

// TestFuelPowerRejectsInsufficientContracts 职责：拒绝不匹配或不足的燃料合同；使用：布局前回归；边界：失败不能留下部分实体。
func TestFuelPowerRejectsInsufficientContracts(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, power := range []industryplan.PowerPlan{
		{FacilityID: 2211, FuelItemID: 1006, Count: 1, PowerMW: 15, FuelRatePerMinute: 100},
		{FacilityID: 2211, FuelItemID: 1802, Count: 1, PowerMW: 15, FuelRatePerMinute: 1},
		{FacilityID: 2210, FuelItemID: 1803, Count: 1, PowerMW: 144, FuelRatePerMinute: 1.2},
		{FacilityID: 2210, FuelItemID: 1804, Count: 1, PowerMW: 144, FuelRatePerMinute: .06},
	} {
		req := Request{PlanetID: 101}
		actual := industryplan.PlanetActual{}
		input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Power: &power}}
		if err := appendPower(context.Background(), &req, input, &actual, catalog); err == nil || len(req.Objects) != 0 || actual.Power != nil {
			t.Fatalf("无效供电合同不应落位: %+v, %v", power, err)
		}
	}
}
