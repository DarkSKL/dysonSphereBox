package build

// 文件职责：为普通塔的真实外供口联合选择互不交叠的有限出口。
// 使用：全场布线之前；边界：只组合已通过原规则的portal，不预留所有端点或整列，不修改物理规则及A*预算。
import "sort"

// stationEscapeChoice 职责：保存已核验的有限出口及精确占位；边界：只供预留组合，不创建实体。
type stationEscapeChoice struct {
	points []vector
	shapes []packingShape
}

// stationEscapePort 职责：绑定真实端点及其合法出口；边界：同塔联合规划不交换物料身份。
type stationEscapePort struct {
	source  *placed
	choices []stationEscapeChoice
}

// stationEscapesConflict 职责：复用连续预留的原半径和精确段距；边界：包围盒只做宽相位，不允许不同物料共享虚拟端点。
func stationEscapesConflict(a, b stationEscapeChoice) bool {
	for _, x := range a.shapes {
		for _, y := range b.shapes {
			if x.high[0] < y.low[0] || y.high[0] < x.low[0] || x.high[1] < y.low[1] || y.high[1] < x.low[1] || x.high[2] < y.low[2] || y.high[2] < x.low[2] {
				continue
			}
			if surfaceSpanDistanceSquared(x.span, y.span) < .46*.46-1e-6 {
				return true
			}
		}
	}
	return false
}

// chooseStationEscapes 职责：同塔最多十二口按原候选顺序联合选择；边界：只限制预留组合尝试，耗尽时不登记部分保护，不改变实际路线搜索能力。
func chooseStationEscapes(ports []stationEscapePort) []int {
	selected := make([]int, len(ports))
	for i := range selected {
		selected[i] = -1
	}
	attempts := 0
	var search func(int) bool
	search = func(n int) bool {
		if n == len(ports) {
			return true
		}
		for i, c := range ports[n].choices {
			attempts++
			if attempts > 100000 {
				return false
			}
			valid := true
			for j := 0; j < n; j++ {
				if stationEscapesConflict(c, ports[j].choices[selected[j]]) {
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			// 前瞻只排除与当前选择全部冲突的剩余口，不剪掉任何可兼容候选。
			for j := n + 1; j < len(ports) && valid; j++ {
				any := false
				for _, other := range ports[j].choices {
					if stationEscapesConflict(c, other) {
						continue
					}
					ok := true
					for k := 0; k < n; k++ {
						if stationEscapesConflict(other, ports[k].choices[selected[k]]) {
							ok = false
							break
						}
					}
					if ok {
						any = true
						break
					}
				}
				if !any {
					valid = false
				}
			}
			if valid {
				selected[n] = i
				if search(n + 1) {
					return true
				}
				selected[n] = -1
			}
		}
		return false
	}
	if search(0) {
		return selected
	}
	return nil
}

// reserveStationInputEscapes 职责：保护普通塔真实外供及入塔口的连续portal；边界：只覆盖真实塔边界，按塔联合选择，未找到完整组合不伪造可行性。
func (env *transportEnvironment) reserveStationInputEscapes(routes []*transportRoute) int {
	if env.densePorts == nil || !env.densePorts.strictBelts {
		return 0
	}
	groups := map[*placed][]*transportRoute{}
	towers := []*placed{}
	seen := map[*placed]bool{}
	for _, r := range routes {
		if r == nil || (r.from == nil) == (r.to == nil) || r.boundary == nil || !surfaceIsStation(r.boundary.record.ItemID) || r.start == nil || r.end == nil {
			continue
		}
		endpoint := r.transportSource()
		if r.to == nil {
			endpoint = r.end
		}
		if seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		if _, ok := groups[r.boundary]; !ok {
			towers = append(towers, r.boundary)
		}
		groups[r.boundary] = append(groups[r.boundary], r)
	}
	count := 0
	for _, tower := range towers {
		// 先在低层完整组合，失败才覆盖全部32层；不省略原候选能力。
		for _, maximumHeight := range []float64{4, transportMaxHeight} {
			ports := []stationEscapePort{}
			for _, r := range groups[tower] {
				source, target := r.transportSource(), r.end
				endpoint, preferred, sourceSide := source, denseExternalStep(r), true
				if r.to == nil {
					endpoint, preferred, sourceSide = target, vector{x: -1}, false
				}
				port := stationEscapePort{source: endpoint}
				outward := env.densePorts.outward(endpoint, preferred, sourceSide)
				directions := append([]vector(nil), transportNeighborOffsets[:]...)
				sort.SliceStable(directions, func(i, j int) bool {
					a, b := directions[i], directions[j]
					return a.x*outward.x+a.y*outward.y > b.x*outward.x+b.y*outward.y
				})
				for _, portal := range env.densePortals(endpoint, source, target, preferred, sourceSide, false, maximumHeight) {
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
	env.reserveStationConsumerEntries(routes)
	if len(towers) > 0 {
		env.reserveStationProductionEscapes(routes)
	}
	return count
}
