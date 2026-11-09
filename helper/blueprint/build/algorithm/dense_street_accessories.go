package build

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/density"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// 文件职责：把真实喷涂机、监测器及其地面宿主与低轨机器核心分离，并在实体接驳后提交完整物料链。
// 使用：各密铺模式的新附件入口；边界：构造阶段不冒充连通工厂，不修改可信喷涂参数、设备等级或供剂需求。

// denseStreetAccessory 职责：保存单物料的独立附件模块；使用：全场独立落位；边界：head/inner为规范外口，join/joinInner为接核心口，不能只移动标记点。
type denseStreetAccessory struct {
	group, item                         int
	input                               bool
	entities                            []*placed
	head, inner, join, joinInner        *placed
	coater                              *placed
	supplyHead, supplyTail, supplyInner *placed
	// material/core 职责：保留规范外口到核心的原始数组顺序；使用：接驳完成后统一重建line；边界：输出数组仍从外部sink起，实际带向相反。
	material, core []*placed
	index          int
}

// denseStreetAccessoryLink 职责：声明必须真实创建的内部有向带；使用：模式接线后填写path；边界：path含start/end，不能把它额外计成规范外部物流边。
type denseStreetAccessoryLink struct {
	group, item  int
	purpose      string
	start, end   *placed
	requiredRate float64
	path         []*placed
	accessory    *denseStreetAccessory
}

// denseStreetAssembly 职责：保存核心、独立附件及待提交内部线；使用：主builder只追加all一次、groups一起登记；边界：street.all仅供刚性核心移动，附件不能重复移动。
type denseStreetAssembly struct {
	street            *denseStreet
	all               []*placed
	accessories       []*denseStreetAccessory
	ports, sprayPorts []denseStreetPort
	links             []*denseStreetAccessoryLink
	committed         bool
}

// buildDenseStreetAccessories 职责：保留真实喷涂/监测配置构建核心和独立附件；使用：无附件时也可裁剪单容量组；边界：至少两台机器，返回时内部链尚待真实接驳。
func (b *builder) buildDenseStreetAccessories(ctx context.Context, p quant.Production, r quant.Recipe, counts []int) (*denseStreetAssembly, error) {
	street, err := b.selectDenseStreet(ctx, p, r, counts, true)
	if err != nil {
		return nil, err
	}
	assembly := &denseStreetAssembly{street: street, all: append([]*placed(nil), street.all...), accessories: street.accessories}
	capacity, err := transport.BeltCapacityPerMinute(b.setting.BeltLv)
	if err != nil {
		return nil, err
	}
	for _, module := range assembly.accessories {
		assembly.all = append(assembly.all, module.entities...)
		l := street.groups[module.group]
		l.all = append(append([]*placed(nil), l.all...), module.entities...)
	}
	for _, port := range street.ports {
		external := port
		for _, module := range assembly.accessories {
			if module.group != port.group || module.item != port.item || module.input != port.input {
				continue
			}
			external.head, external.inner = module.head, module.inner
			wire := &denseStreetAccessoryLink{group: port.group, item: port.item, purpose: "material", requiredRate: port.requiredRate, accessory: module}
			wire.start, wire.end = module.join, port.head
			if !port.input {
				wire.start, wire.end = port.head, module.join
			}
			assembly.links = append(assembly.links, wire)
			break
		}
		assembly.ports = append(assembly.ports, external)
	}
	for group, l := range street.groups {
		var previous *denseStreetAccessory
		spray := productionSprayItem(p)
		rate := 0.
		if p.IncreasePoints > 0 {
			if p.Capacity == nil || p.Capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad {
				return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "街道喷涂附件缺少可信满负荷供剂量"}
			}
			gross, err := configuredRoleInputRate(p, r, spray, sprayInputRole)
			if err != nil {
				return nil, err
			}
			rate = gross * float64(len(l.machines)) / float64(p.MachineCount)
			if !finitePositive(rate) || rate > capacity+1e-6 {
				return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "街道供剂量缺失或超过真实单带能力"}
			}
		}
		for _, module := range assembly.accessories {
			if module.group != group || module.coater == nil {
				continue
			}
			if previous == nil {
				l.coaterInput = module.supplyHead
				assembly.sprayPorts = append(assembly.sprayPorts, denseStreetPort{group: group, item: spray, input: true, head: module.supplyHead, inner: module.supplyHead.output, requiredRate: rate, capacity: capacity})
			} else {
				assembly.links = append(assembly.links, &denseStreetAccessoryLink{group: group, item: spray, purpose: "spray", start: previous.supplyTail, end: module.supplyHead, requiredRate: rate})
			}
			previous = module
		}
	}
	return assembly, nil
}

// detachDenseStreetAccessories 职责：提取真实地面宿主并独立建立每台喷涂机供剂通道；使用：核心压缩前；边界：保留实体/参数和cargo.coater，监测器只迁至模块内真正直线节点。
func (b *builder) detachDenseStreetAccessories(l *line, group int) ([]*denseStreetAccessory, error) {
	removed := map[*placed]bool{}
	modules := []*denseStreetAccessory{}
	for side, rows := range [][][]*placed{l.inputs, l.outputRows()} {
		items := l.recipe.Items
		if side == 1 {
			items = l.recipe.Results
		}
		for index, row := range rows {
			var coater *placed
			if side == 0 && l.info.IncreasePoints > 0 {
				if index >= len(l.coaters) {
					return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "喷涂街道缺少每种实际原料的喷涂机"}
				}
				coater = l.coaters[index]
			}
			monitors := []*placed{}
			for _, entity := range l.all {
				if entity.record.ItemID != 2030 || removed[entity] {
					continue
				}
				for _, node := range row {
					if flySamePoint(entity.a, node.a) {
						monitors = append(monitors, entity)
						break
					}
				}
			}
			if coater == nil && len(monitors) == 0 {
				continue
			}
			if len(row) < 4 || len(monitors) > 1 {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "附件缺少唯一监测器的真实地面直线宿主"}
			}
			if coater != nil && len(monitors) > 0 {
				// 监测器的真实长盒不能封住供剂头的直向入口；增加两个有功能的宿主前缀节点后再独立选址。
				head := row[0]
				prefix := b.byPath([]vector{{head.a.x - 2, head.a.y, 0}, {head.a.x - 1, head.a.y, 0}})
				connect(prefix[len(prefix)-1], head)
				prefix[0].record.Parameters = append([]int32(nil), head.record.Parameters...)
				row = append(prefix, row...)
				l.inputs[index] = row
				l.all = append(append([]*placed(nil), l.all...), prefix...)
			}
			last := 2
			if coater != nil {
				at := -1
				for i, node := range row {
					if flySamePoint(node.a, coater.a) {
						at = i
						break
					}
				}
				if at < 0 {
					return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "真实喷涂机没有对应物料带宿主"}
				}
				last = max(last, at+2)
			}
			for _, monitor := range monitors {
				host := row[1]
				monitor.a, monitor.b = host.a, host.a
				monitor.flowMonitorHost = host
				monitor.record.Yaw, monitor.record.Yaw2 = host.record.Yaw, host.record.Yaw
				last = max(last, 3)
			}
			if last >= len(row) {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "附件真实宿主的两端保护节点不足"}
			}
			prefix := append([]*placed(nil), row[:last+1]...)
			for _, node := range prefix {
				if math.Abs(node.a.z) > 1e-6 || math.Abs(node.a.y-prefix[0].a.y) > 1e-6 {
					return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "附件前缀不是完整地面直线，拒绝裁掉必要转弯或坡道"}
				}
			}
			module := &denseStreetAccessory{group: group, item: items[index], input: side == 0, index: index, head: prefix[0], inner: prefix[1], join: prefix[len(prefix)-1], joinInner: prefix[len(prefix)-2], material: prefix, coater: coater, entities: append([]*placed(nil), prefix...)}
			if side == 0 {
				module.join.output = nil
				module.join.record.OutputToSlot = 0
			}
			if coater != nil {
				module.entities = append(module.entities, coater)
			}
			module.entities = append(module.entities, monitors...)
			for _, monitor := range monitors {
				others := []*placed{}
				for _, entity := range module.entities {
					if entity != monitor {
						others = append(others, entity)
					}
				}
				if !b.flowMonitorNodeClearAmong(prefix[1], others) {
					return nil, &blueprint.Error{Code: "GEN_E_COLLISION", Message: "独立附件的真实监测宿主没有通过占位检查"}
				}
			}
			if coater != nil {
				begin := len(b.all)
				fragment := &line{info: l.info, recipe: l.recipe, machines: l.machines, coaters: []*placed{coater}, inputs: [][]*placed{prefix}}
				if err := b.coatSupply([]*line{fragment}); err != nil {
					return nil, err
				}
				supply := append([]*placed(nil), b.all[begin:]...)
				if b.setting.EnableFlowMonitor {
					// 供剂外口也保留三个真正地面直线节点，后续边界监测不依赖外部接驳恰好从同高方向进入。
					head := fragment.coaterInput
					first := b.make(2000+b.setting.BeltLv, head.a.x-1, head.a.y, head.a.z)
					first.record.Parameters = append([]int32(nil), head.record.Parameters...)
					connect(first, head)
					fragment.coaterInput = first
					supply = append([]*placed{first}, supply...)
				}
				if index+1 < len(l.coaters) {
					// 串接下一台喷涂机时先把出口移出当前喷涂机通道，portal不能从内部合法通道直接穿越机盒。
					tail := supply[len(supply)-1]
					exit := b.make(2000+b.setting.BeltLv, tail.a.x, tail.a.y+1, tail.a.z)
					connect(tail, exit)
					supply = append(supply, exit)
				}
				if len(supply) < 2 {
					return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "独立附件供剂链缺少实际入口与尾端"}
				}
				module.supplyHead, module.supplyTail, module.supplyInner = fragment.coaterInput, supply[len(supply)-1], supply[len(supply)-2]
				module.entities = append(module.entities, supply...)
			}
			for _, monitor := range monitors {
				others := []*placed{}
				for _, entity := range module.entities {
					if entity != monitor {
						others = append(others, entity)
					}
				}
				if !b.flowMonitorNodeClearAmong(prefix[1], others) {
					return nil, &blueprint.Error{Code: "GEN_E_COLLISION", Message: "监测器与真实供剂通道在附件内发生冲突"}
				}
			}
			for _, entity := range module.entities {
				removed[entity] = true
			}
			modules = append(modules, module)
		}
	}
	keep := func(entities []*placed) []*placed {
		result := make([]*placed, 0, len(entities))
		for _, entity := range entities {
			if !removed[entity] {
				result = append(result, entity)
			}
		}
		return result
	}
	l.all, b.all = keep(l.all), keep(b.all)
	return modules, nil
}

// rebindDetachedStreetCargo 职责：在附件尚未接回的构造阶段只证明真实cargo几何；使用：核心周期/相位试探；边界：保留coater指针且不写喷涂通过诊断，提交接驳后必须执行完整喷涂顺序和最终重绑。
func (b *builder) rebindDetachedStreetCargo(ctx context.Context, records []blueprint.Building) error {
	if len(records) != len(b.all) {
		return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "附件构造期记录与实体数量不一致"}
	}
	world := make([]density.CargoPoint, len(b.all))
	indices := map[*placed]int{}
	byIndex := map[int32]*placed{}
	for i, entity := range b.all {
		p := connectedPowerWorld(entity.a)
		world[i] = density.CargoPoint{p[0], p[1], p[2]}
		indices[entity] = i
		byIndex[records[i].Index] = entity
	}
	up := density.CargoPoint{0, 1, 0}
	sampler, err := density.NewCargoSampler(records, world, density.CargoOptions{PlanarUp: &up})
	if err != nil {
		return err
	}
	type choice struct {
		arm     *placed
		index   int
		anchor  *placed
		target  vector
		binding density.CargoBinding
	}
	decisions := []choice{}
	for index, arm := range b.all {
		if !arm.densityCargo.enabled {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		target, err := denseCargoLocalTarget(arm)
		if err != nil {
			return err
		}
		goal := connectedPowerWorld(target)
		allowed := []int32{}
		for _, node := range arm.densityCargo.track {
			if i, ok := indices[node]; ok {
				p := world[i]
				if math.Pow(p[0]-goal[0], 2)+math.Pow(p[1]-goal[1], 2)+math.Pow(p[2]-goal[2], 2) <= referenceCargoAnchorRadius*referenceCargoAnchorRadius {
					allowed = append(allowed, records[i].Index)
				}
			}
		}
		binding, err := sampler.Nearest(allowed, density.CargoPoint{goal[0], goal[1], goal[2]}, referenceCargoTolerance)
		if err != nil {
			return fmt.Errorf("附件构造期货物几何：%w", err)
		}
		anchor, exists := byIndex[binding.AnchorIndex]
		if !exists {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "附件构造期采样返回无效锚点"}
		}
		decisions = append(decisions, choice{arm: arm, index: index, anchor: anchor, target: target, binding: binding})
	}
	for _, value := range decisions {
		arm, r := value.arm, &records[value.index]
		if arm.densityCargo.output {
			arm.output = value.anchor
			arm.record.OutputOffset = value.binding.Offset
			r.OutputObjIdx = value.binding.AnchorIndex
			r.OutputOffset = value.binding.Offset
		} else {
			arm.input = value.anchor
			arm.record.InputOffset = value.binding.Offset
			r.InputObjIdx = value.binding.AnchorIndex
			r.InputOffset = value.binding.Offset
		}
		arm.densityCargo.relative = vector{value.target.x - value.anchor.a.x, value.target.y - value.anchor.a.y, value.target.z - value.anchor.a.z}
	}
	return nil
}

// commitDenseStreetAccessories 职责：核对已存在实体路径并重建每行的完整有向物料/供剂证明；使用：所有模块完成全场路由后；边界：不生成虚拟带、不清空喷涂要求，失败整个候选丢弃。
func (b *builder) commitDenseStreetAccessories(assembly *denseStreetAssembly) error {
	if assembly == nil || assembly.street == nil || assembly.committed {
		return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "附件提交缺少构造结果或重复提交"}
	}
	alive := map[*placed]bool{}
	for _, entity := range b.all {
		alive[entity] = true
	}
	for _, wire := range assembly.links {
		if len(wire.path) < 2 || wire.path[0] != wire.start || wire.path[len(wire.path)-1] != wire.end {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "附件内部线没有返回实际首尾完整路径"}
		}
		for i, node := range wire.path {
			if !alive[node] || !surfaceIsBelt(node.record.ItemID) || i+1 < len(wire.path) && node.output != wire.path[i+1] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "附件内部线缺少实体节点或有向连接"}
			}
		}
	}
	appendUnique := func(dst []*placed, src []*placed) []*placed {
		known := map[*placed]bool{}
		for _, node := range dst {
			known[node] = true
		}
		for _, node := range src {
			if !known[node] {
				dst = append(dst, node)
				known[node] = true
			}
		}
		return dst
	}
	for _, wire := range assembly.links {
		l := assembly.street.groups[wire.group]
		l.all = appendUnique(l.all, wire.path)
		assembly.all = appendUnique(assembly.all, wire.path)
		if wire.purpose != "material" {
			continue
		}
		module := wire.accessory
		path := append([]*placed(nil), wire.path...)
		if !module.input {
			slices.Reverse(path)
		}
		row := append(append(append([]*placed(nil), module.material...), path[1:len(path)-1]...), module.core...)
		if module.input {
			l.inputs[module.index] = row
		} else {
			l.outputs[module.index] = row
			if module.index == 0 {
				l.output = row
			}
		}
	}
	graph := finalMachineMaterialGraph(b.all)
	for _, l := range assembly.street.groups {
		if err := validateLineMachineInserters(l, b.setting); err != nil {
			return err
		}
		if err := validateLineSprayOrder(l); err != nil {
			return err
		}
		if err := validateFinalLineCoaters(l, b.all, graph); err != nil {
			return err
		}
		for _, arm := range l.all {
			if !arm.densityCargo.enabled || arm.densityCargo.output {
				continue
			}
			if err := b.validateDenseCargoSprayOrder(arm, arm.input); err != nil {
				return err
			}
		}
	}
	for _, port := range assembly.sprayPorts {
		if !slices.Contains(b.coaterLinks[port.item], port.head) {
			b.coaterLinks[port.item] = append(b.coaterLinks[port.item], port.head)
		}
	}
	assembly.committed = true
	return nil
}
