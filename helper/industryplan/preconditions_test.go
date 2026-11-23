package industryplan

// 文件职责：核对必需透镜下界的前提、取整与用户配方；使用：大规模规划的资源预检回归；边界：不把可选燃料或实际布局数量当作最低消耗。

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"

	"DysonSphereBox/helper/quant"
)

// mandatoryTestRequest 职责：建立可独立核对的光子目标；使用：必需物料预检；边界：透镜外供仅用于隔离接收机数量。
func mandatoryTestRequest(rate float64) Request {
	return Request{EnableFuel: true,
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1208, RatePerMinute: rate}}, Settings: quant.Settings{MineralizeList: map[string]bool{"引力透镜": true}}},
		Dyson: DysonSettings{Mode: "sphere", Lenses: true, Supplies: []DysonSupply{{StarID: 1, PlanetIDs: []int64{101}, AvailableMW: 300000, Efficiency: .8, Uptime: 1}}},
	}
}

func TestMandatoryInputsLowerBoundConditions(t *testing.T) {
	disabled := false
	for _, tc := range []struct {
		name    string
		prepare func(*Request)
		want    bool
	}{
		{name: "enabled", want: true},
		{name: "fuel_external", prepare: func(r *Request) { r.EnableFuel = false }},
		{name: "mining_disabled", prepare: func(r *Request) { r.EnableMining = &disabled }},
		{name: "without_lenses", prepare: func(r *Request) { r.Dyson.Lenses = false }},
		{name: "sphere_unconfigured", prepare: func(r *Request) { r.Dyson.Mode = "unconfigured" }},
		{name: "photon_external_mode", prepare: func(r *Request) { r.Dyson.Mode = "external" }},
		{name: "photon_explicit_input", prepare: func(r *Request) { r.Quant.Settings.MineralizeList["临界光子"] = true }},
		{name: "without_photon_demand", prepare: func(r *Request) { r.Quant.Targets = []quant.Target{{ItemID: 1101, RatePerMinute: 60}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := mandatoryTestRequest(12)
			if tc.prepare != nil {
				tc.prepare(&req)
			}
			base, err := quant.Solve(context.Background(), req.Quant)
			if err != nil {
				t.Fatal(err)
			}
			got, added, err := mandatoryInputsLowerBound(context.Background(), req, base)
			if err != nil || added != tc.want || !added && !reflect.DeepEqual(got, base) {
				t.Fatalf("前提判断错误：added=%t error=%v", added, err)
			}
		})
	}
}

func TestMandatoryInputsLowerBoundRounding(t *testing.T) {
	for _, tc := range []struct {
		photons, uptime, lenses float64
	}{
		{.1, 1, 1}, {12, 1, 1}, {math.Nextafter(12, math.Inf(1)), 1, 2},
		{24, 1, 2}, {12, .5, 2}, {18, .5, 3}, {8000, 1, 667},
	} {
		req := mandatoryTestRequest(tc.photons)
		// 不同恒星的连续率不能相加，应使用所选供应中最高的一项推导最少台数。
		req.Dyson.Supplies[0].Uptime = tc.uptime / 2
		req.Dyson.Supplies = append(req.Dyson.Supplies, DysonSupply{StarID: 2, PlanetIDs: []int64{201, 202}, AvailableMW: 300000, Efficiency: .5, Uptime: tc.uptime})
		base, err := quant.Solve(context.Background(), req.Quant)
		if err != nil {
			t.Fatal(err)
		}
		got, added, err := mandatoryInputsLowerBound(context.Background(), req, base)
		if err != nil || !added {
			t.Fatalf("光子 %.17g 的必需透镜未计算：%v", tc.photons, err)
		}
		lenses := 0.0
		for _, raw := range got.RawInputs {
			if raw.ItemID == 1209 {
				lenses += raw.RatePerMinute
			}
		}
		if lenses != tc.lenses || !slices.Contains(got.ExternalInputItemIDs, 1209) {
			t.Fatalf("光子 %.17g、连续率 %g 应需 %g 透镜，得到 %g", tc.photons, tc.uptime, tc.lenses, lenses)
		}
	}
}

func TestMandatoryInputsLowerBoundPreservesRequest(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	req := mandatoryTestRequest(13)
	backing := []quant.Target{{ItemID: 1208, RatePerMinute: 13}, {ItemID: 1209, RatePerMinute: 3}, {ItemID: 1101, RatePerMinute: 777}}
	req.Quant.Targets = backing[:2]
	req.Quant.Settings.DirectResourceItemIDs = []int{1003}
	req.Quant.Settings.MineralizeList = map[string]bool{}
	for _, item := range dataset.Items {
		if item.ID != 1208 && item.ID != 1209 && item.ID != 1003 {
			req.Quant.Settings.MineralizeList[item.Name] = true
		}
	}
	index := dataset.Choices[dataset.ByID[1209].Name][0]
	req.Quant.Scheme = quant.Scheme{ItemRecipeChoices: map[string]int{"引力透镜": 1}, SchemeForRecipe: make([]quant.RecipeScheme, len(dataset.Recipes)), CostWeight: quant.CostWeight{Power: 7}}
	req.Quant.Scheme.SchemeForRecipe[index] = quant.RecipeScheme{Building: 1, ProductionMode: 2, IncreasePoints: 4}
	req.energySupply = []quant.ItemRate{{ItemID: 1006, RatePerMinute: 999}}
	base, err := quant.Solve(context.Background(), req.Quant)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(req)
	baseBefore, _ := json.Marshal(base)
	got, added, err := mandatoryInputsLowerBound(context.Background(), req, base)
	if err != nil || !added {
		t.Fatalf("必需透镜量化失败：%v", err)
	}
	after, _ := json.Marshal(req)
	baseAfter, _ := json.Marshal(base)
	if string(before) != string(after) || string(baseBefore) != string(baseAfter) || backing[2] != (quant.Target{ItemID: 1101, RatePerMinute: 777}) {
		t.Fatal("下界量化修改了原请求、底层数组或原量化结果")
	}
	if len(got.Productions) != 1 {
		t.Fatalf("显式外供原料未保留：%+v", got.Productions)
	}
	lens := got.Productions[0]
	if lens.MainItemID != 1209 || lens.OutputCount != 5 || lens.MachineID != dataset.Recipes[index].Factories[1] || lens.ProliferatorMode != 2 || lens.IncreasePoints != 4 {
		t.Fatalf("已有3透镜目标、额外2透镜及设备/增产配置未保留：%+v", lens)
	}
	for i, item := range dataset.Recipes[index].Items {
		want := dataset.Recipes[index].ItemCounts[i] * 5 / 1.25
		found := false
		for _, input := range lens.Inputs {
			if input.ItemID == item && math.Abs(input.RatePerMinute-want) < 1e-8 {
				found = true
			}
		}
		if !found || !slices.Contains(got.ExternalInputItemIDs, item) {
			t.Fatalf("4点增产或原料外供未生效：item=%d want=%g", item, want)
		}
	}
	for _, target := range got.Targets {
		if target.ItemID != 1208 && target.ItemID != 1209 {
			t.Fatalf("可选发电燃料被计入必需下界：%+v", target)
		}
	}
}
