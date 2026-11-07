package build

// 文件职责：复现机器排、插槽、带、研究站堆叠和供电几何；使用：mode 1..6 的基础块；边界：不反推分拣器几何，不修改已有蓝图。
// 依据 ProductLineBuildInfo 与 data/config 的算法和数值事实独立实现。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	_ "embed"
	"encoding/json"
	"math"
)

// geometryData 职责：保存六种机器原始几何；使用：排布；边界：数值不做美化调整。
//
//go:embed geometry.json
var geometryData []byte

// geometry 职责：定义机器槽位几何；使用：每行装配；边界：对应模型固定方向。
type geometry struct {
	// Start 职责：首带偏移；使用：槽位定位；边界：格单位。
	Start float64 `json:"startBeltOffset"`
	// Space 职责：机器中心间距；使用：排布；边界：不加隐含缩放。
	Space float64 `json:"space"`
	// LineSpacing 职责：上下带分隔；使用：排布；边界：格单位。
	LineSpacing float64 `json:"line_spacing"`
	// Bottom 职责：下方中线距离；使用：分拣器端点；边界：原数值。
	Bottom float64 `json:"bottom_mid_distance"`
	// Top 职责：上方中线距离；使用：分拣器端点；边界：原数值。
	Top float64 `json:"top_mid_distance"`
	// Left 职责：横向中线距离；使用：首台位置；边界：原数值。
	Left float64 `json:"left_mid_distance"`
	// Yaw 职责：机器方向；使用：创建；边界：度。
	Yaw float32 `json:"yaw"`
	// Slots 职责：六个分拣器槽位；使用：连线；边界：顺序保持参考值。
	Slots []int8 `json:"soltNum"`
	// Lack 职责：槽位贴合偏移；使用：端点；边界：不取整。
	Lack float64 `json:"bottom_lack"`
	// Outside 职责：边槽微调；使用：端点；边界：不取整。
	Outside float64 `json:"outside_mid_offset"`
}

// line 职责：保留一条产线的入口/机器引用；使用：配送和总线装配；边界：归属由 builder 登记。
type line struct {
	// referenceLowTracks 职责：标识参考图的低双层带单元；使用：局部容量与供电；边界：不跳过最终规则。
	referenceLowTracks bool
	// densePaired 职责：标识不可拆分的相向双排单元；使用：设施周边放置；边界：不能按独立机器行再次拆开。
	densePaired bool
	// all 职责：块建筑；使用：移动；边界：与 builder 指针共享。
	all []*placed
	// inputs 职责：每种原料一条带；使用：配送；边界：保持配方顺序。
	inputs [][]*placed
	// outputs 职责：按配方产物顺序保存机器端独立出货带；使用：各模式逐物品接驳；边界：不同产物不得共享实体带或机器槽位。
	outputs [][]*placed
	// output 职责：保留首条产物带的兼容引用；使用：旧单产物调用；边界：新多产物接驳必须使用outputForItem或outputRows。
	output []*placed
	// machines 职责：包含堆叠的机器；使用：计数；边界：地面分拣器不重复。
	machines []*placed
	// coaters 职责：喷涂机；使用：供剂网络；边界：不假定已有喷涂资源。
	coaters []*placed
	// coaterInput 职责：保存本行供剂干线的实际入口；使用：规范增产剂边接入配送、塔或直连带；边界：仅在coatSupply成功后设置，不是机器配方原料口。
	coaterInput *placed
	// recipe 职责：原始配方；使用：配送槽位；边界：只读副本。
	recipe quant.Recipe
	// info 职责：来源产线；使用：量化对应；边界：不改机器数。
	info quant.Production
}

// extraSpace 职责：读取分类附加间距；使用：机器行；边界：固定格整数。
func (s Setting) extraSpace(typ int) int {
	switch typ {
	case 1:
		return s.SmelterOtherSpace
	case 2:
		return s.WorkbenchOtherSpace
	case 3:
		return s.RefineryOtherSpace
	case 4:
		return s.ChemicalOtherSpace
	case 5:
		return s.ColliderOtherSpace
	case 6:
		return s.MatrixOtherSpace
	}
	return 0
}

// maxCount 职责：读取分类列上限；使用：拆列；边界：不截断总机器数。
func (s Setting) maxCount(typ int) int {
	switch typ {
	case 1:
		return s.SmelterMaxCount
	case 2:
		return s.WorkbenchMaxCount
	case 3:
		return s.RefineryMaxCount
	case 4:
		return s.ChemicalMaxCount
	case 5:
		return s.ColliderMaxCount
	case 6:
		return s.MatrixMaxCount
	}
	return s.MaxMachineInALine
}

// buildLine 职责：装配基础机器行与逐产物独立输出；使用：所有生成策略；边界：输入和产物合计不超过六个真实槽位，容量无解不伪造拓扑。
func (b *builder) buildLine(p quant.Production, r quant.Recipe, count, mode, lineIndex int, last bool) (*line, error) {
	if err := b.checkGeneration("机器行装配"); err != nil {
		return nil, err
	}
	if count <= 0 || count > 10000 {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "机器行必须包含1到10000台真实机器，拒绝空行或超限行"}
	}
	if b.setting.UseMixed || len(r.Items)+len(r.Results) > 6 || len(r.Results) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "独立物料带需要至少一种产物且原料与产物合计至多六种；旧循环混带尚无按所选分拣器等级逐物品验证的实体端口拓扑，需要独立规划"}
	}
	if _, err := productionMaterialFlows(p, r); err != nil {
		return nil, err
	}
	if b.referenceLowTrackEligible(p, r) {
		return b.buildReferenceLowLine(p, r, count, mode, lineIndex, last)
	}
	start := len(b.all)
	gmap := map[int]geometry{}
	if err := json.Unmarshal(geometryData, &gmap); err != nil {
		return nil, err
	}
	conf, ok := gmap[p.MachineType]
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配方机器或输入数量不支持当前布局"}
	}
	l := &line{recipe: r, info: p}
	ports := planMachineInserters(p, r, b.setting, count)
	bottom := ports.bottom
	pre := 0.
	if p.IncreasePoints > 0 && p.ProductionMode >= 0 {
		pre = 4
	}
	ground := count
	pairCount := 0
	if b.canPairDenseLine(p, r, count) {
		pairCount = count / 2
		ground = count - pairCount
	}
	if p.MachineType == 6 {
		ground = int(math.Ceil(float64(count) / float64(ports.layers)))
	}
	spacing := machineSpacing(p.MachineType, conf, b.setting.extraSpace(p.MachineType))
	if b.adapter != nil && b.adapter.density == densityDense {
		var err error
		spacing, err = denseLineSpacing(p.MachineType, b.setting.extraSpace(p.MachineType))
		if err != nil {
			return nil, err
		}
	}
	beltLen := int(float64(ground)*spacing + conf.Start + pre)
	var densePositions []float64
	if b.adapter != nil && b.adapter.density == densityDense {
		densePositions = denseTrackPositions(ground, spacing, float64(int(conf.Start))+pre, float64(beltLen-1))
	}
	machineX := math.Floor(conf.Left + conf.Start + pre)
	machineY := float64(bottom) + conf.Bottom
	belts := make([][]*placed, 0, len(r.Items)+len(r.Results))
	for i := 0; i < len(r.Items)+len(r.Results); i++ {
		if err := b.checkGeneration("机器行传送带装配"); err != nil {
			return nil, err
		}
		y := float64(i)
		if i >= bottom {
			y += conf.LineSpacing
		}
		var row []*placed
		if densePositions != nil {
			row = b.denseTrack(densePositions, y)
		} else {
			row = b.straight(0, y, beltLen, 2)
		}
		belts = append(belts, row)
	}
	for _, port := range ports.inputs {
		track := bottom - port.reach
		if port.top {
			track = bottom + port.reach - 1
		}
		l.inputs = append(l.inputs, belts[track])
	}
	for _, port := range ports.outputs {
		track := bottom - port.reach
		if port.top {
			track = bottom + port.reach - 1
		}
		l.outputs = append(l.outputs, belts[track])
	}
	l.output = l.outputs[0]
	groundMachines := []*placed{}
	for i := 0; i < ground; i++ {
		if i%32 == 0 {
			if err := b.checkGeneration("机器排装配"); err != nil {
				return nil, err
			}
		}
		m := b.make(p.MachineID, machineX+float64(i)*spacing, machineY, 0)
		m.record.RecipeID = int16(p.RecipeID)
		if b.adapter != nil && b.adapter.density == densityDense {
			m.densityPattern = "single-row"
			if pairCount > 0 {
				m.densityPattern = "paired-rows"
			} else if ground == 1 {
				m.densityPattern = "single-machine"
			}
		}
		m.record.Yaw = conf.Yaw
		m.record.Yaw2 = conf.Yaw
		m.record.InputToSlot = 0
		accelerator := p.ProductionMode
		if accelerator < 0 {
			accelerator = 0
		}
		m.record.Parameters = []int32{int32(accelerator)}
		if p.MachineType == 6 {
			m.record.Parameters = []int32{1, int32(accelerator)}
			m.record.OutputToSlot = 14
			m.record.InputFromSlot = 15
			m.record.OutputFromSlot = 15
			m.record.InputToSlot = 14
		}
		groundMachines = append(groundMachines, m)
		l.machines = append(l.machines, m)
	}
	if p.MachineType == 6 {
		remaining := count
		for _, base := range groundMachines {
			if err := b.checkGeneration("研究站堆叠装配"); err != nil {
				return nil, err
			}
			layers := ports.layers
			if remaining < layers {
				layers = remaining
			}
			remaining -= layers
			previous := base
			for z := 1; z < layers; z++ {
				m := b.make(p.MachineID, base.a.x, base.a.y, float64(z)*3)
				m.record = base.record
				m.densityPattern = base.densityPattern
				m.record.Parameters = append([]int32{}, base.record.Parameters...)
				m.input = previous
				previous = m
				l.machines = append(l.machines, m)
			}
		}
	}
	for machineIndex, m := range groundMachines {
		if machineIndex%32 == 0 {
			if err := b.checkGeneration("机器端口装配"); err != nil {
				return nil, err
			}
		}
		for index, port := range ports.inputs {
			if err := b.buildMachineInserterPort(m, l.inputs[index], port, conf, machineIndex, spacing, pre, false, 0); err != nil {
				return nil, err
			}
		}
		for index, port := range ports.outputs {
			filter := 0
			if len(r.Results) > 1 {
				filter = r.Results[index]
			}
			if err := b.buildMachineInserterPort(m, l.outputs[index], port, conf, machineIndex, spacing, pre, true, filter); err != nil {
				return nil, err
			}
		}
	}
	if pairCount > 0 {
		if err := b.completeDensePair(l, pairCount, conf, ports, spacing, pre); err != nil {
			return nil, err
		}
	}
	if pre > 0 {
		for _, row := range l.inputs {
			c := b.make(2313, 2, row[0].a.y, 0)
			c.record.OutputToSlot = 14
			c.record.InputFromSlot = 15
			c.record.OutputFromSlot = 15
			c.record.InputToSlot = 14
			l.coaters = append(l.coaters, c)
		}
	}
	if b.adapter != nil && b.adapter.lineAssembly.reverseOutputRows {
		for _, row := range l.outputs {
			reverse(row)
		}
	}
	if b.adapter != nil && b.adapter.lineAssembly.writeInputParameters {
		for i, row := range l.inputs {
			row[0].record.Parameters = []int32{int32(r.Items[i]), 0}
		}
	}
	for index, row := range l.outputs {
		row[0].record.Parameters = []int32{int32(r.Results[index]), 0}
	}
	if b.setting.EnableFlowMonitor {
		if err := b.addLineFlowMonitors(l, start); err != nil {
			return nil, err
		}
	}
	return b.finishLocalLine(l, conf, p, mode, lineIndex, last, start)
}

// finishLocalLine 职责：为已装配机器单元补齐设施、供电与容量证据；使用：普通和参考密铺共同出口；边界：不重新排列机器或改写参考臂姿态。
func (b *builder) finishLocalLine(l *line, conf geometry, p quant.Production, mode, lineIndex int, last bool, start int) (*line, error) {
	b.previewCheckpoint("assembly")
	if b.adapter != nil && b.adapter.lineAssembly.useFlyTransport {
		if err := b.prepareFlyInletCoaters(l); err != nil {
			return nil, err
		}
		if err := b.coatSupply([]*line{l}); err != nil {
			return nil, err
		}
		if err := b.flyBlocks(l, mode, start); err != nil {
			return nil, err
		}
	}
	if *b.setting.NeedPower {
		if !l.referenceLowTracks && (b.adapter == nil || b.adapter.lineAssembly.useLocalLinePower) {
			// 固定规则只提供低成本的基线杆；随后统一按最终机器和分拣器端点补齐。
			if err := b.linePower(l, conf, p.MachineType, mode, lineIndex, last, start); err != nil {
				return nil, err
			}
		}
		// 每个随后独立移动的本地块必须自足，所有产物数量和模式均按实际消费者补齐。
		if err := b.completeFlyPower(start); err != nil {
			return nil, err
		}
	}
	l.all = b.all[start:]
	if err := validateLineSprayOrder(l); err != nil {
		return nil, err
	}
	if err := b.checkLineMachineInserters(l); err != nil {
		return nil, err
	}
	if err := b.annotateLineProductionRates(l); err != nil {
		return nil, err
	}
	b.machineLines = append(b.machineLines, l)
	b.previewCheckpoint("assembly")
	return l, nil
}

// addLineFlowMonitors 在每条独立物料带上选取空闲节点安装监测器。
// 使用：buildLine 已创建机器、分拣器和物料带后调用；边界：监测器与宿主带同点共存，不更改带链和过滤参数。
func (b *builder) addLineFlowMonitors(l *line, begin int) error {
	rows := append(append([][]*placed{}, l.inputs...), l.outputRows()...)
	seen := map[*placed]bool{}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		mid := len(row) / 2
		var selected *placed
		for distance := 0; distance < len(row); distance++ {
			for _, index := range []int{mid - distance, mid + distance} {
				if index < 0 || index >= len(row) || seen[row[index]] || !b.flowMonitorNodeClearAmong(row[index], b.all[begin:]) {
					continue
				}
				selected = row[index]
				break
			}
			if selected != nil {
				break
			}
		}
		if selected == nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION", Message: "产线物料带没有可安全安装流速监测器的空闲节点"}
		}
		monitor := b.make(2030, selected.a.x, selected.a.y, selected.a.z)
		monitor.record.Yaw = selected.record.Yaw
		monitor.record.Yaw2 = selected.record.Yaw
		seen[selected] = true
	}
	return nil
}

// flowMonitorNodeClear 检查候选带节点是否避开分拣器扫掠及其它设施占地。
// 使用：流速监测器自动选点；边界：允许候选所在传送带共点，不覆盖最终三维碰撞审计。
func (b *builder) flowMonitorNodeClear(belt *placed) bool {
	return b.flowMonitorNodeClearAmong(belt, b.all)
}

// flowMonitorNodeClearAmong 职责：在同一坐标阶段的实体集合内审计监测器选点；使用：本地移块前及全局路由后；边界：调用方不能混用尚未独立平移的块，最终完整碰撞审计仍保留。
func (b *builder) flowMonitorNodeClearAmong(belt *placed, existing []*placed) bool {
	incoming := map[*placed]*placed{}
	for _, entity := range existing {
		if surfaceIsBelt(entity.record.ItemID) && entity.output != nil && surfaceIsBelt(entity.output.record.ItemID) {
			incoming[entity.output] = entity
		}
	}
	if b.adapter != nil && b.adapter.density == densityDense {
		before, after := incoming[belt], belt.output
		if before == nil || after == nil || !surfaceIsBelt(after.record.ItemID) || math.Abs(before.a.z-belt.a.z) > 1e-6 || math.Abs(after.a.z-belt.a.z) > 1e-6 {
			return false
		}
		a, c := vector{belt.a.x - before.a.x, belt.a.y - before.a.y, 0}, vector{after.a.x - belt.a.x, after.a.y - belt.a.y, 0}
		if math.Abs(a.x*c.y-a.y*c.x) > 1e-6 || a.x*c.x+a.y*c.y <= 0 {
			return false
		}
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return false
	}
	monitorShapes, err := catalog.Shapes(2030, b.models["2030"])
	if err != nil {
		return false
	}
	encoding := newPlanarRuleEncoding()
	origin := vector{belt.a.x * planarGridLength, belt.a.z*1.3333333 + .2, belt.a.y * planarGridLength}
	monitorBoxes := make([]placement.Box, 0, len(monitorShapes))
	for _, shape := range monitorShapes {
		box, err := encoding.colliderBox(origin, belt.record.Yaw, shape)
		if err != nil {
			return false
		}
		monitorBoxes = append(monitorBoxes, box)
	}
	toWorld := func(point vector) placement.Vector {
		return placement.Vector{point.x * planarGridLength, point.z*1.3333333 + .2, point.y * planarGridLength}
	}
	for index, other := range existing {
		if index%64 == 0 && b.checkGeneration("流速监测器选点") != nil {
			return false
		}
		if other == belt {
			continue
		}
		if surfaceIsBelt(other.record.ItemID) {
			// 与终检使用相同的0.2米节点抬升及缩短胶囊，低层轨道不能只测未抬升的节点中心。
			point := toWorld(other.a)
			center := point.Add(placement.Vector{0, .2, 0})
			first, second := center, center
			if before, after := incoming[other], other.output; before != nil && after != nil && surfaceIsBelt(after.record.ItemID) {
				first = point.Add(toWorld(before.a).Sub(point).Scale(.65))
				second = point.Add(toWorld(after.a).Sub(point).Scale(.65))
				delta := second.Sub(first).Scale(.45)
				first, second = first.Add(delta).Add(placement.Vector{0, .2, 0}), second.Sub(delta).Add(placement.Vector{0, .2, 0})
			}
			for _, box := range monitorBoxes {
				if box.SegmentDistanceSquared(center, center) < .23*.23 && box.SegmentDistanceSquared(first, second) < .23*.23 {
					return false
				}
			}
			continue
		}
		if other.record.ItemID >= 2011 && other.record.ItemID <= 2014 {
			first, second := toWorld(other.a), toWorld(other.b)
			for _, box := range monitorBoxes {
				if box.SegmentDistanceSquared(first, second) < .55*.55 {
					return false
				}
			}
			continue
		}
		shapes, err := catalog.Shapes(other.record.ItemID, other.record.ModelIndex)
		if err != nil {
			return false
		}
		point := toWorld(other.a)
		for _, shape := range shapes {
			otherBox, err := encoding.colliderBox(vector{point[0], point[1], point[2]}, other.record.Yaw, shape)
			if err != nil {
				return false
			}
			for _, box := range monitorBoxes {
				if box.Overlaps(otherBox) {
					return false
				}
			}
		}
	}
	return true
}

// linePower 职责：按机器参考规则补供电；使用：基础行装配；边界：不生成发电设施。
func (b *builder) linePower(l *line, conf geometry, typ, mode, lineIndex int, last bool, begin int) error {
	_ = conf
	radius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return err
	}
	add := func(x, y float64) error {
		point := vector{x, y, 0}
		// 仅当候选能覆盖尚未覆盖的消费者时才新增；候选点被另一塔覆盖并不能证明可省略。
		needed := false
		for _, consumer := range b.all[begin:] {
			if !finalPowerConsumer(consumer.record) {
				continue
			}
			center := consumer.a
			if flyPowerDistance(center, point) <= radius-1e-4 && !b.flyPowerCovered(center, b.all[begin:]) {
				needed = true
				break
			}
		}
		if err := b.checkGeneration("固定供电候选"); err != nil {
			return err
		}
		if needed && b.flyPowerCandidateAvailable(point, b.all[begin:]) {
			b.make(2201, x, y, 0)
		}
		return nil
	}
	switch typ {
	case 2:
		for i, m := range l.machines {
			if i%4 == 1 || i == len(l.machines)-1 && len(l.machines)%4 == 1 {
				if err := add(m.a.x+2, m.a.y); err != nil {
					return err
				}
			}
		}
	case 4:
		for i, m := range l.machines {
			if i%2 == 1 || i == len(l.machines)-1 && len(l.machines)%2 == 1 {
				if err := add(m.a.x-3, m.a.y+2); err != nil {
					return err
				}
			}
		}
	case 5:
		for _, m := range l.machines {
			if err := add(m.a.x+4, m.a.y); err != nil {
				return err
			}
		}
	default:
		if lineIndex%2 == 0 {
			step := 9.
			if typ == 3 {
				step = 12
			}
			if typ == 6 {
				step = 5
			}
			top, bottom := l.output[0].a.y, l.output[0].a.y
			for _, row := range append(append([][]*placed{}, l.inputs...), l.outputRows()...) {
				if len(row) > 0 {
					top, bottom = math.Max(top, row[0].a.y), math.Min(bottom, row[0].a.y)
				}
			}
			y := top + 1
			if b.adapter != nil && b.adapter.lineAssembly.useBusPowerOffset {
				// 总线回流轨位于出货带外侧一格，电线杆必须再向外让开一格。
				y++
			}
			if lineIndex > 0 && last {
				y = bottom - 1
			}
			start, end := l.machines[0].a.x, l.output[len(l.output)-1].a.x
			lastX := start
			for x := start; x <= end; x += step {
				if err := add(x, y); err != nil {
					return err
				}
				lastX = x
			}
			if end-lastX >= 4 {
				if err := add(end, y); err != nil {
					return err
				}
			}
		}
	}
	if len(l.coaters) > 0 && !(typ == 4 && len(l.machines) == 1) {
		if err := add(l.coaters[0].a.x+1, l.machines[0].a.y); err != nil {
			return err
		}
	}
	return nil
}
