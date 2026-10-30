package quant

// 文件职责：冻结 P3 循环、多目标、守恒、确定性与失败边界；使用：量化内核修改前后运行定向测试；边界：只使用内存夹具和内置数据，不验证蓝图投影或全局最优。

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// TestP3CycleAndSelfConsumption 职责：验证循环 LP 与可化简自消耗均不伪造物料；使用：R-QC-007 循环回归；边界：内存配方仅覆盖已统一的制造物料。
func TestP3CycleAndSelfConsumption(t *testing.T) {
	d := p3CycleDataset()
	g, err := makeGraph(d, Scheme{}, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	needs := newRates()
	needs.add("X", 60)
	amounts, _, _, err := g.calculate(context.Background(), needs)
	if err != nil {
		t.Fatal(err)
	}
	if !finiteRates(amounts) || math.Abs(amounts.values["X"]-80) > 1e-8 || math.Abs(amounts.values["Y"]-40) > 1e-8 || math.Abs(amounts.values["R"]-120) > 1e-8 {
		t.Fatalf("cycle did not return a finite feasible flow: %+v", amounts.values)
	}
	// 原始配方方程：X - .5Y = 60，Y - .5X = 0，R = X + Y。
	if math.Abs(amounts.values["X"]-.5*amounts.values["Y"]-60) > 1e-8 || math.Abs(amounts.values["Y"]-.5*amounts.values["X"]) > 1e-8 || math.Abs(amounts.values["R"]-amounts.values["X"]-amounts.values["Y"]) > 1e-8 {
		t.Fatalf("cycle mass balance changed: %+v", amounts.values)
	}
	zero := p3CycleDataset()
	zero.Recipes[1].Items, zero.Recipes[1].ItemCounts = []int{3}, []float64{1}
	zero.Recipes[2].Items, zero.Recipes[2].ItemCounts = []int{2}, []float64{1}
	g, err = makeGraph(zero, Scheme{}, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	needs = newRates()
	needs.add("X", 60)
	zeroAmounts, zeroSurplus, _, zeroErr := g.calculate(context.Background(), needs)
	quantErr, ok := zeroErr.(*Error)
	if !ok || quantErr.Code != "Q_E_INFEASIBLE" {
		t.Fatalf("zero-input cycle returned wrong error: %v", zeroErr)
	}
	if zeroAmounts != nil || zeroSurplus != nil {
		t.Fatalf("infeasible zero-input cycle returned partial flows: amounts=%+v surplus=%+v", zeroAmounts, zeroSurplus)
	}

	self := &Dataset{
		Items:   []Item{{ID: 1, Name: "R", Type: 1, Speed: 1}, {ID: 3, Name: "S", Type: 2, Speed: 1, WorkEnergyPerTick: 1}},
		ByID:    map[int]Item{1: {ID: 1, Name: "R", Type: 1, Speed: 1}, 3: {ID: 3, Name: "S", Type: 2, Speed: 1, WorkEnergyPerTick: 1}},
		ByName:  map[string]Item{"R": {ID: 1, Name: "R", Type: 1, Speed: 1}, "S": {ID: 3, Name: "S", Type: 2, Speed: 1, WorkEnergyPerTick: 1}},
		Recipes: []Recipe{{ID: 1, Name: "raw", Type: 1, Factories: []int{2301}, Results: []int{1}, ResultCounts: []float64{1}, TimeSpend: 60}, {ID: 2, Name: "self-consume", Type: 2, Factories: []int{2302}, Items: []int{1, 3}, ItemCounts: []float64{1, .5}, Results: []int{3}, ResultCounts: []float64{1}, TimeSpend: 60}},
		Choices: map[string][]int{"R": {0}, "S": {1}}, Order: []string{"R", "S"},
	}
	self.ByID[3] = self.Items[1]
	self.ByName["S"] = self.Items[1]
	self.Recipes[0].Factories = []int{2301}
	self.ByID[1] = Item{ID: 1, Name: "R", Type: 1, Speed: 1}
	self.ByName["R"] = self.ByID[1]
	self.Info.Key = "memory-self"
	g, err = makeGraph(self, Scheme{}, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	needs = newRates()
	needs.add("S", 60)
	amounts, _, _, err = g.calculate(context.Background(), needs)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(amounts.values["R"]-120) > 1e-8 {
		t.Fatalf("self-consumption should double external input, got %v", amounts.values)
	}
}

// TestP3PublicDeterminismAndConservation 职责：独立汇总公开结果并冻结目标顺序/重复目标语义；使用：R-QC-007/008；边界：不把非六类设施投影当作守恒通过。
func TestP3PublicDeterminismAndConservation(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	plastic := d.ByName["塑料"].ID
	graphene := d.ByName["石墨烯"].ID
	first := Request{Dataset: "Vanilla", Targets: []Target{{ItemID: plastic, RatePerMinute: 60}, {ItemID: graphene, RatePerMinute: 60}}, Scheme: Scheme{
		ItemRecipeChoices: map[string]int{"塑料": 1}, SchemeForRecipe: make([]RecipeScheme, len(d.Recipes)), CostWeight: CostWeight{BuildingCost: map[string]float64{"化工厂": 1}, ItemExtraCost: map[string]ItemCost{"塑料": {Enabled: true, Cost: .25}}},
	}, Settings: Settings{MineralizeList: map[string]bool{"石墨烯": false}}}
	plasticRecipe := d.Choices["塑料"][0]
	first.Scheme.SchemeForRecipe[plasticRecipe] = RecipeScheme{IncreasePoints: 4}
	second := first
	second.Targets = []Target{first.Targets[1], first.Targets[0]}
	before, _ := json.Marshal(first)
	a, err := Solve(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Solve(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := Solve(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(first)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Solve mutated the request")
	}
	if a.PlanID != b.PlanID || !reflect.DeepEqual(a, b) {
		t.Fatal("identical requests produced different business results")
	}
	if !reflect.DeepEqual(a.Amounts, reversed.Amounts) || a.MachineCount != reversed.MachineCount || a.PowerMW != reversed.PowerMW {
		t.Fatalf("target permutation changed physical quantities: %v vs %v", a.Amounts, reversed.Amounts)
	}
	assertP3Conservation(t, d, a)
	if encoded, err := json.Marshal(a); err != nil || len(encoded) == 0 {
		t.Fatalf("finite result cannot be JSON encoded: %v", err)
	}

	duplicate := Request{Dataset: "Vanilla", Targets: []Target{{ItemID: plastic, RatePerMinute: 20}, {ItemID: plastic, RatePerMinute: 40}}}
	combined := Request{Dataset: "Vanilla", Targets: []Target{{ItemID: plastic, RatePerMinute: 60}}}
	dupResult, err := Solve(context.Background(), duplicate)
	if err != nil {
		t.Fatal(err)
	}
	oneResult, err := Solve(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dupResult.Amounts, oneResult.Amounts) || dupResult.PowerMW != oneResult.PowerMW || dupResult.MachineCount != oneResult.MachineCount {
		t.Fatalf("duplicate target aggregation changed physical result: %+v vs %+v", dupResult.Amounts, oneResult.Amounts)
	}
	assertP3Conservation(t, d, dupResult)
}

// TestP3LargeDuplicateOrderCanonicalization 职责：固定大数与微小重复目标的求和顺序；使用：R-QC-008 浮点回归；边界：仅比较公开物理结果，1 ulp 以内差异不作优化声明。
func TestP3LargeDuplicateOrderCanonicalization(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	ore := d.ByName["铁矿"]
	a := Request{Dataset: "Vanilla", Targets: []Target{{ItemID: ore.ID, RatePerMinute: 1e9}, {ItemID: ore.ID, RatePerMinute: 4e-8}, {ItemID: ore.ID, RatePerMinute: 4e-8}}}
	b := a
	b.Targets = []Target{a.Targets[2], a.Targets[1], a.Targets[0]}
	left, err := Solve(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := Solve(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.Amounts, right.Amounts) || left.MachineCount != right.MachineCount || left.PowerMW != right.PowerMW {
		t.Fatalf("large duplicate target ordering changed physical result: %+v vs %+v", left.Amounts, right.Amounts)
	}
}

// TestP3NonFiniteFailureIsEmpty 职责：保证非法成本权重失败时不返回部分成功数据；使用：R-QC-009；边界：只验证既有错误契约。
func TestP3NonFiniteFailureIsEmpty(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: d.ByName["铁块"].ID, RatePerMinute: 60}}, Scheme: Scheme{CostWeight: CostWeight{ItemExtraCost: map[string]ItemCost{"未使用成本": {Cost: math.NaN()}}}}})
	quantErr, ok := err.(*Error)
	if !ok || quantErr.Code != "Q_E_BAD_REQUEST" {
		t.Fatalf("non-finite cost weight returned wrong error: %v", err)
	}
	if len(result.Productions) != 0 || len(result.RawInputs) != 0 || len(result.Leftovers) != 0 || len(result.Amounts) != 0 || len(result.Targets) != 0 || len(result.KeyItems) != 0 || len(result.Warnings) != 0 || result.Dataset != "" || result.DataVersion != "" || result.GameVersion != "" || result.PlanID != "" || result.LPCost != 0 || result.PowerMW != 0 || result.MachineCount != 0 {
		t.Fatalf("failure returned partial result: %+v", result)
	}
}

// TestP3CancelledRequestIsEmpty 职责：冻结取消请求错误码和空业务结果；使用：R-QC-009；边界：不等待真实超时。
func TestP3CancelledRequestIsEmpty(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Solve(ctx, Request{Dataset: "Vanilla", Targets: []Target{{ItemID: d.ByName["铁块"].ID, RatePerMinute: 60}}})
	quantErr, ok := err.(*Error)
	if !ok || quantErr.Code != "Q_E_TIMEOUT" {
		t.Fatalf("cancelled request returned wrong error: %v", err)
	}
	if len(result.Productions) != 0 || len(result.Amounts) != 0 || result.PlanID != "" {
		t.Fatalf("cancelled request returned partial result: %+v", result)
	}
}

// TestP3NaturalNegativeDemandAndCostWeights 职责：覆盖已有产线产生的内部负需求和成本权重；使用：P3 求解稳定性回归；边界：固定产线流量单独核对，不计入非自然产线响应。
func TestP3NaturalNegativeDemandAndCostWeights(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	iron := d.ByName["铁块"]
	request := Request{Dataset: "Vanilla", Targets: []Target{{ItemID: iron.ID, RatePerMinute: 60}}, Settings: Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: "铁块", RecipeChoice: 1, Building: 0, MachineCount: 2}}}}
	result, err := Solve(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Productions) != 0 || len(result.RawInputs) != 1 || result.RawInputs[0].Name != "铁矿" || math.Abs(result.RawInputs[0].RatePerMinute-120) > 1e-8 || len(result.Leftovers) != 1 || result.Leftovers[0].Name != "铁块" || math.Abs(result.Leftovers[0].RatePerMinute-60) > 1e-8 {
		t.Fatalf("fixed overproduction should satisfy target and leave 60/min: %+v", result)
	}
	// 独立按配方原始数量换算固定线物料，并与响应外部输入、目标及剩余对账。
	recipe := d.Recipes[d.Choices["铁块"][0]]
	runs := 60 * request.Settings.NaturalProductionLine[0].MachineCount / (recipe.TimeSpend / 60)
	fixed := map[string]float64{}
	for i, id := range recipe.Items {
		fixed[d.ByID[id].Name] -= recipe.ItemCounts[i] * runs
	}
	for i, id := range recipe.Results {
		fixed[d.ByID[id].Name] += recipe.ResultCounts[i] * runs
	}
	fixed["铁块"] -= request.Targets[0].RatePerMinute + result.Leftovers[0].RatePerMinute
	for _, input := range result.RawInputs {
		fixed[input.Name] += input.RatePerMinute
	}
	for name, residual := range fixed {
		if math.Abs(residual) > 1e-8 {
			t.Fatalf("natural-line conservation residual %s=%v; flows=%+v", name, residual, fixed)
		}
	}

	plastic := d.ByName["塑料"]
	base, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: plastic.ID, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	weighted, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: plastic.ID, RatePerMinute: 60}}, Scheme: Scheme{CostWeight: CostWeight{Power: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	if weighted.LPCost <= base.LPCost {
		t.Fatalf("power cost weight did not raise the iron recipe cost: base=%v weighted=%v", base.LPCost, weighted.LPCost)
	}
}

// TestP3NaturalProductionModifierParity 职责：固定已有产线与普通图一致应用全局加速/增产率；使用：R-QC-007 P3 失败回归；边界：仅覆盖 Vanilla 铁块配方的统一制造设施。
func TestP3NaturalProductionModifierParity(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name     string
		mode     int
		want     float64
		settings Settings
	}{
		{name: "acceleration global rate", mode: 1, want: 240, settings: Settings{AccRate: 2}},
		{name: "extra output global rate", mode: 2, want: 150, settings: Settings{IncRate: 2}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			fixture.settings.NaturalProductionLine = []NaturalProduction{{TargetItem: "铁块", RecipeChoice: 1, Building: 0, MachineCount: 1, ProductionMode: fixture.mode, IncreasePoints: 4}}
			g, err := makeGraph(d, Scheme{}, fixture.settings)
			if err != nil {
				t.Fatal(err)
			}
			needs := newRates()
			if err := g.balanceNatural(needs); err != nil {
				t.Fatal(err)
			}
			if got := -needs.values["铁块"]; math.Abs(got-fixture.want) > 1e-8 {
				t.Fatalf("fixed net output did not use configured global multiplier: got=%v want=%v needs=%+v", got, fixture.want, needs.values)
			}
		})
	}
	// 配方 21208 的 mode3 位在 Vanilla 数据中有效；只比较全局 AccRate=2 相对默认的两倍。
	baseSettings := Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: "临界光子", RecipeChoice: 2, Building: 0, MachineCount: 1, ProductionMode: 3, IncreasePoints: 4}}}
	baseGraph, err := makeGraph(d, Scheme{}, baseSettings)
	if err != nil {
		t.Fatal(err)
	}
	baseNeeds := newRates()
	if err := baseGraph.balanceNatural(baseNeeds); err != nil {
		t.Fatal(err)
	}
	boostedSettings := baseSettings
	boostedSettings.AccRate = 2
	boostedGraph, err := makeGraph(d, Scheme{}, boostedSettings)
	if err != nil {
		t.Fatal(err)
	}
	boostedNeeds := newRates()
	if err := boostedGraph.balanceNatural(boostedNeeds); err != nil {
		t.Fatal(err)
	}
	if math.Abs(boostedNeeds.values["临界光子"]/baseNeeds.values["临界光子"]-2) > 1e-8 {
		t.Fatalf("mode3 fixed output ignored AccRate: base=%v boosted=%v", baseNeeds.values, boostedNeeds.values)
	}
	settings := Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: "铁块", RecipeChoice: 1, Building: 0, MachineCount: math.MaxFloat64}}}
	g, err := makeGraph(d, Scheme{}, settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.balanceNatural(newRates()); err == nil {
		t.Fatal("unbounded fixed line count was accepted")
	}
	settings = Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: "铁块", RecipeChoice: 1, Building: 0, MachineCount: 1000001}}}
	g, err = makeGraph(d, Scheme{}, settings)
	if err != nil {
		t.Fatal(err)
	}
	largeNeeds := newRates()
	if err := g.balanceNatural(largeNeeds); err != nil {
		t.Fatalf("large but finite fixed line count should remain compatible: %v", err)
	}
	if !finiteRates(largeNeeds) || largeNeeds.values["铁块"] >= 0 || math.Abs(-largeNeeds.values["铁块"]-60000060) > 1e-6 {
		t.Fatalf("large finite fixed line flow was incorrect: %+v", largeNeeds.values)
	}
}

// assertP3Conservation 按公开净流量计算每个物品的残差；公开目标和剩余均作为消耗项。
func assertP3Conservation(t *testing.T, d *Dataset, result Result) {
	t.Helper()
	flow := map[string]float64{}
	maxFlow := map[string]float64{}
	add := func(name string, value float64) {
		flow[name] += value
		maxFlow[name] = math.Max(maxFlow[name], math.Abs(value))
	}
	for _, raw := range result.RawInputs {
		add(raw.Name, raw.RatePerMinute)
	}
	for _, production := range result.Productions {
		for _, input := range production.Inputs {
			add(input.Name, -input.RatePerMinute)
		}
		for _, output := range production.Outputs {
			add(output.Name, output.RatePerMinute)
		}
	}
	for _, target := range result.Targets {
		add(d.ByID[target.ItemID].Name, -target.RatePerMinute)
	}
	for _, leftover := range result.Leftovers {
		add(leftover.Name, -leftover.RatePerMinute)
	}
	for name, residual := range flow {
		tolerance := 1e-6 * math.Max(1, maxFlow[name])
		if !finite(residual) || !finite(tolerance) || math.Abs(residual) >= tolerance {
			t.Errorf("%s conservation residual %.12g is outside %.12g (flows: %+v)", name, residual, tolerance, flow)
		}
	}
}

// p3CycleDataset 构建两物品相互依赖且由原料支撑的制造循环，避免依赖内置数据偶然保留该拓扑。
func p3CycleDataset() *Dataset {
	raw := Item{ID: 1, Name: "R", Type: 1, Speed: 1}
	x := Item{ID: 2, Name: "X", Type: 2, Speed: 1, Space: 1, WorkEnergyPerTick: 1}
	y := Item{ID: 3, Name: "Y", Type: 2, Speed: 1, Space: 1, WorkEnergyPerTick: 1}
	items := []Item{raw, x, y}
	recipes := []Recipe{
		{ID: 1, Name: "raw", Type: 1, Factories: []int{2301}, Results: []int{1}, ResultCounts: []float64{1}, TimeSpend: 60},
		{ID: 2, Name: "X from Y", Type: 2, Factories: []int{2302}, Items: []int{1, 3}, ItemCounts: []float64{1, .5}, Results: []int{2}, ResultCounts: []float64{1}, TimeSpend: 60},
		{ID: 3, Name: "Y from X", Type: 2, Factories: []int{2302}, Items: []int{1, 2}, ItemCounts: []float64{1, .5}, Results: []int{3}, ResultCounts: []float64{1}, TimeSpend: 60},
	}
	return &Dataset{Info: DatasetInfo{Key: "memory-cycle"}, Items: items, Recipes: recipes, ByID: map[int]Item{1: raw, 2: x, 3: y}, ByName: map[string]Item{"R": raw, "X": x, "Y": y}, Choices: map[string][]int{"R": {0}, "X": {1}, "Y": {2}}, Order: []string{"R", "X", "Y"}}
}
