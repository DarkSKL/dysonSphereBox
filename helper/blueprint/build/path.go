package build

// 文件职责：提供有上限且保序的四邻 A*；使用：物流站连接；边界：不允许跨层，失败返回空路径让上层报错。
// 算法规则对照 dsp-calc BetterBeltUtil.ts，独立实现，见 ../NOTICE.md。

import (
	"container/heap"
	"context"
	"math"
)

// pathNode 职责：保存搜索状态；使用：A*；边界：上限 5000 次展开。
type pathNode struct {
	// point 职责：格点；使用：回溯；边界：高度不变。
	point vector
	// parent 职责：前驱；使用：路径重建；边界：不形成环。
	parent *pathNode
	// distance 职责：起点距离；使用：比较路径；边界：非负。
	distance float64
	// score 职责：曼哈顿估价；使用：选最小项；边界：并列保留插入顺序。
	score float64
	// order 职责：保持同分时原插入顺序；使用：堆比较；边界：请求内单调递增。
	order int
	// position 职责：支持降低距离后的堆更新；使用：heap.Fix；边界：出队后为 -1。
	position int
}

// pathQueue 职责：按估价及插入顺序稳定选择最短路径节点；使用：A*；边界：不改变四邻/5000 次展开规则。
type pathQueue []*pathNode

// Len 职责：返回待搜索节点数；使用：container/heap；边界：不遍历已关闭节点。
func (q pathQueue) Len() int { return len(q) }

// Less 职责：保留参考同分插入序；使用：堆排序；边界：不靠 map 遍历选择路径。
func (q pathQueue) Less(i, j int) bool {
	if q[i].score != q[j].score {
		return q[i].score < q[j].score
	}
	return q[i].order < q[j].order
}

// Swap 职责：交换节点并同步位置；使用：堆维护；边界：不修改距离。
func (q pathQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i]; q[i].position = i; q[j].position = j }

// Push 职责：登记新候选；使用：堆维护；边界：只接受路径节点。
func (q *pathQueue) Push(value interface{}) {
	node := value.(*pathNode)
	node.position = len(*q)
	*q = append(*q, node)
}

// Pop 职责：取出已选节点；使用：堆维护；边界：堆调用前已交换至末尾。
func (q *pathQueue) Pop() interface{} {
	old := *q
	n := old[len(old)-1]
	old[len(old)-1] = nil
	*q = old[:len(old)-1]
	n.position = -1
	return n
}

// pathKey 职责：将固定高度平面的坐标编码为整数哈希键；使用：搜索去重；边界：不量化小数，并将正负零统一为原有浮点相等语义。
func pathKey(point vector) [2]uint64 {
	x, y := point.x, point.y
	if x == 0 {
		x = 0
	}
	if y == 0 {
		y = 0
	}
	return [2]uint64{math.Float64bits(x), math.Float64bits(y)}
}

// findPath 职责：在禁行格中找带路；使用：站点装配；边界：上下文截止后立即退出，拒绝非有限坐标。
func findPath(ctx context.Context, start, end vector, firstX bool, forbidden map[vector]bool) []vector {
	if start.z != end.z {
		return nil
	}
	for _, value := range []float64{start.x, start.y, start.z, end.x, end.y} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}
	}
	first := &pathNode{point: start, score: math.Abs(start.x-end.x) + math.Abs(start.y-end.y)}
	queue := pathQueue{first}
	serial := 0
	// 固定高度不参与去重；整数位键避免每个邻点反复哈希三个 float64，保持小数端点语义。
	// 已出队节点仍登记于 seen，nil 值缓存禁行格，保持原有展开和同分优先级。
	seen := map[[2]uint64]*pathNode{pathKey(start): first}
	for steps := 0; len(queue) > 0 && steps < 5000; steps++ {
		if ctx.Err() != nil {
			return nil
		}
		current := heap.Pop(&queue).(*pathNode)
		if current.point == end {
			out := []vector{}
			for n := current; n != nil; n = n.parent {
				out = append(out, n.point)
			}
			for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
				out[i], out[j] = out[j], out[i]
			}
			return out
		}
		delta := []vector{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}}
		if !firstX {
			delta = []vector{{0, 1, 0}, {0, -1, 0}, {1, 0, 0}, {-1, 0, 0}}
		}
		for _, step := range delta {
			v := vector{current.point.x + step.x, current.point.y + step.y, current.point.z}
			cell := pathKey(v)
			old, known := seen[cell]
			if known && (old == nil || old.position < 0) {
				continue
			}
			if !known && forbidden[v] && v != end {
				seen[cell] = nil
				continue
			}
			dist := current.distance + 1
			if known {
				if dist < old.distance {
					old.distance = dist
					old.score = dist + math.Abs(v.x-end.x) + math.Abs(v.y-end.y)
					old.parent = current
					heap.Fix(&queue, old.position)
				}
			} else {
				serial++
				n := &pathNode{point: v, parent: current, distance: dist, score: dist + math.Abs(v.x-end.x) + math.Abs(v.y-end.y), order: serial}
				seen[cell] = n
				heap.Push(&queue, n)
			}
		}
	}
	return nil
}
