package build

// 文件职责：从定稿Area及真实记录恢复区域归属、坐标框架和实际纬带事实。
// 使用：区域枚举及单区域前检；边界：只读快照，委托既有球面变换和共同geometry事实，不复制物理分段表。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"
)

// RegionLatitudeBand 职责：描述区域实际成员覆盖的一个原生纬带；使用：区域流程条件；边界：仅为真实坐标事实，不代表整厂可建证明。
type RegionLatitudeBand struct {
	// MinimumGrid/MaximumGrid 职责：保存共同表中的纬带闭区间；使用：解释实际成员；边界：不是请求选区。
	MinimumGrid int `json:"minimumGrid"`
	MaximumGrid int `json:"maximumGrid"`
	// LongitudeSegments 职责：保存共同表的经度段数；使用：坐标事实；边界：不自行定义格距。
	LongitudeSegments int `json:"longitudeSegments"`
	// Class 职责：标识equatorial、temperate或polar；使用：区域条件；边界：按实际纬带归类。
	Class string `json:"class"`
}

// RegionDescriptor 职责：交接真实编码区域及其实际成员的只读值副本；使用：引擎区域遍历；边界：修改副本不改变快照，空中转Area也单独列出。
type RegionDescriptor struct {
	// ID 职责：保存候选内稳定区域标识；使用：单区域检查；边界：必须结合快照身份，不能跨候选证明。
	ID string `json:"id"`
	// AreaIndex/ParentAreaIndex 职责：保存实际编码Area及父链；使用：成员和跨区引用；边界：不从用户选区推断。
	AreaIndex       int `json:"areaIndex"`
	ParentAreaIndex int `json:"parentAreaIndex"`
	// Class 职责：标识planar、equatorial、temperate、polar或multi-band；使用：区域工作流条件；边界：multi-band不能冒充单条原生纬带。
	Class string `json:"class"`
	// CoordinateFrame 职责：区分planar、surface-area、native-root；使用：坐标检查诊断；边界：不能混用根坐标和分区局部坐标。
	CoordinateFrame string `json:"coordinateFrame"`
	// BuildingCount 职责：统计实际AreaIndex归属的记录；使用：区域进度；边界：不从SurfacePlan旧计数推断。
	BuildingCount int `json:"buildingCount"`
	// MinimumLatitudeGrid/MaximumLatitudeGrid 职责：保存实际成员端点的世界纬向范围；使用：区域事实；边界：planar无真实纬度，固定返回零。
	MinimumLatitudeGrid float64 `json:"minimumLatitudeGrid"`
	MaximumLatitudeGrid float64 `json:"maximumLatitudeGrid"`
	// Bands 职责：保存上述实际范围覆盖的原生纬带副本；使用：跨带事实；边界：不等同原请求许可范围。
	Bands []RegionLatitudeBand `json:"bands,omitempty"`
}

// regionGeometryView 职责：建立当前调用的只读编号与Area索引；使用：区域检查局部计算；边界：不存入快照或全局缓存。
type regionGeometryView struct {
	encoding *surfaceEncoding
	areas    []blueprint.Area
	records  []blueprint.Building
	byIndex  map[int32]int
	members  [][]int
}

// regionFailure 职责：统一报告明确的区域错误类别；使用：纯区域前检；边界：不把失败转成空的通过证据。
func regionFailure(code, message string, args ...any) error {
	return &blueprint.Error{Code: code, Message: fmt.Sprintf(message, args...)}
}

// regionAreaID 职责：给真实Area建立稳定候选内ID；使用：枚举和边界报告；边界：不作为跨候选内容身份。
func regionAreaID(index int) string { return fmt.Sprintf("area/%d", index) }

// readRegionGeometry 职责：验证编号归属和Area父链并选择真实坐标适配；使用：区域入口；边界：不运行全图碰撞、物流证明或修改编码锚点。
func readRegionGeometry(ctx context.Context, snapshot GeometrySnapshot) (*regionGeometryView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := checkFinalGeometry(ctx, snapshot); err != nil {
		return nil, err
	}
	document := snapshot.data.document
	if len(document.Areas) == 0 || document.PrimaryAreaIndex < 0 || int(document.PrimaryAreaIndex) >= len(document.Areas) {
		return nil, regionFailure("GEN_E_REGION_COORDINATES", "区域集合或主区域无效")
	}
	view := &regionGeometryView{areas: document.Areas, records: document.Buildings, byIndex: make(map[int32]int, len(document.Buildings)), members: make([][]int, len(document.Areas))}
	for i, area := range document.Areas {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if int(area.Index) != i || area.Width <= 0 || area.Height <= 0 || area.AreaSegments <= 0 {
			return nil, regionFailure("GEN_E_REGION_COORDINATES", "区域%d编号、尺寸或分段无效", i)
		}
		if i == int(document.PrimaryAreaIndex) {
			if area.ParentIndex != -1 {
				return nil, regionFailure("GEN_E_REGION_COORDINATES", "主区域%d不能有父区域", i)
			}
		} else if area.ParentIndex < 0 || int(area.ParentIndex) >= len(document.Areas) || int(area.ParentIndex) == i {
			return nil, regionFailure("GEN_E_REGION_COORDINATES", "区域%d父索引无效", i)
		}
		seen := make(map[int]bool, len(document.Areas))
		for current := i; current != int(document.PrimaryAreaIndex); current = int(document.Areas[current].ParentIndex) {
			if current < 0 || current >= len(document.Areas) || seen[current] {
				return nil, regionFailure("GEN_E_REGION_COORDINATES", "区域%d父链循环或未到达主区域", i)
			}
			seen[current] = true
		}
	}
	for i, record := range document.Buildings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if record.Index < 0 {
			return nil, regionFailure("GEN_E_REGION_REFERENCE", "建筑编号%d无效", record.Index)
		}
		if _, exists := view.byIndex[record.Index]; exists {
			return nil, regionFailure("GEN_E_REGION_REFERENCE", "建筑编号%d重复", record.Index)
		}
		area := int(record.AreaIndex)
		if area < 0 || area >= len(view.members) {
			return nil, regionFailure("GEN_E_REGION_REFERENCE", "建筑%d所属区域%d不存在", record.Index, area)
		}
		view.byIndex[record.Index] = i
		view.members[area] = append(view.members[area], i)
	}
	if snapshot.data.surface == nil || snapshot.data.surface.planar {
		if len(document.Areas) != 1 {
			return nil, regionFailure("GEN_E_REGION_COORDINATES", "切平面快照不支持未经核对的多Area坐标")
		}
		view.encoding = newPlanarRuleEncoding()
	} else {
		encoding := *snapshot.data.surface
		if encoding.plan == nil || encoding.plan.GridSegment != defaultSurfaceGridSegment || encoding.plan.PrimaryAreaIndex != int(document.PrimaryAreaIndex) || len(encoding.areas) != len(document.Areas) {
			return nil, regionFailure("GEN_E_REGION_COORDINATES", "球面区域框架或分段未核对")
		}
		for i := range document.Areas {
			if encoding.areas[i] != document.Areas[i] {
				return nil, regionFailure("GEN_E_REGION_COORDINATES", "区域%d实际记录与球面框架不一致", i)
			}
		}
		encoding.positionSnapshot = nil
		view.encoding = &encoding
	}
	return view, nil
}

// regionFiniteRecord 职责：核对记录两端的有限位姿；使用：实际成员及直接边界邻居；边界：不只检查主端点，也不修改NaN或无穷大。
func regionFiniteRecord(record blueprint.Building) error {
	for _, value := range []float32{record.LocalOffset.X, record.LocalOffset.Y, record.LocalOffset.Z, record.LocalOffset2.X, record.LocalOffset2.Y, record.LocalOffset2.Z, record.Yaw, record.Pitch, record.Tilt, record.Yaw2, record.Pitch2, record.Tilt2} {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return regionFailure("GEN_E_REGION_COORDINATES", "建筑%d存在非有限坐标或旋转", record.Index)
		}
	}
	return nil
}

// regionPositions 职责：以正式float32映射求实际记录端点；使用：区域事实和前检；边界：直接调用只读映射，不建立或修改共享位置缓存。
func regionPositions(ctx context.Context, view *regionGeometryView, records []blueprint.Building) ([]vector, []vector, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := regionFiniteRecord(record); err != nil {
			return nil, nil, err
		}
		area := view.areas[int(record.AreaIndex)]
		maxX, maxY := float32(area.Width-1), float32(area.Height-1)
		if view.encoding.rootCoordinates && int(record.AreaIndex) == view.encoding.plan.PrimaryAreaIndex {
			maxX, maxY = view.encoding.entityBounds()
		}
		for _, point := range []blueprint.Point{record.LocalOffset, record.LocalOffset2} {
			if point.X < 0 || point.Y < 0 || point.X > maxX || point.Y > maxY {
				return nil, nil, regionFailure("GEN_E_REGION_RANGE", "建筑%d实际坐标超出区域%d的声明包络", record.Index, record.AreaIndex)
			}
		}
	}
	first, err := view.encoding.surfaceWorldPositionsFor(records, false)
	if err != nil {
		return nil, nil, regionFailure("GEN_E_REGION_COORDINATES", "区域主端点映射失败：%v", err)
	}
	second, err := view.encoding.surfaceWorldPositionsFor(records, true)
	if err != nil {
		return nil, nil, regionFailure("GEN_E_REGION_COORDINATES", "区域次端点映射失败：%v", err)
	}
	frame := buildgeometry.PlanetWorldFrame()
	if view.encoding.planar {
		frame = buildgeometry.PlanarWorldFrame()
	}
	for i := range records {
		for _, position := range []vector{first[i], second[i]} {
			point := buildgeometry.WorldPoint{Frame: frame, X: buildgeometry.WorldLength(position.x), Y: buildgeometry.WorldLength(position.y), Z: buildgeometry.WorldLength(position.z)}
			if err := point.Validate(); err != nil {
				return nil, nil, regionFailure("GEN_E_REGION_COORDINATES", "建筑%d世界坐标无效：%v", records[i].Index, err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return first, second, nil
}

// regionBandClass 职责：按已有五档原生纬带事实给出领域分类；使用：区域描述；边界：不复制纬带分段表或新建纬度阈值。
func regionBandClass(band surfacePackedBand) (string, error) {
	for tier := 1; tier <= 5; tier++ {
		for _, hemisphere := range []int{-1, 1} {
			for _, known := range densityLatitudeTierBands(tier, hemisphere, defaultSurfaceGridSegment) {
				if known == band {
					if tier == 1 {
						return "equatorial", nil
					}
					if tier == 5 {
						return "polar", nil
					}
					return "temperate", nil
				}
			}
		}
	}
	return "", regionFailure("GEN_E_REGION_COORDINATES", "原生纬带没有已核对的分类")
}

// describeRegion 职责：从实际成员世界坐标归纳区域和跨带事实；使用：真实Area遍历；边界：空中转Area使用自身编码范围，原生根不能用首条Band冒充全部成员。
func describeRegion(ctx context.Context, snapshot GeometrySnapshot, view *regionGeometryView, area int) (RegionDescriptor, error) {
	recordArea := snapshot.data.document.Areas[area]
	descriptor := RegionDescriptor{ID: regionAreaID(area), AreaIndex: area, ParentAreaIndex: int(recordArea.ParentIndex), BuildingCount: len(view.members[area]), Class: "planar", CoordinateFrame: "planar"}
	records := make([]blueprint.Building, 0, len(view.members[area]))
	for _, index := range view.members[area] {
		if err := ctx.Err(); err != nil {
			return RegionDescriptor{}, err
		}
		records = append(records, view.records[index])
	}
	if len(records) == 0 {
		records = []blueprint.Building{{AreaIndex: int8(area)}, {AreaIndex: int8(area), LocalOffset: blueprint.Point{Y: float32(recordArea.Height - 1)}}}
	}
	first, second, err := regionPositions(ctx, view, records)
	if err != nil {
		return RegionDescriptor{}, err
	}
	if view.encoding.planar {
		return descriptor, nil
	}
	descriptor.CoordinateFrame = "surface-area"
	if view.encoding.rootCoordinates && area == view.encoding.plan.PrimaryAreaIndex {
		descriptor.CoordinateFrame = "native-root"
	}
	latitudeStep, err := buildgeometry.LatitudeStep32(buildgeometry.Vanilla200)
	if err != nil {
		return RegionDescriptor{}, err
	}
	low, high := math.Inf(1), math.Inf(-1)
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return RegionDescriptor{}, err
		}
		points := []vector{first[i]}
		if regionIsInserter(record.ItemID) {
			points = append(points, second[i])
		}
		for _, point := range points {
			surface, err := buildgeometry.WorldToSurface(buildgeometry.WorldPoint{Frame: buildgeometry.PlanetWorldFrame(), X: buildgeometry.WorldLength(point.x), Y: buildgeometry.WorldLength(point.y), Z: buildgeometry.WorldLength(point.z)})
			if err != nil {
				return RegionDescriptor{}, regionFailure("GEN_E_REGION_COORDINATES", "区域%d世界纬度无效：%v", area, err)
			}
			latitude := float64(surface.Latitude) / float64(latitudeStep)
			low, high = math.Min(low, latitude), math.Max(high, latitude)
		}
	}
	descriptor.MinimumLatitudeGrid, descriptor.MaximumLatitudeGrid = low, high
	// 与共同粘贴格距的半值远离零规则一致，以实际端点归属原生细格；跨带空隙属于同一根区域包络。
	firstRow, lastRow := int(math.Round(low)), int(math.Round(high))
	covered := false
	for _, band := range surfaceLatitudeBands(defaultSurfaceGridSegment) {
		if band.high < firstRow || band.low > lastRow {
			continue
		}
		class, err := regionBandClass(band)
		if err != nil {
			return RegionDescriptor{}, err
		}
		descriptor.Bands = append(descriptor.Bands, RegionLatitudeBand{MinimumGrid: band.low, MaximumGrid: band.high, LongitudeSegments: band.segments, Class: class})
		if band.low <= firstRow && band.high >= firstRow {
			covered = true
		}
	}
	if !covered || len(descriptor.Bands) == 0 || descriptor.Bands[len(descriptor.Bands)-1].MaximumGrid < lastRow {
		return RegionDescriptor{}, regionFailure("GEN_E_REGION_COORDINATES", "区域%d实际成员越过已核对纬带范围", area)
	}
	descriptor.Class = descriptor.Bands[0].Class
	if len(descriptor.Bands) > 1 {
		descriptor.Class = "multi-band"
	}
	return descriptor, nil
}

// GeometryRegions 职责：枚举不可变定稿的真实编码区域和实际纬带；使用：独立测试和便利调用；边界：生产regions必须使用GeometryRegionsContext继承取消，返回独立值副本。
func GeometryRegions(snapshot GeometrySnapshot) ([]RegionDescriptor, error) {
	return GeometryRegionsContext(context.Background(), snapshot)
}

// GeometryRegionsContext 职责：可取消地枚举不可变定稿的真实区域事实；使用：生产regions及状态属性求值；边界：继承当前任务寿命，不用Background遮蔽父取消。
func GeometryRegionsContext(ctx context.Context, snapshot GeometrySnapshot) ([]RegionDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view, err := readRegionGeometry(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	result := make([]RegionDescriptor, len(view.members))
	for area := range view.members {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result[area], err = describeRegion(ctx, snapshot, view, area)
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// regionIsInserter 职责：识别现有原版分拣器记录种类；使用：选择真实第二端点及SlotPoses语义；边界：能力和跨度仍查询共同设备事实。
func regionIsInserter(item int16) bool { return item >= 2011 && item <= 2014 }
