package planet

// 文件职责：把本次规划中已核验的独立制造工区转到其他纬带。使用：可迁星合同试摆新纬区。
// 边界：刚体旋转后重新装配和核验全部实体。地形、矿点和已有建筑仍由目标星的选址流程检查。

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// rotatedTemplateKey 区分原图及目标纬区，同一模板向南、向北的图纸各自核验。
type rotatedTemplateKey struct {
	first *blueprint.Building
	zone  string
}

// equatorialPlacementRegions 为本轮已建成的赤道工区选择转动中心。使用：多星工区另起模板前。
// 边界：海面或原有厂区会切断赤道工地，这时南北半环各试一次。第二至第四档沿用原纬区。
func equatorialPlacementRegions(regions []build.DensityLatitudeOption, fragmented bool) []build.DensityLatitudeOption {
	var targets []build.DensityLatitudeOption
	for _, region := range regions {
		switch {
		case region.Value == "equator" && fragmented:
			north, south := region, region
			north.Value, north.LatitudeLow = "equator-north", 0
			south.Value, south.LatitudeHigh = "equator-south", 0
			targets = append(targets, north, south)
		case region.Position == "center" && region.Tier >= 2 && region.Tier <= 4:
			targets = append(targets, region)
		}
	}
	return targets
}

// oppositeModuleTemplate 职责：寻找同设备、同额定产耗在对侧纬带已生成的原图。使用：新建独立模板之前。边界：无合格图时由调用方继续原搜索。
func oppositeModuleTemplate(ctx context.Context, cache *sync.Map, regions []build.DensityLatitudeOption, target build.DensityLatitudeOption, identity string) (moduleTemplate, string) {
	for _, region := range regions {
		if region.Tier != target.Tier || region.Position != target.Position || (region.LatitudeLow > 0) == (target.LatitudeLow > 0) {
			continue
		}
		return cachedRegionRotation(ctx, cache, region.Value, target, identity)
	}
	return moduleTemplate{}, ""
}

// cachedRegionRotation 只从实际通过的同规格原图构建目标纬区；取消不会登记为几何失败。
func cachedRegionRotation(ctx context.Context, cache *sync.Map, from string, target build.DensityLatitudeOption, identity string) (moduleTemplate, string) {
	for _, prefix := range []string{"", "relocate/", "center/"} {
		v, ok := cache.Load(prefix + from + "/" + identity)
		if !ok {
			continue
		}
		source := v.(moduleTemplate)
		if source.err != nil || source.generated.SurfacePlan == nil || source.generated.LogisticsSummary == nil || len(source.document.Buildings) == 0 {
			continue
		}
		key := rotatedTemplateKey{&source.document.Buildings[0], target.Value}
		stored, ok := cache.Load(key)
		if !ok {
			shape := rotateModuleTemplate(ctx, source, from, target)
			if ctx.Err() != nil {
				return moduleTemplate{}, ""
			}
			stored, _ = cache.LoadOrStore(key, shape)
		}
		shape := stored.(moduleTemplate)
		if len(shape.document.Buildings) > 0 {
			return shape, fmt.Sprintf("rotated/%p/%s", key.first, target.Value)
		}
	}
	return moduleTemplate{}, ""
}

// rotateModuleTemplate 职责：把赤道工区转到目标纬带中心，或将同档工区旋转半周到对侧。使用：复用独立设备布局。
// 边界：仅复用原编号等于建筑顺序的图纸，以保持货口索引、带路和塔槽的原合同。任一实体或引用无法核验时放弃这个形状。
func rotateModuleTemplate(ctx context.Context, source moduleTemplate, from string, target build.DensityLatitudeOption) moduleTemplate {
	step, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	origin := Position{Latitude: float64(float32(source.generated.SurfacePlan.AnchorLatitudeGrid) * float32(step))}
	objects, err := ProjectDocument(ctx, source.document, origin, "")
	if err != nil {
		return moduleTemplate{}
	}
	rotate := func(p Position, yaw float32) (Position, float32) {
		p.Latitude, p.Longitude = -p.Latitude, -p.Longitude
		return p, float32(math.Mod(float64(yaw)+540, 360))
	}
	if from == "equator" {
		rotate = turnEquatorialModule(objects, target, float64(step))
	}
	for i := range objects {
		o := &objects[i]
		if o.Building.Index != int32(i) {
			return moduleTemplate{}
		}
		o.First, o.Building.Yaw = rotate(o.First, o.Building.Yaw)
		o.Second, o.Building.Yaw2 = rotate(o.Second, o.Building.Yaw2)
	}
	rotated, err := Build(ctx, Request{Objects: objects, GameVersion: source.document.GameVersion})
	if err != nil {
		return moduleTemplate{}
	}
	doc, err := blueprint.Decode(rotated.Blueprint, true)
	if err != nil {
		return moduleTemplate{}
	}
	anchor := int(math.Round(rotated.Anchor.Latitude / float64(step)))
	ports := slices.Clone(source.generated.LogisticsSummary.ExternalPorts)
	for i := range ports {
		port := &ports[i]
		if port.BuildingIndex < 0 || port.BuildingIndex >= len(doc.Buildings) {
			return moduleTemplate{}
		}
		end := doc.Buildings[port.BuildingIndex]
		port.AreaIndex = int(end.AreaIndex)
		port.X, port.Y, port.Z = float64(end.LocalOffset.X), float64(end.LocalOffset.Y), float64(end.LocalOffset.Z)
	}
	return moduleTemplate{document: doc, generated: build.Result{
		SurfacePlan:      &build.SurfacePlan{LatitudeZone: target.Value, LatitudeZoneLabel: target.Label, AnchorLatitudeGrid: anchor, PrimaryAreaIndex: int(doc.PrimaryAreaIndex)},
		LogisticsSummary: &build.LogisticsSummary{ExternalPorts: ports},
	}}
}

// turnEquatorialModule 将工区中心移至零经度，再沿子午线旋转至指定纬度。每端随当地正北重算航向。
// 双端使用同一空间旋转，带路、分拣器和塔口保持相对位置。目标星的 Area 和几何在编码时重建。
func turnEquatorialModule(objects []Object, target build.DensityLatitudeOption, step float64) func(Position, float32) (Position, float32) {
	origin, span := longitudeExtent(objects, step)
	center := origin + span/2
	low, high := math.Inf(1), math.Inf(-1)
	for _, obj := range objects {
		low = min(low, obj.First.Latitude, obj.Second.Latitude)
		high = max(high, obj.First.Latitude, obj.Second.Latitude)
	}
	latitude := (target.LatitudeLow + target.LatitudeHigh) * math.Pi / 360
	sine, cosine := math.Sincos(latitude - (low+high)/2)
	return func(p Position, yaw float32) (Position, float32) {
		slon, clon := math.Sincos(p.Longitude - center)
		slat, clat := math.Sincos(p.Latitude)
		up := placement.Vector{clat * slon, slat, -clat * clon}
		longitude, latitude, angle := buildcore.RotateSurfacePose(up, sine, cosine)
		return Position{Longitude: longitude, Latitude: latitude, Height: p.Height}, float32(math.Mod(float64(yaw)+angle+360, 360))
	}
}
