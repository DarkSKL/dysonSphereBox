package planet

// 文件职责：验证实装能源负荷及源数量；使用：充放电和混合能源回归；边界：额定功率来自已核验原版事实，不模拟运行时电池库存。

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestActualElectricalDemandExchangerModes 检验充电枢纽的持续负荷与物流塔自身的充电设定。
func TestActualElectricalDemandExchangerModes(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		parameters [][]int32
		wantMW     float64
	}{
		{"charge", [][]int32{{1}}, 54},
		{"discharge", [][]int32{{-1}}, 0},
		{"idle", [][]int32{{0}}, 0},
		{"unspecified", [][]int32{nil}, 0},
		{"two-chargers-one-discharger", [][]int32{{1}, {-1}, {1}}, 108},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{}
			for _, parameters := range tc.parameters {
				req.Objects = append(req.Objects, Object{Building: blueprint.Building{ItemID: 2209, ModelIndex: 45, Parameters: parameters}})
			}
			station := stationObject(10, Position{}, 2206, 2, 2, false)
			station.Building.Parameters[320] = 250000
			req.Objects = append(req.Objects, station)
			plan := industryplan.PlanetPlan{Plan: quant.Result{Productions: []quant.Production{{PowerMW: 7}}}}
			draw := actualElectricalDemand(req, plan, catalog)
			records := make([]blueprint.Building, 0, len(req.Objects))
			for _, obj := range req.Objects {
				records = append(records, obj.Building)
			}
			charging, err := StationChargingMW(records)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(draw-(tc.wantMW+7+.06)) > 1e-8 || math.Abs(charging-15) > 1e-8 {
				t.Fatalf("持续负荷 %.6f MW，塔尖峰 %.6f MW；期望持续负荷 %.6f MW，含物流塔维持电量", draw, charging, tc.wantMW+7+.06)
			}
		})
	}
}

// TestAppendEnergyChargingCapacity 职责：用实际充电枢纽检验发电容量；使用：整星能源装配回归；边界：45 MW 不能驱动 54 MW 充电，60 MW 可覆盖。
func TestAppendEnergyChargingCapacity(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		generators int
		valid      bool
	}{{"insufficient", 3, false}, {"sufficient", 4, true}} {
		t.Run(tc.name, func(t *testing.T) {
			power := &industryplan.PowerPlan{FacilityID: 2211, FuelItemID: 1802, Count: tc.generators,
				PowerMW: float64(tc.generators) * 15, FuelRatePerMinute: float64(tc.generators) * 1.5, RequiredMW: 54,
				Sources: []industryplan.PowerSource{
					{FacilityID: 2209, Count: 1, Mode: "charge", PeerPlanetID: 102},
					{FacilityID: 2211, Count: tc.generators, FuelItemID: 1802, PowerMW: float64(tc.generators) * 15, FuelRatePerMinute: float64(tc.generators) * 1.5},
				}}
			req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
			err := appendEnergy(context.Background(), &req, ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Power: power}}, &actual, catalog)
			if !tc.valid {
				if err == nil {
					t.Fatal("45 MW 电源被错误认定足以维持 54 MW 充电枢纽")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPowerSourcesInstalled(t, req, power.Sources)
		})
	}
}

// assertPowerSourcesInstalled 职责：按实际蓝图记录核对能源原型及工作模式；使用：装配后回归；边界：独立于报告数量，且要求通过原生几何与严格解码。
func assertPowerSourcesInstalled(t *testing.T, req Request, sources []industryplan.PowerSource) {
	t.Helper()
	want := map[[2]int]int{}
	for _, source := range sources {
		mode := 0
		if source.FacilityID == 2209 {
			mode = -1
			if source.Mode == "charge" {
				mode = 1
			}
		}
		want[[2]int{source.FacilityID, mode}] += source.Count
	}
	got := map[[2]int]int{}
	for _, obj := range req.Objects {
		mode := 0
		if obj.Building.ItemID == 2209 && len(obj.Building.Parameters) > 0 {
			mode = int(obj.Building.Parameters[0])
		}
		got[[2]int{int(obj.Building.ItemID), mode}]++
	}
	for key, count := range want {
		if got[key] != count {
			t.Fatalf("电源 %d 模式 %d 报告 %d 台，实际只安装 %d 台", key[0], key[1], count, got[key])
		}
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatalf("能源设施虽然计数存在但不能组成有效蓝图：%v", err)
	}
	if _, err := blueprint.Decode(built.Blueprint, true); err != nil {
		t.Fatal(err)
	}
}
