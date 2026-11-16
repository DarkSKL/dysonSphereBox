package build

// 文件职责：按设施原型与真实球面点索引单元间距；使用：原生单元装箱；边界：只缩小邻近集合，完整保留原塔距和电力节点三维阈值。
import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// polarStationGroup 职责：关联同原型设施点与共用空间索引；使用：增量落位；边界：点和索引下标必须一致。
type polarStationGroup struct {
	points []placement.Vector
	index  polarSpatialIndex
}

// polarStationIndex 职责：缓存原型间距与已放置设施的空间桶；使用：单次候选私有装箱；边界：不跨任务持有实体或修改物理数据。
type polarStationIndex struct {
	items  []int16
	groups map[int16]*polarStationGroup
	limits map[[2]int16]float64
}

// polarStationDistanceSquared 职责：统一原有两类设施间距；使用：宽相位及独立穷举对照；边界：不同设施类别没有本项互斥，实体碰撞仍独立检查。
func polarStationDistanceSquared(a, b int16) float64 {
	if surfaceIsStation(a) && surfaceIsStation(b) {
		return float64(surfaceStationDistanceLimit(a, b))
	}
	if surfaceIsPowerNode(a) && surfaceIsPowerNode(b) {
		d := math.Sqrt(float64(surfacePowerDistanceLimit(a, b))) + 1e-4
		return d * d
	}
	return 0
}

// add 职责：只登记已确定的真实中心；使用：单元commit；边界：试探位置不得登记。
func (index *polarStationIndex) add(item int16, point placement.Vector) {
	if index.groups == nil {
		index.groups = map[int16]*polarStationGroup{}
		index.limits = map[[2]int16]float64{}
	}
	group := index.groups[item]
	if group == nil {
		group = &polarStationGroup{}
		index.groups[item] = group
		index.items = append(index.items, item)
	}
	group.points = append(group.points, point)
	group.index.add(point, point)
}

// conflicts 职责：仅查询阈值包围范围内的同类设施；使用：每次真实经向位置试探；边界：窄相位仍用原距离平方严格小于判断。
func (index *polarStationIndex) conflicts(item int16, point placement.Vector) bool {
	_, found := index.conflictMargin(item, point)
	return found
}

// conflictMargin 职责：返回一个真实间距冲突的内余量；使用：保守经向步进；边界：余量不含外扩桶误差，不跨越可能有效的坐标。
func (index *polarStationIndex) conflictMargin(item int16, point placement.Vector) (float64, bool) {
	for _, other := range index.items {
		key := [2]int16{item, other}
		limit, known := index.limits[key]
		if !known {
			limit = polarStationDistanceSquared(item, other)
			index.limits[key] = limit
		}
		if limit <= 0 {
			continue
		}
		radius := math.Sqrt(limit)
		margin := placement.Vector{radius, radius, radius}
		group := index.groups[other]
		depth := 0.
		if group.index.any(point.Sub(margin), point.Add(margin), func(i int) bool {
			d := point.Sub(group.points[i])
			distance := d.Dot(d)
			if distance >= limit {
				return false
			}
			depth = radius - math.Sqrt(distance)
			return true
		}) {
			return depth, true
		}
	}
	return 0, false
}
