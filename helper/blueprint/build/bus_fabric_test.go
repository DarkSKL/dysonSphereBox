package build

// 文件职责：以真实量化夹具回归可扩展混带环路及其失败关闭证据。
// 使用：mode6 单期验收和容量矩阵；边界：只验证抽象规则、物理连接和严格协议，不宣称游戏内十分钟持续产量。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestBusAcceptance 职责：覆盖五组真实目标的全机器、全流边与最终规则；使用：mode6 端到端验收；边界：不接受仅通过编码却缺少内部物料环路的结果。
func TestBusAcceptance(t *testing.T) {
	for _, fixture := range loadMatrixFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			plan, err := quant.Solve(context.Background(), fixture.Request)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: 6, Setting: fixture.Setting, Name: fixture.Name})
			if err != nil {
				t.Fatal(err)
			}
			assertGeneratedTransport(t, plan, 6, 3, out)
			assertBusPhysicalGraph(t, plan, out)
			t.Logf("buildings=%d lanes=%d inputs=%d area=%.0f", out.BuildingCount, len(out.LogisticsSummary.Lanes), out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea)
		})
	}
}

// TestBusRealBeltCapacityMatrix 职责：同一真实计划在三个所选等级产生实际环路/入口拆分；使用：等级不变与自动并联回归；边界：不以多条装饰带替代机器和正确物料连接。
func TestBusRealBeltCapacityMatrix(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	counts := [3]int{}
	for level := 1; level <= 3; level++ {
		t.Run(fmt.Sprintf("Mk%d", level), func(t *testing.T) {
			setting := fixture.Setting
			setting.BeltLv = level
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: 6, Setting: setting})
			if err != nil {
				t.Fatal(err)
			}
			assertGeneratedTransport(t, plan, 6, level, out)
			assertBusPhysicalGraph(t, plan, out)
			counts[level-1] = len(out.LogisticsSummary.Lanes)
			t.Logf("buildings=%d lanes=%d", out.BuildingCount, counts[level-1])
		})
	}
	if counts[0] < counts[1] || counts[1] < counts[2] || counts[0] == counts[2] {
		t.Fatalf("真实lane应随带等级自动拆分: %v", counts)
	}
}

// TestBusCompositeCapacityBoundary 职责：综合计划低等级带必须真实承载或明确受限失败；使用：二十万建筑/有界寻路反例；边界：失败不能发布旧串、半成品或擅升等级。
func TestBusCompositeCapacityBoundary(t *testing.T) {
	fixtures := loadMatrixFixtures(t)
	fixture := fixtures[len(fixtures)-1]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for level := 1; level <= 2; level++ {
		t.Run(fmt.Sprintf("Mk%d", level), func(t *testing.T) {
			setting := fixture.Setting
			setting.BeltLv = level
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: 6, Setting: setting})
			if err == nil {
				assertGeneratedTransport(t, plan, 6, level, out)
				assertBusPhysicalGraph(t, plan, out)
				t.Logf("Mk%d buildings=%d lanes=%d inputs=%d area=%.0f", level, out.BuildingCount, len(out.LogisticsSummary.Lanes), out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea)
				return
			}
			var failure *blueprint.Error
			if !errors.As(err, &failure) || (failure.Code != "GEN_E_LAYOUT" && failure.Code != "GEN_E_ROUTE") || out.Blueprint != "" || out.LogisticsSummary != nil {
				t.Fatalf("应为明确的有界布局/寻路失败且不发布部分结果: %+v %v", out, err)
			}
			t.Logf("Mk%d explicit bounded refusal: %v", level, err)
		})
	}
}

// TestBusSurfaceUsesFinalGeometry 职责：自动纬圈请求仍执行最终规则并保留全部环路；使用：旧总线折返回归；边界：不把平面成功当作球面粘贴证据。
func TestBusSurfaceUsesFinalGeometry(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for level := 1; level <= 3; level++ {
		t.Run(fmt.Sprintf("Mk%d", level), func(t *testing.T) {
			setting := fixture.Setting
			setting.BeltLv = level
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: 6, Setting: setting, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200}})
			if err != nil {
				t.Fatal(err)
			}
			assertGeneratedTransport(t, plan, 6, level, out)
			assertBusPhysicalGraph(t, plan, out)
			if out.SurfacePlan == nil || out.RuleAudit.CoordinateSpace == "equatorial-tangent" {
				t.Fatal("自动纬圈请求没有实际进入球面规则")
			}
		})
	}
}

// TestBusFabricRejectsCorruptRingAndProducer 职责：锁定重复环路指针、非有限流量与错接生产源的硬拒绝；使用：独立破坏证据反例；边界：不依赖放宽几何规则观察错误。
func TestBusFabricRejectsCorruptRingAndProducer(t *testing.T) {
	for _, mutate := range []func(*builder){
		func(b *builder) {
			p, q := b.busLayerBelts[0][0], b.busLayerBelts[0][1]
			p.output = q
			q.output = p
			b.busLayerBelts[0] = []*placed{p, q, p, q}
		},
		func(b *builder) { b.busFabric[0].route.rate = math.NaN() },
	} {
		b, _ := testBusExternalInputBuilder()
		b.busFabric = []*busFabricRoute{{route: &transportRoute{edge: b.flowGraph.Edges[0], rate: 30}, layer: 0}}
		mutate(b)
		if err := b.validateBusFabric(); err == nil {
			t.Fatal("损坏环路或NaN流量不得通过")
		}
	}
	producer := &transportRow{owner: 0, line: &line{output: []*placed{{owner: 0}}}}
	branch := &placed{owner: 0}
	arm := &placed{input: &placed{owner: 1}, output: branch}
	route := &transportRoute{rate: 30, from: producer, start: branch, path: []*placed{branch}, filters: []*placed{arm}}
	if err := validateBusFabricProducer(route); err == nil {
		t.Fatal("不属于指定生产者的过滤器不得冒充已连接源")
	}
}

// assertBusPhysicalGraph 职责：独立地在严格解码记录上逐物品搜索生产者到消费者；使用：真实mode6成功用例；边界：忽略错误物品过滤臂，不能靠闭环中其它产线的可达性蒙混过关。
func assertBusPhysicalGraph(t *testing.T, plan quant.Result, out Result) {
	t.Helper()
	decoded, err := blueprint.Decode(out.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	owner := map[int]int{}
	for _, binding := range out.Bindings {
		for _, index := range binding.MachineIndices {
			owner[index] = binding.ProductionIndex
		}
	}
	forward, reverse := make([][]int, len(decoded.Buildings)), make([][]int, len(decoded.Buildings))
	for index, record := range decoded.Buildings {
		if record.OutputObjIdx >= 0 {
			to := int(record.OutputObjIdx)
			forward[index] = append(forward[index], to)
			reverse[to] = append(reverse[to], index)
		}
		if record.InputObjIdx >= 0 {
			from := int(record.InputObjIdx)
			forward[from] = append(forward[from], index)
			reverse[index] = append(reverse[index], from)
		}
	}
	reaches := func(start, want, item int, adjacency [][]int) bool {
		queue, seen := []int{start}, map[int]bool{}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if seen[current] {
				continue
			}
			seen[current] = true
			record := decoded.Buildings[current]
			if record.ItemID >= 2011 && record.ItemID <= 2014 && record.FilterID != 0 && int(record.FilterID) != item {
				continue
			}
			if value, exists := owner[current]; exists {
				if value == want {
					return true
				}
				continue
			}
			queue = append(queue, adjacency[current]...)
		}
		return false
	}
	graph, err := quant.BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]quant.FlowNode{}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	for _, lane := range out.LogisticsSummary.Lanes {
		indices := lane.BuildingIndices
		if lane.FlowKind == "external-input" && busEncodedBeltCycles(decoded.Buildings, indices) < 2 {
			t.Fatalf("原料边 %s 缺少共享环路与真实回收供料闭环", lane.EdgeID)
		}
		if lane.FlowKind == "internal" && !lane.InternalConnected {
			t.Fatalf("mode6内部边未连接: %s", lane.EdgeID)
		}
		if source := nodes[lane.SourceNodeID]; source.Kind == "production" && !reaches(indices[0], source.ProductionIndex, lane.ItemID, reverse) {
			t.Fatalf("过滤物品%d没有来自正确生产者: %s", lane.ItemID, lane.EdgeID)
		}
		if target := nodes[lane.TargetNodeID]; target.Kind == "production" && !reaches(indices[len(indices)-1], target.ProductionIndex, lane.ItemID, forward) {
			t.Fatalf("过滤物品%d没有抵达正确消费者: %s", lane.ItemID, lane.EdgeID)
		}
	}
	found := false
	for _, check := range out.LogisticsSummary.Checks {
		if check.RuleID == "bus-ring-capacity" && check.Status == "passed" && strings.Contains(check.Message, "独立共享混带环路") {
			found = true
		}
	}
	if !found {
		t.Fatal("mode6没有报告已核验的真实共享环路容量")
	}
}

// busEncodedBeltCycles 职责：在独立解码的lane实体集合中统计纯带有向闭环；使用：核对原料回收不是装饰带或仅分拣器存在；边界：不把多条重复索引或非带对象计为第二个环。
func busEncodedBeltCycles(records []blueprint.Building, indices []int) int {
	members := map[int]bool{}
	for _, index := range indices {
		if surfaceIsBelt(records[index].ItemID) {
			members[index] = true
		}
	}
	finished, cycles := map[int]bool{}, 0
	for _, start := range indices {
		if !members[start] || finished[start] {
			continue
		}
		seen, path := map[int]bool{}, []int{}
		for current := start; members[current] && !finished[current]; current = int(records[current].OutputObjIdx) {
			if seen[current] {
				cycles++
				break
			}
			seen[current] = true
			path = append(path, current)
		}
		for _, current := range path {
			finished[current] = true
		}
	}
	return cycles
}
