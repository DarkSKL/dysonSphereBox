package build

// 文件职责：把连续低轨机器街道接入mode1完整物料、供电及最终编码流程。
// 使用：独立超密度候选；边界：单产物单去向的参考核心与真实喷涂/监测附件独立放置，其它产线继续使用同密度全场装配。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// denseConnectedStreetWork 职责：关联独立街道附件、原容量行及辅助路线；使用：接线后一次性提交完整物料链；边界：所有引用都指向主builder真实实体。
type denseConnectedStreetWork struct {
	assembly  *denseStreetAssembly
	rows      []*transportRow
	routes    []*transportRoute
	feedbacks []*denseConnectedStreetFeedback
}

// denseConnectedStreetRowsEligible 职责：统一判断一个owner能否以完整单去向街道接入；使用：装配与确切重复候选归一；边界：按实际规范路线判断，不只看配方标签。
func denseConnectedStreetRowsEligible(rows []*transportRow, setting Setting) bool {
	if len(rows) == 0 {
		return false
	}
	first := rows[0]
	if first.info.MachineCount < 2 || len(first.recipe.Results) != 1 || recipeHasMachineRecycle(first.recipe) || !referenceLowRecipe(first.info, first.recipe) || !referenceSingleArmsSufficient(first.info, first.recipe, setting.InserterLv) {
		return false
	}
	for _, row := range rows {
		if len(row.outputs) != 1 || row.outputs[0].edge.ItemID != row.recipe.Results[0] {
			return false
		}
	}
	return true
}

// denseConnectedStreetPlanKeys 职责：从同一资格判断和真实容量counts生成每街道组数的规范键；使用：去除完全相同的显式街道计划；边界：auto只在单组时折叠，键不证明几何成功，空键表示没有适配owner。
func (b *builder) denseConnectedStreetPlanKeys(ctx context.Context, req Request, dataset *quant.Dataset) (map[int]string, error) {
	probe, err := b.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	probe.denseSpatial, probe.denseMaxSpan = true, 0
	probe.flowGraph = b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	rows, err := probe.planTransportRows(ctx, req, dataset)
	if err != nil {
		return nil, err
	}
	order := []int{}
	byOwner := map[int][]*transportRow{}
	for _, row := range rows {
		if len(byOwner[row.owner]) == 0 {
			order = append(order, row.owner)
		}
		byOwner[row.owner] = append(byOwner[row.owner], row)
	}
	keys := map[int]string{}
	for _, limit := range []int{0, 2, -1} {
		var key strings.Builder
		for _, owner := range order {
			group := byOwner[owner]
			if !denseConnectedStreetRowsEligible(group, b.setting) {
				continue
			}
			counts := make([]int, len(group))
			for i, row := range group {
				counts[i] = row.count
			}
			effective := "auto"
			if limit != 0 || len(group) == 1 {
				size := min(len(group), limit)
				if limit <= 0 {
					size = len(group)
				}
				effective = fmt.Sprintf("%d", size)
			}
			fmt.Fprintf(&key, "owner%d:counts%v:perStreet%s;", owner, counts, effective)
		}
		keys[limit] = key.String()
	}
	return keys, nil
}

// denseConnectedStreetRequestEligible 职责：避免为明显无适配产线的请求创建街道候选；使用：候选预算；边界：真正容量、配方和去向仍由完整行规划后检查。
func denseConnectedStreetRequestEligible(req Request) bool {
	if req.Density != densityDense || req.Mode != 1 {
		return false
	}
	for _, p := range req.Plan.Productions {
		if p.MachineCount > 1 && (p.MachineType == 1 || p.MachineType == 2 || p.MachineType == 4) {
			return true
		}
	}
	return false
}

// prepareDenseConnectedStreets 职责：按owner合并空间街道但保留独立运输容量组；使用：全场落位前；边界：不删除规范边、不把多去向出货冒充单带输出，至少一个真实街道才能成为本类候选。
func (b *builder) prepareDenseConnectedStreets(ctx context.Context, rows []*transportRow, needPower bool) ([]denseSpatialUnit, map[int]bool, error) {
	byOwner := map[int][]*transportRow{}
	order := []int{}
	for _, row := range rows {
		if len(byOwner[row.owner]) == 0 {
			order = append(order, row.owner)
		}
		byOwner[row.owner] = append(byOwner[row.owner], row)
	}
	units := []denseSpatialUnit{}
	handled := map[int]bool{}
	groups, machines, streets, before, after := 0, 0, 0, 0, 0
	maxGap := 0.
	for _, owner := range order {
		ownerRows := byOwner[owner]
		first := ownerRows[0]
		if !denseConnectedStreetRowsEligible(ownerRows, b.setting) {
			continue
		}
		counts := make([]int, len(ownerRows))
		for i, row := range ownerRows {
			counts[i] = row.count
		}
		b.owner = owner
		assembly, err := b.buildDenseStreetAccessories(ctx, first.info, first.recipe, counts)
		if err != nil {
			return nil, nil, err
		}
		feedbacks, err := b.prepareDenseConnectedStreetFeedback(assembly, ownerRows)
		if err != nil {
			return nil, nil, err
		}
		street := assembly.street
		corePoles, err := b.reserveDenseConnectedStreetPower(ctx, assembly, needPower)
		if err != nil {
			return nil, nil, err
		}
		b.all = append(b.all, assembly.all...)
		b.all = append(b.all, corePoles...)
		b.machineLines = append(b.machineLines, street.groups...)
		b.logistics.Checks = append(b.logistics.Checks, street.checks...)
		for group, row := range ownerRows {
			row.line = street.groups[group]
			for _, seam := range street.seams {
				if seam.group == group && seam.rotated {
					row.quarterTurns = 2
				}
				maxGap = math.Max(maxGap, seam.gap)
			}
		}
		for _, port := range assembly.ports {
			row := ownerRows[port.group]
			if port.input {
				row.inputs[port.item] = port.head
				// 保留真实低轨头作为连接证据，不另加旧地面长stub。
				row.inputStubs[port.item] = []*placed{port.head}
				continue
			}
			route := row.outputs[0]
			route.start, route.path = port.head, []*placed{port.head}
			route.sharedPath = append([]*placed{}, row.line.outputForItem(port.item)[1:]...)
			port.head.record.Parameters = []int32{int32(port.item), int32(math.Ceil(route.rate))}
			if route.to == nil {
				route.boundary = port.head
			}
		}
		for _, port := range assembly.sprayPorts {
			row := ownerRows[port.group]
			row.bindSprayInput(port.item, port.head)
			row.sprayStub = []*placed{port.head}
			if productionRecipeInputCount(row.recipe, port.item) == 0 {
				row.inputStubs[port.item] = row.sprayStub
			}
		}
		units = append(units, denseSpatialUnit{entities: append(append([]*placed{}, street.all...), corePoles...), rows: ownerRows, ports: street.ports})
		for _, module := range assembly.accessories {
			units = append(units, denseSpatialUnit{entities: module.entities})
		}
		work := &denseConnectedStreetWork{assembly: assembly, rows: ownerRows, feedbacks: feedbacks}
		for index, wire := range assembly.links {
			if !finitePositive(wire.requiredRate) || wire.requiredRate > b.logistics.BeltCapacityPerMinute+1e-6 {
				return nil, nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "街道附件内部线超过当前等级的真实单带容量"}
			}
			row := ownerRows[wire.group]
			route := &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("street-accessory:%d:%d:%d", owner, wire.group, index), ItemID: wire.item, FlowKind: "street-accessory"}, rate: wire.requiredRate, from: row, to: row, start: wire.start, end: wire.end, path: []*placed{wire.start}}
			work.routes = append(work.routes, route)
			b.denseStreetRoutes = append(b.denseStreetRoutes, route)
		}
		b.denseConnectedStreets = append(b.denseConnectedStreets, work)
		handled[owner] = true
		groups, machines, streets = groups+len(ownerRows), machines+first.info.MachineCount, streets+street.streetCount
		before, after = before+street.beltNodesBefore, after+street.beltNodesAfter
	}
	if len(handled) == 0 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: "本候选没有至少两台机器且单产物单去向的参考街道"}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-connected-street", Status: "passed", Message: fmt.Sprintf("连续街道接入%d/%d条产线、%d台机器、%d个独立容量组，折排%d条双排街道；最大已验证接续余量%.3f格，核心带%d→%d；低轨头直接参与真实portal接线，组内原臂及同物料采样保留，仍须完整供电、cargo重绑和终检；不支持的产线使用同密度实体排布", len(handled), len(order), machines, groups, streets, maxGap, before, after)})
	return units, handled, nil
}

// reserveDenseConnectedStreetPower 职责：为可能被独立喷涂附件围住的宽机器核心先放置真实共享电塔；使用：全场装箱前；边界：仅启用供电且核心两轴均超过真实覆盖半径时使用，不放宽覆盖或制造虚拟禁区。
func (b *builder) reserveDenseConnectedStreetPower(ctx context.Context, assembly *denseStreetAssembly, enabled bool) ([]*placed, error) {
	if !enabled || len(assembly.accessories) == 0 {
		return nil, nil
	}
	radius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return nil, err
	}
	low, high := bounds(assembly.street.all)
	if math.Min(high.x-low.x, high.y-low.y)*planarGridLength <= radius {
		return nil, nil
	}
	core := map[*placed]bool{}
	for _, entity := range assembly.street.all {
		core[entity] = true
	}
	var lastError error
	for step := 0; step <= 12; step++ {
		if step > 0 {
			if assembly.street.streetCount <= 1 {
				break
			}
			// 只有真正无塔位时，按八分之一格增大折排接缝；同一街道内部的机器周期、臂和低轨保持刚性。
			for index := range assembly.street.seams {
				seam := &assembly.street.seams[index]
				if seam.street == 0 {
					continue
				}
				entities := []*placed{}
				for _, entity := range assembly.street.groups[seam.group].all {
					if core[entity] {
						entities = append(entities, entity)
					}
				}
				delta := float64(seam.street) / 8
				shift(entities, 0, delta, 0)
				seam.baseY += delta
			}
		}
		local, err := b.newCandidateBuilder()
		if err != nil {
			return nil, err
		}
		active := true
		local.setting.NeedPower = &active
		local.preview = nil
		local.generationCtx = ctx
		local.connectedGlobalPower, local.denseSpatial = true, true
		local.all = append([]*placed{}, assembly.street.all...)
		before := len(local.all)
		if err := local.completeConnectedPower(ctx); err != nil {
			var failure *blueprint.Error
			if !errors.As(err, &failure) || failure.Code != "GEN_E_POWER_COVERAGE" {
				return nil, err
			}
			lastError = err
			continue
		}
		poles := append([]*placed{}, local.all[before:]...)
		if len(poles) > 0 {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-street-core-power", Status: "passed", Message: fmt.Sprintf("宽街道在附件填充前预置%d座真实共享电塔，折排接缝额外预留%.3f格；仅在实际无塔位时增加接缝，不改街道内部周期。附件供电、球面间距和最终覆盖仍统一复核", len(poles), float64(step)/8)})
		}
		return poles, nil
	}
	return nil, lastError
}

// commitDenseConnectedStreets 职责：在真实辅助线完成后提交附件和最终出货干线；使用：mode1裁剪及供电前；边界：未接上的喷涂或监测宿主不能通过构造期检查冒充完整结果。
func (b *builder) commitDenseConnectedStreets() error {
	for _, work := range b.denseConnectedStreets {
		for index, route := range work.routes {
			work.assembly.links[index].path = append([]*placed{}, route.path...)
		}
		if err := b.commitDenseStreetAccessories(work.assembly); err != nil {
			return err
		}
		for _, feedback := range work.feedbacks {
			path := append(append([]*placed{}, feedback.prefix...), feedback.wire.path[1:]...)
			feedback.row.selfSpray = &connectedSelfSpray{row: feedback.row, item: feedback.wire.item, rate: feedback.wire.requiredRate, source: feedback.source, arm: feedback.arm, path: path}
		}
		for _, row := range work.rows {
			for _, route := range row.outputs {
				output := row.line.outputForItem(route.edge.ItemID)
				if len(output) == 0 || output[0] != route.start {
					return transportAllocationError(route.edge, route.rate, "街道附件完成后真实出货头不一致")
				}
				route.sharedPath = append([]*placed{}, output[1:]...)
				if err := b.validateDenseDirectOutput(route); err != nil {
					return err
				}
			}
		}
	}
	if len(b.denseConnectedStreets) > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-street-accessories", Status: "passed", Message: fmt.Sprintf("%d套街道核心与独立附件已由%d条真实有向内部线连接；保留可信喷涂参数、供剂毛流、监测宿主和逐机取料顺序，辅助线不重复计入规范净流", len(b.denseConnectedStreets), len(b.denseStreetRoutes))})
	}
	if err := b.commitDenseMaterialNetworks(); err != nil {
		return err
	}
	if b.directBoundaryCommit != nil {
		return b.directBoundaryCommit(b.generationCtx)
	}
	return nil
}

// denseConnectedStreetExtent 职责：从实际核心和独立附件估算近方形搜索尺度；使用：附件完整生成后的全场落位；边界：只改变软布局提示，不沿用已删除地面长头的区块面积，不改变碰撞或候选数量。
func (b *builder) denseConnectedStreetExtent(ctx context.Context, units []denseSpatialUnit, limit float64) (float64, error) {
	area := 0.
	for _, unit := range units {
		contour, err := b.denseSpatialContour(ctx, unit, 0)
		if err != nil {
			return 0, err
		}
		cells := map[[2]int]bool{}
		for _, shape := range contour.shapes {
			if shape.kind == "power" || shape.kind == "port" {
				continue
			}
			for x := int(math.Floor(shape.low[0] / planarGridLength)); x <= int(math.Floor(shape.high[0]/planarGridLength)); x++ {
				for y := int(math.Floor(shape.low[2] / planarGridLength)); y <= int(math.Floor(shape.high[2]/planarGridLength)); y++ {
					cells[[2]int{x, y}] = true
				}
			}
		}
		area += float64(len(cells))
	}
	extent := math.Min(limit, math.Max(20, math.Ceil(math.Sqrt(math.Max(1, area)))))
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-street-actual-extent", Status: "passed", Message: fmt.Sprintf("按已生成核心与独立附件的轮廓投影估算全场搜索尺度%.0f格（旧试装上限%.0f）；含线路占用，只作矩形软目标，不当作机器利用率，仍须完整接线验证", extent, limit)})
	return extent, nil
}
