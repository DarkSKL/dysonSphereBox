package build

// 文件职责：在目标球面坐标内构造完整工厂，避免大厂在临时锚点误拒绝。
// 使用：高纬、大型产线及小型密铺几何失败后的原生重试；边界：仅发布经过目标区域、实体、货物与供电复核的结果，不以其它地区代替指定中心或边缘。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"sort"
)

// densityNativeBandKey 职责：把内部原生子环传给共享路由/空间预算；使用：单候选上下文；边界：请求之间不共享。
type densityNativeBandKey struct{}

// densityPolarRootKey 职责：标识跨子环坐标候选；使用：空间预算及长带策略；边界：不影响已有单区候选。
type densityPolarRootKey struct{}

// densityPolarMirrorKey 职责：为极点侧设施模式提供对称装配候选；使用：极区内部搜索；边界：直连模式保留左侧原料入口，不使用半周映射。
type densityPolarMirrorKey struct{}

// densityPrototypeFootprintKey 职责：为原生几何重试启用真实原型占地；使用：已有候选均未通过之后的单次细化；边界：保留已经可用的紧凑布局，不修改配方与设备等级。
type densityPrototypeFootprintKey struct{}

// denseNativeLayoutRetryable 职责：识别小型密铺候选可用目标球面重新规划的空间或姿态失败；使用：保留既有成功布局后的单次原生重试；边界：不重试未知原型、能力不足或无效计划，不改变所选纬带、设备及生产链。
func denseNativeLayoutRetryable(req Request, err error) bool {
	if req.Density != densityDense || req.SurfaceLayout == nil || req.SurfaceLayout.GridSegment != 200 {
		return false
	}
	selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	if !ok || selection.tier >= 4 || req.Plan.MachineCount >= denseNativeLargeMachineThreshold {
		return false
	}
	return denseNativeGeometryFailure(err)
}

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

// generateDensePolar 职责：先在共享球面坐标内比较有限矩形包络，再检查同目标位置的单环候选；使用：极区和大厂入口；边界：不换产量、设备或地区，不返回失败的部分蓝图。
func generateDensePolar(ctx context.Context, req Request, adapter *modeAdapter) (Result, error) {
	selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
	if len(bands) == 0 {
		return Result{}, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "所选区域没有原生纬带"}
	}
	low, high := bands[0].low, bands[len(bands)-1].high
	preferred := (low + high) / 2
	if selection.position == "south" {
		preferred = low
	}
	if selection.position == "north" {
		preferred = high
	}
	sort.SliceStable(bands, func(i, j int) bool {
		return absInt(bandCenter(bands[i])-preferred) < absInt(bandCenter(bands[j])-preferred)
	})
	var last error
	retryFootprints := false
	extents := []int{24, 32, 40, 48, 56, 64}
	if selection.tier < 5 {
		extents = denseTargetFrameExtents(bands)
	}
	if req.Plan.MachineCount >= denseNativeLargeMachineThreshold {
		// 大产线先验证容量更充足的包络，所有较窄候选仍保留，避免把总时限耗在小包络上。
		sort.Sort(sort.Reverse(sort.IntSlice(extents)))
	}
	for _, extent := range extents {
		orientations := []bool{false}
		// 非极区大厂的两侧边缘都需要比较刚体半周对称候选；固定左侧入口的直连模式不参与。
		largeEdge := selection.tier < 5 && req.Plan.MachineCount >= denseNativeLargeMachineThreshold && selection.position != "center"
		if req.Mode >= 2 && req.Mode <= 4 && (largeEdge || selection.hemisphere > 0 && selection.position == "north" || selection.hemisphere < 0 && selection.position == "south") {
			orientations = []bool{false, true}
			if selection.hemisphere > 0 {
				orientations = []bool{true, false}
			}
		}
		for _, opposite := range orientations {
			for _, sameSide := range refineryLayoutVariants(req) {
				candidate := req
				layout := *req.SurfaceLayout
				layout.polarRoot = true
				candidate.SurfaceLayout = &layout
				budget := surfacePackedBand{low: 0, high: extent - 1, segments: 200}
				local := context.WithValue(context.WithValue(ctx, densityNativeBandKey{}, budget), densityPolarRootKey{}, true)
				local = context.WithValue(local, densityPolarMirrorKey{}, opposite)
				local = context.WithValue(local, refinerySameSideKey{}, sameSide)
				prepared, err := prepareGeneration(local, candidate, adapter)
				if err != nil {
					return Result{}, err
				}
				out, err := adapter.generate(local, prepared.builder, prepared.request, prepared.dataset)
				if err == nil {
					return publishFinalBlueprintProgress(local, prepared.request, prepared.dataset, out, nil)
				}
				last = err
				if ctx.Err() != nil {
					return Result{}, err
				}
				retryFootprints = retryFootprints || denseNativeGeometryFailure(err)
				reportProgress(ctx, "polar-rejected", fmt.Sprintf("目标球面%d行候选（对称=%t，同侧炼油=%t）未通过：%v", extent, opposite, sameSide, err), 0, 0)
			}
		}
	}
	for _, band := range bands {
		// 常见生产设施及接线至少需要六行；更窄子环仍在五档定义内，不伪装成可建矩形。
		if band.high-band.low+1 < 6 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		for _, sameSide := range refineryLayoutVariants(req) {
			candidate := req
			layout := *req.SurfaceLayout
			layout.nativeBand = &band
			layout.LatitudeGridIndex = bandCenter(band)
			candidate.SurfaceLayout = &layout
			local := context.WithValue(ctx, densityNativeBandKey{}, band)
			local = context.WithValue(local, refinerySameSideKey{}, sameSide)
			prepared, err := prepareGeneration(local, candidate, adapter)
			if err != nil {
				return Result{}, err
			}
			out, err := adapter.generate(local, prepared.builder, prepared.request, prepared.dataset)
			if err == nil {
				return publishFinalBlueprintProgress(local, prepared.request, prepared.dataset, out, nil)
			}
			last = err
			if ctx.Err() != nil {
				return Result{}, err
			}
			retryFootprints = retryFootprints || denseNativeGeometryFailure(err)
			reportProgress(ctx, "polar-rejected", fmt.Sprintf("原生子环%d..%d（同侧炼油=%t）未通过：%v", band.low, band.high, sameSide, err), 0, 0)
		}
	}
	precise, _ := ctx.Value(densityPrototypeFootprintKey{}).(bool)
	if !precise && retryFootprints && ctx.Err() == nil {
		reportProgress(ctx, "dense-footprint-retry", "正在按真实建造盒细化所选纬带的排布", 0, 0)
		return generateDensePolar(context.WithValue(ctx, densityPrototypeFootprintKey{}, true), req, adapter)
	}
	return Result{}, fmt.Errorf("目标球面完整布局候选未通过：%w", last)
}
