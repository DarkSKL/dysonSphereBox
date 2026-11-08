package build

// 文件职责：在目标球面坐标内构造完整工厂，避免大厂在临时锚点误拒绝。
// 使用：第5档及其它档的大型产线；边界：仅发布经过目标区域、实体、货物与供电复核的结果，不以其它地区代替指定中心或边缘。

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
		if req.Mode >= 2 && req.Mode <= 4 && (selection.hemisphere > 0 && selection.position == "north" || selection.hemisphere < 0 && selection.position == "south") {
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
				reportProgress(ctx, "polar-rejected", fmt.Sprintf("原生子环%d..%d（同侧炼油=%t）未通过：%v", band.low, band.high, sameSide, err), 0, 0)
		}
	}
	return Result{}, fmt.Errorf("目标球面完整布局候选未通过：%w", last)
}
