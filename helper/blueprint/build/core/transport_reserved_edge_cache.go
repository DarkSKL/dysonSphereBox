package build

// 文件职责：把整数边与固定预留带的几何相交核验摊到同一候选的多条路线。不同端口共享几何事实，每次按本路的真实源、宿判断可通行性。

// 整数格按 256 格分页，六个有向邻边各用一位表示与所有预留通道无交集。每个环境最多保存 4096 页。
const reservedPageBits = 12
const reservedOwnerBits = 10

// manyReservedOwners 表示至少三个不同所有者。一条路线至多拥有源和宿两个豁免。
const manyReservedOwners = ^uint32(0)

// reservedOwnerPair 用预留段的一基下标代表其所有者，零表示空，避免缓存持有额外实体指针。
type reservedOwnerPair struct {
	first, second uint32
}

type transportReservedEdgeEntry struct {
	revision, key uint32
	owners        reservedOwnerPair
}

type transportReservedPage struct {
	id, revision uint32
	free         [256]byte
}

// transportReservedEdgeCache 的格号属于当前整数网格，投影属于当前固定坐标框架。
type transportReservedEdgeCache struct {
	grid   *denseIntegerEdgeCache
	native bool
	frame  densePolarFrame
	pages  [1 << reservedPageBits]*transportReservedPage
	// entries 只存确有物理交集的边。大片空场不会逐边挤走端口周围的所有者记录。
	entries [1 << reservedOwnerBits]transportReservedEdgeEntry
}

// integerReservedSpanClear 的版本取自仅追加的预留段数。铺下普通带段只会改变其它占位检查，不能改变此边与预留通道的交集。
func (env *transportEnvironment) integerReservedSpanClear(first, second vector, cell, direction uint32, source, target *placed) bool {
	cache := env.reservedEdgeCache
	grid, frame := env.densePorts.integerEdges, env.densePorts.polarFrame
	if cache == nil || cache.grid != grid || cache.native != (frame != nil) || frame != nil && cache.frame != *frame {
		cache = &transportReservedEdgeCache{grid: grid, native: frame != nil}
		if frame != nil {
			cache.frame = *frame
		}
		env.reservedEdgeCache = cache
	}
	revision := uint32(len(env.reservedSpans))
	bit := byte(1) << direction
	page := cache.page(cell>>8, revision)
	if page.free[cell&255]&bit != 0 {
		return true
	}
	key := cell*6 + direction
	entry := &cache.entries[key*0x9e3779b1>>(32-reservedOwnerBits)]
	if entry.revision != revision || entry.key != key {
		owners := env.reservedOwners(first, second)
		if owners.first == 0 {
			page.free[cell&255] |= bit
			return true
		}
		*entry = transportReservedEdgeEntry{revision: revision, key: key, owners: owners}
	}
	owners := entry.owners
	if owners.first == manyReservedOwners {
		return false
	}
	owner := env.reservedSpans[owners.first-1].owner
	if owner != source && owner != target {
		return false
	}
	if owners.second == 0 {
		return true
	}
	owner = env.reservedSpans[owners.second-1].owner
	return owner == source || owner == target
}

// page 按需取得整数格页，只在格区或通道版本变化后清除该页。
func (cache *transportReservedEdgeCache) page(id, revision uint32) *transportReservedPage {
	bucket := id * 0x9e3779b1 >> (32 - reservedPageBits)
	page := cache.pages[bucket]
	if page == nil {
		page = new(transportReservedPage)
		cache.pages[bucket] = page
	}
	if page.id != id || page.revision != revision {
		*page = transportReservedPage{id: id, revision: revision}
	}
	return page
}

// reservedOwners 只需分清零、一、二或更多所有者。同一端口的多条预留段仍是一个所有者。
func (env *transportEnvironment) reservedOwners(first, second vector) (owners reservedOwnerPair) {
	env.visitReservedIntersections(first, second, func(number int) bool {
		owner := env.reservedSpans[number].owner
		if owners.first == 0 {
			owners.first = uint32(number + 1)
		} else if env.reservedSpans[owners.first-1].owner != owner {
			if owners.second == 0 {
				owners.second = uint32(number + 1)
			} else if env.reservedSpans[owners.second-1].owner != owner {
				owners.first = manyReservedOwners
				return false
			}
		}
		return true
	})
	return owners
}
