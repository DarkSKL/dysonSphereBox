package build

// 文件职责：复现机器排、插槽、带、研究站堆叠和供电几何；使用：mode 1..6 的基础块；边界：不反推分拣器几何，不修改已有蓝图。
// 依据 ProductLineBuildInfo 与 data/config 的算法和数值事实独立实现，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
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
	if p.MachineType == 6 {
		ground = int(math.Ceil(float64(count) / float64(ports.layers)))
	}
	spacing := machineSpacing(p.MachineType, conf, b.setting.extraSpace(p.MachineType))
	beltLen := int(float64(ground)*spacing + conf.Start + pre)
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
		row := b.straight(0, y, beltLen, 2)
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
			b.buildMachineInserterPort(m, l.inputs[index], port, conf, machineIndex, spacing, pre, false, 0)
		}
		for index, port := range ports.outputs {
			filter := 0
			if len(r.Results) > 1 {
				filter = r.Results[index]
			}
			b.buildMachineInserterPort(m, l.outputs[index], port, conf, machineIndex, spacing, pre, true, filter)
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
	if mode == 1 || mode == 3 || mode == 4 {
		for _, row := range l.outputs {
			reverse(row)
		}
	}
	if mode == 1 {
		for i, row := range l.inputs {
			row[0].record.Parameters = []int32{int32(r.Items[i]), 0}
		}
	}
	for index, row := range l.outputs {
		row[0].record.Parameters = []int32{int32(r.Results[index]), 0}
	}
	if mode == 2 || mode == 5 {
		if err := b.coatSupply([]*line{l}); err != nil {
			return nil, err
		}
		if err := b.flyBlocks(l, mode, start); err != nil {
			return nil, err
		}
	}
	if *b.setting.NeedPower {
		if mode == 2 || mode == 5 {
			if err := b.completeFlyPower(start); err != nil {
				return nil, err
			}
		} else {
			b.linePower(l, conf, p.MachineType, mode, lineIndex, last)
			// 多产物可能占满两侧六轨，旧单侧杆列不能据机器中心推定所有长臂已覆盖。
			// 沿用实际覆盖与有界合法补杆，单产物基线和电力半径保持不变。
			if len(l.outputs) > 1 {
				if err := b.completeFlyPower(start); err != nil {
					return nil, err
				}
			}
		}
	}
	l.all = b.all[start:]
	if err := b.checkLineMachineInserters(l); err != nil {
		return nil, err
	}
	return l, nil
}

// linePower 职责：按机器参考规则补供电；使用：基础行装配；边界：不生成发电设施。
func (b *builder) linePower(l *line, conf geometry, typ, mode, lineIndex int, last bool) {
	add := func(x, y float64) { b.make(2201, x, y, 0) }
	switch typ {
	case 2:
		for i, m := range l.machines {
			if i%4 == 1 || i == len(l.machines)-1 && len(l.machines)%4 == 1 {
				add(m.a.x+2, m.a.y)
			}
		}
	case 4:
		for i, m := range l.machines {
			if i%2 == 1 || i == len(l.machines)-1 && len(l.machines)%2 == 1 {
				add(m.a.x-3, m.a.y+2)
			}
		}
	case 5:
		for _, m := range l.machines {
			add(m.a.x+4, m.a.y)
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
			if mode == 6 {
				// 总线回流轨位于出货带外侧一格，电线杆必须再向外让开一格。
				y++
			}
			if lineIndex > 0 && last {
				y = bottom - 1
			}
			start, end := l.machines[0].a.x, l.output[len(l.output)-1].a.x
			lastX := start
			for x := start; x <= end; x += step {
				add(x, y)
				lastX = x
			}
			if end-lastX >= 4 {
				add(end, y)
			}
		}
	}
	if len(l.coaters) > 0 && !(typ == 4 && len(l.machines) == 1) {
		add(l.coaters[0].a.x+1, l.machines[0].a.y)
	}
}
