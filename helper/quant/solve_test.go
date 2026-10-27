package quant

// 文件职责：验证参考算例和 LP 边界；使用：go test；边界：外部 Node golden 必须由独立参考实现生成。

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// TestDatasetAndBlueMatrix 职责：核对数据数量与蓝糖基本比例；使用：P1 回归；边界：不声明游戏实际产能。
func TestDatasetAndBlueMatrix(t *testing.T) {
	all, err := ListDatasets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 12 {
		t.Fatal(len(all))
	}
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Items) != 175 || len(d.Recipes) != 239 {
		t.Fatal(d.Info)
	}
	r, err := Solve(context.Background(), Request{Targets: []Target{{ItemID: 6001, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("machines=%d power=%g amounts=%v key=%v", r.MachineCount, r.PowerMW, r.Amounts, r.KeyItems)
	if math.Abs(r.Amounts["电磁矩阵"]-60) > 1e-7 {
		t.Fatal(r.Amounts)
	}
	if len(r.Productions) < 3 {
		t.Fatal(r.Productions)
	}
	again, err := Solve(context.Background(), Request{Targets: []Target{{ItemID: 6001, RatePerMinute: 60}}})
	if err != nil || again.PlanID != r.PlanID {
		t.Fatal("non deterministic", err)
	}
}

// TestSimplexFeasibility 职责：用手算验证目标值和不可行性；使用：独立 LP 回归；边界：测试模型不来自引擎自身。
func TestSimplexFeasibility(t *testing.T) {
	x, cost, err := minimize(context.Background(), [][]float64{{1, 2}, {3, 1}}, []float64{4, 3}, []float64{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(x[0]-.4) > 1e-8 || math.Abs(x[1]-1.8) > 1e-8 || math.Abs(cost-2.2) > 1e-8 {
		t.Fatal(x, cost)
	}
	if _, _, err := minimize(context.Background(), [][]float64{{1}, {-1}}, []float64{2, -1}, []float64{1}); err == nil {
		t.Fatal("infeasible accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := minimize(ctx, [][]float64{{1}}, []float64{1}, []float64{1}); err == nil {
		t.Fatal("canceled accepted")
	}
}

// TestReferenceGolden 职责：与 Node 参考结果逐项对拍；使用：设置 DSBOX_QUANT_GOLDEN；边界：无 golden 时明确跳过。
func TestReferenceGolden(t *testing.T) {
	path := os.Getenv("DSBOX_QUANT_GOLDEN")
	if path == "" {
		t.Skip("设置 DSBOX_QUANT_GOLDEN 运行 Node 对拍")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name        string             `json:"name"`
		Request     Request            `json:"request"`
		Amounts     map[string]float64 `json:"amounts"`
		Leftovers   map[string]float64 `json:"leftovers"`
		Keys        []string           `json:"keys"`
		Productions []Production       `json:"productions"`
		PowerMW     float64            `json:"powerMw"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Solve(context.Background(), tc.Request)
			if err != nil {
				t.Fatal(err)
			}
			for name, want := range tc.Amounts {
				value := got.Amounts[name]
				if math.Abs(value-want) > 1e-6*math.Max(1, math.Abs(want)) {
					t.Errorf("%s got=%g reference=%g", name, value, want)
				}
			}
			for name, value := range got.Amounts {
				if _, ok := tc.Amounts[name]; !ok && math.Abs(value) > 1e-6 {
					t.Errorf("extra %s=%g", name, value)
				}
			}
			d, _ := LoadDataset(tc.Request.Dataset)
			if len(got.Productions) != len(tc.Productions) {
				t.Errorf("production count got=%d reference=%d", len(got.Productions), len(tc.Productions))
			}
			for _, want := range tc.Productions {
				found := false
				for _, p := range got.Productions {
					if p.MainItemID != want.MainItemID {
						continue
					}
					found = true
					if p.RecipeID != want.RecipeID || p.MachineID != want.MachineID || math.Abs(p.ExactMachineCount-want.ExactMachineCount) > 1e-6*math.Max(1, want.ExactMachineCount) || abs(p.MachineCount-want.MachineCount) > 1 {
						t.Errorf("machine bridge got=%+v reference=%+v", p, want)
					}
				}
				if !found {
					t.Errorf("missing production %+v", want)
				}
			}
			if math.Abs(got.PowerMW-tc.PowerMW) > 1e-6*math.Max(1, tc.PowerMW) {
				t.Errorf("power got=%g reference=%g", got.PowerMW, tc.PowerMW)
			}
			if len(got.KeyItems) != len(tc.Keys) {
				t.Errorf("keys count got=%v reference=%v", got.KeyItems, tc.Keys)
			} else {
				for i, key := range tc.Keys {
					if got.KeyItems[i] != d.ByName[key].ID {
						t.Errorf("keys got=%v reference=%v", got.KeyItems, tc.Keys)
						break
					}
				}
			}
			left := map[string]float64{}
			for _, v := range got.Leftovers {
				left[v.Name] = v.RatePerMinute
			}
			for name, want := range tc.Leftovers {
				if math.Abs(left[name]-want) > 1e-6*math.Max(1, math.Abs(want)) {
					t.Errorf("leftover %s=%g reference=%g", name, left[name], want)
				}
			}
		})
	}
}

// abs 职责：计算取整台数误差；使用：对拍断言；边界：只处理测试规模整数。
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
