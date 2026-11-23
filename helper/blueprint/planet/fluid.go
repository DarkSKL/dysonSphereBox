package planet

// 文件职责：从已知油井与可采液面生成真实采集和物流；使用：单星规划；边界：未知油井速率和水面探测不能视为供给。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// findOil 职责：在一个原版存档油井定位萃取站；使用：实时井口速率；边界：油井位置不吸附到普通建造格。
func findOil(ctx context.Context, req Request, options ProductionOptions, mine industryplan.Mine, points []veinPoint, catalog *placement.Catalog, exporting bool) ([]Object, []int, float64, error) {
	occupied := newObstacleIndex(extendOccupancy(req, catalog))
	minerals := newObstacleIndex(req.minerals)
	index := nextObjectIndex(req.Objects)
	var best []Object
	var chosen int
	bestRate := 0.0
	for i, vein := range points {
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		rate := vein.rate * mine.SpeedMultiplier
		if vein.kind != 7 || vein.item != int64(mine.ItemID) || vein.occupied || rate <= bestRate {
			continue
		}
		position := vein.p
		position.Height = 0
		pw, err := world(position)
		if err != nil || oilWellAt(pw, points) != i {
			continue
		}
		step, err := geometry.PasteLongitudeStep32(geometry.Radians(position.Latitude), geometry.Vanilla200)
		if err != nil {
			continue
		}
		b := blueprint.Building{Index: index, ItemID: 2307, ModelIndex: 61, InputObjIdx: -1, OutputObjIdx: -1}
		if occupied.conflicts(pw, recordRadius(catalog, b)) {
			continue
		}
		for _, yaw := range []float64{90, -90, 0, 180} {
			oil := Object{Building: b, First: position, Second: position, Item: mine.ItemID, Module: "油井"}
			oil.Building.Yaw, oil.Building.Yaw2 = float32(yaw), float32(yaw)
			proposed, capacity := minerTransport(oil, options, occupied, minerals, req.wet, catalog, exporting, yaw, float64(step))
			if available := min(capacity, rate); len(proposed) > 0 && available > bestRate {
				best, chosen, bestRate = proposed, i, available
			}
		}
	}
	if len(best) == 0 {
		return nil, nil, 0, nil
	}
	return best, []int{chosen}, bestRate, nil
}

// oilWellAt 职责：从实际三维粘贴点选择最近的原版油井；使用：铺带前核验萃取站确实命中预定井；边界：近邻按原始井点保守筛选，不以同经纬充当距离通过。
func oilWellAt(center geometry.WorldPoint, points []veinPoint) int {
	x, y, z := float32(center.X), float32(center.Y), float32(center.Z)
	length := float32(math.Sqrt(float64(x*x + y*y + z*z)))
	if length <= 0 {
		return -1
	}
	upX, upY, upZ := x/length, y/length, z/length
	best := float32(.1)
	index := -1
	for candidate, vein := range points {
		if vein.kind != 7 {
			continue
		}
		dx, dy, dz := float32(vein.v[0])-x, float32(vein.v[1])-y, float32(vein.v[2])-z
		// 原版邻域为 10 m。原版碰撞盒伸展有余量，
		// 这里要求井口本身已在距离内，避免用缺失的实时碰撞盒放宽。
		if dx*dx+dy*dy+dz*dz > 100 {
			continue
		}
		vertical := upX*dx + upY*dy + upZ*dz
		dx, dy, dz = dx-upX*vertical, dy-upY*vertical, dz-upZ*vertical
		// 原版选择横向距离平方小于 0.1 的最小值。
		if lateral := dx*dx + dy*dy + dz*dz; lateral < best {
			best, index = lateral, candidate
		}
	}
	return index
}

// liquidSiteCursor 指向同一批泵尚未排查的首格。已排除的岸线不变，已放泵的格子也不能再放一个泵。
type liquidSiteCursor struct{ row, col int }

// findLiquid 职责：搜索满足原版泵四陆点和四液点的真实岸线；使用：抽水和硫酸开采；边界：同一次必须找到不跨障碍的接驳路径。
func findLiquid(ctx context.Context, req Request, input ProductionRequest, mine industryplan.Mine, catalog *placement.Catalog, exporting bool, cursor *liquidSiteCursor) ([]Object, float64, error) {
	liquid := input.Resources.Terrain.WaterItemID
	if req.wet == nil || liquid == nil || int64(mine.ItemID) != *liquid || *liquid != 1000 && *liquid != 1116 {
		return nil, 0, &Error{PlanetID: req.PlanetID, Item: mine.ItemID, Code: "PLANET_LIQUID_UNKNOWN", Message: "抽水站所需的水或硫酸液面无法验证"}
	}
	occupied := newObstacleIndex(extendOccupancy(req, catalog))
	minerals := newObstacleIndex(req.minerals)
	index := nextObjectIndex(req.Objects)
	grid, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	b := blueprint.Building{Index: index, ItemID: 2306, ModelIndex: 60, InputObjIdx: -1, OutputObjIdx: -1}
	radius := recordRadius(catalog, b)
	for n := cursor.row; n <= 470; n++ {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		row := (n + 1) / 2
		if n%2 != 0 {
			row = -row
		}
		latitude := float64(float32(row) * float32(grid))
		step, err := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
		if err != nil {
			continue
		}
		width := int(math.Round(2 * math.Pi / float64(step)))
		firstCol := 0
		if n == cursor.row {
			firstCol = cursor.col
		}
		for col := firstCol; col < width; col++ {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			p := Position{Latitude: latitude, Longitude: float64(col) * float64(step)}
			for _, yaw := range []float64{90, -90, 0, 180} {
				if !pumpFits(p, yaw, req.wet) || req.pumpSite != nil && !req.pumpSite(p, yaw) {
					continue
				}
				pw, err := world(p)
				if err != nil || occupied.conflicts(pw, radius) || minerals.conflicts(pw, radius) {
					continue
				}
				pump := Object{Building: b, First: p, Second: p, Item: mine.ItemID, Module: "液体采集"}
				pump.Building.Yaw, pump.Building.Yaw2 = float32(yaw), float32(yaw)
				proposed, capacity := minerTransport(pump, input.Options, occupied, minerals, req.wet, catalog, exporting, yaw, float64(step))
				if len(proposed) != 0 {
					*cursor = liquidSiteCursor{n, col + 1}
					return proposed, min(capacity, 50*mine.SpeedMultiplier), nil
				}
			}
		}
	}
	return nil, 0, nil
}

// pumpSurfaceFits 职责：按模型 60 的陆脚和取水口核对碰撞面；使用：找泵位和装配终检；边界：水深须超过原版 0.27 米并预留地形生成误差。
func pumpSurfaceFits(p Position, yaw float64, terrain saveanalysis.PlanetTerrain) bool {
	if terrain.WaterHeight == nil {
		return false
	}
	height := func(spot Position) (float64, bool) {
		v, err := world(spot)
		if err != nil {
			return 0, false
		}
		return galaxy.TerrainSurfaceRadius(galaxy.HeightGrid{Precision: terrain.HeightPrecision, Heights: terrain.HeightCentimeters},
			terrain.ModData, galaxy.Vector{X: float64(v.X), Y: float64(v.Y), Z: float64(v.Z)})
	}
	return pumpLandClear(p, yaw, func(spot Position) bool {
		h, known := height(spot)
		return known && h >= 200-.20
	}) && pumpIntakeClear(p, yaw, *terrain.WaterHeight, func(spot Position) bool {
		h, known := height(spot)
		return known && 200+*terrain.WaterHeight-h > .27+.05
	})
}

// pumpFits 职责：按原版 PrefabDesc.BuildConditionConfig 判断岸线；使用：build 25955035 的 water-pump；边界：缺失探测点直接淘汰。
func pumpFits(p Position, yaw float64, wet func(Position) (bool, bool)) bool {
	return pumpLandClear(p, yaw, func(spot Position) bool {
		water, known := wet(spot)
		return known && !water
	}) && pumpIntakeClear(p, yaw, 0, func(spot Position) bool {
		water, known := wet(spot)
		return known && water
	})
}

// pumpLandClear 核对水泵模型 60 的四个陆侧支脚。
func pumpLandClear(p Position, yaw float64, check func(Position) bool) bool {
	return pumpPoints(p, yaw, 0, [][2]float64{{-.7, 2.8}, {.7, 2.8}, {1.05, 1.95}, {-1.05, 1.95}}, check)
}

// pumpIntakeClear 在原版模型 60 的四个取水点检查深度；铺地时同样保留这四处水面。
func pumpIntakeClear(p Position, yaw, waterLevel float64, check func(Position) bool) bool {
	return pumpPoints(p, yaw, waterLevel, [][2]float64{{-.4, -2.6}, {.4, -2.6}, {-1.25, -1.8}, {1.25, -1.8}}, check)
}

// pumpPoints 把原版模型中的局部米数投影到这台泵的经纬与朝向。
func pumpPoints(p Position, yaw, level float64, points [][2]float64, check func(Position) bool) bool {
	s, c := math.Sincos(yaw * math.Pi / 180)
	sinLat, cosLat := math.Sincos(p.Latitude)
	radius := 200 + float64(geometry.BaseLift) + level
	for _, point := range points {
		east, north := point[0]*c+point[1]*s, -point[0]*s+point[1]*c
		// 原版从泵模型的切平面沿球心方向发射射线，高纬度同样按实际径向投影。
		meridian := radius*cosLat - north*sinLat
		spot := Position{Longitude: p.Longitude + math.Atan2(east, meridian), Latitude: math.Atan2(radius*sinLat+north*cosLat, math.Hypot(meridian, east))}
		if !check(spot) {
			return false
		}
	}
	return true
}
