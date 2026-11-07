package build

// 文件职责：用有界紧凑数组缓存密铺整数相邻带边的静态几何结果。
// 使用：路由环境实际边界确定后配置；边界：不缓存动态节点、端口权限、分数节点或长边，不改变任何几何判断。

import "math"

// denseIntegerEdgeCacheBytes 职责：限定单路由环境的紧凑缓存开销；使用：分配前检查；边界：超出时完整回退原稀疏缓存和实算，不裁减搜索空间。
const denseIntegerEdgeCacheBytes = 8 << 20

// denseIntegerEdgeCache 职责：每格以三个两位状态保存正X/Y/Z的无向相邻边；使用：串行候选内复用静态盒结果；边界：0未知、1阻塞、2可通，索引生命周期与不变的静态盒一致。
type denseIntegerEdgeCache struct {
	minimum [3]int
	size    [3]int
	plane   int
	states  []byte
}

// newDenseIntegerEdgeCache 职责：按整数节点实际可达边界创建缓存；使用：环境初始化；边界：非有限、极大坐标和超过8MiB的体积返回nil，绝不扩大原边界。
func newDenseIntegerEdgeCache(minimum, maximum vector) *denseIntegerEdgeCache {
	lower, upper := [3]float64{minimum.x, minimum.y, minimum.z}, [3]float64{maximum.x, maximum.y, maximum.z}
	cache := &denseIntegerEdgeCache{}
	volume := 1
	for axis := range lower {
		lo, hi := math.Ceil(lower[axis]), math.Floor(upper[axis])
		if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || lo > hi || math.Abs(lo) > 1<<30 || math.Abs(hi) > 1<<30 {
			return nil
		}
		extent := hi - lo + 1
		if extent > float64(denseIntegerEdgeCacheBytes/volume) {
			return nil
		}
		cache.minimum[axis], cache.size[axis] = int(lo), int(extent)
		volume *= cache.size[axis]
	}
	cache.plane = cache.size[0] * cache.size[1]
	cache.states = make([]byte, volume)
	return cache
}

// slot 职责：为边界内恰好一格的无向正交边返回唯一格与两位偏移；使用：查找和存储共用；边界：浮点整数检查严格相等，不把分数端口四舍五入成整数。
func (cache *denseIntegerEdgeCache) slot(first, second vector) (int, uint) {
	if cache == nil {
		return -1, 0
	}
	ax, ay, az := int(first.x), int(first.y), int(first.z)
	bx, by, bz := int(second.x), int(second.y), int(second.z)
	if float64(ax) != first.x || float64(ay) != first.y || float64(az) != first.z || float64(bx) != second.x || float64(by) != second.y || float64(bz) != second.z {
		return -1, 0
	}
	lx, ly, lz := cache.minimum[0], cache.minimum[1], cache.minimum[2]
	ux, uy, uz := lx+cache.size[0], ly+cache.size[1], lz+cache.size[2]
	if ax < lx || ax >= ux || ay < ly || ay >= uy || az < lz || az >= uz || bx < lx || bx >= ux || by < ly || by >= uy || bz < lz || bz >= uz {
		return -1, 0
	}
	var shift uint
	switch {
	case ay == by && az == bz && (ax-bx == 1 || bx-ax == 1):
		ax = min(ax, bx)
	case ax == bx && az == bz && (ay-by == 1 || by-ay == 1):
		ay, shift = min(ay, by), 2
	case ax == bx && ay == by && (az-bz == 1 || bz-az == 1):
		az, shift = min(az, bz), 4
	default:
		return -1, 0
	}
	return (az-lz)*cache.plane + (ay-ly)*cache.size[0] + ax - lx, shift
}

// store 职责：只覆盖目标边的两位静态状态；使用：原几何计算完成之后；边界：调用方保证slot有效且没有原生端口权限。
func (cache *denseIntegerEdgeCache) store(slot int, shift uint, clear bool) {
	state := byte(1)
	if clear {
		state = 2
	}
	cache.states[slot] = cache.states[slot]&^(byte(3)<<shift) | state<<shift
}
