package planet

// 文件职责：核对原版电网覆盖和连接边界；使用：真实存档误拒及高层供电回归；边界：不放宽原版半径或碰撞规则。

import (
	"context"
	"errors"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
)

// TestFindPowerPathCoverageBoundary 职责：接受原版范围内已接通节点并拒绝范围外设备；使用：10.458 米误拒回归；边界：无新增落点可掩盖边界错误。
func TestFindPowerPathCoverageBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		distance float64
		covered  bool
	}{
		{"saved-gap", 10.458130721774818, true},
		{"exact-limit", 10.5, true},
		{"just-outside", float64(math.Nextafter32(10.5, float32(math.Inf(1)))), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts := []powerNode{{point: placement.Vector{}, connect: 22.5, cover: 10.5}}
			path, err := findPowerPath(context.Background(), starts, placement.Vector{tc.distance, 0, 0}, 0, 22.5, 10.5,
				func(Position) bool { return false }, powerGridPoint)
			if (err == nil) != tc.covered || len(path) != 0 {
				t.Fatalf("距离 %.12f，覆盖 %t，路径 %d，错误 %v", tc.distance, tc.covered, len(path), err)
			}
		})
	}
	start := Position{Longitude: 5.570044040679932, Latitude: -.10367254912853241}
	target := Position{Longitude: 5.573185443878174, Latitude: -.05152212083339691}
	_, err := findPowerPath(context.Background(), []powerNode{{position: start, point: powerGridPoint(start), connect: 22.5, cover: 10.5}}, powerGridPoint(target), 0, 22.5, 10.5,
		func(Position) bool { t.Fatal("真实存档中的设备已覆盖，不应再寻落点"); return false }, powerGridPoint)
	if err != nil {
		t.Fatal(err)
	}
}

// TestFindPowerPathNodeBoundary 职责：按原版两端较大连接半径核对节点；使用：节点连接回归；边界：等于半径可接，下一单精度值不可接。
func TestFindPowerPathNodeBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		startReach, targetReach, distance float64
		connected                         bool
	}{
		{"existing-larger", 30, 15, 30, true},
		{"target-larger", 15, 30, 30, true},
		{"inside-old-margin", 22.5, 15, 22.4, true},
		{"just-outside", 15, 30, float64(math.Nextafter32(30, float32(math.Inf(1)))), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := findPowerPath(context.Background(), []powerNode{{connect: tc.startReach}}, placement.Vector{tc.distance, 0, 0}, tc.targetReach, 22.5, 10.5,
				func(Position) bool { return false }, powerGridPoint)
			if (err == nil) != tc.connected {
				t.Fatalf("期望连接 %t，错误 %v", tc.connected, err)
			}
		})
	}
}

// TestFixedNodesConnectFromFarEnd 核验工区另一端接电的情况；间距刚刚过界的孤立杆不能混入已接通分量。
func TestFixedNodesConnectFromFarEnd(t *testing.T) {
	node := func(x float64) powerNode { return powerNode{point: placement.Vector{x, 0, 0}, connect: 22.5} }
	network := []powerNode{node(0)}
	fixed := []powerNode{node(66), node(115), node(44), node(22)}
	if got := connectedFixedNodes(t.Context(), network, fixed); len(got) != 3 || got[0].point[0] != 66 {
		t.Fatalf("应经两根已有电杆接回工区：%v", got)
	}
	if got := connectedFixedNodes(t.Context(), network, []powerNode{node(67.51), node(45.01), node(22.51)}); len(got) != 0 {
		t.Fatalf("断开 0.01 游戏米的固定分量不应获电：%v", got)
	}
}

// TestPowerGridSurfaceNormalization 职责：核对原版高层接电点按高度归一的语义；使用：电网距离回归；边界：建筑的真实高度不修改。
func TestPowerGridSurfaceNormalization(t *testing.T) {
	ground := Position{Longitude: .6, Latitude: -.2}
	high := ground
	high.Height = 30
	a, b := powerGridPoint(ground), powerGridPoint(high)
	if powerGridDistanceSquared(a, b) > 1e-6 {
		t.Fatalf("相同经纬的高层接电点未归一: %v / %v", a, b)
	}
	if math.Abs(math.Sqrt(b.Dot(b))-200.2) > 1e-4 || high.Height != 30 {
		t.Fatalf("地表接电点或实体高度错误: %v / %+v", b, high)
	}
	if _, err := findPowerPath(context.Background(), []powerNode{{position: ground, point: a, connect: 22.5, cover: 10.5}}, b, 0, 22.5, 10.5,
		func(Position) bool { return false }, powerGridPoint); err != nil {
		t.Fatal(err)
	}
}

// TestFindPowerPathCancelled 职责：确保已取消父任务不会以已有覆盖提前成功；使用：任务取消回归；边界：不启动新增路径搜索。
func TestFindPowerPathCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := findPowerPath(ctx, []powerNode{{cover: 10.5}}, placement.Vector{}, 0, 22.5, 10.5,
		func(Position) bool { t.Fatal("取消后不应搜索"); return false }, powerGridPoint)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("期望父取消，实际 %v", err)
	}
}

// TestPowerGridCoveredStationBlueprint 职责：证明边界内消费者可保持原装电塔并完成严格编码；使用：原版风机、电塔与星际塔小蓝图；边界：不靠额外电塔遮盖误拒。
func TestPowerGridCoveredStationBlueprint(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	deviceFacts, err := facts.Load()
	if err != nil {
		t.Fatal(err)
	}
	generator, pole := Position{}, Position{Longitude: .09}
	consumer := Position{Longitude: pole.Longitude + 2*math.Asin(10.458130721774818/(2*200.2))}
	req := Request{PlanetID: 101, Objects: []Object{
		{First: generator, Second: generator, Building: blueprint.Building{Index: 0, ItemID: 2203, ModelIndex: 53, InputObjIdx: -1, OutputObjIdx: -1}},
		{First: pole, Second: pole, Building: blueprint.Building{Index: 1, ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}},
		stationObject(2, consumer, 1001, 2, 2, false),
	}}
	if err := connectPowerGrid(context.Background(), &req, catalog, deviceFacts); err != nil {
		t.Fatal(err)
	}
	if len(req.Objects) != 3 {
		t.Fatalf("已覆盖设备被要求增建电塔: %d", len(req.Objects))
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || len(document.Buildings) != 3 {
		t.Fatalf("严格解码失败: %v", err)
	}
}
