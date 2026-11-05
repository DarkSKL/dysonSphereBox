package build

import (
	"context"
	"fmt"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestStandardFieldUsesSharedFacilities 职责：以实际多产线量化请求检查普通模式选中共享设施布局；使用：塔和配送回归；边界：完整生成与严格解码通过才算成功。
func TestStandardFieldUsesSharedFacilities(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 2208, RatePerMinute: 1}}, Dataset: "Vanilla"})
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		mode, item, maximum int
		rule                string
	}{
		{2, 2107, 5, "standard-fly-shared-field"},
		{3, 2103, 2, "standard-station-shared-field"},
		{4, 2104, 2, "standard-station-shared-field"},
	} {
		t.Run(fmt.Sprintf("mode%d", sample.mode), func(t *testing.T) {
			result, err := Generate(context.Background(), Request{Plan: plan, Mode: sample.mode, Density: densityStandard, Setting: Setting{BeltLv: 3, InserterLv: 4}, Name: "普通共享布局回归"})
			if err != nil {
				t.Fatal(err)
			}
			if result.LogisticsSummary == nil || result.LogisticsSummary.Metrics.RouteCrossings != 0 {
				t.Fatal("普通布局缺少完整物流证明或存在带段交叉")
			}
			selected := false
			for _, check := range result.LogisticsSummary.Checks {
				selected = selected || check.RuleID == sample.rule && check.Status == "passed"
				if check.RuleID == "bounded-fly-layout-search" || check.RuleID == "station-surround-packing" {
					t.Fatalf("模式 %d 仍执行旧布局：%s", sample.mode, check.RuleID)
				}
			}
			if !selected {
				t.Fatalf("模式 %d 未选中共享设施全场布局", sample.mode)
			}
			document, err := blueprint.Decode(result.Blueprint, true)
			if err != nil || !document.ChecksumValid {
				t.Fatalf("普通布局编码无效：%v", err)
			}
			facilities := 0
			for _, building := range document.Buildings {
				if int(building.ItemID) == sample.item {
					facilities++
				}
			}
			if facilities == 0 || facilities > sample.maximum {
				t.Fatalf("模式 %d 实际设施数 %d 超出共享布局上界 %d", sample.mode, facilities, sample.maximum)
			}
		})
	}
}

// TestDenseBeltInputsStayLeft 职责：核对传送带模式不同消费端的原料入口均在左侧；使用：南北两端和中部输入；边界：同侧独立端口仍至少相隔两格。
func TestDenseBeltInputsStayLeft(t *testing.T) {
	routes := []*transportRoute{}
	for _, y := range []float64{0, 10, 9, 1} {
		routes = append(routes, &transportRoute{end: &placed{a: vector{x: 8, y: y}}})
	}
	b := &builder{transportRoutes: routes}
	positions := b.denseInputPositions(vector{x: 0, y: 0}, vector{x: 12, y: 10})
	if len(positions) != len(routes) {
		t.Fatalf("入口数 %d，预期 %d", len(positions), len(routes))
	}
	for i, point := range positions {
		if point.x != -2 || routes[i].entryDirection != (vector{x: 1}) {
			t.Fatalf("入口 %d 不在左侧且向工厂：%+v，方向 %+v", i, point, routes[i].entryDirection)
		}
		for j := 0; j < i; j++ {
			if point.y-positions[j].y < 2 && positions[j].y-point.y < 2 {
				t.Fatalf("入口 %d 与 %d 太近", i, j)
			}
		}
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}, Dataset: "Vanilla"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4}, Name: "左侧入口回归"})
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	machineX := math.Inf(1)
	for _, building := range document.Buildings {
		if building.RecipeID > 0 {
			machineX = math.Min(machineX, float64(building.LocalOffset.X))
		}
	}
	inputs := 0
	for _, port := range result.LogisticsSummary.ExternalPorts {
		if port.Kind == "external-input" {
			inputs++
			if port.X >= machineX {
				t.Fatalf("真实蓝图原料入口 x=%.3f 未位于机器左侧 x=%.3f", port.X, machineX)
			}
		}
	}
	if inputs == 0 {
		t.Fatal("真实蓝图缺少外供入口")
	}
}

// TestOrdinaryRouteRejectsBeltCrossing 职责：防止普通共享布局把立体带穿越当作合法接线；使用：路由带段缓存；边界：超密度仍使用其独立路由规则。
func TestOrdinaryRouteRejectsBeltCrossing(t *testing.T) {
	first := &placed{a: vector{x: 1, y: 0}, record: blueprint.Building{ItemID: 2003}}
	second := &placed{a: vector{x: 1, y: 2}, record: blueprint.Building{ItemID: 2003}}
	first.output = second
	index := &denseRoutePortIndex{strictBelts: true, beltCells: map[[3]int][]int{}, beltEdges: map[[2]*placed]bool{}, registered: map[*placed]bool{}, nodes: map[[3]int][]*placed{}}
	index.addNode(first)
	if index.beltSpanClear(vector{x: 0, y: 1}, vector{x: 2, y: 1}) {
		t.Fatal("普通布局允许两条无连接带段相交")
	}
	if !index.beltSpanClear(vector{x: 0, y: 1, z: 1}, vector{x: 2, y: 1, z: 1}) {
		t.Fatal("不同高度且有净距的带段被错误拒绝")
	}
}
