package build

// 文件职责：在目标球面坐标内构造完整工厂，避免大厂在临时锚点误拒绝。
// 使用：高纬、大型产线及小型密铺几何失败后的原生重试；边界：仅发布经过目标区域、实体、货物与供电复核的结果，不以其它地区代替指定中心或边缘。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
)

// densityNativeBandKey 职责：把内部原生子环传给共享路由/空间预算；使用：单候选上下文；边界：请求之间不共享。
type densityNativeBandKey struct{}

// densityPolarRootKey 职责：标识跨子环坐标候选；使用：空间预算及长带策略；边界：不影响已有单区候选。
type densityPolarRootKey struct{}

// densityPolarMirrorKey 职责：为极点侧设施模式提供对称装配候选；使用：极区内部搜索；边界：直连模式保留左侧原料入口，不使用半周映射。
type densityPolarMirrorKey struct{}

// densityPrototypeFootprintKey 职责：为原生几何重试启用真实原型占地；使用：已有候选均未通过之后的单次细化；边界：保留已经可用的紧凑布局，不修改配方与设备等级。
type densityPrototypeFootprintKey struct{}

// denseNativeGeometryFailure 职责：从合并或包装错误中识别可重新排布的几何失败；使用：原生坐标及占地细化的重试边界；边界：能力不足、未知原型和超时均不作为重排依据。
func denseNativeGeometryFailure(err error) bool {
	if surfaceAnchorRetryable(err) {
		return true
	}
	if failure, ok := err.(*blueprint.Error); ok {
		return failure.Code == "GEN_E_DENSITY_LAYOUT" || failure.Code == "GEN_E_SURFACE_CAPACITY" || failure.Code == "GEN_E_DENSITY_STREET_SEAM"
	}
	if group, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range group.Unwrap() {
			if denseNativeGeometryFailure(cause) {
				return true
			}
		}
	} else if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return denseNativeGeometryFailure(wrapped.Unwrap())
	}
	return false
}

// densityLayoutBand 职责：取得布局实际选定的原生带；使用：编码、光标和范围验证；边界：无内部子环时沿用公开选区。
func densityLayoutBand(layout *SurfaceLayout) (surfacePackedBand, string, error) {
	if layout.nativeBand != nil {
		s, _ := densityLatitudeSelectionFor(layout.LatitudeZone)
		return *layout.nativeBand, s.label(), nil
	}
	return densityLatitudeBand(layout.LatitudeZone, layout.GridSegment)
}

// densityContextBand 职责：在空间预算阶段读取同一原生子环；使用：上下文中的极区候选；边界：普通选区无隐式覆盖。
func densityContextBand(ctx context.Context, zone string, segment int) (surfacePackedBand, string, error) {
	if band, ok := ctx.Value(densityNativeBandKey{}).(surfacePackedBand); ok {
		s, _ := densityLatitudeSelectionFor(zone)
		return band, s.label(), nil
	}
	return densityLatitudeBand(zone, segment)
}
