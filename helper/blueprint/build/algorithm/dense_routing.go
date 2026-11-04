package build

// 文件职责：按密铺同次粘贴规则为全场A*登记实际节点占位，避免旧整格近似吞掉低层轨道间的可用空间。
// 使用：dense专用路由；边界：静态建造盒和节点合并仍检查，球面与臂-臂最终规则不放宽；不代表已有世界实体可互穿。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
)

// denseTransportObstacles 职责：以实际格距与原版带节点半径构造静态障碍；使用：密铺候选路径；边界：不缩小原型建造盒，未知模型直接失败。
func denseTransportObstacles(ctx context.Context, entities []*placed) (map[vector]bool, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	encoding := newPlanarRuleEncoding()
	blocked := map[vector]bool{}
	const height, radius = 1.3333333, .23
	for _, entity := range entities {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺实际占位登记已取消或超时"}
		}
		if surfaceIsBelt(entity.record.ItemID) || entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
			continue
		}
		shapes, err := catalog.Shapes(entity.record.ItemID, entity.record.ModelIndex)
		if err != nil {
			return nil, err
		}
		origin := vector{entity.a.x * planarGridLength, entity.a.z*height + .2, entity.a.y * planarGridLength}
		for _, shape := range shapes {
			box, err := encoding.colliderBox(origin, entity.record.Yaw, shape)
			if err != nil {
				return nil, err
			}
			low, high := box.Bounds()
			for x := math.Floor((low[0] - radius) / planarGridLength); x <= math.Ceil((high[0]+radius)/planarGridLength); x++ {
				for y := math.Floor((low[2] - radius) / planarGridLength); y <= math.Ceil((high[2]+radius)/planarGridLength); y++ {
					for z := math.Max(0, math.Floor((low[1]-radius-.4)/height)); z <= math.Ceil((high[1]+radius-.4)/height); z++ {
						point := placement.Vector{x * planarGridLength, z*height + .4, y * planarGridLength}
						if box.SegmentDistanceSquared(point, point) < radius*radius {
							blocked[vector{x, y, z}] = true
						}
					}
				}
			}
		}
	}
	return blocked, nil
}

// blockDenseBeltNode 职责：只屏蔽与真实带节点过近的整数候选；使用：已有低轨和新路由节点；边界：0.35米保证不直接借同点合并，不以整条粗管禁止同批跨带。
func blockDenseBeltNode(blocked map[vector]bool, point vector) {
	for x := math.Floor(point.x); x <= math.Ceil(point.x); x++ {
		for y := math.Floor(point.y); y <= math.Ceil(point.y); y++ {
			for z := math.Max(0, math.Floor(point.z)); z <= math.Ceil(point.z); z++ {
				dx, dy, dz := (x-point.x)*planarGridLength, (y-point.y)*planarGridLength, (z-point.z)*1.3333333
				if dx*dx+dy*dy+dz*dz < .35*.35 {
					blocked[vector{x, y, z}] = true
				}
			}
		}
	}
}
