package build

// 文件职责：为一次运输搜索提供无碰撞整数键及通用坐标后备索引。
// 使用：多终点 A* 的已访问节点表；边界：不缓存占位或端口权限，不跨搜索复用动态状态。

import "math"

// transportSearchIndex 职责：保存当前搜索唯一的节点记录；使用：整数网格避免三浮点键的哈希和比较；边界：分数坐标或越界搜索整体回退，不能截断坐标合并节点。
type transportSearchIndex struct {
	integer map[uint64]*transportPathNode
	general map[vector]*transportPathNode
}

// newTransportSearchIndex 职责：在起点和整个搜索域均可精确编码时选择整数键；使用：搜索开始前；边界：后续节点只由起点加减整数单位生成，非有限或过大边界使用原浮点键。
func newTransportSearchIndex(start, low, high vector) transportSearchIndex {
	within := func(v float64) bool { return !math.IsNaN(v) && v > -(1<<20) && v < (1<<20)-1 }
	if denseIntegerNode(start) && within(start.x) && within(start.y) && start.z >= 0 && start.z <= transportMaxHeight && within(low.x) && within(high.x) && within(low.y) && within(high.y) {
		return transportSearchIndex{integer: make(map[uint64]*transportPathNode)}
	}
	return transportSearchIndex{general: make(map[vector]*transportPathNode)}
}

// transportSearchKey 职责：无重叠编码两条 21 位平面坐标及 6 位高度；使用：已验证范围的整数搜索；边界：邻格预查询允许高度 -1 和 33，但仅合法高度能写入。
func transportSearchKey(point vector) uint64 {
	return uint64(int64(point.x)+(1<<20))<<27 | uint64(int64(point.y)+(1<<20))<<6 | uint64(int64(point.z)+1)
}

// get 职责：读取已访问状态；使用：邻格关闭剪枝与松弛；边界：不存在返回 nil，不改变状态。
func (index *transportSearchIndex) get(point vector) *transportPathNode {
	if index.integer != nil {
		return index.integer[transportSearchKey(point)]
	}
	return index.general[point]
}

// set 职责：登记新发现的合法节点；使用：加入开放队列之前；边界：由调用方先核对边界和真实几何。
func (index *transportSearchIndex) set(point vector, node *transportPathNode) {
	if index.integer != nil {
		index.integer[transportSearchKey(point)] = node
	} else {
		index.general[point] = node
	}
}
