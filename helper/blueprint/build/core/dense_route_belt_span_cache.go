package build

// 文件职责：避免同一带段占位内反复计算整数边的动态带段净空。
// 使用：无端口权限的正交整数邻边通过静态检查之后；边界：不复用分数或斜边、动态节点、连续预留或端口权限。

// denseBeltSpanCacheBits 职责：限制单个活跃寻路索引的动态缓存为128 KiB；使用：固定大小覆写表；边界：竞争槽只覆盖缓存，不影响原查询。
const denseBeltSpanCacheBits = 14

// denseBeltSpanCache 职责：保存有向整数边和已登记带段数量的净空结果；使用：串行占位索引私有；边界：索引的带段只追加，换框架或重建占位创建新索引。
type denseBeltSpanCache [1 << denseBeltSpanCacheBits]uint64

// cachedBeltSpanClear 职责：命中同一占位和方向时复用原精确结论；使用：已有紧凑整数边槽且静态检查通过；边界：每次新带段登记令全部旧项失效，正反方向独立保留原浮点行为。
func (index *denseRoutePortIndex) cachedBeltSpanClear(first, second vector, slot int, shift uint) bool {
	revision := len(index.belts)
	if revision == 0 || !index.strictBelts && !index.boundarySegment(first, second) {
		return true
	}
	if uint64(revision) > uint64(^uint32(0)) {
		return index.indexedBeltSpanClear(first, second)
	}
	if index.beltSpanCache == nil {
		index.beltSpanCache = new(denseBeltSpanCache)
	}
	// 静态槽号小于8Mi个网格节点。低位分别编码三轴、方向和查询结果。
	key := uint32(slot)*12 + uint32(shift)*2
	if first.x > second.x || first.y > second.y || first.z > second.z {
		key += 2
	}
	bucket := key * 0x9e3779b1 >> (32 - denseBeltSpanCacheBits)
	token := uint64(revision)<<32 | uint64(key)
	entry := index.beltSpanCache[bucket]
	if entry&^1 == token {
		return entry&1 != 0
	}
	clear := index.indexedBeltSpanClear(first, second)
	if clear {
		token |= 1
	}
	index.beltSpanCache[bucket] = token
	return clear
}
