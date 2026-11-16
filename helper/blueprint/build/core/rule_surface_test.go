package build

// 文件职责：验证最终规则坐标视图与完整装配坐标等价；使用：规则复制性能回归；边界：独立保留旧完整复制为对照，不跳过任何物理检查。

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// ruleSurfaceFixture 职责：构造带真实坐标元数据和装配指针链的视图；使用：复制差分与基准；边界：指针链不参与最终记录坐标解释。
func ruleSurfaceFixture(count int) *surfaceEncoding {
	s := &surfaceEncoding{rootCoordinates: true, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: 100, Height: 40}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: 0, LayoutWidth: 100, LayoutHeight: 40}, byBuilding: map[*placed]int{}, latitudeOffsets: []int{0}, longitudeScales: []float64{1}, anchorCandidates: []int{0, 1}, relocation: &denseSurfaceRelocation{sourceRecords: make([]blueprint.Building, count), safeAnchors: []int{0, 1}}}
	var previous *placed
	for i := 0; i < count; i++ {
		p := &placed{input: previous}
		if previous != nil {
			previous.output = p
		}
		s.byBuilding[p] = 0
		previous = p
	}
	return s
}

// TestRuleSurfaceProjection 职责：核对真实单项物理规则、世界坐标及缓存键等价；使用：南北纬与失败输入差分；边界：副本写入不能污染原视图。
func TestRuleSurfaceProjection(t *testing.T) {
	ctx := context.Background()
	for _, anchor := range []int{-200, -130, -79, 0, 79, 130, 200} {
		for _, root := range []bool{false, true} {
			source := ruleSurfaceFixture(100)
			source.rootCoordinates = root
			source.plan.AnchorLatitudeGrid = anchor
			source.areas[0].AreaSegments = int16(longitudeSegmentCount(anchor, 200))
			full := newFinalizationCloner().buildSurfaceEncoding(*source)
			c := newFinalizationCloner()
			slim := copyRuleSurface(c, source)
			if len(c.pointers) > 1 || slim.byBuilding != nil || slim.relocation != nil || slim.anchorCandidates != nil {
				t.Fatal("retained assembly graph")
			}
			records := []blueprint.Building{{Index: 0, ItemID: 2001, ModelIndex: 35, InputObjIdx: -1, OutputObjIdx: 1, LocalOffset: blueprint.Point{X: 3, Y: 3}}, {Index: 1, ItemID: 2001, ModelIndex: 35, InputObjIdx: 0, OutputObjIdx: -1, LocalOffset: blueprint.Point{X: 4, Y: 3}}}
			a, ae := full.surfaceWorldPositions(records)
			b, be := slim.surfaceWorldPositions(records)
			if fmt.Sprint(ae) != fmt.Sprint(be) || !reflect.DeepEqual(a, b) {
				t.Fatal("world position differs")
			}
			for _, kind := range []RuleKind{RuleReferences, RulePastedSlots, RuleBeltDistance, RuleStaticColliders, RuleTankPorts, RulePowerSpacing, RuleStationSpacing, RuleBeltCrossings, RuleBeltBuildingOccupancy, RuleInserterGeometry, RuleSplitterClearance, RuleStationPorts, RuleStorageRoof, RulePowerNetwork} {
				n, err := executePlanetRule(ctx, kind, Request{}, &full, records)
				m, other := executePlanetRule(ctx, kind, Request{}, slim, records)
				if n != m || fmt.Sprint(err) != fmt.Sprint(other) {
					t.Fatalf("%s: %d/%v != %d/%v", kind, n, err, m, other)
				}
			}
			first, err := finalRuleFingerprint(ctx, Request{}, &full, records)
			second, other := finalRuleFingerprint(ctx, Request{}, slim, records)
			if err != nil || other != nil || first != second {
				t.Fatal("rule input fingerprint differs")
			}
			slim.areas[0].Width = 5
			slim.plan.LayoutWidth = 5
			slim.longitudeScales[0] = 2
			if source.areas[0].Width != 100 || source.plan.LayoutWidth != 100 || source.longitudeScales[0] != 1 {
				t.Fatal("copy leaked mutable alias")
			}
		}
	}
}

// BenchmarkRuleSurfaceCopy 职责：隔离测量同一万实体来源的坐标视图复制成本；使用：优化前后局部基准；边界：不是整图耗时或RSS声明。
func BenchmarkRuleSurfaceCopy(b *testing.B) {
	source := ruleSurfaceFixture(10000)
	for _, slim := range []bool{false, true} {
		b.Run(fmt.Sprint(slim), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				c := newFinalizationCloner()
				if slim {
					_ = copyRuleSurface(c, source)
				} else {
					_ = c.buildSurfaceEncoding(*source)
				}
			}
		})
	}
}
