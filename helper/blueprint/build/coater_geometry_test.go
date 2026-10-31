package build

// 文件职责：回归反向逻辑输入下的供剂实体、幂等性和真实喷涂计划。
// 使用：局部喷涂修复及各运输模式回归；边界：不把喷涂机器存在当作已验证实际供剂产能。

import (
	"context"
	"fmt"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestCoaterSupplyUsesPhysicalOrderAndGroundInput 职责：保证底侧原料反转不会生成空干线，并登记可连接的供剂入口；使用：旧panic正反回归；边界：供剂不可复制或以空机器行代替实体。
func TestCoaterSupplyUsesPhysicalOrderAndGroundInput(t *testing.T) {
	setting, err := normalizeSetting(Setting{BeltLv: 3, InserterLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	l := &line{info: quant.Production{IncreaseLv: 3, IncreasePoints: 4, ProductionMode: 1}, machines: []*placed{b.make(2303, 8, 4, 0)}, coaters: []*placed{b.make(2313, 2, 2, 0), b.make(2313, 2, 1, 0), b.make(2313, 2, 0, 0)}}
	if err := b.coatSupply([]*line{l}); err != nil {
		t.Fatal(err)
	}
	if l.coaterInput == nil || l.coaterInput.a.z != 0 || len(b.coaterLinks[1143]) != 1 || l.coaterInput.record.Parameters[0] != 1143 {
		t.Fatal("必须生成地面供剂入口并登记真实增产剂")
	}
	registered := false
	for _, p := range l.all {
		registered = registered || p == l.coaterInput
	}
	if !registered {
		t.Fatal("局部占位列表必须包含供剂首带")
	}
	current := l.coaterInput
	seen := map[*placed]bool{}
	for current.output != nil {
		if seen[current] {
			t.Fatal("供剂轨出现循环")
		}
		seen[current] = true
		current = current.output
	}
	if current.a.x != 1 || current.a.y != 2 || current.a.z != 1 {
		t.Fatalf("干线没有覆盖所有物理喷涂高度：%+v", current.a)
	}
	count, warnings := len(b.all), len(b.warnings)
	if err := b.coatSupply([]*line{l}); err != nil {
		t.Fatal(err)
	}
	if len(b.all) != count || len(b.coaterLinks[1143]) != 1 || len(b.warnings) != warnings {
		t.Fatal("重复调用不得复制供剂干线")
	}
	broken := &line{recipe: quant.Recipe{ID: 92}, coaters: l.coaters}
	if err := b.coatSupply([]*line{broken}); err == nil {
		t.Fatal("缺少真实机器不得生成喷涂网络")
	}
	if _, err := b.buildLine(quant.Production{}, quant.Recipe{}, 0, 3, 0, true); err == nil {
		t.Fatal("空机器行必须在装配前明确失败")
	}
}

// TestProliferatedRealTransportModes 职责：用真实92配方的III级喷涂计划验证五种运输模式；使用：附加规范输入、局部几何和编码回归；边界：实际供剂和游戏科技仍需独立验收。
func TestProliferatedRealTransportModes(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	fixture.Setting.NeedCoaterNet = true
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	ri := dataset.RecipeByID[92]
	fixture.Request.Scheme.SchemeForRecipe[ri].ProductionMode = 1
	fixture.Request.Scheme.SchemeForRecipe[ri].IncreasePoints = 4
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: fixture.Setting, Name: "spray3-recipe92"})
			if err != nil {
				t.Fatal(err)
			}
			assertGeneratedTransport(t, plan, mode, fixture.Setting.BeltLv, out)
			decoded, err := blueprint.Decode(out.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if out.RuleAudit == nil || out.LogisticsSummary == nil || len(decoded.Buildings) != out.BuildingCount {
				t.Fatal("缺少最终规则或严格建筑证据")
			}
			sprayRate := 0.
			for _, lane := range out.LogisticsSummary.Lanes {
				if lane.ItemID == 1143 && lane.FlowKind == "internal" {
					sprayRate += lane.RatePerMinute
				}
				if lane.RatePerMinute > lane.CapacityPerMinute+1e-6 {
					t.Fatal("喷涂计划 lane超额")
				}
			}
			if sprayRate < 3.3333333 {
				t.Fatalf("规范增产剂输入被遗漏：%.9f/min", sprayRate)
			}
			machines := 0
			for _, binding := range out.Bindings {
				machines += len(binding.MachineIndices)
			}
			if machines != plan.MachineCount {
				t.Fatalf("真实机器数量变化：%d!=%d", machines, plan.MachineCount)
			}
		})
	}
}
