package build

// 文件职责：将连续街道的分数低轨端点经真实带段连接到共享塔的地面端口。
// 使用：独立街道塔候选；边界：不取整端点、不以整列禁行隔开街道，不向最终库存加入虚拟实体。

import (
	"context"
	"slices"

	"DysonSphereBox/helper/blueprint"
)

// prepareDenseStationPortalIndex 职责：登记真实轮廓并为所有塔口短尾保留精确节点净距；使用：全场街道接线之前；边界：临时节点仅描述尚待创建的塔尾，最终records只能包含实际装配带。
func (b *builder) prepareDenseStationPortalIndex(ctx context.Context, env *transportEnvironment, routes []denseStationRoute, ends []*placed, towers []denseStationTower) error {
	index, err := b.newDenseRoutePortIndex(ctx)
	if err != nil {
		return err
	}
	env.densePorts = index
	makePending := func(point vector) *placed {
		pending := &placed{a: point, record: blueprint.Building{ItemID: int16(2000 + b.setting.BeltLv)}}
		index.addNode(pending)
		return pending
	}
	for i, route := range routes {
		end := ends[i]
		end.record.ItemID = int16(2000 + b.setting.BeltLv)
		index.addNode(end)
		inner := denseStationInner(towers[route.tower].tower.a, route.slot)
		first := makePending(inner[0])
		makePending(inner[1])
		// 相同的内侧邻居可证明出塔或入塔的向外方向；连接只用于接驳候选，不写进最终蓝图。
		end.output = first
		index.previous[end] = []*placed{first}
	}
	for _, tower := range towers {
		for _, slot := range tower.reserved {
			for _, point := range append(denseStationInner(tower.tower.a, slot), denseStationOuter(tower.tower.a, slot)) {
				makePending(point)
			}
		}
	}
	return nil
}

// findDenseStationPath 职责：按真实物料方向连接低轨和塔外端点；使用：街道统一路由；边界：返回仍按机器端至塔端排列，供料链随后反向装配，临时route不进入规范流量账。
func (env *transportEnvironment) findDenseStationPath(ctx context.Context, route *denseStationRoute, outer, tower *placed) ([]vector, int) {
	work := &transportRoute{start: route.head, end: outer, to: &transportRow{}}
	preferred := denseRotateVector(vector{x: -1}, route.row.turns)
	if !route.output {
		work.start, work.end = outer, route.head
		work.to.quarterTurns = route.row.turns
		preferred = vector{outer.a.x - tower.a.x, outer.a.y - tower.a.y, 0}
	}
	points, _, expanded, _, _ := env.findDenseTransportPath(ctx, work, nil, nil, preferred)
	if len(points) == 0 {
		return nil, expanded
	}
	if route.output {
		return append([]vector{route.head.a}, points...), expanded
	}
	// points不含源塔外节点而包含复用机器头；改为机器头→塔外节点，后续保留原塔链证据契约。
	path := append([]vector{outer.a}, points[:len(points)-1]...)
	slices.Reverse(path)
	return append([]vector{route.head.a}, path...), expanded
}
