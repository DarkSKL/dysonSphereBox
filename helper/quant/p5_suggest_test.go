package quant

// 文件职责：验证 P5a 真实求解、账本及隔离；使用：离线量化回归；边界：无网络/游戏，不作为全局最优或蓝图证据。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// p5Dataset 职责：加载只读 Vanilla 测试数据；使用：P5a 夹具；边界：测试不能修改缓存原对象。
func p5Dataset(t *testing.T) *Dataset {
	t.Helper()
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// p5Iron 职责：创建铁块请求；使用：真实收益与无收益回归；边界：索引从数据反查而非假定配方 ID。
func p5Iron(rate float64) Request {
	return Request{Targets: []Target{{ItemID: 1101, RatePerMinute: rate}}}
}

// TestP5RealIron 职责：核对喷剂供应链开销和整数机器评分；使用：真实 Solve 定向场景；边界：不以 LP 成本或功率排名。
func TestP5RealIron(t *testing.T) {
	d := p5Dataset(t)
	index := d.Choices[d.ByID[1101].Name][0]
	for _, tc := range []struct {
		rate      float64
		base      int
		want      []int
		recommend bool
	}{
		{60.24, 2, []int{2, 5, 11, 2, 5, 11}, false},
		{6000, 100, []int{86, 93, 95, 94, 108, 117}, true},
	} {
		t.Run(fmt.Sprintf("rate_%g", tc.rate), func(t *testing.T) {
			req := p5Iron(tc.rate)
			before := suggestCloneRequest(req)
			got, err := Suggest(context.Background(), req, []int{index})
			if err != nil || got.State != SuggestCompleted || got.Eligible != 6 || got.Planned != 6 || got.Attempted != 6 || got.Succeeded != 6 || got.Failed != 0 || got.Omitted != 0 {
				t.Fatalf("unexpected outcome: %+v %v", got, err)
			}
			if !reflect.DeepEqual(req, before) || got.Baseline.MachineCount != tc.base || !got.BaselineAssessable {
				t.Fatalf("baseline or input mismatch: %+v", got.Baseline)
			}
			for i, c := range got.Candidates {
				directRequest := suggestCloneRequest(req)
				directRequest.Scheme.SchemeForRecipe = make([]RecipeScheme, index+1)
				directRequest.Scheme.SchemeForRecipe[index] = RecipeScheme{ProductionMode: c.Mode, IncreasePoints: c.Points}
				direct, directErr := Solve(context.Background(), directRequest)
				if directErr != nil || !reflect.DeepEqual(direct, c.Result) {
					t.Fatalf("candidate %d differs from full direct Solve: %v", i, directErr)
				}
				if !c.Assessable {
					t.Fatalf("candidate %d ledger did not close: %+v", i, c)
				}
				// 整数 oracle 同时覆盖喷剂整条供应链，不能只计算主链设备。
				if c.Result.MachineCount != tc.want[i] {
					t.Fatalf("candidate %d machines=%d want %d", i, c.Result.MachineCount, tc.want[i])
				}
				if len(c.Result.Productions) <= len(got.Baseline.Productions) {
					t.Fatal("spray upstream was not fully solved")
				}
			}
			if (got.Recommendation != nil) != tc.recommend {
				t.Fatalf("wrong recommendation: %+v", got.Recommendation)
			}
			if got.Recommendation != nil && (got.Recommendation.Mode != 1 || got.Recommendation.Points != 1 || got.Recommendation.Result.MachineCount != 86) {
				t.Fatalf("wrong best candidate: %+v", got.Recommendation)
			}
			if tc.rate == 60.24 && got.Baseline.Productions[0].MachineCount != 2 {
				t.Fatal("60.24 rounding regressed")
			}
			again, err := Suggest(context.Background(), req, []int{index})
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("repeat search differs")
			}
		})
	}
}

// TestP5NaturalAndJointLedger 职责：验证固定线净需求符号、倍率、喷剂与真实联产/循环；使用：第5条守恒门禁；边界：只核对 Go 公开投影。
func TestP5NaturalAndJointLedger(t *testing.T) {
	d := p5Dataset(t)
	for _, mode := range []int{0, 1, 2} {
		req := p5Iron(6000)
		req.Settings = Settings{AccRate: 2, IncRate: 2, NaturalProductionLine: []NaturalProduction{{TargetItem: d.ByID[1101].Name, RecipeChoice: 1, MachineCount: 1, ProductionMode: mode, IncreasePoints: 4}}}
		got, err := Suggest(context.Background(), req, []int{d.Choices[d.ByID[1101].Name][0]})
		if err != nil || !got.BaselineAssessable || got.State != SuggestCompleted {
			t.Fatalf("fixed mode %d not closed: %+v %v", mode, got, err)
		}
		for _, candidate := range got.Candidates {
			if !candidate.Assessable {
				t.Fatalf("fixed mode %d candidate not closed: %+v", mode, candidate)
			}
		}
		// 独立手算：已有铁块 mode1/4/Acc=2 是 240/min；mode2/4/Inc=2 是 150/min，矿60/min。
		wantOut, wantIn := 60., 60.
		if mode == 1 {
			wantOut, wantIn = 240, 240
		}
		if mode == 2 {
			wantOut = 150
		}
		settings, _ := normalizeSettings(req.Settings)
		needs := newRates()
		g := &graph{dataset: d, settings: settings}
		if err := g.balanceNatural(needs); err != nil {
			t.Fatal(err)
		}
		if math.Abs(needs.values[d.ByID[1101].Name]+wantOut) > 1e-9 || math.Abs(needs.values[d.ByID[1001].Name]-wantIn) > 1e-9 {
			t.Fatalf("fixed sign/multiplier mismatch: %+v", needs.values)
		}
		if mode > 0 && needs.values[d.ByID[1143].Name] <= 0 {
			t.Fatal("fixed spray missing")
		}
		missingFixed := suggestCloneRequest(req)
		missingFixed.Settings.NaturalProductionLine = nil
		if suggestAssess(d, missingFixed, *got.Baseline) {
			t.Fatal("ledger ignored fixed production")
		}
	}
	for _, name := range []string{"塑料", "石墨烯", "重氢"} {
		req := Request{Targets: []Target{{ItemID: d.ByName[name].ID, RatePerMinute: 60}}}
		out, err := Suggest(context.Background(), req, nil)
		if err != nil || !out.BaselineAssessable {
			t.Fatalf("joint/cycle %s: %+v %v", name, out, err)
		}
	}
	// 固定炼油联产同时产出精炼油和氢，仍只计一次净需求。
	name := "精炼油"
	req := Request{Targets: []Target{{ItemID: d.ByName[name].ID, RatePerMinute: 600}, {ItemID: d.ByName["氢"].ID, RatePerMinute: 600}}, Settings: Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: name, RecipeChoice: 1, MachineCount: 1, ProductionMode: 2, IncreasePoints: 4}}}}
	out, err := Suggest(context.Background(), req, nil)
	if err != nil || !out.BaselineAssessable {
		t.Fatalf("fixed joint: %+v %v", out, err)
	}
}

// TestP5LedgerRejections 职责：冻结严格误差与所有拒绝条件；使用：合成结果和真实结果变体；边界：不修改缓存数据。
func TestP5LedgerRejections(t *testing.T) {
	d := p5Dataset(t)
	req := p5Iron(60)
	base, err := Solve(context.Background(), req)
	if err != nil || !suggestAssess(d, req, base) {
		t.Fatalf("fixture invalid %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Result)
	}{
		{"warning", func(r *Result) { r.Warnings = []string{"external facility"} }},
		{"total", func(r *Result) { r.MachineCount++ }},
		{"under-exact", func(r *Result) { r.Productions[0].ExactMachineCount = 1 + 2e-9 }},
		{"nonfinite-power", func(r *Result) { r.PowerMW = math.Inf(1) }},
		{"nonfinite-lp", func(r *Result) { r.LPCost = math.NaN() }},
		{"nonfinite-amount", func(r *Result) { r.Amounts["unused"] = math.NaN() }},
		{"nonfinite-target", func(r *Result) { r.Targets[0].RatePerMinute = math.NaN() }},
		{"nonfinite-production", func(r *Result) { r.Productions[0].RoundingMargin = math.Inf(-1) }},
		{"nonfinite-flow", func(r *Result) { r.RawInputs[0].RatePerMinute = math.Inf(1) }},
		{"unknown-flow", func(r *Result) { r.RawInputs[0].ItemID = -77 }},
		{"residual", func(r *Result) { r.Productions[0].Outputs[0].RatePerMinute += .001 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(base)
			var result Result
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&result)
			if suggestAssess(d, req, result) {
				t.Fatal("invalid result was assessable")
			}
		})
	}
	// 以单项流量1直接构造精确可表述的阈值：<1e-6 才可通过，等于也失败。
	for _, tc := range []struct {
		delta float64
		want  bool
	}{{.5e-6, true}, {1e-6, false}, {2e-6, false}} {
		r := Result{RawInputs: []ItemRate{{ItemID: 1001, RatePerMinute: tc.delta}}}
		if suggestAssess(d, Request{}, r) != tc.want {
			t.Fatalf("threshold %.12g", tc.delta)
		}
	}
	r := base
	r.Productions = suggestCloneSlice(base.Productions)
	r.Productions[0].ExactMachineCount += .5e-9
	if !suggestAssess(d, req, r) {
		t.Fatal("machine tolerance within 1e-9 rejected")
	}
	settings := req.Settings
	settings.NaturalProductionLine = []NaturalProduction{{TargetItem: "不存在", RecipeChoice: 1, MachineCount: 1}}
	if suggestAssess(d, Request{Settings: settings}, base) {
		t.Fatal("invalid fixed target accepted")
	}
	settings.NaturalProductionLine[0].TargetItem = d.ByID[1101].Name
	broken := *d
	broken.ByID = suggestCloneMap(d.ByID)
	delete(broken.ByID, 1001)
	if suggestAssess(&broken, Request{Settings: settings}, base) {
		t.Fatal("missing fixed input ID accepted")
	}
	broken = *d
	broken.ByName = suggestCloneMap(d.ByName)
	delete(broken.ByName, d.ByID[1001].Name)
	if suggestAssess(&broken, Request{Settings: settings}, base) {
		t.Fatal("missing fixed reverse ID accepted")
	}
	settings.NaturalProductionLine[0].MachineCount = math.MaxFloat64
	if suggestAssess(d, Request{Settings: settings}, base) {
		t.Fatal("fixed rebuild overflow accepted")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1)} {
		settings.NaturalProductionLine[0].MachineCount = value
		if suggestAssess(d, Request{Settings: settings}, base) {
			t.Fatal("nonfinite fixed line accepted")
		}
	}
}

// TestP5RequestIsolation 职责：逐字段覆盖可变引用和显式零值；使用：真实搜索及恶意私有求解器；边界：不允许调用者并发写同一请求。
func TestP5RequestIsolation(t *testing.T) {
	d := p5Dataset(t)
	zero, no, space := 0, false, 0.
	req := p5Iron(6000)
	req.Dataset = "Vanilla"
	req.Scheme = Scheme{ItemRecipeChoices: map[string]int{d.ByID[1101].Name: 1}, SchemeForRecipe: []RecipeScheme{{Building: 1, IncreasePoints: 4}}, CostWeight: CostWeight{Space: &space, Power: .5, BuildingCost: map[string]float64{"电弧熔炉": 0}, ItemExtraCost: map[string]ItemCost{"x": {Cost: 0, Enabled: false, Cumulative: false, OverflowCost: 0}}}}
	req.Settings = Settings{FixedNum: &zero, ProliferateItself: &no, MineralizeList: map[string]bool{"x": false}, NaturalProductionLine: []NaturalProduction{{TargetItem: d.ByID[1101].Name, RecipeChoice: 1, MachineCount: 1}, {TargetItem: d.ByID[1101].Name, RecipeChoice: 1, MachineCount: 0}}, AccRate: 2, IncRate: 2}
	before := suggestCloneRequest(req)
	datasetBefore, _ := json.Marshal(d)
	if _, err := Suggest(context.Background(), req, []int{0}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, req) {
		t.Fatal("real search mutated request")
	}
	datasetAfter, _ := json.Marshal(d)
	if !reflect.DeepEqual(datasetBefore, datasetAfter) {
		t.Fatal("search mutated dataset")
	}
	calls := 0
	_, err := suggestWithSolver(context.Background(), req, []int{0}, func(_ context.Context, in Request) (Result, error) {
		calls++
		want := suggestCloneRequest(before)
		if calls > 1 {
			pairs := [][2]int{{1, 1}, {1, 2}, {1, 4}, {2, 1}, {2, 2}, {2, 4}}
			want.Scheme.SchemeForRecipe[0].ProductionMode = pairs[calls-2][0]
			want.Scheme.SchemeForRecipe[0].IncreasePoints = pairs[calls-2][1]
		}
		if !reflect.DeepEqual(in, want) {
			t.Fatalf("call %d changed non-patch fields: %+v", calls, in)
		}
		in.Targets[0].RatePerMinute = 9
		in.Scheme.ItemRecipeChoices["x"] = 9
		in.Scheme.SchemeForRecipe[0].Building = 0
		*in.Scheme.CostWeight.Space = 9
		in.Scheme.CostWeight.BuildingCost["x"] = 9
		in.Scheme.CostWeight.ItemExtraCost["x"] = ItemCost{Enabled: true}
		*in.Settings.FixedNum = 8
		*in.Settings.ProliferateItself = true
		in.Settings.MineralizeList["x"] = true
		in.Settings.NaturalProductionLine[0].MachineCount = 9
		return Result{}, nil
	})
	if err != nil || calls != 7 || !reflect.DeepEqual(before, req) {
		t.Fatalf("private isolation failed calls=%d err=%v", calls, err)
	}
	for _, empty := range []Request{{}, {Targets: []Target{}, Scheme: Scheme{SchemeForRecipe: []RecipeScheme{}, ItemRecipeChoices: map[string]int{}}, Settings: Settings{NaturalProductionLine: []NaturalProduction{}, MineralizeList: map[string]bool{}}}} {
		if !reflect.DeepEqual(empty, suggestCloneRequest(empty)) {
			t.Fatal("nil/empty shape lost")
		}
	}
}
