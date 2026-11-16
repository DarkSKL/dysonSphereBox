package build

// 文件职责：对实际生产入口执行旧实现差分、全扫描及含建索引的性能对照。
// 使用：公共世界索引重构临时验收；边界：仅离线算法实测，保留到主线统一清理。

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/transport"
)

// spatialTestRecord 职责：创建无隐式连接的有效测试记录；使用：生产检查样本；边界：坐标是蓝图细格，世界转换仍走生产入口。
func spatialTestRecord(index int, item int16, x, y, z float64) blueprint.Building {
	model := int16(0)
	switch item {
	case 2003:
		model = 37
	case 2201:
		model = 44
	case 2303:
		model = 65
	}
	return blueprint.Building{Index: int32(index), ItemID: item, ModelIndex: model, InputObjIdx: -1, OutputObjIdx: -1, LocalOffset: blueprint.Point{X: float32(x), Y: float32(y), Z: float32(z)}}
}

// spatialTestError 职责：固定可比较的错误文本；使用：精确核对首冲突与失败分支；边界：不把 nil 当错误。
func spatialTestError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestSpatialProductionSpacing 职责：对塔距和电力距离执行首冲突差分及全扫描；使用：固定种子、阈值边界和取消样本；边界：全扫描只比较是否冲突，顺序由冻结实现核对。
func TestSpatialProductionSpacing(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(6131))
	for _, station := range []bool{false, true} {
		for sample := 0; sample < 120; sample++ {
			records, points := make([]blueprint.Building, 96), make([]vector, 96)
			for i := range records {
				item := int16(2201 + rng.Intn(13))
				if station {
					item = int16(2103 + rng.Intn(2))
				}
				if i%9 == 0 {
					item = 2003
				}
				records[i] = spatialTestRecord(i, item, 0, 0, 0)
				scale := float64(30 + sample*4)
				points[i] = vector{(rng.Float64() - .5) * scale, (rng.Float64() - .5) * scale, (rng.Float64() - .5) * scale}
			}
			gotA, gotB, err := surfacePowerNodeConflict(ctx, records, points)
			oldA, oldB, oldErr := spatialLegacySurfacePowerNodeConflict(ctx, records, points)
			if station {
				gotA, gotB, err = surfaceStationConflict(ctx, records, points)
				oldA, oldB, oldErr = spatialLegacySurfaceStationConflict(ctx, records, points)
			}
			if gotA != oldA || gotB != oldB || spatialTestError(err) != spatialTestError(oldErr) {
				t.Fatalf("首冲突变化 station=%t sample=%d: %d/%d vs %d/%d: %v/%v", station, sample, gotA, gotB, oldA, oldB, err, oldErr)
			}
			brute := false
			for i := range records {
				for j := 0; j < i; j++ {
					valid := surfaceIsPowerNode(records[i].ItemID) && surfaceIsPowerNode(records[j].ItemID)
					limit := surfacePowerDistanceLimit(records[i].ItemID, records[j].ItemID)
					if station {
						valid = surfaceIsStation(records[i].ItemID) && surfaceIsStation(records[j].ItemID)
						limit = surfaceStationDistanceLimit(records[i].ItemID, records[j].ItemID)
					}
					brute = brute || valid && surfacePowerDistanceSquared(points[i], points[j]) < limit
				}
			}
			if (gotA >= 0) != brute {
				t.Fatalf("全扫描不符 station=%t sample=%d", station, sample)
			}
		}
	}
	for _, pair := range []struct {
		item     int16
		distance float64
	}{{2201, 3.5}, {2203, 10.5}, {2213, 12}, {2103, 15}, {2104, 29}} {
		records := []blueprint.Building{spatialTestRecord(0, pair.item, 0, 0, 0), spatialTestRecord(1, pair.item, 0, 0, 0)}
		for _, delta := range []float64{-1e-4, 0, 1e-4} {
			points := []vector{{-29, -12, -8}, {-29 + pair.distance + delta, -12, -8}}
			check := surfacePowerNodeConflict
			if pair.item < 2200 {
				check = surfaceStationConflict
			}
			a, _, err := check(ctx, records, points)
			if err != nil || (a >= 0) != (delta < 0) {
				t.Fatalf("阈值边界 item=%d delta=%g: %d %v", pair.item, delta, a, err)
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, check := range []func(context.Context, []blueprint.Building, []vector) (int, int, error){surfacePowerNodeConflict, surfaceStationConflict} {
		if _, _, err := check(cancelled, []blueprint.Building{{ItemID: 2201}}, []vector{{}}); err == nil {
			t.Fatal("取消未返回错误")
		}
		if _, _, err := check(ctx, []blueprint.Building{{}}, nil); err == nil {
			t.Fatal("长度不符未返回错误")
		}
	}
}

// spatialTestBelts 职责：生成完整多排连接带和可控建筑；使用：实际交叉与建筑占用检查；边界：规模按行数控制，默认无碰撞。
func spatialTestBelts(rows, width int, buildings bool) []blueprint.Building {
	records := make([]blueprint.Building, 0, rows*width+rows)
	for row := 0; row < rows; row++ {
		for x := 0; x < width; x++ {
			r := spatialTestRecord(len(records), 2003, float64(x-width/2), float64(row*5-rows*2), 0)
			if x+1 < width {
				r.OutputObjIdx = r.Index + 1
			}
			records = append(records, r)
		}
	}
	if buildings {
		for row := 0; row < rows; row++ {
			records = append(records, spatialTestRecord(len(records), 2303, float64(row%width-width/2), float64(row*5-rows*2)+2.4, 0))
		}
	}
	return records
}

// spatialTestPower 职责：生成覆盖完整的多网络电塔与消费者；使用：覆盖和最终裁剪；边界：固定格单位输入，世界距离由正式转换决定。
func spatialTestPower(groups, width int) []blueprint.Building {
	records := make([]blueprint.Building, 0, groups*width*2)
	for group := 0; group < groups; group++ {
		for x := 0; x < width; x++ {
			px, py := float64(x*4-width*2), float64(group*50-groups*25)
			records = append(records, spatialTestRecord(len(records), 2201, px, py, 0))
			consumer := spatialTestRecord(len(records), 2303, px, py+1, 0)
			consumer.RecipeID = 1
			records = append(records, consumer)
		}
	}
	return records
}

// TestSpatialProductionValidators 职责：验证带、建筑占用、供电及裁剪实际入口与旧实现一致；使用：有效/冲突/无覆盖/研究站链样本；边界：检验错误文本和裁剪顺序，不替代游戏验收。
func TestSpatialProductionValidators(t *testing.T) {
	ctx, encoding := context.Background(), newPlanarRuleEncoding()
	rng := rand.New(rand.NewSource(6141))
	for sample := 0; sample < 80; sample++ {
		records := spatialTestBelts(6, 20, true)
		if sample > 0 {
			for i := range records {
				records[i].LocalOffset.X += float32(rng.Float64()*2 - 1)
				records[i].LocalOffset.Y += float32(rng.Float64()*4 - 2)
			}
		}
		if got, want := spatialTestError(encoding.validateBeltCrossings(ctx, records)), spatialTestError(encoding.spatialLegacyValidateBeltCrossings(ctx, records)); got != want {
			t.Fatalf("带交叉 sample=%d: %s vs %s", sample, got, want)
		}
		if got, want := spatialTestError(encoding.validateBeltBuildingOccupancy(ctx, records)), spatialTestError(encoding.spatialLegacyValidateBeltBuildingOccupancy(ctx, records)); got != want {
			t.Fatalf("带建筑 sample=%d: %s vs %s", sample, got, want)
		}
	}
	for sample := 0; sample < 40; sample++ {
		records := spatialTestPower(3, 24)
		if sample%3 == 1 {
			records[len(records)-1].LocalOffset.X += 100
		}
		if sample%3 == 2 {
			for i := range records {
				records[i].LocalOffset.X += float32(rng.Float64() - .5)
			}
		}
		gotN, gotErr := validateFinalPowerCoverage(ctx, encoding, records)
		oldN, oldErr := spatialLegacyValidateFinalPowerCoverage(ctx, encoding, records)
		if gotN != oldN || spatialTestError(gotErr) != spatialTestError(oldErr) {
			t.Fatalf("供电覆盖 sample=%d: %d/%v vs %d/%v", sample, gotN, gotErr, oldN, oldErr)
		}
		got, err := planFinalPowerPrune(ctx, encoding, records)
		want, oldErr := spatialLegacyPlanFinalPowerPrune(ctx, encoding, records)
		if spatialTestError(err) != spatialTestError(oldErr) || !reflect.DeepEqual(got.KeptPoleIndices, want.KeptPoleIndices) || !reflect.DeepEqual(got.RemovedPoleIndices, want.RemovedPoleIndices) || got.NetworkCount != want.NetworkCount {
			t.Fatalf("裁剪 sample=%d: %+v/%v vs %+v/%v", sample, got, err, want, oldErr)
		}
	}
}

// TestSpatialPowerBruteForce 职责：用独立全扫描电网图检验覆盖和分量数；使用：生产覆盖入口验证；边界：样本无研究站链，链行为由旧实现差分另验。
func TestSpatialPowerBruteForce(t *testing.T) {
	ctx, encoding := context.Background(), newPlanarRuleEncoding()
	cover, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		t.Fatal(err)
	}
	connect, err := transport.TeslaPowerConnectDistance()
	if err != nil {
		t.Fatal(err)
	}
	for sample := 0; sample < 8; sample++ {
		records := spatialTestPower(1+sample, 12)
		positions, err := encoding.surfaceWorldPositions(records)
		if err != nil {
			t.Fatal(err)
		}
		visited, components := make([]bool, len(records)), 0
		for i, record := range records {
			if record.ItemID != 2201 || visited[i] {
				continue
			}
			components++
			visited[i] = true
			queue := []int{i}
			for head := 0; head < len(queue); head++ {
				for j, other := range records {
					if other.ItemID == 2201 && !visited[j] && float64(surfacePowerDistanceSquared(positions[queue[head]], positions[j])) <= connect*connect {
						visited[j] = true
						queue = append(queue, j)
					}
				}
			}
		}
		for i, record := range records {
			if !finalPowerConsumer(record) {
				continue
			}
			covered := false
			for j, pole := range records {
				covered = covered || pole.ItemID == 2201 && float64(surfacePowerDistanceSquared(positions[i], positions[j])) <= cover*cover
			}
			if !covered {
				t.Fatal("样本应有覆盖")
			}
		}
		got, err := validateFinalPowerCoverage(ctx, encoding, records)
		if err != nil || got != components {
			t.Fatalf("电网全扫描不符 %d/%v vs %d", got, err, components)
		}
	}
}

// BenchmarkSpatialProduction 职责：对比含建索引、窄相位和错误处理的真实入口；使用：固定样本、旧新交替子基准；边界：不计样本构建，不将本地微基准外推到完整蓝图耗时。
func BenchmarkSpatialProduction(b *testing.B) {
	ctx, encoding := context.Background(), newPlanarRuleEncoding()
	belts, occupancy, power := spatialTestBelts(32, 128, false), spatialTestBelts(32, 128, true), spatialTestPower(8, 64)
	_, _ = encoding.validateBeltBuildingOccupancy(ctx, occupancy), encoding.spatialLegacyValidateBeltBuildingOccupancy(ctx, occupancy)
	type check struct {
		name         string
		old, current func() error
	}
	checks := []check{
		{"belt4096", func() error { return encoding.spatialLegacyValidateBeltCrossings(ctx, belts) }, func() error { return encoding.validateBeltCrossings(ctx, belts) }},
		{"occupancy4128", func() error { return encoding.spatialLegacyValidateBeltBuildingOccupancy(ctx, occupancy) }, func() error { return encoding.validateBeltBuildingOccupancy(ctx, occupancy) }},
		{"coverage1024", func() error { _, err := spatialLegacyValidateFinalPowerCoverage(ctx, encoding, power); return err }, func() error { _, err := validateFinalPowerCoverage(ctx, encoding, power); return err }},
		{"prune1024", func() error { _, err := spatialLegacyPlanFinalPowerPrune(ctx, encoding, power); return err }, func() error { _, err := planFinalPowerPrune(ctx, encoding, power); return err }},
	}
	for _, station := range []bool{false, true} {
		records, points := make([]blueprint.Building, 4096), make([]vector, 4096)
		for i := range records {
			item := int16(2201)
			if station {
				item = 2104
			}
			records[i] = spatialTestRecord(i, item, 0, 0, 0)
			points[i] = vector{float64(i%64-32) * 32, float64(i/64-32) * 32, 0}
		}
		old, current := spatialLegacySurfacePowerNodeConflict, surfacePowerNodeConflict
		if station {
			old, current = spatialLegacySurfaceStationConflict, surfaceStationConflict
		}
		checks = append(checks, check{fmt.Sprintf("spacing4096-station=%t", station), func() error { _, _, err := old(ctx, records, points); return err }, func() error { _, _, err := current(ctx, records, points); return err }})
	}
	for _, check := range checks {
		for _, old := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/legacy=%t", check.name, old), func(b *testing.B) {
				fn := check.current
				if old {
					fn = check.old
				}
				if err := fn(); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := fn(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// TestSpatialPointCache 职责：验证点索引不分配去重缓存而跨桶转换后正确增长；使用：缓存边界回归；边界：不会缓存窄相位结果。
func TestSpatialPointCache(t *testing.T) {
	index := worldSpatialIndex{}
	index.addPoint(vector{-8, -12, -29}, 8, 20)
	index.any(worldSpatialNeighbors(vector{-8, -12, -29}, 8), func(int) bool { return false })
	if len(index.query.marks) != 0 {
		t.Fatal("纯点索引不应分配去重缓存")
	}
	low, high := placement.Vector{-9, -13, -30}, placement.Vector{9, 13, 30}
	index.add(worldSpatialBounds(low, high, 8), 33)
	for _, serial := range []uint32{0, math.MaxUint32} {
		index.query.serial = serial
		calls := map[int]int{}
		index.any(worldSpatialBounds(low, high, 8), func(id int) bool { calls[id]++; return false })
		if calls[20] != 1 || calls[33] != 1 || len(calls) != 2 {
			t.Fatalf("追加或回绕后重复/丢失：%v", calls)
		}
	}
}
