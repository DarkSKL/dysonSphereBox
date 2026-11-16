package build

// 文件职责：提供多模式共同的增量轮廓索引、可放置查询和局部位置搜索。
// 使用：局部单元装配后调用；边界：只登记已接受实体，每候选拥有独立索引及查询代次。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"math"
)

// addShape 职责：统一追加已确定形状及其空间桶；使用：各模式登记局部轮廓；边界：不隐式平移或对齐端口，保持原始插入顺序。
func (packing *packingIndex) addShape(shape packingShape) {
	if packing.cells == nil {
		packing.cells = make(map[[3]int][]int)
	}
	index := len(packing.shapes)
	packing.shapes = append(packing.shapes, shape)
	packingShapeCells(shape, func(cell [3]int) bool {
		packing.cells[cell] = append(packing.cells[cell], index)
		return true
	})
}

// register 职责：按选定位姿提交完整轮廓；使用：直连、塔场、折行及压缩索引；边界：与查询使用同一平移和整数端口对齐，候选失败不调用。
func (packing *packingIndex) register(contour packingContour, dx, dy float64) {
	for _, shape := range contour.shapes {
		packing.addShape(shiftPackingShape(shape, dx, dy))
	}
}

// packingIndex 职责：维护已放置产线的真实几何空间哈希；使用：有界轮廓靠拢；边界：最终物流路由和球面规则仍由统一出口验证。
type packingIndex struct {
	// dense 职责：使用同次密铺带间规则及节点净距；使用：全场密铺索引；边界：普通轮廓检查不改变。
	dense  bool
	shapes []packingShape
	cells  map[[3]int][]int
	high   vector
	ready  bool
	// query 职责：复用所有宽相位共用的查询代次规则；使用：本候选串行查询；边界：不跨候选或协程共享。
	query collisionQueryMarks
	// kindCells 职责：按碰撞规则类别索引密铺形状，避免查询必定互略的类型；使用：增量只读候选筛选；边界：未知类别保留保守检查。
	kindCells map[string]map[[3]int][]int
	// kindCount 职责：记录已进入分类索引的形状数；使用：新增实体后补齐；边界：shapes仅追加。
	kindCount int
	// infillAnchors 职责：保存已落位块的有限边缘候选；使用：水平行架回填旧空槽；边界：只尝试不扩展既有包络的全局落点。
	infillAnchors []vector
}

// fits 职责：查询一条轮廓是否可平移到指定位置；使用：有界候选选择；边界：不把失败候选写入占位索引。
func (packing *packingIndex) fits(ctx context.Context, contour packingContour, dx, dy float64) bool {
	if packing.dense {
		packing.indexDenseKinds()
	}
	for _, source := range contour.shapes {
		if ctx.Err() != nil {
			return false
		}
		shape := shiftPackingShape(source, dx, dy)
		packing.query.begin(len(packing.shapes))
		buckets, bucketCount := [6]map[[3]int][]int{packing.cells}, 1
		if packing.dense {
			buckets, bucketCount = packing.denseQueryBuckets(shape.kind)
		}
		if !packingShapeCells(shape, func(cell [3]int) bool {
			for bucketIndex := 0; bucketIndex < bucketCount; bucketIndex++ {
				bucket := buckets[bucketIndex]
				for _, index := range bucket[cell] {
					if !packing.query.seen(index) {
						var conflict bool
						if packing.dense {
							conflict = denseSpatialConflict(&shape, &packing.shapes[index])
						} else {
							conflict = packingShapesConflict(&shape, &packing.shapes[index])
						}
						if conflict {
							return false
						}
					}
				}
			}
			return true
		}) {
			return false
		}
	}
	return true
}

// place 职责：以固定位置预算在矩形基线周边寻找可利用凹槽；使用：每条机器行增量靠拢；边界：只提交真实几何不冲突的最小包络候选，完整路由失败仍由生成候选回退。
func (packing *packingIndex) place(ctx context.Context, contour packingContour, low, high vector, shelfX, shelfY, heightLimit float64) (float64, float64, error) {
	width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
	// position 职责：保存整数格候选原点；使用：固定预算和稳定去重；边界：只在本次机器行选择中有效。
	type position struct{ x, y float64 }
	candidates := []position{{shelfX, shelfY}}
	seen := map[position]bool{candidates[0]: true}
	add := func(x, y float64) {
		point := position{math.Floor(x), math.Floor(y)}
		if point.x < shelfX || point.y < 0 || point.y+height > math.Max(heightLimit, height) || seen[point] || len(candidates) >= 128 {
			return
		}
		seen[point] = true
		candidates = append(candidates, point)
	}
	// 当前列内先向下靠拢，再试少量横向错位，以利用过滤区、电杆和
	// 长短机器行形成的凹槽；每个位置用真实轮廓验证，而不是减小 bbox。
	for retreat := 0.; retreat <= 18 && len(candidates) < 128; retreat++ {
		for _, offset := range []float64{0, 1, 2, 3, 4, 6, 8} {
			add(shelfX+offset, shelfY-retreat)
		}
	}
	best, bestScore := position{}, math.Inf(1)
	found := false
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return 0, 0, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "机器行凹槽靠拢已取消或超时"}
		}
		if !packing.fits(ctx, contour, candidate.x-low.x, candidate.y-low.y) {
			continue
		}
		maximumX, maximumY := candidate.x+width, candidate.y+height
		if packing.ready {
			maximumX = math.Max(maximumX, packing.high.x)
			maximumY = math.Max(maximumY, packing.high.y)
		}
		score := maximumX * maximumY
		if !found || score < bestScore || score == bestScore && (candidate.y < best.y || candidate.y == best.y && candidate.x < best.x) {
			best, bestScore, found = candidate, score, true
		}
	}
	if !found {
		return 0, 0, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器行在固定轮廓靠拢预算内没有满足静态、带路、端口与供电净空的位置"}
	}
	dx, dy := best.x-low.x, best.y-low.y
	packing.register(contour, dx, dy)
	packing.high.x = math.Max(packing.high.x, best.x+width)
	packing.high.y = math.Max(packing.high.y, best.y+height)
	packing.ready = true
	return best.x, best.y, nil
}
