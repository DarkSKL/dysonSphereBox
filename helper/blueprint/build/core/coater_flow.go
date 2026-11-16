package build

// 文件职责：保证实际进料方向先经过喷涂再到机器，避免只生成喷涂设施但物料未经喷涂。
// 使用：局部机器单元收尾；边界：只处理物料带与喷涂位置，供剂仍由原规范物流图负责。

import "DysonSphereBox/helper/blueprint"

// prepareFlyInletCoaters 职责：把配送反向进料的喷涂机放到新建右侧地面入口；使用：供剂带创建之前；边界：保持原机器和物料带，将新节点写回同一条输入链。
func (b *builder) prepareFlyInletCoaters(l *line) error {
	if len(l.coaters) == 0 {
		return nil
	}
	if len(l.coaters) != len(l.inputs) {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂机与机器输入带数量不一致"}
	}
	for index, row := range l.inputs {
		if err := b.checkGeneration("配送入口喷涂"); err != nil {
			return err
		}
		if len(row) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂输入缺少真实带"}
		}
		end := row[len(row)-1]
		if end.a.z != 0 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "配送喷涂入口尚未降到地面"}
		}
		tail := b.straight(end.a.x+1, end.a.y, 4, 2)
		connect(end, tail[0])
		l.inputs[index] = append(row, tail...)
		coater := l.coaters[index]
		coater.a, coater.b = tail[1].a, tail[1].a
	}
	return nil
}

// validateLineSprayOrder 职责：沿真实输入带方向验证全部机器取料点都在喷涂节点之后；使用：局部实体与连接完成后；边界：同位置的无关喷涂机不算覆盖，缺链、分叉入口或循环明确失败。
func validateLineSprayOrder(l *line) error {
	if len(l.coaters) == 0 {
		return nil
	}
	machines := map[*placed]bool{}
	for _, machine := range l.machines {
		machines[machine] = true
	}
	for _, row := range l.inputs {
		members, indegree, pickups := map[*placed]bool{}, map[*placed]int{}, map[*placed]bool{}
		for _, belt := range row {
			members[belt] = true
		}
		for _, belt := range row {
			if members[belt.output] {
				indegree[belt.output]++
			}
		}
		for _, entity := range l.all {
			if machines[entity.output] && members[entity.input] {
				pickups[entity.input] = true
			}
		}
		var entry *placed
		for _, belt := range row {
			if indegree[belt] == 0 {
				if entry != nil {
					return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "喷涂物料带存在多个未连接入口"}
				}
				entry = belt
			}
		}
		if entry == nil {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "喷涂物料带没有可追踪入口"}
		}
		coated := false
		visited := map[*placed]bool{}
		for node := entry; members[node] && !visited[node]; node = node.output {
			visited[node] = true
			for _, coater := range l.coaters {
				if flySamePoint(node.a, coater.a) {
					coated = true
				}
			}
			if pickups[node] && !coated {
				return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "机器取料点位于喷涂机上游，无法按量化喷涂产能生成"}
			}
		}
		for node := range pickups {
			if !visited[node] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "喷涂入口无法抵达全部机器取料点"}
			}
		}
		if !coated {
			return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "机器输入带没有经过实际喷涂节点"}
		}
	}
	return nil
}
