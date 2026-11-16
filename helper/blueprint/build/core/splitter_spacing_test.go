package build

// 文件职责：验证普通和密铺分流器共用构造但保留各自真实占位间距；使用：大厂输入树回归；边界：不放宽完整带段、槽向和选择体积规则。

import (
	"context"
	"testing"
)

// TestStandardSplitterPortSpacing 职责：三种真实模型四朝向核对普通接头及密铺原位置；使用：独立物理规则和构造证据；边界：旧密铺接头用于普通模式必须能复现失败。
func TestStandardSplitterPortSpacing(t *testing.T) {
	ctx := context.Background()
	for _, model := range []int16{38, 39, 40} {
		for _, yaw := range []float32{0, 90, 180, 270} {
			policy, err := newAssemblyPolicy(1, densityStandard)
			if err != nil {
				t.Fatal(err)
			}
			b := &builder{models: map[int]int16{2001: 35, 2020: model}, setting: Setting{BeltLv: 1}, policy: policy, owner: -1}
			gate, err := b.buildSplitterGate(ctx, splitterGateSpec{model: model, yaw: yaw, input: 0, outputs: []int8{1, 2, 3}, origin: vector{x: 10, y: 10}})
			if err != nil {
				t.Fatal(err)
			}
			alive := map[*placed]bool{}
			for _, p := range b.all {
				alive[p] = true
			}
			if err := b.validateSplitterGateAssembly(gate, alive); err != nil {
				t.Fatal(err)
			}
			records, _, _, err := b.records()
			if err != nil {
				t.Fatal(err)
			}
			enc := newPlanarRuleEncoding()
			for _, kind := range []RuleKind{RulePastedSlots, RuleBeltDistance, RuleBeltCrossings, RuleBeltBuildingOccupancy, RuleSplitterClearance} {
				if _, err := executePlanetRule(ctx, kind, Request{Density: densityStandard}, enc, records); err != nil {
					t.Fatalf("model%d yaw%.0f %s: %v", model, yaw, kind, err)
				}
			}
			profile, err := loadSplitterPortProfile(model)
			if err != nil {
				t.Fatal(err)
			}
			for slot, link := range gate.links {
				for i, p := range splitterGatePortOffsets(profile, slot, yaw, link.input, 0, .36) {
					p.x += gate.gate.a.x
					p.y += gate.gate.a.y
					link.nodes[i].a = p
					link.nodes[i].b = p
				}
			}
			old, _, _, err := b.records()
			if err != nil {
				t.Fatal(err)
			}
			if err := enc.validateBeltCrossings(ctx, old); err == nil {
				t.Fatalf("model%d yaw%.0f old dense spacing unexpectedly satisfies ordinary spans", model, yaw)
			}
			b.policy, err = newAssemblyPolicy(1, densityDense)
			if err != nil {
				t.Fatal(err)
			}
			if err := b.validateSplitterGateAssembly(gate, alive); err != nil {
				t.Fatalf("dense original layout changed: %v", err)
			}
		}
	}
}
