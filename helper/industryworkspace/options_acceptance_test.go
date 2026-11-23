package industryworkspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// TestSavedLockedFuelRecipe 职责：复现未解锁燃料棒仍可导出的问题；使用：显式提供原版 16.dsv；边界：相同外供条件下单独检查存档与模拟入口。
func TestSavedLockedFuelRecipe(t *testing.T) {
	path := os.Getenv("DSP_RECIPE_SAVE")
	if path == "" {
		t.Skip("未指定配方 156 尚未解锁的 16.dsv")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RecipeUnlocked(156) {
		t.Fatal("样本已解锁奇异湮灭燃料棒")
	}
	selection, world := Describe("locked-recipe", snapshot)
	off := false
	q := quant.Request{Targets: []quant.Target{{ItemID: 1804, RatePerMinute: 1}}}
	data, err := quant.LoadVanillaDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	q.Settings.MineralizeList = map[string]bool{}
	for _, id := range data.Recipes[data.RecipeByID[156]].Items {
		q.Settings.MineralizeList[data.ByID[id].Name] = true
	}
	for _, mode := range []string{"save", "custom"} {
		r := Request{SaveID: selection.SaveID, SnapshotID: selection.SnapshotID, Options: Options{TechnologyMode: mode, BeltLevel: 3, InserterLevel: 3},
			Planning: industryplan.Request{CenterPlanetID: 204, EnableMining: &off, EnableLogistics: &off, Profile: "low", Quant: q}}
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		bundle, err := Generate(t.Context(), r, selection, world, snapshot, nil)
		if mode == "custom" {
			if err != nil || !bundle.Report.Exportable || len(bundle.Codes) == 0 {
				t.Fatalf("手动模拟未能导出：%v", err)
			}
			continue
		}
		if err == nil || bundle != nil {
			t.Fatal("未解锁的制造配方进入了施工方案")
		}
		issue := PlayerIssue(err.Error())
		if !strings.Contains(issue.Cause, "尚未解锁「奇异湮灭燃料棒」") || strings.Contains(issue.Cause, "Q_E_") || issue.Action == "" {
			t.Fatalf("缺少玩家可见的具体配方原因：%+v", issue)
		}
	}
}

// TestCachedSavedResources 职责：核对手动查看的精算结果能被已创建的规划世界读取；使用：真实验收档；边界：只精算到首个有确定结果的候选星球。
func TestCachedSavedResources(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定真实快照")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	view, world := Describe("first", snapshot)
	for _, site := range view.Sites {
		if site.Source != "candidate" {
			continue
		}
		if _, _, found := world.CachedResources(site.ID); found {
			t.Fatalf("选档时已精算候选星球 %d", site.ID)
		}
		summary, err := snapshot.ResolvePlanetResources(context.Background(), site.ID)
		if err != nil {
			t.Fatal(err)
		}
		resources, known, found := world.CachedResources(site.ID)
		if !found || known != (summary.Source == "saved" || summary.Source == "generated") || !reflect.DeepEqual(resources, summaryCapacities(summary)) {
			t.Fatalf("已创建的世界未复用星球 %d 的手动精算：来源=%s，命中=%t，已知=%t", site.ID, summary.Source, found, known)
		}
		if known {
			t.Logf("复用候选星球 %d 的确定资源（%s，%d 种）", site.ID, summary.Source, len(resources))
			return
		}
	}
	t.Fatal("验收档没有可精算出确定资源的候选星球")
}

// TestIndustrySavedOptions 职责：用真实快照核对可选设施和土方；使用：显式指定验收档；边界：只读样本，不启动游戏。
func TestIndustrySavedOptions(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定真实快照")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	view, world := Describe("options", snapshot)
	mining := false
	base := func(item int) Request {
		return Request{SaveID: view.SaveID, SnapshotID: view.SnapshotID, Planning: industryplan.Request{
			Quant:          quant.Request{Targets: []quant.Target{{ItemID: item, RatePerMinute: 30}}},
			CenterPlanetID: 301, Profile: "standard", EnableMining: &mining,
		}}
	}
	generate := func(t *testing.T, r Request) (*Bundle, blueprint.Document) {
		t.Helper()
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		b, err := Generate(ctx, r, view, world, snapshot, nil)
		if err != nil {
			t.Fatal(err)
		}
		center := r.Planning.CenterPlanetID
		if !b.Report.Exportable || b.Codes[center] == "" {
			t.Fatalf("不能导出：%s，%v", b.Report.Status, b.Report.Warnings)
		}
		doc, err := blueprint.Decode(b.Codes[center], true)
		if err != nil {
			t.Fatal(err)
		}
		if out := os.Getenv("DSP_OPTION_EVIDENCE"); out != "" {
			stem := strings.ReplaceAll(t.Name(), "/", "-")
			for id, code := range b.Codes {
				name := stem
				if id != center {
					name += fmt.Sprintf("-planet-%d", id)
				}
				if err := os.WriteFile(filepath.Join(out, name+".txt"), []byte(code), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		t.Logf("tick=%d 目标=%v 蓝图实体=%d Area=%d 可选配置=%+v", view.Tick, r.Planning.Quant.Targets, len(doc.Buildings), len(doc.Areas), r.Options)
		return b, doc
	}
	t.Run("defaults", func(t *testing.T) {
		b, doc := generate(t, base(1101))
		if doc.Reform != nil || containsFacility(doc, 2204, 2313, 2101, 2106) || !containsExternal(b, "grid-power") {
			t.Fatalf("可选设施未默认关闭：外供 %+v", b.Report.External)
		}
	})
	t.Run("thermal-external-fuel", func(t *testing.T) {
		r := base(1101)
		r.Options.Power = true
		b, doc := generate(t, r)
		if !containsFacility(doc, 2204) || !containsExternal(b, "fuel-input") || b.Report.Planets[0].Power == nil {
			t.Fatalf("火电或外供煤合同缺失：%+v", b.Report.External)
		}
		for _, machine := range doc.Buildings {
			if machine.ItemID != 2204 {
				continue
			}
			for _, arm := range doc.Buildings {
				if arm.OutputObjIdx == machine.Index && arm.ItemID >= 2011 && arm.ItemID <= 2014 {
					t.Logf("火电=%d 进煤分拣器=%d", machine.Index, arm.Index)
					return
				}
			}
		}
		t.Fatal("火电无实际进煤分拣器")
	})
	t.Run("fuel-without-power", func(t *testing.T) {
		r := base(1101)
		r.Options.Fuel = true
		b, doc := generate(t, r)
		if containsFacility(doc, 2204) || containsExternal(b, "fuel-input") || !containsExternal(b, "grid-power") {
			t.Fatalf("只开燃料时附带了发电负荷：%+v", b.Report.External)
		}
	})
	t.Run("thermal-planned-fuel", func(t *testing.T) {
		r := base(1101)
		r.Options.Power, r.Options.Fuel = true, true
		mine := true
		r.Planning.EnableMining = &mine
		d, err := quant.LoadDataset("Vanilla")
		if err != nil {
			t.Fatal(err)
		}
		r.Planning.Quant.Settings.MineralizeList = map[string]bool{d.ByID[1001].Name: true}
		var coalSite int64
		for _, id := range []int64{301, 302, 401, 101} {
			resources, err := snapshot.ResolvePlanetResources(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			for _, res := range capacities(resources.Resources) {
				if res.ItemID == 1006 && res.Count >= 4 {
					coalSite = id
					break
				}
			}
			if coalSite != 0 {
				break
			}
		}
		if coalSite == 0 {
			t.Fatal("实档周边测试星未找到四个可用煤矿点")
		}
		r.Planning.CenterPlanetID = coalSite
		b, doc := generate(t, r)
		if !containsFacility(doc, 2204) || containsExternal(b, "fuel-input") {
			t.Fatalf("本地采煤未接入火电：%v", b.Report.External)
		}
		for _, sheet := range b.Report.Planets {
			for _, mine := range sheet.MinePlacements {
				if mine.ItemID == 1006 && len(mine.VeinIDs) > 0 && mine.RatePerMinute > 0 {
					if sheet.ID != coalSite {
						routeFound := false
						for _, route := range b.Report.Routes {
							routeFound = routeFound || route.ItemID == 1006 && route.FromPlanetID == sheet.ID && route.ToPlanetID == coalSite && route.RatePerMinute > 0
						}
						if !routeFound {
							continue
						}
					}
					t.Logf("煤源星球=%d 发电星球=%d 煤矿=%d 实际矿点=%v 流量=%.3f/min", sheet.ID, coalSite, mine.BuildingIndex, mine.VeinIDs, mine.RatePerMinute)
					return
				}
			}
		}
		t.Fatal("开启燃料配套却没有真实煤矿机")
	})
	t.Run("spray", func(t *testing.T) {
		r := base(1101)
		d, err := quant.LoadDataset("Vanilla")
		if err != nil {
			t.Fatal(err)
		}
		choice := d.Choices[d.ByID[1101].Name][0]
		r.Planning.Quant.Scheme.SchemeForRecipe = make([]quant.RecipeScheme, len(d.Recipes))
		r.Planning.Quant.Scheme.SchemeForRecipe[choice] = quant.RecipeScheme{IncreasePoints: 2, ProductionMode: 2}
		if err := r.Validate(); err == nil {
			t.Fatal("关闭喷涂却接受增产配方")
		}
		r.Options.Spray = true
		b, doc := generate(t, r)
		if !containsFacility(doc, 2313) {
			t.Fatalf("增产模式缺少实体喷涂机，外供 %+v", b.Report.External)
		}
		for _, line := range b.Plan.Plan.Productions {
			if line.MainItemID == 1101 && line.IncreasePoints == 2 && line.ProliferatorMode == 2 {
				return
			}
		}
		t.Fatal("新增喷涂机未关联指定生产档")
	})
	for _, example := range []struct {
		name     string
		target   int
		facility int16
	}{
		{"solid-buffer", 1101, 2101}, {"liquid-buffer", 1114, 2106},
	} {
		t.Run(example.name, func(t *testing.T) {
			r := base(example.target)
			r.Options.Buffer = true
			_, doc := generate(t, r)
			for _, facility := range doc.Buildings {
				if facility.ItemID == example.facility {
					if facility.ItemID == 2101 && (len(facility.Parameters) != 110 || facility.Parameters[0] != 0) ||
						facility.ItemID == 2106 && (len(facility.Parameters) != 2 || facility.Parameters[0] != 1 || facility.Parameters[1] != 1) {
						t.Fatal("缓存设备无原版设置")
					}
					t.Logf("缓存原型=%d 参数数=%d", facility.ItemID, len(facility.Parameters))
					return
				}
			}
			t.Fatalf("缺少缓存设备 %d", example.facility)
		})
	}
	lastCells := 0
	for _, option := range []string{"construction", "planet"} {
		t.Run("foundation-"+option, func(t *testing.T) {
			r := base(1101)
			r.Options.Foundation = option
			b, doc := generate(t, r)
			if doc.Reform == nil || len(doc.Reform.Rects) == 0 {
				t.Fatal("原版蓝图无地基矩形")
			}
			sheet := b.Report.Planets[0]
			count := 0
			for _, rect := range doc.Reform.Rects {
				count += int(rect.W) * int(rect.H)
			}
			t.Logf("实测地基格=%d 矩形=%d 沙土依据=%t 区间=%d~%d", count, len(doc.Reform.Rects), sheet.SoilEstimateKnown, sheet.SoilLowerBound, sheet.SoilUpperBound)
			if count != sheet.FoundationCells || count <= lastCells || !sheet.SoilEstimateKnown || sheet.SoilUpperBound < sheet.SoilLowerBound {
				t.Fatal("地基用量与蓝图或覆盖范围不一致")
			}
			lastCells = count
		})
	}
}

// containsFacility 职责：查找严格解码后的实体；使用：设施开关闭环；边界：匹配原版原型编号。
func containsFacility(doc blueprint.Document, ids ...int16) bool {
	for _, b := range doc.Buildings {
		for _, id := range ids {
			if b.ItemID == id {
				return true
			}
		}
	}
	return false
}

// containsExternal 职责：核对规划保留的外供条件；使用：发电与燃料开关；边界：仅匹配实际正需求。
func containsExternal(b *Bundle, kind string) bool {
	for _, e := range b.Report.External {
		if e.Kind == kind && (e.PowerMW > 0 || e.RatePerMinute > 0) {
			return true
		}
	}
	return false
}
