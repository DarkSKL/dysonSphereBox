package build

// 文件职责：构造内部直连、仅对外边界使用最少共享塔的真实黑盒候选。
// 使用：普通与超密度塔模式；边界：复用可信机器、分流、供剂、回流及最终逐边证明，几何规则由请求密度决定。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// denseStationBlackboxCandidate 职责：把已验证直连核心与实际外部塔组成同一次完整候选；使用：塔模式比较；边界：原请求模式、等级、产量和选区始终保留。
func (b *builder) denseStationBlackboxCandidate(ctx context.Context, req Request, dataset *quant.Dataset, config contract.StationConfig, variant int) (*builder, error) {
	if variant >= 4 {
		ctx = context.WithValue(ctx, denseMaterialNetworksKey{}, true)
	}
	candidate, err := b.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	// 私有组合只复用局部运输能力，不替换公开mode ID或全局适配器。
	adapter := *candidate.adapter
	adapter.lineAssembly = lineAssemblyCapabilities{reverseOutputRows: true, writeInputParameters: true, useLocalLinePower: true}
	adapter.connectedRows = connectedRowsCapabilities{prepareConnectedSelfSpray: true}
	adapter.logistics = logisticsFinalization{validateConnectedRoutes: true, validateSelfSprays: true, validateEdgeConservation: true}
	adapter.finalizeRecords = finalizeDenseStationBlackbox
	adapter.retrySurfaceAnchor = retryConnectedSurfaceAnchor
	candidate.adapter = &adapter
	candidate.flowGraph, candidate.generationCtx = b.flowGraph, ctx
	candidate.denseSpatial, candidate.connectedCompactBelts, candidate.connectedGlobalPower = true, true, true
	candidate.denseSpatialOrder = variant % 4
	candidate.denseMaxSpan = []float64{18, 24, 32, 24}[variant%4]
	needsRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return candidate, err
	}
	if needsRecycle {
		// 双优先门有不可省略的真实长接头；比较较短机器组以形成矩形，塔槽/带口仍按新增实际端口重新核算。
		candidate.denseMaxSpan = []float64{8, 8, 8, 24}[variant%4]
		candidate.denseRefine = variant%2 == 1
	}
	streetSupported := false
	for _, production := range req.Plan.Productions {
		recipe, recipeErr := datasetRecipeForProduction(dataset, production)
		if recipeErr != nil {
			return candidate, recipeErr
		}
		streetSupported = streetSupported || candidate.denseStationStreetEligible(production, recipe)
	}
	candidate.denseStreetCandidate = variant%2 == 1 && streetSupported
	candidate.denseStreetRotation = (variant / 2) % 2
	if variant >= 4 {
		choice := variant - 4
		// 同等窄纬比例按整厂规模调整；227台固定设备不能硬套约90台参考厂的24格绝对高度。
		scale := math.Max(1, math.Sqrt(float64(req.Plan.MachineCount)/96))
		candidate.denseLatitudeSpan = math.Ceil([]float64{24, 32, 40}[choice]*scale/4) * 4
		candidate.denseLatitudeSpan = denseLatitudeHeightLimit(ctx, candidate.denseLatitudeSpan)
		candidate.denseSpatialOrder = 3
		candidate.denseMaxSpan = []float64{12, 18, 32}[choice]
		if scale > 1 {
			candidate.denseMaxSpan = 32
		}
		candidate.denseStreetGroupLimit = -1
		candidate.denseStreetCandidate = streetSupported
		candidate.denseStreetRotation = 0
		candidate.denseUnitOrder = func(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
			return candidate.denseStationMaterialUnitOrder(ctx, units, choice == 1)
		}
		candidate.directBoundaryAttach = candidate.attachDenseStationSprayExits
		candidate.directBoundaryCommit = candidate.commitDenseStationSprayNetwork
	}
	candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	candidate.warnings = append([]string(nil), b.warnings...)
	candidate.directBoundaryUnits = func(ctx context.Context, rows []*transportRow) ([]denseSpatialUnit, error) {
		if variant >= 4 {
			if err := candidate.prepareDenseStationSprayNetwork(ctx); err != nil {
				return nil, err
			}
		}
		return candidate.buildDenseStationBoundaries(ctx, config, variant)
	}
	candidate.beginPreview()
	limits, err := candidate.layoutHeightCandidates(ctx, req, dataset, true)
	if err != nil {
		return candidate, err
	}
	limit := limits[0]
	if len(limits) > 1 && variant >= 2 {
		limit = limits[1]
	}
	err = candidate.assembleConnectedVariant(ctx, req, dataset, limit, true)
	if err == nil && candidate.denseLatitudeSpan > 0 {
		low, high := bounds(candidate.all)
		if math.Ceil(high.y-low.y)+1 > candidate.denseLatitudeSpan+1e-6 {
			err = &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("大厂候选实际工作格纬向%.3f超过声明预算%.3f，不将加高包装成窄纬布局", math.Ceil(high.y-low.y)+1, candidate.denseLatitudeSpan)}
		}
	}
	return candidate, err
}

// buildDenseStationBoundaries 职责：创建真实塔与每条独立边界的短带口，再一起交给全场轮廓装箱；使用：机器行完成后的公共hook；边界：塔间固定相对距离是必要设施约束，空隙不作为独占矩形。
func (b *builder) buildDenseStationBoundaries(ctx context.Context, config contract.StationConfig, variant int) ([]denseSpatialUnit, error) {
	plan, err := planDenseStationBoundaries(ctx, b.transportRoutes, config.ItemSlots, b.logistics.BeltCapacityPerMinute)
	if err != nil {
		return nil, err
	}
	pitch := math.Ceil(math.Sqrt(float64(surfaceStationDistanceLimit(int16(config.StationID), int16(config.StationID))))/planarGridLength) + 1
	columns := max(1, int(math.Ceil(math.Sqrt(float64(len(plan.towers))))))
	if variant%2 == 1 {
		columns = len(plan.towers)
	}
	if height := denseLatitudeHeightLimit(ctx, math.Inf(1)); !math.IsInf(height, 1) {
		maximumRows := max(1, int(math.Floor((height-10)/pitch))+1)
		if maximumRows < len(plan.towers) {
			columns = max(columns, (len(plan.towers)+maximumRows-1)/maximumRows)
		}
	}
	begin := len(b.all)
	b.owner = -1
	ports := []denseStreetPort{}
	for i := range plan.towers {
		stock := &plan.towers[i]
		stock.tower = b.createStation(vector{float64(i%columns)*pitch + 5, float64(i/columns)*pitch + 5, 0}, stock.items, config)
		for slot := 0; slot < 12; slot++ {
			stock.tower.record.Parameters[192+slot*4] = 0
			stock.tower.record.Parameters[193+slot*4] = 0
		}
		for port, index := range stock.routes {
			route := plan.routes[index]
			output := route.to == nil
			// 均匀使用四边以避免所有外部物料堵在单侧；实际路线仍按真实端点求解。
			slot := []int{10, 1, 7, 4, 9, 0, 6, 3, 11, 2, 8, 5}[port]
			points := append([]vector{denseStationOuter(stock.tower.a, slot)}, denseStationInner(stock.tower.a, slot)...)
			belts := b.byPath(points)
			head, last := belts[0], belts[len(belts)-1]
			head.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
			last.record.Parameters = []int32{int32(route.edge.ItemID), 0}
			base := 192 + slot*4
			owner := 0
			if output {
				last.output = stock.tower
				last.record.OutputToSlot = int8(slot)
				stock.tower.record.Parameters[base] = 2
				route.end, route.boundary = head, stock.tower
				owner = route.from.owner
			} else {
				reverse(belts)
				last.input = stock.tower
				last.record.InputFromSlot = int8(slot)
				stock.tower.record.Parameters[base] = 1
				for itemSlot, item := range stock.items {
					if item.id == route.edge.ItemID {
						stock.tower.record.Parameters[base+1] = int32(itemSlot + 1)
						break
					}
				}
				route.start, route.boundary = head, stock.tower
				route.path = []*placed{head}
				owner = route.to.owner
			}
			b.stationLinks = append(b.stationLinks, stationTransportLink{item: route.edge.ItemID, output: output, tower: stock.tower, belt: last, slot: slot, head: head, path: append([]*placed(nil), belts[1:]...), owner: owner, rate: route.rate})
			ports = append(ports, denseStreetPort{item: route.edge.ItemID, input: output, head: head, inner: belts[1]})
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-station-facility-bound", Status: "passed", Message: fmt.Sprintf("仅对外%d类供需库存；按所选带速聚合的物料下界%d塔/至少%d带口，当前%d条独立边界lane下界%d塔，实际%d塔；库存槽%d、每塔12口；当前lane分配最少塔搜索证明=%t。内部中间流不占塔槽，未假设运输站堆叠科技", plan.roleCount, plan.materialLowerBound, plan.minimumPorts, len(plan.routes), plan.laneLowerBound, len(plan.towers), config.ItemSlots, plan.proven)})
	return []denseSpatialUnit{{entities: append([]*placed(nil), b.all[begin:]...), ports: ports}}, nil
}

// finalizeDenseStationBlackbox 职责：证明每条对外lane独占合法真实塔口，而全规范内部路线随后由公共direct终检证明；使用：仅本候选的私有适配器；边界：不以库存配置代替真实带链。
func finalizeDenseStationBlackbox(ctx context.Context, b *builder, records []blueprint.Building) error {
	if err := b.validateDenseStationSprayNetwork(ctx); err != nil {
		return err
	}
	if err := validateStationPorts(ctx, records); err != nil {
		return err
	}
	indices := map[*placed]int{}
	for i, p := range b.all {
		indices[p] = i
	}
	byHead := map[*placed]*stationTransportLink{}
	usedSlots := map[[2]int]bool{}
	for i := range b.stationLinks {
		link := &b.stationLinks[i]
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒塔边界核验已取消"}
		}
		if link.sprayRecycle != nil || !finitePositive(link.rate) || link.rate > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "黑盒边界不是有效独立净流带"}
		}
		for _, p := range append(append([]*placed{}, link.path...), link.head, link.tower, link.belt) {
			if _, ok := indices[p]; !ok {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "黑盒塔引用了最终库存外的带口"}
			}
		}
		key := [2]int{indices[link.tower], link.slot}
		if usedSlots[key] || byHead[link.head] != nil {
			return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "黑盒独立lane重复使用同一塔口"}
		}
		usedSlots[key] = true
		byHead[link.head] = link
		if err := validateStationLink(link, indices, records); err != nil {
			return err
		}
		// 黑盒对外角色同时约束星际塔的本地/星际字段；只核对本地角色会放过反向远程供需。
		if b.adapter.station.Config.DualRoleFields {
			want := int32(2)
			if link.output {
				want = 1
			}
			parameters := records[indices[link.tower]].Parameters
			found := false
			for slot := 0; slot < b.adapter.station.Config.ItemSlots; slot++ {
				if parameters[slot*6] == int32(link.item) {
					found = parameters[slot*6+2] == want
					break
				}
			}
			if !found {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "黑盒星际库存的远程供需角色与实际对外物料不一致"}
			}
		}
	}
	used := map[*stationTransportLink]float64{}
	shared := map[*stationTransportLink]*denseStationSprayGroup{}
	matchedOwner := map[*stationTransportLink]bool{}
	internal := 0
	for _, route := range b.transportRoutes {
		if route.from != nil && route.to != nil {
			internal++
			continue
		}
		output := route.to == nil
		head := route.start
		if output {
			head = route.end
		}
		link := byHead[head]
		if link == nil || route.boundary != link.tower || link.item != route.edge.ItemID || link.output != output {
			return transportAllocationError(route.edge, route.rate, "黑盒对外份额缺少唯一且方向正确的真实塔口")
		}
		owner := route.to
		if output {
			owner = route.from
		}
		if owner == nil {
			return transportAllocationError(route.edge, route.rate, "黑盒外部端口归属与实际机器行不符")
		}
		var group *denseStationSprayGroup
		if b.denseStationSprayNetwork != nil {
			group = b.denseStationSprayNetwork.byRoute[route]
		}
		if group == nil {
			if used[link] > 0 || owner.owner != link.owner || math.Abs(link.rate-route.rate) > 1e-6 {
				return transportAllocationError(route.edge, route.rate, "独立塔带口被重复分配或改变份额")
			}
		} else {
			if group.physical.start != head || group.physical.boundary != link.tower || used[link] > 0 && shared[link] != group || math.Abs(group.physical.rate-link.rate) > 1e-6 {
				return transportAllocationError(route.edge, route.rate, "共享供剂份额未绑定到已验证的同一真实串供口")
			}
			shared[link] = group
		}
		used[link] += route.rate
		matchedOwner[link] = matchedOwner[link] || owner.owner == link.owner
	}
	if len(used) != len(b.stationLinks) {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "黑盒存在未计入规范图的塔端口"}
	}
	for link, rate := range used {
		if !matchedOwner[link] || math.Abs(rate-link.rate) > 1e-6 {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "黑盒实际塔口聚合份额或来源归属不守恒"}
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-station-blackbox", Status: "passed", Message: fmt.Sprintf("%d条内部lane采用真实传送带，%d条外部lane逐一连接真实库存/带口；全部规范边随后统一验证方向、共享带能力和流量守恒", internal, len(used))})
	return nil
}

// denseStationFacilityCount 职责：统计候选实际塔实体；使用：塔模式优先最少设施，再比较矩形及线路；边界：不以计划库存组数冒充已建塔数。
func denseStationFacilityCount(b *builder) int {
	count := 0
	for _, p := range b.all {
		if surfaceIsStation(p.record.ItemID) {
			count++
		}
	}
	return count
}
