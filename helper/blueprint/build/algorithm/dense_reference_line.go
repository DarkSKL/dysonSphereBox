package build

// 文件职责：根据用户参考图的两排机器与分拣器姿态生成低双层带单元。
// 使用：已覆盖原型的至多三种机器物料密铺；边界：保留参考臂姿态，按真实cargo目标重标定偏移；额外物料采用独立第三槽变体，喷涂在地面外端口接入。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/density"
	"DysonSphereBox/helper/quant"
)

// referenceProjector 职责：把参考区域0姿态投影到首台机器的局部切平面；使用：提取可重复局部关系；边界：只支持已锁定的原版200分段参考图，投影后的候选仍需最终球面校验。
type referenceProjector struct{ origin, east, north [3]float64 }

// newReferenceProjector 职责：建立来源坐标系；使用：单元装配前；边界：参考主锚点按研究基线0解释，不宣称任意纬带平移等价。
func newReferenceProjector(point blueprint.Point) referenceProjector {
	lon, lat := float64(point.X)*2*math.Pi/1000, float64(point.Y)*2*math.Pi/1000
	radius := 200.2 + float64(point.Z)*1.3333333
	return referenceProjector{origin: [3]float64{radius * math.Cos(lat) * math.Sin(lon), radius * math.Sin(lat), -radius * math.Cos(lat) * math.Cos(lon)}, east: [3]float64{math.Cos(lon), 0, math.Sin(lon)}, north: [3]float64{-math.Sin(lat) * math.Sin(lon), math.Cos(lat), math.Sin(lat) * math.Cos(lon)}}
}

// project 职责：保存参考结构的局部水平距离及原始高度；使用：机器、臂和带锚点；边界：不对小数坐标取整，微小地面负高度统一到零。
func (frame referenceProjector) project(point blueprint.Point) vector {
	lon, lat := float64(point.X)*2*math.Pi/1000, float64(point.Y)*2*math.Pi/1000
	radius := 200.2 + float64(point.Z)*1.3333333
	world := [3]float64{radius * math.Cos(lat) * math.Sin(lon), radius * math.Sin(lat), -radius * math.Cos(lat) * math.Cos(lon)}
	result := vector{z: float64(point.Z)}
	for axis, value := range world {
		result.x += (value - frame.origin[axis]) * frame.east[axis] / planarGridLength
		result.y += (value - frame.origin[axis]) * frame.north[axis] / planarGridLength
	}
	if math.Abs(result.z) < .01 {
		result.z = 0
	}
	return result
}

// referenceLowTrackEligible 职责：选择已有真实端口来源的低双层单元；使用：密铺机器装配；边界：其它等级必须再核对建造盒及槽位一致性，不暗改用户设备等级。
func (b *builder) referenceLowTrackEligible(p quant.Production, r quant.Recipe) bool {
	return b.adapter != nil && b.adapter.density == densityDense && referenceLowRecipe(p, r) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv)
}

// referenceTrack 职责：建立带来源锚点的单物料低层轨道及整数地面外端口；使用：参考密铺；边界：小于0.2格的锚点统一复用节点，引用由调用方重新绑定，不重复创建将被游戏合并的节点。
func referenceTrackPoints(anchors []float64, y, z, portY, machineLeft, machineRight float64) []vector {
	sort.Float64s(anchors)
	start, end := math.Min(math.Floor(anchors[0])-2, machineLeft-2), math.Max(math.Ceil(anchors[len(anchors)-1])+2, machineRight+2)
	points := []vector{{0, portY, 0}}
	appendTo := func(target vector) {
		from := points[len(points)-1]
		steps := max(1, int(math.Ceil(math.Max(math.Abs(target.x-from.x), math.Max(math.Abs(target.y-from.y), math.Abs(target.z-from.z))))))
		for step := 1; step <= steps; step++ {
			if step == steps {
				points = append(points, target)
				continue
			}
			fraction := float64(step) / float64(steps)
			points = append(points, vector{from.x + (target.x-from.x)*fraction, from.y + (target.y-from.y)*fraction, from.z + (target.z-from.z)*fraction})
		}
	}
	if start > 5 {
		appendTo(vector{4, portY, 0})
	}
	appendTo(vector{start, y, z})
	for _, x := range anchors {
		if x-points[len(points)-1].x > .2 {
			appendTo(vector{x, y, z})
		}
	}
	appendTo(vector{end, y, z})
	appendTo(vector{end + 6, portY, 0})
	return points
}

// referenceTrack 职责：把局部参考轨道落实为独立实体链；使用：输入与输出；边界：各物料不共享节点。
func (b *builder) referenceTrack(anchors []float64, y, z, portY, left, right float64) []*placed {
	return b.byPath(referenceTrackPoints(anchors, y, z, portY, left, right))
}

// buildReferenceLowLine 职责：参数化来源双排、同侧进出臂和0.125/0.500低层轨道；使用：四模式共同局部单元；边界：保留设备等级和物料需求，按明确行距/相位生成变体，最终重新绑定货物采样。
func (b *builder) buildReferenceLowLine(p quant.Production, r quant.Recipe, count, mode, lineIndex int, last bool) (*line, error) {
	unit, _, ok, err := density.Reference(denseReferenceSource(p.MachineID))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "缺少该原型的低双层带参考"}
	}
	byIndex := map[int32]blueprint.Building{}
	for _, record := range unit.Records {
		byIndex[record.Index] = record
	}
	if len(unit.MachineIndices) != 2 {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "参考单元机器模型或数量不匹配"}
	}
	if err := b.validateReferencePrototype(p.MachineID, unit); err != nil {
		return nil, err
	}
	frame := newReferenceProjector(byIndex[unit.MachineIndices[0]].LocalOffset)
	cargoTargets, err := referenceCargoTargets(unit, frame)
	if err != nil {
		return nil, err
	}
	tightPorts := b.denseSpatial && p.IncreasePoints == 0 && len(r.Results) == 1
	originX := 8.
	if tightPorts {
		originX = 5
	}
	if p.IncreasePoints > 0 {
		originX = 12
	}
	pitch, err := referenceLineSpacing(p.MachineType, b.setting.extraSpace(p.MachineType))
	if err != nil {
		return nil, err
	}
	rowMargin, secondRowX := .04, 0.
	if len(r.Items)+len(r.Results) == 3 {
		rowMargin = 1.2
		if p.MachineType == 2 && len(r.Items) == 2 {
			rowMargin = .4
		}
		if p.MachineType == 4 {
			rowMargin = .04
			secondRowX = pitch / 4
		}
	}
	// 高纬化工盒的长边跨过较大的球面弧度；为双排低轨保留局部净空，避免扩大整厂间距。
	if b.generationCtx != nil && p.MachineType == 4 {
		zone, _ := b.generationCtx.Value(densityLatitudeKey{}).(string)
		if selection, ok := densityLatitudeSelectionFor(zone); ok && selection.tier >= 4 {
			rowMargin += .3
			pitch += .15
		}
	}
	columns := (count + 1) / 2
	anchors := [2][]float64{}
	bandY := 0.
	bandCount := 0
	for row, index := range unit.MachineIndices {
		for _, arm := range unit.Records {
			if arm.ItemID < 2011 || arm.ItemID > 2014 || arm.InputObjIdx != index && arm.OutputObjIdx != index {
				continue
			}
			track, belt := 0, arm.InputObjIdx
			if arm.InputObjIdx == index {
				track, belt = 1, arm.OutputObjIdx
			}
			point := frame.project(byIndex[belt].LocalOffset)
			bandY += point.y
			bandCount++
			for column := 0; column < columns; column++ {
				if column*2+row < count {
					anchors[track] = append(anchors[track], originX+float64(column)*pitch+point.x+float64(row)*secondRowX)
				}
			}
		}
	}
	if bandCount == 0 || len(anchors[0]) == 0 || len(anchors[1]) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "参考图缺少机器真实带端点"}
	}
	bandY /= float64(bandCount)
	// 制造双输入通过第三臂带端高度留出0.4格附加行距；化工双输入/双产物采用第二排四分之一周期错位，仅保留0.04格数值余量。
	bandY += rowMargin / 2
	lo, hi := footprint(int16(p.MachineID))
	machineLeft, machineRight := math.Inf(1), math.Inf(-1)
	for number := 0; number < count; number++ {
		offset := frame.project(byIndex[unit.MachineIndices[number%2]].LocalOffset)
		x := originX + float64(number/2)*pitch + offset.x + float64(number%2)*secondRowX
		machineLeft = math.Min(machineLeft, math.Floor(x+lo.x))
		machineRight = math.Max(machineRight, math.Ceil(x+hi.x))
	}
	begin := len(b.all)
	l := &line{recipe: r, info: p, densePaired: count > 1, referenceLowTracks: true}
	inputY, outputY := -3., math.Ceil(bandY)+4
	if tightPorts {
		inputY, outputY = math.Floor(bandY)-1, math.Ceil(bandY)+1
	}
	l.inputs = [][]*placed{b.referenceTrack(anchors[0], bandY, .125, inputY, machineLeft, machineRight)}
	l.outputs = [][]*placed{b.referenceTrack(anchors[1], bandY, .5, outputY, machineLeft, machineRight)}
	l.output = l.outputs[0]
	for number := 0; number < count; number++ {
		if err := b.checkGeneration("参考低双层带密铺单元"); err != nil {
			return nil, err
		}
		row, column := number%2, number/2
		source := byIndex[unit.MachineIndices[row]]
		position := frame.project(source.LocalOffset)
		position.y += float64(row) * rowMargin
		shiftX := originX + float64(column)*pitch + float64(row)*secondRowX
		machine := b.make(p.MachineID, position.x+shiftX, position.y, 0)
		machine.record.Yaw, machine.record.Yaw2 = source.Yaw, source.Yaw2
		machine.record.RecipeID = int16(p.RecipeID)
		machine.record.Parameters = []int32{int32(max(0, p.ProductionMode))}
		machine.densityPattern = "reference-pair"
		if count == 1 {
			machine.densityPattern = "reference-single"
		}
		l.machines = append(l.machines, machine)
		for _, sourceArm := range unit.Records {
			if sourceArm.ItemID < 2011 || sourceArm.ItemID > 2014 || sourceArm.InputObjIdx != source.Index && sourceArm.OutputObjIdx != source.Index {
				continue
			}
			output := sourceArm.InputObjIdx == source.Index
			track, beltIndex := l.inputs[0], sourceArm.InputObjIdx
			if output {
				track, beltIndex = l.outputs[0], sourceArm.OutputObjIdx
			}
			want := frame.project(byIndex[beltIndex].LocalOffset)
			want.x += shiftX
			anchor := track[0]
			distance := math.Inf(1)
			for _, node := range track {
				candidate := math.Abs(node.a.x-want.x) + math.Abs(node.a.y-bandY)*10 + math.Abs(node.a.z-want.z)*10
				if candidate < distance {
					anchor, distance = node, candidate
				}
			}
			arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
			model := arm.record.ModelIndex
			arm.record = sourceArm
			arm.record.ItemID = int16(2010 + b.setting.InserterLv)
			arm.record.ModelIndex = model
			arm.record.Parameters = append([]int32(nil), sourceArm.Parameters...)
			arm.a, arm.b = frame.project(sourceArm.LocalOffset), frame.project(sourceArm.LocalOffset2)
			arm.a.x += shiftX
			arm.b.x += shiftX
			arm.a.y += float64(row) * rowMargin
			arm.b.y += float64(row) * rowMargin
			arm.densityPattern = "reference-low-arm"
			arm.input, arm.output = anchor, machine
			if output {
				arm.input, arm.output = machine, anchor
			}
			target, exists := cargoTargets[sourceArm.Index]
			if !exists {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "参考分拣器缺少真实货物采样目标"}
			}
			sourceBelt := frame.project(byIndex[beltIndex].LocalOffset)
			target.x += shiftX
			target.y += bandY - sourceBelt.y
			height := .125
			if output {
				height = .5
			}
			target.z += height - sourceBelt.z
			setDenseCargoTarget(arm, anchor, track, target, output)
		}
	}
	if len(r.Items)+len(r.Results) == 3 {
		anchorIndex := 0
		if len(r.Results) == 2 {
			anchorIndex = 1
		}
		if err := b.appendReferenceThirdPort(l, anchors[anchorIndex], bandY, machineLeft, machineRight); err != nil {
			return nil, err
		}
	}
	if p.IncreasePoints > 0 {
		for _, row := range l.inputs {
			coater := b.make(2313, 2, row[0].a.y, 0)
			coater.record.OutputToSlot = 14
			coater.record.InputFromSlot = 15
			coater.record.OutputFromSlot = 15
			coater.record.InputToSlot = 14
			l.coaters = append(l.coaters, coater)
			for _, arm := range b.all[begin:] {
				if arm.densityCargo.enabled && !arm.densityCargo.output && len(arm.densityCargo.track) > 0 && arm.densityCargo.track[0] == row[0] {
					arm.densityCargo.coater = coater
				}
			}
		}
	}
	if b.setting.EnableFlowMonitor {
		// 低层多轨之间放不下检测器时，为每种物料提供真实的短地面直线宿主，不用其它物料共带冒充覆盖。
		extend := func(row []*placed) []*placed {
			head := row[0]
			prefix := b.straight(head.a.x-3, head.a.y, 3, 2)
			connect(prefix[len(prefix)-1], head)
			return append(prefix, row...)
		}
		for index, row := range l.inputs {
			l.inputs[index] = extend(row)
		}
		for index, row := range l.outputs {
			l.outputs[index] = extend(row)
		}
		l.output = l.outputs[0]
	}
	if b.adapter.lineAssembly.reverseOutputRows {
		for _, row := range l.outputs {
			reverse(row)
		}
	}
	if b.adapter.lineAssembly.writeInputParameters {
		for i, row := range l.inputs {
			row[0].record.Parameters = []int32{int32(r.Items[i]), 0}
		}
	}
	for i, row := range l.outputs {
		row[0].record.Parameters = []int32{int32(r.Results[i]), 0}
	}
	if b.setting.EnableFlowMonitor {
		if err := b.addLineFlowMonitors(l, begin); err != nil {
			return nil, err
		}
	}
	if b.logistics != nil {
		message := fmt.Sprintf("原型%d参考局部几何：重复节距%.3f格，附加行距%.3f格，第二排横向错位%.3f格；独立物料变体不宣称与原混带完全等价", p.MachineID, pitch, rowMargin, secondRowX)
		found := false
		for _, check := range b.logistics.Checks {
			if check.RuleID == "reference-local-spacing" && check.Message == message {
				found = true
				break
			}
		}
		if !found {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "reference-local-spacing", Status: "passed", Message: message})
		}
	}
	return b.finishLocalLine(l, geometry{}, p, mode, lineIndex, last, begin)
}
