package build

// 文件职责：复用同一串行路由索引的纯球面格点投影，减少邻边与节点查询中的重复三角函数和坐标校验。
// 使用：denseRoutePortIndex.world；边界：原生投影仍由唯一frame.world完成，不跨候选、不存规则成功或失败，平面路径保持原计算。
import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// denseRouteWorldCacheLimit 职责：限定每个索引缓存条目；使用：插入前清空已满表；边界：不改变任何路由搜索、实体或物理预算。
const denseRouteWorldCacheLimit = 32768

// denseRouteWorldCache 职责：绑定完整固定框架与精确输入位模式；使用：重复查询带中心；边界：输出按值保存，框架变化立即失效，索引本身仍由候选独占。
type denseRouteWorldCache struct {
	frame  densePolarFrame
	points map[[3]uint64]placement.Vector
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
		cache.frame = *index.polarFrame
	}
	key := [3]uint64{math.Float64bits(p.x), math.Float64bits(p.y), math.Float64bits(p.z)}
	if point, exists := cache.points[key]; exists {
		return point
	}
	point := index.polarFrame.world(p, .2)
	if len(cache.points) >= denseRouteWorldCacheLimit {
		clear(cache.points)
	}
	cache.points[key] = point
	return point
}
