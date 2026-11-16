package build

// 文件职责：为普通塔组合的真实生产行出货口保留有限离开通道；边界：仅由存在真实塔边界的组合调用，不保护任意内部节点或整列。
import "sort"

// reserveStationProductionEscapes 职责：按真实生产行联合选择合法短出口；边界：不改变物料路径或实体，所有实际路线仍执行原搜索与完整规则。
func (env *transportEnvironment) reserveStationProductionEscapes(routes []*transportRoute) int {
	if env.densePorts == nil || !env.densePorts.strictBelts {
		return 0
	}
	groups := map[*transportRow][]*transportRoute{}
	towers := []*transportRow{}
	seen := map[*placed]bool{}
	for _, r := range routes {
		if r == nil || r.from == nil || r.to == nil || r.from.line == nil || r.start == nil || r.end == nil || seen[r.transportSource()] {
			continue
		}
		seen[r.transportSource()] = true
		if _, ok := groups[r.from]; !ok {
			towers = append(towers, r.from)
		}
		groups[r.from] = append(groups[r.from], r)
	}
	count := 0
	for _, tower := range towers {
		// 先在低层完整组合，失败才覆盖全部32层；不省略原候选能力。
		for _, maximumHeight := range []float64{4, transportMaxHeight} {
			ports := []stationEscapePort{}
			for _, r := range groups[tower] {
				source, target := r.transportSource(), r.end
				port := stationEscapePort{source: source}
				preferred := denseRotateVector(vector{x: -1}, r.from.quarterTurns)
				outward := env.densePorts.outward(source, preferred, true)
				directions := append([]vector(nil), transportNeighborOffsets[:]...)
				sort.SliceStable(directions, func(i, j int) bool {
					a, b := directions[i], directions[j]
					return a.x*outward.x+a.y*outward.y > b.x*outward.x+b.y*outward.y
				})
				for _, portal := range env.densePortals(source, source, target, preferred, true, false, maximumHeight) {
					exit := portal.points[len(portal.points)-1]
					prefix := make([]packingShape, 0, len(portal.points)-1)
					previous := env.densePorts.world(portal.points[0])
					for _, p := range portal.points[1:] {
						nextWorld := env.densePorts.world(p)
						prefix = append(prefix, beltPackingShape(previous, nextWorld))
						previous = nextWorld
					}
					for _, delta := range directions {
						next := vector{exit.x + delta.x, exit.y + delta.y, exit.z + delta.z}
						if next.x < env.minimum.x || next.x > env.maximum.x || next.y < env.minimum.y || next.y > env.maximum.y || next.z < 0 || next.z > transportMaxHeight || env.blocked[beltCell(next)] || !env.densePorts.nodeClear(next, nil) || env.reservedByOther(next, source, target) || !env.denseTransportStepClear(exit, next) || !env.reservedSpanClear(exit, next, source, target) {
							continue
						}
						choice := stationEscapeChoice{points: append(append([]vector(nil), portal.points...), next)}
						choice.shapes = append(append([]packingShape(nil), prefix...), beltPackingShape(previous, env.densePorts.world(next)))
						port.choices = append(port.choices, choice)
					}
				}
				ports = append(ports, port)
			}
			selected := chooseStationEscapes(ports)
			if selected == nil {
				continue
			}
			for i, which := range selected {
				port := ports[i]
				points := port.choices[which].points
				for j := 1; j < len(points); j++ {
					env.reserveTransportSpan(points[j-1], points[j], port.source)
					// 原生塔口有分数坐标，不能四舍五入为独占整数格：
					// 其粗格可能覆盖世界坐标中不相交的相邻口。整段预留仍由同一精确段距保护。
				}
				count++
			}
			break
		}
	}
	return count
}
