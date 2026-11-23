package planet

// 文件职责：核对船机共用同一座物流塔时的真实充电档位与原版单塔上限。

import (
	"math"
	"slices"
	"strings"
	"testing"

	"DysonSphereBox/helper/industryplan"
)

func TestCombinedStationChargingUsesFundedTower(t *testing.T) {
	req := Request{PlanetID: 5101, Objects: []Object{
		stationObject(0, Position{}, 1101, 1, 1, false),
		stationObject(1, Position{Longitude: 1}, 1101, 2, 0, false),
	}, droneChargingMW: map[int]float64{0: 31.336137844910873}}
	original := slices.Clone(req.Objects[0].Building.Parameters)
	actual := industryplan.PlanetActual{Fleet: []industryplan.FleetStation{{BuildingIndex: 0, Vessels: 10,
		Cargo: []industryplan.FleetCargo{{ItemID: 1101, VesselLoad: 10, MaxVesselMW: 5.097718109668109}}}}}
	if err := configureFleetCharging(&req, industryplan.PlanetPlan{}, &actual); err != nil {
		t.Fatal(err)
	}
	original[320] = 1400000
	if !slices.Equal(original, req.Objects[0].Building.Parameters) || req.Objects[1].Building.Parameters[320] != 1000000 ||
		len(actual.Warnings) != 1 || !strings.Contains(actual.Warnings[0], "84 MW") {
		t.Fatalf("原版5101铁块发送塔在船机合计82.373 MW时应单独升到84 MW，实际 %v", actual.Warnings)
	}
	if err := configureFleetCharging(&req, industryplan.PlanetPlan{}, &actual); err != nil || len(actual.Warnings) != 1 {
		t.Fatalf("相同配置重复核验后产生额外改动：%v %v", err, actual.Warnings)
	}
}

func TestStationChargingLimitsAndOriginalSettings(t *testing.T) {
	for _, tc := range []struct {
		old    int32
		budget float64
		want   int32
		err    bool
	}{
		{1000000, 59.94, 1000000, false},  // 60 MW 足额。
		{1000000, 59.941, 1050000, false}, // 加待机后超出60，下一档63。
		{1250000, 62, 1250000, false},     // 保留已配置的75。
		{100000, 9, 500000, false},        // 原设只有6，首个可设置档位为30。
		{1000000, 299.94, 5000000, false},
		{1000000, 299.95, 1000000, true},
	} {
		b := stationObject(0, Position{}, 1001, 1, 1, false).Building
		b.Parameters[320] = tc.old
		_, err := ConfigureDispatchCharging(&b, 0, 0, tc.budget, 0)
		if (err != nil) != tc.err || b.Parameters[320] != tc.want {
			t.Fatalf("初值 %d，发运 %.3f MW：参数 %d，错误 %v", tc.old, tc.budget, b.Parameters[320], err)
		}
	}
}

func TestDroneChargerAcceptsOnlyCommissionedDronesAndActualFlow(t *testing.T) {
	g := &localCargoSlots{active: true, rate: 600, sources: []localDroneEndpoint{{tower: 0, chargeArc: 600}}, sinks: []localDroneEndpoint{{tower: 1, chargeArc: 600}}}
	upper := droneStationCharging(map[int]*localCargoSlots{1001: g}, map[int]int{1: 100}, 70, 16, 1000)
	want := 600 * (.8 + .04*600) / 70 / 60
	if len(upper) != 1 || math.Abs(upper[1]-want) > 1e-8 {
		t.Fatalf("对端留空、货物吞吐有上限时的逐塔配送功率不正确：%v", upper)
	}
}
