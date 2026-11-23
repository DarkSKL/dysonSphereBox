package planet

// 文件职责：验证制造子图与独立物流直通流分离；使用：逐星制造入口回归；边界：只核对净流守恒，不代替设施落位验证。

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestProductionMaterialPlan 职责：覆盖燃料、光子、部分直通及联产入库；使用：制造子图回归；边界：输入是明确给定的净流，不依赖配方求解结果生成期望值。
func TestProductionMaterialPlan(t *testing.T) {
	iron := quant.Production{MainItemID: 1101, MachineCount: 1, OutputCount: 30,
		Inputs: []quant.ItemRate{{ItemID: 1001, RatePerMinute: 30}}, Outputs: []quant.ItemRate{{ItemID: 1101, RatePerMinute: 30}}}
	for _, tc := range []struct {
		name       string
		plan       quant.Result
		cached     bool
		wantRaw    map[int]float64
		wantTarget map[int]float64
		wantLeft   map[int]float64
		wantExtern []int
	}{
		{
			name: "coal",
			plan: quant.Result{
				Productions:          []quant.Production{iron},
				RawInputs:            []quant.ItemRate{{ItemID: 1001, RatePerMinute: 30}, {ItemID: 1006, RatePerMinute: 60}},
				Targets:              []quant.Target{{ItemID: 1101, RatePerMinute: 30}, {ItemID: 1006, RatePerMinute: 60}},
				ExternalInputItemIDs: []int{1006},
			},
			cached: true, wantRaw: map[int]float64{1001: 30}, wantTarget: map[int]float64{1101: 30},
		},
		{
			name: "photons_and_finished_product_delivery",
			plan: quant.Result{
				Productions: []quant.Production{iron},
				RawInputs: []quant.ItemRate{
					{ItemID: 1001, RatePerMinute: 30}, {ItemID: 1208, RatePerMinute: 90}, {ItemID: 1101, RatePerMinute: 20},
				},
				Targets:              []quant.Target{{ItemID: 1101, RatePerMinute: 50}, {ItemID: 1208, RatePerMinute: 90}},
				ExternalInputItemIDs: []int{1208, 1101},
			},
			wantRaw: map[int]float64{1001: 30}, wantTarget: map[int]float64{1101: 30},
		},
		{
			name: "same_coal_for_production_and_transit",
			plan: quant.Result{
				Productions: []quant.Production{{MainItemID: 1109, MachineCount: 1, OutputCount: 30,
					Inputs: []quant.ItemRate{{ItemID: 1006, RatePerMinute: 60}}, Outputs: []quant.ItemRate{{ItemID: 1109, RatePerMinute: 30}}}},
				RawInputs: []quant.ItemRate{{ItemID: 1006, RatePerMinute: 80}, {ItemID: 1006, RatePerMinute: 70}},
				Targets: []quant.Target{
					{ItemID: 1109, RatePerMinute: 30}, {ItemID: 1006, RatePerMinute: 50}, {ItemID: 1006, RatePerMinute: 40},
				},
				ExternalInputItemIDs: []int{1006},
			},
			wantRaw: map[int]float64{1006: 60}, wantTarget: map[int]float64{1109: 30}, wantExtern: []int{1006},
		},
		{
			name: "byproduct_and_external_storage",
			plan: quant.Result{
				Productions: []quant.Production{{MainItemID: 1114, MachineCount: 1, OutputCount: 30,
					Inputs:  []quant.ItemRate{{ItemID: 1007, RatePerMinute: 30}},
					Outputs: []quant.ItemRate{{ItemID: 1114, RatePerMinute: 30}, {ItemID: 1120, RatePerMinute: 15}}}},
				RawInputs:            []quant.ItemRate{{ItemID: 1007, RatePerMinute: 30}, {ItemID: 1120, RatePerMinute: 30}},
				Targets:              []quant.Target{{ItemID: 1114, RatePerMinute: 30}},
				Leftovers:            []quant.ItemRate{{ItemID: 1120, RatePerMinute: 45}},
				ExternalInputItemIDs: []int{1120},
			},
			cached: true, wantRaw: map[int]float64{1007: 30}, wantTarget: map[int]float64{1114: 30}, wantLeft: map[int]float64{1120: 15},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			graph, err := quant.BuildFlowGraphContext(ctx, tc.plan)
			if err != nil {
				t.Fatalf("样例供需不守恒：%v", err)
			}
			if tc.cached {
				tc.plan.FlowGraph = &graph
			}
			site := industryplan.PlanetPlan{Plan: tc.plan}
			before, _ := json.Marshal(site)
			got, err := productionMaterialPlan(ctx, site)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(site)
			if string(before) != string(after) {
				t.Fatal("制造子图修改了原始整星计划")
			}
			if !reflect.DeepEqual(got.Productions, tc.plan.Productions) {
				t.Fatal("制造设备或配方净流被修改")
			}
			assertRates := func(label string, rates []quant.ItemRate, want map[int]float64) {
				t.Helper()
				actual := map[int]float64{}
				for _, rate := range rates {
					actual[rate.ItemID] += rate.RatePerMinute
				}
				if len(actual) != len(want) {
					t.Fatalf("%s 物料不符：得到 %v，预期 %v", label, actual, want)
				}
				for item, rate := range want {
					if actual[item] != rate {
						t.Fatalf("%s 物品 %d 得到 %v，预期 %v", label, item, actual[item], rate)
					}
				}
			}
			assertRates("原料", got.RawInputs, tc.wantRaw)
			assertRates("剩余副产物", got.Leftovers, tc.wantLeft)
			var targetRates []quant.ItemRate
			for _, target := range got.Targets {
				targetRates = append(targetRates, quant.ItemRate{ItemID: target.ItemID, RatePerMinute: target.RatePerMinute})
			}
			assertRates("制造交付", targetRates, tc.wantTarget)
			if !reflect.DeepEqual(got.ExternalInputItemIDs, tc.wantExtern) {
				t.Fatalf("外供标记不符：得到 %v，预期 %v", got.ExternalInputItemIDs, tc.wantExtern)
			}
			rebuilt, err := quant.BuildFlowGraphContext(ctx, got)
			if err != nil || got.FlowGraph == nil || got.FlowGraph.Digest != rebuilt.Digest {
				t.Fatalf("制造子图未保持守恒或未刷新摘要：%v", err)
			}
			kinds := map[string]string{}
			for _, node := range rebuilt.Nodes {
				kinds[node.ID] = node.Kind
			}
			for _, edge := range rebuilt.Edges {
				if kinds[edge.SourceNodeID] != "production" && kinds[edge.TargetNodeID] != "production" {
					t.Fatalf("制造子图仍含边界直通：%+v", edge)
				}
			}
		})
	}
}
