package build

// 文件职责：独立验证本轮共用评分及物理度量；使用：主线24例重跑前的定向回归；边界：临时测试保留供主线最后清理，不运行旧二进制。

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// shapeTaskMetrics 职责：生成可比较的真实代价值；使用：排序边界测试；边界：不作为几何验证证据。
func shapeTaskMetrics(area, aspect, length float64) LayoutMetrics {
	return LayoutMetrics{EnvelopeArea: area, EnvelopeAspectRatio: aspect, TotalBeltLengthMeters: length, BeltCount: 100}
}

// TestShapePolicyTaskComparison 职责：核对验收优先、设施、面积与线路，所有配送/塔规格一致；使用：本轮回归；边界：只验证最终比较，不声称存在未枚举的可行候选。
func TestShapePolicyTaskComparison(t *testing.T) {
	cases := []struct {
		name   string
		a, b   LayoutMetrics
		fa, fb int
		want   bool
	}{
		{"accepted-before-cheap-strip", shapeTaskMetrics(200, 1.8, 120), shapeTaskMetrics(50, 1.81, 40), 3, 1, true},
		{"same-class-facilities", shapeTaskMetrics(200, 1.7, 120), shapeTaskMetrics(100, 1.2, 40), 1, 2, true},
		{"shape-cost", shapeTaskMetrics(100, 1.4, 100), shapeTaskMetrics(100, 1.7, 100), 2, 2, true},
		{"area-after-equal-long-edge", shapeTaskMetrics(120, 3, 100), shapeTaskMetrics(180, 2, 100), 2, 2, true},
		{"shorter-actual-belt", shapeTaskMetrics(100, 1.4, 99), shapeTaskMetrics(100, 1.4, 100), 2, 2, true},
		{"stable-tie", shapeTaskMetrics(100, 1.4, 100), shapeTaskMetrics(100, 1.4, 100), 2, 2, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := BetterLayoutCandidate(test.a, test.fa, test.b, test.fb); got != test.want {
				t.Fatalf("common got=%v want=%v", got, test.want)
			}
			for _, first := range []DistributorTopology{DistributorStandardField, DistributorDenseBlackbox, DistributorDenseDetached, DistributorInternalMode5} {
				for _, second := range []DistributorTopology{DistributorStandardField, DistributorDenseBlackbox, DistributorDenseDetached, DistributorInternalMode5} {
					if got := BetterDistributorCandidate(DistributorCandidateSpec{Topology: first}, test.fa, test.a, DistributorCandidateSpec{Topology: second}, test.fb, test.b); got != test.want {
						t.Fatalf("distributor %s/%s=%v", first, second, got)
					}
				}
			}
			for _, first := range []string{densityStandard, densityDense} {
				for _, second := range []string{densityStandard, densityDense} {
					if got := BetterStationCandidate(StationCandidateSpec{Density: first, SharedInputs: true}, test.fa, test.a, StationCandidateSpec{Density: second, Compact: true}, test.fb, test.b); got != test.want {
						t.Fatalf("station %s/%s=%v", first, second, got)
					}
				}
			}
		})
	}
	base := shapeTaskMetrics(100, 1.5, 100)
	filled := base
	filled.BlankRatio, filled.FootprintArea, filled.ShapeObjective = 0, 100, -1e9
	if BetterLayoutCandidate(filled, 1, base, 1) || BetterLayoutCandidate(base, 1, filled, 1) {
		t.Fatal("奖励了填带或缓存评分")
	}
	filled.TotalBeltLengthMeters++
	if BetterLayoutCandidate(filled, 1, base, 1) {
		t.Fatal("填带增加实际带长却获胜")
	}
	if !LayoutShapeAccepted(shapeTaskMetrics(100, 1.8, 100)) || LayoutShapeAccepted(shapeTaskMetrics(100, math.Nextafter(1.8, 2), 100)) {
		t.Fatal("验收边界错误")
	}
	for _, invalid := range []LayoutMetrics{{}, shapeTaskMetrics(0, 1, 100), shapeTaskMetrics(100, 0, 100), shapeTaskMetrics(100, math.NaN(), 100), shapeTaskMetrics(100, 1, math.Inf(1)), shapeTaskMetrics(100, 1, -1)} {
		if BetterLayoutCandidate(invalid, 1, base, 1) {
			t.Fatalf("非法候选获胜: %+v", invalid)
		}
	}
	if BetterStationRefinement(1, shapeTaskMetrics(90, 1.9, 99), 1, base) {
		t.Fatal("精修破坏已达标形状")
	}
	if _, err := BetterAdjacentCandidate(VerifiedSnapshot{}, VerifiedSnapshot{}); err == nil {
		t.Fatal("零值快照被接受")
	}
}

// TestShapePolicyTaskOrder 职责：验证归约比较的反对称与传递性；使用：排除epsilon导致次序相关；边界：无效结果不参与候选集合。
func TestShapePolicyTaskOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	type candidate struct {
		m          LayoutMetrics
		facilities int
	}
	values := make([]candidate, 30)
	for i := range values {
		values[i] = candidate{shapeTaskMetrics(100+rng.Float64()*100, 1+rng.Float64()*2, 100+rng.Float64()*100), rng.Intn(3)}
	}
	less := func(a, b candidate) bool { return BetterLayoutCandidate(a.m, a.facilities, b.m, b.facilities) }
	for _, a := range values {
		for _, b := range values {
			if less(a, b) && less(b, a) {
				t.Fatal("反对称失败")
			}
			for _, c := range values {
				if less(a, b) && less(b, c) && !less(a, c) {
					t.Fatal("传递性失败")
				}
			}
		}
	}
}

// TestShapePolicyTaskAdjacent 职责：验证跨带比较采用真实塔/配送器计数并拒绝错配快照；使用：独立比较边界测试；边界：构造值仅用于比较器测试，不取得实际编码权限。
func TestShapePolicyTaskAdjacent(t *testing.T) {
	snapshot := func(metrics LayoutMetrics, items ...int16) VerifiedSnapshot {
		geometry := &finalizedGeometry{}
		for _, item := range items {
			geometry.document.Buildings = append(geometry.document.Buildings, blueprint.Building{ItemID: item})
		}
		return VerifiedSnapshot{geometry: GeometrySnapshot{data: geometry}, validation: FinalValidation{data: &finalValidationData{geometry: geometry, logistics: &LogisticsSummary{Metrics: metrics}}}}
	}
	accepted := snapshot(shapeTaskMetrics(200, 1.8, 120), 2103, 2107)
	strip := snapshot(shapeTaskMetrics(100, 2, 90), 2103)
	if better, err := BetterAdjacentCandidate(accepted, strip); err != nil || !better {
		t.Fatalf("达标档未先于设施数: better=%v err=%v", better, err)
	}
	fewer := snapshot(shapeTaskMetrics(250, 1.7, 150), 2103, 2001, 2303)
	if better, err := BetterAdjacentCandidate(fewer, accepted); err != nil || !better {
		t.Fatalf("同档未比较实际物流设施: better=%v err=%v", better, err)
	}
	wrong := accepted
	wrong.validation = strip.validation
	if _, err := BetterAdjacentCandidate(wrong, strip); err == nil {
		t.Fatal("错配验证被接受")
	}
}

// TestShapePolicyTaskRectangle 职责：验证旋转长条不会靠空白矩形变方，卡壳与独立穷举一致；使用：几何算法回归；边界：测试内穷举不进入生产。
func TestShapePolicyTaskRectangle(t *testing.T) {
	for _, angle := range []float64{0, .1, .7, math.Pi / 4, 1.5} {
		s, c := math.Sincos(angle)
		points := []physicalShapePoint{}
		for _, x := range []float64{-10, 10} {
			for _, y := range []float64{-1, 1} {
				points = append(points, physicalShapePoint{x*c - y*s, x*s + y*c})
			}
		}
		w, h, err := physicalShapeRectangle(points)
		if err != nil || math.Abs(w*h-40) > 1e-9 || math.Abs(math.Max(w, h)/math.Min(w, h)-10) > 1e-9 {
			t.Fatalf("angle=%v w=%v h=%v err=%v", angle, w, h, err)
		}
	}
	rng := rand.New(rand.NewSource(41))
	for run := 0; run < 100; run++ {
		points := make([]physicalShapePoint, 25)
		for i := range points {
			points[i] = physicalShapePoint{rng.Float64() * 30, rng.Float64() * 20}
		}
		best := math.Inf(1)
		for i, a := range points {
			for _, b := range points[i+1:] {
				dx, dy := b.x-a.x, b.y-a.y
				length := math.Hypot(dx, dy)
				dx /= length
				dy /= length
				loX, hiX, loY, hiY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
				for _, p := range points {
					x, y := p.x*dx+p.y*dy, -p.x*dy+p.y*dx
					loX = math.Min(loX, x)
					hiX = math.Max(hiX, x)
					loY = math.Min(loY, y)
					hiY = math.Max(hiY, y)
				}
				best = math.Min(best, (hiX-loX)*(hiY-loY))
			}
		}
		w, h, err := physicalShapeRectangle(append([]physicalShapePoint(nil), points...))
		if err != nil || math.Abs(w*h-best) > 1e-8 {
			t.Fatalf("run=%d got=%g want=%g err=%v", run, w*h, best, err)
		}
	}
}

// TestShapePolicyTaskPhysicalInvariants 职责：验证跨区、空白Area和根尺寸不改变真实地表形状；使用：防止伪造空白；边界：仅使用正式世界映射，不改候选枚举；高度改变会略微改变实体的地表投影，不能作为等形变换。
func TestShapePolicyTaskPhysicalInvariants(t *testing.T) {
	ctx := context.Background()
	encoding := &surfaceEncoding{rootCoordinates: true, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: 21, Height: 21}, {Index: 1, ParentIndex: 0, AreaSegments: 200, AnchorLocalOffsetY: 5, Width: 21, Height: 16}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, LayoutWidth: 21, LayoutHeight: 21}}
	records := []blueprint.Building{}
	for _, x := range []float32{0, 20} {
		for _, y := range []float32{5, 20} {
			records = append(records, blueprint.Building{Index: int32(len(records)), ItemID: 2001, LocalOffset: blueprint.Point{X: x, Y: y}})
		}
	}
	metrics := shapeTaskMetrics(441, 9, 100)
	if err := updatePhysicalLayoutShapeMetrics(ctx, encoding, records, &metrics); err != nil {
		t.Fatal(err)
	}
	base := metrics.EnvelopeAspectRatio
	encoding.plan.LayoutWidth = 200
	encoding.areas[1].Width = 1000
	for i := range records {
		records[i].AreaIndex = 1
		records[i].LocalOffset.Y -= 5
	}
	if err := updatePhysicalLayoutShapeMetrics(ctx, encoding, records, &metrics); err != nil {
		t.Fatal(err)
	}
	if math.Abs(metrics.EnvelopeAspectRatio-base) > 2e-5 {
		t.Fatalf("相同物理外形随元数据/Area变化: %g => %g", base, metrics.EnvelopeAspectRatio)
	}
	if metrics.EnvelopeArea != 441 || metrics.TotalBeltLengthMeters != 100 {
		t.Fatal("度量改写了格面积或真实带长")
	}
	_, err := physicalShapeProjection([]placement.Vector{{1, 0, 0}, {-1, 0, 0}}, false)
	if err == nil {
		t.Fatal("跨半球未拒绝")
	}
	if err := updatePhysicalLayoutShapeMetrics(ctx, encoding, nil, &metrics); err == nil {
		t.Fatal("空实体被伪造为方形")
	}
}

// TestShapePolicyTaskCase21 职责：解码主线普通极区case21验证真实物理比例；使用：本轮离线证据复核；边界：只重算度量，不重跑候选或旧binary。
func TestShapePolicyTaskCase21(t *testing.T) {
	path := os.Getenv("SHAPE_POLICY_CASE21")
	if path == "" {
		t.Skip("设置SHAPE_POLICY_CASE21指向主线case-21.json后核验")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Result struct {
			Blueprint        string
			SurfacePlan      *SurfacePlan
			LogisticsSummary *LogisticsSummary
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(fixture.Result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	encoding := &surfaceEncoding{rootCoordinates: true, areas: doc.Areas, plan: fixture.Result.SurfacePlan}
	metrics := fixture.Result.LogisticsSummary.Metrics
	old := metrics.EnvelopeAspectRatio
	if err := updateDenseRootShapeMetrics(context.Background(), encoding, doc.Buildings, &metrics); err != nil {
		t.Fatal(err)
	}
	if math.Abs(old-metrics.EnvelopeAspectRatio) < 1 {
		t.Fatalf("未纠正Area加权假比例: %g => %g", old, metrics.EnvelopeAspectRatio)
	}
	t.Logf("case21 buildings=%d old-area-weighted=%g raw-root=%g physical=%g accepted=%v grid-area=%g belt-meters=%g", len(doc.Buildings), old, float64(encoding.plan.LayoutWidth)/float64(encoding.plan.LayoutHeight), metrics.EnvelopeAspectRatio, LayoutShapeAccepted(metrics), metrics.EnvelopeArea, metrics.TotalBeltLengthMeters)
}
