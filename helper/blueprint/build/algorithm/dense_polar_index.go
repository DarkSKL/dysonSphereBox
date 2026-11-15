package build

// 文件职责：按世界坐标缩小极区装箱的碰撞候选集合。
// 使用：每次单元装箱独立创建；边界：宽相位只增选邻近对象，实际冲突仍由原有盒、距离与塔距规则决定。

import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// polarSpatialCellSize 职责：定义世界坐标空间桶边长；使用：装箱宽相位；边界：不是碰撞距离或建筑间距。
const polarSpatialCellSize = 8.

// polarSpatialIndex 职责：保存实际几何包围盒覆盖的桶及查询标记；使用：单个候选内增量登记；边界：只追加，不在候选或协程之间共享。
type polarSpatialIndex struct {
	cells    map[[3]int][]int
	marks    []uint32
	sequence uint32
	// bounds 职责：保存已登记对象的世界包围盒；使用：桶命中后先排除不相交对象；边界：与 marks 同序，只用于宽相位，不能代替精确碰撞。
	bounds [][2]placement.Vector
}

// polarSpatialCells 职责：遍历包围盒覆盖的全部桶；使用：登记和查询；边界：边界桶也包含在内，visit返回false时提前停止。
func polarSpatialCells(low, high placement.Vector, visit func([3]int) bool) bool {
	for x := int(math.Floor(low[0] / polarSpatialCellSize)); x <= int(math.Floor(high[0]/polarSpatialCellSize)); x++ {
		for y := int(math.Floor(low[1] / polarSpatialCellSize)); y <= int(math.Floor(high[1]/polarSpatialCellSize)); y++ {
			for z := int(math.Floor(low[2] / polarSpatialCellSize)); z <= int(math.Floor(high[2]/polarSpatialCellSize)); z++ {
				if !visit([3]int{x, y, z}) {
					return false
				}
			}
		}
	}
	return true
}

// add 职责：登记一个新对象并保留原集合下标；使用：单元确认落位之后；边界：登记顺序须与对应几何数组一致。
func (index *polarSpatialIndex) add(low, high placement.Vector) {
	if index.cells == nil {
		index.cells = map[[3]int][]int{}
	}
	id := len(index.marks)
	index.marks = append(index.marks, 0)
	index.bounds = append(index.bounds, [2]placement.Vector{low, high})
	polarSpatialCells(low, high, func(cell [3]int) bool { index.cells[cell] = append(index.cells[cell], id); return true })
}

// any 职责：仅对宽相位命中的唯一对象执行原精确判定；使用：盒、节点和臂查询；边界：毫米外扩只增加候选，不放宽精确距离阈值。
func (index *polarSpatialIndex) any(low, high placement.Vector, conflict func(int) bool) bool {
	if len(index.marks) == 0 {
		return false
	}
	index.sequence++
	if index.sequence == 0 {
		clear(index.marks)
		index.sequence = 1
	}
	margin := placement.Vector{.001, .001, .001}
	low, high = low.Sub(margin), high.Add(margin)
	found := false
	polarSpatialCells(low, high, func(cell [3]int) bool {
		for _, id := range index.cells[cell] {
			if index.marks[id] == index.sequence {
				continue
			}
			index.marks[id] = index.sequence
			bounds := &index.bounds[id]
			if high[0] < bounds[0][0] || low[0] > bounds[1][0] || high[1] < bounds[0][1] || low[1] > bounds[1][1] || high[2] < bounds[0][2] || low[2] > bounds[1][2] {
				continue
			}
			if conflict(id) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// polarSpanBounds 职责：求线段及指定距离邻域的世界包围盒；使用：分拣器宽相位；边界：距离本身仍用既有线段最短距离计算。
func polarSpanBounds(span surfaceBeltSpan, radius float64) (placement.Vector, placement.Vector) {
	var low, high placement.Vector
	for axis := range low {
		low[axis] = math.Min(span.ends[0][axis], span.ends[1][axis]) - radius
		high[axis] = math.Max(span.ends[0][axis], span.ends[1][axis]) + radius
	}
	return low, high
}

// polarPointBoxDistance 职责：计算点到正交盒的距离平方；使用：替代两端相同的线段查询；边界：与原线段距离的退化情形一致，不改变盒尺寸。
func polarPointBoxDistance(box placement.Box, point placement.Vector) float64 {
	delta := point.Sub(box.Center)
	distance := 0.
	for axis := 0; axis < 3; axis++ {
		gap := math.Max(0, math.Abs(delta.Dot(box.Axes[axis]))-box.HalfSize[axis])
		distance += gap * gap
	}
	return distance
}
