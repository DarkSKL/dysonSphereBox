package build

// 文件职责：检查球面蓝图内部的动态物流占用。
// 使用：静态盒检查之后检查带交叉与修正后的分拣器动态盒；边界：尚不覆盖已有世界实体、Unity Physics 和完整带胶囊豁免。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// validateDynamicFacilities 职责：验证端口修正和分拣器互相占用；使用：最终记录出口；边界：按游戏预览规则分拣器与非分拣器互免，不混进静态占用集合。
func (encoding *surfaceEncoding) validateDynamicFacilities(ctx context.Context, records []blueprint.Building) error {
	if err := encoding.validateSurfacePowerNodeSpacing(ctx, records); err != nil {
		return err
	}
	if err := encoding.validateSurfaceStationSpacing(ctx, records); err != nil {
		return err
	}
	if err := encoding.validateBeltCrossings(ctx, records); err != nil {
		return err
	}
	if err := encoding.validateBeltBuildingOccupancy(ctx, records); err != nil {
		return err
	}
	boxes, err := encoding.surfaceInserterBoxes(ctx, records)
	if err != nil {
		return err
	}
	left, right, err := placement.FirstConflict(ctx, boxes)
	if err != nil {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "分拣器动态占用检查已取消或超时"}
		}
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	if left >= 0 {
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("分拣器动态建造盒冲突：建筑 %d 与建筑 %d", left, right)}
	}
	encoding.plan.CollisionCoverage = "staticBuildColliders+powerNodeSpacing+stationSpacing+beltCrossings+beltBuildingOccupancy+spraycoaterChannels+inserterSlotPoses+inserterBuildColliders"
	return nil
}
