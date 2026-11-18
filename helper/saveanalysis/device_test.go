package saveanalysis

// 文件职责：验证特殊组件的运行门槛与检查位置；使用：离线回归；边界：构造状态与实档验收分开记录。

import (
	"slices"
	"testing"
)

func TestFractionatorGates(t *testing.T) {
	f := newFactoryLine()
	for _, tc := range []struct {
		name                string
		input, product, ret int64
		power               float32
		working             bool
		want                []deviceIssue
	}{
		{"加工", 2, 19, 19, 1, true, nil}, {"缺料", 0, 0, 0, 1, false, []deviceIssue{{"input-missing", 1120}}},
		{"两个出口同时满", 2, 20, 20, 1, false, []deviceIssue{{"output-blocked", 1121}, {"return-blocked", 1120}}},
		{"供电优先", 2, 0, 0, 0.09, true, []deviceIssue{{"low-power", 0}}},
	} {
		d := deviceRecord{working: tc.working, DeviceState: DeviceState{Kind: "fractionator", FluidID: 1120, ProductID: 1121, FluidInput: tc.input, Buffered: tc.product, Capacity: 20, FluidReturn: tc.ret, ReturnCapacity: 20, Power: tc.power}}
		v, issues := f.deviceState(d, productionRecipeVersion)
		if !slices.Equal(issues, tc.want) || (tc.want == nil && v.Status != "processing") {
			t.Fatal(tc.name, v, issues)
		}
	}
}
func TestMinerAndLabEvidence(t *testing.T) {
	f := newFactoryLine()
	d := deviceRecord{minerType: 2, speed: 10000, speedDamper: 1, DeviceState: DeviceState{Kind: "miner", ProductID: 1001, Power: 1, Cycle: 1200, Progress: 1199, Buffered: 50, Veins: []int64{2}, ResourceRemaining: 400}}
	v, _ := f.deviceState(d, productionRecipeVersion)
	if v.Status != "processing" {
		t.Fatal(v)
	}
	d.Progress = 1200
	v, _ = f.deviceState(d, productionRecipeVersion)
	if v.Status != "output-blocked" {
		t.Fatal(v)
	}
	d.Veins = nil
	v, _ = f.deviceState(d, productionRecipeVersion)
	if v.Status != "no-source" {
		t.Fatal(v)
	}
	lab := deviceRecord{speed: 15000, requires: []int64{1209, 1305}, served: []int64{2, 1}, products: []int64{6005}, DeviceState: DeviceState{Kind: "matrix-lab", RecipeID: 102, ProductID: 6005, Power: 1, Sign: 6, Progress: 14400000, Buffered: 18}}
	v, _ = f.deviceState(lab, productionRecipeVersion)
	if v.Status != "undetermined" || v.Capacity != 20 {
		t.Fatal(v)
	}
	lab.Buffered++
	v, _ = f.deviceState(lab, productionRecipeVersion)
	if v.Status != "output-blocked" {
		t.Fatal(v)
	}
	v, _ = f.deviceState(lab, "unknown")
	if v.Status != "undetermined" {
		t.Fatal(v)
	}
}
func TestResearchPointsAndStack(t *testing.T) {
	f := newFactoryLine()
	f.currentTech = 5904
	f.researchPending = true
	d := deviceRecord{DeviceState: DeviceState{Kind: "research-lab", ComponentID: 7, EntityID: 100, TechID: 5904, Power: 1, Sign: 6, MatrixPoints: []int64{20, 20, 19, 20, 0, 0}}}
	v, issues := f.deviceState(d, productionRecipeVersion)
	if v.ProductID != 0 || !slices.Equal(issues, []deviceIssue{{"research-input-missing", 6003}, {"research-input-missing", 6005}}) {
		t.Fatal(v, issues)
	}
	lower := d
	lower.NextLabID = 7
	lower.ComponentID = 2
	lower.EntityID = 90
	f.devices = []deviceRecord{d, lower}
	f.sorters = []lineSorter{{101, 89, 90, 6003, false}}
	c := newDeviceGraph(f).check(d, issues[0], &DeviceReport{})
	if c.CheckEntity != 90 || !c.Shared || c.Evidence != "sorter" {
		t.Fatal(c)
	}
	f.devices[1].TechID = 1
	c = newDeviceGraph(f).check(d, issues[0], &DeviceReport{})
	if c.CheckEntity != 100 || c.Evidence != "unresolved" {
		t.Fatal(c)
	}
	f.researchPending = false
	v, _ = f.deviceState(d, productionRecipeVersion)
	if v.Status != "undetermined" {
		t.Fatal(v)
	}
}
