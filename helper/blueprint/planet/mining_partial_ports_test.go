package planet

// 文件职责：验证矿区与进口共同承担出口的实际回执；使用：部分采集出口回归；边界：矿机输送链证明流量，不自动兑现额外库存。

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestMiningPartialExportCompletesTransit 职责：以真实矿机和双塔中转组成 50 采集加 50 进口的 100 出口；使用：严格编解码及完整生成；边界：尚未履约的库存仍由终检拒绝。
func TestMiningPartialExportCompletesTransit(t *testing.T) {
	ctx := context.Background()
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"export", "external-export"} {
		t.Run(kind, func(t *testing.T) {
			peer, remote := int64(103), int32(1)
			if kind == "external-export" {
				peer, remote = 0, 0
			}
			input := miningPartialInput(50)
			input.Plan.Ports = []industryplan.Port{
				{ItemID: 1001, Kind: "mining", RatePerMinute: 50},
				{ItemID: 1001, Kind: "import", PeerPlanetID: 102, RatePerMinute: 50},
				{ItemID: 1001, Kind: kind, PeerPlanetID: peer, RatePerMinute: 100, StorageItems: 1000},
			}
			req, actual := Request{PlanetID: 101, TerrainKnown: true}, industryplan.PlanetActual{}
			var mining []MiningCoverage
			if err := appendMining(ctx, &req, input, &actual, &mining, catalog); err != nil {
				t.Fatal(err)
			}
			wantPartial := []industryplan.Port{input.Plan.Ports[0], {ItemID: 1001, Kind: kind, PeerPlanetID: peer, RatePerMinute: 50}}
			if !reflect.DeepEqual(actual.Ports, wantPartial) || len(mining) != 1 || len(mining[0].VeinIDs) != 1 || mining[0].VeinIDs[0] != 1 || mining[0].RatePerMinute < 50 {
				t.Fatalf("矿区没有按真实接驳回填 50 的部分出口：%+v，覆盖 %+v", actual.Ports, mining)
			}
			if err := appendTransit(ctx, &req, input, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			check := func(built Result, got industryplan.PlanetActual) {
				t.Helper()
				want := map[string]industryplan.Port{}
				for _, port := range input.Plan.Ports {
					port.StorageItems = 0
					want[port.Kind] = port
				}
				if len(got.Ports) != len(want) {
					t.Fatalf("矿区与中转回执遗漏或重复：%+v", got.Ports)
				}
				for _, port := range got.Ports {
					if port != want[port.Kind] {
						t.Fatalf("应为采集 50、进口 50、出口 100，且无新增库存：%+v", got.Ports)
					}
				}
				doc, err := blueprint.Decode(built.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				if doc.Summary.ItemCounts[2301] != 1 || doc.Summary.ItemCounts[2104] != 3 || doc.Summary.ItemCounts[2003] == 0 || built.LocalModes[1001] != 3 {
					t.Fatalf("矿机、真实出矿带或双塔中转缺失：%v，本地角色 %v", doc.Summary.ItemCounts, built.LocalModes)
				}
				roles := map[[2]int32]int{}
				buildings := map[int32]blueprint.Building{}
				var miner, first blueprint.Building
				for _, building := range doc.Buildings {
					buildings[building.Index] = building
					if building.ItemID == 2301 {
						miner = building
					}
					if building.ItemID == 2104 {
						if building.Parameters[0] != 1001 || building.Parameters[3] <= 0 {
							t.Fatalf("物流塔缺少对应矿石的实际槽位：%v", building.Parameters[:6])
						}
						roles[[2]int32{building.Parameters[1], building.Parameters[2]}]++
					}
				}
				if !reflect.DeepEqual(roles, map[[2]int32]int{{1, remote}: 1, {1, 2}: 1, {2, remote}: 1}) {
					t.Fatalf("矿区和中转塔的真实供需角色错误：%v", roles)
				}
				for _, building := range doc.Buildings {
					if building.ItemID == 2003 && building.InputObjIdx == miner.Index {
						first = building
						break
					}
				}
				if first.ItemID != 2003 || first.InputFromSlot != 0 {
					t.Fatal("出矿链没有接到矿机的原版输出口")
				}
				for range len(doc.Buildings) {
					next, exists := buildings[first.OutputObjIdx]
					if !exists {
						t.Fatal("出矿输送链断开")
					}
					if next.ItemID == 2104 {
						if next.Parameters[1] != 1 || next.Parameters[2] != remote || next.Parameters[192+4*int(first.OutputToSlot)] != 2 {
							t.Fatal("出矿带没有接入采集供货塔的真实输入口")
						}
						return
					}
					if next.ItemID != 2003 {
						t.Fatal("出矿带误接到其他设备")
					}
					first = next
				}
				t.Fatal("出矿链未抵达物流塔")
			}
			built, err := Build(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			check(built, actual)
			objects, ports := len(req.Objects), append([]industryplan.Port(nil), actual.Ports...)
			if err := appendTransit(ctx, &req, input, &actual, catalog); err != nil || len(req.Objects) != objects || !reflect.DeepEqual(actual.Ports, ports) {
				t.Fatalf("已补齐的中转被重复建造或计数：%v，%+v", err, actual.Ports)
			}
			if generated, err := Generate(ctx, input); err == nil {
				t.Fatal("未兑现的出口库存通过了完整生成")
			} else {
				var issue *Error
				if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" || issue.Item != 1001 || generated.Blueprint != "" {
					t.Fatalf("额外库存没有由端口终检明确拒绝：%v", err)
				}
			}
			input.Plan.Ports[2].StorageItems = 0
			generated, err := Generate(ctx, input)
			if err != nil {
				t.Fatalf("采集与进口共同出口未完成整星生成：%v", err)
			}
			check(generated.Result, generated.Actual)
		})
	}
}

// TestMiningPartialExportPreservesPriorCoverage 职责：叠加已验证出口并只把采集余量计入本星交付；使用：覆盖写入与剩余量回归；边界：前序模块回执作为已完成能力，不重复占用本次采集流量。
func TestMiningPartialExportPreservesPriorCoverage(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ mine, previous, export, delivery float64 }{
		{50, 25, 75, 0}, {70, 50, 100, 20},
	} {
		input := miningPartialInput(tc.mine)
		input.Plan.Ports = []industryplan.Port{
			{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: 100, StorageItems: 1000},
			{ItemID: 1001, Kind: "delivery", RatePerMinute: 20},
		}
		req := Request{PlanetID: 101, TerrainKnown: true}
		actual := industryplan.PlanetActual{Ports: []industryplan.Port{{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: tc.previous, StorageItems: 7}}}
		var mining []MiningCoverage
		if err := appendMining(context.Background(), &req, input, &actual, &mining, catalog); err != nil {
			t.Fatal(err)
		}
		if actual.Ports[0].RatePerMinute != tc.export || actual.Ports[0].StorageItems != 7 {
			t.Fatalf("前序回执被覆盖或库存虚增：%+v", actual.Ports)
		}
		delivered := 0.0
		for _, port := range actual.Ports {
			if port.Kind == "delivery" {
				delivered += port.RatePerMinute
			}
		}
		if delivered != tc.delivery {
			t.Fatalf("采集出口之后的本地交付重复用量：得到 %g，期望 %g", delivered, tc.delivery)
		}
	}
}

// miningPartialInput 职责：提供一个可验证接驳的固矿点；使用：部分出口实体测试；边界：每点 90 件/分钟，合同仅领取所需部分。
func miningPartialInput(rate float64) ProductionRequest {
	return ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101, Mines: []industryplan.Mine{{ItemID: 1001, FacilityID: 2301, Kind: "solid", SpeedMultiplier: 3, RatePerMinute: rate}}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain(),
			Veins: []saveanalysis.PlanetVein{{ID: 1, ItemID: 1001, VeinType: 1, Amount: 5000, Position: galaxy.Vector{Z: -200}}}},
	}
}
