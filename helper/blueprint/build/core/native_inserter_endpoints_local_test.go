package build

// 文件职责：对冻结端点枚举、表面动态盒和缓存执行独立小场景差分。
// 使用：性能任务局部验证及基准；边界：使用正式目录、跨度与碰撞检查，不构造6007整厂。

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// nativeEndpointFixture 职责：建立真实机器槽位到带或双带端点的小场景；使用：跨纬度和原型差分；边界：不伪造几何表，机器端点从正式槽位反算。
func nativeEndpointFixture(t testing.TB, anchor, item int, machine bool) (*builder, *surfaceEncoding, []blueprint.Building) {
	t.Helper()
	b, err := newBuilder(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	encoding := &surfaceEncoding{rootCoordinates: true, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(longitudeSegmentCount(anchor, 200)), Width: 100, Height: 40}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor, LayoutWidth: 100, LayoutHeight: 40}}
	input := b.make(2001, 20, 10, 0)
	output := b.make(2001, 22, 10, 0)
	arm := b.make(item, 20, 10, 0)
	arm.b = output.a
	arm.record.Parameters = []int32{1}
	arm.record.InputObjIdx, arm.record.OutputObjIdx = 0, 1
	arm.record.InputFromSlot, arm.record.OutputToSlot = 0, 0
	for i, entity := range b.all {
		entity.record.Index = int32(i)
		entity.record.LocalOffset = blueprint.Point{X: float32(entity.a.x), Y: float32(entity.a.y), Z: float32(entity.a.z)}
		entity.record.LocalOffset2 = blueprint.Point{X: float32(entity.b.x), Y: float32(entity.b.y), Z: float32(entity.b.z)}
	}
	if machine {
		input.record.ItemID, input.record.ModelIndex = 2302, b.models[2302]
		input.record.RecipeID = 1
		world, err := encoding.surfaceWorldPositions([]blueprint.Building{input.record})
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := placement.LoadCatalog()
		if err != nil {
			t.Fatal(err)
		}
		geometry, err := catalog.Geometry(input.record.ItemID, input.record.ModelIndex)
		if err != nil {
			t.Fatal(err)
		}
		pose, err := encoding.recordPose(world[0], input.record.Yaw, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		port := surfaceSlotPose(pose, geometry.SlotPoses[0], false)
		for end, point := range []placement.Vector{port.position, port.position.Add(port.axes[2].Scale(2.5))} {
			p := nativeSingleAreaPoint(encoding, point, 20)
			local := blueprint.Point{X: float32(p.x), Y: float32(p.y), Z: float32((math.Sqrt(point.Dot(point)) - 200.2) / 1.3333333)}
			if end == 0 {
				arm.record.LocalOffset = local
			} else {
				arm.record.LocalOffset2 = local
				output.record.LocalOffset = local
				output.record.LocalOffset2 = local
			}
		}
		arm.densityCargo.enabled = true
		aligned, err := nativeBeltEndHeading(encoding, arm.record, true)
		if err != nil {
			t.Fatal(err)
		}
		arm.record = aligned
	}
	records := make([]blueprint.Building, len(b.all))
	for i, entity := range b.all {
		records[i] = entity.record
	}
	return b, encoding, records
}

// TestNativeHotspotEndpointsDifferential 职责：对照完整冻结枚举与最终提交字段；使用：不同纬带、四种分拣器和机器/双带端；边界：逐值比较参数、端口、坐标、错误与实体状态。
func TestNativeHotspotEndpointsDifferential(t *testing.T) {
	for _, anchor := range []int{-170, 0, 160} {
		for _, machine := range []bool{false, true} {
			for item := 2011; item <= 2014; item++ {
				t.Run(fmt.Sprintf("%d/%t/%d", anchor, machine, item), func(t *testing.T) {
					oldB, oldE, oldR := nativeEndpointFixture(t, anchor, item, machine)
					newB, newE, newR := nativeEndpointFixture(t, anchor, item, machine)
					old, oe := oldB.frozenFitNativeInserterEndpoints(context.Background(), oldE, oldR)
					got, ge := newB.fitNativeInserterEndpoints(context.Background(), newE, newR)
					if old != got || fmt.Sprint(oe) != fmt.Sprint(ge) || !reflect.DeepEqual(oldR, newR) || !reflect.DeepEqual(oldB.all, newB.all) {
						t.Fatalf("enumeration changed: %d/%v vs %d/%v", old, oe, got, ge)
					}
					if ge != nil {
						t.Fatalf("fixture must succeed: %v", ge)
					}
				})
			}
		}
	}
}

// TestNativeHotspotSurfaceCacheDifferential 职责：逐值比较缓存命中、记录变化和错误时的动态盒及跨度副作用；使用：完整映射与姿态规则；边界：不缓存校验成功或错误，不跨作用域复用编码。
func TestNativeHotspotSurfaceCacheDifferential(t *testing.T) {
	for _, machine := range []bool{false, true} {
		_, encoding, records := nativeEndpointFixture(t, 160, 2013, machine)
		ctx := withNativeInserterSurfaceCache(context.Background(), encoding)
		for variant := 0; variant < 50; variant++ {
			trial := append([]blueprint.Building(nil), records...)
			trial[2].LocalOffset2.Z += float32(variant%9-4) * .01
			if variant == 10 {
				trial[2].InputObjIdx = 99
			}
			if variant == 11 {
				trial[2].InputFromSlot = -1
			}
			if variant == 12 {
				trial[0].AreaIndex = 10
			}
			if variant == 13 {
				trial[2].Yaw2 += 45
			}
			if variant == 14 {
				trial[0].ModelIndex = -1
			}
			if variant == 15 {
				trial[0].LocalOffset.Y += .1
			}
			if variant == 16 {
				trial[0].Yaw += 90
			}
			if variant == 17 {
				trial[2].LocalOffset2.X += 20
			}
			if variant == 18 {
				trial[2].LocalOffset.Z = float32(math.NaN())
			}
			if variant == 19 {
				trial[2].Index = 1
			}
			for repeat := 0; repeat < 2; repeat++ {
				oldS, newS := map[int32]int32{}, map[int32]int32{}
				old, oe := encoding.frozenSurfaceInserterBoxesMeasured(context.Background(), trial, oldS)
				got, ge := encoding.surfaceInserterBoxesMeasured(ctx, trial, newS)
				if fmt.Sprint(oe) != fmt.Sprint(ge) || !reflect.DeepEqual(old, got) || !reflect.DeepEqual(oldS, newS) {
					t.Fatalf("variant %d/%d machine %t changed: %v/%v", variant, repeat, machine, oe, ge)
				}
			}
		}
	}
}

// TestNativeHotspotCacheScope 职责：验证作用域隔离、缓存容量与枚举取消事件；使用：边界回归；边界：不把对象身份当成跨阶段成功证据。
func TestNativeHotspotCacheScope(t *testing.T) {
	b, encoding, records := nativeEndpointFixture(t, 160, 2013, true)
	ctx := withNativeInserterSurfaceCache(context.Background(), encoding)
	cache := nativeInserterCacheFromContext(ctx, encoding)
	other := *encoding
	if nativeInserterCacheFromContext(ctx, &other) != nil || nativeInserterCacheFromContext(context.Background(), encoding) != nil {
		t.Fatal("cache escaped its explicit encoding scope")
	}
	for i := 0; i < 8200; i++ {
		cache.world[nativeInserterWorldKey{point: blueprint.Point{X: float32(i + 100)}}] = vector{}
		cache.poses[nativeInserterPoseKey{point: vector{x: float64(i + 100)}}] = surfacePose{}
	}
	if _, err := cache.positions(records, false); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.pose(encoding, vector{x: 200.2}, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(cache.world) > 8192 || len(cache.poses) > 8192 {
		t.Fatal("unbounded cache")
	}
	var events []Progress
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = WithProgress(parent, func(p Progress) { events = append(events, p); cancel() })
	if _, err := b.fitNativeInserterEndpoints(ctx, encoding, records); err != context.Canceled {
		t.Fatalf("cancel=%v", err)
	}
	if len(events) != 1 || events[0].Stage != "native-inserter-domains" || events[0].Current != 0 || events[0].Total != 1 {
		t.Fatalf("events=%+v", events)
	}
}

// BenchmarkNativeHotspotEndpoints 职责：比较真实目录下独立端点枚举耗时和分配；使用：冻结与优化实现相同输入；边界：不代表整厂收益，不包括fixture准备成本。
func BenchmarkNativeHotspotEndpoints(b *testing.B) {
	for _, frozen := range []bool{true, false} {
		b.Run(fmt.Sprint(frozen), func(b *testing.B) {
			oldB, encoding, original := nativeEndpointFixture(b, 160, 2013, true)
			snapshots := make([]placed, len(oldB.all))
			for i, p := range oldB.all {
				snapshots[i] = *p
			}
			records := make([]blueprint.Building, len(original))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(records, original)
				for i, p := range oldB.all {
					*p = snapshots[i]
				}
				var err error
				if frozen {
					_, err = oldB.frozenFitNativeInserterEndpoints(context.Background(), encoding, records)
				} else {
					_, err = oldB.fitNativeInserterEndpoints(context.Background(), encoding, records)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// nativeHotspotPhysicalDomains 职责：从真实目录与冻结端点规则提取局部域；使用：物理候选冲突图差分及基准；边界：只平移八个微型单元，不运行整厂流程或减少候选。
func nativeHotspotPhysicalDomains(t testing.TB, dense bool) []nativeInserterDomain {
	t.Helper()
	var result []nativeInserterDomain
	dx, dy := float32(3), float32(4)
	if dense {
		dx, dy = .15, .7
	}
	for i := 0; i < 8; i++ {
		b, encoding, records := nativeEndpointFixture(t, 160, 2013, true)
		for j := range records {
			records[j].LocalOffset.X += float32(i%4) * dx
			records[j].LocalOffset2.X += float32(i%4) * dx
			records[j].LocalOffset.Y += float32(i/4) * dy
			records[j].LocalOffset2.Y += float32(i/4) * dy
		}
		var domains []nativeInserterDomain
		if _, err := b.captureNativeHotspotDomains(context.Background(), encoding, records, &domains); err != nil {
			t.Fatal(err)
		}
		for j := range domains {
			domains[j].index = len(result)
			result = append(result, domains[j])
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return len(result[i].choices) < len(result[j].choices) })
	return result
}

// TestNativeHotspotPhysicalGraph 职责：比较真实端点域的逐位冲突及最终选项；使用：小规模实际动态盒；边界：失败信息同样必须一致。
func TestNativeHotspotPhysicalGraph(t *testing.T) {
	domains := nativeHotspotPhysicalDomains(t, true)
	old, oe := frozenNativeInserterConflictGraph(context.Background(), domains)
	got, ge := nativeInserterConflictGraph(context.Background(), domains)
	if !reflect.DeepEqual(old, got) || fmt.Sprint(oe) != fmt.Sprint(ge) {
		t.Fatal("physical graph changed")
	}
	want, we := frozenSolveNativeInserterDomains(context.Background(), domains)
	actual, ae := solveNativeInserterDomains(context.Background(), domains)
	if !reflect.DeepEqual(want, actual) || fmt.Sprint(we) != fmt.Sprint(ae) {
		t.Fatalf("physical selection changed: %v / %v", we, ae)
	}
	for i, d := range domains {
		t.Logf("domain=%d choices=%d uniqueGeometry=%d", i, len(d.choices), len(nativeInserterGeometries(d).boxes))
	}
}

// BenchmarkNativeHotspotPhysicalGraph 职责：比较真实目录生成的端点域构图成本；使用：固定八个局部单元；边界：不计准备时间，不推断整厂耗时。
func BenchmarkNativeHotspotPhysicalGraph(b *testing.B) {
	for _, dense := range []bool{false, true} {
		domains := nativeHotspotPhysicalDomains(b, dense)
		for _, frozen := range []bool{true, false} {
			b.Run(fmt.Sprintf("dense=%t/frozen=%t", dense, frozen), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var err error
					if frozen {
						_, err = frozenNativeInserterConflictGraph(context.Background(), domains)
					} else {
						_, err = nativeInserterConflictGraph(context.Background(), domains)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
