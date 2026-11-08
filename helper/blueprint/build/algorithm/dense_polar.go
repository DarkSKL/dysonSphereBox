package build

// 文件职责：在极区的全部原生子环中按所选位置选择可容纳整厂的候选。
// 使用：第5档内部生成；边界：不把外侧第一条原生带冒充整个极区，当前只提交同一子环内完整可建结果。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"sort"
)

// densityNativeBandKey 职责：把内部原生子环传给共享路由/空间预算；使用：单候选上下文；边界：请求之间不共享。
type densityNativeBandKey struct{}

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

// generateDensePolar 职责：按地理目标顺序检查可容纳整厂的原生子环；使用：第5档入口；边界：不越出半球极区、不返回失败的部分蓝图，容量不足由多区策略后续处理。
func generateDensePolar(ctx context.Context, req Request, adapter *modeAdapter) (Result, error) {
	selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	bands := densityLatitudeTierBands(5, selection.hemisphere, 200)
	if len(bands) == 0 {
		return Result{}, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "极区没有原生纬带"}
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
	for _, band := range bands {
		// 常见生产设施及接线至少需要六行；更窄子环仍在五档定义内，不伪装成可建矩形。
		if band.high-band.low+1 < 6 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		candidate := req
		layout := *req.SurfaceLayout
		layout.nativeBand = &band
		layout.LatitudeGridIndex = bandCenter(band)
		candidate.SurfaceLayout = &layout
		local := context.WithValue(ctx, densityNativeBandKey{}, band)
		prepared, err := prepareGeneration(local, candidate, adapter)
		if err != nil {
			return Result{}, err
		}
		out, err := adapter.generate(local, prepared.builder, prepared.request, prepared.dataset)
		if err == nil {
			return publishFinalBlueprintProgress(local, prepared.request, prepared.dataset, out, nil)
		}
		last = err
		reportProgress(ctx, "polar-rejected", fmt.Sprintf("极区原生子环%d..%d未通过：%v", band.low, band.high, err), 0, 0)
	}
	return Result{}, fmt.Errorf("极区单环整厂候选未通过：%w", last)
}
