package build

// 文件职责：在一次端点枚举的固定编码作用域中复用纯世界坐标和姿态计算。
// 使用：fitNativeInserterEndpoints 创建子上下文，表面检查读取缓存；边界：不缓存规则成功、不跨锚点、记录提交或求解调用共享，不省略跨度与碰撞验证。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
)

// nativeInserterSurfaceCacheKey 职责：隔离求解内的缓存上下文；使用：固定编码端点枚举；边界：不使用进程全局状态。
type nativeInserterSurfaceCacheKey struct{}

// nativeInserterWorldKey 职责：完整标识原映射函数实际读取的记录字段；使用：单端点世界位置查询；边界：AreaIndex 与三维 float32 坐标逐值参与，不量化或取整。
type nativeInserterWorldKey struct {
	area  int8
	point blueprint.Point
}

// nativeInserterPoseKey 职责：完整标识同一编码下的姿态函数输入；使用：建筑和臂姿态查询；边界：保留俯仰、倾斜和双端不同方向。
type nativeInserterPoseKey struct {
	point            vector
	yaw, pitch, tilt float32
}

// nativeInserterPortKey 职责：完整标识固定编码中已核对端口的变换输入；使用：宿主端口纯计算缓存；边界：包括原型、槽位和输入输出方向，不承担连接有效性证明。
type nativeInserterPortKey struct {
	point       vector
	yaw         float32
	item, model int16
	slot        int8
	output      bool
}

// nativeInserterPortValue 职责：缓存端口姿态和按原调用顺序完成的格长；使用：同宿主同槽位候选；边界：gridReady仅表示纯格长计算已完成，不表示臂长、方向或跨度合法。
type nativeInserterPortValue struct {
	base, port surfacePose
	grid       float64
	gridReady  bool
}

// nativeInserterSurfaceCache 职责：持有一次不可变编码下的纯变换结果；使用：枚举期间共享宿主计算；边界：编码由枚举调用独占且不可修改，每张表达到8192项后清空以限制峰值内存。
type nativeInserterSurfaceCache struct {
	encoding *surfaceEncoding
	world    map[nativeInserterWorldKey]vector
	poses    map[nativeInserterPoseKey]surfacePose
	ports    map[nativeInserterPortKey]*nativeInserterPortValue
}

// withNativeInserterSurfaceCache 职责：显式建立本次枚举的纯计算作用域；使用：固定encoding之后、候选枚举之前；边界：每次调用创建新表，禁止沿用上次纠正证据。
func withNativeInserterSurfaceCache(ctx context.Context, encoding *surfaceEncoding) context.Context {
	cache := &nativeInserterSurfaceCache{encoding: encoding, world: make(map[nativeInserterWorldKey]vector), poses: make(map[nativeInserterPoseKey]surfacePose), ports: make(map[nativeInserterPortKey]*nativeInserterPortValue)}
	return context.WithValue(ctx, nativeInserterSurfaceCacheKey{}, cache)
}

// nativeInserterCacheFromContext 职责：读取与当前编码严格配对的枚举缓存；使用：动态盒入口；边界：无作用域或不同编码对象返回nil并运行原实现。
func nativeInserterCacheFromContext(ctx context.Context, encoding *surfaceEncoding) *nativeInserterSurfaceCache {
	cache, _ := ctx.Value(nativeInserterSurfaceCacheKey{}).(*nativeInserterSurfaceCache)
	if cache == nil || cache.encoding != encoding {
		return nil
	}
	return cache
}

// positions 职责：按原记录次序映射缓存未命中的端点；使用：枚举内反复检查三记录局部域；边界：错误仍由原映射器产生，不缓存失败、不修改输入切片，第一端和第二端仍分开校验。
func (cache *nativeInserterSurfaceCache) positions(records []blueprint.Building, secondary bool) ([]vector, error) {
	result := make([]vector, len(records))
	var recordStorage [3]blueprint.Building
	var indexStorage [3]int
	missing, indices := recordStorage[:0], indexStorage[:0]
	if len(records) > len(recordStorage) {
		missing = make([]blueprint.Building, 0, len(records))
		indices = make([]int, 0, len(records))
	}
	for i, record := range records {
		point := record.LocalOffset
		if secondary {
			point = record.LocalOffset2
		}
		if value, exists := cache.world[nativeInserterWorldKey{record.AreaIndex, point}]; exists {
			result[i] = value
		} else {
			record.LocalOffset = point
			missing = append(missing, record)
			indices = append(indices, i)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}
	positions, err := cache.encoding.surfaceWorldPositionsFor(missing, false)
	if err != nil {
		return nil, err
	}
	if len(cache.world)+len(missing) > 8192 {
		clear(cache.world)
	}
	for i, record := range missing {
		cache.world[nativeInserterWorldKey{record.AreaIndex, record.LocalOffset}] = positions[i]
		result[indices[i]] = positions[i]
	}
	return result, nil
}

// pose 职责：缓存逐值相同的姿态函数输入；使用：端点和宿主；边界：nil缓存直接调用原函数，任何失败不入表。
func (cache *nativeInserterSurfaceCache) pose(encoding *surfaceEncoding, position vector, yaw, pitch, tilt float32) (surfacePose, error) {
	if cache == nil {
		return encoding.recordPose(position, yaw, pitch, tilt)
	}
	key := nativeInserterPoseKey{position, yaw, pitch, tilt}
	if pose, exists := cache.poses[key]; exists {
		return pose, nil
	}
	pose, err := encoding.recordPose(position, yaw, pitch, tilt)
	if err == nil {
		if len(cache.poses) >= 8192 {
			clear(cache.poses)
		}
		cache.poses[key] = pose
	}
	return pose, err
}
