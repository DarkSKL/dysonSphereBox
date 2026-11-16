package build

// 文件职责：穷举塔输入优先的稳定重排；边界：不执行布局，只证明所有份额保留、同类保序与其他模式隔离。
import (
	"context"
	"reflect"
	"sort"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

func TestStationInputRouteOrder(t *testing.T) {
	policy, err := newAssemblyPolicy(3, densityStandard)
	if err != nil {
		t.Fatal(err)
	}
	tower := &placed{record: blueprint.Building{ItemID: 2103}}
	routes := []*transportRoute{{from: &transportRow{}, to: &transportRow{}}, {to: &transportRow{}, boundary: tower}, {from: &transportRow{}, boundary: tower}, {to: &transportRow{}, boundary: tower}, {to: &transportRow{}}}
	original := make([]transportRoute, len(routes))
	for i, r := range routes {
		original[i] = *r
	}
	var visit func(int)
	count := 0
	visit = func(n int) {
		if n < len(routes) {
			for i := n; i < len(routes); i++ {
				routes[n], routes[i] = routes[i], routes[n]
				visit(n + 1)
				routes[n], routes[i] = routes[i], routes[n]
			}
			return
		}
		before := append([]*transportRoute(nil), routes...)
		want := []*transportRoute{}
		for _, r := range before {
			if r.from == nil && r.to != nil && r.boundary == tower {
				want = append(want, r)
			}
		}
		for _, r := range before {
			if !(r.from == nil && r.to != nil && r.boundary == tower) {
				want = append(want, r)
			}
		}
		b := &builder{policy: policy, transportRoutes: append([]*transportRoute(nil), before...), directBoundaryUnits: func(context.Context, []*transportRow) ([]denseSpatialUnit, error) { return nil, nil }}
		b.prioritizeStandardStationInputs()
		if !reflect.DeepEqual(b.transportRoutes, want) {
			t.Fatal("同类顺序或路线成员改变")
		}
		densePolicy, _ := newAssemblyPolicy(3, densityDense)
		b.policy = densePolicy
		b.transportRoutes = before
		b.prioritizeStandardStationInputs()
		if !reflect.DeepEqual(b.transportRoutes, before) {
			t.Fatal("修改了密铺路线")
		}
		count++
	}
	visit(0)
	for i, r := range routes {
		if !reflect.DeepEqual(*r, original[i]) {
			t.Fatal("重排修改了路线本身")
		}
	}
	t.Logf("%d种排列稳定重排与密铺隔离通过", count)
}

// 排序试验仅作为测试记录保留：真实case05已证明它不能替代连续出口保护。
func (b *builder) prioritizeStandardStationInputs() {
	if b.policy == nil || b.policy.density != densityStandard || b.policy.station == nil || b.directBoundaryUnits == nil {
		return
	}
	external := func(r *transportRoute) bool {
		return r != nil && r.from == nil && r.to != nil && r.boundary != nil && surfaceIsStation(r.boundary.record.ItemID)
	}
	sort.SliceStable(b.transportRoutes, func(i, j int) bool { return external(b.transportRoutes[i]) && !external(b.transportRoutes[j]) })
}
