package build

// 文件职责：在装配前证明直连原料带的必要容量下界；使用：最低运输计划与独立容量校验共用。
// 边界：只使用用户许可纬带及真实外供总流，忽略出口预留以保持保守；通过不代表实际接口或完整布局可行。
import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
	"fmt"
	"math"
)

// inputBoundaryCapacity 职责：比较外供每物品最少带数与许可纬带三层接口的理论上限；使用：排除所有几何候选都无法补救的低档位。
// 边界：只用于显式选区的直连方案，不以当前小候选的高度强迫升档；平面、塔及配送模式不适用此下界。
func inputBoundaryCapacity(req Request, setting Setting) error {
	if req.Mode != 1 || req.SurfaceLayout == nil || req.SurfaceLayout.Mode == "planar" {
		return nil
	}
	selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	if !ok {
		return nil
	}
	// surface枚举之前没有实际depth；必要下界采用已授权支持范围的最大深度，避免提前删去深层候选可行的低档。
	// 这只放宽必要条件，不作为真实部署许可；每个实际候选及最终物理校验仍消费其固定depth。
	depth := req.SurfaceLayout.adjacentDepth
	if req.SurfaceLayout.ExpandAdjacent {
		depth = 3
	}
	bands := denseLatitudeAllowedBands(selection, req.SurfaceLayout.ExpandAdjacent, depth)
	if len(bands) == 0 {
		return nil
	}
	maximum := (bands[len(bands)-1].high - bands[0].low + 1) * denseBoundaryLevels
	capacity, err := transport.BeltCapacityPerMinute(setting.BeltLv)
	if err != nil {
		return err
	}
	rates := map[int]float64{}
	for _, item := range req.Plan.RawInputs {
		rates[item.ItemID] += item.RatePerMinute
	}
	minimum := 0
	for _, rate := range rates {
		if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return inputStageError("外供速率不是有限非负值")
		}
		minimum += int(math.Ceil(math.Max(0, rate/capacity-1e-9)))
	}
	if minimum > maximum {
		return &blueprint.Error{Code: "GEN_E_BELT_CAPACITY", Message: fmt.Sprintf("当前带等级的外供至少需要%d条独立原料带，超过许可纬带三层接口理论上限%d条", minimum, maximum)}
	}
	return nil
}
