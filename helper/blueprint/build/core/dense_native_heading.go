package build

// 文件职责：按跨子环真实连线计算分拣器带端的局部水平朝向。
// 使用：原生槽位已固定后的端点候选；边界：不旋转生产建筑、不改俯仰、不移动货物采样。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// nativeBeltEndHeading 职责：使带端水平朝向随实际弧面连线变化；使用：极区臂候选；边界：机器端仍由真实slotPoses决定，最终24/30度检查不变。
func nativeBeltEndHeading(encoding *surfaceEncoding, r blueprint.Building, second bool) (blueprint.Building, error) {
	end := r
	end.LocalOffset = r.LocalOffset2
	world, err := encoding.surfaceWorldPositions([]blueprint.Building{r, end})
	if err != nil {
		return r, err
	}
	direction := placement.Vector{world[1].x - world[0].x, world[1].y - world[0].y, world[1].z - world[0].z}
	point := world[0]
	if second {
		point = world[1]
	}
	frame, err := encoding.recordPose(point, 0, 0, 0)
	if err != nil {
		return r, err
	}
	yaw := float32(math.Mod(math.Atan2(direction.Dot(frame.axes[0]), direction.Dot(frame.axes[2]))*180/math.Pi+360, 360))
	if second {
		r.Yaw2 = yaw
	} else {
		r.Yaw = yaw
	}
	return r, nil
}
