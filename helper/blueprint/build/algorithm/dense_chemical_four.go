package build

// 文件职责：在已有参考双排化工单元外侧增加第四种独立物料。
// 使用：塔模式的独立密铺候选；边界：保留原设备、数量、三种输入与产物，外带在两排外侧串接，不把不同物料混带。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
	"fmt"
	"math"
	"sort"
)

// chemicalFourKey 职责：只为已注册的独立候选启用四物料化工单元；使用：候选上下文；边界：原有候选和普通模式保持原布局。
type chemicalFourKey struct{}

// referenceChemicalFourEligible 职责：限定四物料单元的型号、配方和真实单臂能力；使用：装配及容量拆行；边界：无喷涂、无监测，三输入一产物，原型一致性仍在参考装配入口核验。
func (b *builder) referenceChemicalFourEligible(p quant.Production, r quant.Recipe) bool {
	if b.generationCtx == nil || b.adapter == nil || b.adapter.density != densityDense || b.setting.EnableFlowMonitor {
		return false
	}
	enabled, _ := b.generationCtx.Value(chemicalFourKey{}).(bool)
	return enabled && p.MachineType == 4 && denseReferenceSource(p.MachineID) == 2317 && p.IncreasePoints == 0 && len(r.Items) == 3 && len(r.Results) == 1 && !recipeHasMachineRecycle(r) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv)
}

// appendChemicalOuterPort 职责：把第三原料接到与中心三轨独立的外侧U形带；使用：基础参考两输入单产物装配之后；边界：按实际非对称端口位置取反射中心，保留三格平直货物采样段，最终仍执行原生槽位、净空及cargo终检。
// 逻辑流：1. 核对每台机器的相对端口 => 2. 创建两排外侧及高架过渡 => 3. 复制所选等级的真实臂 => 4. 绑定本物料轨及货物目标。
func (b *builder) appendChemicalOuterPort(l *line) error {
	cat, e := placement.LoadCatalog()
	if e != nil {
		return e
	}
	g, e := cat.Geometry(int16(l.info.MachineID), b.models[itoa(l.info.MachineID)])
	if e != nil {
		return e
	}
	type target struct {
		machine, template *placed
		slot              int8
		point, cargo      vector
		row               int
		midY              float64
	}
	targets := []target{}
	rowYs := []float64{}
	machineSet := map[*placed]bool{}
	for _, m := range l.machines {
		machineSet[m] = true
	}
	firstTrack := map[*placed]bool{}
	for _, p := range l.inputs[0] {
		firstTrack[p] = true
	}
	mirror := func(p vector, midY float64) vector { return vector{p.x, 2*midY - p.y, p.z} }
	for _, arm := range b.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 || !machineSet[arm.output] || !firstTrack[arm.input] {
			continue
		}
		m := arm.output
		if arm.record.OutputToSlot < 0 || int(arm.record.OutputToSlot) >= len(g.SlotPoses) {
			return fmt.Errorf("化工第四物料缺少已核对的原输入槽位")
		}
		old := g.SlotPoses[arm.record.OutputToSlot].Position
		slot := int8(-1)
		best := math.Inf(1)
		for i, pose := range g.SlotPoses {
			v := pose.Position
			if v[2]*old[2] >= 0 {
				continue
			}
			distance := math.Abs(float64(v[0] - old[0]))
			if distance < best {
				slot, best = int8(i), distance
			}
		}
		if slot < 0 || best > 1e-4 {
			return fmt.Errorf("化工第四物料缺少同横向位置的另一侧槽位，误差%.9f米", best)
		}
		midY := m.a.y + math.Cos(float64(m.record.Yaw)*math.Pi/180)*float64(old[2]+g.SlotPoses[slot].Position[2])/2/planarGridLength
		p := mirror(arm.input.a, midY)
		cargo, e := denseCargoLocalTarget(arm)
		if e != nil {
			return e
		}
		cargo = mirror(cargo, midY)
		row := -1
		for i, y := range rowYs {
			if math.Abs(y-p.y) < .01 {
				row = i
				break
			}
		}
		if row < 0 {
			row = len(rowYs)
			rowYs = append(rowYs, p.y)
		}
		targets = append(targets, target{m, arm, slot, p, cargo, row, midY})
	}
	if len(targets) != len(l.machines) || len(rowYs) > 2 {
		return fmt.Errorf("化工第四物料机器与外轨不匹配：%d/%d台、%d排", len(targets), len(l.machines), len(rowYs))
	}
	points := []vector{}
	push := func(to vector) {
		if len(points) == 0 {
			points = append(points, to)
			return
		}
		a := points[len(points)-1]
		d := math.Max(math.Abs(a.x-to.x), math.Max(math.Abs(a.y-to.y), math.Abs(a.z-to.z)))
		if d < 1e-6 {
			return
		}
		steps := int(math.Ceil(d))
		for i := 1; i <= steps; i++ {
			f := float64(i) / float64(steps)
			points = append(points, vector{a.x + (to.x-a.x)*f, a.y + (to.y-a.y)*f, a.z + (to.z-a.z)*f})
		}
	}
	maximum := math.Inf(-1)
	for _, v := range targets {
		maximum = math.Max(maximum, v.point.x)
	}
	right := math.Ceil(maximum) + 6
	for row, y := range rowYs {
		xs := []float64{}
		for _, v := range targets {
			if v.row == row {
				xs = append(xs, v.point.x)
			}
		}
		sort.Float64s(xs)
		if row == 0 {
			push(vector{0, y, 0})
			push(vector{math.Floor(xs[0]) - 2, y, .125})
			for _, x := range xs {
				push(vector{x, y, .125})
			}
			push(vector{math.Ceil(maximum) + 3, y, .125})
			if len(rowYs) > 1 {
				push(vector{right, y, 2})
				push(vector{right, rowYs[1], 2})
				push(vector{math.Ceil(maximum) + 3, rowYs[1], .125})
			}
		} else {
			for i := len(xs) - 1; i >= 0; i-- {
				push(vector{xs[i], y, .125})
			}
			push(vector{0, y, 0})
		}
	}
	track := b.byPath(points)
	l.inputs = append(l.inputs, track)
	for _, v := range targets {
		anchor := track[0]
		best := math.Inf(1)
		for _, node := range track {
			d := math.Abs(node.a.x-v.point.x) + math.Abs(node.a.y-v.point.y) + math.Abs(node.a.z-v.point.z)
			if d < best {
				best, anchor = d, node
			}
		}
		if best > .01 {
			return fmt.Errorf("化工第四物料缺少实际带锚点，误差%.6f格", best)
		}
		a := b.make(int(v.template.record.ItemID), 0, 0, 0)
		a.record = v.template.record
		a.record.Parameters = append([]int32(nil), v.template.record.Parameters...)
		a.a, a.b = mirror(v.template.a, v.midY), mirror(v.template.b, v.midY)
		a.record.Yaw = float32(math.Mod(float64(a.record.Yaw)+180, 360))
		a.record.Yaw2 = float32(math.Mod(float64(a.record.Yaw2)+180, 360))
		a.record.OutputToSlot = v.slot
		a.input, a.output = anchor, v.machine
		a.densityPattern = "reference-low-arm"
		setDenseCargoTarget(a, anchor, track, v.cargo, false)
	}
	if b.logistics != nil {
		found := false
		for _, check := range b.logistics.Checks {
			found = found || check.RuleID == "reference-chemical-four-port"
		}
		if !found {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "reference-chemical-four-port", Status: "passed", Message: "参考化工双排中央三轨保留，第三原料由另一侧真实槽位接入独立外轨；货物采样缓冲、原设备等级和配置毛流保持，仍须完整原生终检"})
		}
	}
	return nil
}
