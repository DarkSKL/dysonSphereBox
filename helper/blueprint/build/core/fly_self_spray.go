package build

// 文件职责：把自产增产剂产线被净流抵消的供剂毛流接回本块真实喷涂入口。
// 使用：模式2在配送净出货端点装配后调用；边界：不创建零速率外供端点、不修改规范流或机器数，实体带与过滤臂仍须完整终检。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// flySelfSpray 职责：保留净流抵消后的回流实体证据；使用：本地毛流重算与完整性终检；边界：不持有可独立篡改的缓存流量。
type flySelfSpray struct {
	// line 职责：保留已校验的真实机器行；使用：重新计算毛流；边界：不以独立缓存替代可信配置。
	line *line
	// arm 职责：记录从同一出货箱取料的实体；使用：槽位、过滤与能力核验；边界：不增加规范净供给。
	arm *placed
	// path 职责：保留回到喷涂入口的有向带链；使用：最终成员和连续性核验；边界：不能借远程配送补链。
	path []*placed
}

// flySelfSprayRates 职责：从可信配置重算本行净出货和回流毛输入；使用：装配与最终核验；边界：毛输出必须等于净出货加回流，不把机器取整余量加入计划流。
func flySelfSprayRates(l *line) (net, returned float64, err error) {
	failure := func(reason string) (float64, float64, error) {
		return 0, 0, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "自产增产剂回流毛流无效：" + reason}
	}
	if l == nil || l.coaterInput == nil || len(l.machines) == 0 || l.info.MachineCount < len(l.machines) || !finitePositive(l.info.OutputCount) {
		return failure("缺少真实机器行或喷涂入口")
	}
	item := productionSprayItem(l.info)
	capacity := l.info.Capacity
	if item == 0 || capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || !finitePositive(capacity.OutputPerMinute) {
		return failure("缺少可信配置毛流")
	}
	if productionItemRate(l.info.Inputs, item) != 0 || !finitePositive(productionItemRate(l.info.Outputs, item)) || len(l.outputForItem(item)) == 0 {
		return failure("自产物品必须只有规范净产出与独立产物带")
	}
	if _, err := productionMaterialFlows(l.info, l.recipe); err != nil {
		return 0, 0, err
	}
	share := float64(len(l.machines)) / float64(l.info.MachineCount)
	scale := l.info.OutputCount / capacity.OutputPerMinute * share
	returned = productionItemRate(capacity.GrossInputs, item) * scale
	gross := productionItemRate(capacity.GrossOutputs, item) * scale
	net = flyLineRate(l.info, item, false, len(l.machines))
	if !finitePositive(net) || !finitePositive(returned) || !finitePositive(gross) || math.Abs(gross-net-returned) > machineMaterialRateTolerance(gross-returned, net, gross) {
		return failure("毛输出不等于规范净出货加回流")
	}
	return net, returned, nil
}

// flyLocalEndpointRate 职责：区分规范端点净流与真实本地出货毛流；使用：分拣器能力终检；边界：回流量每次从可信行重算，不能缩小rate绕过闭环负载。
func flyLocalEndpointRate(endpoint *flyTransportEndpoint) (float64, error) {
	if endpoint.selfSpray == nil {
		return endpoint.rate, nil
	}
	net, returned, err := flySelfSprayRates(endpoint.selfSpray.line)
	if err != nil {
		return 0, err
	}
	if endpoint.input || endpoint.item != productionSprayItem(endpoint.selfSpray.line.info) || math.Abs(endpoint.rate-net) > machineMaterialRateTolerance(endpoint.rate, net, net) {
		return 0, flyEndpointFailure(endpoint, "自产供剂端点物品、方向或净流份额不匹配")
	}
	return net + returned, nil
}

// connectFlySelfSpray 职责：从自产剂的真实出货箱东槽抽取毛流并回到本块供剂入口；使用：净供剂输入为零的配送机器行；边界：只接受可信毛流且有界候选全部失败时拒绝生成，启动库存仍需外部预充。
func (b *builder) connectFlySelfSpray(l *line, begin int) error {
	item := productionSprayItem(l.info)
	_, rate, err := flySelfSprayRates(l)
	if err != nil {
		return err
	}
	var endpoint *flyTransportEndpoint
	for _, candidate := range b.flyEndpoints {
		if candidate.owner == b.owner && candidate.item == item && !candidate.input && candidate.box != nil {
			endpoint = candidate
		}
	}
	if !finitePositive(rate) || endpoint == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "零净供剂输入没有同物品自产毛流，拒绝伪造外供或回流"}
	}
	target := l.coaterInput
	branch := vector{endpoint.box.a.x + 2, endpoint.box.a.y, 0}
	points, pathErr := b.flySelfSprayPath(branch, target, begin)
	if pathErr != nil {
		return pathErr
	}
	if err := b.validateFlyPathEndpoints(points, endpoint.box, target, nil, b.all[begin:]); err != nil {
		return err
	}
	belts := b.byPath(points[:len(points)-1])
	filter := b.make(2010+b.setting.InserterLv, 0, 0, 0)
	if err := flyArmGeometry(filter, endpoint.box, belts[0], 4, true); err != nil {
		return err
	}
	filter.record.FilterID = int16(item)
	actual, err := flyEntityInserterCapacity(filter)
	if err != nil {
		return err
	}
	if actual+1e-6 < rate {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "自产增产剂回流超过已实现的单个真实箱体出料臂能力"}
	}
	connect(belts[len(belts)-1], target)
	belts[0].record.Parameters = []int32{int32(item), int32(math.Ceil(rate))}
	endpoint.selfSpray = &flySelfSpray{line: l, arm: filter, path: append(belts, target)}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "fly-self-spray-recycle", Status: "passed", Message: fmt.Sprintf("产线 %d 被净流抵消的物品 %d 供剂毛流 %.6f/min 已从真实出货箱东槽经所选等级过滤臂和连续有向带回到本块喷涂入口；未创建外供端点或改动规范净流，箱体库存与机队运行仍未验证", b.owner, item, rate)})
	}
	b.warnings = append(b.warnings, fmt.Sprintf("产线 %d 的自产增产剂 %d 使用本块出货箱闭合供剂；首次启动须预充启动库存，离线蓝图不验证库存或循环启动过程", b.owner, item))
	return nil
}

// validateFlySelfSpray 职责：核验同箱真实回流、机器来源及本地毛流带宽；使用：配送端点完整性终检；边界：不穿机器或远程配送补链，不把回流臂能力加到串联出货臂组。
func (b *builder) validateFlySelfSpray(endpoint *flyTransportEndpoint, graph *flyLocalGraph) error {
	recycle := endpoint.selfSpray
	if recycle == nil {
		return nil
	}
	rate, err := flyLocalEndpointRate(endpoint)
	if err != nil {
		return err
	}
	beltCapacity, err := transport.BeltCapacityPerMinute(b.setting.BeltLv)
	if err != nil {
		return err
	}
	if rate > beltCapacity+1e-6 {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "自产剂本地净出货加回流超过单带毛流能力"}
	}
	l, arm, path := recycle.line, recycle.arm, recycle.path
	if arm == nil || len(path) < 2 || arm.input != endpoint.box || arm.output != path[0] || int(arm.record.FilterID) != endpoint.item || arm.record.OutputToSlot != -1 || path[len(path)-1] != l.coaterInput || !graph.sprayInputs[endpoint.item][l.coaterInput] {
		return flyEndpointFailure(endpoint, "自产供剂回流箱臂、独立产物过滤或喷涂端点不匹配")
	}
	slot := arm.record.InputFromSlot
	if slot != 4 && (!b.denseSpatial || slot < 9 || slot > 11) {
		return flyEndpointFailure(endpoint, "自产供剂没有使用已核对的真实回料槽")
	}
	for _, inbound := range endpoint.arms {
		if inbound.record.OutputToSlot == slot {
			return flyEndpointFailure(endpoint, "自产供剂回料与入箱臂占用相同槽位")
		}
	}
	point, ok := storageSlotPoint(endpoint.box.a, endpoint.box.record.Yaw, int(slot))
	if !ok || !flySamePoint(arm.a, point) || !flySamePoint(arm.b, path[0].a) {
		return flyEndpointFailure(endpoint, "自产供剂臂没有连接真实箱体槽位与回流带")
	}
	for _, machine := range l.machines {
		if !graph.members[machine] || machine.owner != endpoint.owner {
			return flyEndpointFailure(endpoint, "自产供剂机器不属于本产线")
		}
	}
	output := l.outputForItem(endpoint.item)
	if len(endpoint.filters) == 0 {
		if err := b.validateDenseFlySelfOutput(endpoint, graph, output); err != nil {
			return err
		}
	}
	for _, filter := range endpoint.filters {
		if !flyBeltReaches(output[len(output)-1], filter.input) {
			return flyEndpointFailure(endpoint, "自产供剂过滤臂未从本机器行独立产物带取料")
		}
	}
	for _, entity := range append([]*placed{arm}, path...) {
		if !graph.members[entity] || entity.owner != endpoint.owner {
			return flyEndpointFailure(endpoint, "自产供剂回流实体不属于本产线")
		}
	}
	seen := map[*placed]bool{}
	for i, belt := range path {
		if !surfaceIsBelt(belt.record.ItemID) || seen[belt] || i > 0 && (path[i-1].output != belt || path[i-1].record.OutputToSlot != 1) {
			return flyEndpointFailure(endpoint, "自产供剂回流带有向链中断、错槽或重复")
		}
		seen[belt] = true
	}
	if err := flyInserterSpanMatchesGeometry(arm); err != nil {
		return err
	}
	capacity, err := flyEntityInserterCapacity(arm)
	if err != nil {
		return err
	}
	if capacity+1e-6 < rate-endpoint.rate {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "自产增产剂真实箱体回流臂能力不足"}
	}
	return nil
}

// flySelfSprayPath 职责：在本机器块有限范围中寻找到供剂入口的真实连续带；使用：自产剂过滤回流候选；边界：既有带和臂均占位，不跨其它配送块，取消与4096格单路预算直接失败。
func (b *builder) flySelfSprayPath(start vector, target *placed, begin int) ([]vector, error) {
	ctx := b.generationCtx
	if ctx == nil {
		ctx = context.Background()
	}
	entities := b.all[begin:]
	minimum, maximum := bounds(entities)
	blocked, err := stationBuildingObstacles(ctx, entities)
	if err != nil {
		return nil, err
	}
	for _, entity := range entities {
		if surfaceIsBelt(entity.record.ItemID) {
			blocked[beltCell(entity.a)] = true
		}
		if entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
			for x := math.Floor(math.Min(entity.a.x, entity.b.x)); x <= math.Ceil(math.Max(entity.a.x, entity.b.x)); x++ {
				for y := math.Floor(math.Min(entity.a.y, entity.b.y)); y <= math.Ceil(math.Max(entity.a.y, entity.b.y)); y++ {
					blocked[vector{x, y, math.Round(entity.a.z)}] = true
				}
			}
		}
	}
	if blocked[beltCell(start)] {
		return nil, &blueprint.Error{Code: "GEN_E_ROUTE", Message: "自产增产剂过滤支路起点已占位"}
	}
	env := &transportEnvironment{blocked: blocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(minimum.x) - 8, math.Floor(minimum.y) - 8, 0}, maximum: vector{math.Ceil(maximum.x) + 8, math.Ceil(maximum.y) + 8, transportMaxHeight}}
	points, _ := env.findTransportPath(ctx, start, target.a, nil, target, false)
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "自产增产剂实体回流已取消或超时"}
	}
	if len(points) < 2 || len(points) > 4096 {
		return nil, &blueprint.Error{Code: "GEN_E_ROUTE", Message: "自产增产剂回流没有预算内的实体带路"}
	}
	return points, nil
}
