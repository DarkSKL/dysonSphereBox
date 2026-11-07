package build

// 文件职责：把容量段与机器街道解耦的核心接入配送器模式完整候选。
// 使用：与已有独立设施密铺候选共同比较；边界：原料回收保留既有完整布局，不绕过喷涂、供剂、全场接线、供电或最终验收。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/quant"
)

// denseFlyStreetEligible 职责：确认配送产线具备真实容量组和已支持的附件拓扑；使用：候选准备和实际装配共用；边界：不静默丢弃喷涂、监测器或反馈回路。
func denseFlyStreetEligible(p quant.Production, r quant.Recipe, counts []int, setting Setting) bool {
	return len(counts) >= 1 && p.MachineCount >= 2 && !recipeHasMachineRecycle(r) && referenceLowRecipe(p, r) && referenceSingleArmsSufficient(p, r, setting.InserterLv)
}

// hasDenseFlyStreet 职责：按实际配送容量拆分判断是否值得增加街道候选；使用：避免无适配产线时重复全部搜索；边界：错误由既有完整生成路径报告，不掩盖其失败。
func (b *builder) hasDenseFlyStreet(req Request, dataset *quant.Dataset) bool {
	if req.Mode != 2 || req.Density != densityDense {
		return false
	}
	constrained := b.denseFlyConstrainedOwners()
	for owner, p := range req.Plan.Productions {
		if constrained[owner] {
			continue
		}
		r, err := datasetRecipeForProduction(dataset, p)
		if err != nil {
			continue
		}
		counts, err := denseFlyLineCounts(p, r, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			continue
		}
		if denseFlyStreetEligible(p, r, counts, b.setting) {
			return true
		}
	}
	return false
}

// prepareDenseFlyStreet 职责：创建完整复合核心并登记每个独立容量组；使用：配送箱/臂单独创建之前；边界：空间整体落位，组内机器与运输身份不能丢失。
func (b *builder) prepareDenseFlyStreet(ctx context.Context, p quant.Production, r quant.Recipe, counts []int, owner int) (*denseFlyStreetWork, error) {
	assembly, err := b.buildDenseStreetAccessories(ctx, p, r, counts)
	if err != nil {
		return nil, err
	}
	street := assembly.street
	b.all = append(b.all, assembly.all...)
	b.machineLines = append(b.machineLines, street.groups...)
	rows := make([]*transportRow, len(counts))
	for group, count := range counts {
		rows[group] = &transportRow{owner: owner, count: count, info: p, recipe: r, line: street.groups[group], inputs: map[int]*placed{}}
	}
	maxGap := 0.
	for _, seam := range street.seams {
		if seam.rotated {
			rows[seam.group].quarterTurns = 2
		}
		maxGap = math.Max(maxGap, seam.gap)
	}
	for _, port := range street.ports {
		if port.input {
			rows[port.group].inputs[port.item] = port.head
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, street.checks...)
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-fly-street", Status: "passed", Message: fmt.Sprintf("配送产线%d的%d台机器保留%d个独立容量组，折排%d条连续双排街道；核心带%d→%d，最大已验证接续余量%.3f格。各组配送箱、真实并联臂和原料/产物端点分别全场选址，再经实体portal连接；最终逐机能力、cargo采样、供电与配送守恒仍须共同验收", owner, p.MachineCount, len(counts), street.streetCount, street.beltNodesBefore, street.beltNodesAfter, maxGap)})
	}
	work := &denseFlyStreetWork{assembly: assembly, rows: rows, byEntity: map[*placed]*transportRow{}}
	coreMembers := map[*placed]bool{}
	for _, entity := range street.all {
		coreMembers[entity] = true
	}
	for group, l := range street.groups {
		for _, entity := range l.all {
			if coreMembers[entity] {
				work.byEntity[entity] = rows[group]
			}
		}
	}
	work.units = append(work.units, denseSpatialUnit{entities: street.all, rows: rows, ports: street.ports})
	for _, module := range assembly.accessories {
		row := &transportRow{owner: owner, line: street.groups[module.group], inputs: map[int]*placed{}}
		for _, entity := range module.entities {
			work.byEntity[entity] = row
		}
		ports := []denseStreetPort{{head: module.head, inner: module.inner}, {head: module.join, inner: module.joinInner}}
		if module.supplyHead != nil {
			ports = append(ports, denseStreetPort{head: module.supplyHead, inner: module.supplyHead.output}, denseStreetPort{head: module.supplyTail, inner: module.supplyInner})
		}
		work.units = append(work.units, denseSpatialUnit{entities: module.entities, rows: []*transportRow{row}, ports: ports})
	}
	for index, link := range assembly.links {
		route := &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("dense-fly-accessory:%d:%d", owner, index), ItemID: link.item, FlowKind: "internal"}, rate: link.requiredRate, from: work.byEntity[link.start], to: work.byEntity[link.end], start: link.start, end: link.end, path: []*placed{link.start}}
		route.from.outputs = append(route.from.outputs, route)
		route.to.inputs[link.item] = link.end
		work.routes = append(work.routes, route)
		b.transportRoutes = append(b.transportRoutes, route)
	}
	return work, nil
}

// denseFlyStreetWork 职责：保留附件实际所属集合和待提交内部线；使用：配送设施与附件分别落位；边界：内部线不新增规范配送份额。
type denseFlyStreetWork struct {
	assembly *denseStreetAssembly
	rows     []*transportRow
	byEntity map[*placed]*transportRow
	units    []denseSpatialUnit
	routes   []*transportRoute
}

// trimDenseFlyOrdinaryTails 职责：只裁剪尚未由共享核心证明过的普通产线尾部；使用：附件提交之前；边界：不截断module.core保存的必要采样链。
func (b *builder) trimDenseFlyOrdinaryTails(works []*denseFlyStreetWork) error {
	protected := map[*line]bool{}
	for _, work := range works {
		for _, l := range work.assembly.street.groups {
			protected[l] = true
		}
	}
	all := b.transportRoutes
	selected := []*transportRoute{}
	for _, route := range all {
		if route.from != nil && protected[route.from.line] || route.to != nil && protected[route.to.line] {
			continue
		}
		selected = append(selected, route)
	}
	b.transportRoutes = selected
	err := b.trimConnectedTails()
	b.transportRoutes = all
	return err
}

// completeDenseFlyStreets 职责：在所有真实内部路径形成后提交附件与喷涂证明；使用：规范配送拓扑终检之前；边界：没有实际采用附件时不标记附件街道。
func (b *builder) completeDenseFlyStreets(works []*denseFlyStreetWork) error {
	modules, wires := 0, 0
	for _, work := range works {
		for i, route := range work.routes {
			work.assembly.links[i].path = route.path
			for _, entity := range route.path {
				entity.owner = work.rows[work.assembly.links[i].group].owner
			}
		}
		if err := b.commitDenseStreetAccessories(work.assembly); err != nil {
			return err
		}
		modules += len(work.assembly.accessories)
		wires += len(work.routes)
	}
	if modules > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-fly-street-accessories", Status: "passed", Message: fmt.Sprintf("%d个真实喷涂/监测附件独立选址，%d条内部实体带完成接驳与供剂串接；保持机器喷涂参数、逐组供料/出货和自产剂毛流，最终统一证明实际能力与覆盖", modules, wires)})
	}
	return nil
}

// routeDenseFlyStreets 职责：先完成附件内部真实链再接外围配送设施；使用：减少外围大流量路径抢占核心短通道；边界：保留全部路线与实体，失败仍丢弃整个候选。
func (b *builder) routeDenseFlyStreets(ctx context.Context, works []*denseFlyStreetWork) error {
	all := b.transportRoutes
	defer func() { b.transportRoutes = all }()
	aux := []*transportRoute{}
	members := map[*transportRoute]bool{}
	for _, work := range works {
		for _, route := range work.routes {
			aux = append(aux, route)
			members[route] = true
		}
	}
	if len(aux) > 0 {
		b.transportRoutes = aux
		if err := b.routeTransport(ctx); err != nil {
			return err
		}
	}
	outer := []*transportRoute{}
	for _, route := range all {
		if !members[route] {
			outer = append(outer, route)
		}
	}
	b.transportRoutes = outer
	return b.routeTransport(ctx)
}

// denseFlyAccessoryLength 职责：补计附件自身通道与内部桥接的唯一真实带边；使用：配送局部线路成本；边界：延用局部坐标单位，最终米制总长仍由全部记录独立计算。
func (b *builder) denseFlyAccessoryLength(works []*denseFlyStreetWork) float64 {
	seen := map[*placed]bool{}
	for _, endpoint := range b.flyEndpoints {
		members := map[*placed]bool{endpoint.link: true}
		for _, node := range endpoint.belts {
			members[node] = true
		}
		for node := range members {
			if members[node.output] {
				seen[node] = true
			}
		}
	}
	length := 0.
	add := func(node *placed) {
		if node == nil || seen[node] || !surfaceIsBelt(node.record.ItemID) || node.output == nil || !surfaceIsBelt(node.output.record.ItemID) {
			return
		}
		seen[node] = true
		other := node.output.a
		length += math.Sqrt(math.Pow(node.a.x-other.x, 2) + math.Pow(node.a.y-other.y, 2) + math.Pow(node.a.z-other.z, 2))
	}
	for _, work := range works {
		for _, module := range work.assembly.accessories {
			for _, node := range module.entities {
				add(node)
			}
		}
		for _, route := range work.routes {
			for i := 0; i+1 < len(route.path); i++ {
				add(route.path[i])
			}
		}
	}
	return length
}
