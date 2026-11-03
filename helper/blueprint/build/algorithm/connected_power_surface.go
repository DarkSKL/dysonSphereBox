package build

// 文件职责：在已固定的真实球面坐标系中完成紧凑直连候选补电。
// 使用：机器及带路由收尾后、正式 finish 前；边界：只接受原版单区域映射，后续仍执行完整规则，不以无塔几何替代完整验收。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
)

// completeConnectedSurfacePower 职责：统一监测器、球面锚点、地面补电和分区计数；使用：全局补电独立候选；边界：不扩大 Area、不重选补电后的锚点，任一失败由候选整体丢弃。
// 逻辑流：1. 完成边界监测 => 2. 审计无塔几何并固定映射 => 3. 地面联合补电 => 4. 更新最终实体计数。
func (b *builder) completeConnectedSurfacePower(ctx context.Context, req Request) error {
	originalPower := b.setting.NeedPower
	disabled := false
	b.setting.NeedPower = &disabled
	monitorErr := b.completeBoundaryFlowMonitors(ctx, req.Mode)
	b.setting.NeedPower = originalPower
	if monitorErr != nil {
		return monitorErr
	}
	if req.SurfaceLayout == nil || req.SurfaceLayout.Mode == "planar" {
		return &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "全局球面补电候选缺少球面布局参数"}
	}
	encoding, err := b.prepareSurfaceEncodingContext(ctx, req.SurfaceLayout)
	if err != nil {
		return err
	}
	if encoding.planar || len(encoding.areas) != 1 || encoding.plan.GridSegment != 200 {
		return &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "全局紧凑补电目前只接受原版 200 分段的单区域球面候选"}
	}
	records, _, _, err := b.records()
	if err != nil {
		return err
	}
	if err := encoding.applyRecords(b.all, records); err != nil {
		return err
	}
	if err := encoding.validateAnchorCandidates(ctx, records); err != nil {
		return err
	}
	// 当前锚点已通过无塔机器及路线几何；供电后不再平移或重选坐标系。
	encoding.anchorCandidates = []int{encoding.plan.AnchorLatitudeGrid}
	b.connectedPowerEncoding = encoding
	if err := b.completeConnectedPower(ctx); err != nil {
		return err
	}
	encoding.plan.Bands[0].BuildingCount = len(b.all)
	return nil
}
