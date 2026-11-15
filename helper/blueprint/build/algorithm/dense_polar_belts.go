package build

// 文件职责：在极区原生格距中补足局部单元被拉长的传送带段。
// 使用：局部坐标固定后、占位及跨单元布线前；边界：保留已有带头和分拣器宿主，补带同步到实体路径并接受最终货物与几何复验。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// expandPolarBeltPath 职责：同步有向或反向保存的原始带序；使用：机器带及设施路径；边界：仅扩展原本相邻的两个实体，不按坐标猜连接。
func expandPolarBeltPath(path []*placed, bridges map[*placed]connectedSurfaceBridge) []*placed {
	result := make([]*placed, 0, len(path))
	for i, p := range path {
		result = append(result, p)
		if i+1 == len(path) {
			continue
		}
		next := path[i+1]
		if bridge, ok := bridges[p]; ok && bridge.target == next {
			result = append(result, bridge.belts...)
		} else if bridge, ok := bridges[next]; ok && bridge.target == p {
			for j := len(bridge.belts) - 1; j >= 0; j-- {
				result = append(result, bridge.belts[j])
			}
		}
	}
	return result
}

// resamplePolarLocalBelts 职责：将球面中过长的原带边拆为真实节点并同步所有局部证据；使用：极区单元定位；边界：只补带，不挪动既有节点、不提升设备等级或隐藏超量建筑。
func (b *builder) resamplePolarLocalBelts(ctx context.Context, units []denseSpatialUnit) error {
	bridges := map[*placed]connectedSurfaceBridge{}
	original := append([]*placed(nil), b.all...)
	owner := b.owner
	defer func() { b.owner = owner }()
	for _, p := range original {
		if err := ctx.Err(); err != nil {
			return err
		}
		next := p.output
		if !surfaceIsBelt(p.record.ItemID) || next == nil || !surfaceIsBelt(next.record.ItemID) {
			continue
		}
		delta := b.polarFrame.world(p.a, .2).Sub(b.polarFrame.world(next.a, .2))
		length := math.Sqrt(delta.Dot(delta))
		if length <= denseRouteMaximumSpan {
			continue
		}
		count := int(math.Ceil(length / 1.8))
		if count > 256 || len(b.all)+count-1 > blueprint.MaxBuildingCount {
			return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "极区局部带段补点超出实体预算"}
		}
		added := make([]*placed, 0, count-1)
		b.owner = p.owner
		for i := 1; i < count; i++ {
			u := float64(i) / float64(count)
			q := b.make(int(p.record.ItemID), p.a.x+(next.a.x-p.a.x)*u, p.a.y+(next.a.y-p.a.y)*u, p.a.z+(next.a.z-p.a.z)*u)
			q.record.Yaw, q.record.Yaw2 = p.record.Yaw, p.record.Yaw2
			added = append(added, q)
		}
		previous := p
		for _, q := range added {
			connect(previous, q)
			previous = q
		}
		connect(previous, next)
		if next.input == p {
			next.input = previous
		}
		bridges[p] = connectedSurfaceBridge{target: next, belts: added}
	}
	if len(bridges) == 0 {
		return nil
	}
	// 先同步不保证有向排序的实体集合，再同步所有保留的有向/反向路径视图。
	extendMembers := func(members []*placed) []*placed {
		result := append([]*placed(nil), members...)
		for _, p := range members {
			if bridge, ok := bridges[p]; ok {
				result = append(result, bridge.belts...)
			}
		}
		return result
	}
	rows := map[*transportRow]bool{}
	for i := range units {
		units[i].entities = extendMembers(units[i].entities)
		for _, row := range units[i].transportRows() {
			rows[row] = true
		}
	}
	for _, line := range b.machineLines {
		line.all = extendMembers(line.all)
		for i := range line.inputs {
			line.inputs[i] = expandPolarBeltPath(line.inputs[i], bridges)
		}
		for i := range line.outputs {
			line.outputs[i] = expandPolarBeltPath(line.outputs[i], bridges)
		}
		line.output = expandPolarBeltPath(line.output, bridges)
	}
	for _, p := range b.all {
		if p.densityCargo.enabled {
			// 只纳入原物料轨两个已知相邻节点之间的真实补带，保持取货窗口的物料归属。
			p.densityCargo.track = expandPolarBeltPath(p.densityCargo.track, bridges)
		}
	}
	for _, ref := range b.connectedSurfacePathRefs() {
		*ref = expandPolarBeltPath(*ref, bridges)
	}
	seen := map[*transportRoute]bool{}
	updateRoute := func(route *transportRoute) {
		if route != nil && !seen[route] {
			route.path = expandPolarBeltPath(route.path, bridges)
			route.sharedPath = expandPolarBeltPath(route.sharedPath, bridges)
			seen[route] = true
		}
	}
	for row := range rows {
		for item, path := range row.inputStubs {
			row.inputStubs[item] = expandPolarBeltPath(path, bridges)
		}
		row.sprayStub = expandPolarBeltPath(row.sprayStub, bridges)
		for _, route := range row.outputs {
			updateRoute(route)
		}
	}
	for _, route := range b.denseStreetRoutes {
		updateRoute(route)
	}
	for _, endpoint := range b.flyEndpoints {
		endpoint.belts = expandPolarBeltPath(endpoint.belts, bridges)
	}
	for i := range b.stationLinks {
		b.stationLinks[i].path = expandPolarBeltPath(b.stationLinks[i].path, bridges)
	}
	if plan := b.denseMaterialNetworks; plan != nil {
		for _, routes := range [][]*transportRoute{plan.canonical, plan.physical, plan.joins} {
			for _, route := range routes {
				updateRoute(route)
			}
		}
		for _, group := range plan.groups {
			group.intake = expandPolarBeltPath(group.intake, bridges)
			updateRoute(group.intakeRoute)
			for i := range group.tracks {
				group.tracks[i] = expandPolarBeltPath(group.tracks[i], bridges)
			}
			for i, p := range group.predecessors {
				if bridge, ok := bridges[p]; ok && bridge.target == group.entries[i] {
					group.predecessors[i] = bridge.belts[len(bridge.belts)-1]
				}
			}
			for _, tap := range group.taps {
				tap.mainPrefix = expandPolarBeltPath(tap.mainPrefix, bridges)
				updateRoute(tap.route)
			}
		}
	}
	return nil
}
