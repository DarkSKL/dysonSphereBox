package saveanalysis

// 文件职责：验证按星球选品、多产物交付及证据不足时的返回值。
// 使用：离线回归；边界：构造用例与实档独立核对分别记录。

import (
	"slices"
	"testing"
)

func TestProductionSelection(t *testing.T) {
	f := newFactoryLine()
	f.power, f.sign = []float32{0, 1, 1}, []byte{6, 0}
	refinery := circuitRecord{id: 1, entity: 1, recipe: 16, kind: 3, progress: 2400000, requires: []int64{1007}, served: []int64{0}, products: []int64{1120, 1114}, produced: []int64{20, 0}}
	f.machines = []circuitRecord{refinery, {id: 2, entity: 2, recipe: 999, active: true, products: []int64{1114}, produced: []int64{1}}}
	f.addBelt(1, 11, 0)
	f.sorters = []lineSorter{{101, 1, 11, 1120, false}, {102, 11, 20, 1120, false}}
	history := make([]int64, 14)
	history[1] = 5
	s := Snapshot{worldView: WorldView{Planets: []PlanetLink{{PlanetID: 104}, {PlanetID: 201}}}, gameVersion: productionRecipeVersion, lines: map[int64]*factoryLine{104: f}, items: map[int64][]planetRecord{
		1007: {{planetID: 104, totals: history}}, 999: {{planetID: 104, totals: make([]int64, 14)}}, 1114: {{planetID: 201, totals: history}},
	}}
	items, err := s.ProductionItems(104)
	if err != nil || !slices.Equal(items, []ProductionItem{{1007, 0, true}, {1114, 2, false}, {1120, 1, false}}) {
		t.Fatalf("选项 %+v %v", items, err)
	}
	for _, id := range []int64{1114, 1120} {
		r, err := s.ProductionLine(104, id)
		if err != nil || len(r.Machines) == 0 {
			t.Fatalf("查询 %d: %v", id, err)
		}
		if m := r.Machines[0]; m.Status != "output-blocked" || !m.RecipeMatched || m.Outputs[1].Buffered != 0 || m.Outputs[0].DeliveryLimit != 19 {
			t.Fatal(m)
		}
		if len(r.Findings) != 1 || r.Findings[0].ItemID != 1120 || r.Findings[0].CheckBelt != 1 {
			t.Fatal(r.Findings)
		}
	}
	h, err := s.ProductionLine(104, 1007)
	if err != nil || len(h.Machines) > 0 || h.Coverage == "" {
		t.Fatal(h, err)
	}
	for _, params := range [][2]int64{{104, 999}, {201, 1120}, {0, 1114}} {
		if _, err := s.ProductionLine(params[0], params[1]); err == nil {
			t.Fatal(params)
		}
	}
	for _, change := range []string{"version", "item order", "recipe kind"} {
		a := refinery
		version := productionRecipeVersion
		switch change {
		case "version":
			version = "other"
		case "item order":
			a.products = []int64{1114, 1120}
		case "recipe kind":
			a.kind = 4
		}
		m := f.diagnose(a, version, 1114)
		if m.RecipeMatched || m.Status != "undetermined" || m.Outputs[0].Batch != 0 {
			t.Fatal(change, m)
		}
	}
}

// TestProductionDeliveryLimits 覆盖三类交付规则的等号边界和双产物同时积压。
func TestProductionDeliveryLimits(t *testing.T) {
	f := newFactoryLine()
	f.sign = []byte{6}
	f.power = []float32{0, 1}
	for _, id := range []int64{1, 16, 50, 104} {
		p := productionRecipes[id]
		a := circuitRecord{id: 1, entity: 1, recipe: id, kind: p.kind, progress: p.ticks * 10000, requires: p.inputs, served: p.counts, products: p.products, produced: make([]int64, len(p.products))}
		m := f.diagnose(a, productionRecipeVersion, p.products[0])
		for i, o := range m.Outputs {
			a.produced[i] = o.DeliveryLimit
		}
		if got := f.diagnose(a, productionRecipeVersion, p.products[0]).Status; got != "undetermined" {
			t.Fatal(id, got)
		}
		for i := range a.produced {
			a.produced[i]++
		}
		m = f.diagnose(a, productionRecipeVersion, p.products[0])
		if m.Status != "output-blocked" || len(prioritizeCircuit(CircuitReport{Machines: []CircuitMachine{m}})) != len(p.products) {
			t.Fatal(id, m)
		}
	}
}

// TestProductionMixedBelt 核实同机的两种候选产物共用物理带段时，双方均标为共享。
func TestProductionMixedBelt(t *testing.T) {
	r := CircuitReport{Machines: []CircuitMachine{{EntityID: 1, Connections: []CircuitConnection{{Routes: []int{0, 1}}}}}, Routes: []CircuitRoute{
		{ItemID: 1120, Belts: []int64{10, 11}}, {ItemID: 1114, Belts: []int64{11, 12}},
	}}
	markSharedCircuitRoutes(&r)
	if !r.Routes[0].Shared || !r.Routes[1].Shared {
		t.Fatal(r.Routes)
	}
}

// TestProductionPeerBeyondCheckpoint 保证检查点上游分支中的邻机不会被报成检查点下游。
func TestProductionPeerBeyondCheckpoint(t *testing.T) {
	f := newFactoryLine()
	f.addBelt(1, 11, 2)
	f.addBelt(2, 12, 0)
	attached := map[int64][]lineSorter{11: {{101, 11, 30, 1114, false}}, 12: {{102, 12, 31, 1114, false}}}
	peer := CircuitMachine{EntityID: 30, Status: "output-blocked", Inputs: []CircuitInput{{ItemID: 1114}}}
	other := peer
	other.EntityID = 31
	r := CircuitReport{Neighbors: []CircuitMachine{peer, other}, Findings: []CircuitFinding{{Status: "output-blocked", CheckBelt: 2, ItemID: 1114}}}
	f.addNeighborEvidence(&r, map[int64][]int64{2: {1}}, attached)
	if !slices.Equal(r.Findings[0].RelatedBlockedEntities, []int64{31}) {
		t.Fatal(r.Findings)
	}
}
