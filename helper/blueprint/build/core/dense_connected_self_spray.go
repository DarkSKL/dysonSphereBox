package build

// 文件职责：为连续街道的自产增产剂增加独立真实出货与过滤回料模块。
// 使用：mode1附件候选；边界：净交付和喷涂毛流分开核算，沿用既有connectedSelfSpray最终容量及连通证明。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseConnectedStreetFeedback 职责：关联回料臂、出货取点与待接喷涂入口；使用：辅助路径完成后建立最终回路证据；边界：prefix包含真实过滤臂落料带及路由起点。
type denseConnectedStreetFeedback struct {
	row         *transportRow
	source, arm *placed
	prefix      []*placed
	wire        *denseStreetAccessoryLink
}

// prepareDenseConnectedStreetFeedback 职责：在独立地面出货模块上放置真实过滤回料臂；使用：自产剂无净外供的街道；边界：不借用低轨远距抓取，不升级设备，不增加规范外供。
func (b *builder) prepareDenseConnectedStreetFeedback(assembly *denseStreetAssembly, rows []*transportRow) ([]*denseConnectedStreetFeedback, error) {
	result := []*denseConnectedStreetFeedback{}
	for group, row := range rows {
		item := productionSprayItem(row.info)
		if item == 0 || productionItemRate(row.info.Inputs, item) > 0 || len(row.recipe.Results) != 1 || row.recipe.Results[0] != item {
			continue
		}
		capacity := row.info.Capacity
		if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || !finitePositive(capacity.OutputPerMinute) {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "街道自产剂回流缺少可信毛流"}
		}
		rate := productionItemRate(capacity.GrossInputs, item) * row.info.OutputCount / capacity.OutputPerMinute * float64(row.count) / float64(row.info.MachineCount)
		if !finitePositive(rate) || rate > b.logistics.BeltCapacityPerMinute+1e-6 {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "街道自产剂回流毛流无效或超过单带容量"}
		}
		var supply *placed
		for _, port := range assembly.sprayPorts {
			if port.group == group && port.item == item {
				supply = port.head
			}
		}
		if supply == nil {
			return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "街道自产剂没有真实喷涂入口"}
		}
		local, err := b.newCandidateBuilder()
		if err != nil {
			return nil, err
		}
		local.owner, local.preview = row.owner, nil
		var module *denseStreetAccessory
		for _, candidate := range assembly.accessories {
			if candidate.group == group && !candidate.input && candidate.item == item {
				module = candidate
			}
		}
		if module == nil {
			material := local.straight(0, 0, 6, 2)
			reverse(material)
			core := assembly.street.groups[group].outputForItem(item)
			module = &denseStreetAccessory{group: group, item: item, input: false, index: 0, material: material, core: append([]*placed{}, core...), head: material[0], inner: material[1], join: material[len(material)-1], joinInner: material[len(material)-2]}
			assembly.accessories = append(assembly.accessories, module)
			_, outputRates, err := machineInserterRates(row.info, row.recipe)
			if err != nil {
				return nil, err
			}
			assembly.links = append(assembly.links, &denseStreetAccessoryLink{group: group, item: item, purpose: "material", start: core[0], end: module.join, requiredRate: outputRates[0] * float64(row.count), accessory: module})
			for i, port := range assembly.ports {
				if port.group == group && !port.input && port.item == item {
					assembly.ports[i].head, assembly.ports[i].inner = module.head, module.inner
				}
			}
		} else {
			for len(module.material) < 6 {
				last := module.material[len(module.material)-1]
				previous := module.material[len(module.material)-2]
				node := local.make(2000+b.setting.BeltLv, last.a.x+last.a.x-previous.a.x, last.a.y+last.a.y-previous.a.y, last.a.z)
				node.record.Yaw, node.record.Yaw2 = last.record.Yaw, last.record.Yaw2
				connect(node, last)
				module.material = append(module.material, node)
			}
			module.join, module.joinInner = module.material[len(module.material)-1], module.material[len(module.material)-2]
			for _, wire := range assembly.links {
				if wire.accessory == module && wire.purpose == "material" {
					wire.end = module.join
				}
			}
		}
		source := module.material[len(module.material)-3]
		direction := vector{module.join.a.x - module.joinInner.a.x, module.join.a.y - module.joinInner.a.y, 0}
		prefix := local.byPath([]vector{{source.a.x - direction.y, source.a.y + direction.x, source.a.z}, {source.a.x - direction.y*2, source.a.y + direction.x*2, source.a.z}})
		arm, err := local.beltTransfer(source, prefix[0], b.setting.InserterLv, item)
		if err != nil {
			return nil, err
		}
		actual, err := flyEntityInserterCapacity(arm)
		if err != nil {
			return nil, err
		}
		if actual+1e-6 < rate {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "街道自产剂真实回料臂不足所需毛流"}
		}
		module.entities = append(module.entities, local.all...)
		assembly.all = append(assembly.all, local.all...)
		assembly.street.groups[group].all = append(assembly.street.groups[group].all, local.all...)
		wire := &denseStreetAccessoryLink{group: group, item: item, purpose: "self-spray", start: prefix[len(prefix)-1], end: supply, requiredRate: rate}
		assembly.links = append(assembly.links, wire)
		result = append(result, &denseConnectedStreetFeedback{row: row, source: source, arm: arm, prefix: prefix, wire: wire})
	}
	return result, nil
}

// validateDenseConnectedSelfBranch 职责：拒绝仅凭密铺标志免去自产剂分流；使用：独立出货证明；边界：这里只核对真实支路，毛流叠加仍由validateConnectedSelfSprays执行。
func validateDenseConnectedSelfBranch(row *transportRow, item int) error {
	r := row.selfSpray
	if r == nil || r.row != row || r.item != item || !finitePositive(r.rate) || r.source == nil || r.arm == nil || len(r.path) < 2 || r.arm.input != r.source || r.arm.output != r.path[0] || int(r.arm.record.FilterID) != item || r.path[len(r.path)-1] != row.line.coaterInput {
		return fmt.Errorf("自产剂必须有真实过滤臂和完整回料分支")
	}
	for i := 1; i < len(r.path); i++ {
		if r.path[i-1].output != r.path[i] {
			return fmt.Errorf("自产剂回料分支存在断链")
		}
	}
	actual, err := flyEntityInserterCapacity(r.arm)
	if err != nil {
		return err
	}
	if actual+1e-6 < r.rate {
		return fmt.Errorf("自产剂回料臂真实能力不足")
	}
	return nil
}
