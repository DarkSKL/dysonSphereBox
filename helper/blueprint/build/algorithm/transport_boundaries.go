package build

// 文件职责：统一将所有模式的边界接口定位到最终编码建筑及其 Area 局部坐标。
// 使用：物流摘要收尾后调用，供预览器与人工定位共同使用。
// 边界：装配平面坐标只服务布局，不能在球面映射后冒充蓝图局部坐标；缺失建筑或 Area 必须失败。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// finalizeBoundaryCoordinates 职责：以最终 records 覆盖中间端点坐标并补齐 Area 身份；使用：所有模式编码前的共同出口；边界：BuildingIndex 是最终数组下标，不从物品或几何邻近猜测接口归属。
func finalizeBoundaryCoordinates(summary *LogisticsSummary, records []blueprint.Building, areas []blueprint.Area) error {
	knownAreas := make(map[int8]bool, len(areas))
	for _, area := range areas {
		knownAreas[area.Index] = true
	}
	for i := range summary.ExternalPorts {
		port := &summary.ExternalPorts[i]
		if port.BuildingIndex < 0 || port.BuildingIndex >= len(records) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("物品 %d 的边界端点引用蓝图外建筑 %d", port.ItemID, port.BuildingIndex)}
		}
		record := records[port.BuildingIndex]
		if !knownAreas[record.AreaIndex] {
			return &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("物品 %d 的边界建筑 %d 引用不存在的 Area %d", port.ItemID, port.BuildingIndex, record.AreaIndex)}
		}
		port.AreaIndex = int(record.AreaIndex)
		port.X, port.Y, port.Z = float64(record.LocalOffset.X), float64(record.LocalOffset.Y), float64(record.LocalOffset.Z)
	}
	return nil
}
