package build

// 文件职责：为最终机器带侧建立只读物料窗口身份，覆盖参考臂及普通/并联臂。
// 使用：固定offset跨纬验证；边界：窗口scope不改变生成Nearest锚点白名单，不把未知物料轨道当作已证明。

import "DysonSphereBox/helper/blueprint"

// denseMachineCargoScope 职责：保存真实机器带侧、物料链与喷涂宿主；使用：取放样本身份验证；边界：不记录或修改理想世界坐标。
type denseMachineCargoScope struct {
	arm    *placed
	item   int
	output bool
	track  []*placed
	coater *placed
}

// denseMachineCargoScopes 职责：从最终存活行登记推导每根机器臂的实际物料窗口；使用：原生范围证明；边界：参考元数据独立夹具可直接核验，正式机器臂必须拥有已登记物料行。
func (b *builder) denseMachineCargoScopes() ([]denseMachineCargoScope, error) {
	alive := map[*placed]bool{}
	result := []denseMachineCargoScope{}
	known := map[*placed]int{}
	for _, entity := range b.all {
		alive[entity] = true
		if entity.densityCargo.enabled {
			known[entity] = len(result)
			result = append(result, denseMachineCargoScope{arm: entity, output: entity.densityCargo.output, track: entity.densityCargo.track, coater: entity.densityCargo.coater})
		}
	}
	for _, line := range b.machineLines {
		if line == nil {
			continue
		}
		machines := map[*placed]bool{}
		for _, machine := range line.machines {
			if alive[machine] {
				machines[machine] = true
			}
		}
		type rowScope struct {
			item   int
			track  []*placed
			coater *placed
		}
		inputs, outputs := map[*placed]*rowScope{}, map[*placed]*rowScope{}
		for direction, rows := range [][][]*placed{line.inputs, line.outputRows()} {
			for rowIndex, row := range rows {
				scope := &rowScope{}
				items := line.recipe.Items
				if direction == 1 {
					items = line.recipe.Results
				}
				if rowIndex >= len(items) {
					return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "机器物料行缺少可信配方物品身份"}
				}
				scope.item = items[rowIndex]
				members := map[*placed]bool{}
				for _, node := range row {
					if alive[node] && surfaceIsBelt(node.record.ItemID) {
						scope.track = append(scope.track, node)
						members[node] = true
					}
				}
				if direction == 0 {
					for _, coater := range line.coaters {
						host, err := b.denseCargoCoaterHost(coater)
						if err != nil {
							return nil, err
						}
						if members[host] {
							scope.coater = coater
							break
						}
					}
					if len(line.coaters) > 0 && scope.coater == nil {
						return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "机器取货窗口缺少本物料行的真实喷涂宿主"}
					}
				}
				for _, node := range scope.track {
					if direction == 0 {
						inputs[node] = scope
					} else {
						outputs[node] = scope
					}
				}
			}
		}
		for _, arm := range line.all {
			if !alive[arm] || arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
				continue
			}
			output := machines[arm.input]
			input := machines[arm.output]
			if !output && !input {
				continue
			}
			var row *rowScope
			if output && !input {
				row = outputs[arm.output]
			} else if input && !output {
				row = inputs[arm.input]
			}
			if row == nil {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "最终机器臂缺少唯一已登记物料窗口轨道"}
			}
			scope := denseMachineCargoScope{arm: arm, output: output, track: row.track, coater: row.coater, item: row.item}
			if i, exists := known[arm]; exists {
				if result[i].output != output {
					return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "机器带侧方向与参考身份不符"}
				}
				if result[i].coater != nil && result[i].coater != row.coater {
					return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "机器窗口喷涂身份与参考元数据不符"}
				}
				result[i] = scope
			} else {
				known[arm] = len(result)
				result = append(result, scope)
			}
		}
	}
	for _, arm := range b.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
			continue
		}
		machine := arm.input != nil && arm.input.record.RecipeID > 0 || arm.output != nil && arm.output.record.RecipeID > 0
		if _, exists := known[arm]; machine && !exists {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "未登记机器臂不能计入固定offset可用范围"}
		}
	}
	return b.completeCargoWindowScopes(result)
}
