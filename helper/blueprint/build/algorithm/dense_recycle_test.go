package build

// 文件职责：回归能量矩阵密铺回流区的实体开销与闭环；使用：消除冗余设施时比较完整生成结果；边界：不减少机器或关闭终检。

import (
	"context"
	"fmt"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestDenseEnergyRecycle 职责：复现60/min能量矩阵及裂解前置；使用：统计回流门和完整运输；边界：保留最低带/臂等级。
func TestDenseEnergyRecycle(t *testing.T) {
	plan := denseEnergyPlan(t)
	result, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Density: densityDense, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring1-center"}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	gates, belts, arms := 0, 0, 0
	for _, r := range doc.Buildings {
		if r.ItemID == 2020 {
			gates++
		}
		if surfaceIsBelt(r.ItemID) {
			belts++
		}
		if r.ItemID >= 2011 && r.ItemID <= 2014 {
			arms++
		}
	}
	counts := map[int]int{}
	for _, record := range doc.Buildings {
		if record.RecipeID > 0 {
			counts[int(record.RecipeID)]++
		}
	}
	for _, production := range plan.Productions {
		if counts[production.RecipeID] != production.MachineCount {
			t.Fatal("优化改变了生产机器数量")
		}
	}
	if gates == 0 || gates > 2 || belts >= 621 || result.LogisticsSummary.Metrics.EnvelopeArea >= 1200 || result.LogisticsSummary.Metrics.TotalBeltLengthMeters >= 803.17 {
		t.Fatal("回流冗余未比修复前的703建筑/4门布局减少")
	}
	outlets := map[int32]bool{}
	for _, port := range result.LogisticsSummary.ExternalPorts {
		if port.Kind == "leftover" && port.ItemID == 1120 {
			outlets[doc.Buildings[port.BuildingIndex].Index] = true
		}
	}
	verifyDenseHydrogenBranches(t, doc.Buildings, outlets)
	t.Logf("buildings=%d gates=%d belts=%d arms=%d area=%.0f length=%.2f", result.BuildingCount, gates, belts, arms, result.LogisticsSummary.Metrics.EnvelopeArea, result.LogisticsSummary.Metrics.TotalBeltLengthMeters)
}

// verifyDenseHydrogenBranches 职责：从实际导出带链证明优先回机与直接净出货；使用：冗余移除后的独立验证；边界：不把物流运输、第二合流门或带间过滤臂当作直连。
func verifyDenseHydrogenBranches(t *testing.T, records []blueprint.Building, outlets map[int32]bool) {
	t.Helper()
	byIndex := map[int32]blueprint.Building{}
	for _, r := range records {
		byIndex[r.Index] = r
	}
	machineInputs := map[int32][]blueprint.Building{}
	for _, arm := range records {
		if arm.ItemID >= 2011 && arm.ItemID <= 2014 {
			machine, ok := byIndex[arm.OutputObjIdx]
			if ok && machine.RecipeID > 0 {
				machineInputs[arm.InputObjIdx] = append(machineInputs[arm.InputObjIdx], machine)
			}
		}
	}
	returns := 0
	for _, gate := range records {
		if gate.ItemID != 2020 || gate.FilterID != 1120 {
			continue
		}
		if len(gate.Parameters) != 6 || gate.Parameters[2] != 1 || gate.Parameters[4] != 4 {
			t.Fatal("保留了无净补料用途的合流门，或丢失回流优先级")
		}
		for _, isReturn := range []bool{true, false} {
			reached := false
			for _, start := range records {
				if !surfaceIsBelt(start.ItemID) || start.InputObjIdx != gate.Index || (start.InputFromSlot == 2) != isReturn {
					continue
				}
				current := start
				seen := map[int32]bool{}
				for surfaceIsBelt(current.ItemID) && !seen[current.Index] {
					seen[current.Index] = true
					if !isReturn && outlets[current.Index] {
						reached = true
					}
					for _, machine := range machineInputs[current.Index] {
						if isReturn && machine.RecipeID == 58 || !isReturn && machine.ItemID == 2901 {
							reached = true
						}
					}
					if reached {
						break
					}
					next, ok := byIndex[current.OutputObjIdx]
					if !ok {
						break
					}
					current = next
				}
			}
			if !reached {
				t.Fatalf("门%d的回流=%t未沿纯传送带抵达消费机器或指定剩余氢气出口", gate.Index, isReturn)
			}
		}
		returns++
	}
	if returns == 0 {
		t.Fatal("没有保留回流优先出口")
	}
}

// TestRecycleIntakeGuards 职责：覆盖零补料直接回机及需要新料时的拒绝/保留合流；使用：反向回归；边界：只检查局部链契约，完整生成由上例执行。
func TestRecycleIntakeGuards(t *testing.T) {
	setting, err := normalizeSetting(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	b.adapter, err = resolveModeAdapter(1, densityDense)
	if err != nil {
		t.Fatal(err)
	}
	b.denseSpatial = true
	input := b.make(2001, 0, 0, 0)
	stub := b.straight(-5, 0, 5, 2)
	connect(stub[len(stub)-1], input)
	row := &transportRow{inputs: map[int]*placed{}, recipe: quant.Recipe{Items: []int{1120}}, line: &line{inputs: [][]*placed{{input}}}}
	entry := &machineRecycle{row: row, item: 1120}
	if err := b.prepareRecycleIntake(entry, stub, -9); err != nil {
		t.Fatal(err)
	}
	row.inputs[1120] = entry.fresh
	indices := map[*placed]int{}
	for i, p := range b.all {
		indices[p] = i
	}
	if err := validateRecycleIntake(entry, indices); err != nil {
		t.Fatal(err)
	}
	entry.netInput = 1
	if validateRecycleIntake(entry, indices) == nil {
		t.Fatal("存在新料时允许省去合流门")
	}
	entry.netInput = 0
	row.info.Inputs = []quant.ItemRate{{ItemID: 1120, RatePerMinute: 1}}
	if validateRecycleIntake(entry, indices) == nil {
		t.Fatal("伪造零净输入绕过了真实计划")
	}
	row.info.Inputs = nil
	stub[len(stub)-1].output = nil
	if validateRecycleIntake(entry, indices) == nil {
		t.Fatal("断开的回机带未被拒绝")
	}
	connect(stub[len(stub)-1], input)
	withFresh := &machineRecycle{row: row, item: 1120, netInput: 1}
	if err := b.prepareRecycleIntake(withFresh, stub, -9); err != nil {
		t.Fatal(err)
	}
	if withFresh.inputGate == nil || withFresh.fresh == withFresh.returned {
		t.Fatal("需要新料时未保留独立优先合流门")
	}
}

// denseEnergyPlan 职责：生成与截图一致的裂解前置能量矩阵计划；使用：密铺回归共用；边界：固定60/min且不矿化任何前置。
func denseEnergyPlan(t *testing.T) quant.Result {
	t.Helper()
	dataset, err := quant.LoadDataset("")
	if err != nil {
		t.Fatal(err)
	}
	name := dataset.ByID[1109].Name
	choice := 0
	for i, index := range dataset.Choices[name] {
		if dataset.Recipes[index].ID == 58 {
			choice = i + 1
		}
	}
	if choice == 0 {
		t.Fatal("缺少裂解配方")
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 6002, RatePerMinute: 60}}, Scheme: quant.Scheme{ItemRecipeChoices: map[string]int{name: choice}}})
	if err != nil {
		t.Fatal(err)
	}

	return plan
}

// TestDenseRecycleLogisticsModes 职责：覆盖共享回流优化在配送器和两类塔中的完整闭环；使用：防止修改公共回流装配后破坏其它模式；边界：保留各模式终检。
func TestDenseRecycleLogisticsModes(t *testing.T) {
	plan := denseEnergyPlan(t)
	for _, mode := range []int{2, 3, 4} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			result, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Density: densityDense, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring1-center"}})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			gates, mergeGates := 0, 0
			for _, record := range doc.Buildings {
				if record.ItemID == 2020 && record.FilterID == 1120 && len(record.Parameters) == 6 {
					gates++
					if record.Parameters[3] == 1 {
						mergeGates++
					}
				}
			}
			if gates == 0 || mergeGates != 0 {
				t.Fatal("零净输入仍有合流门，或优先回流门已缺失")
			}
			passed := false
			for _, check := range result.LogisticsSummary.Checks {
				if check.RuleID == "machine-local-recycle" && check.Status == "passed" {
					passed = true
				}
			}
			if !passed {
				t.Fatal("缺少完整回流证明")
			}
			t.Logf("buildings=%d gates=%d", result.BuildingCount, gates)
		})
	}
}
