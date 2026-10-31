package build

// 文件职责：多原料按比例混带、跨层补料与机器循环供料；使用：useMixed 和六原料研究站；边界：比例只建模理论速率，不模拟堵带时序。
// 按 MixDistributeHelper、ABelt/BBelt/CBelt、MixBeltLine 的算法意图独立实现，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// mixPlan 职责：保存物料分配；使用：混料入口；边界：保持原配方索引便于站点物品对应。
type mixPlan struct {
	// index 职责：配方原料位置；使用：输出 heads；边界：不按排序反推。
	index int
	// id 职责：原料原型；使用：过滤；边界：不为零。
	id int
	// expected 职责：期望带宽；使用：最小利用率；边界：每秒单位。
	expected float64
	// real 职责：1.5 阶梯分配；使用：限速夹子；边界：剩余带宽不静默挪用。
	real float64
}

// mixAllocation 职责：比例分配、保底及补短板；使用：混料装配；边界：不改变配方消耗比例。
func mixAllocation(r quant.Recipe) []mixPlan {
	plans := make([]mixPlan, len(r.Items))
	total := 0.
	for _, v := range r.ItemCounts {
		total += v
	}
	zeros := 0
	for i, id := range r.Items {
		expected := r.ItemCounts[i] / total * 30
		real := math.Floor(expected/1.5) * 1.5
		plans[i] = mixPlan{index: i, id: id, expected: expected, real: real}
		if real <= 0 {
			zeros++
		}
	}
	if zeros > 0 {
		remainExpected := 0.
		for _, p := range plans {
			if p.real > 0 {
				remainExpected += p.expected
			}
		}
		for i := range plans {
			if plans[i].real == 0 {
				plans[i].real = 1.5
			} else {
				plans[i].real = math.Floor((plans[i].expected/remainExpected*(30-float64(zeros)*1.5))/1.5) * 1.5
			}
		}
	}
	sum := 0.
	for _, p := range plans {
		sum += p.real
	}
	minimum := func() int {
		index := 0
		for i := range plans {
			if plans[i].real/plans[i].expected < plans[index].real/plans[index].expected {
				index = i
			}
		}
		return index
	}
	for remain := 30 - sum; remain >= 1.5; remain -= 1.5 {
		index := minimum()
		before := plans[index].real / plans[index].expected
		plans[index].real += 1.5
		after := plans[minimum()].real / plans[minimum()].expected
		if after <= before {
			plans[index].real -= 1.5
			break
		}
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].real != plans[j].real {
			return plans[i].real > plans[j].real
		}
		return plans[i].id < plans[j].id
	})
	return plans
}

// mixTransfers 职责：用一格夹子表示分配；使用：混料门控；边界：不把 0.75 档混入参考 1.5 阶梯。
func mixTransfers(rate float64) []transfer {
	out := []transfer{}
	for _, entry := range []transfer{{3, 1, 6}, {2, 1, 3}, {1, 1, 1.5}} {
		for rate >= entry.power-1e-8 {
			out = append(out, entry)
			rate -= entry.power
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].level < out[j].level })
	return out
}

// mixInlet 职责：保存某原料 A/B/C 路径；使用：混料循环组装；边界：指针只在请求内存在。
type mixInlet struct {
	// plan 职责：原料分配；使用：过滤；边界：无自动喷涂优化。
	plan mixPlan
	// entry 职责：外部供料口；使用：站点连接；边界：地面整数格。
	entry *placed
	// recycle 职责：多余原料返回口；使用：闭环过滤；边界：独立于外部入口。
	recycle *placed
	// gates 职责：定量输出格；使用：按比例夹取；边界：地面高度。
	gates []*placed
	// transfers 职责：限速组合；使用：创建夹子；边界：每个输出格一组。
	transfers []transfer
}

// mixedInlets 职责：按参考跨层补料几何生成入口；使用：混带机器行；边界：不使用二维路径跨过其它原料。
func (b *builder) mixedInlets(plans []mixPlan) ([]mixInlet, float64) {
	start := len(b.all)
	n := len(plans)
	lengths := make([]int, n)
	for i := n - 1; i >= 0; i-- {
		lengths[i] = 1
		if i < n-1 {
			lengths[i] = 1 + lengths[i+1] + len(mixTransfers(plans[i+1].real))
			if n-i-2 > 0 {
				lengths[i] += 2
			}
			if *b.setting.Mix2 {
				lengths[i]++
			}
		}
	}
	result := make([]mixInlet, n)
	width := 0.
	for i, plan := range plans {
		height := n - i - 1
		xoff := float64(i * 2)
		transfers := mixTransfers(plan.real)
		points := []vector{{xoff, 1, 0}}
		straightStart := 1
		if height > 0 {
			for h := 0; h <= height; h++ {
				points = append(points, vector{xoff + 1 + .002*float64(h), 1, float64(h)})
			}
			straightStart = 2
		}
		var lastStraight vector
		for j := 0; j < lengths[i]; j++ {
			lastStraight = vector{xoff + float64(straightStart+j), 1, float64(height)}
			points = append(points, lastStraight)
		}
		gateStart := straightStart + lengths[i]
		if height > 0 {
			for h := 0; h <= height; h++ {
				points = append(points, vector{xoff + float64(2+lengths[i]) + .002*float64(h), 1, float64(height - h)})
			}
			gateStart = 3 + lengths[i]
		}
		for j := range transfers {
			points = append(points, vector{xoff + float64(gateStart+j), 1, 0})
		}
		a := b.byPath(points)
		gates := a[len(a)-len(transfers):]
		// 外部入口 B 先向右爬升，再向原料循环 A 的高架段接入。
		bp := []vector{}
		for j := 0; j < 4; j++ {
			bp = append(bp, vector{float64(j) - 3, float64(n-i) + 1, 0})
		}
		for h := 0; h <= height; h++ {
			bp = append(bp, vector{1 + .002*float64(h), float64(n-i) + 1, float64(h)})
		}
		// 零层只有一格直段时不得强行走到 x=2：该格属于混料 2.0 收集带，会造成重叠和斜向断接。
		if lastStraight.x >= 2 {
			bp = append(bp, vector{2, float64(n-i) + 1, float64(height)})
		}
		bp = append(bp, manhattan(bp[len(bp)-1], vector{lastStraight.x, 2, float64(height)}, false)[1:]...)
		bb := b.byPath(bp)
		bb[0].record.Parameters = []int32{int32(plan.id), 0}
		for _, candidate := range a {
			if candidate.a == lastStraight {
				bb[len(bb)-1].output = candidate
				break
			}
		}
		bb[len(bb)-1].record.OutputToSlot = 2
		result[i] = mixInlet{plan: plan, entry: bb[0], recycle: a[0], gates: gates, transfers: transfers}
		if len(gates) > 0 {
			width = math.Max(width, gates[len(gates)-1].a.x+2)
		}
	}
	shift(b.all[start:], 3, 0, 0)
	return result, width + 3
}

// buildMixedLine 职责：以循环混带给两侧机器供货；使用：多原料及 useMixed；边界：需要白爪，未满足时明确提示而非丢掉输入。
func (b *builder) buildMixedLine(p quant.Production, r quant.Recipe, count, mode int) (*line, error) {
	if err := b.checkGeneration("循环混带机器行"); err != nil {
		return nil, err
	}
	if count <= 0 || count > 10000 {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "循环混带机器行必须包含1到10000台真实机器"}
	}
	start := len(b.all)
	if len(r.Items) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "混带配方没有原料"}
	}
	maps := map[int]geometry{}
	if err := json.Unmarshal(geometryData, &maps); err != nil {
		return nil, err
	}
	conf, ok := maps[p.MachineType]
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器类型不支持混带"}
	}
	plans := mixAllocation(r)
	inlets, width := b.mixedInlets(plans)
	pre := 3.
	needCoater := p.IncreasePoints > 0 && p.ProductionMode >= 0
	if needCoater {
		pre = 8
	}
	spacing := conf.Space + float64(b.setting.extraSpace(p.MachineType))
	ground := count
	if p.MachineType == 6 {
		ground = int(math.Ceil(float64(count) / float64(b.setting.MaxLays)))
	}
	length := int(width+pre+float64(int(math.Ceil(float64(ground)/2)))*spacing) + 2
	send := b.straight(1, 0, length, 2)
	back := b.straight(1, 0, length, 2)
	shift(back, 0, 0, 1)
	reverse(back)
	output := b.straight(0, -1, length, 2)
	if mode != 6 {
		reverse(output)
	}
	l := &line{recipe: r, info: p, output: output, inputs: make([][]*placed, len(r.Items))}
	for _, inlet := range inlets {
		l.inputs[inlet.plan.index] = []*placed{inlet.entry}
		var collector []*placed
		if *b.setting.Mix2 {
			points := []vector{}
			for j := 0; j < len(inlet.gates)+1; j++ {
				points = append(points, vector{inlet.gates[0].a.x + float64(j), 2, 0})
			}
			last := points[len(points)-1]
			points = append(points, vector{last.x, 1, 0})
			collector = b.byPath(points)
			lastBelt := collector[len(collector)-1]
			lastBelt.output = send[int(math.Round(lastBelt.a.x))-1]
			lastBelt.record.OutputToSlot = 2
		}
		for j, gate := range inlet.gates {
			target := send[int(math.Round(gate.a.x))-1]
			if len(collector) > 0 {
				target = collector[j]
			}
			if _, err := b.beltTransfer(gate, target, inlet.transfers[j].level, 0); err != nil {
				return nil, err
			}
		}
		source := send[int(math.Round(inlet.recycle.a.x))-1]
		if _, err := b.beltTransfer(source, inlet.recycle, 4, inlet.plan.id); err != nil {
			return nil, err
		}
	}
	for _, right := range []bool{false, true} {
		x := send[0].a.x
		if right {
			x = send[len(send)-1].a.x
		}
		splitter := b.make(2020, x, 0, 0)
		splitter.record.ModelIndex = 39
		splitter.record.Parameters = make([]int32, 6)
		splitter.record.OutputToSlot = 14
		splitter.record.InputFromSlot = 15
		splitter.record.OutputFromSlot = 15
		splitter.record.InputToSlot = 14
		if right {
			send[len(send)-1].output = splitter
			send[len(send)-1].record.OutputToSlot = 2
			back[len(back)-1].input = splitter
			back[len(back)-1].record.InputFromSlot = 3
		} else {
			send[0].input = splitter
			send[0].record.InputFromSlot = 0
			back[0].output = splitter
			back[0].record.OutputToSlot = 1
		}
	}
	sides := []int{int(math.Ceil(float64(count) / 2)), count / 2}
	for side, sideCount := range sides {
		groundCount := sideCount
		if p.MachineType == 6 {
			groundCount = int(math.Ceil(float64(sideCount) / float64(b.setting.MaxLays)))
		}
		remaining := sideCount
		for i := 0; i < groundCount; i++ {
			x := width + pre + float64(i)*spacing
			y := 1 + conf.Bottom
			if side == 1 {
				y = -conf.Top - 2
			}
			base := b.make(p.MachineID, x, y, 0)
			base.record.RecipeID = int16(p.RecipeID)
			base.record.Yaw = conf.Yaw
			base.record.Yaw2 = conf.Yaw
			base.record.InputToSlot = 0
			acc := max(0, p.ProductionMode)
			base.record.Parameters = []int32{int32(acc)}
			if p.MachineType == 6 {
				base.record.Parameters = []int32{1, int32(acc)}
				base.record.OutputToSlot = 14
				base.record.InputFromSlot = 15
				base.record.OutputFromSlot = 15
				base.record.InputToSlot = 14
			}
			l.machines = append(l.machines, base)
			layers := 1
			if p.MachineType == 6 {
				layers = min(remaining, b.setting.MaxLays)
			}
			remaining -= layers
			previous := base
			for z := 1; z < layers; z++ {
				m := b.make(p.MachineID, x, y, float64(z*3))
				m.record = base.record
				m.record.Parameters = append([]int32{}, base.record.Parameters...)
				m.input = previous
				previous = m
				l.machines = append(l.machines, m)
			}
			inArm := b.make(2014, 0, 0, 0)
			outArm := b.make(2014, 0, 0, 0)
			inArm.record.InputFromSlot = -1
			inArm.output = base
			outArm.input = base
			outArm.record.OutputToSlot = -1
			if side == 0 {
				inArm.input = send[int(math.Round(x-1))-1]
				inArm.record.OutputToSlot = conf.Slots[0]
				inArm.record.Yaw = 0
				inArm.record.Yaw2 = 0
				px := x - conf.Left + conf.Outside
				inArm.a = vector{px, y - conf.Bottom - 1, 0}
				inArm.b = vector{px, y - conf.Bottom + conf.Lack, 0}
				inArm.record.Parameters = []int32{1}
				outArm.output = output[int(math.Round(x))]
				outArm.record.InputFromSlot = conf.Slots[1]
				outArm.record.Yaw = 180
				outArm.record.Yaw2 = 180
				px = x - conf.Left + 1
				outArm.a = vector{px, y - conf.Bottom + conf.Lack, 0}
				outArm.b = vector{px, y - conf.Bottom - 2, 0}
				outArm.record.Parameters = []int32{2}
			} else {
				inArm.input = send[int(math.Round(x+1))-1]
				inArm.record.OutputToSlot = conf.Slots[5]
				inArm.record.Yaw = 180
				inArm.record.Yaw2 = 180
				px := x - conf.Left - conf.Outside + 2
				inArm.a = vector{px, y + conf.Top + 2, 0}
				inArm.b = vector{px, y + conf.Top - conf.Lack, 0}
				inArm.record.Parameters = []int32{2}
				outArm.output = output[int(math.Round(x-1))]
				outArm.record.InputFromSlot = conf.Slots[3]
				outArm.record.Yaw = 0
				outArm.record.Yaw2 = 0
				px = x - conf.Left + conf.Outside
				outArm.a = vector{px, y + conf.Top - conf.Lack, 0}
				outArm.b = vector{px, y + conf.Top + 1, 0}
				outArm.record.Parameters = []int32{1}
			}
		}
	}
	if needCoater {
		c := b.make(2313, width+3, 0, 0)
		c.record.OutputToSlot = 14
		c.record.InputFromSlot = 15
		c.record.OutputFromSlot = 15
		c.record.InputToSlot = 14
		l.coaters = append(l.coaters, c)
		for _, belt := range back {
			if belt.a.x >= width+1 && belt.a.x < width+5 {
				belt.a.z = 2
				belt.b.z = 2
			}
		}
	}
	if *b.setting.NeedPower {
		for x := width + pre; x < float64(length); x += 5 {
			b.make(2201, x, 1, 0)
		}
	}
	if b.setting.InserterLv < 4 {
		b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 的混带机器输入使用集装分拣器（白爪），需已解锁对应科技", r.ID))
	}
	if b.setting.BeltLv < 3 {
		b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 混料按 30/s 分配，当前带等级较低可能堵带，建议极速传送带", r.ID))
	}
	if mode == 2 || mode == 5 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "多原料混带请使用模式 1、3、4 或 6，配送器模式不支持该入口"}
	}
	l.all = b.all[start:]
	return l, nil
}
