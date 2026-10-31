package build

// 文件职责：防止统一容量与最终规则沙箱再次从生成主链脱落。
// 使用：归一化、真实拆行和 finish 集成回归；边界：不把编码成功当作游戏放置或运行验收。

import (
	"context"
	"errors"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestNormalizeSettingUsesSelectedBeltCapacity 职责：确认旧 BeltSpeed 不能覆盖等级；使用：旧工作区兼容回归；边界：缺省等级使用 Mk.I，不推测四层堆叠。
func TestNormalizeSettingUsesSelectedBeltCapacity(t *testing.T) {
	for level, capacity := range []float64{360, 360, 720, 1800} {
		setting, err := normalizeSetting(Setting{BeltLv: level, BeltSpeed: 7200})
		if err != nil {
			t.Fatal(err)
		}
		if setting.BeltSpeed != capacity || setting.BeltLv != max(1, level) {
			t.Fatalf("level %d: normalized capacity %.0f, level %d", level, setting.BeltSpeed, setting.BeltLv)
		}
	}
}

// TestLineCountsCreateRealParallelRows 职责：把4200/min拆成真实机器行而非诊断装饰带；使用：三个等级的自动入口下界；边界：所有70台机器保留，每行共享输出和输入都不超额。
func TestLineCountsCreateRealParallelRows(t *testing.T) {
	production := quant.Production{MachineType: 1, MachineCount: 70, ExactMachineCount: 70,
		Inputs: []quant.ItemRate{{ItemID: 1001, RatePerMinute: 4200}}, Outputs: []quant.ItemRate{{ItemID: 1101, RatePerMinute: 4200}}}
	recipe := quant.Recipe{ID: 1, TimeSpend: 60, Items: []int{1001}, ItemCounts: []float64{1}, Results: []int{1101}, ResultCounts: []float64{1}}
	for level, want := range map[int]int{1: 12, 2: 6, 3: 3} {
		setting, err := normalizeSetting(Setting{BeltLv: level, BeltSpeed: 7200, MaxMachineInALine: 100, SmelterMaxCount: 100})
		if err != nil {
			t.Fatal(err)
		}
		counts, err := lineCounts(production, recipe, setting, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(counts) != want {
			t.Fatalf("Mk.%d rows=%d, want %d", level, len(counts), want)
		}
		total := 0
		for _, count := range counts {
			total += count
			if count <= 0 || float64(count)*60 > setting.BeltSpeed+1e-6 {
				t.Fatalf("Mk.%d row count %d exceeds actual single-belt capacity %.0f", level, count, setting.BeltSpeed)
			}
		}
		if total != production.MachineCount {
			t.Fatalf("Mk.%d changed machines: %d", level, total)
		}
	}
}

// TestFinishRejectsPlanarCollisionBeforeEncoding 职责：确认平面请求同样经过最终静态碰撞规则；使用：主链耦合回归；边界：错误不发布部分蓝图。
func TestFinishRejectsPlanarCollisionBeforeEncoding(t *testing.T) {
	setting, err := normalizeSetting(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	noPower := false
	setting.NeedPower = &noPower
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	graph := quant.FlowGraph{}
	b.flowGraph, b.logistics = &graph, newLogisticsSummary(graph, setting)
	b.make(2302, 0, 0, 0)
	b.make(2302, 0, 0, 0)
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.finish(context.Background(), Request{Mode: 1, Setting: setting}, dataset)
	var failure *blueprint.Error
	if !errors.As(err, &failure) || failure.Code != "GEN_E_COLLISION" || out.Blueprint != "" {
		t.Fatalf("finish must reject final collision without publishing blueprint: %+v, %v", out, err)
	}
}

// TestFinishIncludesExecutedRuleAudit 职责：要求成功出口包含实际执行的规则证据；使用：防止nil摘要/审计回归；边界：切平面结果仍披露部分规则和运行未验收。
func TestFinishIncludesExecutedRuleAudit(t *testing.T) {
	setting, err := normalizeSetting(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	graph := quant.FlowGraph{}
	b.flowGraph, b.logistics = &graph, newLogisticsSummary(graph, setting)
	b.byPath([]vector{{0, 0, 0}, {1, 0, 0}})
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.finish(context.Background(), Request{Mode: 1, Setting: setting}, dataset)
	if err != nil {
		t.Fatal(err)
	}
	if out.RuleAudit == nil || out.RuleAudit.CoordinateSpace != "equatorial-tangent" || out.RuleAudit.Coverage != "partial-numeric" || len(out.RuleAudit.Checks) == 0 || len(out.RuleAudit.Unverified) == 0 || out.LogisticsSummary == nil {
		t.Fatalf("success is missing executed checks or coverage boundaries: %+v", out.RuleAudit)
	}
}
