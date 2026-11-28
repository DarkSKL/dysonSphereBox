package build

// 文件职责：在一次直连路由准备内共享完全相同的固定占位。
// 使用：地面边界预检和主路占位初始化按当前几何核对后共享。
// 边界：静态索引发布后只读；真实带与端口查询状态由各次构造独立维护。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// denseRouteStaticPreparationKey 职责：区分单次路由准备的上下文值；使用：routeTransport；边界：独立准备入口默认没有共享状态。
type denseRouteStaticPreparationKey struct{}

// denseRouteStaticPreparation 职责：交接一个候选的固定占位；使用：同一builder的串行路由准备；边界：主路由只取一次，其他builder不可消费。
type denseRouteStaticPreparation struct {
	owner    *builder
	geometry *denseRouteStaticGeometry
}

// denseRouteStaticIndex 职责：保存按原建筑次序构造的静态盒和空间桶；使用：整段净空查询；边界：盒原点、身份和桶共用同一下标，构造完毕后只读。
type denseRouteStaticIndex struct {
	boxes   []packingShape
	origins []placement.Vector
	owners  []*placed
	cells   map[[3]int][]int
}

// denseRouteStaticGeometry 职责：绑定静态索引实际使用的实体、框架与箱罐顶盖规则；使用：准备阶段交接；边界：始终重新核对当前输入。
type denseRouteStaticGeometry struct {
	index    denseRouteStaticIndex
	entities []denseRouteStaticEntity
	frame    *densePolarFrame
	roofs    bool
}

// denseRouteStaticEntity 职责：保存带身份的固定建造盒输入；使用：与当前实体逐项核对；边界：浮点按位比较，记录连接与带位置不参与。
type denseRouteStaticEntity struct {
	entity      *placed
	x, y, z     uint64
	yaw         uint32
	item, model int16
}

// denseRouteStaticObstacle 职责：筛出进入静态桶的建筑；使用：构造及核对；边界：传送带和分拣器由各自的动态规则处理。
func denseRouteStaticObstacle(entity *placed) bool {
	item := entity.record.ItemID
	return !surfaceIsBelt(item) && (item < 2011 || item > 2014)
}

// denseRouteStaticSignature 职责：记录建造盒所消费的实体值；使用：备份原值或核对；边界：身份变化按不同静态版本处理。
func denseRouteStaticSignature(entity *placed) denseRouteStaticEntity {
	return denseRouteStaticEntity{
		entity: entity,
		x:      math.Float64bits(entity.a.x),
		y:      math.Float64bits(entity.a.y),
		z:      math.Float64bits(entity.a.z),
		yaw:    math.Float32bits(entity.record.Yaw),
		item:   entity.record.ItemID,
		model:  entity.record.ModelIndex,
	}
}

// matches 职责：检查当前静态序列可精准复用；使用：新动态视图初始化；边界：顺序、身份、姿态、模型、框架或策略变化即重建，核对过程中响应取消。
func (g *denseRouteStaticGeometry) matches(ctx context.Context, entities []*placed, frame *densePolarFrame, roofs bool) (bool, error) {
	if g.roofs != roofs || (g.frame == nil) != (frame == nil) || (frame != nil && *g.frame != *frame) {
		return false, nil
	}
	i := 0
	for _, entity := range entities {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !denseRouteStaticObstacle(entity) {
			continue
		}
		if i == len(g.entities) || g.entities[i] != denseRouteStaticSignature(entity) {
			return false, nil
		}
		i++
	}
	return i == len(g.entities), nil
}
