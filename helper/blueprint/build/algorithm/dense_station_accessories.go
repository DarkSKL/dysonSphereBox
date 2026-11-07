package build

// 文件职责：将共享街道附件契约转换为塔模式的独立占位单元、规范塔口和内部真实接驳。
// 使用：喷涂/监测街道完整候选；边界：核心与附件分别移动，内部物料连接不重复计入规范流图。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// registerDenseStationAssembly 职责：唯一提交核心及附件库存，并按规范外口登记塔需求；使用：共享构造成功之后；边界：此时附件到核心尚待真实接线，不能直接发布结果。
func (b *builder) registerDenseStationAssembly(assembly *denseStreetAssembly, production quant.Production, rows *[]*denseStationRow, routes *[]denseStationRoute) error {
	if assembly == nil || assembly.street == nil || len(assembly.street.groups) == 0 {
		return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "塔街道附件缺少真实核心"}
	}
	street := assembly.street
	unit := &denseStationUnit{entities: append([]*placed(nil), street.all...), ports: append([]denseStreetPort(nil), street.ports...), assembly: assembly, links: assembly.links}
	coreMembers := map[*placed]bool{}
	for _, entity := range street.all {
		coreMembers[entity] = true
	}
	b.all = append(b.all, assembly.all...)
	b.machineLines = append(b.machineLines, street.groups...)
	b.logistics.Checks = append(b.logistics.Checks, street.checks...)
	turns := make([]int, len(street.groups))
	for _, seam := range street.seams {
		if seam.rotated {
			turns[seam.group] = 2
		}
	}
	for group, l := range street.groups {
		row := &denseStationRow{line: l, unit: unit, turns: turns[group]}
		for _, entity := range l.all {
			if coreMembers[entity] {
				row.entities = append(row.entities, entity)
			}
		}
		unit.rows = append(unit.rows, row)
		share := float64(len(l.machines)) / float64(production.MachineCount)
		leftovers := b.stationLeftoverKinds(l.machines[0].owner)
		appendRoute := func(port denseStreetPort, spray bool) error {
			if port.head == nil || port.item <= 0 {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "街道附件缺少真实外部物料端口"}
			}
			rate := productionItemRate(production.Outputs, port.item)
			if port.input {
				role := recipeInputRole
				if spray {
					role = sprayInputRole
				}
				var err error
				rate, err = productionRoleInputRate(production, l.recipe, port.item, role)
				if err != nil {
					return err
				}
			}
			route := stationRoute{head: port.head, item: port.item, output: !port.input, rate: rate * share}
			if spray && route.rate == 0 && productionItemRate(production.Outputs, port.item) > 0 {
				route.sprayRecycle = l
			}
			row.routes = append(row.routes, len(*routes))
			*routes = append(*routes, denseStationRoute{stationRoute: route, row: row, leftover: !port.input && leftovers[port.item]})
			return nil
		}
		for _, port := range assembly.ports {
			if port.group == group {
				if err := appendRoute(port, false); err != nil {
					return err
				}
			}
		}
		for _, port := range assembly.sprayPorts {
			if port.group == group {
				if err := appendRoute(port, true); err != nil {
					return err
				}
			}
		}
		*rows = append(*rows, row)
	}
	for _, accessory := range assembly.accessories {
		if accessory == nil || accessory.head == nil || accessory.inner == nil || accessory.join == nil || accessory.joinInner == nil {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "街道附件缺少真实内外端口及相邻节点"}
		}
		module := &denseStationUnit{entities: append([]*placed(nil), accessory.entities...), links: assembly.links}
		module.ports = append(module.ports, denseStreetPort{group: accessory.group, item: accessory.item, input: accessory.input, head: accessory.head, inner: accessory.inner}, denseStreetPort{group: accessory.group, item: accessory.item, input: !accessory.input, head: accessory.join, inner: accessory.joinInner})
		if accessory.supplyHead != nil {
			if accessory.supplyHead.output == nil {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "附件供剂入口缺少真实相邻带节点"}
			}
			module.ports = append(module.ports, denseStreetPort{group: accessory.group, item: productionSprayItem(production), input: true, head: accessory.supplyHead, inner: accessory.supplyHead.output})
		}
		if accessory.supplyTail != nil {
			for _, entity := range accessory.entities {
				if surfaceIsBelt(entity.record.ItemID) && entity.output == accessory.supplyTail {
					module.ports = append(module.ports, denseStreetPort{group: accessory.group, item: productionSprayItem(production), head: accessory.supplyTail, inner: entity})
					break
				}
			}
		}
		unit.accessories = append(unit.accessories, module)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-station-street-core", Status: "passed", Message: fmt.Sprintf("产线%d保持%d台机器的连续相对周期，并将%d个真实附件作为独立轮廓落位；外部塔口与内部接驳随后分别核验", b.owner, production.MachineCount, len(assembly.accessories))})
	return nil
}

// denseStationAssemblies 职责：按核心单元去重共享附件装配；使用：内部路径提交；边界：普通行没有附件装配，不从实体坐标猜测归属。
func denseStationAssemblies(rows []*denseStationRow) []*denseStreetAssembly {
	seen := map[*denseStreetAssembly]bool{}
	assemblies := []*denseStreetAssembly{}
	for _, row := range rows {
		if row.unit == nil || row.unit.assembly == nil || seen[row.unit.assembly] {
			continue
		}
		assemblies = append(assemblies, row.unit.assembly)
		seen[row.unit.assembly] = true
	}
	return assemblies
}

// routeDenseStationAccessories 职责：为所有共享附件契约创建完整有向带并填入path；使用：真实轮廓落位后；边界：路径只属于物料轨，不追加新的规范lane或塔份额，提交由共享helper负责。
func (b *builder) routeDenseStationAccessories(ctx context.Context, env *transportEnvironment, rows []*denseStationRow) error {
	for _, assembly := range denseStationAssemblies(rows) {
		for _, link := range assembly.links {
			if link == nil || link.start == nil || link.end == nil || link.group < 0 || link.group >= len(assembly.street.groups) {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "塔街道附件内部接驳契约无效"}
			}
			if !finitePositive(link.requiredRate) || link.requiredRate > b.logistics.BeltCapacityPerMinute+1e-6 {
				return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "塔街道附件内部毛流超过所选真实单带能力"}
			}
			work := &transportRoute{start: link.start, end: link.end, to: &transportRow{}}
			points, _, expanded, _, _ := env.findDenseTransportPath(ctx, work, nil, nil, vector{x: -1})
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "塔街道附件真实接驳已取消或超时"}
			}
			if len(points) == 0 {
				return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("塔街道附件%s物品%d的%+v至%+v没有真实路径，展开%d", link.purpose, link.item, link.start.a, link.end.a, expanded)}
			}
			b.owner = assembly.street.groups[link.group].machines[0].owner
			middle := b.byPath(points[:len(points)-1])
			path := append([]*placed{link.start}, middle...)
			path = append(path, link.end)
			for i := 1; i < len(path); i++ {
				connect(path[i-1], path[i])
			}
			link.path = path
			for _, node := range middle {
				env.densePorts.addNode(node)
				blockDenseBeltNode(env.blocked, node.a)
				env.beltCorridors[beltColumn(node.a)] = true
			}
			env.version++
		}
	}
	return nil
}
