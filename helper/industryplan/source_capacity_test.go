package industryplan

// 文件职责：验证矿源部分成功后的容量回传；使用：跨星采矿修复回归；边界：合成回执只证明规划守恒，实体矿机能力由生成器验证。

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

// TestSourceCapacityRetainsPartialMining 职责：保留已接通矿量并由另一星补足；使用：完整规划回归；边界：另一种原料及制造分工不随铁矿限额迁走。
func TestSourceCapacityRetainsPartialMining(t *testing.T) {
	req := Request{CenterPlanetID: 101, Quant: quant.Request{Targets: []quant.Target{
		{ItemID: 1101, RatePerMinute: 150}, {ItemID: 1104, RatePerMinute: 30},
	}}}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
			{ItemID: 1001, Kind: "solid", CapacityPerMinute: 100, UnitRatePerMinute: 30},
			{ItemID: 1002, Kind: "solid", CapacityPerMinute: 100, UnitRatePerMinute: 30},
		}},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{1000, 0, 0}, Resources: []Resource{
			{ItemID: 1001, Kind: "solid", CapacityPerMinute: 100, UnitRatePerMinute: 30},
		}},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var first PlanetPlan
	failures := 0
	result, err := Plan(ctx, req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 && sourceCapacityRate(site, 1001) > 60 {
			first = site
			failures++
			return PlanetActual{}, &SourcePlacementError{ItemID: 1001, AvailableRatePerMinute: 60, Err: errors.New("后续矿点没有可接通的落位")}
		}
		return PlanetActual{Ports: site.Ports, Power: site.Power}, nil
	}, nil)
	if err != nil || !result.Verified || result.Repairs != 1 || failures != 1 {
		t.Fatalf("部分采矿回执未一次修复完成：错误 %v，验证 %t，修复 %d，失败 %d，状态 %s，说明 %v", err, result.Verified, result.Repairs, failures, result.Status, result.Warnings)
	}
	if sourceCapacityRate(first, 1001) != 100 || sourceCapacityRate(first, 1002) != 30 {
		t.Fatalf("首轮未按本星实际能力分配：%+v", first.Mines)
	}
	if len(result.Planets) != 2 {
		t.Fatalf("应仅使用两颗星：%+v", result.Planets)
	}
	for _, site := range result.Planets {
		switch site.PlanetID {
		case 101:
			if sourceCapacityRate(site, 1001) != 60 || sourceCapacityRate(site, 1002) != 30 || !reflect.DeepEqual(site.BlockIDs, first.BlockIDs) || !reflect.DeepEqual(site.Plan.Productions, first.Plan.Productions) {
				t.Fatalf("已成功矿量丢失或无关制造、铜矿改变：矿业 %+v，制造 %+v", site.Mines, site.Plan.Productions)
			}
		case 102:
			if sourceCapacityRate(site, 1001) != 90 || len(site.Plan.Productions) != 0 {
				t.Fatalf("另一星未仅补足剩余铁矿：%+v", site)
			}
		default:
			t.Fatalf("出现未声明星球 %d", site.PlanetID)
		}
	}
	if len(result.Routes) != 1 || result.Routes[0].FromPlanetID != 102 || result.Routes[0].ToPlanetID != 101 || result.Routes[0].ItemID != 1001 || result.Routes[0].RatePerMinute != 90 {
		t.Fatalf("修复后的跨星流量未守恒：%+v", result.Routes)
	}
	for _, external := range result.External {
		if external.ItemID == 1001 || external.ItemID == 1002 {
			t.Fatalf("真实矿量被外供替代：%+v", external)
		}
	}
	if result.Plan.MachineCount != first.Plan.MachineCount {
		t.Fatalf("制造数量改变：机器 %d/%d", result.Plan.MachineCount, first.Plan.MachineCount)
	}
}

// TestSourceCapacityRepairIsolation 职责：逐步降低指定矿源容量且保留其他决策；使用：容量映射回归；边界：新映射不得反写前一候选。
func TestSourceCapacityRepairIsolation(t *testing.T) {
	key := resourceSite{planetID: 101, itemID: 1001}
	otherItem := resourceSite{planetID: 101, itemID: 1002}
	otherPlanet := resourceSite{planetID: 102, itemID: 1001}
	policy := assignment{center: 101, strategy: 2,
		moves: map[string]int64{"iron": 101, "copper": 102}, changed: map[string]bool{"copper": true},
		excludedSources: map[resourceSite]bool{{planetID: 103, itemID: 1001}: true},
		sourceLimits:    map[resourceSite]float64{otherItem: 40, otherPlanet: 80},
		fullFactories:   map[int64]bool{103: true}, powerMultipliers: map[int64]float64{101: 1.2},
	}
	first, ok := repairAssignment(policy, Result{}, 101, &SourcePlacementError{ItemID: 1001, AvailableRatePerMinute: 60, Err: errors.New("局部矿区不可用")}, nil, nil)
	if !ok || first.sourceLimits[key] != 60 || first.excludedSources[key] || len(policy.sourceLimits) != 2 {
		t.Fatalf("未保留局部能力或反写旧候选：新 %+v，旧 %+v，可修复 %t", first, policy, ok)
	}
	second, ok := repairAssignment(first, Result{}, 101, &SourcePlacementError{ItemID: 1001, AvailableRatePerMinute: 50, Err: errors.New("只接通更少矿点")}, nil, nil)
	if !ok || second.sourceLimits[key] != 50 || first.sourceLimits[key] != 60 || second.excludedSources[key] {
		t.Fatalf("进一步降低上限时丢失矿源或污染前一候选：%+v，%t", second, ok)
	}
	if second.sourceLimits[otherItem] != 40 || second.sourceLimits[otherPlanet] != 80 || second.center != policy.center || second.strategy != policy.strategy ||
		!reflect.DeepEqual(second.moves, policy.moves) || !reflect.DeepEqual(second.changed, policy.changed) ||
		!reflect.DeepEqual(second.excludedSources, policy.excludedSources) || !reflect.DeepEqual(second.fullFactories, policy.fullFactories) || !reflect.DeepEqual(second.powerMultipliers, policy.powerMultipliers) {
		t.Fatalf("矿源限额改变无关原料、星球或分工：%+v", second)
	}
}

// TestSourceCapacityInvalidOrRepeatedFeedbackStops 职责：让不能降低容量的失败反馈有限终止；使用：零值、非法值及重复回执回归；边界：排除范围仅为本星本物料。
func TestSourceCapacityInvalidOrRepeatedFeedbackStops(t *testing.T) {
	for _, tc := range []struct {
		name      string
		available float64
		limited   bool
	}{
		{"没有接通", 0, false}, {"负数", -1, false}, {"NaN", math.NaN(), false}, {"无穷", math.Inf(1), false},
		{"重复上限", 60, true}, {"上限反增", 70, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := resourceSite{planetID: 101, itemID: 1001}
			other := resourceSite{planetID: 101, itemID: 1002}
			policy := assignment{center: 101, moves: map[string]int64{"iron": 101}, sourceLimits: map[resourceSite]float64{other: 40}}
			if tc.limited {
				policy.sourceLimits[key] = 60
			}
			cause := fmt.Errorf("逐星采矿：%w", &SourcePlacementError{ItemID: 1001, AvailableRatePerMinute: tc.available, Err: errors.New("采集失败")})
			first, ok := repairAssignment(policy, Result{}, 101, cause, nil, nil)
			if !ok || !first.excludedSources[key] || len(first.excludedSources) != 1 || policy.excludedSources[key] || !reflect.DeepEqual(first.sourceLimits, policy.sourceLimits) || !reflect.DeepEqual(first.moves, policy.moves) {
				t.Fatalf("无进展反馈未限定排除范围：新 %+v，旧 %+v，可修复 %t", first, policy, ok)
			}
			second, again := repairAssignment(first, Result{}, 101, cause, nil, nil)
			if again || !reflect.DeepEqual(second, first) {
				t.Fatalf("同一失败仍被当作修复进展：%+v，%t", second, again)
			}
		})
	}
}

// sourceCapacityRate 职责：汇总指定物品的矿源分配；使用：断言实际规划速率；边界：不把进口端口重复计为采矿。
func sourceCapacityRate(site PlanetPlan, itemID int) float64 {
	rate := 0.0
	for _, mine := range site.Mines {
		if mine.ItemID == itemID {
			rate += mine.RatePerMinute
		}
	}
	return rate
}
