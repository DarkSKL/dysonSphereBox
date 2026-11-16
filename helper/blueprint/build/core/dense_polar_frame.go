package build

// 文件职责：固定局部单元与跨单元布线使用的真实目标球面坐标。
// 使用：全场落位后、寻路前；边界：单元内部采用仿射格距补偿，不在整厂完工后逐行剪切机器与分拣器。

import (
	"DysonSphereBox/helper/blueprint"
	buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
)

// densePolarFrame 职责：记录目标纬度和主区经向格长；使用：球面路由及正式编码；边界：仅原版半径200、零经度起点。
type densePolarFrame struct {
	start, height, segments     int
	latitudeStep, longitudeStep float32
	// flipHemisphere 职责：在路由结束后以球面半周旋转映射回目标极区；使用：设施对称候选；边界：只写出前应用一次。
	flipHemisphere bool
	// longitudeBudget 职责：限制矩形经向搜索范围；使用：大厂或显式邻带装箱；边界：零值保留既有极区整圈预算。
	longitudeBudget float64
	// position 职责：保存非极区矩形从中心或南北边缘展开的顺序；使用：大厂球面装箱；边界：空值保留极区既有顺序。
	position string
	// compactLongitude 职责：跨纬度比较最短经向右界；使用：独立均衡矩形及邻带候选；边界：既有严格选区候选不改变扫描顺序。
	compactLongitude bool
}

// longitudeLimit 职责：预留主Area整数边界与两侧八格接线预算；使用：极区单元装箱及路由共享范围；边界：不增加整圈容量，不把跨缝实体藏到区域之外。
func (frame densePolarFrame) longitudeLimit() float64 {
	rootCell := float64(frame.segments) / float64(longitudeSegmentCount(frame.start, 200))
	limit := float64(frame.segments*5) - rootCell - 16
	if frame.longitudeBudget > 0 {
		limit = math.Min(limit, frame.longitudeBudget)
	}
	return limit
}

// unitScale 职责：按生产阵列最窄处或独立物流设施中心计算格距；使用：定位与纬向避让；边界：物流端口不随无关塔的纬度放大，独立塔间距另用世界距离限制。
func (frame densePolarFrame) unitScale(unit denseSpatialUnit, low, high vector) float64 {
	latitude := math.Abs(float64(frame.start) + (low.y+high.y)/2)
	for _, p := range unit.entities {
		if p.record.RecipeID > 0 {
			latitude = math.Max(math.Abs(float64(frame.start)+low.y), math.Abs(float64(frame.start)+high.y))
			break
		}
	}
	return float64(frame.segments) / 200 / math.Cos(latitude*math.Pi/500)
}

// world 职责：用原生float32公式计算球面点；使用：静态盒、带节点和连接距离；边界：lift为节点球抬升，不改变蓝图层高。
func (frame densePolarFrame) world(p vector, lift float64) placement.Vector {
	if math.IsNaN(lift) || math.IsInf(lift, 0) {
		panic("原生内部节点抬升必须为有限世界长度")
	}
	transform := frame.geometryFrame()
	world, err := transform.ProjectNative32(buildgeometry.GridPoint{Frame: transform.Grid, East: buildgeometry.GridLength(p.x), North: buildgeometry.GridLength(p.y), Height: buildgeometry.HeightLayers(p.z)})
	if err != nil {
		// 外部输入和frame建立时已验证，内部有界网格只产生有限位置；违例是调用者破坏几何不变量，不能回退NaN或原点。
		panic(err)
	}
	point := placement.Vector{float64(world.X), float64(world.Y), float64(world.Z)}
	if lift != 0 {
		point = point.Add(surfaceUnit(point).Scale(lift))
	}
	return point
}

// geometryFrame 职责：交接原生固定步长与float32纬度原点；使用：极区世界点和构造验证；边界：与实际粘贴的按落点纬带选步长保持不同入口。
func (frame densePolarFrame) geometryFrame() buildgeometry.SurfaceFrame {
	return buildgeometry.SurfaceFrame{Grid: buildgeometry.Frame{Planet: buildgeometry.Vanilla200, ID: "native-polar-root", Space: buildgeometry.SurfaceGrid}, Origin: buildgeometry.SurfacePosition{Frame: buildgeometry.PlanetWorldFrame(), Latitude: buildgeometry.Radians(float32(frame.start) * frame.latitudeStep)}, LongitudeStep: buildgeometry.Radians(frame.longitudeStep), LatitudeStep: buildgeometry.Radians(frame.latitudeStep)}
}

// blockNode 职责：在真实球面净距球内登记整数格障碍；使用：原有及新增带节点；边界：平面候选保持既有行为，空间步长按当前纬度计算。
func (index *denseRoutePortIndex) blockNode(blocked map[vector]bool, p vector) {
	if index.polarFrame == nil {
		blockDenseBeltNode(blocked, p)
		return
	}
	frame := index.polarFrame
	width := math.Abs(math.Cos((float64(frame.start)+p.y)*math.Pi/500) * 200.2 * float64(frame.longitudeStep))
	radiusX := math.Ceil(.35 / math.Max(.01, width))
	world := index.world(p)
	for x := math.Floor(p.x - radiusX); x <= math.Ceil(p.x+radiusX); x++ {
		for y := math.Floor(p.y - 1); y <= math.Ceil(p.y+1); y++ {
			for z := math.Max(0, math.Floor(p.z-1)); z <= math.Ceil(p.z+1); z++ {
				q := vector{x, y, z}
				delta := index.world(q).Sub(world)
				if delta.Dot(delta) < .35*.35 {
					blocked[q] = true
				}
			}
		}
	}
}

// placePolarUnitFrames 职责：按各单元目标纬度补偿内部格距，再交给真实球面路由；使用：极区及大厂布局候选；边界：所有端口、cargo目标和附属设施随同一单元移动。
func (b *builder) placePolarUnitFrames(ctx context.Context, units []denseSpatialUnit) error {
	root, _ := ctx.Value(densityPolarRootKey{}).(bool)
	if !root || b.polarFrame != nil {
		return nil
	}
	zone, _ := ctx.Value(densityLatitudeKey{}).(string)
	selection, _ := densityLatitudeSelectionFor(zone)
	flipHemisphere, _ := ctx.Value(densityPolarMirrorKey{}).(bool)
	if flipHemisphere {
		selection.hemisphere = -selection.hemisphere
		if selection.position == "north" {
			selection.position = "south"
		} else if selection.position == "south" {
			selection.position = "north"
		}
	}
	expand, _ := ctx.Value(denseAdjacentLatitudeKey{}).(bool)
	bands := denseLatitudeAllowedBands(selection, expand)
	budget, _, err := densityContextBand(ctx, zone, 200)
	if err != nil {
		return err
	}
	if len(bands) == 0 {
		return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "极区缺少原生子环"}
	}
	if selection.tier == 5 && selection.hemisphere < 0 && selection.position == "south" {
		// 南极侧让较大单元朝赤道排列；整单元半周旋转保留槽位手性和内部连接。
		rotatePlacedQuarter(b.all, 2)
		minimum, _ := bounds(b.all)
		shift(b.all, -math.Floor(minimum.x), -math.Floor(minimum.y), 0)
		seen := map[*transportRow]bool{}
		for _, unit := range units {
			for _, row := range unit.transportRows() {
				if !seen[row] {
					row.quarterTurns = (row.quarterTurns + 2) % 4
					seen[row] = true
				}
			}
		}
	}
	compact := denseBalancedFrameEnabled(ctx) && (selection.tier < 5 || expand) && (len(units) >= 64 || expand && len(units) > 1)
	if compact {
		if err := b.reserveDenseUnitPower(ctx, units); err != nil {
			return err
		}
	}
	for _, entity := range b.all {
		if err := geometryGridPoint(entity.a).Validate(); err != nil {
			return err
		}
		if err := geometryGridPoint(entity.b).Validate(); err != nil {
			return err
		}
	}
	height := budget.high - budget.low + 1
	low, high := bounds(b.all)
	// 按实体包络加接线余量定位，不能因借用较大搜索预算而偏离所选中心或边缘。
	if !compact {
		height = min(height, int(math.Ceil(high.y-low.y))+5)
	}
	referenceLow, referenceHigh := bands[0].low, bands[len(bands)-1].high
	if expand {
		referenceLow, referenceHigh = denseLatitudeReferenceRange(selection)
	}
	start := (referenceLow + referenceHigh - height + 1) / 2
	if selection.position == "south" {
		start = referenceLow
		if expand {
			start += 2
		}
		if selection.tier == 5 && selection.hemisphere < 0 {
			// 极点最内侧只剩少量格子；预留四行实体半径，避免把局部单元中心压在极点上。
			start += 4
		}
	}
	if selection.position == "north" {
		start = referenceHigh - height + 1
		if expand {
			start -= 2
		}
	}
	latitudeRadians, err := buildgeometry.LatitudeStep32(buildgeometry.Vanilla200)
	if err != nil {
		return err
	}
	step := float32(latitudeRadians)
	segments := 0
	for y := start; y < start+height; y++ {
		segments = max(segments, longitudeSegmentCount(y, 200))
	}
	// 寻路采用覆盖区最细的经向格网；若直接沿南极带头的粗网格，靠赤道一侧一步便可能超过合法带长。
	// 写出前统一换算回主Area原生格距，保持同一球面位置。
	frame := &densePolarFrame{start: start, height: height, segments: segments, latitudeStep: step, longitudeStep: float32(2*float32(math.Pi)) / float32(segments*5), flipHemisphere: flipHemisphere, compactLongitude: compact}
	transform := frame.geometryFrame()
	if _, err := transform.ProjectNative32(buildgeometry.GridPoint{Frame: transform.Grid}); err != nil {
		return err
	}
	if selection.tier < 5 || expand {
		frame.position = selection.position
		frame.longitudeBudget = denseTargetLongitudeBudget(frame, units)
	}
	// 均衡候选将逐单元重新装箱；临时平面整体高度不代表最终纬向容量。
	if !compact && high.y-low.y+1 > float64(height) {
		return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "极区单元排向超过预留纬向容量"}
	}
	center := (low.x + high.x) / 2
	minimum := math.Inf(1)
	moved := map[*placed]bool{}
	for _, unit := range units {
		a, c := bounds(unit.entities)
		// 单元不能跨过同一纬线整圈；对平面排版落得过近极点的长单元，先沿纬向移到有真实容量的位置。
		// 仅在超整圈时移动，既有可容纳单元保留原位置；之后仍按球面实体逐一避碰。
		for tries := 0; tries < height; tries++ {
			scale := frame.unitScale(unit, a, c)
			if (c.x-a.x)*scale < float64(frame.segments*5)*.9 {
				break
			}
			dy := -float64(selection.hemisphere)
			if a.y+dy < 0 || c.y+dy > float64(height-1) {
				break
			}
			shift(unit.entities, 0, dy, 0)
			a.y += dy
			c.y += dy
		}
		cx, cy := (a.x+c.x)/2, (a.y+c.y)/2
		centerScale := float64(frame.segments) / 200 / math.Cos((float64(start)+cy)*math.Pi/500)
		scale := frame.unitScale(unit, a, c)
		origin := (cx - center) * centerScale
		for _, p := range unit.entities {
			if moved[p] {
				continue
			}
			moved[p] = true
			p.a.x = origin + (p.a.x-cx)*scale
			p.b.x = origin + (p.b.x-cx)*scale
			p.densityCargo.relative.x *= scale
			p.nativeUnitScale = scale
			lo, _ := footprint(p.record.ItemID)
			minimum = math.Min(minimum, math.Min(p.a.x, p.b.x)+lo.x)
		}
	}
	shift(b.all, -math.Floor(minimum), 0, 0)
	b.polarFrame = frame
	b.denseLatitudeSpan = float64(height)
	b.nativeLatitudeScaled, b.nativeLongitudeScale = true, 1
	if err := b.resamplePolarLocalBelts(ctx, units); err != nil {
		return err
	}
	return b.repairPolarUnitOccupancy(ctx, units)
}
