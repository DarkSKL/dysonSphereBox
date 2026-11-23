package planet

// 文件职责：检查存量建筑和预建造空间；使用：选址及地基屏蔽；边界：采用已知存量范围的保守球体。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// obstacleBox 职责：保留存档占位及保守半径；使用：球面快速筛选；边界：不修改对象。
type obstacleBox struct {
	center geometry.WorldPoint
	radius float64
}

// stationObstacle 职责：保留物流塔中心及原型；使用：原版塔间最小距离核对；边界：不将专用塔距扩成普通设施禁建区。
type stationObstacle struct {
	center geometry.WorldPoint
	item   int16
}

// obstacleIndex 职责：按世界空间 16 米体素分别组织实体、地表和物流塔；使用：筛选候选落点；边界：实体范围可跨体素，物流塔只按中心登记。
type obstacleIndex struct {
	volume, surface map[[3]int][]obstacleBox
	stations        map[[3]int][]stationObstacle
}

// obstacleCell 职责：换算坐标所属体素；使用：空间哈希；边界：负坐标向下取整。
func obstacleCell(value float64) int { return int(math.Floor(value / 16)) }

// newObstacleIndex 职责：载入已知存档范围；使用：反复布局查询；边界：无效数据由装配入口拒绝。
func newObstacleIndex(obstacles []Obstacle) obstacleIndex {
	out := obstacleIndex{}
	for _, o := range obstacles {
		out.add(o)
	}
	return out
}

// add 职责：将新占用登记到当前体素索引；使用：递增电网搜索；边界：存档坏数据由装配入口统一拒绝。
func (idx *obstacleIndex) add(o Obstacle) {
	center, err := world(o.Position)
	if err != nil || o.Radius <= 0 || o.Radius > 100 {
		return
	}
	if o.stationItem == 2103 || o.stationItem == 2104 {
		if idx.stations == nil {
			idx.stations = make(map[[3]int][]stationObstacle)
		}
		key := [3]int{obstacleCell(float64(center.X)), obstacleCell(float64(center.Y)), obstacleCell(float64(center.Z))}
		idx.stations[key] = append(idx.stations[key], stationObstacle{center, o.stationItem})
	}
	target := &idx.volume
	if o.surfaceOnly {
		target = &idx.surface
		center = surfacePoint(center)
	}
	if *target == nil {
		*target = make(map[[3]int][]obstacleBox)
	}
	x, y, z := float64(center.X), float64(center.Y), float64(center.Z)
	for ix := obstacleCell(x - o.Radius); ix <= obstacleCell(x+o.Radius); ix++ {
		for iy := obstacleCell(y - o.Radius); iy <= obstacleCell(y+o.Radius); iy++ {
			for iz := obstacleCell(z - o.Radius); iz <= obstacleCell(z+o.Radius); iz++ {
				key := [3]int{ix, iy, iz}
				(*target)[key] = append((*target)[key], obstacleBox{center, o.Radius})
			}
		}
	}
}

// conflicts 职责：查询候选与存量的包围球冲突；使用：建筑和地基格；边界：高度参与计算。
func (idx obstacleIndex) conflicts(p geometry.WorldPoint, radius float64) bool {
	return overlapsObstacle(idx.volume, p, radius) || idx.surfaceConflict(p, radius)
}

// stationConflict 职责：按原版单精度距离拒绝过近物流塔；使用：塔候选在一般占地之外单独核对；边界：任一星际塔为29米，普通塔为15米，等于阈值允许且普通设备不参与。
func (idx obstacleIndex) stationConflict(p geometry.WorldPoint, item int16) bool {
	if item != 2103 && item != 2104 || len(idx.stations) == 0 {
		return false
	}
	x, y, z := float64(p.X), float64(p.Y), float64(p.Z)
	for ix := obstacleCell(x - 29); ix <= obstacleCell(x+29); ix++ {
		for iy := obstacleCell(y - 29); iy <= obstacleCell(y+29); iy++ {
			for iz := obstacleCell(z - 29); iz <= obstacleCell(z+29); iz++ {
				for _, other := range idx.stations[[3]int{ix, iy, iz}] {
					limit := float32(225)
					if item == 2104 || other.item == 2104 {
						limit = 841
					}
					dx, dy, dz := float32(p.X)-float32(other.center.X), float32(p.Y)-float32(other.center.Y), float32(p.Z)-float32(other.center.Z)
					if dx*dx+dy*dy+dz*dz < limit {
						return true
					}
				}
			}
		}
	}
	return false
}

// surfaceConflict 职责：检查绝对地表占地；使用：虚拟基地区域避让；边界：设备本身的高度不能越过地表限制。
func (idx obstacleIndex) surfaceConflict(p geometry.WorldPoint, radius float64) bool {
	return len(idx.surface) != 0 && overlapsObstacle(idx.surface, surfacePoint(p), radius)
}

// surfaceSpanConflict 职责：检查分拣跨距与带连接的中间占地；使用：最终地表区校验；边界：端点单独检查，中间距以半格余量覆盖。
func (idx obstacleIndex) surfaceSpanConflict(a, b Position, radius float64) bool {
	if len(idx.surface) == 0 || a == b {
		return false
	}
	first, err := world(a)
	if err != nil {
		return false
	}
	second, err := world(b)
	if err != nil {
		return false
	}
	first, second = surfacePoint(first), surfacePoint(second)
	dx, dy, dz := second.X-first.X, second.Y-first.Y, second.Z-first.Z
	length := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
	steps := min(16, int(math.Ceil(length)))
	for i := 1; i < steps; i++ {
		ratio := geometry.WorldLength(float64(i) / float64(steps))
		p := geometry.WorldPoint{Frame: first.Frame, X: first.X + dx*ratio, Y: first.Y + dy*ratio, Z: first.Z + dz*ratio}
		if idx.surfaceConflict(p, radius+length/float64(steps)/2) {
			return true
		}
	}
	return false
}

// surfaceRouteConflict 职责：找到中间跨进基地的分拣器或带路；使用：候选与最终几何共用；边界：没有区域或模块外带口时忽略。
func (idx obstacleIndex) surfaceRouteConflict(objects []Object, catalog *placement.Catalog) int {
	if len(idx.surface) == 0 {
		return -1
	}
	belts := make(map[int32]Position)
	for _, obj := range objects {
		if b := obj.Building; b.ItemID >= 2001 && b.ItemID <= 2003 {
			belts[b.Index] = obj.First
		}
	}
	for i, obj := range objects {
		b := obj.Building
		radius := groundRadius(catalog, b)
		if idx.surfaceSpanConflict(obj.First, obj.Second, radius) {
			return i
		}
		if b.ItemID >= 2001 && b.ItemID <= 2003 && b.OutputObjIdx >= 0 {
			if target, ok := belts[b.OutputObjIdx]; ok && idx.surfaceSpanConflict(obj.First, target, radius) {
				return i
			}
		}
	}
	return -1
}

// surfacePoint 职责：保留方向并归一至已验证的原版地表半径；使用：球面占地比较；边界：仅接受由原版球面映射产生的非零世界点。
func surfacePoint(p geometry.WorldPoint) geometry.WorldPoint {
	x, y, z := float64(p.X), float64(p.Y), float64(p.Z)
	scale := geometry.WorldLength((200 + float64(geometry.BaseLift)) / math.Sqrt(x*x+y*y+z*z))
	p.X, p.Y, p.Z = p.X*scale, p.Y*scale, p.Z*scale
	return p
}

// overlapsObstacle 职责：执行单一三维体素集合的相交查询；使用：普通占用及已投影地表区；边界：使用游戏米。
func overlapsObstacle(boxes map[[3]int][]obstacleBox, p geometry.WorldPoint, radius float64) bool {
	if len(boxes) == 0 {
		return false
	}
	x, y, z := float64(p.X), float64(p.Y), float64(p.Z)
	for ix := obstacleCell(x - radius); ix <= obstacleCell(x+radius); ix++ {
		for iy := obstacleCell(y - radius); iy <= obstacleCell(y+radius); iy++ {
			for iz := obstacleCell(z - radius); iz <= obstacleCell(z+radius); iz++ {
				for _, other := range boxes[[3]int{ix, iy, iz}] {
					dx, dy, dz := float64(p.X-other.center.X), float64(p.Y-other.center.Y), float64(p.Z-other.center.Z)
					r := radius + other.radius
					if dx*dx+dy*dy+dz*dz < r*r {
						return true
					}
				}
			}
		}
	}
	return false
}

// recordRadius 职责：获取原型建造盒的保守球；使用：存量避让；边界：动态建筑最少预留一格，未知原型由最终几何复核。
func recordRadius(catalog *placement.Catalog, building blueprint.Building) float64 {
	r := .65
	if shapes, err := catalog.Shapes(building.ItemID, building.ModelIndex); err == nil {
		for _, shape := range shapes {
			magnitude := func(v [3]float32) float64 {
				return math.Sqrt(float64(v[0])*float64(v[0]) + float64(v[1])*float64(v[1]) + float64(v[2])*float64(v[2]))
			}
			r = max(r, magnitude(shape.Position)+magnitude(shape.Extents))
		}
	}
	return r
}

// groundRadius 职责：计算原型地表水平投影包围；使用：地基和不占物流塔空间的地面电杆；边界：不把设备高度当横向半径。
func groundRadius(catalog *placement.Catalog, building blueprint.Building) float64 {
	radius := .45
	if boxes, err := catalog.Shapes(building.ItemID, building.ModelIndex); err == nil {
		for _, box := range boxes {
			radius = max(radius, math.Hypot(float64(box.Position[0]), float64(box.Position[2]))+math.Hypot(float64(box.Extents[0]), float64(box.Extents[2])))
		}
	}
	return radius
}

// placementIndices 保存分别适用存量建筑规则和矿点规则的只读空间索引。
type placementIndices struct {
	obstacles, minerals obstacleIndex
}

// indexPlacement 为不可变的障碍、矿点各建立一次索引。存量半径和端点在建索引前检查。
func indexPlacement(req Request) (*placementIndices, error) {
	for _, obstacle := range req.Obstacles {
		if !validPosition(obstacle.Position) || !(obstacle.Radius > 0 && obstacle.Radius <= 100) {
			return nil, &Error{PlanetID: req.PlanetID, Code: "PLANET_OBSTACLE", Message: "存量建筑位置或占用半径无效"}
		}
	}
	return &placementIndices{newObstacleIndex(req.Obstacles), newObstacleIndex(req.minerals)}, nil
}

// validatePlacement 检查实际双端点、地面和已知障碍，保留物品与模块定位。
func validatePlacement(ctx context.Context, req Request, records []blueprint.Building) error {
	if len(req.Obstacles) == 0 && len(req.minerals) == 0 && req.wet == nil {
		return nil
	}
	indices := req.placementIndices
	if indices == nil {
		var err error
		indices, err = indexPlacement(req)
		if err != nil {
			return err
		}
	}
	index, minerals := indices.obstacles, indices.minerals
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	relayError := func(obj Object) error {
		return &Error{PlanetID: req.PlanetID, Item: obj.Item, Module: obj.Module, Code: "PLANET_RELAY_OCCUPIED", Message: fmt.Sprintf("建筑 %d 的占地触及已知黑雾基地区域", obj.Building.Index)}
	}
	if i := index.surfaceRouteConflict(req.Objects, catalog); i >= 0 {
		return relayError(req.Objects[i])
	}
	for i, obj := range req.Objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		ground := groundRadius(catalog, records[i])
		if obj.Building.ItemID == 2213 && (req.geothermal == nil || !req.geothermal(obj.First)) {
			return &Error{PlanetID: req.PlanetID, Item: 2213, Code: "PLANET_GEOTHERMAL_DATA", Message: "地热落点的熔岩类型或三点深度未通过核验"}
		}
		if obj.Building.ItemID == 2306 && req.pumpSite != nil && !req.pumpSite(obj.First, float64(obj.Building.Yaw)) {
			return &Error{PlanetID: req.PlanetID, Item: obj.Item, Module: obj.Module, Code: "PLANET_WATER", Message: "抽水站支脚或取水口的实测地形未满足施工条件"}
		}
		if req.terrainConflict([]Object{obj}, catalog) {
			return &Error{PlanetID: req.PlanetID, Item: obj.Item, Module: obj.Module, Code: "PLANET_WATER", Message: fmt.Sprintf("建筑 %d 的占地触及液面或未能确认的岸线", obj.Building.Index)}
		}
		for _, position := range []Position{obj.First, obj.Second} {
			p, err := world(position)
			if err != nil {
				return err
			}
			if index.surfaceConflict(p, ground) {
				return relayError(obj)
			}
			if overlapsObstacle(index.volume, p, ground) || index.stationConflict(p, records[i].ItemID) {
				return &Error{PlanetID: req.PlanetID, Item: obj.Item, Module: obj.Module, Code: "PLANET_OCCUPIED", Message: fmt.Sprintf("建筑 %d 与存量或预建造占用冲突", obj.Building.Index)}
			}
			item := records[i].ItemID
			if item != 2301 && item != 2316 && item != 2307 && minerals.conflicts(p, ground) {
				return &Error{PlanetID: req.PlanetID, Item: obj.Item, Module: obj.Module, Code: "PLANET_VEIN_OCCUPIED", Message: fmt.Sprintf("建筑 %d 占用了需保留的矿点", obj.Building.Index)}
			}
		}
	}
	return nil
}

// terrainConflict 职责：检查端点及四向占地边缘；使用：候选选址和终检；边界：未知地形仍由结果告警。
func (req *Request) terrainConflict(objects []Object, catalog *placement.Catalog) bool {
	wet := req.wet
	if wet == nil {
		return false
	}
	for _, obj := range objects {
		if obj.Building.ItemID == 2104 && req.stationPad != nil {
			pads := [][2]float64{{4, -2}, {-4, -2}, {-4, 2}, {4, 2}, {2, -4}, {-2, -4}, {-2, 4}, {2, 4}}
			if !surfacePoints(obj.First, float64(obj.Building.Yaw), 0, pads, req.stationPad) {
				return true
			}
		}
		if obj.Building.ItemID == 2213 {
			for _, p := range geothermalPoints(obj.First) {
				if water, known := wet(p); !known || !water {
					return true
				}
			}
			continue
		}
		if obj.Building.ItemID == 2306 {
			// 抽水站本体必须跨岸，由原版专用陆点和进液点规则复核。
			if !pumpFits(obj.First, float64(obj.Building.Yaw), wet) {
				return true
			}
			continue
		}
		radius := groundRadius(catalog, obj.Building) / 200
		for _, point := range []Position{obj.First, obj.Second} {
			if point.Height > .5 {
				continue
			}
			dlon := radius / max(.01, math.Cos(point.Latitude))
			for _, delta := range [][2]float64{{0, 0}, {radius, 0}, {-radius, 0}, {0, dlon}, {0, -dlon}} {
				p := Position{Latitude: point.Latitude + delta[0], Longitude: point.Longitude + delta[1]}
				if water, known := wet(p); !known || water {
					return true
				}
			}
			if point == obj.Second {
				break
			}
		}
	}
	return false
}
