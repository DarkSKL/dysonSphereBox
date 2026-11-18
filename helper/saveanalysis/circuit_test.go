package saveanalysis

import "testing"

// TestCircuitCycleState 核实保存时刻的状态、投料及交付门槛，避免把低库存直接解释为停工。
func TestCircuitCycleState(t *testing.T) {
	f := newFactoryLine()
	f.power = []float32{0, 1}
	for _, tc := range []struct {
		name                     string
		sign                     byte
		active                   bool
		iron, progress, produced int64
		want                     string
	}{
		{"本批加工中", 0, true, 0, 300000, 19, "processing"},
		{"差一件铁", 6, false, 1, 0, 0, "input-missing"},
		{"到交付门槛仍有空间", 6, false, 2, 600000, 18, "undetermined"},
		{"完成本批但无法交付", 6, false, 2, 600000, 19, "output-blocked"},
		{"缓冲高但本批未到交付点", 6, false, 2, 590000, 19, "undetermined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.sign = []byte{tc.sign}
			a := circuitRecord{entity: 1, recipe: 50, active: tc.active, progress: tc.progress, kind: 4, products: []int64{1301}, produced: []int64{tc.produced}, requires: []int64{1101, 1104}, served: []int64{tc.iron, 1}}
			if got := f.diagnose(a, productionRecipeVersion, 1301).Status; got != tc.want {
				t.Fatalf("状态 %s，预期 %s", got, tc.want)
			}
		})
	}
}

// TestCircuitSharedBelt 构造带环和跨带分拣，验证只沿符合过滤条件的方向追踪，共享接收端只访问一次。
func TestCircuitSharedBelt(t *testing.T) {
	f := newFactoryLine()
	f.addBelt(1, 11, 2)
	f.addBelt(2, 12, 1)
	f.addBelt(3, 13, 0)
	links := []lineSorter{{101, 12, 13, 1301, false}, {102, 13, 21, 1301, false}, {103, 12, 22, 1101, false}, {104, 13, 23, 0, false}}
	adj := map[int64][]lineSorter{}
	for _, s := range links {
		adj[s.pick] = append(adj[s.pick], s)
		adj[s.drop] = append(adj[s.drop], s)
	}
	r := f.trace(1, "out", 1301, map[int64][]int64{1: {2}, 2: {1}}, adj)
	if !r.Shared || len(r.Belts) != 3 || len(r.Transfers) != 1 || len(r.Endpoints) != 2 || r.Endpoints[0].EntityID != 21 || r.Endpoints[1].EntityID != 23 {
		t.Fatalf("连接错误 %+v", r)
	}
}
