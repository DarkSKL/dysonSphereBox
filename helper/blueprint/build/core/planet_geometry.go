package build

// 文件职责：将局部生成器的原版物理检查复用于整星装配；使用：已完成 Area 映射的文档；边界：不代替游戏实贴。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
)

// ValidatePlanetGeometry 职责：复核同一星球内的静态碰撞、传送带接线及物流塔；使用：整星文档编码前；边界：地形、存量和跨星运力由调用方验证。
func ValidatePlanetGeometry(ctx context.Context, document blueprint.Document, latitude int) error {
	encoding := &surfaceEncoding{areas: document.Areas, plan: &SurfacePlan{
		GridSegment: 200, AnchorLatitudeGrid: latitude, PrimaryAreaIndex: int(document.PrimaryAreaIndex),
	}}
	if err := encoding.validateStaticColliders(ctx, document.Buildings); err != nil {
		return err
	}
	// 纬向平移会改变带节点与设备建造盒的间距，按整图投影重新核对原版连接和附加通道。
	if err := encoding.validateBeltBuildingOccupancy(ctx, document.Buildings); err != nil {
		return err
	}
	if err := encoding.validateSurfaceStationSpacing(ctx, document.Buildings); err != nil {
		return err
	}
	if err := encoding.validateBeltDistancesContext(ctx, document.Buildings); err != nil {
		return err
	}
	if err := encoding.validateBeltCrossings(ctx, document.Buildings); err != nil {
		return err
	}
	if err := encoding.validateSurfacePowerNodeSpacing(ctx, document.Buildings); err != nil {
		return err
	}
	if _, err := encoding.surfaceInserterBoxes(ctx, document.Buildings); err != nil {
		return err
	}
	return validateStationPorts(ctx, document.Buildings, 0)
}
