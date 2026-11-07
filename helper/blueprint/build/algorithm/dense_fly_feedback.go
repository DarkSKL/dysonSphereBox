package build

// 文件职责：核验密铺配送的回收锚点与自产剂独立出货来源。
// 使用：配送反馈物流终检；边界：依据真实成员、独立机器带和有向连接证明来源，不从坐标邻近补证据。

// denseFlyLeftoverAnchor 职责：按真实成员、输入角色和物品保留回收锚点；使用：全场密铺旋转后；边界：存在同物品输入时仍优先要求同物品，不把空间靠近冒充物料循环。
func (b *builder) denseFlyLeftoverAnchor(endpoint *flyTransportEndpoint) (*placed, bool) {
	if endpoint == nil || endpoint.leftoverAnchor == nil {
		return nil, false
	}
	members := map[*placed]bool{}
	for _, group := range b.flyLayoutBlocks {
		contains := false
		for _, entity := range b.all[group.begin:group.end] {
			if entity == endpoint.box {
				contains = true
				break
			}
		}
		if contains {
			for _, entity := range b.all[group.begin:group.end] {
				members[entity] = true
			}
			break
		}
	}
	if !members[endpoint.leftoverAnchor] {
		return nil, false
	}
	matched, eligible, sameItemExists := false, false, false
	for _, candidate := range b.flyEndpoints {
		if !candidate.input || candidate.owner != endpoint.owner || !members[candidate.box] || !members[candidate.link] {
			continue
		}
		if candidate.item == endpoint.item {
			sameItemExists = true
		}
		if candidate.box == endpoint.leftoverAnchor {
			eligible = true
			matched = candidate.item == endpoint.item
		}
	}
	if !eligible || sameItemExists && !matched {
		return nil, false
	}
	return endpoint.leftoverAnchor, matched
}

// validateDenseFlySelfOutput 职责：证明无二次过滤的自产剂来自唯一独立机器出货带；使用：密铺反馈终检；边界：每台机器都须真实供到该带，多产物须机器端过滤，完整到箱链仍由公共配送核验。
func (b *builder) validateDenseFlySelfOutput(endpoint *flyTransportEndpoint, graph *flyLocalGraph, output []*placed) error {
	if !b.denseSpatial || endpoint.selfSpray == nil || len(output) == 0 || endpoint.link != output[0] || !flyBeltReaches(output[len(output)-1], endpoint.link) {
		return flyEndpointFailure(endpoint, "自产剂没有完整独立出货带来源")
	}
	l := endpoint.selfSpray.line
	members, machines := map[*placed]bool{}, map[*placed]bool{}
	for _, belt := range output {
		members[belt] = true
	}
	for _, machine := range l.machines {
		machines[machine] = true
	}
	feeds := map[*placed]bool{}
	for _, entity := range l.all {
		if !members[entity.output] || entity.input == nil || entity.record.ItemID < 2011 || entity.record.ItemID > 2014 {
			continue
		}
		if !machines[entity.input] || !graph.members[entity] || entity.owner != endpoint.owner {
			return flyEndpointFailure(endpoint, "自产剂独立带混入其它机器来源")
		}
		if len(l.recipe.Results) > 1 && int(entity.record.FilterID) != endpoint.item {
			return flyEndpointFailure(endpoint, "自产剂多产物机器端缺少正确过滤")
		}
		feeds[entity.input] = true
	}
	for machine := range machines {
		if !feeds[machine] {
			return flyEndpointFailure(endpoint, "自产剂某台机器没有供到独立出货带")
		}
	}
	return nil
}
