package build

// 文件职责：用唯一三模型profile构造真实槽点、原生过渡和离壳接带，并保存端口角色证明。
// 使用：原料和产物分流树共享一门构造；边界：不改变profile、碰撞例外或物料份额，所有接头均为所选等级真实带实体。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// splitterGateModelKey 职责：为独立候选选择真实三形态；使用：context携带int16模型38/39/40；边界：不属于用户等级设置，不接受未知模型。
type splitterGateModelKey struct{}

// splitterGateSpec 职责：指定一个共同门的实体姿态及端口角色；使用：模型角色选择或显式定向接头；边界：input和outputs必须是互异的真实槽0..3。
type splitterGateSpec struct {
	model   int16
	yaw     float32
	origin  vector
	input   int8
	outputs []int8
}

// splitterGateAssembly 职责：保存已注册共同门及完整有向接头；使用：原料/产物树装配；边界：input外向内、outputs内向外，元数据不替代最终实际图核验。
type splitterGateAssembly struct {
	gate            *placed
	entities, input []*placed
	outputs         map[int8][]*placed
	inputSlot       int8
	outputSlots     []int8
	model           int16
	links           map[int8]*splitterGatePortLink
}

// splitterGatePortLink 职责：保存一个端口不可裁除的真实节点及流向；使用：共同门最终证明；边界：位置必须从当前gate姿态和只读profile重算。
type splitterGatePortLink struct {
	assembly *splitterGateAssembly
	slot     int8
	input    bool
	nodes    []*placed
}

// splitterGateModel 职责：读取当前候选的模型选择；使用：共同门默认角色；边界：缺省38，非法显式值失败而不换型。
func splitterGateModel(ctx context.Context) (int16, error) {
	model := int16(38)
	if ctx != nil {
		switch value := ctx.Value(splitterGateModelKey{}).(type) {
		case int16:
			model = value
		case int:
			if value < 38 || value > 40 {
				return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "分流器候选模型未核对"}
			}
			model = int16(value)
		case nil:
		default:
			return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "分流器候选模型类型未核对"}
		}
	}
	if model < 38 || model > 40 {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "分流器候选模型未核对"}
	}
	return model, nil
}

// splitterGateRotate 职责：按真实建筑Yaw旋转原型米制向量；使用：槽点及方向投影；边界：尚不施加蓝图格距、平移或原生经向比例。
func splitterGateRotate(point placement.Vector, yaw float32) placement.Vector {
	angle := float64(yaw) * math.Pi / 180
	s, c := math.Sin(angle), math.Cos(angle)
	return placement.Vector{point[0]*c + point[2]*s, point[1], -point[0]*s + point[2]*c}
}

// defaultSplitterGateSpec 职责：按真实高度选择地面西向输入及优先地面东向输出；使用：两种树共享角色规则；边界：其余输出保持真实上层高度，38/39可保持Yaw90槽2，40由profile选到Yaw0槽3。
func defaultSplitterGateSpec(ctx context.Context, count int) (splitterGateSpec, error) {
	model, err := splitterGateModel(ctx)
	if err != nil {
		return splitterGateSpec{}, err
	}
	profile, err := loadSplitterPortProfile(model)
	if err != nil || count < 1 || count > 3 {
		return splitterGateSpec{}, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共同分流门缺少已核对模型或有效出口数"}
	}
	low := math.Inf(1)
	for _, port := range profile.ports {
		low = math.Min(low, port.point[1])
	}
	spec := splitterGateSpec{model: model, input: -1}
	choose := func(yaw float32, slot int8) bool {
		port := profile.ports[slot]
		direction := splitterGateRotate(port.forward, yaw)
		if math.Abs(port.point[1]-low) < 1e-5 && direction[0] < -.999 && math.Abs(direction[2]) < 1e-5 {
			spec.yaw, spec.input = yaw, slot
			return true
		}
		return false
	}
	if !choose(90, 2) {
		for _, yaw := range []float32{0, 90, 180, 270} {
			for slot := int8(0); slot < 4; slot++ {
				if choose(yaw, slot) {
					break
				}
			}
			if spec.input >= 0 {
				break
			}
		}
	}
	if spec.input < 0 {
		return spec, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "分流器没有可核对的地面西向输入"}
	}
	for slot := int8(0); slot < 4; slot++ {
		if slot != spec.input {
			spec.outputs = append(spec.outputs, slot)
		}
	}
	sort.SliceStable(spec.outputs, func(i, j int) bool {
		a, c := profile.ports[spec.outputs[i]], profile.ports[spec.outputs[j]]
		if math.Abs(a.point[1]-c.point[1]) > 1e-5 {
			return a.point[1] < c.point[1]
		}
		return splitterGateRotate(a.forward, spec.yaw)[0] > splitterGateRotate(c.forward, spec.yaw)[0]+1e-5
	})
	spec.outputs = append([]int8{}, spec.outputs[:count]...)
	return spec, nil
}

// splitterGatePortOffsets 职责：保留真实槽点、1.1米原生过渡及离壳带头三个独立实体位置；使用：构造及重算；边界：外端同时满足保守净空和0.35米节点间距，不挪动首槽点。
func splitterGatePortOffsets(profile splitterPortProfile, slot int8, yaw float32, input bool) []vector {
	port := profile.ports[slot]
	terminal := port.transition
	radius := math.Hypot(terminal[0], terminal[2])
	terminal = terminal.Add(port.forward.Scale(math.Max(.36, profile.clearanceMeters-radius)))
	points := []placement.Vector{port.point, port.transition, terminal}
	if input {
		points[0], points[2] = points[2], points[0]
	}
	result := make([]vector, len(points))
	for i, point := range points {
		point = splitterGateRotate(point, yaw)
		result[i] = vector{point[0] / planarGridLength, point[2] / planarGridLength, point[1] / 1.3333333}
	}
	return result
}

// buildSplitterGate 职责：建立一门真实模型和所有指定端口带；使用：原料/产物共同装配；边界：参数先完整验证，所有带使用所选等级，错误不回退为旧一格端口。
func (b *builder) buildSplitterGate(ctx context.Context, spec splitterGateSpec) (*splitterGateAssembly, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if spec.model == 0 {
		model, err := splitterGateModel(ctx)
		if err != nil {
			return nil, err
		}
		spec.model = model
	}
	profile, err := loadSplitterPortProfile(spec.model)
	if err != nil || b.setting.BeltLv < 1 || b.setting.BeltLv > 3 || spec.input < 0 || spec.input > 3 || len(spec.outputs) < 1 || len(spec.outputs) > 3 || math.IsNaN(float64(spec.yaw)) || math.IsInf(float64(spec.yaw), 0) || math.Abs(math.Remainder(float64(spec.yaw), 90)) > 1e-5 {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共同分流门模型、槽位或正交姿态未核对"}
	}
	for _, coordinate := range []float64{spec.origin.x, spec.origin.y, spec.origin.z} {
		if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共同分流门实体位置不是有限值"}
		}
	}
	used := map[int8]bool{spec.input: true}
	for _, slot := range spec.outputs {
		if slot < 0 || slot > 3 || used[slot] {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共同分流门端口角色重复或越界"}
		}
		used[slot] = true
	}
	begin := len(b.all)
	gate := b.make(2020, spec.origin.x, spec.origin.y, spec.origin.z)
	gate.record.ModelIndex, gate.record.Yaw, gate.record.Yaw2 = spec.model, spec.yaw, spec.yaw
	gate.record.Parameters = make([]int32, 6)
	module := &splitterGateAssembly{gate: gate, model: spec.model, inputSlot: spec.input, outputSlots: append([]int8{}, spec.outputs...), outputs: map[int8][]*placed{}, links: map[int8]*splitterGatePortLink{}}
	gate.splitterGate = module
	add := func(slot int8, input bool) []*placed {
		points := splitterGatePortOffsets(profile, slot, spec.yaw, input)
		for i := range points {
			points[i].x += gate.a.x
			points[i].y += gate.a.y
			points[i].z += gate.a.z
		}
		path := b.byPath(points)
		link := &splitterGatePortLink{assembly: module, slot: slot, input: input, nodes: append([]*placed{}, path...)}
		for _, node := range path {
			node.splitterPortLink = link
		}
		module.links[slot] = link
		if input {
			last := path[len(path)-1]
			last.output, last.record.OutputToSlot = gate, slot
		} else {
			path[0].input, path[0].record.InputFromSlot = gate, slot
		}
		return path
	}
	module.input = add(spec.input, true)
	for _, slot := range spec.outputs {
		module.outputs[slot] = add(slot, false)
	}
	module.entities = append([]*placed{}, b.all[begin:]...)
	return module, nil
}

// validateSplitterGateAssembly 职责：从只读profile和当前实体姿态重新证明完整接头及角色；使用：两类树的最终证明；边界：不信任构造时绝对坐标，不允许裁槽点、错模型、错高度或借其它槽位。
func (b *builder) validateSplitterGateAssembly(module *splitterGateAssembly, alive map[*placed]bool) error {
	fail := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共同分流门接头无效：" + reason}
	}
	if module == nil || module.gate == nil || !alive[module.gate] || module.gate.splitterGate != module || module.gate.record.ItemID != 2020 || module.gate.record.ModelIndex != module.model || module.gate.record.Yaw2 != module.gate.record.Yaw || module.gate.input != nil || module.gate.output != nil || len(module.gate.record.Parameters) != 6 || module.gate.record.FilterID != 0 {
		return fail("真实本体或保护身份改变")
	}
	gate := module.gate
	for _, parameter := range gate.record.Parameters {
		if parameter != 0 {
			return fail("无优先/过滤参数改变")
		}
	}
	profile, err := loadSplitterPortProfile(module.model)
	if err != nil || math.IsNaN(float64(gate.record.Yaw)) || math.IsInf(float64(gate.record.Yaw), 0) || math.Abs(math.Remainder(float64(gate.record.Yaw), 90)) > 1e-5 {
		return fail("模型或正交姿态未覆盖")
	}
	expected := map[int8]bool{module.inputSlot: true}
	if module.inputSlot < 0 || module.inputSlot > 3 || len(module.outputSlots) < 1 || len(module.outputSlots) > 3 {
		return fail("指定角色数量或输入槽无效")
	}
	for _, slot := range module.outputSlots {
		if slot < 0 || slot > 3 || expected[slot] {
			return fail("指定角色重复或越界")
		}
		expected[slot] = true
	}
	owned := map[*placed]bool{}
	for _, entity := range module.entities {
		if !alive[entity] || owned[entity] {
			return fail("附件实体未存活或重复登记")
		}
		owned[entity] = true
	}
	if !owned[gate] || len(owned) != 1+3*len(expected) {
		return fail("附件缺少本体或完整真实接头")
	}
	scale := 1.
	if gate.nativeUnitScale > 0 {
		scale = gate.nativeUnitScale
	} else if b.nativeLatitudeScaled && b.nativeLongitudeScale > 0 {
		scale = b.nativeLongitudeScale
	}
	used := map[int8]bool{}
	for slot, link := range module.links {
		if slot < 0 || slot > 3 || !expected[slot] || used[slot] || link == nil || link.assembly != module || link.slot != slot || link.input != (slot == module.inputSlot) || len(link.nodes) != 3 {
			return fail("端口角色、节点数或保护身份不一致")
		}
		used[slot] = true
		positions := splitterGatePortOffsets(profile, slot, gate.record.Yaw, link.input)
		for i, node := range link.nodes {
			want := positions[i]
			want.x = gate.a.x + want.x*scale
			want.y += gate.a.y
			want.z += gate.a.z
			if !alive[node] || !owned[node] || node.splitterPortLink != link || int(node.record.ItemID) != 2000+b.setting.BeltLv || node.record.ModelIndex != node.record.ItemID-1966 || math.IsNaN(node.a.x+node.a.y+node.a.z) || math.IsInf(node.a.x+node.a.y+node.a.z, 0) || math.Abs(node.a.x-want.x) > 1e-5 || math.Abs(node.a.y-want.y) > 1e-5 || math.Abs(node.a.z-want.z) > 1e-5 {
				return fail(fmt.Sprintf("槽%d的真实点/过渡/带头位置、高度或等级改变", slot))
			}
			if i+1 < len(link.nodes) && (node.output != link.nodes[i+1] || node.record.OutputToSlot != 1) {
				return fail("端口内部有向链中断")
			}
		}
		if link.input {
			last := link.nodes[len(link.nodes)-1]
			if !sameDenseMaterialNodes(link.nodes, module.input) || last.output != gate || last.record.OutputToSlot != slot {
				return fail("输入槽实际方向不符")
			}
		} else if !sameDenseMaterialNodes(link.nodes, module.outputs[slot]) || link.nodes[0].input != gate || link.nodes[0].record.InputFromSlot != slot {
			return fail("输出槽实际方向不符")
		}
	}
	if len(used) != len(module.outputSlots)+1 || !used[module.inputSlot] {
		return fail("缺少指定端口")
	}
	for _, slot := range module.outputSlots {
		if !used[slot] || slot == module.inputSlot {
			return fail("指定输出槽缺失或复用了输入")
		}
	}
	return nil
}
