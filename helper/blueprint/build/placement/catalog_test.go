package placement

// 文件职责：验证内置游戏碰撞目录覆盖多模型物品的每个原型变体。
// 使用：球面生成器回归测试，防止混带分流器模型遗漏后被失败关闭。
// 边界：只核对离线数值目录，不模拟 Unity 场景、动态碰撞或实际蓝图粘贴。

import "testing"

// TestCatalogCoversSplitterVariants 职责：确保四向分流器的模型 38、39、40 均可按物品身份查询。
// 使用：验证 ItemProto.ModelCount 对碰撞目录的覆盖；混带模式使用模型 39。
// 边界：查询通过只表示数值事实可用，不能代替游戏内放置验收。
func TestCatalogCoversSplitterVariants(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []int16{38, 39, 40} {
		shapes, err := catalog.Shapes(2020, model)
		if err != nil {
			t.Fatalf("splitter model %d: %v", model, err)
		}
		if len(shapes) == 0 {
			t.Fatalf("splitter model %d has no build collider", model)
		}
	}
}

// TestCatalogCoversPrefabPortGeometry 职责：确保分流器、机器和喷涂机的 PrefabDesc 端口事实已进入目录。
// 使用：验证动态对象适配阶段不会因为资源提取遗漏而悄悄使用默认端口。
// 边界：只验证离线目录的字段数量和基本尺寸，不宣称分拣器扫掠或 Unity Physics 已通过。
func TestCatalogCoversPrefabPortGeometry(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		item       int16
		model      int16
		ports      int
		slots      int
		addonAreas int
	}{
		{name: "splitter", item: 2020, model: 39, ports: 4},
		{name: "assembler", item: 2303, model: 65, slots: 12},
		{name: "spray-coater", item: 2030, model: 208, addonAreas: 1},
	}
	for _, tc := range cases {
		geometry, err := catalog.Geometry(tc.item, tc.model)
		if err != nil {
			t.Fatalf("%s geometry: %v", tc.name, err)
		}
		if len(geometry.PortPoses) != tc.ports || len(geometry.SlotPoses) != tc.slots || len(geometry.AddonAreaColPoses) != tc.addonAreas {
			t.Fatalf("%s geometry counts = ports %d, slots %d, addon %d", tc.name, len(geometry.PortPoses), len(geometry.SlotPoses), len(geometry.AddonAreaColPoses))
		}
	}
}
