package saveanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sync"
	"testing"

	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestResolvedResourceCache 职责：核对单球精算缓存；使用：覆盖并发读取、取消和换档；边界：无法确定的成功结果保留未知来源。
func TestResolvedResourceCache(t *testing.T) {
	world := WorldView{Planets: []PlanetLink{{PlanetID: 101, StarID: 1}, {PlanetID: 102, StarID: 1}}}
	s := &Snapshot{
		worldView: world,
		surfaces: map[int64]*planetSurface{101: {veins: []PlanetVein{
			{ID: 1, VeinType: 1, ItemID: 1001, Amount: 200, Position: galaxy.Vector{X: 200}},
		}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ResolvePlanetResources(ctx, 102); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消应返回错误：%v", err)
	}
	if _, found := s.CachedPlanetResources(102); found {
		t.Fatal("已取消的精算写入了缓存")
	}
	if _, err := s.ResolvePlanetResources(context.Background(), 103); err == nil {
		t.Fatal("不存在的星球应返回错误")
	}
	if _, found := s.CachedPlanetResources(103); found {
		t.Fatal("失败的精算写入了缓存")
	}
	if _, err := s.ResolvePlanetResources(context.Background(), 102); err != nil {
		t.Fatal(err)
	}
	if summary, found := s.CachedPlanetResources(102); !found || summary.Source != "unknown" {
		t.Fatalf("未知结果的精算记录不正确：%+v，命中=%t", summary, found)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			summary, err := s.ResolvePlanetResources(context.Background(), 101)
			if err != nil || len(summary.Resources) != 1 || summary.Resources[0].AvailableAmount == nil || *summary.Resources[0].AvailableAmount != 200 {
				t.Errorf("同一快照未返回独立的可采量：%+v %v", summary, err)
				return
			}
			*summary.Resources[0].AvailableAmount = 0
		}()
	}
	workers.Wait()
	if summary, found := s.CachedPlanetResources(101); !found || len(summary.Resources) != 1 ||
		summary.Resources[0].AvailableAmount == nil || *summary.Resources[0].AvailableAmount != 200 {
		t.Fatalf("查询结果修改波及缓存：%+v，命中=%t", summary, found)
	}
	other := &Snapshot{worldView: world}
	if _, found := other.CachedPlanetResources(101); found {
		t.Fatal("新快照复用了旧存档的精算结果")
	}
}

// TestAvailableDeposits 职责：验证地质总量和可选容量保持独立；使用：覆盖埋矿与多矿机重叠；边界：普通地形高差保留。
func TestAvailableDeposits(t *testing.T) {
	planet := PlanetLink{PlanetID: 101, StarID: 1, HasFactory: true}
	vein := func(id, kind, item, amount int64, radius float64) PlanetVein {
		return PlanetVein{ID: id, VeinType: kind, ItemID: item, Amount: amount, Position: galaxy.Vector{X: radius}}
	}
	s := &Snapshot{
		surfaces: map[int64]*planetSurface{101: {veins: []PlanetVein{
			vein(1, 1, 1001, 100, 196), vein(2, 1, 1001, 200, 140), vein(3, 1, 1001, 300, 150), vein(4, 1, 1001, 400, 200),
			vein(5, 7, 1007, 10000, 198), vein(6, 7, 1007, 20000, 150), vein(7, 7, 1007, 30000, 140),
		}}},
		lines: map[int64]*factoryLine{101: {devices: []deviceRecord{
			{DeviceState: DeviceState{Kind: "miner", EntityID: 12, Veins: []int64{2, 4, 7}}},
			{DeviceState: DeviceState{Kind: "miner", EntityID: 24, Veins: []int64{2}}},
		}}},
	}
	summary, err := s.planetResourceSummary(context.Background(), planet, false, galaxy.HeightGrid{})
	if err != nil || len(summary.Resources) != 2 {
		t.Fatalf("概要缺少资源：%+v %v", summary, err)
	}
	check := func(v PlanetResource, count, occupied, available, amount, occupiedAmount, availableAmount int64) {
		t.Helper()
		for _, pair := range []struct {
			got  *int64
			want int64
		}{
			{v.Count, count}, {v.OccupiedCount, occupied}, {v.AvailableCount, available},
			{v.Amount, amount}, {v.OccupiedAmount, occupiedAmount}, {v.AvailableAmount, availableAmount},
		} {
			if pair.got == nil || *pair.got != pair.want {
				t.Fatalf("物品 %d 的数量字段不一致：实际 %v，预期 %d", v.ItemID, pair.got, pair.want)
			}
		}
	}
	check(summary.Resources[0], 4, 2, 1, 1000, 600, 100)
	oil := summary.Resources[1]
	check(oil, 3, 1, 1, 60000, 30000, 10000)
	if oil.Rate == nil || oil.AvailableRate == nil || oil.OccupiedRate == nil ||
		math.Abs(*oil.Rate-2.4) > 1e-6 || math.Abs(*oil.AvailableRate-.4) > 1e-6 || math.Abs(*oil.OccupiedRate-1.2) > 1e-6 {
		t.Fatalf("原油速率概要错误：%+v", oil)
	}
}

// TestRelayScanAvailability 核对已扫描目录只公布地质总量，并在精化命中后扣除埋矿。
func TestRelayScanAvailability(t *testing.T) {
	planet := PlanetLink{PlanetID: 401, StarID: 4}
	s := &Snapshot{
		virtualRelayZones: map[int64][]RelayZone{401: {{Radius: 40, Depth: 60}}},
		scanned: map[int64]*scannedPlanet{401: {groups: []PlanetVeinGroup{
			{VeinType: 1, Count: 2, Amount: 300},
		}}},
	}
	before, err := s.planetResourceSummary(context.Background(), planet, false, galaxy.HeightGrid{})
	if err != nil || before.Source != "candidate" || len(before.Resources) != 1 || before.Resources[0].AvailableCount != nil || *before.Resources[0].Count != 2 {
		t.Fatalf("扫描目录地质与可采量状态不符：%+v %v", before, err)
	}
	s.generatedVeins.Store(planet.PlanetID, generatedPlanetVeins{ok: true, veins: []PlanetVein{
		{ItemID: 1001, Amount: 100, PredictedBurial: true}, {ItemID: 1001, Amount: 200},
	}})
	after, err := s.planetResourceSummary(context.Background(), planet, true, galaxy.HeightGrid{})
	if err != nil || after.Source != "saved" || len(after.Resources) != 1 {
		t.Fatalf("扫描精化状态不符：%+v %v", after, err)
	}
	row := after.Resources[0]
	if *row.Count != 2 || *row.AvailableCount != 1 || *row.Amount != 300 || *row.AvailableAmount != 200 {
		t.Fatalf("精化应保持地质量并扣除埋矿：%+v", row)
	}
}

// TestPendingFactoryBurialAvailability 验证待本地化实体按实际厂区矿点计算，和占用重合时只扣一次。
func TestPendingFactoryBurialAvailability(t *testing.T) {
	planet := PlanetLink{PlanetID: 6301, HasFactory: true}
	s := &Snapshot{
		groundBaseZones: map[int64][]RelayZone{6301: {{Realized: true, Radius: 40, Depth: 60, Position: galaxy.Vector{X: 200.2}}}},
		surfaces: map[int64]*planetSurface{6301: {veins: []PlanetVein{
			{ID: 1, ItemID: 1001, VeinType: 1, Amount: 100, Position: galaxy.Vector{X: 200}},
			{ID: 2, ItemID: 1001, VeinType: 1, Amount: 200, Position: galaxy.Vector{X: 199, Y: 6}},
			{ID: 3, ItemID: 1001, VeinType: 1, Amount: 300, Position: galaxy.Vector{Z: 200}},
		}}},
		lines: map[int64]*factoryLine{6301: {devices: []deviceRecord{
			{DeviceState: DeviceState{Kind: "miner", EntityID: 12, Veins: []int64{1}}},
		}}},
	}
	candidate, err := s.planetResourceSummary(context.Background(), planet, false, galaxy.HeightGrid{})
	if err != nil || candidate.Source != "candidate" || candidate.Resources[0].AvailableCount != nil {
		t.Fatalf("待本地化厂区目录未等待精化：%+v %v", candidate, err)
	}
	grid := galaxy.HeightGrid{Precision: 64, Heights: []uint16{20000}}
	detail, err := s.planetResourceSummary(context.Background(), planet, true, grid)
	if err != nil || detail.Source != "saved" || len(detail.Resources) != 1 {
		t.Fatalf("待本地化厂区未恢复实存供给：%+v %v", detail, err)
	}
	ore := detail.Resources[0]
	if *ore.Count != 3 || *ore.OccupiedCount != 1 || *ore.AvailableCount != 1 || *ore.AvailableAmount != 300 {
		t.Fatalf("深埋和占用交集扣减不符：%+v", ore)
	}
}

// TestRelayBurialRadius 覆盖三维近邻与仅避让的已落地基地。
func TestRelayBurialRadius(t *testing.T) {
	grid := galaxy.HeightGrid{Precision: 64, Heights: []uint16{20000}}
	zone := RelayZone{Position: galaxy.Vector{X: 200.2}, Radius: 40, Depth: 60}
	onSurface := func(angle float64) galaxy.Vector {
		return galaxy.Vector{X: 200 * math.Cos(angle), Z: 200 * math.Sin(angle)}
	}
	angleAtDistance := func(distance float64) float64 {
		const surfaceRadius = 200 - 0.13
		const centerRadius = 200.2
		return math.Acos((surfaceRadius*surfaceRadius + centerRadius*centerRadius - distance*distance) / (2 * surfaceRadius * centerRadius))
	}
	if !relayBurial(onSurface(angleAtDistance(40.5)), grid, nil, []RelayZone{zone}) ||
		relayBurial(onSurface(angleAtDistance(41.1)), grid, nil, []RelayZone{zone}) {
		t.Fatal("40.5 米边缘应从供给扣除，41.1 米处仍可纳入供给")
	}
	zone.Depth = 0
	if relayBurial(onSurface(0), grid, nil, []RelayZone{zone}) {
		t.Fatal("已实体化基地的占地范围不能重复扣减矿量")
	}
}

// TestSavedPlanetResources 职责：验证真实主档空间数据和全量覆盖；使用：通过环境变量选择存档；边界：样本不随代码提交。
func TestSavedPlanetResources(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定真实主档")
	}
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	world, catalog := s.World(), s.PlanetResourceCatalog()
	for _, planet := range world.Planets {
		if s.themes[planet.PlanetID].ID != 1 {
			continue
		}
		detail, err := s.PlanetResourceDetail(planet.PlanetID)
		if err != nil || detail.VeinSource != "saved" || len(detail.Veins) == 0 {
			t.Fatalf("出生星球 %d 缺少厂区矿点：%+v", planet.PlanetID, err)
		}
		t.Logf("出生星球 %d 使用存档矿点，共 %d 个", planet.PlanetID, len(detail.Veins))
		surface := s.surfaces[planet.PlanetID]
		s.surfaces[planet.PlanetID] = &planetSurface{}
		depleted, err := s.PlanetResourceDetail(planet.PlanetID)
		if err != nil || depleted.VeinSource != "saved" || len(depleted.Veins) != 0 {
			t.Fatalf("出生星球已清空的矿点被错误补充：%v", err)
		}
		delete(s.surfaces, planet.PlanetID)
		missing, err := s.PlanetResourceDetail(planet.PlanetID)
		s.surfaces[planet.PlanetID] = surface
		if err != nil || missing.VeinSource != "unknown" || len(missing.Veins) != 0 {
			t.Fatalf("出生星球缺少厂区矿点时返回了未验证坐标：%v", err)
		}
		break
	}
	var candidates int
	for _, planet := range catalog.Planets {
		if planet.Source != "candidate" {
			continue
		}
		candidates++
		if _, ok := s.generatedVeins.Load(planet.PlanetID); ok {
			t.Fatal("资源目录触发矿点生成")
		}
		for _, entry := range planet.Resources {
			if entry.Source == "candidate" && (entry.Amount != nil || entry.Count != nil || entry.Rate != nil) {
				t.Fatal("候选目录填入了未验证的资源数值")
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := s.ResolvePlanetResources(ctx, planet.PlanetID); !errors.Is(err, context.Canceled) {
			t.Fatalf("精化未响应取消：%v", err)
		}
		if _, err := s.PlanetResourceDetailContext(ctx, planet.PlanetID); !errors.Is(err, context.Canceled) {
			t.Fatalf("详情未响应取消：%v", err)
		}
	}
	if world.Tick == 7174748 {
		for _, planetID := range []int64{301, 302, 401} {
			detail, err := s.PlanetResourceDetail(planetID)
			if err != nil {
				t.Fatal(err)
			}
			deposits := 0
			for _, vein := range detail.Veins {
				if vein.PredictedBurial {
					deposits++
				}
			}
			if planetID == 401 && (len(detail.RelayZones) != 5 || deposits == 0 || deposits >= len(detail.Veins)) {
				t.Fatalf("401 应保留五个虚拟基地且只影响部分矿点：%d/%d/%d", len(detail.RelayZones), deposits, len(detail.Veins))
			}
			if planetID != 401 && (len(detail.RelayZones) != 0 || deposits != 0) {
				t.Fatalf("%d 无虚拟基地，矿点不应被扣减", planetID)
			}
			t.Logf("%d 虚拟基地 %d，初始矿点 %d，预计落地埋矿 %d，概要来源 %s", planetID, len(detail.RelayZones), len(detail.Veins), deposits, detail.Planet.Source)
		}
	}
	resources := s.PlanetResources()
	if world.SHA256 != resources.SHA256 || world.Tick != resources.Tick || len(world.Planets) != len(resources.Planets) {
		t.Fatal("资源快照与星系快照不一致")
	}
	var gas, unknown, veins, occupied, buildings, prebuilds int
	verifiedUnbuilt := map[int]bool{}
	for i, planet := range resources.Planets {
		if planet.PlanetID != world.Planets[i].PlanetID {
			t.Fatal("资源星球顺序与星系清单不一致")
		}
		if planet.Gas {
			gas++
		}
		if planet.Source == "unknown" {
			unknown++
		}
		if !world.Planets[i].HasFactory {
			algorithm := s.themes[planet.PlanetID].Algorithm
			if planet.Source == "saved" && !planet.Gas && !verifiedUnbuilt[algorithm] {
				if s.checkUnbuiltSpatial(t, planet) {
					verifiedUnbuilt[algorithm] = true
				}
			}
			continue
		}
		detail, err := s.PlanetResourceDetail(planet.PlanetID)
		if err != nil {
			t.Fatal(err)
		}
		if hasPendingBurial(detail.RelayZones) {
			var predicted, realized int
			for _, vein := range detail.Veins {
				if vein.PredictedBurial {
					predicted++
				}
			}
			for _, zone := range detail.RelayZones {
				if zone.Realized && zone.Depth > 0 {
					realized++
				}
			}
			t.Logf("厂区 %d 待本地化的实体基地 %d，另计可能埋矿 %d", planet.PlanetID, realized, predicted)
		}
		if _, err := json.Marshal(detail); err != nil {
			t.Fatalf("星球 %d 的空间数据无法编码：%v", planet.PlanetID, err)
		}
		if planet.PlanetID == 3402 {
			for _, resource := range planet.Resources {
				if resource.Kind != "oil" || resource.AvailableCount == nil {
					continue
				}
				deep, deepOccupied := 0, 0
				for _, node := range detail.Veins {
					if node.ItemID == resource.ItemID && node.IsDeepBuried() {
						deep++
						if len(node.MinerEntityIDs) != 0 {
							deepOccupied++
						}
					}
				}
				if *resource.AvailableCount != *resource.Count-*resource.OccupiedCount-int64(deep-deepOccupied) {
					t.Fatal("实档油井可选数未正确扣除深埋与占用交集")
				}
				t.Logf("3402 原油：总井 %d / 占用 %d / 可选 %d / 深埋 %d / 深埋且占用 %d，基础油速总 %.6f / 占用 %.6f / 可选 %.6f 件/秒",
					*resource.Count, *resource.OccupiedCount, *resource.AvailableCount, deep, deepOccupied, *resource.Rate, *resource.OccupiedRate, *resource.AvailableRate)
			}
		}
		veins += len(detail.Veins)
		buildings += len(detail.Buildings)
		for _, vein := range detail.Veins {
			if vein.ID <= 0 || vein.ItemID <= 0 || vein.GroupID <= 0 || vein.Position.X == 0 && vein.Position.Y == 0 && vein.Position.Z == 0 {
				t.Errorf("星球 %d 矿点位置或标识无效：%+v", planet.PlanetID, vein)
				break
			}
			if len(vein.MinerEntityIDs) > 0 {
				occupied++
			}
		}
		for _, building := range detail.Buildings {
			if building.Prebuild {
				prebuilds++
			}
		}
	}
	t.Logf("星球 %d，气态 %d，目录候选 %d，精查后概要未知 %d，矿点 %d，已占矿点 %d，建筑和预建造 %d，其中预建造 %d", len(resources.Planets), gas, candidates, unknown, veins, occupied, buildings, prebuilds)
	t.Logf("同存档矿群与生成矿点数量、矿量一致的未建厂地形算法 %d 种", len(verifiedUnbuilt))
}

// checkUnbuiltSpatial 职责：用未建厂星球扫描概要交叉复核矿点；使用：真实档对照；边界：只核对保留了扫描群的星球。
func (s *Snapshot) checkUnbuiltSpatial(t *testing.T, planet PlanetResourceSummary) bool {
	t.Helper()
	if len(s.scanned[planet.PlanetID].groups) == 0 {
		return false
	}
	detail, err := s.PlanetResourceDetail(planet.PlanetID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.VeinSource != "generated" {
		return false
	}
	type total struct{ count, amount int64 }
	byItem := make(map[int64]total)
	for _, vein := range detail.Veins {
		sum := byItem[vein.ItemID]
		sum.count++
		sum.amount += vein.Amount
		byItem[vein.ItemID] = sum
	}
	for _, row := range planet.Resources {
		if row.Count == nil || row.Amount == nil {
			continue
		}
		if got := byItem[row.ItemID]; got.count != *row.Count || got.amount != *row.Amount {
			t.Errorf("未建厂星球 %d 物品 %d 的生成矿点与存档矿群不符：%+v/%+v", planet.PlanetID, row.ItemID, got, total{*row.Count, *row.Amount})
			return false
		}
		delete(byItem, row.ItemID)
	}
	if len(byItem) != 0 {
		t.Errorf("未建厂星球 %d 额外生成了未扫描矿种：%v", planet.PlanetID, byItem)
		return false
	}
	return true
}
