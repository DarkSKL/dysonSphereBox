package build

// 文件职责：为坐标下降复用每个单元的碰撞索引，避免每次移动重建全厂哈希桶。
// 使用：纯平移压缩；边界：仍执行相同窄相位规则，只重建已经移动的单元。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
)

// denseCompactIndexEntry 职责：保存单元当前轮廓及其空间桶包络；使用：排除不共享桶的单元；边界：按桶范围而非物理盒重叠筛选，保留电力等距离规则。
type denseCompactIndexEntry struct {
	packing   *packingIndex
	low, high placement.Vector
	present   bool
}

// denseCompactIndex 职责：合并固定设施与可移动单元的独立只读索引；使用：单元坐标下降；边界：第零组固定，检查时仅排除正在移动的组。
type denseCompactIndex struct {
	dense   bool
	entries []denseCompactIndexEntry
}

// replace 职责：更新一个单元的轮廓和桶；使用：初始化或有效移动后；边界：其他单元索引保持不变。
func (index *denseCompactIndex) replace(group int, contour packingContour) {
	entry := denseCompactIndexEntry{packing: &packingIndex{dense: index.dense, cells: map[[3]int][]int{}}, low: placement.Vector{math.Inf(1), math.Inf(1), math.Inf(1)}, high: placement.Vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}}
	for _, shape := range contour.shapes {
		entry.present = true
		for axis := 0; axis < 3; axis++ {
			entry.low[axis] = math.Min(entry.low[axis], shape.low[axis])
			entry.high[axis] = math.Max(entry.high[axis], shape.high[axis])
		}
	}
	entry.packing.register(contour, 0, 0)
	index.entries[group] = entry
}

// fits 职责：在其他组的原有空间桶内检查同一候选；使用：替代逐单元重建全厂索引；边界：只省略必不共享桶的组，不省略任何可能冲突形状。
func (index *denseCompactIndex) fits(ctx context.Context, group int, contour packingContour, dx, dy float64) bool {
	if ctx.Err() != nil {
		return false
	}
	own := index.entries[group]
	delta := placement.Vector{dx * planarGridLength, 0, dy * planarGridLength}
	low, high := own.low.Add(delta), own.high.Add(delta)
	for i, other := range index.entries {
		if i == group || !other.present {
			continue
		}
		separated := false
		for axis := 0; axis < 3; axis++ {
			// 两组gridPort各可能发生半格舍入；桶包络容纳两侧修正，不能把真实端口柱筛掉。
			margin := planarGridLength
			if math.Floor((high[axis]+margin)/8) < math.Floor(other.low[axis]/8) || math.Floor((low[axis]-margin)/8) > math.Floor(other.high[axis]/8) {
				separated = true
				break
			}
		}
		if !separated && !other.packing.fits(ctx, contour, dx, dy) {
			return false
		}
	}
	return true
}
