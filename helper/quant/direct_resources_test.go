package quant

// 文件职责：核对默认制造、矿源直采与玩家外供的量化差别。以原版配方为基准，采集落位交由产业规划复核。

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestDirectResourceSilicon 用人造恒星固定需求核对石矿、硅石和外供来源分类。
func TestDirectResourceSilicon(t *testing.T) {
	req := Request{Targets: []Target{{ItemID: 2210, RatePerMinute: 200}}}
	materialRate := func(result Result, id int) float64 {
		for _, row := range result.RawInputs {
			if row.ItemID == id {
				return row.RatePerMinute
			}
		}
		return 0
	}
	base, err := Solve(context.Background(), req)
	if err != nil || materialRate(base, 1005) != 644000 || materialRate(base, 1003) != 0 {
		t.Fatalf("默认配方石矿需求错误：%v %v", base.RawInputs, err)
	}
	req.Settings.DirectResourceItemIDs = []int{1003}
	direct, err := Solve(context.Background(), req)
	if err != nil || materialRate(direct, 1005) != 84000 || materialRate(direct, 1003) != 56000 || slices.Contains(direct.ExternalInputItemIDs, 1003) {
		t.Fatalf("直接采集没有作为硅石原料需求：%v %v %v", direct.RawInputs, direct.ExternalInputItemIDs, err)
	}
	req.Settings.MineralizeList = map[string]bool{"硅石": true}
	if _, err = Solve(context.Background(), req); err == nil || !strings.Contains(err.Error(), "外供") {
		t.Fatalf("直接采集与玩家外供冲突未拒绝：%v", err)
	}
	req.Settings.DirectResourceItemIDs = nil
	external, err := Solve(context.Background(), req)
	if err != nil || materialRate(external, 1003) != 56000 || !slices.Contains(external.ExternalInputItemIDs, 1003) {
		t.Fatalf("玩家外供被错误合并为采集：%v %v %v", external.RawInputs, external.ExternalInputItemIDs, err)
	}
	req.Settings.MineralizeList = nil
	req.Settings.DirectResourceItemIDs = []int{1003}
	req.Scheme.ItemRecipeChoices = map[string]int{"硅石": 1}
	if _, err = Solve(context.Background(), req); err == nil || !strings.Contains(err.Error(), "制造配方") {
		t.Fatalf("直接采集与人工配方冲突未拒绝：%v", err)
	}
	req.Settings.DirectResourceItemIDs = []int{2210}
	if _, err = Solve(context.Background(), req); err == nil || !strings.Contains(err.Error(), "没有矿机或抽水站采集来源") {
		t.Fatal("正常制造设备不能作为可采集矿物", err)
	}
}
