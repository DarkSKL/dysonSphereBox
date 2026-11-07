package build

// 文件职责：在所有移动、裁剪、补电和货物重绑完成后，重新证明每台生产设备的真实物料接入与容量。
// 使用：编码前共同出口；边界：早期局部检查和物流端口存在都不能替代最终逐机证明，不改动被审计布局。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
)

// validateFinalMachineIntegrity 职责：按可信产线核对最终机器并重建每个存活物料带、臂和堆叠的能力；使用：cargo重绑后；边界：元数据丢失、重复归属、缺臂或断带均硬失败。
func (b *builder) validateFinalMachineIntegrity(ctx context.Context, req Request, dataset *quant.Dataset) error {
	alive := map[*placed]bool{}
	actual := make([]int, len(req.Plan.Productions))
	incoming := map[*placed][]*placed{}
	for _, entity := range b.all {
		alive[entity] = true
		if entity.output != nil {
			incoming[entity.output] = append(incoming[entity.output], entity)
		}
		if entity.record.RecipeID <= 0 {
			continue
		}
		if entity.owner < 0 || entity.owner >= len(actual) {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终生产机器缺少可信产线归属"}
		}
		production := req.Plan.Productions[entity.owner]
		if int(entity.record.ItemID) != production.MachineID || int(entity.record.RecipeID) != production.RecipeID {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终机器原型或配方与可信计划不符"}
		}
		actual[entity.owner]++
	}
	for owner, count := range actual {
		if count != req.Plan.Productions[owner].MachineCount {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("产线%d最终机器数量%d与计划%d不符", owner, count, req.Plan.Productions[owner].MachineCount)}
		}
	}
	claimed := map[*placed]bool{}
	materialGraph := finalMachineMaterialGraph(b.all)
	for _, registered := range b.machineLines {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终机器物料核验已取消或超时"}
		}
		if registered == nil || len(registered.machines) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终产线缺少实际机器登记"}
		}
		owner := registered.machines[0].owner
		if owner < 0 || owner >= len(req.Plan.Productions) {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终产线登记与计划不匹配"}
		}
		view := *registered
		view.info = req.Plan.Productions[owner]
		recipe, err := datasetRecipeForProduction(dataset, view.info)
		if err != nil {
			return err
		}
		view.recipe = recipe
		view.all = make([]*placed, 0, len(registered.all))
		for _, entity := range registered.all {
			if alive[entity] {
				view.all = append(view.all, entity)
			}
		}
		for _, machine := range view.machines {
			if !alive[machine] || claimed[machine] || machine.owner != owner {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终机器已丢失、重复登记或跨产线归属"}
			}
			claimed[machine] = true
		}
		filter := func(rows [][]*placed) [][]*placed {
			result := make([][]*placed, len(rows))
			for i, row := range rows {
				for _, belt := range row {
					if alive[belt] {
						result[i] = append(result[i], belt)
					}
				}
			}
			return result
		}
		view.inputs, view.outputs = filter(view.inputs), filter(view.outputRows())
		if len(view.outputs) > 0 {
			view.output = view.outputs[0]
		}
		if err := validateLineMachineInserters(&view, b.setting); err != nil {
			return fmt.Errorf("产线%d最终机器端口核验：%w", owner, err)
		}
		if err := validateLineSprayOrder(&view); err != nil {
			return err
		}
		for direction, rows := range [][][]*placed{view.inputs, view.outputs} {
			for index, row := range rows {
				if err := validateFinalMachineTrack(row, incoming); err != nil {
					return fmt.Errorf("产线%d最终物料带：%w", owner, err)
				}
				entry, exit := finalMachineTrackEnds(row, incoming)
				head, item := entry, 0
				if direction == 0 {
					item = view.recipe.Items[index]
				} else {
					head, item = exit, view.recipe.Results[index]
				}
				if err := b.validateFinalLineBoundary(registered, head, item, direction == 0, req.Mode, materialGraph); err != nil {
					return err
				}
			}
		}
		if err := validateFinalLineCoaters(&view, b.all, materialGraph); err != nil {
			return err
		}
	}
	for _, entity := range b.all {
		if entity.record.RecipeID > 0 && !claimed[entity] {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终生产机器没有逐机物料和能力证明"}
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-machine-material-access", Status: "passed", Message: "在最终移动、裁剪和货物重绑后逐台复核数量、配方、真实槽位、全部独立物料带、有向连通、堆叠分拣能力及喷涂先后；不复用早期容量摘要"})
	}
	return nil
}

// validateFinalMachineTrack 职责：证明一条已声明物料带的全部节点仍构成同一条有向链；使用：最终逐台端口证明；边界：不用同位置节点补链，分支入口或环路不冒充独立通道。
func validateFinalMachineTrack(row []*placed, incoming map[*placed][]*placed) error {
	fail := func(reason string) error { return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: reason} }
	if len(row) == 0 {
		return fail("缺少存活的物料带")
	}
	members := map[*placed]bool{}
	for _, node := range row {
		if !surfaceIsBelt(node.record.ItemID) || members[node] {
			return fail("物料带含非带或重复节点")
		}
		members[node] = true
	}
	var entry *placed
	for _, node := range row {
		previous := 0
		for _, before := range incoming[node] {
			if members[before] {
				previous++
			}
		}
		if previous > 1 {
			return fail("独立物料带内部出现未声明汇流")
		}
		if previous == 0 {
			if entry != nil {
				return fail("物料带内部断开，出现多个入口")
			}
			entry = node
		}
	}
	if entry == nil {
		return fail("物料带内部出现未声明循环")
	}
	visited := map[*placed]bool{}
	for node := entry; members[node] && !visited[node]; node = node.output {
		visited[node] = true
	}
	if len(visited) != len(members) {
		return fail("物料带不能有向到达全部机器接点")
	}
	return nil
}
