package build

// 文件职责：回归自动运输升档、请求隔离和有界重试。
// 使用：完整生成与容量失败路径验证；边界：只读内置配方，保留真实设备与最终终检，不以错误文案代替物品等级。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// transportUpgradePlan 职责：建立高速卡晶制造或普通铁块的最小生产夹具；使用：真实容量回归；边界：原料显式外供，机器能力仍由量化及原型重建。
func transportUpgradePlan(t *testing.T, fast bool) quant.Result {
	t.Helper()
	req := quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}}
	if fast {
		dataset, err := quant.LoadDataset("")
		if err != nil {
			t.Fatal(err)
		}
		req.Targets[0].ItemID = 1126
		req.Settings.MineralizeList = map[string]bool{}
		recipe := dataset.Recipes[dataset.RecipeByID[28]]
		for _, id := range recipe.Items {
			req.Settings.MineralizeList[dataset.ByID[id].Name] = true
		}
		req.Scheme.SchemeForRecipe = make([]quant.RecipeScheme, len(dataset.Recipes))
		req.Scheme.SchemeForRecipe[dataset.RecipeByID[28]].Building = 3
	}
	plan, err := quant.Solve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// TestAutomaticTransportUpgrade 职责：证明低档设备不足时导出真实高档带臂并说明；使用：普通和密铺公开入口；边界：不改变输入请求或生产机器数。
func TestAutomaticTransportUpgrade(t *testing.T) {
	plan := transportUpgradePlan(t, true)
	for _, density := range []string{densityStandard, densityDense} {
		for _, mode := range []int{1, 2, 3, 4} {
			t.Run(fmt.Sprintf("%s/mode%d", density, mode), func(t *testing.T) {
				req := Request{Plan: plan, Mode: mode, Density: density, Setting: Setting{BeltLv: 1, InserterLv: 1}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}}
				if density == densityDense {
					req.SurfaceLayout = &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring1-center"}
				}
				before, _ := json.Marshal(req)
				result, err := Generate(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				after, _ := json.Marshal(req)
				if string(after) != string(before) {
					t.Fatal("升档修改了调用方请求")
				}
				doc, err := blueprint.Decode(result.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				belts, arms, machines := map[int16]int{}, map[int16]int{}, 0
				for _, r := range doc.Buildings {
					if surfaceIsBelt(r.ItemID) {
						belts[r.ItemID]++
					}
					if r.ItemID >= 2011 && r.ItemID <= 2014 {
						arms[r.ItemID]++
					}
					if r.RecipeID > 0 {
						machines++
					}
				}
				if belts[2001] > 0 || arms[2011] > 0 || machines != plan.MachineCount {
					t.Fatalf("未替换不足的设备或改变生产机器：%v %v %d", belts, arms, machines)
				}
				if result.LogisticsSummary.BeltLevel != 2 || len(belts) != 1 || belts[2002] == 0 {
					t.Fatalf("没有使用满足需求的最低带等级：%v", belts)
				}
				found := false
				for _, warning := range result.Warnings {
					found = found || strings.Contains(warning, "自动升档")
				}
				if !found {
					t.Fatal("缺少实际升档说明")
				}
				t.Logf("belts=%v inserters=%v buildings=%d", belts, arms, result.BuildingCount)
			})
		}
	}
}

// TestTransportUpgradeRetry 职责：覆盖候选错误链、等级上限、非容量错误和原等级足够的路径；使用：真实公开流程使用的重试器；边界：成功分支仍执行真实装配与终检。
func TestTransportUpgradeRetry(t *testing.T) {
	req := Request{Plan: transportUpgradePlan(t, false), Mode: 1, Density: densityStandard, Setting: Setting{BeltLv: 1, InserterLv: 1}}
	base, _ := resolveModeAdapter(1, densityStandard)
	t.Run("wrapped-capacity", func(t *testing.T) {
		adapter := *base
		levels := []int{}
		adapter.generate = func(ctx context.Context, b *builder, r Request, d *quant.Dataset) (Result, error) {
			levels = append(levels, b.setting.InserterLv)
			if b.setting.InserterLv < 3 {
				return Result{}, fmt.Errorf("候选：%w", errors.Join(&blueprint.Error{Code: "GEN_E_COLLISION"}, &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY"}))
			}
			return base.generate(ctx, b, r, d)
		}
		result, err := generateWithTransportUpgrades(context.Background(), req, &adapter)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(levels, []int{1, 2, 3}) || len(result.Warnings) == 0 {
			t.Fatalf("没有逐档重试：%v", levels)
		}
	})
	t.Run("maximum", func(t *testing.T) {
		adapter := *base
		seen := map[[2]int]bool{}
		adapter.generate = func(_ context.Context, b *builder, _ Request, _ *quant.Dataset) (Result, error) {
			key := [2]int{b.setting.BeltLv, b.setting.InserterLv}
			if seen[key] {
				t.Fatal("重复等级组合")
			}
			seen[key] = true
			return Result{}, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY"}
		}
		result, err := generateWithTransportUpgrades(context.Background(), req, &adapter)
		if err == nil || result.Blueprint != "" || len(seen) != 12 || !seen[[2]int{3, 4}] {
			t.Fatalf("最高档无解时未有界失败：%v %v", seen, err)
		}
	})
	t.Run("no-capacity-failure", func(t *testing.T) {
		adapter := *base
		calls := 0
		adapter.generate = func(_ context.Context, _ *builder, _ Request, _ *quant.Dataset) (Result, error) {
			calls++
			return Result{}, &blueprint.Error{Code: "GEN_E_COLLISION"}
		}
		_, err := generateWithTransportUpgrades(context.Background(), req, &adapter)
		if err == nil || calls != 1 {
			t.Fatal("非容量错误触发升档")
		}
	})
	for _, level := range []Setting{{BeltLv: 1, InserterLv: 1}, {BeltLv: 3, InserterLv: 4}} {
		t.Run(fmt.Sprintf("sufficient-%d-%d", level.BeltLv, level.InserterLv), func(t *testing.T) {
			candidate := req
			candidate.Setting = level
			result, err := Generate(context.Background(), candidate)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range result.Warnings {
				if strings.Contains(w, "自动升档") {
					t.Fatal("设备能力足够仍升档")
				}
			}
			if result.LogisticsSummary.BeltLevel != level.BeltLv {
				t.Fatal("改变了已有传送带等级")
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range doc.Buildings {
				if record.ItemID >= 2011 && record.ItemID <= 2014 && int(record.ItemID) != 2010+level.InserterLv {
					t.Fatal("改变了已有分拣器等级")
				}
			}
		})
	}
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := Generate(ctx, req)
		var failure *blueprint.Error
		if !errors.As(err, &failure) || failure.Code != "GEN_E_CANCELED" || result.Blueprint != "" {
			t.Fatalf("取消后未立即停止：%v", err)
		}
	})
}

// TestTransportUpgradeNativePreview 职责：证明原生纬带与升档重试共用唯一正式预览提交；使用：高纬入口回归；边界：不关闭球面或容量检查。
func TestTransportUpgradeNativePreview(t *testing.T) {
	plan := transportUpgradePlan(t, true)
	completed := 0
	ctx := WithProgress(context.Background(), func(p Progress) {
		if p.Kind == ProgressKindPreviewComplete {
			completed++
		}
	})
	result, err := Generate(ctx, Request{Plan: plan, Mode: 1, Density: densityDense, EmitPreviewDeltas: true, PreviewProtocolVersion: 4, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring4-north-center"}})
	if err != nil {
		t.Fatal(err)
	}
	if completed != 1 || result.WinnerCandidateID == "" || result.LogisticsSummary.BeltLevel < 2 {
		t.Fatalf("原生升档结果或预览提交无效：completed=%d", completed)
	}
}
