package planet

// 文件职责：验证同一规划任务跨星复用制造模板；使用：真实装配与缓存隔离回归；边界：缓存只复用生产形状，星球合同和可修改参数必须独立。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// TestManufacturingTemplateCacheIsolation 职责：证明同硬件不同需求复用额定模板且不会串改合同；使用：机器数、设置和任务作用域分别核对失效；边界：真实生成和严格编解码，回执只认当前需求。
func TestManufacturingTemplateCacheIsolation(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	parent = search.WithControl(parent, search.New(2*time.Minute, nil))
	generations := 0
	parent = build.WithWorkflowFinish(parent, func(_ *workflow.RunResult, _ error) { generations++ })
	ctx := WithTemplateCache(parent)
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	base := moduleRegionTestPlan(t, ctx, 60)
	if len(base.Productions) != 1 || base.MachineCount != 1 {
		t.Fatalf("缓存回归应只有一台真实熔炉：%+v", base.Productions)
	}
	needPower := true
	inputFor := func(plan quant.Result, planet, peer int64, name string, importedItem, belt int) ProductionRequest {
		plan.PlanID = fmt.Sprintf("cache-planet-%d", planet)
		input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: planet, Plan: plan},
			Options: ProductionOptions{Name: name, Setting: build.Setting{NeedPower: &needPower, BeltLv: belt, InserterLv: 3}}}
		for _, raw := range plan.RawInputs {
			port := industryplan.Port{ItemID: raw.ItemID, Kind: "external", RatePerMinute: raw.RatePerMinute}
			if raw.ItemID == importedItem {
				port.Kind, port.PeerPlanetID = "import", peer
			}
			input.Plan.Ports = append(input.Plan.Ports, port)
		}
		for _, target := range plan.Targets {
			input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: target.ItemID, Kind: "export", PeerPlanetID: peer, RatePerMinute: target.RatePerMinute})
		}
		return input
	}
	assemble := func(ctx context.Context, input ProductionRequest) (Request, industryplan.PlanetActual) {
		t.Helper()
		before, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req := Request{PlanetID: input.Plan.PlanetID, Name: input.Options.Name}
		actual := industryplan.PlanetActual{}
		if err := appendManufacturing(ctx, &req, input, &actual, catalog); err != nil {
			t.Fatalf("星球%d制造装配失败：%v", req.PlanetID, err)
		}
		if len(actual.Ports) != len(input.Plan.Ports) {
			t.Fatalf("跨星模板残留或丢失合同：%+v", actual.Ports)
		}
		for _, want := range input.Plan.Ports {
			rate := 0.0
			for _, port := range actual.Ports {
				if port.ItemID == want.ItemID && port.Kind == want.Kind && port.PeerPlanetID == want.PeerPlanetID {
					rate += port.RatePerMinute
				}
			}
			if math.Abs(rate-want.RatePerMinute) > quant.FlowTolerance(rate, want.RatePerMinute) {
				t.Fatalf("缓存沿用了其他星球的端口：期望%+v，实际%+v", want, actual.Ports)
			}
			remote := int32(0)
			if want.Kind == "import" {
				remote = 2
			} else if want.Kind == "export" {
				remote = 1
			}
			found := false
			for _, object := range req.Objects {
				b := object.Building
				if b.ItemID != 2104 || len(b.Parameters) != 2048 {
					continue
				}
				if slot := stationSlot(&b, want.ItemID); slot >= 0 {
					found = true
					if b.Parameters[slot*6+2] != remote {
						t.Fatalf("星球%d物品%d应使用远程角色%d，实际%d", req.PlanetID, want.ItemID, remote, b.Parameters[slot*6+2])
					}
				}
			}
			if !found {
				t.Fatalf("合同没有真实塔槽：%+v", want)
			}
		}
		built, err := Build(ctx, req)
		if err != nil {
			t.Fatalf("缓存装配不能完成整星编码：%v", err)
		}
		doc, err := blueprint.Decode(built.Blueprint, true)
		if err != nil || doc.Summary.ItemCounts[2302] != input.Plan.Plan.MachineCount || doc.Name != input.Options.Name {
			t.Fatalf("严格解码、机器数或当前星球名称不符：%v，名称%q，设备%v", err, doc.Name, doc.Summary.ItemCounts)
		}
		if doc.Summary.ItemCounts[int16(2000+input.Options.Setting.BeltLv)] == 0 {
			t.Fatalf("缓存没有采用当前传送带设置%d：%v", input.Options.Setting.BeltLv, doc.Summary.ItemCounts)
		}
		after, err := json.Marshal(input)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("额定模板生成或合同分配改写原输入：%v", err)
		}
		return req, actual
	}
	snapshot := func(req Request, actual industryplan.PlanetActual) []byte {
		t.Helper()
		data, err := json.Marshal(struct {
			Objects []Object
			Ports   []industryplan.Port
		}{req.Objects, actual.Ports})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	firstInput := inputFor(base, 101, 201, "第一颗制造星", 1001, 1)
	first, firstActual := assemble(ctx, firstInput)
	if generations != 1 {
		t.Fatalf("首次简单模块应只生成一次，实际%d", generations)
	}
	// 首次需求仅60/min，缓存中的真实边界仍须具备该设备75/min满负荷净流。
	ctx.Value(templateCacheKey{}).(*sync.Map).Range(func(_, value any) bool {
		template := value.(moduleTemplate)
		if template.err != nil || template.generated.LogisticsSummary == nil {
			t.Fatalf("首份额定模板没有真实物流核验：%v", template.err)
		}
		for _, side := range []struct {
			kind  string
			rates []quant.ItemRate
		}{{"external-input", base.Productions[0].Capacity.Inputs}, {"target-output", base.Productions[0].Capacity.Outputs}} {
			for _, want := range side.rates {
				rate := 0.0
				for _, port := range template.generated.LogisticsSummary.ExternalPorts {
					if port.Kind == side.kind && port.ItemID == want.ItemID {
						rate += port.RatePerMinute
					}
				}
				if math.Abs(rate-want.RatePerMinute) > quant.FlowTolerance(rate, want.RatePerMinute) {
					t.Fatalf("缓存只忽略需求差异却没有按额定值生成带宽：%s 物品%d，%g/%g", side.kind, want.ItemID, rate, want.RatePerMinute)
				}
			}
		}
		return true
	})
	firstSnapshot := snapshot(first, firstActual)
	assemble(ctx, inputFor(base, 102, 302, "第二颗制造星", 1143, 1))
	if generations != 1 {
		t.Fatalf("仅修改星球、名称与供需合同却重新生成了模板：%d", generations)
	}
	if !bytes.Equal(firstSnapshot, snapshot(first, firstActual)) {
		t.Fatal("第二颗星球的塔角色配置改写了第一颗星球的实体或端口")
	}
	changedRate := moduleRegionTestPlan(t, ctx, 75)
	assemble(ctx, inputFor(changedRate, 103, 303, "改变产量", 1001, 1))
	if generations != 1 {
		t.Fatalf("60到75/min仍为相同硬件，却重新生成了模板：%d", generations)
	}
	moreMachines := moduleRegionTestPlan(t, ctx, 76)
	assemble(ctx, inputFor(moreMachines, 104, 304, "增加机器", 1001, 1))
	if generations != 2 {
		t.Fatalf("增加到两台机器后未重新生成：%d", generations)
	}
	assemble(ctx, inputFor(base, 105, 305, "改变传送带", 1001, 3))
	if generations != 3 {
		t.Fatalf("改变设备设置后未重新生成：%d", generations)
	}
	assemble(WithTemplateCache(parent), inputFor(base, 106, 306, "独立规划任务", 1001, 1))
	if generations != 4 {
		t.Fatalf("新任务错误复用了其他任务的模板：%d", generations)
	}
	if !bytes.Equal(firstSnapshot, snapshot(first, firstActual)) {
		t.Fatal("后续模板生成改写了第一颗星球的实体或端口")
	}
	t.Logf("6次真实装配仅生成%d次；同硬件60到75/min复用额定模板，新增机器、设置和新任务分别触发新模板，精确合同与严格编解码通过", generations)
}

// TestModuleMaterialPlanUsesValidatedCapacity 职责：核对联产循环配方的额定净流和设备毛流；使用：模板规范化、防伪及旧快照回归；边界：原方案及其 Capacity 引用不得被改写。
func TestModuleMaterialPlanUsesValidatedCapacity(t *testing.T) {
	ctx := context.Background()
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	const hydrogen = 1120
	const graphite = 1109
	const refinedOil = 1114
	recipeIndex := dataset.RecipeByID[58]
	choice := 0
	for i, candidate := range dataset.Choices[dataset.ByID[hydrogen].Name] {
		if candidate == recipeIndex {
			choice = i + 1
		}
	}
	if choice == 0 {
		t.Fatal("缺少X射线裂解真实配方")
	}
	external := map[string]bool{}
	for _, item := range dataset.Items {
		if item.ID != hydrogen {
			external[item.Name] = true
		}
	}
	plan, err := quant.Solve(ctx, quant.Request{Targets: []quant.Target{{ItemID: hydrogen, RatePerMinute: 10}},
		Scheme: quant.Scheme{ItemRecipeChoices: map[string]int{dataset.ByID[hydrogen].Name: choice}}, Settings: quant.Settings{MineralizeList: external}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Productions) != 1 || plan.Productions[0].RecipeID != 58 || plan.Productions[0].MachineCount != 1 {
		t.Fatalf("回归样例不是一台真实X射线裂解：%+v", plan.Productions)
	}
	before, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	desired := plan.Productions[0]
	material, err := moduleMaterialPlan(ctx, plan, desired, 0)
	if err != nil {
		t.Fatal(err)
	}
	full := material.Productions[0]
	closeRate := func(actual, expected float64) bool { return math.Abs(actual-expected) < 1e-10 }
	if full.Capacity == desired.Capacity || full.ExactMachineCount != 1 || full.RoundingMargin != 0 || !closeRate(full.OutputCount, 15) || full.Capacity.Utilization != 1 || full.PowerMW != full.Capacity.PowerMW || material.PowerMW != full.PowerMW {
		t.Fatalf("额定模板未独立转换为完整机器能力：%+v", full)
	}
	rate := func(rates []quant.ItemRate, item int) float64 {
		value := 0.0
		for _, flow := range rates {
			if flow.ItemID == item {
				value += flow.RatePerMinute
			}
		}
		return value
	}
	if !closeRate(rate(full.Inputs, refinedOil), 15) || rate(full.Inputs, hydrogen) != 0 || !closeRate(rate(full.Outputs, hydrogen), 15) || !closeRate(rate(full.Outputs, graphite), 15) {
		t.Fatalf("模板净流误用了循环毛流：输入%+v，输出%+v", full.Inputs, full.Outputs)
	}
	if !closeRate(rate(full.Capacity.GrossInputs, hydrogen), 30) || !closeRate(rate(full.Capacity.GrossOutputs, hydrogen), 45) {
		t.Fatalf("设备端口丢失循环氢的满负荷毛流：%+v", full.Capacity)
	}
	if rate(material.RawInputs, hydrogen) != 0 || !closeRate(rate(material.RawInputs, refinedOil), 15) || len(material.Targets) != 2 || material.FlowGraph == nil {
		t.Fatalf("模块外部边界未保留额定净流及联产：原料%+v，目标%+v", material.RawInputs, material.Targets)
	}
	if _, err := quant.ValidatedProductionCapacity(dataset, full); err != nil {
		t.Fatalf("转换后无法通过权威能力核验：%v", err)
	}
	tampered := desired
	fake := *desired.Capacity
	fake.OutputPerMinute *= 2
	tampered.Capacity = &fake
	if _, err := moduleMaterialPlan(ctx, plan, tampered, 0); err == nil {
		t.Fatal("伪造的Capacity绕过了原型复核")
	}
	legacy := desired
	legacy.Capacity = nil
	old, err := moduleMaterialPlan(ctx, plan, legacy, 0)
	if err != nil || old.Productions[0].Capacity != nil || old.Productions[0].OutputCount != desired.OutputCount || old.PowerMW != desired.PowerMW {
		t.Fatalf("旧快照被补写额定流或改变原需求：%v，%+v", err, old)
	}
	after, err := json.Marshal(plan)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("模板规范化改写了原需求或Capacity：%v", err)
	}
}
