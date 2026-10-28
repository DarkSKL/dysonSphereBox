package build

import (
	"testing"

	"DysonSphereBox/helper/quant"
)

// surfaceGenerationRequest 为端到端生成校验构造单产线稳定输入。
// 职责：复用原版数据集生成有效的 build 请求。
// 使用：Surface 模式与旧平面兼容回归。
// 边界：仅用于测试，不模拟游戏存档、行星或真实 Unity 碰撞环境。
func surfaceGenerationRequest(t *testing.T) Request {
	t.Helper()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	for _, recipe := range dataset.Recipes {
		for _, machineID := range recipe.Factories {
			machineType, _ := quant.MachineKind(machineID)
			if machineType < 1 || machineType > 6 || len(recipe.Results) == 0 {
				continue
			}
			rawInputs := make([]quant.ItemRate, len(recipe.Items))
			for i, item := range recipe.Items {
				rawInputs[i] = quant.ItemRate{ItemID: item, RatePerMinute: 60 * recipe.ItemCounts[i] / recipe.ResultCounts[0]}
			}
			return Request{
				Plan: quant.Result{
					Dataset: dataset.Info.Key, DataVersion: dataset.Info.DataVersion, PlanID: "surface-test-plan",
					Targets:     []quant.Target{{ItemID: recipe.Results[0], RatePerMinute: 60}},
					RawInputs:   rawInputs,
					Productions: []quant.Production{{RecipeID: recipe.ID, MachineCount: 2, MachineType: machineType, MachineID: machineID, OutputCount: 60, MainItemID: recipe.Results[0], ProductionMode: -1}},
				},
				Mode:    1,
				Setting: Setting{NeedPower: boolPointer(false)},
				Name:    "surface-test",
			}
		}
	}
	t.Fatal("dataset has no supported manufacturing recipe")
	return Request{}
}

// boolPointer 创建测试配置用布尔指针。
// 职责：让布局设置显式关闭可选供电设施。
// 使用：surfaceGenerationRequest。
// 边界：不返回共享可变状态。
func boolPointer(value bool) *bool { return &value }

// TestLongitudeSegmentCountMatchesGameGrid 锁定游戏 PlanetGrid.segmentTable 的关键纬圈值。
// 职责：保护赤道、中纬与极区的离散经度容量。
// 使用：运行 helper/blueprint/build 包测试时调用。
// 边界：测试网格映射，不声称验证 Unity Physics 碰撞。
func TestLongitudeSegmentCountMatchesGameGrid(t *testing.T) {
	cases := []struct {
		latitude int
		want     int
	}{
		{latitude: 0, want: 200},
		{latitude: 100, want: 160},
		{latitude: 200, want: 60},
		{latitude: 250, want: 4},
	}
	for _, testCase := range cases {
		if got := longitudeSegmentCount(testCase.latitude, 200); got != testCase.want {
			t.Errorf("longitudeSegmentCount(%d, 200) = %d, want %d", testCase.latitude, got, testCase.want)
		}
	}
}

// TestGenerateSurfacePlanAndLegacyCompatibility 验证新模式产出真实静态盒诊断且旧请求仍走平面。
// 职责：保护 SurfaceLayout 可选字段的兼容边界和生成结果标记。
// 使用：运行 helper/blueprint/build 包测试时调用。
// 边界：验证 Go 生成/编码与已知静态盒，不证明游戏内可放置。
func TestGenerateSurfacePlanAndLegacyCompatibility(t *testing.T) {
	request := surfaceGenerationRequest(t)
	legacy, err := Generate(t.Context(), request)
	if err != nil {
		t.Fatalf("legacy planar generation failed: %v", err)
	}
	if legacy.SurfacePlan != nil || legacy.Blueprint == "" {
		t.Fatalf("legacy planar result changed unexpectedly: %+v", legacy.SurfacePlan)
	}

	request.SurfaceLayout = &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}
	surface, err := Generate(t.Context(), request)
	if err != nil {
		t.Fatalf("surface generation failed: %v", err)
	}
	if surface.Blueprint == "" || surface.SurfacePlan == nil {
		t.Fatalf("surface result is missing blueprint or plan: %+v", surface.SurfacePlan)
	}
	if surface.SurfacePlan.SelectedLatitudeGrid != 0 || surface.SurfacePlan.CollisionCoverage != "staticBuildColliders" || !surface.SurfacePlan.StaticCollidersChecked || !surface.SurfacePlan.BeltConnectionsChecked || surface.SurfacePlan.PlacementStatus != "partial" {
		t.Fatalf("unexpected surface plan: %+v", surface.SurfacePlan)
	}
}
