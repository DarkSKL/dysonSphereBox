package build

// 文件职责：以真实量化夹具执行带等级矩阵及独立的最终实体图核验。
// 使用：跨模式回归验证机器、逐边流量、边界接口与所选等级；边界：离线验收不等同于实际游戏十分钟产量。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// matrixFixture 职责：读取已锁定的真实量化输入；使用：矩阵回归；边界：不依赖前端默认值，也不覆盖原有基线。
type matrixFixture struct {
	// Name 职责：保留固定案例标识；使用：失败定位；边界：不参与物流求解。
	Name string `json:"name"`
	// Request 职责：固定真实量化输入；使用：重跑量化器；边界：不手工伪造产线。
	Request quant.Request `json:"request"`
	// Setting 职责：固定设备、带和布局选项；使用：只覆盖矩阵中的等级；边界：其它选项沿用版本化夹具。
	Setting Setting `json:"setting"`
}

// loadMatrixFixtures 职责：加载并确认五组真实验收样本；使用：生成矩阵；边界：文件缺失或数据集变化必须明确失败。
func loadMatrixFixtures(t *testing.T) []matrixFixture {
	t.Helper()
	var fixtures struct {
		Cases []matrixFixture `json:"cases"`
	}
	data, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 5 {
		t.Fatalf("真实量化夹具应为五组，实际 %d", len(fixtures.Cases))
	}
	return fixtures.Cases
}

// assertGeneratedTransport 职责：独立核对最终编码记录与原计划；使用：所有成功候选的端到端回归；边界：不复用生成器容量/连接校验以防相同实现掩盖错误。
func assertGeneratedTransport(t *testing.T, plan quant.Result, mode, level int, out Result) {
	t.Helper()
	decoded, err := blueprint.Decode(out.Blueprint, true)
	if err != nil {
		t.Fatalf("严格解码失败: %v", err)
	}
	if out.BuildingCount != len(decoded.Buildings) || len(decoded.Buildings) > blueprint.MaxBuildingCount {
		t.Fatal("最终建筑数或规模上限不符")
	}
	if out.RuleAudit == nil || out.RuleAudit.Coverage != "partial-numeric" || len(out.RuleAudit.Checks) < 6 {
		t.Fatal("缺少已执行的最终规则证据")
	}
	summary := out.LogisticsSummary
	capacity, err := transport.BeltCapacityPerMinute(level)
	if err != nil {
		t.Fatal(err)
	}
	if summary == nil || summary.BeltLevel != level || summary.BeltCapacityPerMinute != capacity {
		t.Fatal("摘要丢失或改变所选带等级")
	}
	graph, err := quant.BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FlowGraphDigest != graph.Digest {
		t.Fatal("模式更改了规范物料图")
	}
	machineOwner := map[int]int{}
	sprayers := map[int][]blueprint.Building{}
	bindingCounts := make([]int, len(plan.Productions))
	for _, binding := range out.Bindings {
		if binding.ProductionIndex < 0 || binding.ProductionIndex >= len(plan.Productions) {
			t.Fatal("产线映射越界")
		}
		production := plan.Productions[binding.ProductionIndex]
		for _, index := range binding.BuildingIndices {
			if index >= 0 && index < len(decoded.Buildings) && decoded.Buildings[index].ItemID == 2313 {
				sprayers[binding.ProductionIndex] = append(sprayers[binding.ProductionIndex], decoded.Buildings[index])
			}
		}
		for _, index := range binding.MachineIndices {
			if index < 0 || index >= len(decoded.Buildings) {
				t.Fatal("机器索引越界")
			}
			if _, duplicate := machineOwner[index]; duplicate {
				t.Fatalf("机器 %d 重复归属", index)
			}
			if int(decoded.Buildings[index].ItemID) != production.MachineID || int(decoded.Buildings[index].RecipeID) != production.RecipeID {
				t.Fatalf("机器 %d 的设备/配方与量化不一致", index)
			}
			machineOwner[index] = binding.ProductionIndex
			bindingCounts[binding.ProductionIndex]++
		}
	}
	for owner, production := range plan.Productions {
		if bindingCounts[owner] != production.MachineCount {
			t.Fatalf("产线 %d 机器变更：%d != %d", owner, bindingCounts[owner], production.MachineCount)
		}
	}
	for index, record := range decoded.Buildings {
		if surfaceIsBelt(record.ItemID) && int(record.ItemID) != 2000+level {
			t.Fatalf("建筑 %d 静默改变带等级：%d", index, record.ItemID)
		}
	}
	laneTotals, boundaryTotals := map[string]float64{}, map[string]float64{}
	edges := map[string]quant.FlowEdge{}
	nodes := map[string]quant.FlowNode{}
	for _, edge := range graph.Edges {
		edges[edge.ID] = edge
	}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	// 从实际记录建立有向图；分拣器的输入引用表示源建筑到分拣器，输出引用表示分拣器到目标建筑。
	forward, reverse := make([][]int, len(decoded.Buildings)), make([][]int, len(decoded.Buildings))
	connectRecords := func(from, to int) {
		if from < 0 || to < 0 {
			return
		}
		if from >= len(forward) || to >= len(forward) {
			t.Fatal("最终连接越界")
		}
		forward[from] = append(forward[from], to)
		reverse[to] = append(reverse[to], from)
	}
	for index, record := range decoded.Buildings {
		connectRecords(index, int(record.OutputObjIdx))
		connectRecords(int(record.InputObjIdx), index)
	}
	reachesOwner := func(start, owner int, adjacency [][]int) bool {
		visited := make([]bool, len(adjacency))
		queue := []int{start}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if visited[current] {
				continue
			}
			visited[current] = true
			if found, machine := machineOwner[current]; machine {
				if found == owner {
					return true
				}
				continue
			}
			queue = append(queue, adjacency[current]...)
		}
		return false
	}
	// 喷涂机通过原型附加通道从一层供剂带取料，蓝图不会为该通道写机器Input引用。
	reachesSprayer := func(start, owner int) bool {
		visited := make([]bool, len(forward))
		queue := []int{start}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if visited[current] {
				continue
			}
			visited[current] = true
			belt := decoded.Buildings[current]
			if surfaceIsBelt(belt.ItemID) {
				for _, sprayer := range sprayers[owner] {
					if belt.AreaIndex == sprayer.AreaIndex && math.Abs(float64(belt.LocalOffset.X-sprayer.LocalOffset.X)+1) < 1e-4 && math.Abs(float64(belt.LocalOffset.Y-sprayer.LocalOffset.Y)) < 1e-4 && math.Abs(float64(belt.LocalOffset.Z-sprayer.LocalOffset.Z)-1) < 1e-4 {
						return true
					}
				}
			}
			queue = append(queue, forward[current]...)
		}
		return false
	}
	for _, lane := range summary.Lanes {
		if lane.RatePerMinute <= 0 || lane.RatePerMinute > capacity+1e-6 || lane.CapacityPerMinute != capacity || len(lane.BuildingIndices) == 0 {
			t.Fatalf("无实体或超载 lane: %+v", lane)
		}
		for _, index := range lane.BuildingIndices {
			if index < 0 || index >= len(decoded.Buildings) {
				t.Fatal("lane 建筑索引越界")
			}
		}
		edge, canonical := edges[lane.EdgeID]
		if !canonical {
			t.Fatalf("实体路线 %s 不属于规范流图", lane.EdgeID)
		}
		if edge.ItemID != lane.ItemID || edge.FlowKind != lane.FlowKind || edge.SourceNodeID != lane.SourceNodeID || edge.TargetNodeID != lane.TargetNodeID {
			t.Fatalf("lane 指向错误物料/供需端: %+v", lane)
		}
		laneTotals[lane.EdgeID] += lane.RatePerMinute
		if mode == 1 {
			if node := nodes[edge.SourceNodeID]; node.Kind == "production" && !reachesOwner(lane.BuildingIndices[0], node.ProductionIndex, reverse) {
				t.Fatalf("边 %s 没有实体生产者", edge.ID)
			}
			if node := nodes[edge.TargetNodeID]; node.Kind == "production" {
				last := lane.BuildingIndices[len(lane.BuildingIndices)-1]
				if productionSprayItem(plan.Productions[node.ProductionIndex]) == lane.ItemID {
					if !reachesSprayer(last, node.ProductionIndex) {
						t.Fatalf("边 %s 没有抵达真实喷涂附加通道", edge.ID)
					}
				} else if !reachesOwner(last, node.ProductionIndex, forward) {
					t.Fatalf("边 %s 没有实体消费者", edge.ID)
				}
			}
			if edge.FlowKind == "internal" && !lane.InternalConnected {
				t.Fatalf("内部边 %s 未连接", edge.ID)
			}
		}
	}
	for _, port := range summary.ExternalPorts {
		if port.BuildingIndex < 0 || port.BuildingIndex >= len(decoded.Buildings) || port.RatePerMinute <= 0 || port.RatePerMinute > capacity+1e-6 {
			t.Fatalf("无效边界: %+v", port)
		}
		if edge, canonical := edges[port.EdgeID]; canonical {
			if port.ItemID != edge.ItemID || port.Kind != edge.FlowKind || edge.FlowKind == "internal" {
				t.Fatal("边界语义不符")
			}
			boundaryTotals[port.EdgeID] += port.RatePerMinute
		}
	}
	for _, edge := range graph.Edges {
		tolerance := math.Max(1e-6, edge.RatePerMinute*1e-8)
		if math.Abs(laneTotals[edge.ID]-edge.RatePerMinute) > tolerance {
			t.Fatalf("边 %s lane 流量 %.9f != %.9f", edge.ID, laneTotals[edge.ID], edge.RatePerMinute)
		}
		if edge.FlowKind != "internal" && math.Abs(boundaryTotals[edge.ID]-edge.RatePerMinute) > tolerance {
			t.Fatalf("边 %s 边界流量 %.9f != %.9f", edge.ID, boundaryTotals[edge.ID], edge.RatePerMinute)
		}
	}
}

// TestConnectedRealBeltCapacityMatrix 职责：同一真实量化输入在三个带等级重跑，核对机器/图不变、lane 自动变化；使用：单期容量验收；边界：不将被生产的极速传送带与物流等级混淆。
func TestConnectedRealBeltCapacityMatrix(t *testing.T) {
	fixtures := loadMatrixFixtures(t)
	for _, fixture := range fixtures {
		plan, err := quant.Solve(context.Background(), fixture.Request)
		if err != nil {
			t.Fatal(err)
		}
		counts := [4]int{}
		for _, level := range []int{1, 2, 3} {
			t.Run(fmt.Sprintf("%s-Mk%d", fixture.Name, level), func(t *testing.T) {
				setting := fixture.Setting
				setting.BeltLv = level
				out, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: setting, Name: fixture.Name})
				if err != nil {
					t.Fatal(err)
				}
				assertGeneratedTransport(t, plan, 1, level, out)
				counts[level] = len(out.LogisticsSummary.Lanes)
				t.Logf("machines=%d buildings=%d lanes=%d inputs=%d area=%.0f", plan.MachineCount, out.BuildingCount, counts[level], out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea)
			})
		}
		if counts[1] < counts[2] || counts[2] < counts[3] {
			t.Fatalf("%s 降级带反而减少实际路线：%v", fixture.Name, counts)
		}
	}
}

// TestStationRealBeltCapacityMatrix 职责：核验塔模式在三个带等级下的真实槽位和逐边端口；使用：综合目标容量回归；边界：物流机队速度仍属于明确未覆盖的运行条件。
func TestStationRealBeltCapacityMatrix(t *testing.T) {
	fixtures := loadMatrixFixtures(t)
	fixture := fixtures[len(fixtures)-1]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{3, 4} {
		for _, level := range []int{1, 2, 3} {
			t.Run(fmt.Sprintf("mode%d-Mk%d", mode, level), func(t *testing.T) {
				setting := fixture.Setting
				setting.BeltLv = level
				out, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: setting, Name: fixture.Name})
				if err != nil {
					t.Fatal(err)
				}
				assertGeneratedTransport(t, plan, mode, level, out)
				t.Logf("machines=%d buildings=%d lanes=%d inputs=%d area=%.0f", plan.MachineCount, out.BuildingCount, len(out.LogisticsSummary.Lanes), out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea)
			})
		}
	}
}

// TestConnectedGenerationIgnoresProductionArrayOrder 职责：核验规范图排序真正进入最终几何；使用：同一真实计划打乱后回归；边界：允许产线原索引映射改变，不允许改变建筑、路线或空间指标。
func TestConnectedGenerationIgnoresProductionArrayOrder(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: fixture.Setting})
	if err != nil {
		t.Fatal(err)
	}
	plan.Productions = append([]quant.Production(nil), plan.Productions...)
	for left, right := 0, len(plan.Productions)-1; left < right; left, right = left+1, right-1 {
		plan.Productions[left], plan.Productions[right] = plan.Productions[right], plan.Productions[left]
	}
	second, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: fixture.Setting})
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedTransport(t, plan, 1, 3, second)
	before, err := blueprint.Decode(first.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	after, err := blueprint.Decode(second.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Buildings, after.Buildings) || first.LogisticsSummary.FlowGraphDigest != second.LogisticsSummary.FlowGraphDigest || first.LogisticsSummary.Metrics != second.LogisticsSummary.Metrics {
		t.Fatal("产线数组顺序改变了实体布局、物流图或空间结果")
	}
}

// TestCancelledGenerationNeverPublishesBlueprint 职责：锁定取消请求不得发布已生成候选或旧串；使用：各模式生成生命周期回归；边界：计时只检查预先取消的快速响应，不声称覆盖游戏进程。
func TestCancelledGenerationNeverPublishesBlueprint(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for mode := 1; mode <= 6; mode++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		out, err := Generate(ctx, Request{Plan: plan, Mode: mode, Setting: fixture.Setting})
		if err == nil || out.Blueprint != "" || out.BuildingCount != 0 || out.LogisticsSummary != nil || out.RuleAudit != nil {
			t.Fatalf("取消模式 %d 发布了成功结果: %+v, %v", mode, out, err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("预先取消模式 %d 的响应过慢：%s", mode, elapsed)
		}
	}
}
