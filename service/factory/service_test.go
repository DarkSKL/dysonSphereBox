package factory

// 文件职责：验证真实服务 JSON 边界和方案防篡改；使用：go test；边界：不初始化真实数据库或游戏目录。

import (
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/environment"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"encoding/json"
	"math"
	"testing"
)

// TestBridgeRoundTrip 职责：模拟 Wails 序列化后生成和反算；使用：服务联动回归；边界：不模拟桌面窗口。
func TestBridgeRoundTrip(t *testing.T) {
	s := &Service{}
	result := s.Solve(quant.Request{Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 60}}})
	if result.Code != "OK" {
		t.Fatal(result.Status)
	}
	raw, _ := json.Marshal(result.Data)
	var plan quant.Result
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	generated := s.Generate(build.Request{Plan: plan, Mode: 3})
	if generated.Code != "OK" {
		t.Fatal(generated.Status)
	}
	legacy, err := environment.ParseBlueprintCode(generated.Data.Blueprint)
	if err != nil || legacy.Summary.BuildingCount != generated.Data.BuildingCount {
		t.Fatal("existing environment reader regression", err)
	}
	inspection := s.Inspect(contract.InspectRequest{Blueprint: generated.Data.Blueprint, Dataset: "Vanilla"})
	if inspection.Code != "OK" || len(inspection.Data.Machines) != 9 {
		t.Fatal(inspection)
	}
	outputs := map[int]float64{}
	for _, out := range inspection.Data.Outputs {
		outputs[out.ItemID] = out.RatePerMinute
	}
	if math.Abs(outputs[6001]-60) > 1e-7 {
		t.Fatal(outputs)
	}
	plan.Productions[0].MachineCount++
	if got := s.Generate(build.Request{Plan: plan, Mode: 3}); got.Code != "GEN_E_PLAN_STALE" {
		t.Fatal(got.Status)
	}
	s.busy.Store(true)
	if got := s.Generate(build.Request{Plan: plan, Mode: 3}); got.Code != "GEN_E_BUSY" {
		t.Fatal(got.Status)
	}
}

// TestCacheEviction 职责：验证旧方案不会在缓存淘汰后继续执行；使用：多次量化；边界：不访问任何文件。
func TestCacheEviction(t *testing.T) {
	s := &Service{}
	var first quant.Result
	for i := 1; i <= 17; i++ {
		r := s.Solve(quant.Request{Targets: []quant.Target{{ItemID: 6001, RatePerMinute: float64(i)}}})
		if r.Code != "OK" {
			t.Fatal(r.Status)
		}
		if i == 1 {
			first = *r.Data
		}
	}
	if len(s.plans) != 16 {
		t.Fatal(len(s.plans))
	}
	if r := s.Generate(build.Request{Plan: first, Mode: 1}); r.Code != "GEN_E_PLAN_STALE" {
		t.Fatal(r.Status)
	}
}
