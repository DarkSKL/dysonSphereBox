package industryplan

// 文件职责：核对能源与临界光子的行为约束；使用：规划能源回归；边界：合成星球只提供明确输入，不替代实际蓝图落位验收。

import (
	"context"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/quant"
)

// energyCandidateForTest 职责：建立带明确持续负荷的候选；使用：能源分配隔离测试；边界：负荷不包含隐含制造、矿业或环境修正。
func energyCandidateForTest(t *testing.T, req Request, loads map[int64]float64) *candidate {
	t.Helper()
	req, profile, err := normalize(req)
	if err != nil {
		t.Fatal(err)
	}
	c := &candidate{req: req, world: map[int64]*location{}, profile: profile, sphereUsed: map[int64]float64{}}
	for id, load := range loads {
		planet := Planet{ID: id, StarID: id / 100, Name: "测试星球", Buildable: true, ResourcesKnown: true, WindFactor: 1, Position: [3]float64{float64(id) * 10000, 0, 0}}
		c.ids = append(c.ids, id)
		c.world[id] = &location{planet: planet, balance: map[int]float64{}, plan: PlanetPlan{PlanetID: id}}
		if load > 0 {
			c.world[id].plan.Plan.Productions = []quant.Production{{PowerMW: load}}
		}
	}
	slices.Sort(c.ids)
	c.center, c.storage = c.ids[0], c.ids[0]
	return c
}

// energyPlanForTest 职责：通过完整入口执行短合成规划；使用：声明与制造闭合回归；边界：超时作为失败，不接受未完成候选。
func energyPlanForTest(t *testing.T, req Request, world World) (Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return Plan(ctx, req, world, nil, nil)
}

// TestEnergyDisabledIgnoresRetainedChoices 职责：关闭供电时忽略隐藏的未完成设置；使用：完整入口开关往返回归；边界：重新开启仍严格校验并保留外电负荷声明。
func TestEnergyDisabledIgnoresRetainedChoices(t *testing.T) {
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true,
		Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 100}}}}}
	for _, tc := range []struct {
		name   string
		energy EnergySettings
		error  string
	}{
		{"grid-capacity", EnergySettings{Default: EnergyChoice{Mode: "grid"}}, "电网可用于新产线的余量"},
		{"charging-site", EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2209}}, "充电星球"},
		{"override-grid", EnergySettings{Default: EnergyChoice{Mode: "auto"}, Overrides: []EnergyOverride{
			{PlanetID: 101, Choice: EnergyChoice{Mode: "grid"}},
		}}, "电网可用于新产线的余量"},
		{"override-charging-site", EnergySettings{Default: EnergyChoice{Mode: "auto"}, Overrides: []EnergyOverride{
			{PlanetID: 101, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209}},
		}}, "充电星球"},
		{"stale-override", EnergySettings{Default: EnergyChoice{Mode: "auto"}, Overrides: []EnergyOverride{
			{PlanetID: 999, Choice: EnergyChoice{Mode: "grid"}},
		}}, "无效或重复星球"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{CenterPlanetID: 101, Energy: tc.energy,
				Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}}}
			result, err := energyPlanForTest(t, req, world)
			if err != nil || result.Status != "conditional" || len(result.Planets) == 0 {
				t.Fatalf("关闭供电后应可条件规划：err=%v status=%s warnings=%v", err, result.Status, result.Warnings)
			}
			gridCount := 0
			for _, external := range result.External {
				if external.Kind == "grid-power" {
					gridCount++
					if external.PlanetID != 101 || !(external.PowerMW > 0) || !finite(external.PowerMW) {
						t.Fatalf("外电条件缺少实际负荷：%+v", external)
					}
				}
			}
			if gridCount != 1 {
				t.Fatalf("外电负荷条件重复或缺失：%+v", result.External)
			}
			for _, planet := range result.Planets {
				if planet.Power != nil {
					t.Fatalf("关闭供电仍生成了电源合同：%+v", planet.Power)
				}
			}
			req.EnablePower = true
			if _, err := energyPlanForTest(t, req, world); err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("重新开启应恢复电源校验：want=%q err=%v", tc.error, err)
			}
		})
	}
}

// TestEnergyDisabledKeepsDysonConstraints 职责：关闭供电不免除光子的声明与恒星额度；使用：完整入口回归；边界：多个接收星不能复制同一戴森球余量。
func TestEnergyDisabledKeepsDysonConstraints(t *testing.T) {
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 201, StarID: 2, Buildable: true, ResourcesKnown: true},
	}}
	source := DysonSupply{StarID: 1, PlanetIDs: []int64{101, 102}, AvailableMW: 120, Efficiency: 1, Uptime: 1}
	for _, tc := range []struct {
		name   string
		dyson  DysonSettings
		status string
		error  string
	}{
		{"missing", DysonSettings{Mode: "unconfigured"}, "missing_data", ""},
		{"shared-capacity", DysonSettings{Mode: "sphere", Supplies: []DysonSupply{source}}, "infeasible", ""},
		{"duplicate-star", DysonSettings{Mode: "sphere", Supplies: []DysonSupply{source, source}}, "", "同一恒星只能设置一次"},
		{"other-star", DysonSettings{Mode: "sphere", Supplies: []DysonSupply{
			{StarID: 1, PlanetIDs: []int64{201}, AvailableMW: 240, Efficiency: 1, Uptime: 1},
		}}, "", "接收星球必须属于所选恒星"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{CenterPlanetID: 101, Energy: EnergySettings{Default: EnergyChoice{Mode: "grid"}}, Dyson: tc.dyson,
				Quant: quant.Request{Targets: []quant.Target{{ItemID: 1208, RatePerMinute: 12}}}}
			result, err := energyPlanForTest(t, req, world)
			if tc.error != "" {
				if err == nil || !strings.Contains(err.Error(), tc.error) {
					t.Fatalf("关闭供电不应绕过戴森球声明校验：want=%q err=%v", tc.error, err)
				}
			} else if err != nil || result.Status != tc.status {
				t.Fatalf("关闭供电不应绕过光子供给限制：err=%v status=%s want=%s warnings=%v", err, result.Status, tc.status, result.Warnings)
			}
		})
	}
}

// TestEnergyPhotonDeclaration 职责：禁止隐式外供光子；使用：用户未声明与显式外供回归；边界：关闭采矿不能免除戴森球前提。
func TestEnergyPhotonDeclaration(t *testing.T) {
	disabled := false
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true}}}
	for _, mode := range []string{"", "unconfigured", "external"} {
		t.Run("mode="+mode, func(t *testing.T) {
			req := Request{CenterPlanetID: 101, EnableMining: &disabled,
				Quant:  quant.Request{Targets: []quant.Target{{ItemID: 1208, RatePerMinute: 6}}},
				Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}}, Dyson: DysonSettings{Mode: mode}}
			result, err := energyPlanForTest(t, req, world)
			if mode != "external" {
				if err == nil && result.Status != "missing_data" && result.Status != "infeasible" {
					t.Fatalf("未声明戴森球或外供却接受光子需求: status=%s external=%+v", result.Status, result.External)
				}
				return
			}
			if err != nil || result.Status != "conditional" {
				t.Fatalf("显式光子外供应可条件交付: %v status=%s warnings=%v", err, result.Status, result.Warnings)
			}
			rate := 0.0
			for _, external := range result.External {
				if external.ItemID == 1208 && external.Kind == "declared-input" {
					rate += external.RatePerMinute
				}
			}
			if math.Abs(rate-6) > 1e-6 {
				t.Fatalf("显式光子外供缺失或重复: %.6f", rate)
			}
			for _, site := range result.Planets {
				if site.Photons != nil {
					t.Fatal("外供光子不应额外生成射线采集设备")
				}
			}
		})
	}
}

// TestEnergyDysonReceiverOwnership 职责：验证接收星归属与唯一额度；使用：声明入口回归；边界：不能借其他恒星的功率。
func TestEnergyDysonReceiverOwnership(t *testing.T) {
	planets := map[int64]Planet{
		101: {ID: 101, StarID: 1, Buildable: true}, 102: {ID: 102, StarID: 1, Buildable: true},
		201: {ID: 201, StarID: 2, Buildable: true}, 103: {ID: 103, StarID: 1},
	}
	valid := DysonSupply{StarID: 1, PlanetIDs: []int64{101, 102}, AvailableMW: 240, Efficiency: 1, Uptime: 1}
	req := Request{Dyson: DysonSettings{Mode: "sphere", Supplies: []DysonSupply{valid}}}
	if err := validateEnergy(req, planets); err != nil {
		t.Fatal(err)
	}
	for name, sources := range map[string][]DysonSupply{
		"other-star":  {{StarID: 1, PlanetIDs: []int64{201}, AvailableMW: 240, Efficiency: 1, Uptime: 1}},
		"unbuildable": {{StarID: 1, PlanetIDs: []int64{103}, AvailableMW: 240, Efficiency: 1, Uptime: 1}},
		"same-star":   {valid, {StarID: 1, PlanetIDs: []int64{101}, AvailableMW: 240, Efficiency: 1, Uptime: 1}},
		"same-planet": {{StarID: 1, PlanetIDs: []int64{101, 101}, AvailableMW: 240, Efficiency: 1, Uptime: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			req.Dyson.Supplies = sources
			if err := validateEnergy(req, planets); err == nil {
				t.Fatalf("无效戴森球声明被接受: %+v", sources)
			}
		})
	}
}

// TestEnergyDysonBudgetSharedByReceivers 职责：限制多星光子采集总额；使用：同恒星多接收星回归；边界：多个接收星不复制戴森球容量。
func TestEnergyDysonBudgetSharedByReceivers(t *testing.T) {
	req := Request{Dyson: DysonSettings{Mode: "sphere", Supplies: []DysonSupply{{StarID: 1, PlanetIDs: []int64{101, 102}, AvailableMW: 120, Efficiency: 1, Uptime: 1}}}}
	c := energyCandidateForTest(t, req, map[int64]float64{101: 0, 102: 0})
	c.world[101].balance[1208], c.world[102].balance[1208] = -6, -6
	c.allocatePhotons(false)
	if c.failure == "" {
		t.Fatal("120 MW 戴森球不应在两颗星上分别提供 6 光子/分钟")
	}
	collected, spent := 0.0, 0.0
	for _, site := range c.world {
		if p := site.plan.Photons; p != nil {
			collected += p.RatePerMinute
			spent += p.SpherePowerMW
		}
	}
	if collected > 6+1e-6 || spent > 120+1e-6 || math.Abs(c.sphereUsed[1]-spent) > 1e-6 {
		t.Fatalf("同恒星功率被重复使用: photons=%g spent=%g ledger=%g", collected, spent, c.sphereUsed[1])
	}
}

// TestEnergyDysonBudgetSharedWithElectricity 职责：让光子与发电共用额度；使用：光子采集后的电源分配；边界：两种接收模式均计入同一恒星。
func TestEnergyDysonBudgetSharedWithElectricity(t *testing.T) {
	for _, capacity := range []float64{120, 135} {
		req := Request{EnablePower: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2208}},
			Dyson: DysonSettings{Mode: "sphere", Supplies: []DysonSupply{{StarID: 1, PlanetIDs: []int64{101, 102}, AvailableMW: capacity, Efficiency: 1, Uptime: 1}}}}
		c := energyCandidateForTest(t, req, map[int64]float64{101: 0, 102: 15})
		c.world[101].balance[1208] = -6
		c.allocatePhotons(false)
		if c.failure != "" {
			t.Fatal(c.failure)
		}
		err := c.planEnergy()
		if capacity == 120 && err == nil {
			t.Error("已用满 120 MW 采集光子后不应再接入 15 MW 射线发电")
		}
		if capacity == 135 && (err != nil || math.Abs(c.sphereUsed[1]-135) > 1e-6) {
			t.Errorf("135 MW 应刚好覆盖光子和发电: used=%g err=%v", c.sphereUsed[1], err)
		}
	}
}

// TestEnergyAutomaticLargeLoadUsesNonCoal 职责：验证大负荷的自动电源策略；使用：避免默认大量煤火电回归；边界：不固定评分公式或具体优胜设备。
func TestEnergyAutomaticLargeLoadUsesNonCoal(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := energyCandidateForTest(t, Request{Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}}, Dyson: DysonSettings{Mode: "unconfigured"}}, map[int64]float64{101: 2000})
	source, err := c.chooseEnergy(c.world[101], 2000, catalog)
	if err != nil || source.FacilityID == 2204 || source.PowerMW < 2000 || source.Count <= 0 {
		t.Fatalf("大负荷应在有条件的其他电源中选择: %+v, %v", source, err)
	}
	c.req.Energy.AllowedFacilityIDs = []int{2204}
	source, err = c.chooseEnergy(c.world[101], 2000, catalog)
	if err != nil || source.FacilityID != 2204 {
		t.Fatalf("用户限制电源后仍应尊重选择: %+v, %v", source, err)
	}
}

// TestEnergyAutomaticExcludesStorage 职责：防止自动推荐把搬运或储能设备当作电源；使用：用户允许列表回归；边界：固定枢纽的充电合同由既有测试覆盖。
func TestEnergyAutomaticExcludesStorage(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := energyCandidateForTest(t, Request{Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}, AllowedFacilityIDs: []int{2206, 2209}}}, map[int64]float64{101: 54})
	if source, err := c.chooseEnergy(c.world[101], 54, catalog); err == nil || source.Count != 0 {
		t.Fatalf("蓄电器或无充电闭环的枢纽被推荐为持续电源: %+v, %v", source, err)
	}
	c.req.Energy.AllowedFacilityIDs = append(c.req.Energy.AllowedFacilityIDs, 2203)
	if source, err := c.chooseEnergy(c.world[101], 54, catalog); err != nil || source.FacilityID != 2203 {
		t.Fatalf("有风力时应使用真实发电设备: %+v, %v", source, err)
	}
}

// TestEnergyAutomaticRequiresFuelSupply 职责：不推荐没有上游来源的电厂；使用：自动内产与明确外供对比；边界：选择外供燃料后不再虚构内部燃料生产。
func TestEnergyAutomaticRequiresFuelSupply(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{EnableFuel: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}, AllowedFacilityIDs: []int{2211}}}
	c := energyCandidateForTest(t, req, map[int64]float64{101: 150})
	if _, err := c.chooseEnergy(c.world[101], 150, catalog); err == nil {
		t.Fatal("没有燃料原料的存档不应被推荐氘棒内产")
	}
	c.req.Quant.Settings.MineralizeList = map[string]bool{"氘核燃料棒": true}
	if source, err := c.chooseEnergy(c.world[101], 150, catalog); err != nil || source.FacilityID != 2211 {
		t.Fatalf("明确外供氘棒应允许聚变: %+v, %v", source, err)
	}
	c.req.Quant.Settings.MineralizeList = nil
	c.req.EnableFuel = false
	if source, err := c.chooseEnergy(c.world[101], 150, catalog); err != nil || source.FacilityID != 2211 {
		t.Fatalf("关闭燃料内产应保留玩家外供电厂: %+v, %v", source, err)
	}
}

// TestEnergyAutomaticFuelSharesDysonRemainder 职责：给反物质燃料预留同一戴森球的剩余额度；使用：两个用电星的自动推荐；边界：原生产光子优先且不能按星球复制额度。
func TestEnergyAutomaticFuelSharesDysonRemainder(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{EnableFuel: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}, AllowedFacilityIDs: []int{2210}},
		Quant: quant.Request{Settings: quant.Settings{MineralizeList: map[string]bool{"湮灭约束球": true, "钛合金": true}}},
		Dyson: DysonSettings{Mode: "sphere", Supplies: []DysonSupply{{StarID: 1, PlanetIDs: []int64{101}, AvailableMW: 240, Efficiency: 1, Uptime: 1}}}}
	c := energyCandidateForTest(t, req, map[int64]float64{101: 72, 102: 72})
	c.sphereUsed[1] = 120
	estimates := newEnergyEstimates()
	first, err := c.chooseEnergyEstimated(c.world[101], 72, catalog, estimates)
	if err != nil || first.FacilityID != 2210 {
		t.Fatalf("剩余 120 MW 应能供一台恒星的普通反物质棒: %+v, %v", first, err)
	}
	if second, err := c.chooseEnergyEstimated(c.world[102], 72, catalog, estimates); err == nil {
		t.Fatalf("两台恒星合计需要 7.2 光子/分，不应共享仅剩的 6 光子/分额度: %+v", second)
	}
	if c.sphereUsed[1] != 120 {
		t.Fatal("估价不应提前扣除实际戴森球功率，避免下一轮重复记账")
	}
}

// TestEnergyAutomaticRejectsNegativeNetGeneration 职责：拒绝燃料制造耗电大于其供电的反馈环；使用：能源守恒边界；边界：负荷为单位燃料估价的合成输入。
func TestEnergyAutomaticRejectsNegativeNetGeneration(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := energyCandidateForTest(t, Request{EnableFuel: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}, AllowedFacilityIDs: []int{2211, 2203}}}, map[int64]float64{101: 15})
	for _, cost := range []float64{10, 11} {
		estimates := newEnergyEstimates()
		// 一台聚变提供 15 MW，消耗 1.5 棒/分；每棒/分制造耗电 >=10 MW 时没有净供电。
		estimates.fuels[1802] = fuelEstimate{powerMW: cost}
		source, err := c.chooseEnergyEstimated(c.world[101], 15, catalog, estimates)
		if err != nil || source.FacilityID != 2203 {
			t.Fatalf("无净供电的聚变链不应被推荐: perFuel=%g source=%+v err=%v", cost, source, err)
		}
	}
}

// TestEnergyFuelEstimationUsesRecipeSettings 职责：尊重用户的配方和外供选择；使用：真实原版反物质燃料辅助量化；边界：只估计一件每分钟，不把取整机器放大。
func TestEnergyFuelEstimationUsesRecipeSettings(t *testing.T) {
	c := energyCandidateForTest(t, Request{EnableFuel: true, Quant: quant.Request{Settings: quant.Settings{MineralizeList: map[string]bool{"湮灭约束球": true, "钛合金": true}}}}, map[int64]float64{101: 72})
	estimate := c.estimateFuel(1803, newEnergyEstimates())
	if estimate.err != nil {
		t.Fatal(estimate.err)
	}
	photons := 0.0
	for _, raw := range estimate.plan.RawInputs {
		if raw.ItemID == 1208 {
			photons += raw.RatePerMinute
		}
	}
	if math.Abs(photons-6) > 1e-6 || estimate.machines <= 0 || estimate.machines >= float64(estimate.plan.MachineCount) || estimate.powerMW < estimate.plan.PowerMW {
		t.Fatalf("反物质辅助链估价不符: photons=%g exact=%g rounded=%d power=%g planPower=%g", photons, estimate.machines, estimate.plan.MachineCount, estimate.powerMW, estimate.plan.PowerMW)
	}
	c.req.Quant = c.req.Quant.WithUnlockedRecipes(func(id int) bool { return id != 44 })
	if locked := c.estimateFuel(1803, newEnergyEstimates()); locked.err == nil || !strings.Contains(locked.err.Error(), "尚未解锁「反物质燃料棒」") {
		t.Fatalf("辅助燃料估价应报告锁定的制造配方：%v", locked.err)
	}
	c.req.Quant.Settings.MineralizeList["反物质燃料棒"] = true
	external := c.estimateFuel(1803, newEnergyEstimates())
	if external.err != nil || external.machines != 0 || external.powerMW != 0 || !slices.Contains(external.plan.ExternalInputItemIDs, 1803) {
		t.Fatalf("明确外供燃料不应保留内部机器和功耗: %+v", external)
	}
}

// TestEnergyFixedFuelCompatibility 职责：验证指定燃料的兼容性与额定；使用：火电、聚变、恒星选择；边界：不接受其他燃料类别。
func TestEnergyFixedFuelCompatibility(t *testing.T) {
	catalog, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := energyCandidateForTest(t, Request{}, map[int64]float64{101: 1})
	for _, tc := range []struct {
		facility, fuel int
		mw, rate       float64
	}{{2204, 1120, 2.16, 18}, {2211, 1802, 15, 1.5}, {2210, 1803, 72, .6}, {2210, 1804, 144, .12}} {
		source, err := c.sourceFor(c.world[101], EnergyChoice{Mode: "fixed", FacilityID: tc.facility, FuelItemID: tc.fuel}, 1, catalog)
		if err != nil || source.FuelItemID != tc.fuel || math.Abs(source.PowerMW-tc.mw) > 1e-6 || math.Abs(source.FuelRatePerMinute-tc.rate) > 1e-6 {
			t.Errorf("指定燃料额定不符: facility=%d fuel=%d source=%+v err=%v", tc.facility, tc.fuel, source, err)
		}
	}
	for _, pair := range [][2]int{{2204, 1802}, {2211, 1006}, {2210, 1802}, {2204, 2207}, {2210, 1001}} {
		if _, err := c.sourceFor(c.world[101], EnergyChoice{Mode: "fixed", FacilityID: pair[0], FuelItemID: pair[1]}, 1, catalog); err == nil {
			t.Errorf("电厂 %d 不应接受燃料 %d", pair[0], pair[1])
		}
	}
}

// TestEnergyFuelManufacturingClosesPower 职责：验证燃料制造与采矿负荷回算；使用：完整量化和规划回归；边界：能源配套物品不能成为用户交付或隐式外供。
func TestEnergyFuelManufacturingClosesPower(t *testing.T) {
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
		{ItemID: 1001, Kind: "solid", Count: 100}, {ItemID: 1006, Kind: "solid", Count: 100},
	}}}}
	req := Request{CenterPlanetID: 101, EnablePower: true, Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 240}}},
		Energy: EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2204, FuelItemID: 1109}}}
	baseline, err := energyPlanForTest(t, req, world)
	if err != nil || len(baseline.Planets) != 1 || baseline.Planets[0].Power == nil {
		t.Fatalf("外供燃料基准未生成: %v status=%s warnings=%v", err, baseline.Status, baseline.Warnings)
	}
	req.EnableFuel = true
	result, err := energyPlanForTest(t, req, world)
	if err != nil || len(result.Planets) == 0 || result.Status == "infeasible" || result.Status == "missing_data" {
		t.Fatalf("燃料制造配套应能闭合: %v status=%s warnings=%v", err, result.Status, result.Warnings)
	}
	fuelOutput, fuelUse, coalRate, required := 0.0, 0.0, 0.0, 0.0
	for _, site := range result.Planets {
		if site.Power == nil || site.Power.PowerMW+1e-6 < site.Power.RequiredMW {
			t.Fatalf("配套制造后的电力不足: %+v", site.Power)
		}
		required += site.Power.RequiredMW
		for _, production := range site.Plan.Productions {
			for _, output := range production.Outputs {
				if output.ItemID == 1109 {
					fuelOutput += output.RatePerMinute
				}
			}
			for _, input := range production.Inputs {
				if input.ItemID == 1109 {
					fuelOutput -= input.RatePerMinute
				}
			}
		}
		for _, port := range site.Ports {
			if port.ItemID == 1109 && port.Kind == "power-fuel" {
				fuelUse += port.RatePerMinute
			}
			if port.ItemID == 1109 && (port.Kind == "delivery" || port.Kind == "external") {
				t.Fatalf("辅助燃料不应变成用户目标或外供: %+v", port)
			}
		}
		for _, mine := range site.Mines {
			if mine.ItemID == 1006 {
				coalRate += mine.RatePerMinute
			}
		}
	}
	if fuelUse <= 0 || fuelOutput+1e-6 < fuelUse || coalRate+1e-6 < fuelUse*2 || required <= baseline.Planets[0].Power.RequiredMW {
		t.Fatalf("燃料产量、煤矿或额外用电未闭合: output=%g use=%g coal=%g required=%g base=%g", fuelOutput, fuelUse, coalRate, required, baseline.Planets[0].Power.RequiredMW)
	}
	if hasExternalKind(result, "fuel-input") || hasExternalKind(result, "grid-power") {
		t.Fatalf("完整配套仍有燃料或电力缺口: %+v", result.External)
	}
}

// TestAutoPowerChangeReleasesAuxiliaryCoal 核对煤量只够首次估算、加入采煤用电后改风电的闭环，保留玩家自己要求的煤交付。
func TestAutoPowerChangeReleasesAuxiliaryCoal(t *testing.T) {
	ctx := t.Context()
	const requestedCoal = 30.0
	store := false
	req, profile, err := normalize(Request{CenterPlanetID: 101, EnablePower: true, EnableFuel: true, EnableStorage: &store,
		Quant:  quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 6000}, {ItemID: 1006, RatePerMinute: requestedCoal}}},
		Energy: EnergySettings{Default: EnergyChoice{Mode: "auto"}, AllowedFacilityIDs: []int{2204, 2203}}})
	if err != nil {
		t.Fatal(err)
	}
	planet := Planet{ID: 101, StarID: 1, Buildable: true, WindFactor: 1, ResourcesKnown: true,
		Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 1000}, {ItemID: 1006, Kind: "solid", Count: 1000}}}
	world := World{Planets: []Planet{planet}}
	ids := []int64{planet.ID}
	planets := map[int64]Planet{planet.ID: planet}
	base, err := quant.Solve(ctx, req.Quant)
	if err != nil {
		t.Fatal(err)
	}
	base, err = SplitProductions(ctx, base, 32)
	if err != nil {
		t.Fatal(err)
	}
	policy := assignment{center: planet.ID}
	initial := evaluate(ctx, req, base, planets, ids, ids, profile, policy, nil)
	if len(initial.EnergyNeeds) != 1 || initial.EnergyNeeds[0].ItemID != 1006 || initial.EnergyNeeds[0].RatePerMinute <= 0 {
		t.Fatalf("首次估算应当选燃煤：%+v，%v", initial.EnergyNeeds, initial.Warnings)
	}
	policy.sourceLimits = map[resourceSite]float64{{planetID: planet.ID, itemID: 1006}: requestedCoal + initial.EnergyNeeds[0].RatePerMinute}
	result := evaluateEnergy(ctx, req, base, world, planets, ids, ids, profile, policy, func(Progress) {})
	if len(result.Planets) != 1 || result.Planets[0].Power == nil || result.Planets[0].Power.FacilityID != 2203 || len(result.EnergyNeeds) != 0 {
		t.Fatalf("旧燃煤备产不能阻止本轮风电方案成立：%v，%v，%+v", result.Status, result.Warnings, result.EnergyNeeds)
	}
	coal := map[string]float64{}
	for _, p := range result.Planets[0].Ports {
		if p.ItemID == 1006 {
			coal[p.Kind] += p.RatePerMinute
		}
	}
	if coal["mining"] != requestedCoal || coal["delivery"] != requestedCoal || len(coal) != 2 {
		t.Fatalf("更换电源后只应采集并交付玩家要求的 %g 煤/分钟，实得 %v", requestedCoal, coal)
	}
	req.Energy.Overrides = []EnergyOverride{{PlanetID: planet.ID, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2204, FuelItemID: 1109}}}
	req.settledAutoPower = map[int64]int{planet.ID: 2203}
	fixed := evaluateEnergy(ctx, req, base, world, planets, ids, ids, profile, assignment{center: planet.ID}, func(Progress) {})
	if len(fixed.Planets) != 1 || fixed.Planets[0].Power == nil || fixed.Planets[0].Power.FacilityID != 2204 ||
		fixed.Planets[0].Power.FuelItemID != 1109 || len(fixed.EnergyNeeds) != 1 || fixed.EnergyNeeds[0].ItemID != 1109 {
		t.Fatalf("自动试排的电源不能覆盖玩家逐星指定的烧石墨火电：%s，%v", fixed.Status, fixed.Warnings)
	}
}

// TestEnergyExchangeRejectsCycles 职责：禁止电池循环凭空供能；使用：自充电和跨星循环回归；边界：充电端必须有独立电源。
func TestEnergyExchangeRejectsCycles(t *testing.T) {
	for _, peer := range []int64{101, 102} {
		req := Request{EnablePower: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: peer},
			Overrides: []EnergyOverride{{PlanetID: 102, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 101}}}}}
		c := energyCandidateForTest(t, req, map[int64]float64{101: 54, 102: 54})
		if err := c.planEnergy(); err == nil {
			t.Fatalf("充放电循环被接受: charge=%d", peer)
		}
	}
}

// TestEnergyExchangeConservesSharedCharging 职责：核对多放电星共用充电端的设备和能量；使用：共享充电基础设施；边界：每对满空电池均须往返且不能作为持续耗材制造。
func TestEnergyExchangeConservesSharedCharging(t *testing.T) {
	req := Request{EnablePower: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "fixed", FacilityID: 2211}, Overrides: []EnergyOverride{
		{PlanetID: 102, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 101}},
		{PlanetID: 103, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 101}},
	}}}
	c := energyCandidateForTest(t, req, map[int64]float64{101: 0, 102: 54, 103: 54})
	if err := c.planEnergy(); err != nil {
		t.Fatal(err)
	}
	charge := c.world[101].plan.Power
	if charge == nil || charge.RequiredMW < 108 || charge.PowerMW < charge.RequiredMW {
		t.Fatalf("充电端缺少两路合计 108 MW 的独立供能: %+v", charge)
	}
	for _, peer := range []int64{102, 103} {
		chargers := 0
		for _, source := range charge.Sources {
			if source.FacilityID == 2209 && source.Mode == "charge" && source.PeerPlanetID == peer {
				chargers += source.Count
			}
		}
		full, empty := 0.0, 0.0
		for _, route := range c.result.Routes {
			if route.FromPlanetID == 101 && route.ToPlanetID == peer && route.ItemID == 2207 {
				full += route.RatePerMinute
			}
			if route.FromPlanetID == peer && route.ToPlanetID == 101 && route.ItemID == 2206 {
				empty += route.RatePerMinute
			}
		}
		if chargers != 1 || math.Abs(full-6) > 1e-6 || math.Abs(empty-full) > 1e-6 || c.world[peer].plan.Power == nil || math.Abs(c.world[peer].plan.Power.PowerMW-full*9) > 1e-6 {
			t.Errorf("星球 %d 的电池闭环不守恒: chargers=%d full=%g empty=%g power=%+v", peer, chargers, full, empty, c.world[peer].plan.Power)
		}
	}
	for _, need := range c.result.EnergyNeeds {
		if need.ItemID == 2206 || need.ItemID == 2207 {
			t.Fatalf("循环电池被当作持续制造耗材: %+v", need)
		}
	}
}

// TestEnergyExchangeChargesFromExistingGrid 职责：核对已有电网承担真实充电负荷；使用：共享充电端回归；边界：声明余量包含本地负荷，外电不能计为新建发电。
func TestEnergyExchangeChargesFromExistingGrid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		capacity float64
		allowed  bool
	}{{"enough", 120, true}, {"insufficient", 113, false}} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{EnablePower: true, Energy: EnergySettings{Default: EnergyChoice{Mode: "grid", GridCapacityMW: tc.capacity}, Overrides: []EnergyOverride{
				{PlanetID: 102, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 101}},
				{PlanetID: 103, Choice: EnergyChoice{Mode: "fixed", FacilityID: 2209, ChargePlanetID: 101}},
			}}}
			c := energyCandidateForTest(t, req, map[int64]float64{101: 6, 102: 54, 103: 54})
			err := c.planEnergy()
			if !tc.allowed {
				if err == nil || !strings.Contains(err.Error(), "电网余量不足") {
					t.Fatalf("已有电网未覆盖本地 6 MW 与充电 108 MW 时应拒绝：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			power := c.world[101].plan.Power
			if power == nil || power.RequiredMW != 114 || power.ExternalGridMW != tc.capacity || power.PowerMW != 0 || power.Count != 0 || len(power.Sources) != 2 {
				t.Fatalf("外接容量、总负荷或真实充电设备缺失：%+v", power)
			}
			for i, peer := range []int64{102, 103} {
				source := power.Sources[i]
				if source.FacilityID != 2209 || source.Mode != "charge" || source.PeerPlanetID != peer || source.Count != 1 || source.PowerMW != 0 {
					t.Fatalf("充电源被外电声明覆盖或冒充发电：%+v", source)
				}
				discharge := c.world[peer].plan.Power
				if discharge == nil || discharge.PowerMW != 54 || discharge.ExternalGridMW != 0 {
					t.Fatalf("放电端错误使用充电端外电声明：%+v", discharge)
				}
			}
			gridCount := 0
			for _, external := range c.result.External {
				if external.Kind == "grid-power" {
					gridCount++
					if external.PlanetID != 101 || external.PowerMW != 114 {
						t.Fatalf("外电条件未记录实际负荷：%+v", external)
					}
				}
			}
			if gridCount != 1 {
				t.Fatalf("外电条件重复或缺失：%+v", c.result.External)
			}
		})
	}
}

// TestEnergySolarStorageCoversNight 职责：同时核对跨夜电量与充放电功率；使用：太阳能配套蓄电器；边界：日照和夜长是测试明确声明的条件。
func TestEnergySolarStorageCoversNight(t *testing.T) {
	for _, tc := range []struct{ duty, night float64 }{{.5, 600}, {.1, 1}, {1, 0}} {
		choice := EnergyChoice{Mode: "fixed", FacilityID: 2205, SolarFactor: 1, SolarDuty: tc.duty, NightSeconds: tc.night}
		c := energyCandidateForTest(t, Request{EnablePower: true, Energy: EnergySettings{Default: choice}}, map[int64]float64{101: 10})
		if err := c.planEnergy(); err != nil {
			t.Fatal(err)
		}
		power := c.world[101].plan.Power
		if power == nil || power.PowerMW < 10 {
			t.Fatalf("太阳能平均供电不足: %+v", power)
		}
		count, stored, dayGeneration := 0, 0.0, 0.0
		for _, source := range power.Sources {
			if source.FacilityID == 2206 {
				if source.Mode != "buffer" || source.PowerMW != 0 {
					t.Fatalf("蓄电器不应算作持续发电: %+v", source)
				}
				count += source.Count
				stored += source.StoredMJ
			}
			if source.FacilityID == 2205 {
				dayGeneration += source.PowerMW / tc.duty
			}
		}
		if tc.duty == 1 {
			if count != 0 {
				t.Fatal("声明连续日照且无夜晚时不应增加跨夜电池")
			}
			continue
		}
		chargeMW := 10 * (1 - tc.duty) / tc.duty
		if stored+1e-6 < 10*tc.night || float64(count)*2.25 < 10 || float64(count)*1.5+1e-6 < chargeMW || dayGeneration-10+1e-6 < chargeMW {
			t.Errorf("跨夜储能或白昼充电不足: duty=%g night=%g count=%d stored=%g generation=%g", tc.duty, tc.night, count, stored, dayGeneration)
		}
	}
}
