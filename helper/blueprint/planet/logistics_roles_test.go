package planet

// 文件职责：保护跨模块物流塔角色并核对制造边界重配；使用：整星装配回归；边界：仅制造边界拥有其指定塔的远程配置。

import (
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/industryplan"
)

// TestConfigureProductionPortsPreservesOtherModules 职责：防止制造配置清除矿区及光子塔供货；使用：多个模块共存回归；边界：相同物品不会扩大制造边界的配置范围。
func TestConfigureProductionPortsPreservesOtherModules(t *testing.T) {
	buildings := []blueprint.Building{
		stationObject(0, Position{}, 1001, 1, 1, true).Building,
		stationObject(1, Position{}, 1208, 1, 1, false).Building,
		stationObject(2, Position{}, 1001, 2, 1, false).Building,
		stationObject(3, Position{}, 1208, 2, 1, false).Building,
		stationObject(4, Position{}, 1101, 1, 2, false).Building,
	}
	configureRemoteGroup(&buildings[0], 1<<35)
	configureRemoteGroup(&buildings[1], 1<<36)
	mining := append([]int32(nil), buildings[0].Parameters...)
	photon := append([]int32(nil), buildings[1].Parameters...)
	// 制造塔上已无合同用途的旧远程槽仍应清空。
	buildings[4].Parameters[6], buildings[4].Parameters[7], buildings[4].Parameters[8], buildings[4].Parameters[9] = 1006, 1, 1, 10000
	plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1001, Kind: "export", PeerPlanetID: 102, RatePerMinute: 30},
		{ItemID: 1208, Kind: "export", PeerPlanetID: 102, RatePerMinute: 6},
		{ItemID: 1001, Kind: "import", PeerPlanetID: 103, RatePerMinute: 60},
		{ItemID: 1208, Kind: "import", PeerPlanetID: 103, RatePerMinute: 12},
		{ItemID: 1101, Kind: "export", PeerPlanetID: 102, RatePerMinute: 60},
	}}
	boundary := []build.BoundaryPort{
		{ItemID: 1001, Kind: "external-input", BuildingIndex: 2, RatePerMinute: 60},
		{ItemID: 1208, Kind: "external-input", BuildingIndex: 3, RatePerMinute: 12},
		{ItemID: 1101, Kind: "target-output", BuildingIndex: 4, RatePerMinute: 60},
	}
	actual, err := configureProductionPorts(plan, buildings, boundary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(buildings[0].Parameters, mining) || !reflect.DeepEqual(buildings[1].Parameters, photon) {
		t.Fatal("制造配置改写了已有矿区或光子塔的供货、补给或远程分组")
	}
	if buildings[2].Parameters[2] != 2 || buildings[3].Parameters[2] != 2 || buildings[4].Parameters[2] != 1 {
		t.Fatal("制造塔未按交付界面的进出口重新设置远程供需")
	}
	if buildings[4].Parameters[8] != 0 {
		t.Fatal("制造塔的旧远程槽未清除")
	}
	if !reflect.DeepEqual(actual, plan.Ports[2:]) {
		t.Fatalf("制造实际能力错误地包含其他模块或遗漏本模块：%+v", actual)
	}
}

// TestConfigureProductionPortsLocalAndRemoteRoles 职责：区分界面指定的远程流和本地交付；使用：制造塔角色重配；边界：实际速率不超出真实边界。
func TestConfigureProductionPortsLocalAndRemoteRoles(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		input    bool
		remote   int32
		reported bool
	}{
		{"import", true, 2, true},
		{"external", true, 0, true},
		{"mining", true, 0, false},
		{"export", false, 1, true},
		{"external-export", false, 0, true},
		{"delivery", false, 0, true},
		{"storage", false, 0, true},
		{"power-fuel", false, 0, false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			kind, local := "target-output", 1
			if tc.input {
				kind, local = "external-input", 2
			}
			previousRemote := 1
			if tc.remote == 1 {
				previousRemote = 2
			}
			tower := stationObject(0, Position{}, 1101, local, previousRemote, false).Building
			buildings := []blueprint.Building{tower}
			want := industryplan.Port{ItemID: 1101, Kind: tc.kind, PeerPlanetID: 102, RatePerMinute: 45}
			if tc.kind == "storage" {
				want.StorageItems = int(tower.Parameters[3]) + 1
			}
			actual, err := configureProductionPorts(industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{want}}, buildings,
				[]build.BoundaryPort{{ItemID: 1101, Kind: kind, BuildingIndex: 0, RatePerMinute: 30}})
			if err != nil {
				t.Fatal(err)
			}
			if buildings[0].Parameters[1] != int32(local) || buildings[0].Parameters[2] != tc.remote {
				t.Fatalf("%s 应保留本地角色并设置远程 %d，实际 %v", tc.kind, tc.remote, buildings[0].Parameters[:4])
			}
			if !tc.reported {
				if len(actual) != 0 {
					t.Fatal("制造边界不应代替矿机或电厂报告实际能力")
				}
				return
			}
			want.RatePerMinute = 30
			want.StorageItems = min(want.StorageItems, int(tower.Parameters[3]))
			if !reflect.DeepEqual(actual, []industryplan.Port{want}) {
				t.Fatalf("制造能力未受实际带路和塔容量约束：%+v", actual)
			}
		})
	}
}

// TestConfigureProductionPortsWithoutManufacturing 职责：确保空制造边界不清理其他模块；使用：纯采集或光子星回归；边界：没有制造塔就不产生制造实际端口。
func TestConfigureProductionPortsWithoutManufacturing(t *testing.T) {
	buildings := []blueprint.Building{
		stationObject(0, Position{}, 1001, 1, 1, false).Building,
		stationObject(1, Position{}, 1208, 1, 1, false).Building,
	}
	before := [][]int32{append([]int32(nil), buildings[0].Parameters...), append([]int32(nil), buildings[1].Parameters...)}
	actual, err := configureProductionPorts(industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1001, Kind: "export", RatePerMinute: 30},
		{ItemID: 1208, Kind: "export", RatePerMinute: 6},
	}}, buildings, nil)
	if err != nil || len(actual) != 0 {
		t.Fatalf("空制造边界产生了实际端口或错误：%+v，%v", actual, err)
	}
	for i, b := range buildings {
		if !reflect.DeepEqual(b.Parameters, before[i]) {
			t.Fatalf("空制造边界改写了第 %d 个非制造塔", i)
		}
	}
}

// TestConfigureProductionPortsLocalTotalsDoNotConsumeBoundary 职责：防止采矿和发电总量耗尽制造进出口；使用：本地分流与进口并存回归；边界：本地来源/消耗由独立模块证明。
func TestConfigureProductionPortsLocalTotalsDoNotConsumeBoundary(t *testing.T) {
	t.Run("采矿200外送100并进口100供制造200", func(t *testing.T) {
		buildings := []blueprint.Building{
			stationObject(0, Position{}, 1001, 1, 1, false).Building,
			stationObject(1, Position{}, 1001, 2, 0, false).Building,
		}
		miningTower := append([]int32(nil), buildings[0].Parameters...)
		plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
			{ItemID: 1001, Kind: "mining", RatePerMinute: 200},
			{ItemID: 1001, Kind: "export", PeerPlanetID: 102, RatePerMinute: 100},
			{ItemID: 1001, Kind: "import", PeerPlanetID: 103, RatePerMinute: 100},
		}}
		actual, err := configureProductionPorts(plan, buildings, []build.BoundaryPort{
			{ItemID: 1001, Kind: "external-input", BuildingIndex: 1, RatePerMinute: 200},
		}, plan.Ports[:2]...)
		if err != nil || !reflect.DeepEqual(actual, plan.Ports[2:]) || buildings[1].Parameters[2] != 2 || !reflect.DeepEqual(buildings[0].Parameters, miningTower) {
			t.Fatalf("本地采矿总量吞掉制造进口或改变矿区塔：回执 %+v，错误 %v", actual, err)
		}
	})
	t.Run("本地燃料总量不吞出口", func(t *testing.T) {
		buildings := []blueprint.Building{stationObject(0, Position{}, 1803, 1, 0, false).Building}
		plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
			{ItemID: 1803, Kind: "power-fuel", RatePerMinute: 200},
			{ItemID: 1803, Kind: "export", PeerPlanetID: 102, RatePerMinute: 100},
		}}
		actual, err := configureProductionPorts(plan, buildings, []build.BoundaryPort{
			{ItemID: 1803, Kind: "target-output", BuildingIndex: 0, RatePerMinute: 200},
		})
		if err != nil || !reflect.DeepEqual(actual, plan.Ports[1:]) || buildings[0].Parameters[2] != 1 {
			t.Fatalf("本地电厂总量吞掉制造出口：回执 %+v，错误 %v", actual, err)
		}
	})
}

// TestConfigureProductionPortsCreditsPriorPartialRate 职责：由制造补足已有矿区/光子的同一边界；使用：输出50+50和输入3+60回归；边界：返回最终合同值供覆盖合并，库存只增加实际新槽能力。
func TestConfigureProductionPortsCreditsPriorPartialRate(t *testing.T) {
	for _, tc := range []struct {
		name, kind, boundary string
		item, local, storage int
		oldRate, newRate     float64
		oldStorage           int
	}{
		{"已有出口50加制造50", "export", "target-output", 1101, 1, 30, 50, 50, 20},
		{"光子端进口3加制造60", "import", "external-input", 1209, 2, 20, 3, 60, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buildings := []blueprint.Building{stationObject(0, Position{}, tc.item, tc.local, 0, false).Building}
			buildings[0].Parameters[3] = int32(tc.storage)
			prior := industryplan.Port{ItemID: tc.item, Kind: tc.kind, PeerPlanetID: 102, RatePerMinute: tc.oldRate, StorageItems: tc.oldStorage}
			want := prior
			want.RatePerMinute += tc.newRate
			want.StorageItems = 100
			ports, err := configureProductionPorts(industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{want}}, buildings,
				[]build.BoundaryPort{{ItemID: tc.item, Kind: tc.boundary, BuildingIndex: 0, RatePerMinute: tc.newRate}}, prior)
			want.StorageItems = tc.oldStorage + tc.storage
			if err != nil || !reflect.DeepEqual(ports, []industryplan.Port{want}) {
				t.Fatalf("没有补足剩余份额或增加虚假库存：得到 %+v，期望 %+v，错误 %v", ports, want, err)
			}
			actual := industryplan.PlanetActual{Ports: []industryplan.Port{prior}}
			for _, port := range ports {
				addActualPort(&actual, port)
			}
			if !reflect.DeepEqual(actual.Ports, []industryplan.Port{want}) {
				t.Fatalf("覆盖合并后重复累计或丢失已有份额：%+v", actual.Ports)
			}
		})
	}
}

// TestConfigureProductionPortsSkipsCoveredPeer 职责：已满足的对端不再占用制造带速；使用：光子进口与另一制造进口并存；边界：不同Peer的回执不能互相抵扣。
func TestConfigureProductionPortsSkipsCoveredPeer(t *testing.T) {
	buildings := []blueprint.Building{stationObject(0, Position{}, 1209, 2, 0, false).Building}
	plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 3},
		{ItemID: 1209, Kind: "import", PeerPlanetID: 103, RatePerMinute: 60},
	}}
	ports, err := configureProductionPorts(plan, buildings, []build.BoundaryPort{
		{ItemID: 1209, Kind: "external-input", BuildingIndex: 0, RatePerMinute: 60},
	}, plan.Ports[0])
	if err != nil || !reflect.DeepEqual(ports, plan.Ports[1:]) {
		t.Fatalf("已满足的光子进口再次占制造带速，或错误抵扣另一对端：%+v，%v", ports, err)
	}
}

// TestConfigureProductionPortsSharesRealStorageOnce 职责：同一制造槽的库存只向多个合同分配一次；使用：已有库存与新槽共同承担；边界：速率仍分别满足，库存不足不能用流量通过冒充。
func TestConfigureProductionPortsSharesRealStorageOnce(t *testing.T) {
	buildings := []blueprint.Building{stationObject(0, Position{}, 1101, 1, 0, false).Building}
	buildings[0].Parameters[3] = 100
	prior := industryplan.Port{ItemID: 1101, Kind: "storage", RatePerMinute: 10, StorageItems: 40}
	plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1101, Kind: "storage", RatePerMinute: 30, StorageItems: 100},
		{ItemID: 1101, Kind: "delivery", RatePerMinute: 40, StorageItems: 100},
	}}
	ports, err := configureProductionPorts(plan, buildings, []build.BoundaryPort{
		{ItemID: 1101, Kind: "target-output", BuildingIndex: 0, RatePerMinute: 60},
	}, prior)
	want := append([]industryplan.Port(nil), plan.Ports...)
	want[1].StorageItems = 40
	if err != nil || !reflect.DeepEqual(ports, want) || ports[0].StorageItems+ports[1].StorageItems != prior.StorageItems+100 {
		t.Fatalf("同一制造库存被重复承诺：%+v，错误 %v", ports, err)
	}
}
