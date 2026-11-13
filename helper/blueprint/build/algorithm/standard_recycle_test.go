package build

// 文件职责：回归普通物流模式的X射线裂解本地氢气回流。
// 使用：公开Generate入口到严格解码的完整回归；边界：保留原模式设施、机器数及全部终检，不代表游戏内启动库存验收。

import (
	"context"
	"fmt"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestStandardXRayRecycle 职责：覆盖普通直连、配送器、行星塔、星际塔的裂解生产链及旧模式门禁；使用：能力声明和回流装配回归；边界：不切换密度、不只测试绕过入口的内部函数。
func TestStandardXRayRecycle(t *testing.T) {
	dataset, err := quant.LoadDataset("")
	if err != nil {
		t.Fatal(err)
	}
	graphite := dataset.ByID[1109].Name
	choice := 0
	for i, index := range dataset.Choices[graphite] {
		if dataset.Recipes[index].ID == 58 {
			choice = i + 1
		}
	}
	if choice == 0 {
		t.Fatal("缺少X射线裂解配方")
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 6002, RatePerMinute: 60}}, Scheme: quant.Scheme{ItemRecipeChoices: map[string]int{graphite: choice}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			result, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Density: densityStandard, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}})
			if mode == 5 {
				failure, ok := err.(*blueprint.Error)
				if !ok || failure.Code != "GEN_E_PORT_UNVERIFIED" {
					t.Fatalf("未实现回收的内部旧模式必须继续拒绝，实际：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Buildings) == 0 || len(doc.Buildings) != result.BuildingCount {
				t.Fatal("导出建筑数量无效")
			}
			counts := map[int]int{}
			facilities := 0
			facilityID := map[int]int16{2: 2107, 3: 2103, 4: 2104}[mode]
			for _, record := range doc.Buildings {
				if record.RecipeID > 0 {
					counts[int(record.RecipeID)]++
				}
				if record.ItemID == facilityID {
					facilities++
				}
			}
			for _, p := range plan.Productions {
				if counts[p.RecipeID] != p.MachineCount {
					t.Fatalf("配方%d实际机器数%d与计划%d不符", p.RecipeID, counts[p.RecipeID], p.MachineCount)
				}
			}
			if mode != 1 && facilities == 0 {
				t.Fatal("缺少所选模式的实际物流设施")
			}
			passed := false
			for _, check := range result.LogisticsSummary.Checks {
				if check.RuleID == "machine-local-recycle" && check.Status == "passed" {
					passed = true
				}
			}
			if !passed {
				t.Fatal("缺少本地毛流回收终检证据")
			}
			verifyHydrogenReturnBelts(t, doc.Buildings)
			t.Logf("buildings=%d facilities=%d", result.BuildingCount, facilities)
		})
	}
}

// verifyHydrogenReturnBelts 职责：独立追踪导出记录中的优先出氢口到优先回氢口；使用：避免只打开门禁却没有真实回流带；边界：仅沿传送带实际输出引用，不把物流运输或机器内部关系当作回流。
func verifyHydrogenReturnBelts(t *testing.T, records []blueprint.Building) {
	t.Helper()
	byIndex := map[int32]blueprint.Building{}
	for _, record := range records {
		byIndex[record.Index] = record
	}
	returns := 0
	for _, gate := range records {
		if gate.ItemID != 2020 || gate.FilterID != 1120 || len(gate.Parameters) != 6 || gate.Parameters[2] != 1 || gate.Parameters[4] != 4 || gate.Parameters[5] != 1120 {
			continue
		}
		found := false
		for _, belt := range records {
			if belt.ItemID < 2001 || belt.ItemID > 2003 || belt.InputObjIdx != gate.Index || belt.InputFromSlot != 2 {
				continue
			}
			seen := map[int32]bool{}
			current := belt
			for current.ItemID >= 2001 && current.ItemID <= 2003 && !seen[current.Index] {
				seen[current.Index] = true
				next, exists := byIndex[current.OutputObjIdx]
				if !exists {
					break
				}
				if next.ItemID == 2020 {
					found = next.Index != gate.Index && current.OutputToSlot == 3 && next.FilterID == 1120 && len(next.Parameters) == 6 && next.Parameters[3] == 1 && next.Parameters[4] == 8 && next.Parameters[5] == 1120 && len(seen) > 1
					break
				}
				current = next
			}
		}
		if !found {
			t.Fatalf("优先出氢门%d未通过真实传送带连接优先回氢门", gate.Index)
		}
		returns++
	}
	if returns == 0 {
		t.Fatal("蓝图没有本地双优先氢气回流门")
	}
}
