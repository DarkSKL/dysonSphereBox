package build

// 文件职责：装配来源可追踪的炼油厂同侧三槽与1.5格地面带。
// 使用：无喷涂、无监测的等离子精炼密铺候选；边界：保留机器等级、总数和独立物流，最终仍逐项核验原生几何及货物采样。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/density"
	"DysonSphereBox/helper/quant"
	"fmt"
	"math"
)

// denseRefineryPitch 职责：为Yaw零度炼油厂保留实际横向建造盒净距；使用：行装配及细分预算；边界：仍叠加用户额外间距，并接受最终球面核验。
const denseRefineryPitch = 3.15

// refinerySameSideKey 职责：选择极区候选的局部炼油拓扑；使用：同侧与分侧密铺比较；边界：不更换密度模式、机器等级或物料计划。
type refinerySameSideKey struct{}

// refineryCollinearKey 职责：为独立候选启用同线错相三轨；使用：塔模式黑盒比较；边界：既有同侧三轨与分侧候选继续保留。
type refineryCollinearKey struct{}

// denseRefineryCargoBand 职责：在已核对的2308槽位外保留合法短臂及带节点净空；使用：同线错相候选；边界：只适用于当前等离子精炼原型，最终仍复核实际跨度和建造盒。
const denseRefineryCargoBand = 3.75

// refineryLayoutVariants 职责：只为本次支持的炼油产线保留两种真实局部拓扑；使用：极区候选；边界：新拓扑不适合边缘时仍须对分侧拓扑进行同样的完整终检。
func refineryLayoutVariants(req Request) []bool {
	if req.Setting.EnableFlowMonitor {
		return []bool{true}
	}
	for _, production := range req.Plan.Productions {
		if production.MachineID == 2308 && production.RecipeID == 16 && production.IncreasePoints == 0 {
			return []bool{true, false}
		}
	}
	return []bool{true}
}

// denseRefineryEligible 职责：只选择来源已覆盖的三物料单臂配方；使用：局部装配与行预算；边界：不提升分拣器、不伪造其它配方或附属设施。
func (b *builder) denseRefineryEligible(p quant.Production, r quant.Recipe) bool {
	if b.generationCtx != nil {
		if enabled, specified := b.generationCtx.Value(refinerySameSideKey{}).(bool); specified && !enabled {
			return false
		}
	}
	return b.adapter != nil && b.adapter.density == densityDense && p.MachineID == 2308 && p.IncreasePoints == 0 && !b.setting.EnableFlowMonitor && len(r.Items) == 1 && r.Items[0] == 1007 && len(r.Results) == 2 && ((r.Results[0] == 1114 && r.Results[1] == 1120) || (r.Results[0] == 1120 && r.Results[1] == 1114)) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv)
}

// buildDenseRefineryLine 职责：重复来源机器及三根独立取放臂；使用：密铺局部候选；边界：旧offset仅用于还原目标，新轨必须重新绑定真实货物采样。
func (b *builder) buildDenseRefineryLine(p quant.Production, r quant.Recipe, count, mode, lineIndex int, last bool) (*line, error) {
	unit, err := density.RefineryReference()
	if err != nil {
		return nil, err
	}
	source := unit.Records[0]
	if source.ItemID != 2308 || source.ModelIndex != b.models["2308"] {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "炼油参考原型不匹配"}
	}
	frame := newReferenceProjector(source.LocalOffset)
	goals, err := referenceCargoTargets(unit, frame)
	if err != nil {
		return nil, err
	}
	footprint, err := b.refineryLayoutFootprint()
	if err != nil {
		return nil, err
	}
	step, err := b.refineryBeltStep()
	if err != nil {
		return nil, err
	}
	collinear := false
	if b.generationCtx != nil {
		collinear, _ = b.generationCtx.Value(refineryCollinearKey{}).(bool)
	}
	begin := len(b.all)
	l := &line{recipe: r, info: p, referenceLowTracks: true}
	originX, originY, pitch := 4., 6., denseRefineryPitch+float64(b.setting.RefineryOtherSpace)
	armSources := make([]blueprint.Building, 3)
	for _, arm := range unit.Records {
		if arm.ItemID < 2011 || arm.ItemID > 2014 {
			continue
		}
		slot := 0
		if arm.InputObjIdx == source.Index {
			slot = 1
			if arm.FilterID == 1120 {
				slot = 2
			}
		}
		armSources[slot] = arm
	}
	byIndex := map[int32]blueprint.Building{}
	for _, record := range unit.Records {
		byIndex[record.Index] = record
	}
	tracks := make([][]*placed, 3)
	for slot, arm := range armSources {
		index := arm.InputObjIdx
		if slot > 0 {
			index = arm.OutputObjIdx
		}
		anchor := frame.project(byIndex[index].LocalOffset)
		start, end := originX-2, originX+float64(count-1)*pitch+1
		y := originY + anchor.y
		if collinear {
			// 三条独立带沿同一纬线错开1/3节点周期；端点保留同一相位，不能均分短尾后让异物料节点相撞。
			// 比原轨多保留两格入口直线，避免整厂接线的转弯侵入首台机器的货物采样窗口。
			start -= 2
			phase := float64(slot) * step / 3
			end = start + math.Ceil((end-start)/step)*step + phase
			start += phase
			y = originY - denseRefineryCargoBand
		}
		points := refineryTrackPoints(start, end, y, step)
		tracks[slot] = b.byPath(points)
	}
	l.inputs = [][]*placed{tracks[0]}
	l.outputs = make([][]*placed, len(r.Results))
	for i, item := range r.Results {
		slot := 1
		if item == 1120 {
			slot = 2
		}
		l.outputs[i] = tracks[slot]
	}
	l.output = l.outputs[0]
	for i := 0; i < count; i++ {
		dx := originX + float64(i)*pitch
		machine := b.make(2308, dx, originY, 0)
		machine.record.Yaw, machine.record.Yaw2 = 0, 0
		machine.record.RecipeID = int16(p.RecipeID)
		machine.record.Parameters = []int32{int32(max(0, p.ProductionMode))}
		machine.layoutFootprint = footprint
		machine.densityPattern = "refinery-same-side"
		if count == 1 {
			machine.densityPattern = "refinery-same-side-single"
		}
		l.machines = append(l.machines, machine)
		for slot, original := range armSources {
			track := tracks[slot]
			sourceBelt := original.InputObjIdx
			if slot > 0 {
				sourceBelt = original.OutputObjIdx
			}
			point := frame.project(byIndex[sourceBelt].LocalOffset)
			point.x += dx
			point.y += originY
			deltaY := 0.
			if collinear {
				deltaY = originY - denseRefineryCargoBand - point.y
				point.y += deltaY
			}
			anchor := track[0]
			distance := math.Inf(1)
			for _, node := range track {
				d := math.Abs(node.a.x - point.x)
				if d < distance {
					anchor, distance = node, d
				}
			}
			arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
			model := arm.record.ModelIndex
			arm.record = original
			arm.record.ItemID = int16(2010 + b.setting.InserterLv)
			arm.record.ModelIndex = model
			arm.record.Parameters = append([]int32(nil), original.Parameters...)
			arm.a, arm.b = frame.project(original.LocalOffset), frame.project(original.LocalOffset2)
			arm.a.x += dx
			arm.b.x += dx
			arm.a.y += originY
			arm.b.y += originY
			if collinear {
				if slot == 0 {
					arm.a.y += deltaY
				} else {
					arm.b.y += deltaY
				}
				arm.record.Parameters = []int32{int32(math.Round(math.Hypot(arm.b.x-arm.a.x, arm.b.y-arm.a.y)))}
			}
			arm.densityPattern = "reference-low-arm"
			arm.input, arm.output = anchor, machine
			if slot > 0 {
				arm.input, arm.output = machine, anchor
			}
			goal := goals[original.Index]
			goal.x += dx
			goal.y += originY + deltaY
			setDenseCargoTarget(arm, anchor, track, goal, slot > 0)
		}
	}
	if b.adapter.lineAssembly.reverseOutputRows {
		for _, row := range l.outputs {
			reverse(row)
		}
	}
	for i, row := range l.inputs {
		row[0].record.Parameters = []int32{int32(r.Items[i]), 0}
	}
	for i, row := range l.outputs {
		row[0].record.Parameters = []int32{int32(r.Results[i]), 0}
	}
	if b.logistics != nil {
		if collinear {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "refinery-collinear-lanes", Status: "passed", Message: "等离子精炼使用同线错相的三条独立地面带，保留首台取放缓冲；各轨独立接线，实际跨度与逐物料能力随后复核，不放宽节点合并或货物误差限制"})
		}
		found := false
		for _, check := range b.logistics.Checks {
			found = found || check.RuleID == "refinery-same-side-reference"
		}
		if !found {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "refinery-same-side-reference", Status: "passed", Message: fmt.Sprintf("炼油厂同侧三独立轨：按实际原型核对槽位与货物来源，完整几何随后终检；来源 %s，SHA256 %s", density.RefineryReferenceURL, density.RefineryReferenceSHA256)})
		}
	}
	return b.finishLocalLine(l, geometry{}, p, mode, lineIndex, last, begin)
}

// refineryBeltStep 职责：在原生纬带补偿前收紧局部带长；使用：同侧三轨；边界：极区跨环由后续真实球面重采样处理，不能用平面1.5格放宽原生米制限制。
func (b *builder) refineryBeltStep() (float64, error) {
	const maximum = 1.5
	if b.generationCtx == nil {
		return maximum, nil
	}
	if root, _ := b.generationCtx.Value(densityPolarRootKey{}).(bool); root {
		return maximum, nil
	}
	zone, _ := b.generationCtx.Value(densityLatitudeKey{}).(string)
	selection, selected := densityLatitudeSelectionFor(zone)
	if !selected || selection.tier < 4 {
		return maximum, nil
	}
	band, _, err := densityContextBand(b.generationCtx, zone, 200)
	if err != nil {
		return 0, err
	}
	// 编码按最靠极点的一端补偿经向格距；原带在靠赤道的一端会变长。
	// 包络取整再预留一格，使用整个候选带内最不利的米制长度。
	near := max(0, min(math.Abs(float64(band.low)), math.Abs(float64(band.high)))-1)
	far := max(math.Abs(float64(band.low)), math.Abs(float64(band.high))) + 1
	meters := planarGridLength * 200.2 / 200 * math.Cos(near*math.Pi/500) / math.Cos(far*math.Pi/500)
	step := min(maximum, math.Sqrt(5.3)/meters*.99)
	if math.IsNaN(step) || step < .7/planarGridLength {
		return 0, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选子环在当前炼油厂带长预算内没有合法节点格距"}
	}
	return step, nil
}

// refineryTrackPoints 职责：以不超过1.5格的主体带覆盖完整取放窗口；使用：三条平行地面轨；边界：末段不足0.35米净距时仅均分最后两段，不截断窗口。
func refineryTrackPoints(start, end, y, step float64) []vector {
	points := []vector{{start, y, 0}}
	for x := start + step; x < end-1e-8; x += step {
		points = append(points, vector{x, y, 0})
	}
	gap := end - points[len(points)-1].x
	if gap < .35/planarGridLength && len(points) > 1 {
		points = points[:len(points)-1]
		previous := points[len(points)-1]
		points = append(points, vector{(previous.x + end) / 2, y, 0})
	}
	points = append(points, vector{end, y, 0})
	return points
}

// refineryLayoutFootprint 职责：从实际建造盒提取Yaw零度外包；使用：长轴已旋转的炼油厂宽高预算；边界：查表失败拒绝装配，不使用旧横向模板的短纬向估算。
func (b *builder) refineryLayoutFootprint() (*[2]vector, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	shapes, err := catalog.Shapes(2308, b.models["2308"])
	if err != nil {
		return nil, err
	}
	low, high := vector{math.Inf(1), math.Inf(1), 0}, vector{math.Inf(-1), math.Inf(-1), 0}
	encoding := newPlanarRuleEncoding()
	for _, shape := range shapes {
		box, err := encoding.colliderBox(vector{0, .2, 0}, 0, shape)
		if err != nil {
			return nil, err
		}
		a, c := box.Bounds()
		low.x = math.Min(low.x, a[0]/planarGridLength)
		low.y = math.Min(low.y, a[2]/planarGridLength)
		high.x = math.Max(high.x, c[0]/planarGridLength)
		high.y = math.Max(high.y, c[2]/planarGridLength)
	}
	return &[2]vector{low, high}, nil
}

// layoutDenseSpatialRows 职责：使用实际获选局部模板的重复节距细分容量组；使用：直连、配送和塔候选；边界：各原单带组独立守恒，不改变产线总数或带速。
func (b *builder) layoutDenseSpatialRows(p quant.Production, r quant.Recipe, counts []int, maxSpan float64) ([]int, error) {
	if b.referenceChemicalFourEligible(p, r) {
		pitch, err := referenceLineSpacing(p.MachineType, b.setting.extraSpace(p.MachineType))
		if err != nil {
			return nil, err
		}
		limit := 2 * max(1, int(math.Floor(maxSpan/pitch))+1)
		result := []int{}
		for _, count := range counts {
			if count < 1 {
				return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "化工容量组缺少真实机器"}
			}
			parts := (count + limit - 1) / limit
			for i := 0; i < parts; i++ {
				n := count / parts
				if i < count%parts {
					n++
				}
				result = append(result, n)
			}
		}
		return result, nil
	}
	if !b.denseRefineryEligible(p, r) {
		return denseSpatialRowCounts(p, r, b.setting, counts, maxSpan)
	}
	limit := max(1, int(math.Floor(maxSpan/(denseRefineryPitch+float64(b.setting.RefineryOtherSpace))))+1)
	result := []int{}
	for _, count := range counts {
		if count < 1 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "炼油厂容量组缺少真实机器"}
		}
		parts := (count + limit - 1) / limit
		for i := 0; i < parts; i++ {
			n := count / parts
			if i < count%parts {
				n++
			}
			result = append(result, n)
		}
	}
	return result, nil
}
