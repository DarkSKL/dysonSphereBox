package industryplan

// 文件职责：验证扩产裁剪范围后的配置引用；使用：充分开发回归；边界：合成布局回执只验证规划契约，不代替真实落位测试。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// TestExpansionIgnoresUnusedReferences 职责：确认未使用的配置星不会令扩产虚假停止；使用：仓储、能源和接收声明回归；边界：不能借用未获选星的矿量或增大戴森球额度。
func TestExpansionIgnoresUnusedReferences(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*Request)
		photons bool
	}{
		{name: "unused_warehouse", prepare: func(req *Request) { req.StoragePlanetID = 102 }},
		{name: "unused_energy_override", prepare: func(req *Request) {
			req.Energy.Overrides = []EnergyOverride{{PlanetID: 102, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 201}}}
		}},
		{name: "unused_receivers", prepare: func(req *Request) {
			req.Dyson = DysonSettings{Mode: "sphere", Supplies: []DysonSupply{
				{StarID: 1, PlanetIDs: []int64{102}, AvailableMW: 240, Efficiency: 1, Uptime: 1},
				{StarID: 2, PlanetIDs: []int64{201}, AvailableMW: 120, Efficiency: 1, Uptime: 1},
			}}
		}},
		{name: "active_receiver_keeps_original_budget", photons: true, prepare: func(req *Request) {
			req.StoragePlanetID = 102
			req.Quant.Targets = append(req.Quant.Targets, quant.Target{ItemID: 1208, RatePerMinute: 3})
			req.Dyson = DysonSettings{Mode: "sphere", Supplies: []DysonSupply{
				{StarID: 1, PlanetIDs: []int64{101, 102}, AvailableMW: 240, Efficiency: 1, Uptime: 1},
				{StarID: 2, PlanetIDs: []int64{201}, AvailableMW: 120, Efficiency: 1, Uptime: 1},
			}}
		}},
		{name: "disabled_charging_reference", prepare: func(req *Request) {
			req.Energy.Default = EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 102}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{DevelopFully: true, CenterPlanetID: 101, Profile: "low",
				Quant:  quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}},
				Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}},
			}
			tc.prepare(&req)
			world := World{Planets: []Planet{
				{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 4}}},
				{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{10000, 0, 0}, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 100}}},
				{ID: 201, StarID: 2, Buildable: true, ResourcesKnown: true, Position: [3]float64{2000000, 0, 0}, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 1000}}},
			}}
			before, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := Plan(ctx, req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
				if site.PlanetID != 101 {
					t.Fatalf("扩产使用了首轮之外的星球 %d", site.PlanetID)
				}
				return PlanetActual{Ports: site.Ports, Power: site.Power}, nil
			}, nil)
			if err != nil || !result.Verified || len(result.AchievedTargets) != len(req.Quant.Targets) || result.AchievedTargets[0].RatePerMinute != 120 {
				t.Fatalf("应达到选中星球的 120/min 资源上限：目标 %+v，状态 %s，警告 %v，错误 %v", result.AchievedTargets, result.Status, result.Warnings, err)
			}
			if len(result.Planets) != 1 || result.Planets[0].PlanetID != 101 || len(result.ExtraTargets) != len(req.Quant.Targets) || result.ExtraTargets[0].RatePerMinute != 90 {
				t.Fatalf("扩产范围或额外产量不符：星球 %+v，额外产量 %+v", result.Planets, result.ExtraTargets)
			}
			if tc.photons {
				photon := result.Planets[0].Photons
				if result.AchievedTargets[1].RatePerMinute != 12 || photon == nil || photon.Count != 2 || photon.RatePerMinute != 12 || photon.SpherePowerMW != 240 || photon.Efficiency != 1 || photon.Uptime != 1 {
					t.Fatalf("过滤额外接收星改变了原戴森球额度：%+v", photon)
				}
			}
			after, err := json.Marshal(req)
			if err != nil || string(before) != string(after) {
				t.Fatal("扩产改写了用户的原始配置", err)
			}
		})
	}
}
