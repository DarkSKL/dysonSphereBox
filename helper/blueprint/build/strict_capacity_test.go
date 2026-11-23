package build_test

// 文件职责：验证标准黑盒在寻路前排除必留设施超高的严格候选；使用：真实磁铁模块与原生回退；边界：测试完整生成与边界守恒，不使用时间或候选数量作为淘汰条件。

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// TestStrictStationCapacityBeforeRouting 职责：复现2000宇宙矩阵第8块的39x34必留包络；使用：极区拒绝与赤道可行对照；边界：原生跨带结果仍须通过生成器全部核验和严格编解码。
func TestStrictStationCapacityBeforeRouting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = search.WithControl(ctx, search.New(2*time.Minute, nil))
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	request := quant.Request{Targets: []quant.Target{{ItemID: 1102, RatePerMinute: 1599.2531626666669}},
		Settings: quant.Settings{MineralizeList: map[string]bool{dataset.ByID[1143].Name: true}},
		Scheme:   quant.Scheme{SchemeForRecipe: make([]quant.RecipeScheme, len(dataset.Recipes))}}
	for index, recipe := range dataset.Recipes {
		if recipe.ID == 2 {
			request.Scheme.SchemeForRecipe[index] = quant.RecipeScheme{ProductionMode: 2, IncreasePoints: 4}
		}
	}
	plan, err := quant.Solve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Productions) != 1 || plan.MachineCount != 32 || plan.Productions[0].RecipeID != 2 {
		t.Fatalf("回归不是实际32台磁铁模块：%+v", plan.Productions)
	}
	for _, zone := range []string{"equator", "ring5-north-center"} {
		t.Run(zone, func(t *testing.T) {
			var performance build.GenerationPerformance
			local := build.WithPerformance(ctx, func(value build.GenerationPerformance) { performance = value })
			early := map[string]string{}
			local = build.WithWorkflowFinish(local, func(trace *workflow.RunResult, _ error) {
				if trace == nil {
					return
				}
				for _, step := range trace.Trace {
					if step.Tool == "station.assemble@1" && strings.Contains(step.Failure, "标准黑盒必留设施包络") {
						early[strings.TrimSuffix(step.Node, "/solve")] = step.Failure
					}
				}
			})
			needPower := true
			started := time.Now()
			result, err := build.Generate(local, build.Request{Plan: plan, Mode: 4, CandidateConcurrency: 2,
				Setting:       build.Setting{NeedPower: &needPower, BeltLv: 3, InserterLv: 3},
				SurfaceLayout: &build.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: zone, ExpandAdjacent: true}})
			elapsed := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			if result.SurfacePlan == nil || result.LogisticsSummary == nil {
				t.Fatal("生成缺少完整纬带或物流核验")
			}
			if zone == "equator" {
				if len(early) != 0 || len(result.SurfacePlan.Bands) != 1 {
					t.Fatalf("可用的严格赤道候选被误剪：提前拒绝%d，纬带%+v", len(early), result.SurfacePlan.Bands)
				}
			} else {
				foundEnvelope := false
				for _, failure := range early {
					foundEnvelope = foundEnvelope || strings.Contains(failure, "39x34") && strings.Contains(failure, "最大高度15")
				}
				if !foundEnvelope || len(result.SurfacePlan.Bands) < 2 || result.SurfacePlan.AnchorLatitudeGrid < 180 {
					t.Fatalf("没有保留39x34严格拒绝后的极区原生回退：拒绝%v，结果%+v", early, result.SurfacePlan)
				}
			}
			matched, routingSeen := 0, false
			for _, candidate := range performance.CandidateDetails {
				pruned := false
				for path := range early {
					if strings.HasPrefix(candidate.ID, path+"/") {
						pruned = true
						matched++
					}
				}
				for _, stage := range candidate.Stages {
					if strings.HasPrefix(stage.Stage, "routing") {
						routingSeen = true
						if pruned {
							t.Fatalf("确定超高的严格候选仍进入运输寻路：%s", candidate.ID)
						}
					}
				}
			}
			if matched != len(early) || !routingSeen {
				t.Fatalf("缺少候选级阶段证据：提前拒绝%d，匹配%d，有效候选寻路%t", len(early), matched, routingSeen)
			}
			for _, flow := range []struct {
				kind  string
				items []quant.ItemRate
			}{{"external-input", plan.RawInputs}, {"target-output", []quant.ItemRate{{ItemID: 1102, RatePerMinute: plan.Targets[0].RatePerMinute}}}} {
				for _, want := range flow.items {
					rate := 0.0
					for _, port := range result.LogisticsSummary.ExternalPorts {
						if port.Kind == flow.kind && port.ItemID == want.ItemID {
							rate += port.RatePerMinute
						}
					}
					if math.Abs(rate-want.RatePerMinute) > quant.FlowTolerance(rate, want.RatePerMinute) {
						t.Fatalf("回退改变边界流量：%s %+v，实际%g", flow.kind, want, rate)
					}
				}
			}
			document, err := blueprint.Decode(result.Blueprint, true)
			if err != nil || document.Summary.ItemCounts[2302] != 32 {
				t.Fatalf("原生回退不能严格解码或丢失机器：%v，%v", err, document.Summary.ItemCounts)
			}
			encoded, err := blueprint.Encode(document, document.EnvelopeVersion)
			if err != nil || !encoded.Verified || encoded.Blueprint != result.Blueprint {
				t.Fatalf("严格编解码改变了已验证产物：%v", err)
			}
			t.Logf("%s 完整生成%s；提前拒绝%d个候选且均未进入routing，最终锚点%d、%dx%d、实体%d，32机与边界守恒", zone, elapsed, len(early), result.SurfacePlan.AnchorLatitudeGrid, result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight, result.BuildingCount)
		})
	}
}
