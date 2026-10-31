package factory

// 文件职责：验证可信量化快照隔离、派生图兼容与离线方案规范化。
// 使用：go test ./helper/factory；边界：仅使用内置真实配方，不访问个人资源、数据库或游戏世界。

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
)

// snapshotRequest 职责：固定真实配方输入；使用：快照及导出测试共用；边界：参数显式记录，不依赖前端设置。
func snapshotRequest() quant.Request {
	fixed, spray, space := 2, false, 1.0
	return quant.Request{
		Dataset: "Vanilla",
		Targets: []quant.Target{{ItemID: 2003, RatePerMinute: 120}},
		Scheme:  quant.Scheme{CostWeight: quant.CostWeight{Space: &space, Power: 0}},
		Settings: quant.Settings{MiningSpeedMultiple: 1, FixedNum: &fixed, StackResearchLab: 1,
			ProliferateItself: &spray, MineralizeList: map[string]bool{}},
	}
}

// snapshotPlan 职责：获取真实求解基准；使用：本文件测试；边界：求解失败立即终止，避免假数据掩盖兼容错误。
func snapshotPlan(t *testing.T) quant.Result {
	t.Helper()
	plan, err := quant.Solve(context.Background(), snapshotRequest())
	if err != nil {
		t.Fatal(err)
	}
	if plan.FlowGraph == nil || len(plan.FlowGraph.Edges) == 0 || len(plan.Productions) == 0 {
		t.Fatal("真实求解没有完整物料图")
	}
	return plan
}

// TestCloneQuantPlanDeepIsolation 职责：证明每层嵌套切片、映射和流图均与源隔离；使用：服务缓存回归；边界：只改变副本。
func TestCloneQuantPlanDeepIsolation(t *testing.T) {
	original := snapshotPlan(t)
	original.ExternalInputItemIDs = []int{1001}
	original.Leftovers = []quant.ItemRate{{ItemID: 1120, RatePerMinute: 10}}
	original.KeyItems = []int{1120}
	original.Warnings = []string{"原提示"}
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := CloneQuantPlan(original)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, copied) {
		t.Fatal("复制改变了原量化字段")
	}
	copied.Targets[0].RatePerMinute++
	copied.Productions[0].MachineCount++
	copied.Productions[0].TargetItemIDs[0]++
	copied.Productions[0].Inputs[0].RatePerMinute++
	copied.Productions[0].Outputs[0].RatePerMinute++
	copied.RawInputs[0].RatePerMinute++
	copied.ExternalInputItemIDs[0]++
	copied.Leftovers[0].RatePerMinute++
	copied.KeyItems[0]++
	copied.Warnings[0] = "已修改"
	for key := range copied.Amounts {
		copied.Amounts[key]++
		break
	}
	copied.FlowGraph.Nodes[0].ID = "篡改节点"
	copied.FlowGraph.Edges[0].RatePerMinute++
	copied.FlowGraph.Components[0].NodeIDs[0] = "篡改组件"
	copied.FlowGraph.ProductionOrder[0]++
	copied.FlowGraph.Balances[0].DemandPerMinute++
	after, err := json.Marshal(original)
	if err != nil || string(before) != string(after) {
		t.Fatalf("修改副本污染了可信源: %v", err)
	}
	if _, err := CloneQuantPlan(quant.Result{PowerMW: math.NaN()}); err == nil {
		t.Fatal("非有限快照不应成为共享引用降级")
	}
}

// TestMatchQuantSnapshotDerivedGraphCompatibility 职责：兼容遗漏或篡改派生图，但拒绝真实量化字段改动；使用：可信生成边界回归；边界：通过不代表允许直接使用候选图。
func TestMatchQuantSnapshotDerivedGraphCompatibility(t *testing.T) {
	trusted := snapshotPlan(t)
	for _, variant := range []string{"missing", "forged"} {
		candidate, err := CloneQuantPlan(trusted)
		if err != nil {
			t.Fatal(err)
		}
		if variant == "missing" {
			candidate.FlowGraph = nil
			candidate.ExternalInputItemIDs = nil
		} else {
			candidate.FlowGraph = &quant.FlowGraph{Digest: "forged", Edges: []quant.FlowEdge{{ID: "negative", RatePerMinute: -999}}}
			candidate.ExternalInputItemIDs = []int{999999}
		}
		if !MatchQuantSnapshot(trusted, candidate) {
			t.Fatalf("派生图或外供标记 %s 不应改变可信量化匹配", variant)
		}
	}
	mutations := []struct {
		name   string
		change func(*quant.Result)
	}{
		{"目标", func(p *quant.Result) { p.Targets[0].RatePerMinute++ }},
		{"机器", func(p *quant.Result) { p.Productions[0].MachineCount++ }},
		{"输入", func(p *quant.Result) { p.Productions[0].Inputs[0].RatePerMinute++ }},
		{"输出", func(p *quant.Result) { p.Productions[0].Outputs[0].RatePerMinute++ }},
		{"原料", func(p *quant.Result) { p.RawInputs[0].RatePerMinute++ }},
		{"数据版本", func(p *quant.Result) { p.DataVersion = "different" }},
		{"方案ID", func(p *quant.Result) { p.PlanID = "different" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate, err := CloneQuantPlan(trusted)
			if err != nil {
				t.Fatal(err)
			}
			mutation.change(&candidate)
			if MatchQuantSnapshot(trusted, candidate) {
				t.Fatal("修改真实量化字段却匹配可信快照")
			}
		})
	}
}

// TestMarshalWorkspaceRestoresCanonicalGraph 职责：验证旧方案导出重建同一规范图；使用：离线方案回归；边界：导出不信任客户端自造边。
func TestMarshalWorkspaceRestoresCanonicalGraph(t *testing.T) {
	trusted := snapshotPlan(t)
	for _, variant := range []string{"missing", "forged"} {
		t.Run(variant, func(t *testing.T) {
			candidate, err := CloneQuantPlan(trusted)
			if err != nil {
				t.Fatal(err)
			}
			candidate.FlowGraph = nil
			if variant == "forged" {
				candidate.FlowGraph = &quant.FlowGraph{Digest: "client-forged", Nodes: []quant.FlowNode{{ID: "not-real"}}}
			}
			text, err := MarshalWorkspace(contract.WorkspaceRequest{Name: "真实蓝图回归", Mode: 1, Request: snapshotRequest(), Plan: candidate})
			if err != nil {
				t.Fatal(err)
			}
			var saved struct {
				// FormatVersion 职责：接收版本证据；使用：导出断言；边界：不执行兼容迁移。
				FormatVersion int `json:"formatVersion"`
				contract.WorkspaceRequest
			}
			if err := json.Unmarshal([]byte(text), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.FormatVersion != 1 || saved.Plan.FlowGraph == nil || !reflect.DeepEqual(saved.Plan.FlowGraph, trusted.FlowGraph) {
				t.Fatal("导出没有恢复后端规范图")
			}
			if !MatchQuantSnapshot(trusted, saved.Plan) {
				t.Fatal("导出改变了真实量化账")
			}
		})
	}
	trusted.Targets[0].RatePerMinute++
	_, err := MarshalWorkspace(contract.WorkspaceRequest{Mode: 1, Request: snapshotRequest(), Plan: trusted})
	var failure *blueprint.Error
	if !errors.As(err, &failure) || failure.Code != "GEN_E_PLAN_STALE" {
		t.Fatalf("目标被篡改应明确失败，got %v", err)
	}
}
