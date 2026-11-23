package industryplan

// 文件职责：验证资源调查、采集约束与缓存复用；使用：跨星分工回归；边界：不把候选标签或调查次数当成已确认矿量。

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestSharedMiningRequiresGeometry 检查单星尝试和全区缺矿时的上界；未经实际复核或已有明确速率的矿源沿用原值。
func TestSharedMiningRequiresGeometry(t *testing.T) {
	ore := Resource{ItemID: 1001, Kind: "solid", Count: 1, Source: "saved"}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{ore}}},
		ResolveResources: func(context.Context, int64) ([]Resource, bool, error) { return []Resource{ore}, true, nil }}
	req := Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 90}}}}
	estimate, err := Plan(t.Context(), req, world, nil, nil)
	if err != nil || estimate.Status != "infeasible" || len(estimate.ResourceGaps) != 1 || estimate.ResourceGaps[0].AvailablePerMinute != 30 {
		t.Fatalf("没有实体复核时不能承诺共享采矿：%s，%+v，%v", estimate.Status, estimate.ResourceGaps, err)
	}
	verified, err := Plan(t.Context(), req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if len(site.Mines) != 1 || site.Mines[0].RatePerMinute != 90 {
			t.Fatalf("没有向单星下达真实需要的矿业合同：%+v", site.Mines)
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !verified.Verified || len(verified.Planets) != 1 || len(verified.Routes) != 0 {
		t.Fatalf("全区初筛跳过了可以试排的单星：%s，%v", verified.Status, err)
	}
	req, _, err = normalize(req)
	if err != nil {
		t.Fatal(err)
	}
	req.probeSharedMining = true
	for _, tc := range []struct {
		resource Resource
		facility int
		want     float64
	}{
		{ore, 2301, 120}, {ore, 2316, 60},
		{Resource{Kind: "solid", Source: "saved", Count: 1, CapacityPerMinute: 22, UnitRatePerMinute: 11}, 2301, 22},
		{Resource{Kind: "oil", Source: "saved", Count: 1, BaseRatePerMinute: 18}, 2301, 18},
		{Resource{Kind: "solid", Source: "candidate", Count: 1}, 2301, 30},
	} {
		req.Mining.FacilityID = tc.facility
		if capacity, _, _ := resourceCapacity(req, tc.resource, 90); capacity != tc.want {
			t.Fatalf("%s 的设施 %d 上界为 %g，应为 %g", tc.resource.Kind, tc.facility, capacity, tc.want)
		}
	}
}

// TestMiningOrderConsolidatesSites 用一处零碎新矿和一处足额新矿验证选址；本地与已开工矿仍优先。
func TestMiningOrderConsolidatesSites(t *testing.T) {
	c := candidate{world: map[int64]*location{}, ids: []int64{101, 102, 201, 202}, req: Request{Mining: MiningSettings{FacilityID: 2301, SpeedMultiplier: 1.5, FacilityMultiplier: 1, CoveredVeins: 8}}}
	for i, id := range c.ids {
		count := float64(20)
		if id == 101 || id == 102 {
			count = 1
		}
		c.world[id] = &location{planet: Planet{ID: id, StarID: id / 100, Position: [3]float64{float64(i), 0, 0}, Resources: []Resource{{ItemID: 1006, Kind: "solid", Count: count}}}}
	}
	to := c.world[101]
	got := c.miningOrder(to, 1006, 600, nil)
	if !reflect.DeepEqual(got, []int64{101, 201, 202, 102}) {
		t.Fatalf("应在本地矿之后先开足额矿区：%v", got)
	}
	c.world[202].plan.Ports = []Port{{ItemID: 1001, Kind: "import"}}
	got = c.miningOrder(to, 1006, 600, nil)
	if got[1] != 202 {
		t.Fatalf("相同产能应复用开工星球：%v", got)
	}
	c.sourceLimits = map[resourceSite]float64{{planetID: 202, itemID: 1006}: 0}
	got = c.miningOrder(to, 1006, 600, nil)
	if got[1] != 201 {
		t.Fatalf("不得优先采用已查明没有余量的矿区：%v", got)
	}
}

// TestMineServesItsOwnPlanetBeforeExport 防止按需求星编号先后把本星用矿发出，再为相同用量进口。
func TestMineServesItsOwnPlanetBeforeExport(t *testing.T) {
	manual := false
	c := candidate{ids: []int64{101, 201, 301}, world: map[int64]*location{},
		req: Request{EnableLogistics: &manual, Mining: MiningSettings{FacilityID: 2301, SpeedMultiplier: 1, FacilityMultiplier: 1, CoveredVeins: 4}}}
	for i, id := range c.ids {
		site := &location{planet: Planet{ID: id, StarID: id / 100, ResourcesKnown: true, Buildable: true, Position: [3]float64{float64(i) * 10000}}, balance: map[int]float64{}}
		if id != 101 {
			site.planet.Resources = []Resource{{ItemID: 1006, Kind: "solid", Count: 4}}
		}
		if id != 301 {
			site.balance[1006] = -120
		}
		c.world[id] = site
	}
	c.world[201].plan.Ports = []Port{{Kind: "delivery", ItemID: 6006, RatePerMinute: 1}}
	c.allocate(1006, true, false, false)
	if len(c.result.Routes) != 1 || c.result.Routes[0].FromPlanetID != 301 || c.result.Routes[0].ToPlanetID != 101 || c.result.Routes[0].RatePerMinute != 120 {
		t.Fatalf("应由 201 自用、301 向 101 供货：%+v", c.result.Routes)
	}
}

// TestMiningRatesAndSiteRepair 检查矿机的基础周期与工作倍率，真实选址失败时保留中心制造和交付。
func TestMiningRatesAndSiteRepair(t *testing.T) {
	ore := Resource{ItemID: 1001, Kind: "solid", Count: 4}
	for _, tc := range []struct {
		facility          int
		setting, capacity float64
	}{{2301, 1, 120}, {2316, 1, 240}, {2316, 3, 720}} {
		req, _, err := normalize(Request{Mining: MiningSettings{FacilityID: tc.facility, FacilityMultiplier: tc.setting}})
		capacity, _, _ := resourceCapacity(req, ore, 30)
		if err != nil || capacity != tc.capacity {
			t.Fatalf("矿机 %d 在档位 %g 的能力为 %g，错误 %v", tc.facility, tc.setting, capacity, err)
		}
	}
	req := Request{CenterPlanetID: 101, Profile: "low", Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}}}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{ore}}, {ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{ore}, Position: [3]float64{4000, 0, 0}}}}
	result, err := Plan(context.Background(), req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 && len(site.Mines) > 0 {
			return PlanetActual{}, &SourcePlacementError{ItemID: 1001, Err: errors.New("已占用的矿区")}
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !result.Verified || result.Repairs != 1 || len(result.Routes) != 1 || result.Routes[0].FromPlanetID != 102 || result.Routes[0].ToPlanetID != 101 || result.Routes[0].ItemID != 1001 {
		t.Fatalf("矿区修正错误：%v，路线 %+v，原因 %v", err, result.Routes, result.Warnings)
	}
}

// TestResourceRefinementBudget 确认新档候选只在有需求时逐颗展开，停止条件不会误把候选数当证明。
func TestResourceRefinementBudget(t *testing.T) {
	req, _, err := normalize(Request{Profile: "low", CenterPlanetID: 115})
	if err != nil {
		t.Fatal(err)
	}
	base := quant.Result{RawInputs: []quant.ItemRate{{ItemID: 1001, RatePerMinute: 60}}}
	for _, enough := range []bool{true, false} {
		planets := map[int64]Planet{}
		var ids, opened []int64
		for id := int64(101); id <= 120; id++ {
			ids = append(ids, id)
			planets[id] = Planet{ID: id, StarID: 1, Buildable: true, Resources: []Resource{{ItemID: 1001, Kind: "solid", Source: "candidate"}}}
		}
		world := World{ResolveResources: func(_ context.Context, id int64) ([]Resource, bool, error) {
			opened = append(opened, id)
			if enough {
				return []Resource{{ItemID: 1001, Kind: "solid", Source: "generated", Count: 4}}, true, nil
			}
			return nil, true, nil
		}}
		n, limited, err := resolveCandidateResources(context.Background(), req, world, base, planets, ids, 8, func(Progress) {})
		if err != nil || len(opened) != n || opened[0] != 115 {
			t.Fatal("候选未优先展开指定中心", opened, err)
		}
		if enough && (n != 1 || limited) || !enough && (n != 8 || !limited) {
			t.Fatalf("错误停止条件，已开 %d，预算到顶 %t", n, limited)
		}
	}
}

// TestResourceSurveyAndCache 职责：确认一轮查遍必要星球并区分不足、未知与足量；使用：调查窗口回归；边界：重复规划复用缓存，不降低真实矿量约束。
func TestResourceSurveyAndCache(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lastVeins float64
		known     bool
		status    string
	}{
		{"全区不足", 2, true, "infeasible"},
		{"仍缺资料", 0, false, "missing_data"},
		{"最后一星补足", 10, true, "conditional"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{Profile: "low", CenterPlanetID: 101, Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 750}}}}
			cache := make(map[int64][]Resource)
			var opened []int64
			world := World{CachedResources: func(id int64) ([]Resource, bool, bool) {
				rows, found := cache[id]
				return rows, id != 109 || tc.known, found
			}, ResolveResources: func(_ context.Context, id int64) ([]Resource, bool, error) {
				if _, seen := cache[id]; seen {
					t.Fatalf("重复计入星球 %d", id)
				}
				opened = append(opened, id)
				points := 2.0
				if id == 109 {
					points = tc.lastVeins
				}
				cache[id] = []Resource{{ItemID: 1001, Kind: "solid", Count: points}}
				return cache[id], id != 109 || tc.known, nil
			}}
			for id := int64(101); id <= 109; id++ {
				world.Planets = append(world.Planets, Planet{ID: id, StarID: 1, Buildable: true, Position: [3]float64{float64(id), 0, 0}, Resources: []Resource{{ItemID: 1001, Kind: "solid", Source: "candidate"}}})
			}
			plan := func() Result {
				t.Helper()
				result, err := Plan(context.Background(), req, world, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			first := plan()
			if len(opened) != 9 || first.ResourcesInspected != 9 || first.Status != tc.status || first.LimitStage != "" || opened[0] != 101 || opened[8] != 109 {
				t.Fatalf("首轮未完成必要调查：查矿 %v，状态 %s，阶段 %s，缺口 %+v", opened, first.Status, first.LimitStage, first.ResourceGaps)
			}
			if tc.status == "conditional" {
				if len(first.ResourceGaps) != 0 || first.Candidates == 0 {
					t.Fatal("矿量已足仍无法开始分工")
				}
				mined, delivered := 0.0, 0.0
				for _, site := range first.Planets {
					local := 0.0
					for _, mine := range site.Mines {
						if mine.ItemID == 1001 {
							local += mine.RatePerMinute
						}
					}
					capacity := 60.0
					if site.PlanetID == 109 {
						capacity = 300
					}
					if local > capacity {
						t.Fatalf("星球 %d 采矿 %g 超过真实矿点能力 %g", site.PlanetID, local, capacity)
					}
					mined += local
					for _, port := range site.Ports {
						if port.Kind == "delivery" && port.ItemID == 1101 {
							delivered += port.RatePerMinute
						}
					}
				}
				if mined != 750 || delivered != 750 {
					t.Fatalf("足量方案未完成真实采矿与净交付：采矿 %g，交付 %g", mined, delivered)
				}
			} else {
				if first.Candidates != 0 || len(first.ResourceGaps) != 1 {
					t.Fatalf("已确认的资源缺口仍进入布局：候选 %d，缺口 %+v", first.Candidates, first.ResourceGaps)
				}
				gap := first.ResourceGaps[0]
				available, unknown := 540.0, 0
				if !tc.known {
					available, unknown = 480, 1
				}
				if gap.ItemID != 1001 || gap.RequiredPerMinute != 750 || gap.AvailablePerMinute != available || gap.PendingPlanets != 0 || gap.UnknownPlanets != unknown {
					t.Fatalf("最终真实矿量或未知来源错误：%+v", gap)
				}
			}
			for repeat := 0; repeat < 2; repeat++ {
				cached := plan()
				if cached.ResourcesInspected != 0 || len(opened) != 9 || cached.Status != tc.status || cached.LimitStage != "" || !reflect.DeepEqual(cached.ResourceGaps, first.ResourceGaps) {
					t.Fatalf("缓存复用改变矿源结论或重复调查：新增 %d，状态 %s，缺口 %+v", cached.ResourcesInspected, cached.Status, cached.ResourceGaps)
				}
			}
		})
	}
}

// TestDeliveryRowsAndOrbitalLimit 防止同物品的交付目的地被覆盖，也不把巨星的已知速率当成可粘贴设施。
func TestDeliveryRowsAndOrbitalLimit(t *testing.T) {
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true}, {ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{3000, 0, 0}}, {ID: 103, StarID: 1, ResourcesKnown: true, Resources: []Resource{{ItemID: 1011, Kind: "orbital", BaseRatePerMinute: 60, UnitRatePerMinute: 30}}}}}
	off, on := false, true
	r := Request{CenterPlanetID: 101, EnableMining: &off, TargetPlanetIDs: []int64{101, 102}, Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 10}, {ItemID: 1101, RatePerMinute: 20}}}}
	result, err := Plan(context.Background(), r, world, nil, nil)
	delivered := map[int64]float64{}
	for _, p := range result.Planets {
		for _, port := range p.Ports {
			if port.Kind == "delivery" {
				delivered[p.PlanetID] += port.RatePerMinute
			}
		}
	}
	if err != nil || delivered[101] != 10 || delivered[102] != 20 {
		t.Fatalf("同物品的目的地被合并，%v %v", delivered, err)
	}
	r = Request{CenterPlanetID: 101, EnableOrbitalCollectors: &on, Quant: quant.Request{Targets: []quant.Target{{ItemID: 1011, RatePerMinute: 10}}}}
	result, err = Plan(context.Background(), r, world, nil, nil)
	if err != nil || result.Status != "infeasible" || !hasExternalKind(result, "collector-placement") {
		t.Fatalf("巨星放置限制未回传，结果 %s，%v %v", result.Status, result.External, err)
	}
}

// TestRouteRestrictions 验证原档禁运先于船队分配，多对多同品货物不能因共组而放开未确认的航线。
func TestRouteRestrictions(t *testing.T) {
	from, to := &location{planet: Planet{ID: 101}}, &location{planet: Planet{ID: 201}}
	req, profile, err := normalize(Request{})
	if err != nil {
		t.Fatal(err)
	}
	c := candidate{req: req, profile: profile, remoteRules: func(a, b, item int64) (bool, bool) { return true, a == 101 && b == 201 && item == 1120 }}
	if value := c.transfer(from, to, 1120, 30); value != 0 || len(c.result.Routes) != 0 || c.bannedRoute == "" {
		t.Fatal("已有禁运未阻止航运", value)
	}
	routes := []Route{{FromPlanetID: 101, ToPlanetID: 201, ItemID: 1120, VesselLoad: 1}, {FromPlanetID: 101, ToPlanetID: 302, ItemID: 1120, VesselLoad: 1}}
	if needsStationRoutes(routes) {
		t.Fatal("单一来源不需要手工站对站配对")
	}
	routes = append(routes, Route{FromPlanetID: 102, ToPlanetID: 201, ItemID: 1120, VesselLoad: 1})
	if !needsStationRoutes(routes) {
		t.Fatal("多源多目的同品物流仍允许普通共组匹配")
	}
}
