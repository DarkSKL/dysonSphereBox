package industryplan

// 文件职责：核对机器切片的总量、原型能力和联产守恒；使用：跨星分工回归；边界：不运行实体蓝图布局。

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestProductionBlockIdentity 职责：证明能源增量切片后搬迁仍命中原配方；使用：跨星修复回归；边界：同一配方内部按块序号延续身份。
func TestProductionBlockIdentity(t *testing.T) {
	a := quant.Production{RecipeID: 1, MainItemID: 1101, MachineID: 2302}
	b := quant.Production{RecipeID: 2, MainItemID: 1104, MachineID: 2302}
	before := productionBlockIDs([]quant.Production{a, a, b})
	after := productionBlockIDs([]quant.Production{a, a, a, b})
	policy := assignment{moves: map[string]int64{before[2]: 201}, changed: map[string]bool{before[2]: true}}
	if after[0] != before[0] || after[1] != before[1] || after[3] != before[2] || after[2] == before[2] {
		t.Fatalf("前序块增长改变了身份：%v => %v", before, after)
	}
	if policy.moves[after[3]] != 201 || policy.moves[after[2]] != 0 || !policy.changed[after[3]] {
		t.Fatal("修复记录错误地迁移到新增能源模块")
	}
}

// TestSplitProductionsConservation 职责：验证默认块、尾块、满载和多输入联产；使用：模块分割回归；边界：以原量化结果为总量基准，切片能力另由原型复核。
func TestSplitProductionsConservation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		item, recipe   int
		rate           float64
		mode, points   int
		maxMachines    int
		wantCounts     []int
		joint, recycle bool
	}{
		{name: "default_and_fractional_tail", item: 1101, recipe: 1, rate: 3855, wantCounts: []int{32, 32, 1}},
		{name: "full_load", item: 1101, recipe: 1, rate: 3900, maxMachines: 7, wantCounts: []int{7, 7, 7, 7, 7, 7, 7, 7, 7, 2}},
		{name: "full_load_fractional_machine_rate", item: 1305, recipe: 52, rate: 750, maxMachines: 7, wantCounts: []int{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 2}},
		{name: "sprayed_coproducts", item: 1114, recipe: 16, rate: 2414.0625, mode: 2, points: 4, maxMachines: 17, wantCounts: []int{17, 17, 17, 14}, joint: true},
		{name: "recycling_coproducts", item: 1109, recipe: 58, rate: 1950, mode: 1, points: 4, maxMachines: 32, wantCounts: []int{32, 32, 1}, joint: true, recycle: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := moduleTestPlan(t, tc.item, tc.recipe, tc.rate, tc.mode, tc.points)
			if len(plan.Productions) != 1 {
				t.Fatalf("隔离配方样例应只有一条产线：%d", len(plan.Productions))
			}
			production := plan.Productions[0]
			if tc.joint && (len(production.Inputs) < 2 || len(production.Outputs) < 2) {
				t.Fatal("样例没有覆盖多输入联产")
			}
			if tc.recycle && len(production.Capacity.GrossInputs) <= len(production.Inputs) {
				t.Fatal("样例没有覆盖配方内循环毛流")
			}
			before, _ := json.Marshal(plan)
			got, err := SplitProductions(context.Background(), plan, tc.maxMachines)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(plan)
			if string(before) != string(after) {
				t.Fatal("切片修改了输入量化方案")
			}
			counts := make([]int, len(got.Productions))
			for i, part := range got.Productions {
				counts[i] = part.MachineCount
			}
			if !reflect.DeepEqual(counts, tc.wantCounts) {
				t.Fatalf("整数块数量不符：得到 %v，预期 %v", counts, tc.wantCounts)
			}
			limit := tc.maxMachines
			if limit <= 0 {
				limit = 32
			}
			checkModuleConservation(t, plan, got, limit)
		})
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 1301, RatePerMinute: 400}, {ItemID: 1101, RatePerMinute: 25}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := SplitProductions(context.Background(), plan, 2)
	if err != nil {
		t.Fatal(err)
	}
	checkModuleConservation(t, plan, got, 2)
}

// TestSplitProductionsUnchangedAndInvalid 职责：确认无需切片时保持原快照并拒绝不可信能力；使用：入口边界回归；边界：取消不返回部分结果。
func TestSplitProductionsUnchangedAndInvalid(t *testing.T) {
	plan := moduleTestPlan(t, 1101, 1, 120, 0, 0)
	got, err := SplitProductions(context.Background(), plan, 32)
	if err != nil || !reflect.DeepEqual(got, plan) {
		t.Fatalf("无需切片时方案被改动：%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SplitProductions(ctx, plan, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("未传播取消：%v", err)
	}
	plan.Productions[0].Capacity.PowerMW++
	if _, err := SplitProductions(context.Background(), plan, 1); err == nil {
		t.Fatal("切片替不符原型的旧能力重新背书")
	}
}

// TestSplitProductionsMissingParentCapacity 职责：证明旧快照的子块能力也来自原型重建；使用：可信父能力切分回归；边界：子块不共享可变额定流数组。
func TestSplitProductionsMissingParentCapacity(t *testing.T) {
	plan := moduleTestPlan(t, 1114, 16, 2414.0625, 2, 4)
	legacy := plan
	legacy.Productions = append([]quant.Production(nil), plan.Productions...)
	legacy.Productions[0].Capacity = nil
	got, err := SplitProductions(context.Background(), legacy, 17)
	if err != nil {
		t.Fatal(err)
	}
	checkModuleConservation(t, plan, got, 17)
	if legacy.Productions[0].Capacity != nil {
		t.Fatal("切片不应补写旧快照的能力字段")
	}
	parentRate := plan.Productions[0].Capacity.GrossInputs[0].RatePerMinute
	siblingRate := got.Productions[1].Capacity.GrossInputs[0].RatePerMinute
	got.Productions[0].Capacity.GrossInputs[0].RatePerMinute++
	if plan.Productions[0].Capacity.GrossInputs[0].RatePerMinute != parentRate || got.Productions[1].Capacity.GrossInputs[0].RatePerMinute != siblingRate {
		t.Fatal("子块额定流与父线或其他子块共用数组")
	}
}

// moduleTestPlan 职责：从真实原型求解隔离配方；使用：切片测试固定机器和喷涂配置；边界：其他物料全部外供，不用切片实现计算期望值。
func moduleTestPlan(t *testing.T, item, recipe int, rate float64, mode, points int) quant.Result {
	t.Helper()
	dataset, err := quant.LoadDataset("")
	if err != nil {
		t.Fatal(err)
	}
	index := dataset.RecipeByID[recipe]
	choice := 0
	for i, candidate := range dataset.Choices[dataset.ByID[item].Name] {
		if candidate == index {
			choice = i + 1
		}
	}
	if choice == 0 {
		t.Fatal("样例物品没有指定配方")
	}
	req := quant.Request{Targets: []quant.Target{{ItemID: item, RatePerMinute: rate}},
		Scheme:   quant.Scheme{ItemRecipeChoices: map[string]int{dataset.ByID[item].Name: choice}, SchemeForRecipe: make([]quant.RecipeScheme, len(dataset.Recipes))},
		Settings: quant.Settings{MineralizeList: map[string]bool{}},
	}
	req.Scheme.SchemeForRecipe[index] = quant.RecipeScheme{Building: 0, ProductionMode: mode, IncreasePoints: points}
	for _, material := range dataset.Items {
		if material.ID != item {
			req.Settings.MineralizeList[material.Name] = true
		}
	}
	plan, err := quant.Solve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// checkModuleConservation 职责：逐原产线汇总切片及额定能力；使用：单配方和完整制造链对拍；边界：允许浮点运算舍入，不允许少机器或少物料。
func checkModuleConservation(t *testing.T, plan, got quant.Result, limit int) {
	t.Helper()
	dataset, err := quant.LoadDataset(plan.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	closeTo := func(label string, value, want float64) {
		t.Helper()
		if math.Abs(value-want) > 1e-9*max(1, math.Abs(want)) {
			t.Fatalf("%s 不守恒：得到 %.15g，预期 %.15g", label, value, want)
		}
	}
	metadata, expected := got, plan
	metadata.Productions, expected.Productions = nil, nil
	metadata.FlowGraph, expected.FlowGraph = nil, nil
	metadata.Statistics, expected.Statistics = nil, nil
	if !reflect.DeepEqual(metadata, expected) {
		t.Fatal("目标、原料、副产物或方案总量被改动")
	}
	cursor := 0
	for _, original := range plan.Productions {
		count := 1 + (original.MachineCount-1)/limit
		parts := got.Productions[cursor : cursor+count]
		cursor += count
		machineCount := 0
		sums := make([]float64, 6)
		vectors := make([][]quant.ItemRate, 6)
		for _, part := range parts {
			if part.MachineCount <= 0 || part.MachineCount > limit {
				t.Fatal("切片包含非正或超限的整数机器数")
			}
			machineCount += part.MachineCount
			if _, err := quant.ValidatedProductionCapacity(dataset, part); err != nil {
				t.Fatalf("切片原型能力不符：%v", err)
			}
			closeTo("单机产能", part.Capacity.OutputPerMachinePerMinute, original.Capacity.OutputPerMachinePerMinute)
			closeTo("单机周期", part.Capacity.CyclesPerMinute, original.Capacity.CyclesPerMinute)
			closeTo("相同利用率", part.Capacity.Utilization, original.Capacity.Utilization)
			for i, value := range []float64{part.ExactMachineCount, part.OutputCount, part.PowerMW, part.RoundingMargin, part.Capacity.OutputPerMinute, part.Capacity.PowerMW} {
				sums[i] += value
			}
			for i, rates := range [][]quant.ItemRate{part.Inputs, part.Outputs, part.Capacity.Inputs, part.Capacity.Outputs, part.Capacity.GrossInputs, part.Capacity.GrossOutputs} {
				vectors[i] = append(vectors[i], rates...)
			}
			configuration, wantConfig := part, original
			configuration.MachineCount, wantConfig.MachineCount = 0, 0
			configuration.ExactMachineCount, wantConfig.ExactMachineCount = 0, 0
			configuration.OutputCount, wantConfig.OutputCount = 0, 0
			configuration.PowerMW, wantConfig.PowerMW = 0, 0
			configuration.RoundingMargin, wantConfig.RoundingMargin = 0, 0
			configuration.Inputs, wantConfig.Inputs = nil, nil
			configuration.Outputs, wantConfig.Outputs = nil, nil
			configuration.Capacity, wantConfig.Capacity = nil, nil
			if !reflect.DeepEqual(configuration, wantConfig) {
				t.Fatal("切片改变了设备、配方、喷涂或服务目标")
			}
		}
		if machineCount != original.MachineCount {
			t.Fatal("整数机器总数不守恒")
		}
		for i, value := range []float64{original.ExactMachineCount, original.OutputCount, original.PowerMW, original.RoundingMargin, original.Capacity.OutputPerMinute, original.Capacity.PowerMW} {
			closeTo("连续总量", sums[i], value)
		}
		for i, rates := range [][]quant.ItemRate{original.Inputs, original.Outputs, original.Capacity.Inputs, original.Capacity.Outputs, original.Capacity.GrossInputs, original.Capacity.GrossOutputs} {
			actual := map[int]float64{}
			for _, rate := range vectors[i] {
				actual[rate.ItemID] += rate.RatePerMinute
			}
			for _, rate := range rates {
				closeTo("物料总量", actual[rate.ItemID], rate.RatePerMinute)
				delete(actual, rate.ItemID)
			}
			if len(actual) != 0 {
				t.Fatal("切片增加了物料种类")
			}
		}
	}
	if cursor != len(got.Productions) {
		t.Fatal("多出没有来源的切片")
	}
	graph, err := quant.BuildFlowGraphContext(context.Background(), got)
	if err != nil || got.FlowGraph == nil || got.FlowGraph.Digest != graph.Digest {
		t.Fatalf("切片后流图未重建或不守恒：%v", err)
	}
	if plan.Statistics != nil {
		if got.Statistics == nil || len(got.Statistics.Lines) != len(got.Productions) {
			t.Fatal("统计产线数量未更新")
		}
		for i, line := range got.Statistics.Lines {
			if line.ProductionIndex != i || line.Count != got.Productions[i].MachineCount {
				t.Fatal("统计生产下标未更新")
			}
		}
	}
}
