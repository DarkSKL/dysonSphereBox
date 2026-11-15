package build

// 文件职责：检查最终静态建造盒的真实球面投影是否留在所选档位内。
// 使用：密铺蓝图正式规则检查前；边界：不依赖布局用的粗略格数，不替代建筑之间的碰撞检测或游戏实测。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
)

// denseLatitudeProjectionTolerance 职责：保留球面坐标投影约1.3厘米的数值容差；使用：静态盒纬向范围比较；边界：不放宽任何建造盒碰撞、带节点净距或分拣器规则。
const denseLatitudeProjectionTolerance = .01

// validateDenseStaticLatitude 职责：逐角点验证已导出静态实体的纬向范围；使用：最终重定位及补电之后；边界：仅作用于有效五档选区，未知原型与越界实体均不输出。
func (encoding *surfaceEncoding) validateDenseStaticLatitude(ctx context.Context, layout *SurfaceLayout, records []blueprint.Building) error {
	if layout == nil || layout.LatitudeZone == "" || encoding.planar {
		return nil
	}
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	if !ok {
		return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "真实建造盒检查需要有效的五档选区"}
	}
	bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, layout.GridSegment)
	if len(bands) == 0 {
		return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "所选档位没有可检查的原生纬带"}
	}
	low, high := float64(bands[0].low), float64(bands[len(bands)-1].high)
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if surfaceIsBelt(record.ItemID) || record.ItemID >= 2011 && record.ItemID <= 2014 {
			continue
		}
		shapes, err := catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for _, shape := range shapes {
			box, err := encoding.colliderBox(world[i], record.Yaw, shape)
			if err != nil {
				return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
			}
			for mask := 0; mask < 8; mask++ {
				corner := box.Center
				for axis := 0; axis < 3; axis++ {
					sign := -1.
					if mask&(1<<axis) != 0 {
						sign = 1
					}
					corner = corner.Add(box.Axes[axis].Scale(sign * box.HalfSize[axis]))
				}
				latitude := math.Asin(corner[1]/math.Sqrt(corner.Dot(corner))) * 500 / math.Pi
				if latitude < low-denseLatitudeProjectionTolerance || latitude > high+denseLatitudeProjectionTolerance {
					return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("建筑%d（物品%d）的真实建造盒超出第%d档纬带：角点%.3f格，允许%.0f至%.0f格", record.Index, record.ItemID, selection.tier, latitude, low, high)}
				}
			}
		}
	}
	return nil
}
