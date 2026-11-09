package build

// 文件职责：构建与运输容量分组解耦的双排机器街道原型，保留独立低轨、真实采样和逐组容量。
// 使用：密铺适配器在全场接线前调用；边界：裸入口不包含喷涂/监测，显式附件入口独立保留这些设备；外围接驳和供电完成前不能作为整厂结果。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/density"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// denseStreetFoldClearanceKey 职责：携带完整候选的真实折排间隙；使用：密芯供电无解后试探必要净空；边界：仅增加实体间距，不改变原型或供电规则。
type denseStreetFoldClearanceKey struct{}

// denseStreetPort 职责：公开容量段的真实低轨端口；使用：调用方创建分数坐标到全场网格的实体接驳；边界：不能用坐标标记代替连续带。
type denseStreetPort struct {
	// group/item/input 职责：标识原始容量组与物料方向；使用：绑定规范物料图；边界：组下标保持调用方counts顺序。
	group, item int
	input       bool
	// head/inner 职责：保存端口与相邻内侧带节点；使用：移动后确定向外方向；边界：两者始终属于同一物料链。
	head, inner *placed
	// requiredRate/capacity 职责：记录该组满负荷需求及所选带容量；使用：接驳能力检查；边界：不假定货物堆叠或额外科技。
	requiredRate, capacity float64
}

// outward 职责：按当前实体位置给出端口向外方向；使用：平移旋转后的接驳；边界：仅为单位水平方向，不生成带。
func (port denseStreetPort) outward() vector {
	delta := vector{port.head.a.x - port.inner.a.x, port.head.a.y - port.inner.a.y, 0}
	length := math.Hypot(delta.x, delta.y)
	if length == 0 {
		return vector{}
	}
	return vector{delta.x / length, delta.y / length, 0}
}

// denseStreetSeam 职责：保存相邻容量段的实际接续证据；使用：诊断必要间距和独立带相位；边界：数值是有界候选中首个通过值，不声称数学最小值。
type denseStreetSeam struct {
	// group 职责：定位本接续段；使用：回连原始counts；边界：不是空间排序号。
	group int
	// gap/phase 职责：记录机器额外间距与仅带节点的横向偏移；使用：复现实验；边界：相位不移动机器、臂或货物目标。
	gap, phase float64
	// rotated 职责：标识奇数组为填满两排使用了180度刚性旋转；使用：端口方向解释；边界：不交换物料。
	rotated bool
	// street/baseY 职责：记录折排街道及其实际小数基线；使用：复现紧凑矩形；边界：不是固定区块高度。
	street int
	baseY  float64
}

// denseStreet 职责：保存已通过局部几何和采样检查的街道；使用：适配器接入阶段；边界：未接全场运输或供电。
type denseStreet struct {
	// groups 职责：保存各容量组的实际产线；使用：连同all接入transportRow与机器终检登记；边界：按输入counts下标返回，不能只接管实体而丢掉产线元数据。
	groups []*line
	// all 职责：保存机器、臂及必要低轨；使用：整体落位和最终编号；边界：不包含已裁掉的地面坡道。
	all []*placed
	// ports 职责：保存所有独立物料端口；使用：后续连续接驳；边界：不能合并不同容量组的节点身份。
	ports []denseStreetPort
	// seams 职责：保存各组接续的几何选择；使用：审阅间距；边界：不代表游戏实测。
	seams []denseStreetSeam
	// beltNodesBefore/beltNodesAfter 职责：统计原低轨单元与街道核心的带数；使用：解释移除地面长头的收益；边界：后续外围接驳尚未计入after。
	beltNodesBefore, beltNodesAfter int
	// streetCount 职责：记录折排后的双排街道数；使用：防止只用长条换取密度；边界：各容量段依旧独立。
	streetCount int
	// checks 职责：保留被选中参考装配的真实局部间距证据；使用：scratch接管时转移诊断；边界：不复用试探过程的cargo成功日志。
	checks []LogisticsCheck
	// accessories 职责：保存显式附件入口拆出的独立模块；使用：构造assembly；边界：裸入口为空，附件不属于核心all。
	accessories []*denseStreetAccessory
}

// buildDenseStreet 职责：在独立候选中创建连续机器周期与断开的容量段；使用：无喷涂/监测的参考机器街道；边界：成功前不改变调用方，最终仍需全场路由、供电和球面验收。
func (b *builder) buildDenseStreet(ctx context.Context, p quant.Production, r quant.Recipe, counts []int) (*denseStreet, error) {
	return b.selectDenseStreet(ctx, p, r, counts, false)
}

// selectDenseStreet 职责：在明确构造阶段中比较矩形核心；使用：裸街道和附件入口；边界：阶段标志不改变可信生产数据或完整终检。
func (b *builder) selectDenseStreet(ctx context.Context, p quant.Production, r quant.Recipe, counts []int, attachments bool) (*denseStreet, error) {
	limits := []int{1, 2, 3, len(counts)}
	if attachments && b.denseStreetGroupLimit != 0 {
		limit := b.denseStreetGroupLimit
		if limit < 0 {
			limit = len(counts)
		}
		limits = []int{min(len(counts), limit)}
	}
	seen := map[int]bool{}
	var best *denseStreet
	bestScore := math.Inf(1)
	var lastError error
	for _, limit := range limits {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if limit < 1 || limit > len(counts) || seen[limit] {
			continue
		}
		seen[limit] = true
		candidate, err := b.buildDenseStreetRows(ctx, p, r, counts, limit, attachments)
		if err != nil {
			lastError = err
			continue
		}
		low, high := bounds(candidate.all)
		width, height := high.x-low.x, high.y-low.y
		if width <= 0 || height <= 0 {
			continue
		}
		aspect := math.Max(width, height) / math.Min(width, height)
		score := width * height * math.Max(1, aspect/1.8)
		if best == nil || score < bestScore {
			best, bestScore = candidate, score
		}
	}
	if best != nil {
		return best, nil
	}
	if lastError != nil {
		return nil, lastError
	}
	return nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: "街道原型需要至少两个有效容量组"}
}

// buildDenseStreetRows 职责：按指定容量段数折排连续街道；使用：矩形候选比较；边界：折排间距来自实际机器盒，不使用普通区块外包尺寸。
func (b *builder) buildDenseStreetRows(ctx context.Context, p quant.Production, r quant.Recipe, counts []int, perStreet int, accessoryStage ...bool) (*denseStreet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, denseCollisionKey{}, true)
	foldClearance := .04
	if value, ok := ctx.Value(denseStreetFoldClearanceKey{}).(float64); ok {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < .04 || value > 1 {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: "街道折排净空候选超出已声明范围"}
		}
		foldClearance = value
	}
	attachments := len(accessoryStage) > 0 && accessoryStage[0]
	if b.adapter == nil || b.adapter.density != densityDense || !referenceLowRecipe(p, r) || !referenceSingleArmsSufficient(p, r, b.setting.InserterLv) || !attachments && (p.IncreasePoints > 0 || b.setting.EnableFlowMonitor || len(counts) < 2) || len(counts) < 1 || p.MachineCount < 2 || perStreet < 1 || perStreet > len(counts) {
		message := "裸街道需要至少两个参考容量组，无喷涂/监测且所选分拣器能力充足"
		if attachments {
			message = "附件街道需要有效参考容量组、至少两台实际机器，并满足所选分拣器的真实能力"
		}
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: message}
	}
	ratesIn, ratesOut, err := machineInserterRates(p, r)
	if err != nil {
		return nil, err
	}
	capacity, err := transport.BeltCapacityPerMinute(b.setting.BeltLv)
	if err != nil {
		return nil, err
	}
	total := 0
	for group, count := range counts {
		if count < 1 || count > 10000 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "街道容量组机器数无效"}
		}
		total += count
		for _, rates := range [][]float64{ratesIn, ratesOut} {
			for _, rate := range rates {
				if rate*float64(count) > capacity+1e-6 {
					return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("街道容量组%d的%.3f/min超过单带%.3f/min", group, rate*float64(count), capacity)}
				}
			}
		}
	}
	if total != p.MachineCount {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "街道容量组机器总数与可信计划不一致"}
	}
	work, err := b.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	// 使用本地副本仅固定参考街道的带方向与附属设施策略，不回查全局注册表，避免生产分派的包初始化环。
	localAdapter := *work.adapter
	localAdapter.lineAssembly = lineAssemblyCapabilities{reverseOutputRows: true, writeInputParameters: true}
	work.adapter = &localAdapter
	disabled := false
	work.setting.NeedPower = &disabled
	work.owner, work.generationCtx, work.preview = b.owner, ctx, nil
	work.logistics = &LogisticsSummary{}
	work.denseSpatial = true
	pitch, err := referenceLineSpacing(p.MachineType, b.setting.extraSpace(p.MachineType))
	if err != nil {
		return nil, err
	}
	// 两台只读形状探针给出当前模板的两排相位，不把探针实体加入返回库存。
	probe, err := work.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	probe.owner, probe.generationCtx, probe.denseSpatial = work.owner, ctx, true
	probeLine, err := probe.buildReferenceLowLine(p, r, 2, 1, 0, true)
	if err != nil {
		return nil, err
	}
	basis := vector{probeLine.machines[1].a.x - probeLine.machines[0].a.x, probeLine.machines[1].a.y - probeLine.machines[0].a.y, 0}
	street := &denseStreet{groups: make([]*line, len(counts))}
	order := make([]int, len(counts))
	for i := range order {
		order[i] = i
	}
	// 偶数组先完整填列，奇数组随后交替旋转补齐另一排；容量组身份与台数不变。
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]]%2 < counts[order[j]]%2 })
	rowCounts := [2]int{}
	extraX := 0.
	baseY := 0.
	for number, group := range order {
		if err := work.checkGeneration("机器街道容量段"); err != nil {
			return nil, err
		}
		l, err := work.buildReferenceLowLine(p, r, counts[group], 1, 0, true)
		if err != nil {
			return nil, err
		}
		for _, entity := range l.all {
			if surfaceIsBelt(entity.record.ItemID) {
				street.beltNodesBefore++
			}
		}
		var modules []*denseStreetAccessory
		if attachments {
			modules, err = work.detachDenseStreetAccessories(l, group)
			if err != nil {
				return nil, err
			}
			street.accessories = append(street.accessories, modules...)
		}
		if err := work.trimDenseStreetTracks(l); err != nil {
			return nil, fmt.Errorf("街道容量组%d：%w", group, err)
		}
		for _, module := range modules {
			rows := l.inputs
			if !module.input {
				rows = l.outputRows()
			}
			module.core = append([]*placed(nil), rows[module.index]...)
		}
		if number > 0 && number%perStreet == 0 {
			current := map[*placed]bool{}
			for _, entity := range l.all {
				current[entity] = true
			}
			previous := []*placed{}
			for _, entity := range work.all {
				if !current[entity] {
					previous = append(previous, entity)
				}
			}
			_, previousHigh, err := denseStreetMachineYBounds(previous)
			if err != nil {
				return nil, err
			}
			currentLow, _, err := denseStreetMachineYBounds(l.all)
			if err != nil {
				return nil, err
			}
			baseY = previousHigh - currentLow + foldClearance
			rowCounts, extraX = [2]int{}, 0
		}
		rotated := rowCounts[0] > rowCounts[1]
		if rotated {
			rotatePlacedQuarter(l.all, 2)
			shift(l.all, 0, basis.y, 0)
		}
		minimumX := [2]float64{math.Inf(1), math.Inf(1)}
		added := [2]int{}
		for _, machine := range l.machines {
			row := 0
			if math.Abs(machine.a.y-basis.y) < math.Abs(machine.a.y) {
				row = 1
			}
			minimumX[row] = math.Min(minimumX[row], machine.a.x)
			added[row]++
		}
		moveX := math.Inf(1)
		for row := 0; row < 2; row++ {
			if added[row] == 0 {
				continue
			}
			target := float64(rowCounts[row])*pitch + extraX
			if row == 1 {
				target += basis.x
			}
			shiftX := target - minimumX[row]
			if math.IsInf(moveX, 1) {
				moveX = shiftX
			} else if math.Abs(moveX-shiftX) > 1e-5 {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_PARITY", Message: "容量组的奇偶布局无法按同一周期接续两排"}
			}
		}
		shift(l.all, moveX, baseY, 0)
		phases := []float64{0, .1, -.1, .2, -.2, .3, -.3, .4, -.4, .5, -.5}
		currentGap, currentPhase := 0., 0.
		var accepted bool
		var lastError error
		seam := denseStreetSeam{group: group, rotated: rotated, street: number / perStreet, baseY: baseY}
		// 先保留已验收的小接缝；短容量组无解时，再搜索一个机器周期，仍保留全部采样节点与真实臂姿态。
		for pass := 0; pass < 2 && !accepted; pass++ {
			steps := 20
			if pass == 1 {
				steps = int(math.Ceil(pitch / .05))
				phases = append(phases, .6, -.6, .7, -.7, .8, -.8, .9, -.9, 1, -1)
			}
			for step := 0; step <= steps && !accepted; step++ {
				gap := math.Min(float64(step)*.05, pitch)
				if len(street.seams) == 0 && step > 0 {
					break
				}
				shift(l.all, gap-currentGap, 0, 0)
				currentGap = gap
				for _, phase := range phases {
					if pass == 1 && gap <= 1 && math.Abs(phase) <= .5 {
						continue
					}
					if err := work.checkGeneration("街道断口与独立节点相位"); err != nil {
						return nil, err
					}
					shiftDenseStreetBelts(l, phase-currentPhase)
					currentPhase = phase
					lastError = denseStreetNodeSeparation(work.all, l.all)
					if lastError != nil {
						continue
					}
					lastError = work.validateDenseStreetCore(ctx, attachments)
					if lastError == nil {
						accepted = true
						seam.gap, seam.phase = gap, phase
						break
					}
				}
			}
		}
		if !accepted {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SEAM", Message: fmt.Sprintf("容量组%d在紧凑接缝及一个机器周期、±1格带相位的扩展搜索内未通过；保留必要采样节点，末次错误：%v", group, lastError)}
		}
		street.groups[group] = l
		street.seams = append(street.seams, seam)
		rowCounts[0] += added[0]
		rowCounts[1] += added[1]
		extraX += seam.gap
		for input, rows := range [][][]*placed{l.inputs, l.outputs} {
			items, rates := r.Items, ratesIn
			if input == 1 {
				items, rates = r.Results, ratesOut
			}
			for i, row := range rows {
				street.ports = append(street.ports, denseStreetPort{group: group, item: items[i], input: input == 0, head: row[0], inner: row[1], requiredRate: rates[i] * float64(counts[group]), capacity: capacity})
			}
		}
	}
	street.all = append([]*placed(nil), work.all...)
	street.streetCount = (len(counts) + perStreet - 1) / perStreet
	for _, entity := range street.all {
		entity.productionRate = nil
		if surfaceIsBelt(entity.record.ItemID) {
			street.beltNodesAfter++
		}
	}
	for _, l := range street.groups {
		if err := validateLineMachineInserters(l, work.setting); err != nil {
			return nil, err
		}
		if err := work.annotateLineProductionRates(l); err != nil {
			return nil, err
		}
	}
	for _, check := range work.logistics.Checks {
		if check.RuleID == "reference-local-spacing" {
			street.checks = append(street.checks, check)
		}
	}
	return street, nil
}

// trimDenseStreetTracks 职责：以货物采样证明必要范围后移除独立地面坡道；使用：街道组的局部准备；边界：不删机器/臂，不以删除货物目标换取几何成功。
func (b *builder) trimDenseStreetTracks(l *line) error {
	removed := map[*placed]bool{}
	goals := map[*placed]vector{}
	for _, arm := range l.all {
		if arm.densityCargo.enabled {
			target, err := denseCargoLocalTarget(arm)
			if err != nil {
				return err
			}
			goals[arm] = target
		}
	}
	for side, rows := range [][][]*placed{l.inputs, l.outputs} {
		for index, track := range rows {
			height := .125
			if side == 1 {
				height = .5
			}
			if index > 0 {
				height = .3125
			}
			flat := []*placed{}
			members := map[*placed]bool{}
			for _, node := range track {
				members[node] = true
				if math.Abs(node.a.z-height) < 1e-6 {
					flat = append(flat, node)
				}
			}
			arms := []*placed{}
			for _, arm := range l.all {
				anchor := arm.input
				if side == 1 {
					anchor = arm.output
				}
				if arm.densityCargo.enabled && arm.densityCargo.output == (side == 1) && members[anchor] {
					arms = append(arms, arm)
				}
			}
			if len(flat) < 2 || len(arms) == 0 {
				return &blueprint.Error{Code: "GEN_E_DENSITY_STREET_CARGO", Message: "街道低轨没有足够的实际机器采样"}
			}
			low, high := len(flat)-1, 0
			for _, arm := range arms {
				at, best := 0, math.Inf(1)
				for i, node := range flat {
					distance := math.Abs(node.a.x - goals[arm].x)
					if distance < best {
						at, best = i, distance
					}
				}
				low, high = min(low, at), max(high, at)
			}
			var kept []*placed
			var bindings map[*placed]int
			for halo := 1; halo <= 3; halo++ {
				candidate := flat[max(0, low-halo):min(len(flat), high+halo+1)]
				found, err := denseStreetTrackBindings(candidate, arms, goals, side == 1)
				if err == nil {
					kept, bindings = candidate, found
					break
				}
			}
			if len(kept) == 0 {
				return &blueprint.Error{Code: "GEN_E_DENSITY_STREET_CARGO", Message: "移除地面坡道后没有覆盖全部真实货物目标的低轨"}
			}
			alive := map[*placed]bool{}
			for _, node := range kept {
				alive[node] = true
			}
			for _, node := range track {
				if !alive[node] {
					removed[node] = true
				}
			}
			for _, node := range kept {
				if node.output != nil && !alive[node.output] {
					node.output = nil
					node.record.OutputToSlot = 0
				}
			}
			for _, arm := range arms {
				anchor := kept[bindings[arm]]
				if side == 1 {
					arm.output = anchor
				} else {
					arm.input = anchor
				}
				coater := arm.densityCargo.coater
				setDenseCargoTarget(arm, anchor, kept, goals[arm], side == 1)
				arm.densityCargo.coater = coater
			}
			if side == 0 {
				l.inputs[index] = append([]*placed(nil), kept...)
			} else {
				l.outputs[index] = append([]*placed(nil), kept...)
			}
		}
	}
	keep := func(entities []*placed) []*placed {
		out := make([]*placed, 0, len(entities))
		for _, entity := range entities {
			if !removed[entity] {
				out = append(out, entity)
			}
		}
		return out
	}
	l.all = keep(l.all)
	b.all = keep(b.all)
	l.output = l.outputs[0]
	for _, entity := range l.all {
		entity.productionRate = nil
	}
	return b.annotateLineProductionRates(l)
}

// denseStreetTrackBindings 职责：证明候选低轨的自然带头/带尾保护区不会吞掉取放目标；使用：最小裁剪窗口选择；边界：只绑定当前物料段。
func denseStreetTrackBindings(track, arms []*placed, goals map[*placed]vector, output bool) (map[*placed]int, error) {
	records := make([]blueprint.Building, len(track))
	world := make([]density.CargoPoint, len(track))
	allowed := make([]int32, len(track))
	for i, node := range track {
		next := i + 1
		if output {
			next = i - 1
		}
		records[i] = node.record
		records[i].Index = int32(i)
		records[i].InputObjIdx = -1
		records[i].OutputObjIdx = -1
		if next >= 0 && next < len(track) {
			records[i].OutputObjIdx = int32(next)
		}
		world[i] = density.CargoPoint{node.a.x * planarGridLength, node.a.z*1.3333333 + .2, node.a.y * planarGridLength}
		allowed[i] = int32(i)
	}
	up := density.CargoPoint{0, 1, 0}
	sampler, err := density.NewCargoSampler(records, world, density.CargoOptions{PlanarUp: &up})
	if err != nil {
		return nil, err
	}
	result := map[*placed]int{}
	for _, arm := range arms {
		target := goals[arm]
		binding, err := sampler.Nearest(allowed, density.CargoPoint{target.x * planarGridLength, target.z*1.3333333 + .2, target.y * planarGridLength}, referenceCargoTolerance)
		if err != nil {
			return nil, err
		}
		result[arm] = int(binding.AnchorIndex)
	}
	return result, nil
}

// shiftDenseStreetBelts 职责：只移动本段带节点并保持绝对货物目标不动；使用：相邻容量段节点避合并；边界：不移动机器/臂，不共享节点。
func shiftDenseStreetBelts(l *line, dx float64) {
	for _, entity := range l.all {
		if surfaceIsBelt(entity.record.ItemID) {
			entity.a.x += dx
			entity.b.x += dx
		}
	}
	for _, arm := range l.all {
		if arm.densityCargo.enabled {
			arm.densityCargo.relative.x -= dx
		}
	}
}

// validateDenseStreetCore 职责：运行最终出口同源的静态、动态、带长和货物采样检查；使用：每个接续候选；边界：没有外围路线与供电，不宣称完整工厂验收。
func (b *builder) validateDenseStreetCore(ctx context.Context, accessoryStage ...bool) error {
	records, _, _, err := b.records()
	if err != nil {
		return err
	}
	encoding := newPlanarRuleEncoding()
	if len(accessoryStage) > 0 && accessoryStage[0] {
		err = b.rebindDetachedStreetCargo(ctx, records)
	} else {
		err = b.rebindReferenceCargo(ctx, encoding, records)
	}
	if err != nil {
		return err
	}
	if err = encoding.validateBeltDistancesContext(ctx, records); err != nil {
		return err
	}
	if err = encoding.validateStaticColliders(ctx, records); err != nil {
		return err
	}
	return encoding.validateDynamicFacilities(ctx, records)
}

// denseStreetMachineYBounds 职责：取得真实机器建造盒的切平面南北范围；使用：街道折排；边界：不使用传送带空尾或普通footprint估计机器外形。
func denseStreetMachineYBounds(all []*placed) (float64, float64, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return 0, 0, err
	}
	encoding := newPlanarRuleEncoding()
	low, high := math.Inf(1), math.Inf(-1)
	for _, entity := range all {
		if entity.record.RecipeID <= 0 {
			continue
		}
		shapes, err := catalog.Shapes(entity.record.ItemID, entity.record.ModelIndex)
		if err != nil {
			return 0, 0, err
		}
		for _, shape := range shapes {
			box, err := encoding.colliderBox(vector{entity.a.x * planarGridLength, entity.a.z*1.3333333 + .2, entity.a.y * planarGridLength}, entity.record.Yaw, shape)
			if err != nil {
				return 0, 0, err
			}
			a, c := box.Bounds()
			low, high = math.Min(low, a[2]/planarGridLength), math.Max(high, c[2]/planarGridLength)
		}
	}
	if math.IsInf(low, 0) || math.IsInf(high, 0) {
		return 0, 0, &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: "街道缺少真实生产机器"}
	}
	return low, high, nil
}

// denseStreetNodeSeparation 职责：核对不同容量段的同高节点至少保留0.35米独立相位；使用：允许带段重叠但不能合并节点；边界：异高节点仍须通过实际球面去重/碰撞规则，不把0.25米原始分层扩大成禁止区。
func denseStreetNodeSeparation(all, current []*placed) error {
	members := map[*placed]bool{}
	for _, entity := range current {
		members[entity] = true
	}
	world := func(p *placed) vector {
		return vector{p.a.x * planarGridLength, p.a.y * planarGridLength, p.a.z * 1.3333333}
	}
	cell := func(p vector) [3]int {
		return [3]int{int(math.Floor(p.x / .35)), int(math.Floor(p.y / .35)), int(math.Floor(p.z / .35))}
	}
	cells := map[[3]int][]vector{}
	for _, entity := range all {
		if surfaceIsBelt(entity.record.ItemID) && !members[entity] {
			p := world(entity)
			key := cell(p)
			cells[key] = append(cells[key], p)
		}
	}
	for _, entity := range current {
		if !surfaceIsBelt(entity.record.ItemID) {
			continue
		}
		point := world(entity)
		key := cell(point)
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, other := range cells[[3]int{key[0] + x, key[1] + y, key[2] + z}] {
						if math.Abs(point.z-other.z) > 1e-6 {
							continue
						}
						if math.Pow(point.x-other.x, 2)+math.Pow(point.y-other.y, 2)+math.Pow(point.z-other.z, 2) < .35*.35-1e-9 {
							return &blueprint.Error{Code: "GEN_E_DENSITY_STREET_PHASE", Message: "相邻容量段的节点相位小于0.35米，不能合并独立带"}
						}
					}
				}
			}
		}
	}
	return nil
}
