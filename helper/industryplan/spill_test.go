package industryplan

// 文件职责：验证真实容量反馈后的制造后缀迁移；使用：规模规划回归；边界：合成容量只证明有限修复策略，不替代实体蓝图落位。

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"DysonSphereBox/helper/quant"
)

// TestSpreadFactorySeed 核对集中大单的每一块都有唯一归属，并覆盖不可用地表和缺失工序身份的后备。
func TestSpreadFactorySeed(t *testing.T) {
	world := map[int64]Planet{101: {ID: 101, StarID: 1, Buildable: true}, 102: {ID: 102, StarID: 1, Buildable: true, DrySurface: true}, 103: {ID: 103, StarID: 1, Buildable: true}, 201: {ID: 201, StarID: 2, Buildable: true, DrySurface: true}}
	var rows []quant.Production
	for range 32 {
		rows = append(rows, quant.Production{MachineCount: 32, RecipeID: 1, MainItemID: 1101})
	}
	a := PlanetPlan{PlanetID: 101, BlockIDs: productionBlockIDs(rows), Plan: quant.Result{MachineCount: 1024, Productions: rows}}
	seed := evaluated{result: Result{Plan: a.Plan, Planets: []PlanetPlan{a, {PlanetID: 103}, {PlanetID: 105}}}}
	hosts := []int64{101, 102, 103, 201}
	p, ok := spreadFactorySeed(t.Context(), seed, world, hosts, World{}, Request{})
	counts := map[int64]int{}
	for _, key := range a.BlockIDs {
		counts[p.moves[key]]++
	}
	if !ok || !p.estimateLocalPower || len(p.moves) != len(rows) || counts[102] <= counts[101] || counts[101] == 0 || counts[103] == 0 || len(counts) != 3 {
		t.Fatalf("集中合同分担或地表筛选有误：%v %v", ok, counts)
	}
	// 同为有水主题，大陆星可承接的连续工区多于岛屿星；纯陆星不应触发额外地形读取。
	survey := World{FactoryLandShare: func(_ context.Context, id int64) (float64, bool) {
		switch id {
		case 101:
			return .94, true
		case 103:
			return .095, true
		default:
			t.Fatalf("读取了分担范围外或已知纯陆的星球 %d", id)
			return 0, false
		}
	}}
	p, ok = spreadFactorySeed(t.Context(), seed, world, hosts, survey, Request{})
	counts = map[int64]int{}
	for _, id := range p.moves {
		counts[id]++
	}
	if !ok || counts[101] <= counts[103] || counts[103] == 0 {
		t.Fatalf("测得的连续陆地份额未用于分工：%v", counts)
	}
	seed.result.Planets[0].BlockIDs = nil
	if _, ok = spreadFactorySeed(t.Context(), seed, world, hosts, World{}, Request{}); ok {
		t.Fatal("没有工序身份的种子不能改写")
	}
}

// TestSpreadFactorySeedUsesVacantNeighbors 职责：核对已建旱星的占地及同星系空旱星分担。使用：大单首次分工回归。边界：已建邻星、陌生星系以及原星份额充足时均不增加承接星。
func TestSpreadFactorySeedUsesVacantNeighbors(t *testing.T) {
	planets := map[int64]Planet{
		101: {ID: 101, StarID: 1, Buildable: true, DrySurface: true, HasFactory: true},
		103: {ID: 103, StarID: 1, Buildable: true, DrySurface: true},
		104: {ID: 104, StarID: 1, Buildable: true, DrySurface: true, HasFactory: true},
		201: {ID: 201, StarID: 2, Buildable: true},
		203: {ID: 203, StarID: 2, Buildable: true, DrySurface: true},
		301: {ID: 301, StarID: 3, Buildable: true},
		401: {ID: 401, StarID: 4, Buildable: true, DrySurface: true},
	}
	var rows []quant.Production
	for range 64 {
		rows = append(rows, quant.Production{MachineCount: 32, RecipeID: 1, MainItemID: 1101})
	}
	origin := PlanetPlan{PlanetID: 101, BlockIDs: productionBlockIDs(rows), Plan: quant.Result{MachineCount: 2048, Productions: rows}}
	seed := evaluated{result: Result{Plan: origin.Plan, Planets: []PlanetPlan{origin, {PlanetID: 201}, {PlanetID: 301}}}}
	for _, share := range []float64{.1, .9} {
		world := World{FactoryLandShare: func(_ context.Context, id int64) (float64, bool) {
			if id != 101 && id != 201 && id != 301 {
				t.Fatalf("读取了原分工之外的星球 %d", id)
			}
			return share, true
		}}
		policy, changed := spreadFactorySeed(t.Context(), seed, planets, []int64{101, 103, 104, 201, 203, 301, 401}, world, Request{})
		hosted := map[int64]int{}
		for _, id := range policy.moves {
			hosted[id]++
		}
		if !changed || len(policy.moves) != len(rows) || policy.factoryLand[101] != share || hosted[104] != 0 || hosted[401] != 0 {
			t.Fatalf("已有厂区、分工总量或承接范围错误，份额 %v：%v", share, hosted)
		}
		if share == .1 && (hosted[103] <= hosted[101] || hosted[203] <= hosted[201]) || share == .9 && (hosted[103] != 0 || hosted[203] != 0) {
			t.Fatalf("空旱星未按可用面积分担，份额 %v：%v", share, hosted)
		}
	}
}

// TestOverflowUsesHostLand 验证原星第一块即失败时，岛屿先接本地矿的工序，其余交给陆地星；原集中方案仍可复查。
func TestOverflowUsesHostLand(t *testing.T) {
	source := spillTestSite(101, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	island := spillTestSite(103)
	island.Ports = []Port{{Kind: "mining", ItemID: 1001, RatePerMinute: 100}}
	single := assignment{fullFactories: map[int64]bool{101: true}, moves: map[string]int64{}}
	for i, block := range source.BlockIDs {
		source.Plan.Productions[i].Inputs = []quant.ItemRate{{ItemID: 1001, RatePerMinute: 1}}
		single.moves[block] = 103
	}
	previous := Result{Planets: []PlanetPlan{source, spillTestSite(102), island}}
	world := map[int64]Planet{101: {ID: 101, Buildable: true}, 102: {ID: 102, Buildable: true, DrySurface: true}, 103: {ID: 103, Buildable: true}}
	if _, changed := distributeFactoryOverflow(single, previous, 101, world); changed {
		t.Fatal("缺少实测前缀及地形的分工发生改变")
	}
	single.factoryLand = map[int64]float64{101: .4, 102: 1, 103: .1}
	spread, changed := distributeFactoryOverflow(single, previous, 101, world)
	if !changed || spread.moves["a"] != 103 {
		t.Fatalf("岛屿未承接可由本地矿供料的第一块：%v", spread.moves)
	}
	for _, block := range source.BlockIDs[1:] {
		if spread.moves[block] != 102 || single.moves[block] != 103 {
			t.Fatalf("其余工序或集中后备分工有误：%v / %v", spread.moves, single.moves)
		}
	}
}

// TestOverflowFollowsLocalMaterial 校验原料生产方和后续消费方随制造块逐个归位，保持对应上下游在同一颗星。
func TestOverflowFollowsLocalMaterial(t *testing.T) {
	planets := map[int64]Planet{
		101: {ID: 101, StarID: 1, Buildable: true},
		102: {ID: 102, StarID: 1, Buildable: true, DrySurface: true},
		103: {ID: 103, StarID: 1, Buildable: true, Position: [3]float64{5000}},
		201: {ID: 201, StarID: 2, Buildable: true, DrySurface: true},
	}
	source := spillTestSite(101, "甲", "乙", "冶炼", "制造")
	ore := quant.ItemRate{ItemID: 1001, RatePerMinute: 90}
	iron := quant.ItemRate{ItemID: 1101, RatePerMinute: 90}
	source.Plan.Productions[2].Inputs = []quant.ItemRate{ore}
	source.Plan.Productions[2].Outputs = []quant.ItemRate{iron}
	source.Plan.Productions[3].Inputs = []quant.ItemRate{iron}
	mine := spillTestSite(103)
	mine.Ports = []Port{{ItemID: 1001, Kind: "mining", RatePerMinute: 90}}
	previous := Result{Planets: []PlanetPlan{source, spillTestSite(102), mine}}
	policy := assignment{fullFactories: map[int64]bool{101: true}, moves: map[string]int64{"甲": 101, "乙": 101, "冶炼": 102, "制造": 102}}
	grouped, ok := distributeFactoryOverflow(policy, previous, 101, planets)
	if !ok || grouped.moves["冶炼"] != 103 || grouped.moves["制造"] != 103 {
		t.Fatalf("与本地矿和冶炼的两个连接未保留：%v", grouped.moves)
	}
	if got := factoryDestination(101, map[int64]bool{201: true}, nil, planets, []int64{103, 201}); got != 103 {
		t.Fatalf("同恒星系可施工时优先选择了 %d", got)
	}
}

// TestVerifiedFreightRetainsConstructibleFallback 验证量化换位仍须逐星过施工，失败时保留已成图的原分工。
func TestVerifiedFreightRetainsConstructibleFallback(t *testing.T) {
	a, b := spillTestSite(101, "铁"), spillTestSite(201, "铜")
	a.Ports = []Port{{ItemID: 1002, Kind: "mining", RatePerMinute: 60}}
	b.Ports = []Port{{ItemID: 1001, Kind: "mining", RatePerMinute: 60}}
	for i, site := range []*PlanetPlan{&a, &b} {
		site.Plan.Productions[0] = quant.Production{RecipeID: i + 1, MachineID: 2302, MachineCount: 1,
			Inputs: []quant.ItemRate{{ItemID: 1001 + i, RatePerMinute: 60}}}
	}
	old := Result{Status: "conditional", Planets: []PlanetPlan{a, b}, Routes: []Route{{WarperRatePerMinute: 1}}, Repairs: 3}
	estimate := func(p assignment) Result {
		if p.moves["铁"] != 201 || p.moves["铜"] != 101 {
			t.Fatalf("换位不符：%v", p.moves)
		}
		out := old
		out.Routes = nil
		return out
	}
	for _, fits := range []bool{false, true} {
		got := improveVerifiedFreight(t.Context(), old, assignment{}, estimate, func(next *Result, _ assignment) (int64, error) {
			if fits {
				return 0, nil
			}
			return 201, errors.New("新工区无法落位")
		}, nil)
		if (len(got.Routes) == 0) != fits || (!fits && got.Repairs != 4) {
			t.Fatalf("施工结果 %v 对应的分工或修正次数不符：%+v", fits, got)
		}
	}
}

// TestSwapOverflowUsesBothEnds 同机型同台数的两个待建工区互换后，原星前缀、设备和运输方向都要保持可核对。
func TestSwapOverflowUsesBothEnds(t *testing.T) {
	site := spillTestSite(101, "已落位", "铁", "铜")
	for i := range site.Plan.Productions {
		r := &site.Plan.Productions[i]
		r.RecipeID, r.MachineID, r.MachineCount = i+1, 2302, 32
	}
	site.Plan.Productions[1].Inputs = []quant.ItemRate{{ItemID: 1001, RatePerMinute: 80}}
	site.Plan.Productions[2].Inputs = []quant.ItemRate{{ItemID: 1002, RatePerMinute: 80}}
	moves := map[string]int64{"已落位": 101, "铁": 102, "铜": 103}
	balance := map[int64]map[int]float64{102: {1001: -80, 1002: 80}, 103: {1001: 80, 1002: -80}}
	manufacturing := map[int64]map[int]float64{102: {1001: -80}, 103: {1002: -80}}
	matchOverflowBlocks(&site, 1, moves, balance, manufacturing)
	if moves["已落位"] != 101 || moves["铁"] != 103 || moves["铜"] != 102 ||
		balance[102][1001] != 0 || balance[102][1002] != 0 || balance[103][1001] != 0 || balance[103][1002] != 0 {
		t.Fatalf("未消除两地原料对运：%v，%v", moves, balance)
	}
}

// TestSwapOverflowPreservesLocalUpstream 两颗星已经就地消耗了制造供给时，单看含矿区的净货量不能决定换位。
func TestSwapOverflowPreservesLocalUpstream(t *testing.T) {
	site := spillTestSite(101, "甲", "乙")
	for i := range site.Plan.Productions {
		row := &site.Plan.Productions[i]
		row.RecipeID, row.MachineID, row.MachineCount = i+1, 2302, 32
		row.Inputs = []quant.ItemRate{{ItemID: 1101 + i, RatePerMinute: 80}}
	}
	moves := map[string]int64{"甲": 102, "乙": 103}
	all := map[int64]map[int]float64{102: {1101: -80, 1102: 80}, 103: {1101: 80, 1102: -80}}
	manufacturing := map[int64]map[int]float64{102: {}, 103: {}}
	matchOverflowBlocks(&site, 0, moves, all, manufacturing)
	if moves["甲"] != 102 || moves["乙"] != 103 {
		t.Fatalf("打散了原有本地制造供料：%v", moves)
	}
}

// TestFactorySpillAcrossThreePlanets 职责：将六块制造按每星两块连续装满三颗星；使用：完整规划回归；边界：已放前缀保留，两次失败应完成两次批量迁移。
func TestFactorySpillAcrossThreePlanets(t *testing.T) {
	disabled := false
	req := Request{CenterPlanetID: 101, EnableMining: &disabled,
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60 * 32 * 6}}}}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{1000, 0, 0}},
		{ID: 103, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{2000, 0, 0}},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var failed []int64
	result, err := Plan(ctx, req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if len(site.Plan.Productions) > 2 {
			failed = append(failed, site.PlanetID)
			return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 2, Err: errors.New("剩余制造区不能放下下一模块")}
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !result.Verified || result.Repairs != 2 || len(result.Planets) != 3 || !slices.Equal(failed, []int64{101, 102}) {
		t.Fatalf("未按容量连续填满三颗星：%v，已验证 %t，修复 %d，失败顺序 %v，状态 %s，说明 %v", err, result.Verified, result.Repairs, failed, result.Status, result.Warnings)
	}
	blocks := productionBlockIDs(result.Plan.Productions)
	if len(blocks) != 6 || result.Plan.MachineCount != 192 {
		t.Fatalf("样例制造总量错误：%d 块，%d 台", len(blocks), result.Plan.MachineCount)
	}
	for i, site := range result.Planets {
		if site.PlanetID != int64(101+i) || !slices.Equal(site.BlockIDs, blocks[i*2:i*2+2]) || site.Plan.MachineCount != 64 {
			t.Fatalf("星球 %d 已成功前缀被迁走或后缀丢失：%v，%d 台", site.PlanetID, site.BlockIDs, site.Plan.MachineCount)
		}
	}
}

// TestFactorySpillPrefersUsedAndDoesNotCycle 职责：先复用未满星并允许再次迁移剩余后缀；使用：目标顺序和旧 changed 标记回归；边界：无可用星时结束，不回流至满星。
func TestFactorySpillPrefersUsedAndDoesNotCycle(t *testing.T) {
	planets := map[int64]Planet{
		101: {ID: 101, Buildable: true},
		102: {ID: 102, Buildable: true, Position: [3]float64{10, 0, 0}},
		103: {ID: 103, Buildable: true, Position: [3]float64{100, 0, 0}},
		104: {ID: 104, Buildable: true, Position: [3]float64{50, 0, 0}},
	}
	buildable := []int64{101, 102, 103, 104}
	excluded := resourceSite{planetID: 101, itemID: 1001}
	policy := assignment{center: 101, strategy: 2,
		moves: map[string]int64{"a": 101, "x": 103, "y": 104}, changed: map[string]bool{"b": true, "c": true},
		excludedSources: map[resourceSite]bool{excluded: true}, fullFactories: map[int64]bool{104: true}}
	previous := Result{Planets: []PlanetPlan{spillTestSite(101, "a", "b", "c"), spillTestSite(103, "x"), spillTestSite(104, "y")}}
	rootCause := errors.New("已使用星球剩余空地不足")
	cause := fmt.Errorf("逐星生成：%w", &FactoryPlacementError{FirstUnplaced: 1, Err: rootCause})
	if !errors.Is(cause, rootCause) {
		t.Fatal("容量反馈没有保留底层错误")
	}
	first, ok := repairAssignment(policy, previous, 101, cause, planets, buildable)
	wantFirst := map[string]int64{"a": 101, "b": 103, "c": 103, "x": 103, "y": 104}
	if !ok || !reflect.DeepEqual(first.moves, wantFirst) || !reflect.DeepEqual(first.fullFactories, map[int64]bool{101: true, 104: true}) {
		t.Fatalf("应跳过更近的新星与满星，迁移整个后缀至已有星：%+v，%t", first, ok)
	}
	if first.center != policy.center || first.strategy != policy.strategy || !reflect.DeepEqual(first.changed, policy.changed) || !first.excludedSources[excluded] || len(policy.moves) != 3 || len(policy.fullFactories) != 1 {
		t.Fatal("批量迁移修改了其他策略、资源排除或原请求映射")
	}
	previous.Planets = []PlanetPlan{spillTestSite(101, "a"), spillTestSite(103, "x", "b", "c"), spillTestSite(104, "y")}
	second, ok := repairAssignment(first, previous, 103, &FactoryPlacementError{FirstUnplaced: 2, Err: rootCause}, planets, buildable)
	wantSecond := map[string]int64{"a": 101, "b": 103, "c": 102, "x": 103, "y": 104}
	if !ok || !reflect.DeepEqual(second.moves, wantSecond) || !second.fullFactories[103] || first.fullFactories[103] {
		t.Fatalf("changed 阻止再次迁移、成功前缀改变或原满星集合被污染：%+v，%t", second, ok)
	}
	previous.Planets = []PlanetPlan{spillTestSite(101, "a"), spillTestSite(102, "c"), spillTestSite(103, "x", "b"), spillTestSite(104, "y")}
	last, ok := repairAssignment(second, previous, 102, &FactoryPlacementError{FirstUnplaced: 0, Err: rootCause}, planets, buildable)
	if ok || !reflect.DeepEqual(last, second) {
		t.Fatalf("没有承接星时仍循环搬回满星：%+v，%t", last, ok)
	}
}

// TestSpreadOverflowAcrossActiveSurfaces 用已放下的两块作首轮参考，验证原星前缀、空地优先级和集中后备分工。
func TestSpreadOverflowAcrossActiveSurfaces(t *testing.T) {
	planets := map[int64]Planet{
		101: {ID: 101, Buildable: true},
		102: {ID: 102, Buildable: true, Position: [3]float64{10}},
		103: {ID: 103, Buildable: true, DrySurface: true, Position: [3]float64{100}},
		104: {ID: 104, Buildable: true, DrySurface: true, Position: [3]float64{20}},
	}
	previous := Result{Planets: []PlanetPlan{spillTestSite(101, "a", "b", "c", "d", "e", "f"), spillTestSite(102), spillTestSite(103)}}
	single, ok := repairAssignment(assignment{center: 101}, previous, 101,
		&FactoryPlacementError{FirstUnplaced: 2, Err: errors.New("本星放下两块制造")}, planets, []int64{101, 102, 103, 104})
	if !ok || single.moves["c"] != 103 || single.moves["f"] != 103 {
		t.Fatalf("原集中方案应优先已纳入建设的无液面星球：%+v", single)
	}
	spread, changed := distributeFactoryOverflow(single, previous, 101, planets)
	want := map[string]int64{"a": 101, "b": 101, "c": 103, "d": 103, "e": 102, "f": 102}
	if !changed || !reflect.DeepEqual(spread.moves, want) || single.moves["f"] != 103 || spread.fullFactories[102] || spread.fullFactories[103] {
		t.Fatalf("首轮分工应保留原星前缀、已用星和可继续尝试的集中方案：%+v", spread)
	}
}

// TestDeferredRegionsSpillBeforeFullSearch 两处不可合并的交付已要求跨星；邻星能接住后缀时，原星无需为整批未放下的块重算所有纬区。
func TestDeferredRegionsSpillBeforeFullSearch(t *testing.T) {
	disabled := false
	req := Request{CenterPlanetID: 101, EnableMining: &disabled, TargetPlanetIDs: []int64{101, 102},
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 3840}, {ItemID: 1101, RatePerMinute: 3840}}}}
	world := World{Planets: []Planet{
		{ID: 101, Buildable: true, StarID: 1, ResourcesKnown: true},
		{ID: 102, Buildable: true, StarID: 1, ResourcesKnown: true, Position: [3]float64{10000}},
	}}
	var original int64
	var blocks []string
	result, err := Plan(t.Context(), req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if original == 0 && len(site.BlockIDs) > 1 {
			original, blocks = site.PlanetID, slices.Clone(site.BlockIDs)
			if !site.CanRelocate {
				t.Fatal("两处固定交付的多星方案没有先使用可迁移搜索")
			}
			return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 1, DeferredRegions: true, Err: errors.New("其余制造块未在本轮选区落位")}
		}
		if site.PlanetID == original && slices.Equal(site.BlockIDs, blocks) && !site.CanRelocate {
			t.Fatal("邻星仍有充足空间，提前开始了原星整批制造的完整纬区搜索")
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !result.Verified || original == 0 || len(result.Planets) != 2 {
		t.Fatalf("未核验通过可承接的邻星分工：错误 %v，状态 %s，星球 %d", err, result.Status, len(result.Planets))
	}
	for _, site := range result.Planets {
		want := blocks[1:]
		if site.PlanetID == original {
			want = blocks[:1]
		}
		if !slices.Equal(site.BlockIDs, want) {
			t.Fatalf("星球 %d 的制造块 %v，期望 %v", site.PlanetID, site.BlockIDs, want)
		}
	}
}

// TestRepowerPreservesFactoryWhenNeighborCannotBuild 核对原星补足供电后仍能保留全部制造块，并向两处交付。
func TestRepowerPreservesFactoryWhenNeighborCannotBuild(t *testing.T) {
	disabled := false
	req := Request{CenterPlanetID: 101, EnableMining: &disabled, EnablePower: true, TargetPlanetIDs: []int64{101, 102},
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 3840}, {ItemID: 1101, RatePerMinute: 3840}}}}
	world := World{Planets: []Planet{
		{ID: 101, Buildable: true, StarID: 1, ResourcesKnown: true},
		{ID: 102, Buildable: true, StarID: 1, ResourcesKnown: true, Position: [3]float64{10000}},
	}}
	var original int64
	var blocks []string
	var minimum float64
	var repowered bool
	result, err := Plan(t.Context(), req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		actual := PlanetActual{Ports: site.Ports, Power: site.Power}
		if len(site.BlockIDs) == 0 {
			return actual, nil
		}
		if original == 0 {
			original, blocks = site.PlanetID, slices.Clone(site.BlockIDs)
			minimum = site.Power.PowerMW + 5
		}
		if site.PlanetID != original {
			return PlanetActual{}, errors.New("邻星无法容纳制造")
		}
		if !slices.Equal(site.BlockIDs, blocks) {
			return actual, nil
		}
		if site.Power.PowerMW < minimum {
			return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 1, DeferredRegions: true, MinimumPowerMW: minimum, Err: errors.New("原星有待补电的完整候选")}
		}
		repowered = true
		return actual, nil
	}, nil)
	if err != nil || !result.Verified || !repowered || len(result.Planets) != 2 {
		t.Fatalf("原星补电后的完整分工未恢复：%v，已核验 %t，补电 %t，星数 %d", err, result.Verified, repowered, len(result.Planets))
	}
}

// TestDeferredRegionsRestoreOriginalAssignment 当其他施工星不能接收制造后缀，仍在原分工上启用完整纬区并核对两处交付。
func TestDeferredRegionsRestoreOriginalAssignment(t *testing.T) {
	disabled := false
	req := Request{CenterPlanetID: 101, EnableMining: &disabled, TargetPlanetIDs: []int64{101, 102},
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 3840}, {ItemID: 1101, RatePerMinute: 3840}}}}
	world := World{Planets: []Planet{
		{ID: 101, Buildable: true, StarID: 1, ResourcesKnown: true},
		{ID: 102, Buildable: true, StarID: 1, ResourcesKnown: true, Position: [3]float64{10000}},
	}}
	var original int64
	var count int
	var triedCenters, retried bool
	result, err := Plan(context.Background(), req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if len(site.BlockIDs) == 0 {
			return PlanetActual{Ports: site.Ports}, nil
		}
		if original == 0 {
			if !site.CanRelocate || len(site.BlockIDs) < 3 {
				t.Fatalf("样例未进入已有两处交付星的多块布局：%+v", site.BlockIDs)
			}
			original, count = site.PlanetID, len(site.BlockIDs)
			return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 1, DeferredRegions: true, Err: errors.New("低纬已满")}
		}
		if site.PlanetID == original && len(site.BlockIDs) == count && !site.CanRelocate {
			if site.CenterRegionsOnly {
				triedCenters = true
				return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 2, DeferredRegions: true, Err: errors.New("中心选区的第三块没有落点")}
			}
			retried = true
			return PlanetActual{Ports: site.Ports}, nil
		}
		return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: -1, Err: errors.New("分摊后不能安放")}
	}, nil)
	if err != nil || !triedCenters || !retried || !result.Verified || len(result.Planets) != 2 {
		t.Fatalf("未以原分工逐步恢复全纬区搜索：中心 %t，全区 %t，%s %v", triedCenters, retried, result.Status, err)
	}
}

// TestNewFactoryBlocksAvoidFullPlanets 职责：让能源重新量化新增的块避开已满星；使用：稳定模块身份后的增量回归；边界：明确固定的成功前缀继续保留。
func TestNewFactoryBlocksAvoidFullPlanets(t *testing.T) {
	disabled := false
	req, profile, err := normalize(Request{CenterPlanetID: 101, EnableMining: &disabled,
		Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60 * 32 * 3}}}})
	if err != nil {
		t.Fatal(err)
	}
	base, err := quant.Solve(context.Background(), req.Quant)
	if err != nil {
		t.Fatal(err)
	}
	base, err = SplitProductions(context.Background(), base, 32)
	if err != nil {
		t.Fatal(err)
	}
	blocks := productionBlockIDs(base.Productions)
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{1000, 0, 0}},
		{ID: 103, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{10, 0, 0}},
	}}
	planets, ids, buildable, err := validateWorld(req, world)
	if err != nil {
		t.Fatal(err)
	}
	policy := assignment{center: 101, moves: map[string]int64{blocks[0]: 101, blocks[1]: 102}, fullFactories: map[int64]bool{101: true}}
	result := evaluate(context.Background(), req, base, planets, ids, buildable, profile, policy, nil)
	if result.Status != "conditional" || len(result.Planets) != 2 || !slices.Equal(result.Planets[0].BlockIDs, blocks[:1]) || !slices.Equal(result.Planets[1].BlockIDs, blocks[1:]) {
		t.Fatalf("新增块被塞回满星或固定前缀被搬走：状态 %s，星球 %+v", result.Status, result.Planets)
	}
}

// spillTestSite 职责：建立身份与生产下标一一对应的制造片段；使用：修复策略隔离测试；边界：不用于实体布局。
func spillTestSite(id int64, blocks ...string) PlanetPlan {
	site := PlanetPlan{PlanetID: id, BlockIDs: blocks}
	for range blocks {
		site.Plan.Productions = append(site.Plan.Productions, quant.Production{MachineCount: 1})
	}
	return site
}
