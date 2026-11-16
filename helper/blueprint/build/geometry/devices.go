package geometry

// 文件职责：以量纲明确的只读值交接已有原型几何和设备能力事实。
// 使用：布局、供电和物流边界查询；边界：只委托placement与transport，不维护第二份设备表或修改目录。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/transport"
)

// DeviceGeometry 职责：保存一次独立的原型盒和端口姿态值；使用：模式边界读取；边界：切片由目录复制，修改返回值不会改变事实源。
type DeviceGeometry struct {
	// Colliders 职责：保存真实静态建造盒；使用：世界变换；边界：不是参考布局排距。
	Colliders []placement.Collider
	// Prefab 职责：保存端口及附加区域姿态；使用：真实几何；边界：不包含运行时Unity实体。
	Prefab placement.PrefabGeometry
}

// QueryDeviceGeometry 职责：读取已核对原型的独立几何副本；使用：模式共同查询；边界：未知映射、缺少姿态或损坏来源明确失败。
func QueryDeviceGeometry(item, model int16) (DeviceGeometry, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return DeviceGeometry{}, err
	}
	shapes, err := catalog.Shapes(item, model)
	if err != nil {
		return DeviceGeometry{}, err
	}
	prefab, err := catalog.Geometry(item, model)
	if err != nil {
		return DeviceGeometry{}, err
	}
	return DeviceGeometry{Colliders: shapes, Prefab: prefab}, nil
}

// BeltCapacity 职责：返回现有唯一带等级容量的每分钟量纲；使用：工具和几何运输边界；边界：未知等级不回退无限容量。
func BeltCapacity(level int) (ItemsPerMinute, error) {
	value, err := transport.BeltCapacityPerMinute(level)
	return ItemsPerMinute(value), err
}

// InserterCapacity 职责：返回已核对跨度和等级的每分钟额定能力；使用：真实端口边界；边界：不推测科技、堆叠或供电。
func InserterCapacity(level, reach int) (ItemsPerMinute, error) {
	value, err := transport.InserterCapacityPerMinute(level, reach)
	return ItemsPerMinute(value), err
}

// PowerCoverRadius 职责：返回现有电塔事实的世界覆盖长度；使用：直连及配送供电；边界：覆盖不等于发电或节点连接距离。
func PowerCoverRadius() (WorldLength, error) {
	value, err := transport.TeslaPowerCoverRadius()
	return WorldLength(value), err
}

// PowerConnectDistance 职责：返回现有电塔事实的连接世界长度；使用：电网边界；边界：不与覆盖半径混用。
func PowerConnectDistance() (WorldLength, error) {
	value, err := transport.TeslaPowerConnectDistance()
	return WorldLength(value), err
}
