package build

// 文件职责：验证有限塔口连续保护及所有者豁免；边界：不运行完整大厂，不以局部测试代替case05验收。
import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"testing"
)

func TestStationInputEscapes(t *testing.T) {
	index := routeMinimalIndex(nil)
	source := routeMinimalNode(vector{})
	source.input = routeMinimalNode(vector{x: 1})
	index.addNode(source)
	target := routeMinimalNode(vector{-6, 3, 0})
	index.addNode(target)
	env := &transportEnvironment{densePorts: index, minimum: vector{-10, -10, 0}, maximum: vector{10, 10, 32}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
	index.blockNode(env.blocked, source.a)
	index.blockNode(env.blocked, target.a)
	route := &transportRoute{start: source, end: target, to: &transportRow{}, boundary: &placed{record: blueprint.Building{ItemID: 2103}}}
	if env.reserveStationInputEscapes([]*transportRoute{route, route}) != 1 {
		t.Fatal("未唯一保护真实塔口")
	}
	if len(env.reservedSpans) < 2 {
		t.Fatal("没有连续portal及出口边")
	}
	if !env.reservedSpanClear(source.a, vector{x: -1}, source, target) {
		t.Fatal("自身出口被预留阻塞")
	}
	if env.reservedSpanClear(vector{-.5, -1, 0}, vector{-.5, 1, 0}, nil, nil) {
		t.Fatal("外来整段可穿越预留节点之间")
	}
	if len(index.registered) != 2 {
		t.Fatal("预留创建了实际节点")
	}
	if len(env.reserved) != 0 {
		t.Fatal("把分数塔口通道取整成了独占格")
	}
	index.strictBelts = false
	if env.reserveStationInputEscapes([]*transportRoute{route}) != 0 {
		t.Fatal("影响了密铺")
	}
	index.strictBelts = true
	route.from = &transportRow{}
	if env.reserveStationInputEscapes([]*transportRoute{route}) != 0 {
		t.Fatal("预留了内部端点")
	}
}

func TestStationInputEscapesJointChoice(t *testing.T) {
	span := func(y float64) stationEscapeChoice {
		return stationEscapeChoice{shapes: []packingShape{beltPackingShape(placement.Vector{0, y, 0}, placement.Vector{1, y, 0})}}
	}
	// 第一个口的首选堵死第二个口；必须回溯选择它的另一个已合法出口。
	ports := []stationEscapePort{{choices: []stationEscapeChoice{span(0), span(2)}}, {choices: []stationEscapeChoice{span(0)}}}
	selected := chooseStationEscapes(ports)
	if len(selected) != 2 || selected[0] != 1 || selected[1] != 0 {
		t.Fatalf("未找到完整兼容组合: %v", selected)
	}
	ports[0].choices = ports[0].choices[:1]
	if chooseStationEscapes(ports) != nil {
		t.Fatal("把交叉预留当作完整可行组合")
	}
}

func TestStationInputEscapesDelivery(t *testing.T) {
	index := routeMinimalIndex(nil)
	source, target := routeMinimalNode(vector{4, 4, 0}), routeMinimalNode(vector{})
	target.output = routeMinimalNode(vector{x: 1})
	index.addNode(source)
	index.addNode(target)
	env := &transportEnvironment{densePorts: index, minimum: vector{-10, -10, 0}, maximum: vector{10, 10, 32}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
	r := &transportRoute{from: &transportRow{}, start: source, end: target, boundary: &placed{record: blueprint.Building{ItemID: 2103}}}
	if env.reserveStationInputEscapes([]*transportRoute{r}) != 1 {
		t.Fatal("未保护真实入塔交付口")
	}
	for _, span := range env.reservedSpans {
		if span.owner != target {
			t.Fatal("入塔许可没有绑定真实目标身份")
		}
	}
	if !env.reservedSpanClear(vector{x: -1}, target.a, source, target) {
		t.Fatal("拒绝交付路线自身")
	}
	if env.reservedSpanClear(vector{-.5, -1, 0}, vector{-.5, 1, 0}, nil, nil) {
		t.Fatal("允许其他路线横穿入塔通道")
	}
}

func TestStationProductionEscapesRealRowOnly(t *testing.T) {
	idx := routeMinimalIndex(nil)
	source := routeMinimalNode(vector{})
	source.input = routeMinimalNode(vector{x: 1})
	target := routeMinimalNode(vector{-5, 3, 0})
	idx.addNode(source)
	idx.addNode(target)
	env := &transportEnvironment{densePorts: idx, minimum: vector{-10, -10, 0}, maximum: vector{10, 10, 32}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
	r := &transportRoute{from: &transportRow{}, to: &transportRow{}, start: source, end: target}
	if env.reserveStationProductionEscapes([]*transportRoute{r}) != 0 {
		t.Fatal("保护了没有真实生产行的辅助节点")
	}
	r.from.line = &line{}
	if env.reserveStationProductionEscapes([]*transportRoute{r}) != 1 {
		t.Fatal("未保护真实生产行出口")
	}
	for _, span := range env.reservedSpans {
		if span.owner != source {
			t.Fatal("出口所有者错误")
		}
	}
}
