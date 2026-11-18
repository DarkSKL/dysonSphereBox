package build

// 文件职责：共用侧向接驳的物理守卫，保留既有半格候选的逐项行为。
// 使用：原完整寻路失败后；边界：不移动端点、不更改2.15米硬上限或任何净空、端三跳、预留及整链规则，不作为主候选替代。

import (
	"math"
	"sort"
)

// denseLateralPortals 职责：在调用方固定的侧移距离与邻近整数点中生成真实短链。
// 使用：半格原候选及1/1.5格备用共用全部物理判断；边界：perLevelLimit为0时保留这组有限出口，所有段仍按真实世界净空校验，不因侧移格数获得距离豁免。
func (env *transportEnvironment) denseLateralPortals(endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool, lengths []float64, perLevelLimit int) []denseRoutePortal {
	if env.densePorts == nil || endpoint == nil || fixedGroundStem {
		return nil
	}
	index := env.densePorts
	outward := index.outward(endpoint, preferred, sourceSide)
	linked := index.linkedPortBuildings(endpoint, sourceSide)
	clear := func(path []vector, point vector) bool {
		return env.densePortalPointClear(path, point, source, target, linked)
	}
	byExit := map[vector]denseRoutePortal{}
	for _, length := range lengths {
		for _, sign := range []float64{-1, 1} {
			stem := vector{endpoint.a.x - sign*outward.y*length, endpoint.a.y + sign*outward.x*length, endpoint.a.z}
			base := []vector{endpoint.a}
			if !clear(base, stem) {
				continue
			}
			base = append(base, stem)
			for z := math.Floor(stem.z); z <= min(float64(transportMaxHeight), math.Ceil(stem.z)+1); z++ {
				for dx := -1.; dx <= 1; dx++ {
					for dy := -1.; dy <= 1; dy++ {
						point := vector{math.Round(stem.x) + dx, math.Round(stem.y) + dy, z}
						if !clear(base, point) {
							continue
						}
						candidate := denseRoutePortal{points: []vector{endpoint.a, stem, point}, height: max(endpoint.a.z, z)}
						candidate.cost = denseRouteLength(candidate.points)
						if old, ok := byExit[point]; !ok || candidate.cost < old.cost-1e-9 {
							byExit[point] = candidate
						}
					}
				}
			}
		}
	}
	portals := make([]denseRoutePortal, 0, len(byExit))
	for _, p := range byExit {
		portals = append(portals, p)
	}
	sort.Slice(portals, func(i, j int) bool {
		a, b := portals[i], portals[j]
		if a.height != b.height {
			return a.height < b.height
		}
		if math.Abs(a.cost-b.cost) > 1e-9 {
			return a.cost < b.cost
		}
		x, y := a.points[2], b.points[2]
		if x.x != y.x {
			return x.x < y.x
		}
		if x.y != y.y {
			return x.y < y.y
		}
		return x.z < y.z
	})
	out := []denseRoutePortal{}
	levels := map[int]int{}
	for _, p := range portals {
		level := int(math.Ceil(p.height))
		if perLevelLimit <= 0 || levels[level] < perLevelLimit {
			out = append(out, p)
			levels[level]++
		}
	}
	return out
}

// densePortalPointClear 职责：统一有限接驳的新节点、真实连续段、端三跳权限及路径自距判断。
// 使用：半格、外侧及先升层源端接驳共用；边界：不豁免已有端点以外的节点，不放宽米制段长或连续预留。
func (env *transportEnvironment) densePortalPointClear(path []vector, point vector, source, target *placed, linked map[*placed]int) bool {
	if point.x < env.minimum.x || point.x > env.maximum.x || point.y < env.minimum.y || point.y > env.maximum.y || point.z < 0 || point.z > transportMaxHeight || !env.densePorts.nodeClear(point, nil) || env.reservedByOther(point, source, target) || !env.reservedSpanClear(path[len(path)-1], point, source, target) || !env.densePorts.spanClearWithLinks(path[len(path)-1], point, densePortalLinkPermissions(linked, len(path)-1, len(path))) {
		return false
	}
	for _, previous := range path {
		delta := env.densePorts.world(previous).Sub(env.densePorts.world(point))
		if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
			return false
		}
	}
	return true
}
