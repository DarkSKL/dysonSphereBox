package quant

// 文件职责：提供不依赖前端、HiGHS 或在线服务的 P2 固定行为夹具；使用：离线回归和对拍前置检查；边界：夹具结果不是官方游戏真值，不覆盖蓝图或真实游戏。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"testing"
)

// TestP2Fixture 职责：固定普通配方、增产、采集倍率和副产物数值；使用：P2 离线回归。
// 使用：每次 P2 修改后运行 `go test ./helper/quant -run TestP2Fixture`。
// 边界：参考项目结果因模型/数据/许可边界不一致时只记录不可比，不把夹具当官方真值。
func TestP2Fixture(t *testing.T) {
	d, err := LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	iron := d.ByName["铁块"]
	spray := d.ByID[1143].Name
	near := func(t *testing.T, field string, got, want float64) {
		t.Helper()
		if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: got %.12g want %.12g", field, got, want)
		}
	}
	rate := func(flows []ItemRate, item string) float64 {
		for _, flow := range flows {
			if flow.Name == item {
				return flow.RatePerMinute
			}
		}
		return 0
	}
	t.Run("ordinary recipe", func(t *testing.T) {
		result, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: iron.ID, RatePerMinute: 60.24}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Productions) != 1 || result.Productions[0].MachineCount != 2 {
			t.Fatalf("ordinary fixture changed: %+v", result.Productions)
		}
		near(t, "exact devices", result.Productions[0].ExactMachineCount, 1.004)
		near(t, "ore", rate(result.RawInputs, "铁矿"), 60.24)
	})
	for _, fixture := range []struct {
		mode              int
		exact, ore, power float64
	}{{1, .5, 60, .45}, {2, .8, 48, .72}} {
		t.Run(map[int]string{1: "acceleration", 2: "extra output"}[fixture.mode], func(t *testing.T) {
			index, ok := d.RecipeByID[1]
			if !ok {
				t.Fatal("iron recipe missing")
			}
			scheme := Scheme{SchemeForRecipe: make([]RecipeScheme, len(d.Recipes))}
			scheme.SchemeForRecipe[index] = RecipeScheme{ProductionMode: fixture.mode, IncreasePoints: 4}
			result, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: iron.ID, RatePerMinute: 60}}, Scheme: scheme, Settings: Settings{MineralizeList: map[string]bool{spray: true}}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Productions) != 1 || result.Productions[0].ProliferatorMode != fixture.mode || result.Productions[0].IncreasePoints != 4 {
				t.Fatalf("proliferator fixture did not preserve mode: %+v", result.Productions)
			}
			near(t, "exact devices", result.Productions[0].ExactMachineCount, fixture.exact)
			near(t, "ore", rate(result.Productions[0].Inputs, "铁矿"), fixture.ore)
			near(t, "self-sprayed cost", rate(result.Productions[0].Inputs, spray), fixture.ore/74)
			near(t, "power MW", result.PowerMW, fixture.power)
		})
	}
	t.Run("mining multiplier", func(t *testing.T) {
		base, err := makeGraph(d, Scheme{}, Settings{})
		if err != nil {
			t.Fatal(err)
		}
		settings := Settings{MiningSpeedMultiple: 2}
		boosted, err := makeGraph(d, Scheme{}, settings)
		if err != nil {
			t.Fatal(err)
		}
		near(t, "mining throughput ratio", boosted.nodes["铁矿"].rate/base.nodes["铁矿"].rate, 2)
		result, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: d.ByName["铁矿"].ID, RatePerMinute: 60}}, Settings: settings})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.RawInputs) != 1 || math.Abs(result.RawInputs[0].RatePerMinute-60) > 1e-9 {
			t.Fatalf("mining fixture changed: %+v", result.RawInputs)
		}
	})
	t.Run("byproduct", func(t *testing.T) {
		result, err := Solve(context.Background(), Request{Dataset: "Vanilla", Targets: []Target{{ItemID: d.ByName["精炼油"].ID, RatePerMinute: 60}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Leftovers) == 0 {
			t.Fatalf("byproduct fixture lost leftovers: %+v", result)
		}
		near(t, "hydrogen surplus", rate(result.Leftovers, "氢"), 30)
		near(t, "crude input", rate(result.RawInputs, "原油"), 60)
		if len(result.Productions) != 1 {
			t.Fatalf("unexpected refining chain: %+v", result.Productions)
		}
		near(t, "refinery devices", result.Productions[0].ExactMachineCount, 2)
	})
	t.Run("lens coefficients only", func(t *testing.T) {
		index, ok := d.RecipeByID[21208]
		if !ok {
			t.Fatal("lens recipe missing")
		}
		choice := 0
		for i, recipe := range d.Choices["临界光子"] {
			if recipe == index {
				choice = i + 1
			}
		}
		if choice == 0 {
			t.Fatal("lens recipe not selectable")
		}
		scheme := Scheme{ItemRecipeChoices: map[string]int{"临界光子": choice}, SchemeForRecipe: make([]RecipeScheme, len(d.Recipes))}
		base, err := makeGraph(d, scheme, Settings{})
		if err != nil {
			t.Fatal(err)
		}
		scheme.SchemeForRecipe[index] = RecipeScheme{ProductionMode: 3, IncreasePoints: 4}
		boosted, err := makeGraph(d, scheme, Settings{})
		if err != nil {
			t.Fatal(err)
		}
		near(t, "unsprayed lens/min", base.nodes["临界光子"].inputs.values["引力透镜"]*60, .5)
		near(t, "sprayed lens/min", boosted.nodes["临界光子"].inputs.values["引力透镜"]*60, .25)
		near(t, "lens spray/min", boosted.nodes["临界光子"].inputs.values[spray]*60, .25/74)
	})
}

// TestP2DatasetManifest 职责：校验全部内置数据键、SHA-256 和数量；使用：离线来源复核；边界：不裁定第三方数据哪一方正确。
func TestP2DatasetManifest(t *testing.T) {
	var manifest map[string]struct {
		SHA256  string `json:"sha256"`
		Items   int    `json:"items"`
		Recipes int    `json:"recipes"`
	}
	raw, err := dataFiles.ReadFile("data/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	datasets, err := ListDatasets()
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(datasets))
	for _, dataset := range datasets {
		keys = append(keys, dataset.Key)
	}
	sort.Strings(keys)
	manifestKeys := make([]string, 0, len(manifest))
	for key := range manifest {
		manifestKeys = append(manifestKeys, key)
	}
	sort.Strings(manifestKeys)
	if len(keys) != len(manifestKeys) {
		t.Fatalf("manifest dataset count mismatch: embedded=%v manifest=%v", keys, manifestKeys)
	}
	for i := range keys {
		if keys[i] != manifestKeys[i] {
			t.Fatalf("manifest dataset keys mismatch: embedded=%v manifest=%v", keys, manifestKeys)
		}
	}
	for key, expected := range manifest {
		data, err := dataFiles.ReadFile("data/" + key + ".json")
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != expected.SHA256 {
			t.Errorf("%s hash mismatch: got %s want %s", key, got, expected.SHA256)
		}
		dataset, err := LoadDataset(key)
		if err != nil {
			t.Fatal(err)
		}
		if len(dataset.Items) != expected.Items || len(dataset.Recipes) != expected.Recipes {
			t.Errorf("%s counts changed: got %d/%d want %d/%d", key, len(dataset.Items), len(dataset.Recipes), expected.Items, expected.Recipes)
		}
	}
}

// BenchmarkP2Solve 职责：记录典型目标的求解耗时和分配；使用：P2 本机性能基线；边界：不证明全局最优或跨机器性能。
func BenchmarkP2Solve(b *testing.B) {
	for name, targets := range map[string][]Target{
		"iron":          {{ItemID: 1101, RatePerMinute: 60.24}},
		"multi-product": {{ItemID: 1115, RatePerMinute: 60}, {ItemID: 1120, RatePerMinute: 60}},
	} {
		b.Run(name, func(b *testing.B) {
			request := Request{Dataset: "Vanilla", Targets: targets}
			if _, err := Solve(context.Background(), request); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Solve(context.Background(), request); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
