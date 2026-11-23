package planet

// 文件职责：将同星绝对端点装配到原版纬带 Area；使用：单星蓝图导出；边界：以固定零度朝向粘贴。

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
)

// band 职责：表示一段经度格数恒定的纬带；使用：原版 Area 及地基格；边界：纬格为闭区间。
type band struct{ low, high, segments int }

// Build 职责：生成一份原版整星蓝图；使用：已经落位的生产与物流对象；边界：任何必需校验失败则不返回蓝图。
func Build(ctx context.Context, request Request) (Result, error) {
	fail := func(object Object, code, message string) (Result, error) {
		return Result{}, &Error{PlanetID: request.PlanetID, Item: object.Item, Module: object.Module, Code: code, Message: message}
	}
	if request.Gas {
		return fail(Object{}, "PLANET_GAS_NATIVE_UNSUPPORTED", "原版 25955035 在气态巨星禁用蓝图粘贴，轨道采集器需在游戏中单独部署")
	}
	if len(request.Objects) == 0 {
		return fail(Object{}, "PLANET_EMPTY", "没有可装配建筑")
	}
	full := request.Foundation == "planet"
	switch request.Foundation {
	case "", "none", "construction", "planet":
	default:
		return fail(Object{}, "PLANET_FOUNDATION_MODE", "无法识别地基模式")
	}
	if (full || request.Foundation == "construction") && !request.TerrainKnown {
		return fail(Object{}, "PLANET_TERRAIN_UNKNOWN", "地形保护格缺失")
	}
	latitudeStep, err := geometry.LatitudeStep32(geometry.Vanilla200)
	if err != nil {
		return Result{}, err
	}
	step := float64(latitudeStep)
	low, high := 249, -249
	records := make([]blueprint.Building, len(request.Objects))
	indices := make(map[int32]int, len(records))
	for i, obj := range request.Objects {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if !validPosition(obj.First) || !validPosition(obj.Second) {
			return fail(obj, "PLANET_POSITION", "建筑端点缺少有效经纬或高度")
		}
		if obj.Building.Index < 0 {
			return fail(obj, "PLANET_INDEX", "建筑编号须为非负数")
		}
		if _, exists := indices[obj.Building.Index]; exists {
			return fail(obj, "PLANET_INDEX", "建筑编号重复")
		}
		indices[obj.Building.Index] = i
		for _, p := range []Position{obj.First, obj.Second} {
			row := int(math.Round(p.Latitude / step))
			low, high = min(low, row), max(high, row)
		}
	}
	if low < -249 || high > 249 {
		return fail(Object{}, "PLANET_POLAR_POINT", "极点独立网格未通过落位校验")
	}
	if full {
		low, high = -249, 249
	} else if request.Foundation == "construction" {
		low, high = max(-249, low-12), min(249, high+12)
	}
	bands := latitudeBands(low, high)
	primary := 0
	for i, b := range bands {
		if b.segments > bands[primary].segments {
			primary = i
		}
	}
	anchor := Position{Latitude: float64(float32(bands[primary].low) * float32(step))}
	span := 2 * math.Pi
	if !full {
		commonSegments := bands[primary].segments
		for _, b := range bands {
			commonSegments = gcd(commonSegments, b.segments)
		}
		// 原版会拒绝子 Area 原点未对齐经向格的粘贴；共用经度须处于所有纬带的公约格。
		primaryStep := float64(float32(2*float32(math.Pi)) / float32(bands[primary].segments*5))
		commonStep := primaryStep * float64(bands[primary].segments/commonSegments)
		anchor.Longitude, span = longitudeExtent(request.Objects, commonStep)
		if request.Foundation == "construction" {
			padding := math.Ceil(12*primaryStep/commonStep) * commonStep
			anchor.Longitude -= padding
			span = min(2*math.Pi, span+2*padding)
		}
	}
	areas := make([]blueprint.Area, len(bands))
	for i, b := range bands {
		parent, offset := -1, 0
		if i < primary {
			parent = i + 1
		} else if i > primary {
			parent = i - 1
		}
		if parent >= 0 {
			offset = b.low - bands[parent].low
		}
		areaWidth := min(b.segments*5, int(math.Ceil(span*float64(b.segments*5)/(2*math.Pi)))+1)
		areas[i] = blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(b.segments),
			AnchorLocalOffsetY: int16(offset), Width: int16(areaWidth), Height: int16(b.high - b.low + 1)}
	}
	if err := validateAreaAnchor(anchor, bands, step); err != nil {
		return fail(Object{}, "PLANET_AREA_GRID", err.Error())
	}
	for i, obj := range request.Objects {
		n := 0
		row := int(math.Round(obj.First.Latitude / step))
		for row > bands[n].high {
			n++
		}
		b := bands[n]
		longitudeStep := float64(float32(2*float32(math.Pi)) / float32(b.segments*5))
		first := positiveAngle(obj.First.Longitude - anchor.Longitude)
		second := first + math.Remainder(obj.Second.Longitude-obj.First.Longitude, 2*math.Pi)
		record := obj.Building
		record.Index, record.AreaIndex = int32(i), int8(n)
		record.LocalOffset = localOffset(obj.First, first, b.low, step, longitudeStep)
		record.LocalOffset2 = localOffset(obj.Second, second, b.low, step, longitudeStep)
		for _, link := range []*int32{&record.InputObjIdx, &record.OutputObjIdx} {
			if *link == -1 {
				continue
			}
			target, exists := indices[*link]
			if !exists || target == i {
				return fail(obj, "PLANET_LINK", fmt.Sprintf("连接引用 %d 无效", *link))
			}
			*link = int32(target)
		}
		records[i] = record
	}
	if err := validatePlacement(ctx, request, records); err != nil {
		return Result{}, err
	}
	doc := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Patch: 1,
		Name: request.Name, GameVersion: request.GameVersion, Areas: areas, Buildings: records,
		PrimaryAreaIndex: int32(primary), CursorTargetArea: int32(primary),
		Description: fmt.Sprintf("按固定经纬度和零度朝向粘贴。经度 %.6f°，纬度 %.6f°。", math.Remainder(anchor.Longitude, 2*math.Pi)*180/math.Pi, anchor.Latitude*180/math.Pi),
		DragBoxSize: blueprint.PointInt{X: int32(areas[primary].Width), Y: int32(high - low + 1)}}
	if err := validateNative(ctx, doc, bands[primary].low); err != nil {
		return fail(Object{}, "PLANET_GEOMETRY", err.Error())
	}
	count := 0
	if full || request.Foundation == "construction" {
		doc.Reform, count, err = foundation(ctx, request, bands, areas, step, anchor.Longitude)
		if err != nil {
			return Result{}, err
		}
	}
	encoded, err := blueprint.Encode(doc, 1)
	if err != nil {
		return fail(Object{}, "PLANET_ENCODE", err.Error())
	}
	droneItems, localModes := localLogistics(records)
	result := Result{Blueprint: encoded.Blueprint, Anchor: anchor, BuildingCount: len(records), AreaCount: len(areas), FoundationCells: count,
		ItemCounts: map[int16]int{}, StationStorage: map[int]int{}, LocalDroneItems: droneItems, LocalModes: localModes}
	if count != 0 && request.soilUpper != nil {
		result.SoilEstimateKnown, result.SoilUpperBound = true, *request.soilUpper
	}
	result.Anchor.Longitude = math.Remainder(result.Anchor.Longitude, 2*math.Pi)
	for _, record := range records {
		result.ItemCounts[record.ItemID]++
		if record.ItemID != 2104 || len(record.Parameters) != 2048 {
			continue
		}
		result.StationCount++
		for slot := range 5 {
			base := slot * 6
			if item := int(record.Parameters[base]); item > 0 {
				result.StationStorage[item] += int(record.Parameters[base+3])
			}
		}
	}
	return result, nil
}

// validateAreaAnchor 职责：复核原版纬带及经纬格锚点容差；使用：整星编码的发布门；边界：不接收旋转或任意自定义行星。
func validateAreaAnchor(anchor Position, bands []band, latitudeStep float64) error {
	latIndex := float64(float32(anchor.Latitude) / float32(latitudeStep))
	if math.Abs(latIndex-math.Round(latIndex)) > .01 {
		return fmt.Errorf("主粘贴锚点不在原版纬向整数细格")
	}
	for i, b := range bands {
		latitude := geometry.Radians(float32(b.low) * float32(latitudeStep))
		longitudeStep, err := geometry.PasteLongitudeStep32(latitude, geometry.Vanilla200)
		if err != nil {
			return err
		}
		longitudeCells := float64(float32(anchor.Longitude) / float32(longitudeStep))
		// GenerateAreaGratBoxByBPData 把子原点偏离经度格超过 .01 记作禁止条件 z。
		if math.Abs(longitudeCells-math.Round(longitudeCells)) > .01 {
			return fmt.Errorf("Area %d 起点经度没有对齐原版经向整数细格", i)
		}
		segments, _ := geometry.LongitudeSegmentsAtGrid(b.high, geometry.Vanilla200)
		if segments != b.segments {
			return fmt.Errorf("Area %d 的结束纬度越过其原版纬带", i)
		}
	}
	return nil
}

// longitudeExtent 职责：选择最宽空白经区作为接缝；使用：缩小拖拽框和保证单次放置；边界：原点吸附至主纬带格。
func longitudeExtent(objects []Object, step float64) (origin, span float64) {
	values := make([]float64, 0, len(objects)*2)
	for _, o := range objects {
		values = append(values, positiveAngle(o.First.Longitude), positiveAngle(o.Second.Longitude))
	}
	slices.Sort(values)
	next, widest := values[0], values[0]+2*math.Pi-values[len(values)-1]
	for i := 1; i < len(values); i++ {
		if gap := values[i] - values[i-1]; gap > widest {
			widest, next = gap, values[i]
		}
	}
	origin = math.Floor(next/step) * step
	for _, value := range values {
		span = max(span, positiveAngle(value-origin))
	}
	return origin, span
}

// gcd 职责：取得原生经度格数的公约数；使用：兼容段不同比例的 Area 原点；边界：参数均为正数。
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// latitudeBands 职责：切分连续原版纬带；使用：保留相邻 Area 父链；边界：传入有效纬格范围。
func latitudeBands(low, high int) []band {
	var bands []band
	for row := low; row <= high; row++ {
		segments, _ := geometry.LongitudeSegmentsAtGrid(row, geometry.Vanilla200)
		if len(bands) == 0 || bands[len(bands)-1].segments != segments {
			bands = append(bands, band{low: row, high: row, segments: segments})
		} else {
			bands[len(bands)-1].high = row
		}
	}
	return bands
}

// validPosition 职责：识别可序列化球面坐标；使用：端点及占用输入；边界：纬度限于有效极点之间。
func validPosition(p Position) bool {
	return !math.IsNaN(p.Longitude) && !math.IsInf(p.Longitude, 0) && math.Abs(p.Latitude) <= math.Pi/2 && math.Abs(p.Height) <= 128
}

// positiveAngle 职责：将经度折叠到正向回环；使用：统一原版 Area 的本地零经度；边界：不改变球面位置。
func positiveAngle(v float64) float64 {
	v = math.Mod(v, 2*math.Pi)
	if v < 0 {
		v += 2 * math.Pi
	}
	return v
}

// localOffset 职责：将球面端点投影至所在 Area；使用：双端点编码；边界：经向先按首端解开接缝。
func localOffset(p Position, longitude float64, low int, latStep, lonStep float64) blueprint.Point {
	return blueprint.Point{X: float32(longitude / lonStep), Y: float32(p.Latitude/latStep - float64(low)), Z: float32(p.Height)}
}
