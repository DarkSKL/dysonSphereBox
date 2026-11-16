package build

// 文件职责：从最终编码记录统计真实传送带连接成本。
// 使用：物流摘要收尾及独立离线回归；边界：只度量已核验连接，不替代连接池、碰撞、容量或游戏输送检查。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// updateTotalBeltMetrics 职责：从最终世界位置计量唯一带实体及全部带间连接；使用：各模式共同收尾；边界：单次弦长不计重复lane，不把添加传送带作为提高空间利用率的奖励。
func updateTotalBeltMetrics(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building, metrics *LayoutMetrics) error {
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	metrics.BeltCount, metrics.TotalBeltLengthMeters = 0, 0
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全带长度统计已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		metrics.BeltCount++
		next, ok := byIndex[record.OutputObjIdx]
		if !ok || !surfaceIsBelt(records[next].ItemID) {
			continue
		}
		a, c := positions[i], positions[next]
		metrics.TotalBeltLengthMeters += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
	}
	return nil
}

// updateBeltConnectionMetrics 职责：按唯一实体输出重算跨区和升降带段；使用：所有模式完成最终编号后；边界：Area 局部 Z 均为离地高度，平面 XY 或不同 Area 原点不参与高度推断。
func updateBeltConnectionMetrics(metrics *LayoutMetrics, records []blueprint.Building) error {
	metrics.CrossAreaConnections, metrics.CrossLayerConnections = 0, 0
	for _, record := range records {
		if !surfaceIsBelt(record.ItemID) || record.OutputObjIdx < 0 {
			continue
		}
		if int(record.OutputObjIdx) >= len(records) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("带 %d 指向蓝图外建筑", record.Index)}
		}
		target := records[record.OutputObjIdx]
		if target.AreaIndex != record.AreaIndex {
			metrics.CrossAreaConnections++
		}
		if surfaceIsBelt(target.ItemID) && math.Abs(float64(record.LocalOffset.Z)-float64(target.LocalOffset.Z)) > 1e-6 {
			metrics.CrossLayerConnections++
		}
	}
	return nil
}
