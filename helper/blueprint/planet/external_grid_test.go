package planet

// 文件职责：验证外接电网条件下的真实设备、电网和负荷回执；使用：充电枢纽与制造接入回归；边界：严格编解码不替代玩家在游戏中连接已有电网。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestExternalGridChargingBlueprint 职责：核对外电驱动充电枢纽且保留电池实连接；使用：60 MW 通过、45 MW 拒绝；边界：充电器不能被计为新建发电。
func TestExternalGridChargingBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		capacity float64
		valid    bool
	}{{"sufficient", 60, true}, {"insufficient", 45, false}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			power := &industryplan.PowerPlan{ExternalGridMW: tc.capacity, RequiredMW: 54,
				Sources: []industryplan.PowerSource{{FacilityID: 2209, Count: 1, Mode: "charge", PeerPlanetID: 102}}}
			input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Power: power, Ports: []industryplan.Port{
				{ItemID: 2206, Kind: "import", PeerPlanetID: 102, RatePerMinute: 6},
				{ItemID: 2207, Kind: "export", PeerPlanetID: 102, RatePerMinute: 6},
			}}}
			req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
			err := appendEnergy(ctx, &req, input, &actual, catalog)
			if !tc.valid {
				var issue *Error
				if !errors.As(err, &issue) || issue.Code != "PLANET_POWER" || actual.Power != nil {
					t.Fatalf("45 MW 不足以供给充电器时应拒绝供电合同：%v，%+v", err, actual.Power)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if actual.Power == nil || actual.Power.PowerMW != 0 || actual.Power.ExternalGridMW != 60 || actual.Power.RequiredMW < 54 || actual.Power.RequiredMW > 60 || !req.externalGrid {
				t.Fatalf("外电或真实充电负荷回执错误：%+v", actual.Power)
			}
			if !strings.Contains(strings.Join(actual.Warnings, " "), "接入已有电网") {
				t.Fatal("外电声明缺少玩家接线条件")
			}
			built, err := Build(ctx, req)
			if err != nil {
				t.Fatalf("外电充电设施不能组成有效蓝图：%v", err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Summary.ItemCounts[2209] != 1 || doc.Summary.ItemCounts[2104] != 2 {
				t.Fatalf("充电枢纽或满空电池物流塔缺失：%v", doc.Summary.ItemCounts)
			}
			for _, device := range doc.Buildings {
				if device.ItemID != 2209 {
					continue
				}
				if len(device.Parameters) != 1 || device.Parameters[0] != 1 {
					t.Fatalf("枢纽没有工作于充电模式：%+v", device)
				}
				connected := map[int32]bool{}
				energyAssertBeltPath(t, doc.Buildings, device.Index, 2206, true, 2, connected)
				energyAssertBeltPath(t, doc.Buildings, device.Index, 2207, false, 0, connected)
			}
		})
	}
}

// TestExternalGridManufacturingConnection 职责：核对无发电机的制造区接入节点；使用：复用已有节点与新建接入塔回归；边界：只检验电气装配，不宣称单台制造机已有完整供料线。
func TestExternalGridManufacturingConnection(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	factCatalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	furnace, ok := factCatalog.Lookup(2302)
	if !ok || furnace.Power == nil {
		t.Fatal("缺少熔炉原型")
	}
	load := float64(furnace.Power.Work) * .00006
	for _, existing := range []bool{false, true} {
		name := "new_access_node"
		if existing {
			name = "existing_node"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			position := Position{Longitude: .03}
			req := Request{PlanetID: 101, Objects: []Object{{First: position, Second: position, Building: blueprint.Building{
				Index: 1, ItemID: 2302, ModelIndex: int16(furnace.ModelIndex), RecipeID: 1, InputObjIdx: -1, OutputObjIdx: -1,
			}}}}
			if existing {
				req.Objects = append(req.Objects, Object{Building: blueprint.Building{Index: 2, ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}})
			}
			input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101,
				Power: &industryplan.PowerPlan{ExternalGridMW: 1, RequiredMW: load},
				Plan:  quant.Result{Productions: []quant.Production{{PowerMW: load}}},
			}}
			actual := industryplan.PlanetActual{}
			if err := appendEnergy(ctx, &req, input, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			if actual.Power == nil || actual.Power.PowerMW != 0 || actual.Power.ExternalGridMW != 1 || actual.Power.RequiredMW != load {
				t.Fatalf("制造负荷或外电回执不符：%+v", actual.Power)
			}
			built, err := Build(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Summary.ItemCounts[2302] != 1 || doc.Summary.ItemCounts[2201] < 1 || existing && len(doc.Buildings) != 2 {
				t.Fatalf("外电接入口缺失或未复用已有节点：%v", doc.Summary.ItemCounts)
			}
		})
	}
}
