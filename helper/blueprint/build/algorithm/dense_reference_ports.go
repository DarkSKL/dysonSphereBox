package build

// 文件职责：为参考同侧三槽密铺单元补充第三种独立物料。
// 使用：两输入一产物或一输入两产物；边界：各物料有独立带和臂，不恢复循环混带，不改变设备等级。

import (
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// referenceLowRecipe 职责：声明参考三槽局部拓扑的适用范围；使用：容量拆分和装配；边界：喷涂从地面入口接入，机器回流另行处理，未知原型不套用来源姿态。
func referenceLowRecipe(p quant.Production, r quant.Recipe) bool {
	if denseReferenceSource(p.MachineID) == 0 || len(r.Items) < 1 || len(r.Results) < 1 || len(r.Items)+len(r.Results) > 3 {
		return false
	}
	for _, input := range r.Items {
		for _, output := range r.Results {
			if input == output {
				return false
			}
		}
	}
	return true
}

// appendReferenceThirdPort 职责：以同侧剩余真实槽位及0.3125层独立带扩展一个物料；使用：参考变体装配；边界：不存在空槽或容量不足时拒绝，保留原双层轨道。
func (b *builder) appendReferenceThirdPort(l *line, anchors []float64, bandY, left, right float64) error {
	input := len(l.recipe.Items) == 2
	height, portY := .3125, l.inputs[0][0].a.y-3
	if !input {
		portY = l.outputs[0][0].a.y + 3
	}
	points, err := b.referenceThirdTrack(l, referenceTrackPoints(anchors, bandY, height, portY, left, right))
	if err != nil {
		return err
	}
	track := b.byPath(points)
	if input {
		l.inputs = append(l.inputs, track)
	} else {
		l.outputs = append(l.outputs, track)
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	geometry, err := catalog.Geometry(int16(l.info.MachineID), b.models[itoa(l.info.MachineID)])
	if err != nil {
		return err
	}
	machines := map[*placed]bool{}
	for _, m := range l.machines {
		machines[m] = true
	}
	inputs, outputs := map[*placed]*placed{}, map[*placed]*placed{}
	for _, arm := range b.all {
		if arm.record.ItemID < 2011 || arm.record.ItemID > 2014 {
			continue
		}
		if machines[arm.output] {
			inputs[arm.output] = arm
		}
		if machines[arm.input] {
			outputs[arm.input] = arm
		}
	}
	for _, machine := range l.machines {
		if err := b.checkGeneration("参考密铺独立第三物料"); err != nil {
			return err
		}
		template, other := inputs[machine], outputs[machine]
		if !input {
			template, other = other, template
		}
		if template == nil || other == nil {
			return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "参考单元缺少已有物料臂"}
		}
		oldSlot, usedSlot := template.record.OutputToSlot, other.record.InputFromSlot
		if !input {
			oldSlot, usedSlot = template.record.InputFromSlot, other.record.OutputToSlot
		}
		newSlot := int8(-1)
		for slot := oldSlot / 3 * 3; slot < oldSlot/3*3+3; slot++ {
			if slot != oldSlot && slot != usedSlot {
				newSlot = slot
				break
			}
		}
		if oldSlot < 0 || newSlot < 0 || int(newSlot) >= len(geometry.SlotPoses) {
			return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "参考单元没有同侧第三个真实槽位"}
		}
		a, c := geometry.SlotPoses[oldSlot].Position, geometry.SlotPoses[newSlot].Position
		x, z := float64(c[0]-a[0]), float64(c[2]-a[2])
		angle := float64(machine.record.Yaw) * math.Pi / 180
		dx, dy := (math.Cos(angle)*x+math.Sin(angle)*z)/planarGridLength, (-math.Sin(angle)*x+math.Cos(angle)*z)/planarGridLength
		oldAnchor := template.input
		if !input {
			oldAnchor = template.output
		}
		anchor := track[0]
		nearest := math.Inf(1)
		for _, candidate := range track {
			distance := math.Abs(candidate.a.x-oldAnchor.a.x-dx) + math.Abs(candidate.a.y-bandY)*10 + math.Abs(candidate.a.z-height)*10
			if distance < nearest {
				anchor, nearest = candidate, distance
			}
		}
		arm := b.make(int(template.record.ItemID), 0, 0, 0)
		arm.record = template.record
		arm.record.Parameters = append([]int32(nil), template.record.Parameters...)
		arm.a, arm.b = template.a, template.b
		arm.a.x += dx
		arm.b.x += dx
		arm.a.y += dy
		arm.b.y += dy
		arm.densityPattern = "reference-low-arm"
		if input {
			tipRaise := height - .125
			if l.info.MachineType == 2 {
				tipRaise = .30
				if machine.a.y > bandY {
					tipRaise = .29
				}
				// 第3档第三输入臂的抬头姿态会逼近24度端点限制；降低动画端点，带高及真实货物采样点保持独立校验。
				if b.generationCtx != nil {
					zone, _ := b.generationCtx.Value(densityLatitudeKey{}).(string)
					if selection, ok := densityLatitudeSelectionFor(zone); ok && selection.tier == 3 {
						tipRaise -= .01
					}
				}
			}
			// 带端动画姿态与真实cargo高度分开：制造变体调整臂盒净空，采样目标仍只按实际带高差变化。
			arm.a.z += tipRaise
			arm.input, arm.output = anchor, machine
			arm.record.OutputToSlot = newSlot
		} else {
			arm.b.z += height - .5
			arm.input, arm.output = machine, anchor
			arm.record.InputFromSlot = newSlot
			arm.record.FilterID = int16(l.recipe.Results[1])
			template.record.FilterID = int16(l.recipe.Results[0])
		}
		target, err := denseCargoLocalTarget(template)
		if err != nil {
			return err
		}
		target.x += dx
		target.y += dy
		if input {
			target.z += height - .125
		} else {
			target.z += height - .5
		}
		setDenseCargoTarget(arm, anchor, track, target, !input)
	}
	return nil
}

// referenceThirdTrack 职责：给第三轨节点寻找不会被原版位置桶合并的相位；使用：独立多物料低层带；边界：只在有限横向窗口调整节点，保留首尾整数端口与每段连接距离。
func (b *builder) referenceThirdTrack(l *line, source []vector) ([]vector, error) {
	foreign := append(append([]*placed(nil), l.inputs[0]...), l.outputs[0]...)
	sort.Slice(foreign, func(i, j int) bool { return foreign[i].a.x < foreign[j].a.x })
	clear := func(p vector) bool {
		start := sort.Search(len(foreign), func(i int) bool { return foreign[i].a.x >= p.x-.35 })
		for i := start; i < len(foreign) && foreign[i].a.x <= p.x+.35; i++ {
			q := foreign[i].a
			dx, dy, dz := (p.x-q.x)*planarGridLength, (p.y-q.y)*planarGridLength, (p.z-q.z)*1.3333333
			if dx*dx+dy*dy+dz*dz < .35*.35 {
				return false
			}
		}
		return true
	}
	points := make([]vector, 0, len(source))
	for index, point := range source {
		if err := b.checkGeneration("密铺第三轨节点相位"); err != nil {
			return nil, err
		}
		found := false
		for step := 0; step <= 320; step++ {
			candidate := point
			if index != 0 && index != len(source)-1 {
				sign := 1.
				if step%2 == 1 {
					sign = -1
				}
				candidate.x += sign * float64((step+1)/2) * .005
			} else if step > 0 {
				break
			}
			if len(points) > 0 {
				previous := points[len(points)-1]
				distance := math.Sqrt(math.Pow((candidate.x-previous.x)*planarGridLength, 2) + math.Pow((candidate.y-previous.y)*planarGridLength, 2) + math.Pow((candidate.z-previous.z)*1.3333333, 2))
				if candidate.x <= previous.x+.01 || distance < .35 || distance > 2.15 {
					continue
				}
			}
			if clear(candidate) {
				points = append(points, candidate)
				found = true
				break
			}
		}
		if !found && (index == 0 || index == len(source)-1) {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "第三物料轨道无法保持独立节点与安全接线距离"}
		}
	}
	return points, nil
}
