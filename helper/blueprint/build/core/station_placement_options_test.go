package build

import (
	"testing"

	"DysonSphereBox/helper/blueprint"
)

func TestStationPlacementRetainsOneDistinctFootprint(t *testing.T) {
	// 同一候选族里两个最高分使用相同锚点与包络。第三个形状仍应保留给整星落位。
	sample := func(anchor, towers int) VerifiedSnapshot {
		doc := blueprint.Document{Buildings: make([]blueprint.Building, towers)}
		for i := range doc.Buildings {
			doc.Buildings[i].ItemID = 2104
		}
		geometry := &finalizedGeometry{document: doc}
		return VerifiedSnapshot{geometry: GeometrySnapshot{data: geometry}, validation: FinalValidation{data: &finalValidationData{
			geometry:    geometry,
			logistics:   &LogisticsSummary{Metrics: LayoutMetrics{EnvelopeArea: 80, EnvelopeAspectRatio: 1.5}},
			surfacePlan: &SurfacePlan{AnchorLatitudeGrid: anchor, LayoutWidth: 25, LayoutHeight: 10},
		}}}
	}
	a, b, c, d := sample(-24, 1), sample(-24, 2), sample(-30, 3), sample(-31, 4)
	for _, sequence := range [][]VerifiedSnapshot{{b, d, a, c}, {c, a, d, b}} {
		var options StationPlacementOptions
		for i, v := range sequence {
			options.Offer(v, string(rune('a'+i)))
		}
		other := options.Alternative(a.validation.data.surfacePlan)
		if other == nil || other.geometry.data != c.geometry.data {
			t.Fatal("同外形候选挤掉了已验的不同锚纬")
		}
		other = options.Alternative(c.validation.data.surfacePlan)
		if other == nil || other.geometry.data != a.geometry.data {
			t.Fatal("未沿用原塔布局的同档设施数排序")
		}
	}
}
