package facts

// 文件职责：核对原版能源事实的读取契约；使用：能源规划回归；边界：只验证已提取额定值，不模拟游戏运行。

import "testing"

// TestVanillaEnergyFacilities 职责：覆盖全部原版发电与储能设施；使用：防止字段映射遗漏；边界：环境、预热和喷涂另计。
func TestVanillaEnergyFacilities(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id, model int
		power     Power
	}{
		{2203, 53, Power{Node: true, Generator: true, Wind: true, Generation: 5000}},
		{2204, 54, Power{Node: true, Generator: true, Generation: 36000, FuelUse: 45000, FuelMask: 1}},
		{2205, 55, Power{Node: true, Generator: true, Photovoltaic: true, Generation: 6000}},
		{2206, 46, Power{Node: true, Accumulator: true, InputEnergyPerTick: 25000, OutputEnergyPerTick: 37500, MaxAcuEnergy: 540000000}},
		{2207, 46, Power{Node: true, Accumulator: true, InputEnergyPerTick: 25000, OutputEnergyPerTick: 37500, MaxAcuEnergy: 540000000}},
		{2208, 73, Power{Node: true, Generator: true, Gamma: true, Generation: 100000, CatalystMask: 1, ProductID: 1208, ProductHeat: 1200000000}},
		{2209, 45, Power{Node: true, Exchanger: true, MaxAcuEnergy: 540000000, ExchangeEnergyPerTick: 900000}},
		{2210, 56, Power{Node: true, Generator: true, Generation: 1200000, FuelUse: 1200000, FuelMask: 4}},
		{2211, 118, Power{Node: true, Generator: true, Generation: 250000, FuelUse: 250000, FuelMask: 2}},
		{2213, 255, Power{Node: true, Generator: true, Geothermal: true, Generation: 80000}},
	}
	for _, tc := range cases {
		item, ok := catalog.Lookup(tc.id)
		if !ok || !item.PowerKnown || item.Power == nil {
			t.Errorf("设施 %d 缺少已核验的电力事实", tc.id)
			continue
		}
		if item.ModelIndex != tc.model || *item.Power != tc.power {
			t.Errorf("设施 %d: model=%d power=%+v，期望 model=%d power=%+v", tc.id, item.ModelIndex, *item.Power, tc.model, tc.power)
		}
	}
}

// TestVanillaFuelEnergy 职责：覆盖燃料类别和未喷涂热值；使用：燃料闭合计算；边界：不把燃料类别作为建筑运行状态。
func TestVanillaFuelEnergy(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id, fuelType int
		heat         int64
	}{
		{1006, 1, 2700000}, {1030, 1, 1500000}, {1031, 1, 500000},
		{1011, 1, 4800000}, {1109, 1, 6750000}, {1112, 1, 900000},
		{1007, 1, 4050000}, {1114, 1, 4500000}, {1120, 1, 9000000}, {1121, 1, 9000000},
		{1801, 1, 54000000}, {1802, 2, 600000000}, {1803, 4, 7200000000}, {1804, 4, 72000000000},
		{1123, 1, 96000}, {1124, 1, 84000}, {1117, 1, 1800000},
		{1128, 1, 9720000}, {1129, 1, 21600000}, {1130, 1, 54000000},
		{1141, 1, 3240000}, {1142, 1, 8856000}, {1143, 1, 21255000},
		{5206, 1, 3600000}, {2207, 8, 540000000},
	}
	for _, tc := range cases {
		item, ok := catalog.Lookup(tc.id)
		if !ok || item.FuelType != tc.fuelType || item.HeatValue != tc.heat {
			t.Errorf("燃料 %d: found=%v type=%d heat=%d，期望 type=%d heat=%d", tc.id, ok, item.FuelType, item.HeatValue, tc.fuelType, tc.heat)
		}
	}
	for _, id := range []int{1001, 1208, 1209, 2206} {
		item, ok := catalog.Lookup(id)
		if !ok || item.FuelType != 0 || item.HeatValue != 0 {
			t.Errorf("非燃料 %d 不应获得燃料类别或热值: %+v", id, item)
		}
	}
}

// TestEnergyLookupReturnsCopy 职责：保护目录的共享事实；使用：查询调用方隔离回归；边界：覆盖电力指针与普通物品，未知ID仍返回false。
func TestEnergyLookupReturnsCopy(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	first, ok := catalog.Lookup(2209)
	if !ok || first.Power == nil {
		t.Fatal("缺少能量枢纽事实")
	}
	wantPower := *first.Power
	first.Name = "修改的副本"
	first.FuelType = 99
	first.HeatValue = -1
	*first.Power = Power{}
	again, ok := catalog.Lookup(2209)
	if !ok || again.Power == nil || again.Power == first.Power || *again.Power != wantPower || again.Name == first.Name || again.FuelType != 0 || again.HeatValue != 0 {
		t.Fatalf("Lookup 副本修改污染目录: %+v", again)
	}
	fuel, ok := catalog.Lookup(1006)
	if !ok || fuel.Power != nil || fuel.PowerKnown {
		t.Fatalf("煤矿不应被识别为有电力组件的设施: %+v", fuel)
	}
	if unknown, ok := catalog.Lookup(-1); ok || unknown != (Item{}) {
		t.Fatalf("未知物品应返回空值与false: %+v, %v", unknown, ok)
	}
}
