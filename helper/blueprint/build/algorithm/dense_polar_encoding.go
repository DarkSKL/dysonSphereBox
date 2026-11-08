package build

// 文件职责：以主区坐标编码极区跨原生子环的完整实体。
// 使用：极区整厂候选；边界：区域覆盖真实完整包络，机器保持正交，最终几何、货物采样与供电全部重验。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
)

// preparePolarRootEncoding 职责：保留主区坐标的跨环实体并补齐原生Area父链；使用：固定目标极区后的编码；边界：不借缩小Area矩形隐藏实体，不接受超过整圈或极区的坐标。
func (b *builder) preparePolarRootEncoding(ctx context.Context, layout *SurfaceLayout) (*surfaceEncoding, error) {
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	if !ok || selection.tier != 5 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "跨子环编码需要第5档选区"}
	}
	bands := densityLatitudeTierBands(5, selection.hemisphere, 200)

	if b.polarFrame == nil {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "极区路由没有固定的原生单元坐标"}
	}
	if b.polarFrame.flipHemisphere {
		// 同时反转经度、纬度和本地朝向，对应真实球面刚体旋转；不剪切单元或镜像槽位。
		// 所有实体引用和取货相对量随整体旋转，目标半球仍重新核验几何、跨度及实际采样。
		_, high := bounds(b.all)
		last := int(math.Ceil(high.y))
		rotatePlacedQuarter(b.all, 2)
		shift(b.all, math.Ceil(high.x), float64(last), 0)
		b.polarFrame.start = -b.polarFrame.start - last
		b.polarFrame.height = last + 1
		b.polarFrame.flipHemisphere = false
	}
	minimum, _ := bounds(b.all)
	if rows := int(math.Floor(minimum.y)); rows > 0 {
		// 起点必须贴合真实实体包络；不能留下空纬行来假装贴近所选边界。
		shift(b.all, 0, -float64(rows), 0)
		b.polarFrame.start += rows
		b.polarFrame.height -= rows
	}
	rootSegments := longitudeSegmentCount(b.polarFrame.start, 200)
	if b.polarFrame.segments != rootSegments {
		// 路由细网格与原生主Area仅经度单位不同；同时换算货物目标和箱体槽位证据，不改变球面位置。
		scale := float64(rootSegments) / float64(b.polarFrame.segments)
		for _, p := range b.all {
			p.a.x *= scale
			p.b.x *= scale
			p.densityCargo.relative.x *= scale
			p.nativeUnitScale *= scale
		}
		b.nativeLongitudeScale *= scale
		b.polarFrame.segments = rootSegments
		b.polarFrame.longitudeStep = surfacePasteLongitudeStep(float32(b.polarFrame.start) * b.polarFrame.latitudeStep)
	}
	low, high := bounds(b.all)
	if low.y < 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "极区线路越过原生南侧边界"}
	}
	shiftX := -math.Floor(low.x)
	shift(b.all, shiftX, 0, 0)
	start := b.polarFrame.start
	height := int(math.Ceil(high.y)) + 1
	if start < bands[0].low || start+height-1 > bands[len(bands)-1].high {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "极区完整包络超出第5档"}
	}
	width := int(math.Ceil(high.x+shiftX)) + 1
	if width < 1 || width > rootSegments*5 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("极区完整包络经向%d格超过主区整圈%d格", width, rootSegments*5)}
	}
	encoding := &surfaceEncoding{rootCoordinates: true, byBuilding: map[*placed]int{}, plan: makeSurfacePlan(layout.Mode, 200, layout.LatitudeGridIndex, start, width, height), anchorCandidates: []int{start}}
	encoding.plan.AnchorLatitudeGrid = start
	for _, band := range bands {
		first, last := max(start, band.low), min(start+height-1, band.high)
		if first > last {
			continue
		}
		i := len(encoding.areas)
		parent, offset := -1, 0
		if i > 0 {
			parent = i - 1
			offset = first - encoding.plan.Bands[i-1].MinLatitudeGrid
		}
		scale := float64(band.segments) / float64(rootSegments)
		areaWidth := int(math.Ceil(float64(width-1)*scale)) + 1
		if !candidateFitsRing(first, 0, last-first, areaWidth, 200) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "极区子环区域矩形超出原生容量"}
		}
		encoding.areas = append(encoding.areas, blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(band.segments), AnchorLocalOffsetY: int16(offset), Width: int16(areaWidth), Height: int16(last - first + 1)})
		encoding.latitudeOffsets = append(encoding.latitudeOffsets, first-start)
		encoding.longitudeScales = append(encoding.longitudeScales, scale)
		encoding.plan.Bands = append(encoding.plan.Bands, SurfaceBandPlan{AreaIndex: i, MinLatitudeGrid: first, MaxLatitudeGrid: last, LongitudeSegments: band.segments})
	}
	for _, p := range b.all {
		encoding.byBuilding[p] = 0
	}
	encoding.plan.Bands[0].BuildingCount = len(b.all)
	b.nativeLatitudeScaled = true
	return encoding, ctx.Err()
}

// entityBounds 职责：取得实体坐标允许的完整包络；使用：原生端点修正；边界：常规单区保持既有规则。
func (encoding *surfaceEncoding) entityBounds() (float32, float32) {
	if encoding.rootCoordinates {
		return float32(encoding.plan.LayoutWidth - 1), float32(encoding.plan.LayoutHeight - 1)
	}
	return float32(encoding.areas[0].Width - 1), float32(encoding.areas[0].Height - 1)
}
