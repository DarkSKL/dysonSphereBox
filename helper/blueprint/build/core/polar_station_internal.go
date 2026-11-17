package build

// 文件职责：提前拒绝当前纬度单元内明确违反原生物流塔间距的组合。
// 使用：makePart的动态位置预检；边界：只检查物流塔，不检查可裁剪电力节点，不修改实体、占位或正式15/29米规则。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// polarStationInternalUncertainty 职责：为后续经度刚体转移及原生float32重投影保留一毫米数值不确定带。
// 使用：仅当前置检查的保守拒绝界；边界：不是正式间距容差，临界候选仍由原225/841平方阈值严格终检。
const polarStationInternalUncertainty = .001

// validatePolarStationInternal 职责：用正式单精度距离及原型硬阈值检查同一单元的所有物流塔对。
// 使用：本纬度part动态构建后、经向位置尝试前；边界：明确违规返回可换纬的GEN_E_DENSITY_LAYOUT，取消原样传播，不改变任何实体或输入切片。
func validatePolarStationInternal(ctx context.Context, stations []polarStationOccupancy) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, left := range stations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !surfaceIsStation(left.item) {
			continue
		}
		for _, right := range stations[i+1:] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !surfaceIsStation(right.item) {
				continue
			}
			a := vector{left.point[0], left.point[1], left.point[2]}
			c := vector{right.point[0], right.point[1], right.point[2]}
			distance := math.Sqrt(float64(surfacePowerDistanceSquared(a, c)))
			limit := math.Sqrt(float64(surfaceStationDistanceLimit(left.item, right.item)))
			if distance < limit-polarStationInternalUncertainty {
				return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("原生单元内物流塔相隔 %.6f 米，明确小于 %g 米硬间距；当前纬度不能仅靠经度刚体转移修复", distance, limit)}
			}
		}
	}
	return ctx.Err()
}
