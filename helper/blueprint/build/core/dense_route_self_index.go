package build

// 文件职责：用路径局部空间桶检查带链自身节点的真实净距。
// 使用：完整密铺带链提交前的独立复核；边界：只缩小相邻候选，不替代静态盒、端口或既有带节点检查。

import "DysonSphereBox/helper/blueprint/build/placement"

// denseRouteSelfIndex 职责：保存当前路径已检查的世界坐标；使用：避免长路线反复转换球面坐标及全量两两比较；边界：短路径直接扫描，索引只存活于一次完整检查。
type denseRouteSelfIndex struct {
	points []placement.Vector
	cells  map[[3]int][]int
}

// newDenseRouteSelfIndex 职责：按路径规模选择直接扫描或空间桶；使用：开始整链复核时；边界：两种策略采用相同净距阈值。
func newDenseRouteSelfIndex(size int) denseRouteSelfIndex {
	index := denseRouteSelfIndex{points: make([]placement.Vector, 0, size)}
	if size > 16 {
		index.cells = make(map[[3]int][]int)
	}
	return index
}

// addClear 职责：拒绝与已检查节点距离过近的新节点并登记无冲突位置；使用：按原路径顺序调用；边界：宽相位包含净距球覆盖的边界桶，精确距离及容差不变。
func (index *denseRouteSelfIndex) addClear(point placement.Vector) bool {
	near := func(other placement.Vector) bool {
		delta := point.Sub(other)
		return delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9
	}
	if index.cells == nil {
		for _, other := range index.points {
			if near(other) {
				return false
			}
		}
	} else {
		padding := placement.Vector{denseRouteNodeClearance, denseRouteNodeClearance, denseRouteNodeClearance}
		low, high := denseRouteNodeCell(point.Sub(padding)), denseRouteNodeCell(point.Add(padding))
		for x := low[0]; x <= high[0]; x++ {
			for y := low[1]; y <= high[1]; y++ {
				for z := low[2]; z <= high[2]; z++ {
					for _, id := range index.cells[[3]int{x, y, z}] {
						if near(index.points[id]) {
							return false
						}
					}
				}
			}
		}
		cell := denseRouteNodeCell(point)
		index.cells[cell] = append(index.cells[cell], len(index.points))
	}
	index.points = append(index.points, point)
	return true
}
