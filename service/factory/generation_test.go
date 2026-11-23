package factory

import (
	"testing"

	"DysonSphereBox/helper/blueprint/build"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
)

// TestFactoryGenerationFromSolvedPlan 验证已有量化页面在任务调度改动后仍能用同一份方案生成和回读两种布局。
func TestFactoryGenerationFromSolvedPlan(t *testing.T) {
	service := &Service{}
	solved := service.Solve(quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}})
	if solved.Code != "OK" || solved.Data == nil || solved.Data.PlanID == "" {
		t.Fatalf("量化未返回有效方案：%+v", solved.Status)
	}
	for _, mode := range []int{1, 4} {
		output := service.Generate(build.Request{Plan: *solved.Data, Mode: mode})
		if output.Code != "OK" || output.Data == nil || output.Data.BuildingCount == 0 {
			t.Fatalf("布局模式 %d 生成失败：%+v", mode, output.Status)
		}
		decoded := service.Decode(contract.DecodeRequest{Blueprint: output.Data.Blueprint})
		if decoded.Code != "OK" || decoded.Data == nil || len(decoded.Data.Buildings) != output.Data.BuildingCount {
			t.Fatalf("布局模式 %d 的解码建筑数与生成结果不符：%+v", mode, decoded.Status)
		}
		var smelters, stations int
		for _, building := range decoded.Data.Buildings {
			switch building.ItemID {
			case 2302:
				if building.RecipeID == 1 {
					smelters++
				}
			case 2104:
				stations++
			}
		}
		if smelters != 1 || mode == 1 && stations != 0 || mode == 4 && stations == 0 {
			t.Fatalf("布局模式 %d 的铁块熔炉 %d，星际物流站 %d", mode, smelters, stations)
		}
		t.Logf("布局模式 %d 生成并回读 %d 个建筑，含 %d 台铁块熔炉和 %d 座星际物流站", mode, len(decoded.Data.Buildings), smelters, stations)
	}
}
