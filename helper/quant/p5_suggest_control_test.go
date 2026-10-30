package quant

// 文件职责：验证 P5a 枚举、失败、预算与推荐状态机；使用：同包函数参数注入；边界：无生产全局测试钩子，不等待真实二十秒。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"
)

// p5Balanced 职责：构造可控机器数且数值闭合的结果；使用：状态/排序测试；边界：不冒充真实 Solve 或游戏产能。
func p5Balanced(machines int) Result {
	return Result{Targets: []Target{{ItemID: 1101, RatePerMinute: 60}}, MachineCount: machines,
		RawInputs:   []ItemRate{{ItemID: 1001, RatePerMinute: 60}},
		Productions: []Production{{MachineCount: machines, ExactMachineCount: 1, Inputs: []ItemRate{{ItemID: 1001, RatePerMinute: 60}}, Outputs: []ItemRate{{ItemID: 1101, RatePerMinute: 60}}}}}
}

// TestP5AuthorizationAndEnumeration 职责：验证授权边界、位掩码与归一去重；使用：冻结规则1/2；边界：只修改局部数据副本。
func TestP5AuthorizationAndEnumeration(t *testing.T) {
	d := p5Dataset(t)
	for _, tc := range []struct {
		mask  int
		modes []int
	}{{0, nil}, {1, []int{1, 1, 1}}, {2, []int{2, 2, 2}}, {3, []int{1, 1, 1, 2, 2, 2}}, {4, nil}, {8, nil}} {
		copyD := *d
		copyD.Recipes = suggestCloneSlice(d.Recipes)
		copyD.Recipes[0].Proliferator = tc.mask
		got, err := suggestChoices(&copyD, p5Iron(60), []int{0})
		if err != nil || len(got) != len(tc.modes) {
			t.Fatalf("mask %d: %v %v", tc.mask, got, err)
		}
		for i, c := range got {
			if c.Mode != tc.modes[i] {
				t.Fatalf("mask %d mode=%d", tc.mask, c.Mode)
			}
		}
	}
	for _, original := range []RecipeScheme{{IncreasePoints: 4}, {ProductionMode: 1}, {ProductionMode: 1, IncreasePoints: 4}} {
		req := p5Iron(60)
		req.Scheme.SchemeForRecipe = []RecipeScheme{original}
		got, err := suggestChoices(d, req, []int{0})
		if err != nil || len(got) != 6 {
			t.Fatalf("normalization: %v %v", got, err)
		}
		if original.ProductionMode == 1 {
			for _, c := range got {
				if c.Mode == 1 && c.Points == 4 {
					t.Fatal("default mode1/0 was not deduplicated against mode1/4")
				}
			}
		} else if got[0].Mode == 0 {
			t.Fatal("mode0 residual points were not normalized")
		}
	}
	for i, r := range d.Recipes {
		if r.Proliferator == 8 {
			got, err := suggestChoices(d, p5Iron(60), []int{i})
			if err != nil || len(got) != 0 {
				t.Fatal("fractionation generated a disabling candidate")
			}
			calls := 0
			out, err := suggestWithSolver(context.Background(), p5Iron(60), []int{i}, func(context.Context, Request) (Result, error) {
				calls++
				return p5Balanced(1), nil
			})
			if err != nil || calls != 1 || out.State != SuggestCompleted || out.Eligible != 0 || out.Recommendation != nil {
				t.Fatal("zero eligible did not complete without recommendation")
			}
		}
	}
	for _, auth := range [][]int{{0, 0}, {-1}, {len(d.Recipes)}, make([]int, 65)} {
		calls := 0
		out, err := suggestWithSolver(context.Background(), p5Iron(60), auth, func(context.Context, Request) (Result, error) { calls++; return Result{}, nil })
		if err == nil || calls != 0 || out.Recommendation != nil || len(out.Candidates) != 0 {
			t.Fatalf("invalid auth accepted: %v %+v %v", auth, out, err)
		}
	}
	indices := make([]int, 64)
	for i := range indices {
		indices[i] = i
	}
	if _, err := suggestChoices(d, p5Iron(60), indices); err != nil {
		t.Fatalf("64 valid authorizations rejected: %v", err)
	}
	if _, err := Suggest(nil, p5Iron(60), nil); err == nil {
		t.Fatal("nil context accepted")
	}
	req := p5Iron(60)
	req.Dataset = "TheyComeFromVoid"
	if out, err := Suggest(context.Background(), req, nil); err == nil || out.Recommendation != nil {
		t.Fatal("other dataset accepted")
	}
	for _, auth := range [][]int{nil, {}, {1, 0}} {
		before := suggestCloneSlice(auth)
		calls := 0
		out, err := suggestWithSolver(context.Background(), p5Iron(60), auth, func(context.Context, Request) (Result, error) { calls++; return p5Balanced(10 - calls), nil })
		if err != nil || !reflect.DeepEqual(auth, before) {
			t.Fatalf("auth mutation/error: %v", err)
		}
		if len(auth) == 0 {
			if calls != 1 || out.State != SuggestCompleted || out.Planned != 0 || out.Recommendation != nil {
				t.Fatalf("empty auth: %+v", out)
			}
		} else {
			if calls != 9 || out.Eligible != 12 || out.Planned != 8 || out.Attempted != 8 || out.Succeeded != 8 || out.Omitted != 4 || out.State != SuggestLimited || out.Recommendation != nil {
				t.Fatalf("limit: %+v", out)
			}
			for i, c := range out.Candidates {
				if c.RecipeIndex != i/6 {
					t.Fatal("authorization not sorted before truncation")
				}
			}
		}
	}
}

// TestP5ZeroFillAndBaselineErrors 职责：验证仅单项变更、零填充与原始基线错误；使用：请求兼容与非法值；边界：失败不返回伪候选。
func TestP5ZeroFillAndBaselineErrors(t *testing.T) {
	d := p5Dataset(t)
	index := 1
	req := p5Iron(60)
	calls := 0
	_, err := suggestWithSolver(context.Background(), req, []int{index}, func(_ context.Context, in Request) (Result, error) {
		calls++
		if calls == 1 {
			if in.Scheme.SchemeForRecipe != nil {
				t.Fatal("baseline was padded")
			}
		} else {
			if len(in.Scheme.SchemeForRecipe) != index+1 || in.Scheme.SchemeForRecipe[0] != (RecipeScheme{}) {
				t.Fatal("short scheme not zero filled")
			}
		}
		return p5Balanced(1), nil
	})
	if err != nil || calls != 7 {
		t.Fatalf("padding test: %v calls=%d", err, calls)
	}
	baseErr := qerror("Q_E_NO_RECIPE", "baseline")
	out, err := suggestWithSolver(context.Background(), req, []int{0}, func(context.Context, Request) (Result, error) { return Result{}, baseErr })
	if err != baseErr || len(out.Candidates) != 0 || out.Baseline != nil || out.Recommendation != nil {
		t.Fatalf("baseline error lost: %+v %v", out, err)
	}
	for _, bad := range []Request{
		{}, p5Iron(math.NaN()), p5Iron(math.Inf(1)), p5Iron(-1),
		{Targets: req.Targets, Scheme: Scheme{CostWeight: CostWeight{BuildingCost: map[string]float64{"unused": math.NaN()}}}},
		{Targets: req.Targets, Settings: Settings{AccRate: math.Inf(1)}},
		{Targets: req.Targets, Settings: Settings{NaturalProductionLine: []NaturalProduction{{TargetItem: d.ByID[1101].Name, RecipeChoice: 1, MachineCount: math.MaxFloat64}}}},
	} {
		out, err := Suggest(context.Background(), bad, nil)
		if err == nil || out.Recommendation != nil || out.Baseline != nil || len(out.Candidates) != 0 {
			t.Fatalf("bad input: %+v %v", out, err)
		}
	}
}

// TestP5FailuresAndRecommendationGate 职责：冻结部分失败及不可判定不推荐；使用：受控错误和物理变体；边界：单次超时仍继续后续候选。
func TestP5FailuresAndRecommendationGate(t *testing.T) {
	for _, failure := range []error{qerror("Q_E_INFEASIBLE", "candidate"), qerror("Q_E_TIMEOUT", "single solve 5s"), fmt.Errorf("wrapped: %w", qerror("Q_E_BAD_REQUEST", "wrapped")), errors.New("unknown")} {
		calls := 0
		out, err := suggestWithSolver(context.Background(), p5Iron(60), []int{0}, func(ctx context.Context, _ Request) (Result, error) {
			calls++
			if calls == 3 {
				if ctx.Err() != nil {
					t.Fatal("fixture total context expired")
				}
				return Result{}, failure
			}
			return p5Balanced(11 - calls), nil
		})
		wantCode := "Q_E_INTERNAL"
		var qe *Error
		if errors.As(failure, &qe) {
			wantCode = qe.Code
		}
		if err != nil || out.State != SuggestCompleted || out.Attempted != 6 || out.Succeeded != 5 || out.Failed != 1 || len(out.Candidates) != 5 || len(out.Failures) != 1 || out.Failures[0].Points != 2 || out.Failures[0].Code != wantCode || out.Recommendation != nil {
			t.Fatalf("failure handling: %+v %v", out, err)
		}
	}
	for _, badCall := range []int{1, 2, 7} {
		for _, defect := range []string{"warning", "machines", "residual", "nonfinite"} {
			calls := 0
			out, err := suggestWithSolver(context.Background(), p5Iron(60), []int{0}, func(context.Context, Request) (Result, error) {
				calls++
				r := p5Balanced(11 - calls)
				if calls == badCall {
					switch defect {
					case "warning":
						r.Warnings = []string{"w"}
					case "machines":
						r.MachineCount++
					case "residual":
						r.RawInputs[0].RatePerMinute++
					case "nonfinite":
						r.LPCost = math.NaN()
					}
				}
				return r, nil
			})
			if err != nil || out.Succeeded != 6 || out.State != SuggestCompleted || out.Recommendation != nil {
				t.Fatalf("bad call %d %s: %+v %v", badCall, defect, out, err)
			}
		}
	}
	// 同机器数不推荐；全部候选一致下降时按 mode/points 稳定选首项。
	for _, improve := range []bool{false, true} {
		calls := 0
		out, err := suggestWithSolver(context.Background(), p5Iron(60), []int{0}, func(context.Context, Request) (Result, error) {
			calls++
			count := 10
			if calls > 1 && improve {
				count = 9
			}
			return p5Balanced(count), nil
		})
		if err != nil || (out.Recommendation != nil) != improve {
			t.Fatalf("improvement gate: %+v %v", out, err)
		}
		if improve && (out.Recommendation.Mode != 1 || out.Recommendation.Points != 1) {
			t.Fatal("tie not broken by numeric tuple")
		}
	}
}

// TestP5StableScoreOrder 职责：独立验证所有排序层级；使用：打乱候选后排序；边界：不重排返回评估序列。
func TestP5StableScoreOrder(t *testing.T) {
	in := []SuggestCandidate{
		{SuggestChoice: SuggestChoice{2, 2, 4}, Result: Result{MachineCount: 4}},
		{SuggestChoice: SuggestChoice{2, 2, 1}, Result: Result{MachineCount: 4}},
		{SuggestChoice: SuggestChoice{2, 1, 4}, Result: Result{MachineCount: 4}},
		{SuggestChoice: SuggestChoice{1, 2, 4}, Result: Result{MachineCount: 4}},
		{SuggestChoice: SuggestChoice{9, 2, 4}, Result: Result{MachineCount: 3}},
	}
	sort.SliceStable(in, func(i, j int) bool { return suggestLess(in[i], in[j]) })
	want := []SuggestChoice{{9, 2, 4}, {1, 2, 4}, {2, 1, 4}, {2, 2, 1}, {2, 2, 4}}
	for i, c := range in {
		if c.SuggestChoice != want[i] {
			t.Fatalf("score order: %+v", in)
		}
	}
}

// TestP5ContextLifecycle 职责：验证父/总 context 优先级、迟到结果和派生预算；使用：可控取消/短 deadline；边界：无脱离调用生命周期的 goroutine，不宣称硬实时。
func TestP5ContextLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stopCall   int
		deadline   bool
		solveError bool
	}{
		{"pre-cancel", 0, false, false}, {"baseline-cancel", 1, false, false}, {"baseline-error-cancel", 1, false, true},
		{"candidate-cancel", 2, false, false}, {"candidate-error-cancel", 2, false, true}, {"last-late-success", 7, false, false},
		{"pre-deadline", 0, true, false}, {"baseline-deadline", 1, true, true}, {"candidate-deadline", 2, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			if tc.deadline {
				cancel()
				duration := 20 * time.Millisecond
				if tc.stopCall == 0 {
					duration = -time.Second
				}
				parent, cancel = context.WithTimeout(context.Background(), duration)
			}
			defer cancel()
			if tc.stopCall == 0 && !tc.deadline {
				cancel()
			}
			calls := 0
			out, err := suggestWithSolver(parent, p5Iron(60), []int{0}, func(ctx context.Context, _ Request) (Result, error) {
				calls++
				if calls == tc.stopCall {
					if tc.deadline {
						<-ctx.Done()
					} else {
						cancel()
					}
					if tc.solveError {
						return Result{}, qerror("Q_E_TIMEOUT", "late error")
					}
				}
				return p5Balanced(11 - calls), nil
			})
			want := SuggestCanceled
			if tc.deadline {
				want = SuggestTimeout
			}
			if err != nil || out.State != want || calls != tc.stopCall || out.Recommendation != nil {
				t.Fatalf("context priority: %+v calls=%d err=%v", out, calls, err)
			}
			if tc.stopCall <= 1 && out.Baseline != nil {
				t.Fatal("late baseline exposed")
			}
			if out.Attempted != max(0, tc.stopCall-1) || out.Succeeded != max(0, tc.stopCall-2) || out.Failed != 0 {
				t.Fatalf("context counters: %+v", out)
			}
		})
	}
	for _, earlier := range []bool{false, true} {
		parent := context.Background()
		cancel := func() {}
		if earlier {
			parent, cancel = context.WithTimeout(parent, time.Second)
		}
		defer cancel()
		_, err := suggestWithSolver(parent, p5Iron(60), nil, func(ctx context.Context, _ Request) (Result, error) {
			deadline, ok := ctx.Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining > 20*time.Second || (!earlier && remaining < 19*time.Second) || (earlier && remaining > time.Second) {
				t.Fatalf("incorrect budget: %v %v", ok, remaining)
			}
			return p5Balanced(1), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestP5FixedLineSingleTermTolerance 职责：验证多条固定线的最大单项容差；使用：同向及反向内存账本；边界：每条净项只计一次，不按聚合后的净项定容差。
func TestP5FixedLineSingleTermTolerance(t *testing.T) {
	d := &Dataset{ByID: map[int]Item{1: {ID: 1, Name: "A"}, 2: {ID: 2, Name: "B"}, 2302: {ID: 2302, Name: "f", Speed: 1}}, ByName: map[string]Item{"A": {ID: 1, Name: "A"}, "B": {ID: 2, Name: "B"}, "f": {ID: 2302, Name: "f", Speed: 1}}, Choices: map[string][]int{"A": {0}, "B": {1}}, Recipes: []Recipe{
		{Factories: []int{2302}, Items: []int{2}, ItemCounts: []float64{1}, Results: []int{1}, ResultCounts: []float64{1}, TimeSpend: 60},
		{Factories: []int{2302}, Items: []int{1}, ItemCounts: []float64{1}, Results: []int{2}, ResultCounts: []float64{1}, TimeSpend: 60},
	}}
	line := NaturalProduction{TargetItem: "A", RecipeChoice: 1, MachineCount: 1}
	for _, delta := range []float64{3e-5, 9e-5} {
		req := Request{Settings: Settings{NaturalProductionLine: []NaturalProduction{line, line}}}
		out := Result{RawInputs: []ItemRate{{ItemID: 2, RatePerMinute: 120}}, Targets: []Target{{ItemID: 1, RatePerMinute: 60}, {ItemID: 1, RatePerMinute: 60 + delta}}}
		if suggestAssess(d, req, out) != (delta < 6e-5) {
			t.Fatalf("same direction delta=%g", delta)
		}
		req.Settings.NaturalProductionLine[1].TargetItem = "B"
		out = Result{RawInputs: []ItemRate{{ItemID: 1, RatePerMinute: delta}}}
		if suggestAssess(d, req, out) != (delta < 6e-5) {
			t.Fatalf("opposing direction delta=%g", delta)
		}
	}
}
