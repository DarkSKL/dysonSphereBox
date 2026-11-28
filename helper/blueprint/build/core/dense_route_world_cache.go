package build

// 文件职责：复用同一串行路由索引的纯球面格点投影，减少邻边与节点查询中的重复三角函数和坐标校验。
// 使用：denseRoutePortIndex.world；边界：原生投影仍由唯一frame.world完成，不跨候选、不存规则成功或失败，平面路径保持原计算。
import (
	"math"
	"math/bits"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// denseRouteWorldCacheLimit 限定后备投影表条目数；表满时清空，另有 256 个近期查询槽。
const denseRouteWorldCacheLimit = 32768

// denseRouteWorldRecent 保存近期邻边反复使用的精确投影。直接寻址的小表只在索引积累了一批不同坐标后创建。
type denseRouteWorldRecent [256]struct {
	key   [3]uint64
	world placement.Vector
	valid bool
}

// denseRouteWorldCache 职责：绑定完整固定框架与精确输入位模式；使用：重复查询带中心；边界：输出按值保存，框架变化立即失效，索引本身仍由候选独占。
type denseRouteWorldCache struct {
	frame  densePolarFrame
	points map[[3]uint64]placement.Vector
	recent *denseRouteWorldRecent
}

// cachedWorld 职责：返回与原float32球面转换逐位一致的带中心；使用：仅非nil原生frame的内部入口。
// 边界：正负零不合并，非法坐标仍由原方法拒绝，错误不会存入缓存；容量耗尽仅清空纯结果而不影响求解语义。
func (index *denseRoutePortIndex) cachedWorld(p vector) placement.Vector {
	cache := index.worldCache
	if cache == nil {
		cache = &denseRouteWorldCache{frame: *index.polarFrame, points: make(map[[3]uint64]placement.Vector)}
		index.worldCache = cache
	} else if cache.frame != *index.polarFrame {
		clear(cache.points)
		if cache.recent != nil {
			clear(cache.recent[:])
		}
		cache.frame = *index.polarFrame
	}
	key := [3]uint64{math.Float64bits(p.x), math.Float64bits(p.y), math.Float64bits(p.z)}
	if cache.recent == nil && len(cache.points) >= 32 {
		cache.recent = new(denseRouteWorldRecent)
	}
	var slot uint64
	if cache.recent != nil {
		h := key[0] ^ bits.RotateLeft64(key[1], 21) ^ bits.RotateLeft64(key[2], 42)
		slot = ((h ^ (h >> 29)) * 0x9e3779b97f4a7c15) >> 56
		if entry := &cache.recent[slot]; entry.valid && entry.key == key {
			return entry.world
		}
	}
	point, exists := cache.points[key]
	if !exists {
		point = index.polarFrame.world(p, .2)
		if len(cache.points) >= denseRouteWorldCacheLimit {
			clear(cache.points)
		}
		cache.points[key] = point
	}
	if cache.recent != nil {
		entry := &cache.recent[slot]
		entry.key, entry.world, entry.valid = key, point, true
	}
	return point
}
