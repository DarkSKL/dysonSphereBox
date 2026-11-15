package build

// 文件职责：提供超密度全场布局的端口旋转与独立单去向出货证明。
// 使用：空间搜索和最终物流核验；边界：不能省略多去向分流或用无过滤混带冒充独立物料。

// denseRotateVector 职责：旋转局部端口方向；使用：全场四向摆放；边界：高度不变。
func denseRotateVector(point vector, turns int) vector {
	for step := 0; step < (turns%4+4)%4; step++ {
		point.x, point.y = point.y, -point.x
	}
	return point
}

// validateDenseDirectOutput 职责：证明无二次过滤支路的输出来自本行独立物料带且只有一个去向；使用：完整物流出口；边界：任一结构不满足仍拒绝，原料与出货能力仍按实际机器臂检查。
func (b *builder) validateDenseDirectOutput(route *transportRoute) error {
	if handled, err := b.validateDenseProducerSplitOutput(route); handled { return err }
	if handled, err := b.validateDenseMaterialDirectOutput(route); handled {
		return err
	}
	fail := func(reason string) error {
		return transportAllocationError(route.edge, route.rate, "独立出货直连无效："+reason)
	}
	if !b.denseSpatial || route.from == nil || route.from.line == nil || len(route.path) == 0 {
		return fail("没有完整密铺来源")
	}
	row := route.from
	if productionSprayItem(row.info) == route.edge.ItemID {
		if err := validateDenseConnectedSelfBranch(row, route.edge.ItemID); err != nil {
			return fail(err.Error())
		}
	}
	for _, recycle := range row.recycles {
		if recycle.item == route.edge.ItemID {
			return fail("回流必须保留专用分支")
		}
	}
	count := 0
	for _, other := range row.outputs {
		if other.edge.ItemID == route.edge.ItemID {
			count++
			if other != route {
				return fail("同一产物存在其它去向")
			}
		}
	}
	belts := row.line.outputForItem(route.edge.ItemID)
	if count != 1 || len(belts) == 0 || belts[0] != route.start || route.path[0] != route.start {
		return fail("未接到实际独立产物带")
	}
	for i := len(belts) - 1; i > 0; i-- {
		if belts[i].output != belts[i-1] {
			return fail("机器出货带内部断开")
		}
	}
	members := map[*placed]bool{}
	for _, belt := range belts {
		members[belt] = true
	}
	machines := map[*placed]bool{}
	for _, machine := range row.line.machines {
		machines[machine] = true
	}
	feeds := 0
	for _, entity := range row.line.all {
		if machines[entity.input] && members[entity.output] {
			feeds++
			if len(row.recipe.Results) > 1 && int(entity.record.FilterID) != route.edge.ItemID {
				return fail("多产物机器端没有明确过滤")
			}
		}
	}
	if feeds == 0 {
		return fail("没有真实机器出货臂")
	}
	return nil
}
