package factory

// 文件职责：验证方案记录可复核及配置原子写入；使用：go test；边界：只用临时文件，不测试原生保存框。

import (
	"DysonSphereBox/helper/blueprint/build"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestWorkspaceRecord 职责：核对目标/手选配置/结果/布局完整保存与篡改拒绝；使用：方案导出回归；边界：不伪造 Windows 对话框证据。
func TestWorkspaceRecord(t *testing.T) {
	request := quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 60}}}
	plan, err := quant.Solve(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	req := contract.WorkspaceRequest{Name: "蓝糖方案", Request: request, Plan: plan, Setting: build.Setting{BeltLv: 3}, Mode: 3}
	raw, err := MarshalWorkspace(req)
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		FormatVersion int `json:"formatVersion"`
		contract.WorkspaceRequest
	}
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		t.Fatal(err)
	}
	if record.FormatVersion != 1 || record.Plan.PlanID != plan.PlanID || record.Request.Targets[0].RatePerMinute != 60 || record.Setting.BeltLv != 3 {
		t.Fatal("lost workspace state")
	}
	target := filepath.Join(t.TempDir(), "plan.json")
	if err := ExportJSON(target, raw); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if string(content) != raw {
		t.Fatal("export corruption")
	}
	req.Plan.MachineCount++
	if _, err := MarshalWorkspace(req); err == nil {
		t.Fatal("tampered result accepted")
	}
	if err := ExportJSON(filepath.Join(t.TempDir(), "app.db"), raw); err == nil {
		t.Fatal("non-JSON accepted")
	}
}
