package industryplan

// 文件职责：验证已知供电空间容量驱动的制造后缀迁移；使用：电厂落位失败回归；边界：分工测试不替代实体蓝图终检。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// TestPowerCapacityMovesSufficientSuffix 职责：保留最大可供电前缀并一次移动足够负荷；使用：已有星优先及策略隔离；边界：基础设施需求不能随制造机器一并扣除。
func TestPowerCapacityMovesSufficientSuffix(t *testing.T) {
	site := spillTestSite(101, "a", "b", "c", "d")
	site.Power = &PowerPlan{RequiredMW: 95}
	for i, load := range []float64{30, 20, 15, 10} {
		site.Plan.Productions[i].PowerMW = 1
		site.Plan.Productions[i].Capacity = &quant.ProductionCapacity{PowerMW: load}
	}
	excluded := resourceSite{planetID: 101, itemID: 1001}
	limited := resourceSite{planetID: 103, itemID: 1006}
	policy := assignment{center: 101, strategy: 2,
		moves: map[string]int64{"a": 101, "x": 103}, changed: map[string]bool{"b": true, "c": true},
		excludedSources: map[resourceSite]bool{excluded: true}, sourceLimits: map[resourceSite]float64{limited: 60},
		fullFactories: map[int64]bool{104: true}, powerMultipliers: map[int64]float64{101: 1.2}}
	previous := Result{Planets: []PlanetPlan{site, spillTestSite(103, "x"), spillTestSite(104, "y")}}
	planets := map[int64]Planet{101: {ID: 101}, 102: {ID: 102, Position: [3]float64{10}},
		103: {ID: 103, Position: [3]float64{100}}, 104: {ID: 104, Position: [3]float64{50}}}
	available := 50.0
	rootCause := errors.New("剩余空地无法继续放置电厂")
	cause := fmt.Errorf("逐星复核：%w", &PowerLoadError{RequiredMW: 90, AvailableMW: &available, Err: rootCause})
	got, ok := repairAssignment(policy, previous, 101, cause, planets, []int64{101, 102, 103, 104})
	want := map[string]int64{"a": 101, "b": 103, "c": 103, "d": 103, "x": 103, "y": 104}
	if !ok || !reflect.DeepEqual(got.moves, want) || !got.fullFactories[101] || !got.fullFactories[104] || !errors.Is(cause, rootCause) {
		t.Fatalf("未按容量迁移完整后缀或未复用既有星：%+v，%t", got, ok)
	}
	if got.center != policy.center || got.strategy != policy.strategy || !reflect.DeepEqual(got.changed, policy.changed) ||
		!reflect.DeepEqual(got.excludedSources, policy.excludedSources) || !reflect.DeepEqual(got.sourceLimits, policy.sourceLimits) ||
		!reflect.DeepEqual(got.powerMultipliers, policy.powerMultipliers) || len(policy.moves) != 2 || len(policy.fullFactories) != 1 {
		t.Fatal("容量反馈污染原策略或改变其他资源、能源约束")
	}
	// 已保留的 30 MW 制造加 20 MW 固定负荷刚好达到容量，不应重复迁移。
	site.Plan.Productions, site.BlockIDs = site.Plan.Productions[:1], site.BlockIDs[:1]
	site.Power = &PowerPlan{RequiredMW: 50}
	previous.Planets[0] = site
	if _, ok := repairAssignment(got, previous, 101, &PowerLoadError{RequiredMW: 50, AvailableMW: &available, Err: rootCause}, planets, []int64{101, 102, 103, 104}); ok {
		t.Fatal("已满足容量仍继续迁移")
	}
}

// TestPowerCapacityRejectsUnrepairableDemand 职责：保留固定需求过高及无可迁移负荷的真实失败；使用：反馈边界；边界：不能降级成普通单模块迁移或补更多电厂。
func TestPowerCapacityRejectsUnrepairableDemand(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		required, available, load    float64
		missingBlocks, noDestination bool
	}{
		{name: "fixed-demand", required: 80, available: 19, load: 60},
		{name: "no-movable-load", required: 80, available: 40},
		{name: "already-enough", required: 80, available: 80, load: 60},
		{name: "negative-capacity", required: 80, available: -1, load: 60},
		{name: "nan-capacity", required: 80, available: math.NaN(), load: 60},
		{name: "infinite-demand", required: math.Inf(1), available: 40, load: 60},
		{name: "unknown-identity", required: 80, available: 40, load: 60, missingBlocks: true},
		{name: "no-destination", required: 80, available: 40, load: 60, noDestination: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			site := spillTestSite(101, "a")
			site.Power = &PowerPlan{RequiredMW: tc.required}
			site.Plan.Productions[0].PowerMW = tc.load
			if tc.missingBlocks {
				site.BlockIDs = nil
			}
			policy := assignment{center: 101, moves: map[string]int64{"a": 101}, fullFactories: map[int64]bool{102: tc.noDestination}}
			planets := map[int64]Planet{101: {ID: 101}, 102: {ID: 102}}
			feedback := &PowerLoadError{RequiredMW: tc.required, AvailableMW: &tc.available, Err: errors.New("供电空间失败")}
			for repeat := 0; repeat < 2; repeat++ {
				got, ok := repairAssignment(policy, Result{Planets: []PlanetPlan{site}}, 101, feedback, planets, []int64{101, 102})
				if ok || !reflect.DeepEqual(got, policy) {
					t.Fatalf("不可修复反馈仍改变分工：%+v，%t", got, ok)
				}
			}
		})
	}
}

// TestPowerCapacityReal1203Suffix 职责：复现真实 64 模块合同在 38 台恒星可用时的首轮迁移；使用：failure-031 的额定机器负荷与需求快照；边界：只计算迁移规模，不声称剩余布局已经成功。
func TestPowerCapacityReal1203Suffix(t *testing.T) {
	site := PlanetPlan{PlanetID: 1203, Power: &PowerPlan{RequiredMW: 3692.4779737825984}}
	for _, group := range []struct {
		blocks, machines int
		perMachine       float64
	}{
		{4, 32, .9}, {29, 32, 2.4}, {1, 14, 2.4}, {24, 32, .9}, {1, 16, .9}, {4, 32, 1.2}, {1, 32, 1.8},
	} {
		for range group.blocks {
			site.BlockIDs = append(site.BlockIDs, fmt.Sprintf("module-%d", len(site.BlockIDs)))
			site.Plan.Productions = append(site.Plan.Productions, quant.Production{MachineCount: group.machines,
				Capacity: &quant.ProductionCapacity{PowerMW: float64(group.machines) * group.perMachine}})
		}
	}
	available := 38.0 * 72
	first, ok := powerCapacityPrefix(site, &PowerLoadError{RequiredMW: site.Power.RequiredMW, AvailableMW: &available})
	machines, removed := 0, 0.0
	for _, production := range site.Plan.Productions[first:] {
		machines += production.MachineCount
		removed += production.Capacity.PowerMW
	}
	remaining := site.Power.RequiredMW - removed
	if !ok || len(site.Plan.Productions) != 64 || first != 32 || machines != 990 || math.Abs(removed-1027.2) > 1e-8 || math.Abs(remaining-2665.2779737825984) > 1e-8 {
		t.Fatalf("实样本迁移规模错误：首块 %d，移动 %d 台/%.9f MW，剩余 %.9f MW，%t", first, machines, removed, remaining, ok)
	}
	if remaining+site.Plan.Productions[first].Capacity.PowerMW <= available+1e-6 {
		t.Fatal("迁移了本可保留的额外模块")
	}
	t.Logf("保留 %d 块；迁移 %d 块/%d 台/%.3f MW；保留需求 %.3f MW，可用 %.3f MW", first, 64-first, machines, removed, remaining, available)
}

// TestPowerCapacityReplansFuelAndRoutes 职责：从容量反馈进入完整分工、燃料量化和端口复核；使用：多模块迁移闭环；边界：验证器模拟空间容量，实体容量另有蓝图测试。
func TestPowerCapacityReplansFuelAndRoutes(t *testing.T) {
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 1000}, {ItemID: 1006, Kind: "solid", Count: 1000}}},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{1000}},
	}}
	req := Request{CenterPlanetID: 101, EnablePower: true, EnableFuel: true,
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60 * 32 * 6}}},
		// 每船 1200 件时首轮搬迁即可在 50 MW 内完成剩余制造、采矿及派船。
		Logistics: LogisticsSettings{CargoPerVessel: 1200},
		Energy:    EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2204, FuelItemID: 1109}}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	available := 50.0
	var first *PlanetPlan
	validate := func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 && site.Power != nil && site.Power.RequiredMW > available+1e-6 {
			if first == nil {
				copy := site
				first = &copy
			}
			return PlanetActual{}, &PowerLoadError{RequiredMW: site.Power.RequiredMW, AvailableMW: &available, Err: errors.New("已放机组只能供应 50 MW")}
		}
		return PlanetActual{Ports: site.Ports, Power: site.Power}, nil
	}
	result, err := Plan(ctx, req, world, validate, nil)
	if err != nil || !result.Verified || first == nil || result.Repairs != 1 {
		t.Fatalf("容量反馈未一次完成重算复核：错误%v，已验证%t，修复%d，状态%s，说明%v", err, result.Verified, result.Repairs, result.Status, result.Warnings)
	}
	blocks := map[string]int64{}
	var center, remote *PlanetPlan
	for i := range result.Planets {
		site := &result.Planets[i]
		if site.PlanetID == 101 {
			center = site
		} else if site.PlanetID == 102 {
			remote = site
		}
		for _, block := range site.BlockIDs {
			blocks[block] = site.PlanetID
		}
		if site.Power != nil && site.Power.PowerMW+1e-6 < site.Power.RequiredMW {
			t.Fatalf("星球%d未补足供电", site.PlanetID)
		}
	}
	if center == nil || remote == nil || center.Power == nil || remote.Power == nil || center.Power.RequiredMW > available+1e-6 || center.Power.Count >= first.Power.Count || center.Power.FuelRatePerMinute >= first.Power.FuelRatePerMinute || remote.Power.FuelRatePerMinute <= 0 {
		t.Fatalf("迁移没有重算双方供电和燃料：初始%+v，中心%+v，远端%+v", first.Power, center, remote)
	}
	firstMoved := len(first.BlockIDs)
	for i, block := range first.BlockIDs {
		if blocks[block] != 101 {
			firstMoved = i
			break
		}
	}
	if len(first.BlockIDs)-firstMoved < 2 {
		t.Fatalf("仍只迁移一块：%v", blocks)
	}
	for i, block := range first.BlockIDs {
		want := int64(101)
		if i >= firstMoved {
			want = 102
		}
		if blocks[block] != want {
			t.Fatalf("制造前缀丢失或后缀不连续：%s 到%d，应到%d", block, blocks[block], want)
		}
	}
	delivery := 0.0
	for _, route := range result.Routes {
		if route.ItemID == 1101 && route.FromPlanetID == 102 && route.ToPlanetID == 101 {
			delivery += route.RatePerMinute
		}
	}
	if delivery <= 0 || hasExternalKind(result, "fuel-input") || hasExternalKind(result, "grid-power") {
		t.Fatalf("迁移后物流或燃料闭合失效：路线%+v，外供%+v", result.Routes, result.External)
	}
	for _, tc := range []struct {
		cargo    float64
		verified bool
	}{{200, false}, {600, true}} {
		req.Logistics.CargoPerVessel = tc.cargo
		replanned, err := Plan(ctx, req, world, validate, nil)
		if err != nil || replanned.Verified != tc.verified {
			t.Fatalf("单船 %.0f 件的 50 MW 容量复核：状态 %s，已核验 %t，错误 %v", tc.cargo, replanned.Status, replanned.Verified, err)
		}
		for _, p := range replanned.Planets {
			if replanned.Verified && p.PlanetID == 101 && (p.Power == nil || p.Power.RequiredMW > available+1e-6 || p.Power.RequiredMW < p.OutboundVesselMW) {
				t.Fatalf("中心采用更小船舱后仍须保证实际派船用电与 50 MW 上限：%+v", p.Power)
			}
		}
	}
}
