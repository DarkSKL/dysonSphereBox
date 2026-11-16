package build

// 文件职责：复用世界坐标宽相位的桶范围、登记和稳定查询遍历。
// 使用：极区装箱、带占用、塔距和供电规则各自持有实例；边界：桶边长由调用方指定为世界单位，不接受网格坐标，不缓存精确碰撞结果。

import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// worldSpatialRange 职责：保存闭合整数桶范围；使用：将坐标换算一次后供登记或查询；边界：不是世界包围盒，遍历顺序固定为 x、y、z。
type worldSpatialRange struct {
	low, high [3]int
}

// worldSpatialBounds 职责：把世界包围盒映射到闭合桶范围；使用：带段和静态盒登记；边界：cellSize 必须为有限正数，负坐标向下取整，阈值外扩由调用方完成。
func worldSpatialBounds(low, high placement.Vector, cellSize float64) worldSpatialRange {
	var cells worldSpatialRange
	for axis := range low {
		cells.low[axis] = int(math.Floor(low[axis] / cellSize))
		cells.high[axis] = int(math.Floor(high[axis] / cellSize))
	}
	return cells
}

// worldSpatialNeighbors 职责：求点所在桶及相邻一圈桶；使用：保留塔距和供电原有 27 桶宽相位；边界：不按查询半径重算范围，不改变单精度窄相位。
func worldSpatialNeighbors(point vector, cellSize float64) worldSpatialRange {
	cell := worldSpatialPoint(point, cellSize).low
	return worldSpatialRange{low: [3]int{cell[0] - 1, cell[1] - 1, cell[2] - 1}, high: [3]int{cell[0] + 1, cell[1] + 1, cell[2] + 1}}
}

// worldSpatialPoint 职责：只换算一次单点桶坐标；使用：点索引登记和邻域查询共用；边界：不重复求相同上下界，不对负坐标截断。
func worldSpatialPoint(point vector, cellSize float64) worldSpatialRange {
	cell := [3]int{int(math.Floor(point.x / cellSize)), int(math.Floor(point.y / cellSize)), int(math.Floor(point.z / cellSize))}
	return worldSpatialRange{low: cell, high: cell}
}

// each 职责：按原宽相位顺序枚举桶；使用：登记和单独桶访问；边界：返回 false 立即终止，包含正好落在桶边界的上界。
func (cells worldSpatialRange) each(visit func([3]int) bool) bool {
	for x := cells.low[0]; x <= cells.high[0]; x++ {
		for y := cells.low[1]; y <= cells.high[1]; y++ {
			for z := cells.low[2]; z <= cells.high[2]; z++ {
				if !visit([3]int{x, y, z}) {
					return false
				}
			}
		}
	}
	return true
}

// worldSpatialIndex 职责：保存对象编号的桶及可复用查询去重缓存；使用：调用内增量追加，零值可用；边界：编号非负且同一对象仅登记一次，实例不能并发或重入查询，几何变更须重建。
type worldSpatialIndex struct {
	cells map[[3]int][]int
	query collisionQueryMarks
	// size 职责：记录最大编号加一；使用：查询标记扩容；边界：允许记录下标有空洞。
	size int
	// spanning 职责：标识是否有跨桶对象；使用：纯点索引跳过去重开销；边界：一旦跨桶登记就保持为 true。
	spanning bool
}

// add 职责：将对象编号追加到全部覆盖桶；使用：几何确认后；边界：不排序、不变换世界坐标、不保留几何副本。
func (index *worldSpatialIndex) add(cells worldSpatialRange, id int) {
	if index.cells == nil {
		index.cells = make(map[[3]int][]int)
	}
	index.size = max(index.size, id+1)
	index.spanning = index.spanning || cells.low != cells.high
	cells.each(func(cell [3]int) bool {
		index.cells[cell] = append(index.cells[cell], id)
		return true
	})
}

// addPoint 职责：只登记点所在桶；使用：塔距和供电增量建图；边界：桶大小由调用方传入世界单位，保留原记录编号。
func (index *worldSpatialIndex) addPoint(point vector, cellSize float64, id int) {
	if index.cells == nil {
		index.cells = make(map[[3]int][]int)
	}
	index.size = max(index.size, id+1)
	cell := worldSpatialPoint(point, cellSize).low
	index.cells[cell] = append(index.cells[cell], id)
}

// any 职责：按首次遇到的桶和插入顺序访问唯一编号；使用：调用方执行原窄相位，返回 true 提前终止；边界：不附加包围盒过滤，不缓存判定，跨桶对象复用现有序号标记。
func (index *worldSpatialIndex) any(cells worldSpatialRange, visit func(int) bool) bool {
	if index.size == 0 {
		return false
	}
	if index.spanning {
		index.query.begin(index.size)
	}
	// 热点查询直接读桶，避免为每个空桶调用闭包；顺序仍与each一致，仅对象命中时调用窄相位。
	for x := cells.low[0]; x <= cells.high[0]; x++ {
		for y := cells.low[1]; y <= cells.high[1]; y++ {
			for z := cells.low[2]; z <= cells.high[2]; z++ {
				for _, id := range index.cells[[3]int{x, y, z}] {
					if index.spanning && index.query.seen(id) {
						continue
					}
					if visit(id) {
						return true
					}
				}
			}
		}
	}
	return false
}
