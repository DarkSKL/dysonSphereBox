package build

// 文件职责：在忽略三维障碍的平面图上计算走廊代价下界，避免软代价导致三维搜索泛洪。
// 使用：同一路线的相邻端口候选共用势场；边界：不产生可用路线，不绕过三维占位，预计算格数也受固定预算限制。

import (
	"context"
	"math"
)

// transportCorridorPotential 职责：保存一个占位版本的平面最短距离势场；使用：A* 一致下界；边界：忽略建筑和高度，不作实体连通证据。
type transportCorridorPotential struct {
	version, minX, minY, width, height int
	root                               [2]int
	distances                          []int
}

// prepareTransportHeuristic 职责：以有界整数权重最短路强化走廊下界；使用：每路首个端口候选，邻格复用同一势场；边界：过大平面回退曼哈顿，不增加三维展开上限，取消不发布部分势场。
func (env *transportEnvironment) prepareTransportHeuristic(ctx context.Context, target vector) {
	if !env.preferCorridor || ctx.Err() != nil {
		return
	}
	root := beltColumn(target)
	if target.x != float64(root[0]) || target.y != float64(root[1]) {
		env.corridorPotential = nil
		return
	}
	if old := env.corridorPotential; old != nil && old.version == env.version && math.Abs(float64(old.root[0]-root[0]))+math.Abs(float64(old.root[1]-root[1])) <= 4 {
		return
	}
	env.corridorPotential = nil
	minX, minY := int(math.Floor(env.minimum.x)), int(math.Floor(env.minimum.y))
	width, height := int(math.Ceil(env.maximum.x))-minX+1, int(math.Ceil(env.maximum.y))-minY+1
	if width <= 0 || height <= 0 || width > transportSearchLimit/height || root[0] < minX || root[0] >= minX+width || root[1] < minY || root[1] >= minY+height {
		return
	}
	field := &transportCorridorPotential{version: env.version, minX: minX, minY: minY, width: width, height: height, root: root, distances: make([]int, width*height)}
	corridors := make([]bool, width*height)
	for column, occupied := range env.beltCorridors {
		x, y := column[0]-minX, column[1]-minY
		if occupied && x >= 0 && x < width && y >= 0 && y < height {
			corridors[y*width+x] = true
		}
	}
	for i := range field.distances {
		field.distances[i] = math.MaxInt
	}
	start := (root[1]-minY)*width + root[0] - minX
	field.distances[start] = 0
	// 权重只可能为四分之一格的 4/5，使用六个循环桶，避免为每格分配堆节点。
	var buckets [6][]int
	buckets[0] = append(buckets[0], start)
	pending, distance, expanded := 1, 0, 0
	for pending > 0 {
		if expanded%4096 == 0 && ctx.Err() != nil {
			return
		}
		bucket := distance % len(buckets)
		if len(buckets[bucket]) == 0 {
			distance++
			continue
		}
		last := len(buckets[bucket]) - 1
		index := buckets[bucket][last]
		buckets[bucket] = buckets[bucket][:last]
		pending--
		if field.distances[index] != distance {
			continue
		}
		expanded++
		x, y := index%width, index/width
		for _, delta := range [][2]int{{-1, 0}, {0, 1}, {1, 0}, {0, -1}} {
			nx, ny := x+delta[0], y+delta[1]
			if nx < 0 || nx >= width || ny < 0 || ny >= height {
				continue
			}
			next := ny*width + nx
			cost := 5
			if corridors[index] && corridors[next] {
				cost = 4
			}
			candidate := distance + cost
			if candidate < field.distances[next] {
				field.distances[next] = candidate
				buckets[candidate%len(buckets)] = append(buckets[candidate%len(buckets)], next)
				pending++
			}
		}
	}
	if ctx.Err() == nil {
		env.corridorPotential = field
	}
}

// distanceAt 职责：读取平面最短路势能；使用：相邻目标共享的三角不等式下界；边界：场外坐标必须回退原下界。
func (field *transportCorridorPotential) distanceAt(point vector) (float64, bool) {
	column := beltColumn(point)
	if point.x != float64(column[0]) || point.y != float64(column[1]) {
		return 0, false
	}
	x, y := column[0]-field.minX, column[1]-field.minY
	if x < 0 || x >= field.width || y < 0 || y >= field.height {
		return 0, false
	}
	return float64(field.distances[y*field.width+x]) / 4, true
}
