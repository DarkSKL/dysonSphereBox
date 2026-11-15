package build

// 文件职责：为一次运输搜索提供无碰撞整数键及通用坐标后备索引。
// 使用：多终点 A* 的已访问节点表；边界：不缓存占位或端口权限，不跨搜索复用动态状态。

import "math"

// transportSearchIndex 职责：保存当前搜索唯一的节点记录；使用：整数网格避免三浮点键的哈希和比较；边界：分数坐标或越界搜索整体回退，不能截断坐标合并节点。
type transportSearchIndex struct {
	integer map[uint64]*transportPathNode
	general map[vector]*transportPathNode
	// pages 职责：按需分配有界整数网格页；使用：避免热点坐标哈希；边界：超大或分数网格仍用原索引。
	pages []*[256]*transportPathNode
	// pageGeneration,generation 职责：隔离连续搜索的已访问槽；使用：按需清页而非反复分配；边界：旧页在本轮首次写入前不可见。
	pageGeneration []uint32
	generation     uint32
	// minX,minY,width,height 职责：无损定位页槽；使用：精确整数坐标；边界：查询相邻越界节点返回空，不发生坐标环绕。
	minX, minY, width, height int
}

// newTransportSearchIndex 职责：在起点和整个搜索域均可精确编码时选择整数键；使用：搜索开始前；边界：后续节点只由起点加减整数单位生成，非有限或过大边界使用原浮点键。
func newTransportSearchIndex(start, low, high vector) transportSearchIndex {
	within := func(v float64) bool { return !math.IsNaN(v) && v > -(1<<20) && v < (1<<20)-1 }
	if denseIntegerNode(start) && within(start.x) && within(start.y) && start.z >= 0 && start.z <= transportMaxHeight && within(low.x) && within(high.x) && within(low.y) && within(high.y) {
		minX, minY := int(math.Floor(low.x)), int(math.Floor(low.y))
		width, height := int(math.Ceil(high.x))-minX+1, int(math.Ceil(high.y))-minY+1
		if width > 0 && height > 0 && width <= (1<<22)/height/(transportMaxHeight+1) {
			size := width * height * (transportMaxHeight + 1)
			count := (size + 255) / 256
			return transportSearchIndex{pages: make([]*[256]*transportPathNode, count), pageGeneration: make([]uint32, count), generation: 1, minX: minX, minY: minY, width: width, height: height}
		}
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
	if index.pages != nil {
		x, y, z := int(point.x)-index.minX, int(point.y)-index.minY, int(point.z)
		if x < 0 || x >= index.width || y < 0 || y >= index.height || z < 0 || z > transportMaxHeight {
			return nil
		}
		slot := (z*index.height+y)*index.width + x
		if index.pageGeneration[slot>>8] != index.generation {
			return nil
		}
		if page := index.pages[slot>>8]; page != nil {
			return page[slot&255]
		}
		return nil
	}
	if index.integer != nil {
		return index.integer[transportSearchKey(point)]
	}
	return index.general[point]
}

// set 职责：登记新发现的合法节点；使用：加入开放队列之前；边界：由调用方先核对边界和真实几何。
func (index *transportSearchIndex) set(point vector, node *transportPathNode) {
	if index.pages != nil {
		slot := (int(point.z)*index.height+int(point.y)-index.minY)*index.width + int(point.x) - index.minX
		page := index.pages[slot>>8]
		if page == nil {
			page = new([256]*transportPathNode)
			index.pages[slot>>8] = page
		}
		if index.pageGeneration[slot>>8] != index.generation {
			clear(page[:])
			index.pageGeneration[slot>>8] = index.generation
		}
		page[slot&255] = node
	} else if index.integer != nil {
		index.integer[transportSearchKey(point)] = node
	} else {
		index.general[point] = node
	}
}

// transportNodeArena 职责：成块保存当前搜索节点；使用：串行调用间复用内存；边界：每轮重置状态，搜索结束前节点地址稳定。
type transportNodeArena struct {
	blocks []*[256]transportPathNode
	used   int
}

// add 职责：分配稳定节点地址并写入全部初始状态；使用：开放队列创建节点；边界：单轮内不覆盖已有节点，下一轮须同步切换访问索引代次。
func (arena *transportNodeArena) add(value transportPathNode) *transportPathNode {
	block, offset := arena.used>>8, arena.used&255
	if block == len(arena.blocks) {
		arena.blocks = append(arena.blocks, new([256]transportPathNode))
	}
	node := &arena.blocks[block][offset]
	*node = value
	arena.used++
	return node
}

// reset 职责：为下一次同域整数搜索开启独立代次；使用：候选私有寻路工作区；边界：分数坐标或边界变化重新选择索引，不能复用旧关闭状态。
func (index *transportSearchIndex) reset(start, low, high vector) {
	if index.pages != nil && denseIntegerNode(start) && start.z >= 0 && start.z <= transportMaxHeight && int(math.Floor(low.x)) == index.minX && int(math.Floor(low.y)) == index.minY && int(math.Ceil(high.x))-index.minX+1 == index.width && int(math.Ceil(high.y))-index.minY+1 == index.height {
		index.generation++
		if index.generation == 0 {
			clear(index.pageGeneration)
			index.generation = 1
		}
		return
	}
	*index = newTransportSearchIndex(start, low, high)
}
