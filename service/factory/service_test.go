package factory

// 文件职责：验证服务层量化快照与返回结果的深拷贝隔离。
// 使用：factory 服务回归；边界：只调用纯量化引擎，不触发 Wails、数据库或文件对话框。

import (
	"testing"

	"DysonSphereBox/helper/quant"
)

// serviceRequest 职责：固定最小真实量化输入；使用：Solve 隔离测试；边界：不依赖前端默认状态。
func serviceRequest() quant.Request {
	fixed, spray, space := 2, false, 1.0
	return quant.Request{
		Dataset: "Vanilla",
		Targets: []quant.Target{{ItemID: 2003, RatePerMinute: 120}},
		Scheme:  quant.Scheme{CostWeight: quant.CostWeight{Space: &space}},
		Settings: quant.Settings{MiningSpeedMultiple: 1, FixedNum: &fixed, StackResearchLab: 1,
			ProliferateItself: &spray, MineralizeList: map[string]bool{}},
	}
}

// TestServiceSolveSnapshotIsolation 职责：证明服务缓存不会与 Wails 返回对象共享可变成员；使用：防止前端编辑响应污染后续生成；边界：仅断言可信计划字段。
func TestServiceSolveSnapshotIsolation(t *testing.T) {
	service := &Service{}
	response := service.Solve(serviceRequest())
	if response.Code != "OK" || response.Data == nil {
		t.Fatalf("Solve 失败: %+v", response.Status)
	}
	returned := response.Data
	service.mu.Lock()
	cached, ok := service.plans[returned.PlanID]
	service.mu.Unlock()
	if !ok {
		t.Fatal("Solve 成功后没有保存可信快照")
	}
	if cached.FlowGraph == nil || len(cached.Productions) == 0 {
		t.Fatal("缓存快照缺少真实生产结果")
	}
	if returned.FlowGraph == nil || len(returned.Productions) == 0 {
		t.Fatal("Solve 未返回完整计划")
	}
	// 模拟前端对响应进行编辑；这些改变不得穿透缓存。
	returned.Targets[0].RatePerMinute++
	returned.Productions[0].MachineCount++
	if len(returned.Productions[0].Inputs) > 0 {
		returned.Productions[0].Inputs[0].RatePerMinute++
	}
	if len(returned.Productions[0].Outputs) > 0 {
		returned.Productions[0].Outputs[0].RatePerMinute++
	}
	if len(returned.RawInputs) > 0 {
		returned.RawInputs[0].RatePerMinute++
	}
	if returned.FlowGraph != nil {
		if len(returned.FlowGraph.Nodes) > 0 {
			returned.FlowGraph.Nodes[0].ID = "client-forged"
		}
		if len(returned.FlowGraph.Edges) > 0 {
			returned.FlowGraph.Edges[0].RatePerMinute++
		}
		if len(returned.FlowGraph.Components) > 0 && len(returned.FlowGraph.Components[0].NodeIDs) > 0 {
			returned.FlowGraph.Components[0].NodeIDs[0] = "client-forged"
		}
		if len(returned.FlowGraph.ProductionOrder) > 0 {
			returned.FlowGraph.ProductionOrder[0]++
		}
	}
	for key := range returned.Amounts {
		returned.Amounts[key]++
		break
	}
	service.mu.Lock()
	cachedAfter, ok := service.plans[returned.PlanID]
	service.mu.Unlock()
	if !ok {
		t.Fatal("编辑返回对象后可信快照丢失")
	}
	if cachedAfter.Targets[0].RatePerMinute == returned.Targets[0].RatePerMinute {
		t.Fatal("目标速率通过返回对象污染缓存")
	}
	if cachedAfter.Productions[0].MachineCount == returned.Productions[0].MachineCount {
		t.Fatal("机器数量通过返回对象污染缓存")
	}
	if len(cachedAfter.FlowGraph.Nodes) > 0 && cachedAfter.FlowGraph.Nodes[0].ID == "client-forged" {
		t.Fatal("节点图通过返回对象污染缓存")
	}

	// 再次求解同一请求，确认缓存键不重复且仍为规范结果。
	second := service.Solve(serviceRequest())
	if second.Code != "OK" || second.Data == nil {
		t.Fatalf("重复 Solve 失败: %+v", second.Status)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.order) != 1 {
		t.Fatalf("同一 PlanID 不应重复占用缓存顺序: %v", service.order)
	}
	if len(service.plans) != 1 || service.plans[second.Data.PlanID].FlowGraph == nil {
		t.Fatal("重复求解未保留规范缓存图")
	}
}
