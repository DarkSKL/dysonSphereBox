package saveanalysis

import (
	"slices"
	"testing"
)

// TestReceiverSupplySharedSlot 验证缺料接收机共用货槽时的根设备归属，配置候选不会被展开成实载来源。
func TestReceiverSupplySharedSlot(t *testing.T) {
	station := &layoutStationComponent{planetId: 42, id: 1, entityId: 71, storage: []*layoutStationStore{{itemId: 12, localLogic: 2}}}
	candidate := &layoutStationComponent{planetId: 42, id: 2, entityId: 72, storage: []*layoutStationStore{{itemId: 12, localLogic: 1}}}
	snap := &Snapshot{lines: map[int64]*factoryLine{42: {}}, stations: []*layoutStationComponent{station, candidate}}
	branches := inputBranches{report: SupplyReport{PlanetID: 42, ItemID: 12}, byEntity: map[int64]SupplyBranch{}}
	m := &CircuitMachine{Outputs: []ProductionOutput{{ItemID: 15}}}
	out := OutputDiagnosis{Receivers: []OutputReceiver{{EntityID: 40, Machine: m}, {EntityID: 41, Machine: m}, {EntityID: 43, Machine: m}}}
	for i, entity := range []int64{40, 41} {
		branches.byEntity[entity] = SupplyBranch{CheckEntity: entity, SourceSlots: []SupplySlot{station.slot(0)}}
		out.Checks = append(out.Checks, OutputCheck{EntityID: entity, Status: "consumer-input-missing", ItemID: 12, Receivers: []int{i}, RootEntities: []int64{int64(i + 1), 3}, Unresolved: i == 1})
	}
	w := chainWalk{s: snap, report: ProductionDiagnosis{PlanetID: 42}, inputs: map[[3]int64]inputBranches{{42, 15, 12}: branches}, catalog: map[[2]int64]map[int64]ChainNode{}}
	if err := w.linkReceiverSupply(&out); err != nil {
		t.Fatal(err)
	}
	if out.Receivers[2].SupplyNode != nil || out.Supply == nil || len(out.Supply.Nodes) != 3 {
		t.Fatal(out)
	}
	c := out.Supply.Checks[0]
	f := out.Supply.Nodes[c.Node].Freight
	if c.Status != "empty-slot" || f == nil || len(f.ModeCandidates) != 1 || len(f.Tasks) != 0 ||
		!slices.Equal(c.DirectEntities, []int64{40, 41}) || !slices.Equal(c.ReceiverEntities, []int64{40, 41}) ||
		!slices.Equal(c.RootEntities, []int64{1, 2, 3}) || !c.Shared || !c.Unresolved {
		t.Fatal(c, f)
	}
}

// TestChainImpactAcrossCycle 检查两条生产线在货槽汇合、运输环回时，目标实体和直接缺料实体各计一次。
func TestChainImpactAcrossCycle(t *testing.T) {
	missing := []ChainConstraint{{"input-missing", 10}}
	report := ProductionDiagnosis{Roots: []int{0, 1, 2}, Nodes: []ChainNode{
		{EntityID: 11, PlanetID: 104, Constraints: missing, Inputs: []ChainInput{{ItemID: 10, Sources: []int{3}, Shared: true}}},
		{EntityID: 12, PlanetID: 104, Constraints: missing, Inputs: []ChainInput{{ItemID: 10, Sources: []int{3}, Unresolved: true}}},
		{EntityID: 13, PlanetID: 104},
		{EntityID: 14, PlanetID: 104, Freight: &StationFreight{Slot: SupplySlot{ItemID: 10}}, LoadedSources: []int{4}},
		{EntityID: 15, PlanetID: 102, Freight: &StationFreight{Slot: SupplySlot{ItemID: 10}}, LoadedSources: []int{3}},
	}}

	impact := report.impacts()
	if !slices.Equal(impact[4].roots, []int64{11, 12}) || !impact[4].shared || !impact[4].unresolved {
		t.Fatal(impact)
	}
	report.prioritize()
	c := report.Checks[0]
	if c.Node != 3 || !slices.Equal(c.RootEntities, []int64{11, 12}) || !slices.Equal(c.DirectEntities, []int64{11, 12}) || c.Priority != 1 {
		t.Fatal(report.Checks)
	}
}

// TestChainMultiOutputIssue 同一台设备通过两种成品关联的设备组应合并到同一条建议。
func TestChainMultiOutputIssue(t *testing.T) {
	report := ProductionDiagnosis{Roots: []int{0, 1}, Nodes: []ChainNode{
		{PlanetID: 104, EntityID: 1, Constraints: []ChainConstraint{{"input-missing", 10}}, Inputs: []ChainInput{{ItemID: 10, Sources: []int{2}}}},
		{PlanetID: 104, EntityID: 2, Constraints: []ChainConstraint{{"input-missing", 20}}, Inputs: []ChainInput{{ItemID: 20, Sources: []int{3}}}},
		{PlanetID: 104, EntityID: 3, ProductID: 10, Constraints: []ChainConstraint{{Status: "low-power"}}},
		{PlanetID: 104, EntityID: 3, ProductID: 20, Constraints: []ChainConstraint{{Status: "low-power"}}},
	}}
	report.prioritize()
	if len(report.Checks) != 3 || report.Checks[0].Status != "low-power" || !slices.Equal(report.Checks[0].RootEntities, []int64{1, 2}) {
		t.Fatal(report.Checks)
	}
}
