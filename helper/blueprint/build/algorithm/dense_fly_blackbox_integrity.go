package build

// 文件职责：证明配送黑盒的每条外部份额都接到单物料真实箱体，而内部份额保留完整直连证据。
// 使用：公共机器、碰撞与记录规则之后；边界：不模拟配送机器人飞行吞吐，库存存在不能代替真实箱臂或有向物料链。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// finalizeDenseFlyBlackbox 职责：核验所有边界箱体、配送器、并联臂及所分配的净流；使用：候选适配器收尾；边界：不重新生成实体或放宽普通配送完整性。
func finalizeDenseFlyBlackbox(ctx context.Context, b *builder, _ []blueprint.Building) error {
	fail := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "配送黑盒边界不完整：" + reason}
	}
	if b.denseFlyBlackbox == nil || b.directBoundaryUnits == nil || b.adapter.mode != 2 {
		return fail("缺少专属规划状态")
	}
	alive, claimed, assigned := map[*placed]bool{}, map[*placed]bool{}, map[*transportRoute]bool{}
	for _, entity := range b.all {
		alive[entity] = true
	}
	stores := map[*denseFlyBoundaryStore]bool{}
	usedSlots := map[*placed]map[int8]bool{}
	observedPorts := map[*denseFlyBoundaryStore]int{}
	for _, store := range b.denseFlyBlackbox.stores {
		if store == nil || stores[store] || !alive[store.box] || !alive[store.delivery] || claimed[store.box] || claimed[store.delivery] || len(store.ports) == 0 || len(store.ports) > 2 {
			return fail("真实库存、配送器或双面接口登记重复/缺失")
		}
		stores[store], claimed[store.box], claimed[store.delivery] = true, true, true
		usedSlots[store.box] = map[int8]bool{}
	}
	graph := finalMachineMaterialGraph(b.all)
	for _, facility := range b.denseFlyBlackbox.facilities {
		if err := ctx.Err(); err != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送黑盒边界核验已取消"}
		}
		endpoint := facility.endpoint
		if endpoint == nil || !stores[facility.store] || endpoint.box != facility.store.box || endpoint.delivery != facility.store.delivery || endpoint.item != facility.store.item || endpoint.input != facility.store.input || !alive[endpoint.link] {
			return fail("箱体、配送器或物料头缺失或重复占用")
		}
		registered := 0
		for _, port := range facility.store.ports {
			if port == facility {
				registered++
			}
		}
		if registered != 1 {
			return fail("物料带没有在所属真实库存登记唯一接口")
		}
		observedPorts[facility.store]++
		role := int32(1)
		if endpoint.input {
			role = 2
		}
		delivery, box := endpoint.delivery, endpoint.box
		if box.record.ItemID != 2101 || delivery.record.ItemID != 2107 || delivery.input != box || delivery.record.InputFromSlot != 13 || delivery.record.InputToSlot != 0 || int(delivery.record.FilterID) != endpoint.item || len(delivery.record.Parameters) < 2 || delivery.record.Parameters[0] != 2 || delivery.record.Parameters[1] != role || !flySamePoint(delivery.a, vector{box.a.x, box.a.y, box.a.z + 1.875}) {
			return fail("配送器物料、供需角色或箱顶真实挂接槽错误")
		}
		belts := map[*placed]bool{}
		previous := endpoint.link
		for _, belt := range endpoint.belts {
			if !alive[belt] || !surfaceIsBelt(belt.record.ItemID) || int(belt.record.ItemID) != 2000+b.setting.BeltLv || belts[belt] || belt == endpoint.link {
				return fail("箱体尾带不是所选等级的唯一存活带链")
			}
			if endpoint.input && (belt.output != previous || belt.record.OutputToSlot != 1) || !endpoint.input && (previous.output != belt || previous.record.OutputToSlot != 1) {
				return fail("箱体尾带方向错误或中断")
			}
			belts[belt], previous = true, belt
		}
		if len(belts) == 0 {
			return fail("箱体没有真实接驳尾带")
		}
		if err := b.validateFlyFacilityLink(endpoint, 2, belts); err != nil {
			return err
		}
		capacity := 0.
		for _, arm := range endpoint.arms {
			if !alive[arm] || claimed[arm] || int(arm.record.ItemID) != 2010+b.setting.InserterLv {
				return fail("真实箱臂缺失、重复或擅自升级")
			}
			claimed[arm] = true
			slot := arm.record.OutputToSlot
			if endpoint.input {
				slot = arm.record.InputFromSlot
			}
			if usedSlots[box][slot] {
				return fail("同一箱体的不同物料带重复使用同一个真实臂槽")
			}
			usedSlots[box][slot] = true
			part, err := flyEntityInserterCapacity(arm)
			if err != nil {
				return err
			}
			capacity += part
		}
		total := 0.
		for _, route := range facility.routes {
			if route == nil || assigned[route] || route.boundary != delivery || route.edge.ItemID != endpoint.item || !finitePositive(route.rate) || len(route.path) < 2 || route.path[len(route.path)-1] != route.end {
				return fail("规范边界份额重复、物品不符或路线没有抵达真实终点")
			}
			assigned[route] = true
			total += route.rate
			pathSeen := map[*placed]bool{}
			for i, node := range route.path {
				if !alive[node] || !surfaceIsBelt(node.record.ItemID) || pathSeen[node] {
					return fail("边界路线包含失效或重复的真实带节点")
				}
				pathSeen[node] = true
				if i > 0 && (route.path[i-1].output != node || route.path[i-1].record.OutputToSlot != 1) {
					return fail("边界路线的有向带链中断")
				}
			}
			if endpoint.input {
				if route.from != nil || route.to == nil || !finalMaterialReaches(graph, endpoint.link, route.start, endpoint.item) {
					return fail("输入箱未通过真实单物料带或过滤支路供应指定路线")
				}
				if len(facility.routes) == 1 {
					if route.start != endpoint.link {
						return fail("单去向输入没有使用实际箱体出料头")
					}
				} else {
					branchCapacity := 0.
					for _, arm := range route.filters {
						if !alive[arm] || claimed[arm] || arm.input == nil || arm.output == nil || int(arm.record.ItemID) != 2010+b.setting.InserterLv || int(arm.record.FilterID) != endpoint.item || arm.record.InputFromSlot != -1 || arm.record.OutputToSlot != -1 || !flyBeltReaches(endpoint.link, arm.input) || !flyBeltReaches(arm.output, route.start) {
							return fail("输入分流缺少对应物料、方向和唯一真实过滤臂")
						}
						claimed[arm] = true
						if err := flyInserterSpanMatchesGeometry(arm); err != nil {
							return fail(err.Error())
						}
						part, err := flyEntityInserterCapacity(arm)
						if err != nil {
							return err
						}
						branchCapacity += part
					}
					if branchCapacity+1e-6 < route.rate {
						return fail("输入分流的所选等级真实过滤臂能力不足")
					}
				}
			} else if route.from == nil || route.to != nil || route.end != endpoint.link {
				return fail("目标或副产物没有实际进入所登记出货箱")
			}
		}
		if math.Abs(total-endpoint.rate) > 1e-6 || total > capacity+1e-6 || total > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("配送边界物品%d合计%.6f/min，实际箱臂%.6f/min、所选单带%.6f/min；不能只检验每条小份额", endpoint.item, total, capacity, b.logistics.BeltCapacityPerMinute)}
		}
	}
	for _, store := range b.denseFlyBlackbox.stores {
		if observedPorts[store] != len(store.ports) {
			return fail("真实库存存在未核验的物料接口")
		}
	}
	internal, boundary := 0, 0
	for _, route := range b.transportRoutes {
		if route.from != nil && route.to != nil {
			if assigned[route] || route.boundary != nil {
				return fail("内部物料被误登记为外部配送")
			}
			internal++
		} else {
			if !assigned[route] {
				return fail("存在未接入真实箱体配送器的外部边界")
			}
			boundary++
		}
	}
	for _, entity := range b.all {
		if (entity.record.ItemID == 2101 || entity.record.ItemID == 2107) && !claimed[entity] {
			return fail("存在未登记用途的额外箱体或配送器")
		}
	}
	b.logistics.Checks = append(b.logistics.Checks,
		LogisticsCheck{RuleID: "dense-fly-blackbox-boundaries", Status: "passed", Message: fmt.Sprintf("%d条内部份额通过真实传送带连接；%d条外部份额由%d组单物料箱体/配送器、%d个独立物料带接口承担，逐接口聚合全部净流、逐箱排除重复槽位并核对所选等级臂及带宽；未用一个配送器输送多种物料", internal, boundary, len(b.denseFlyBlackbox.stores), len(b.denseFlyBlackbox.facilities))},
		LogisticsCheck{RuleID: "fly-runtime-throughput", Status: "unverified", Message: "边界箱臂与带路已按计划流量证明；配送机器人数量、运输距离及科技决定实际机队吞吐，不能从箱体库存或带宽推定"})
	return nil
}
