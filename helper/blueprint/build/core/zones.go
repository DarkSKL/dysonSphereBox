package build

// 文件职责：在独立分区规划契约与既有公开结果 DTO 之间转换。
// 使用：当前生成路径兼容调用；边界：算法由 shared/zoning 维护，不在门面重复分区规则。

import (
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/zoning"
	"DysonSphereBox/helper/quant"
)

// planZones 职责：复用独立规划并保留既有结果类型；使用：直连和配送布局；边界：不修改请求或共享规划输出。
func planZones(plan quant.Result) []Zone {
	planned := zoning.Plan(plan)
	zones := make([]Zone, 0, len(planned))
	for _, zone := range planned {
		zones = append(zones, zoneFromLayout(zone))
	}
	return zones
}

// zoneFromLayout 职责：把规划身份转换为公开分区 DTO；使用：布局交接；边界：最终坐标仍由装配和收尾填写。
func zoneFromLayout(zone contract.Zone) Zone {
	return Zone{Key: zone.Key, TargetItemID: zone.TargetItemID, ProductionIndices: append([]int{}, zone.ProductionIndices...)}
}
