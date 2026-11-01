package build

// 文件职责：装配物流配送器和四向分流模式；使用：mode 2/5；边界：保留箱体、配送器和端口关系，不声称物流逐帧模拟。
// 行为核对 dsp-calc FlyBlock.ts；依据其算法独立实现，见 ../NOTICE.md。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// storageSlotPoint 职责：把 2101 小型储物仓的已核对槽位方向映射为局部整数格锚点；使用：配送箱体分拣器候选；边界：仅覆盖东西侧已核对槽位，最终端口以版本化slotPoses修正，未知槽位拒绝。
func storageSlotPoint(center vector, yaw float32, slot int) (vector, bool) {
	offset, ok := map[int]vector{4: {x: 1}, 9: {x: -1, y: -1}, 10: {x: -1}, 11: {x: -1, y: 1}}[slot]
	if !ok {
		return center, false
	}
	angle := float64(yaw) * math.Pi / 180
	return vector{center.x + offset.x*math.Cos(angle) + offset.y*math.Sin(angle), center.y - offset.x*math.Sin(angle) + offset.y*math.Cos(angle), center.z}, true
}

// flyArmGeometry 职责：按箱体槽位建立一侧分拣器的真实端点和跨度参数；使用：mode2 箱体接驳；边界：两端距离必须落在 1..3 格，不能用箱体中心冒充槽位。
func flyArmGeometry(arm, box, belt *placed, boxSlot int, input bool) error {
	boxPoint, ok := storageSlotPoint(box.a, box.record.Yaw, boxSlot)
	if !ok {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("箱体 %d 槽位 %d 没有已核对端口几何", box.record.Index, boxSlot)}
	}
	distance := math.Hypot(boxPoint.x-belt.a.x, boxPoint.y-belt.a.y)
	if distance < .9 || distance > 3.01 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("配送箱体槽位 %d 与带端跨度 %.3f 超出 1..3 格", boxSlot, distance)}
	}
	arm.record.Parameters = []int32{int32(math.Round(distance))}
	arm.record.InputFromSlot = -1
	arm.record.OutputToSlot = -1
	direction := vector{boxPoint.x - belt.a.x, boxPoint.y - belt.a.y, 0}
	if input {
		direction.x, direction.y = -direction.x, -direction.y
	}
	yaw := math.Atan2(direction.x, direction.y) * 180 / math.Pi
	if yaw < 0 {
		yaw += 360
	}
	arm.record.Yaw = float32(yaw)
	arm.record.Yaw2 = arm.record.Yaw
	arm.a, arm.b = belt.a, boxPoint
	if input {
		arm.input, arm.output = box, belt
		arm.record.InputFromSlot = int8(boxSlot)
		arm.record.OutputToSlot = -1
		arm.a, arm.b = boxPoint, belt.a
	} else {
		arm.input, arm.output = belt, box
		arm.record.InputFromSlot = -1
		arm.record.OutputToSlot = int8(boxSlot)
		arm.a, arm.b = belt.a, boxPoint
	}
	return nil
}

// manhattan 职责：生成轴优先整数路径；使用：配送器局部带；边界：不同高度不自动连接。
func manhattan(a, b vector, firstX bool) []vector {
	out := []vector{a}
	p := a
	step := func(xAxis bool) {
		for {
			delta := b.y - p.y
			if xAxis {
				delta = b.x - p.x
			}
			if math.Abs(delta) < 1e-8 {
				break
			}
			if delta > 0 {
				delta = math.Min(1, delta)
			} else {
				delta = math.Max(-1, delta)
			}
			if xAxis {
				p.x += delta
			} else {
				p.y += delta
			}
			out = append(out, p)
		}
	}
	step(firstX)
	step(!firstX)
	return out
}

// flySplitterApproach 职责：把 mode5 带链终点放在已核对的水平分流器端口外一格；使用：Model39、Yaw90 的地面西/东端接驳；边界：该模型没有南北地面端口，不能按最近方向随意接入。
func flySplitterApproach(from, splitter vector) vector {
	point := splitter
	if from.x < splitter.x {
		point.x--
	} else {
		point.x++
	}
	return point
}

// flyBlocks 职责：在行末连接逐物品配送块；使用：mode 2/5；边界：输入按真实轨位从上往下展开，输出逆向阶梯排布，避免无连接交叉。
func (b *builder) flyBlocks(l *line, mode, begin int) error {
	if len(l.recipe.Results) > 1 {
		return b.flyIndependentOutputBlocks(l, mode, begin)
	}
	// 普通原料与增产剂共用配送容量/份额登记，但增产剂不是机器配方槽位，
	// 不修改l.inputs或recipe.Items；它通过独立的地面供剂入口接回喷涂轨。
	type inlet struct {
		item int
		link *placed
	}
	inlets := make([]inlet, 0, len(l.inputs)+1)
	for i, row := range l.inputs {
		reverse(row)
		inlets = append(inlets, inlet{l.recipe.Items[i], row[len(row)-1]})
	}
	if l.coaterInput != nil {
		entry := l.coaterInput
		right := l.output[len(l.output)-1].a.x
		points := manhattan(entry.a, vector{entry.a.x - 1, entry.a.y, 0}, true)
		points = append(points, manhattan(points[len(points)-1], vector{entry.a.x - 1, entry.a.y - 2, 0}, false)[1:]...)
		points = append(points, manhattan(points[len(points)-1], vector{right, entry.a.y - 2, 0}, true)[1:]...)
		if err := b.validateFlyPathCandidate(points, entry, nil, b.all[begin:]); err != nil {
			return err
		}
		path := b.byPath(points[1:])
		reverse(path)
		connect(path[0], entry)
		inlets = append(inlets, inlet{productionSprayItem(l.info), path[len(path)-1]})
	}
	// 输入/输出配送端点使用独立的纵向轨位，避开机器输出干线和
	// 相邻箱体建造盒；每个端点仍由自身的分拣器接回机器带。
	// 端点区与机器行、电力杆和输出干线保留独立净空；使用固定的
	// 外侧余量而非按输入数量累加，避免跨行端点互相穿越。
	beginY := l.output[0].a.y + 8
	inputOffset, inputStep := 18., 6.
	if b.flyCompact {
		// 输入竖轨必须位于全部输出干线之外，并让下一轨避开上一箱体；
		// 仅压缩已知空余，不改变箱体、分拣器跨距或带间净空。
		beginY = l.output[0].a.y + 4
		inputOffset = 6*float64(len(l.recipe.Results)) + 7
		inputStep = 5
	}
	order := make([]int, len(inlets))
	for i := range order {
		order[i] = i
	}
	// 向上折返的接驳必须先从最高输入带向外展开；否则较低输入的
	// 内侧竖带会截断较高输入横带。以真实带高排序，也覆盖混带槽位顺序。
	sort.SliceStable(order, func(i, j int) bool {
		return inlets[order[i]].link.a.y > inlets[order[j]].link.a.y
	})
	for depth, i := range order {
		if err := b.checkGeneration("配送输入端点"); err != nil {
			return err
		}
		row := inlets[i]
		// 端点沿用各物料带自己的纬向轨位，避免所有配送器共用一条
		// 横向通道后在同一格垂直折返；路径因此天然按物品行分离。
		// 端点横坐标只相对本行带尾局部偏移，避免按整条产线的
		// 全局 beginX 累加造成超宽蓝图；不同物料仍保持独立 lane。
		linkX := row.link.a.x
		rate := flyLineRate(l.info, row.item, true, len(l.machines))
		endpoint, err := b.flyAtRate(row.item, row.link, true, linkX+inputOffset+inputStep*float64(depth), beginY+3*float64(depth), mode, begin, rate)
		if err != nil {
			return err
		}
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	// 单产物保留既有并联过滤几何；多产物在上方入口使用各自机器带，
	// 不再从本分支拼出共享的混合出货干线。
	outputBaseY := beginY + 3*float64(len(order)) + 6
	if b.flyCompact {
		outputBaseY -= 3
	}
	for outputIndex, item := range l.recipe.Results {
		if err := b.checkGeneration("配送输出端点"); err != nil {
			return err
		}
		row := l.outputForItem(item)
		if len(row) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 产物 %d 缺少独立机器出货带", l.recipe.ID, item)}
		}
		trunk := row[len(row)-1]
		trunkPoints := manhattan(trunk.a, vector{trunk.a.x + 6, trunk.a.y, trunk.a.z}, true)
		trunkPath := b.byPath(trunkPoints[1:])
		if len(trunkPath) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配送输出干线没有可用带节点"}
		}
		// byPath 只连接新建节点；先把原输出带尾接入干线，避免
		// 过滤支路从一条脱离机器输出的孤立带开始。
		connect(trunk, trunkPath[0])
		trunk = trunkPath[len(trunkPath)-1]
		rate := flyLineRate(l.info, item, false, len(l.machines))
		branch, filters, err := b.flyOutputFilters(trunkPath, item, rate)
		if err != nil {
			return err
		}
		// 输出支路使用输入端点之后的独立 lane；若仍从 18 格起步，
		// 会与最后一条输入配送竖轨共线并触发无连接交叉。
		outputX := branch.a.x + 18 + 6*float64(len(order)+outputIndex)
		if b.flyCompact {
			outputX = branch.a.x + 16
		}
		endpoint, err := b.flyAtRate(item, branch, false, outputX, outputBaseY+3*float64(len(l.recipe.Results)-1-outputIndex), mode, begin, rate)
		if err != nil {
			return err
		}
		endpoint.filters = append(endpoint.filters, filters...)
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	return nil
}

// fly 职责：创建一个物品的箱体/配送器/桥接带；使用：行末连接；边界：生成后容量和运力由上层提示。
func (b *builder) fly(item int, link *placed, input bool, x, y float64, mode, begin int) (*flyTransportEndpoint, error) {
	return b.flyAtRate(item, link, input, x, y, mode, begin, 0)
}

// flyAtRate 职责：按计划份额装配有限真实并联臂的配送设施；使用：完整机器行端点；边界：不拆分或增加计划流量，有界槽位不足仍由本地能力诊断诚实告警。
func (b *builder) flyAtRate(item int, link *placed, input bool, x, y float64, mode, begin int, rate float64) (*flyTransportEndpoint, error) {
	return b.flyAtRateApproach(item, link, input, x, y, mode, begin, rate, input)
}

// flyAtRateApproach 职责：按指定轴顺序连接独立物料口与真实配送设施；使用：兼容单产物几何及多产物轨位银行；边界：顺序只影响有界局部路径，不改变带方向、等级或计划份额。
func (b *builder) flyAtRateApproach(item int, link *placed, input bool, x, y float64, mode, begin int, rate float64, firstX bool) (*flyTransportEndpoint, error) {
	endpoint := &flyTransportEndpoint{owner: b.owner, item: item, input: input, link: link, rate: rate}
	// 每个端点单元相对其真实 link 固定落在右侧，避免按整条产线
	// 累加 x 把配送器推到数千格外并产生不可接线的长尾。
	minOffset := 16.
	if b.flyCompact {
		minOffset = 10
	}
	offset := math.Max(minOffset, math.Min(64, x-link.a.x))
	facilityX := link.a.x + offset
	var splitter *placed
	if mode == 5 {
		splitter = b.make(2020, facilityX, y, 0)
		splitter.record.ModelIndex = 39
		splitter.record.Parameters = make([]int32, 6)
		splitter.record.OutputToSlot = 14
		splitter.record.InputFromSlot = 15
		splitter.record.OutputFromSlot = 15
		splitter.record.InputToSlot = 14
	}
	z := 0.
	if mode == 5 {
		z = 2
	}
	box := b.make(2101, facilityX, y, z)
	box.record.Yaw, box.record.Yaw2 = 0, 0
	box.record.OutputToSlot = 14
	box.record.InputFromSlot = 15
	box.record.OutputFromSlot = 15
	box.record.InputToSlot = 14
	box.record.Parameters = make([]int32, 110)
	box.record.Parameters[0] = 29
	for _, fluid := range []int{1000, 1007, 1114, 1116, 1120, 1121} {
		if fluid == item {
			box.record.Parameters[0] = 0
		}
	}
	delivery := b.make(2107, facilityX, y, z+1.875)
	delivery.record.Yaw = 0
	delivery.record.Yaw2 = 0
	delivery.input = box
	delivery.record.InputFromSlot = 13
	delivery.record.InputToSlot = 0
	delivery.record.FilterID = int16(item)
	delivery.record.Parameters = make([]int32, 128)
	delivery.record.Parameters[0] = 2
	delivery.record.Parameters[1] = 1
	if input {
		delivery.record.Parameters[1] = 2
	}
	delivery.record.Parameters[2] = 30000
	delivery.record.Parameters[3] = 1
	if mode == 5 {
		box.input = splitter
		box.record.InputFromSlot = 15
		path := manhattan(link.a, flySplitterApproach(link.a, splitter.a), firstX)
		if len(path) < 2 {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "四向分流器与机器带端点重合，无法建立实体接驳带"}
		}
		if err := b.validateFlyPathCandidate(path, link, splitter, b.all[begin:]); err != nil {
			return nil, err
		}
		belts := b.byPath(path[1:])
		belts[len(belts)-1].record.Parameters = []int32{int32(item), 0}
		endpoint.splitter, endpoint.box, endpoint.delivery, endpoint.belts = splitter, box, delivery, belts
		// Model39 在 Yaw90 时地面西口为2，地面东口为0；高层口1/3
		// 不参与本地带接驳，箱体与分流器仍使用堆叠连接槽15。
		slot := int8(0)
		if path[len(path)-1].x < splitter.a.x {
			slot = 2
		}
		if input {
			reverse(belts)
			// reverse 只重写有向引用，切片首尾仍是机器侧/分流器侧。
			first, last := belts[0], belts[len(belts)-1]
			last.input = splitter
			last.record.InputFromSlot = slot
			connect(first, link)
		} else {
			connect(link, belts[0])
			last := belts[len(belts)-1]
			last.output = splitter
			last.record.OutputToSlot = slot
		}
		return endpoint, nil
	}
	storage, err := planFlyStorageArms(rate, b.setting.InserterLv)
	if err != nil {
		return nil, err
	}
	// 西侧9/10/11是真实独立槽位，纵向接驳尾带逐一经过每根臂；
	// 两格离箱中心仍在真实建造盒之外，槽外一格提高单臂能力。
	tail := storage.beltPoints(box.a)
	// 输入先横向到独立竖轨；输出先上行到所有输入设施之外再横向。
	// 多产物输出的纬向轨位逆序，后续竖轨不会截断已生成的横向支路。
	points := manhattan(link.a, tail[0], firstX)
	points = append(points, tail[1:]...)
	if len(points) < 2 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("配送器与机器带端点没有可用接驳带：端点(%.1f,%.1f)，槽位外端(%.1f,%.1f)", link.a.x, link.a.y, tail[0].x, tail[0].y)}
	}
	if err := b.validateFlyPathCandidate(points, link, nil, b.all[begin:]); err != nil {
		return nil, err
	}
	points = points[1:]
	belts := b.byPath(points)
	belts[len(belts)-1].record.Parameters = []int32{int32(item), 0}
	endpoint.box, endpoint.delivery, endpoint.belts = box, delivery, belts
	if input {
		reverse(belts)
		// reverse 只重写有向引用，切片首节点仍靠近机器带。
		connect(belts[0], link)
	} else {
		connect(link, belts[0])
	}
	for index, slot := range storage.slots {
		arm := b.make(2010+b.setting.InserterLv, facilityX, 0, 0)
		if endpoint.arm == nil {
			endpoint.arm = arm
		}
		endpoint.arms = append(endpoint.arms, arm)
		belt := belts[len(belts)-len(storage.slots)+index]
		if err := flyArmGeometry(arm, box, belt, slot, input); err != nil {
			return nil, err
		}
		if !input {
			arm.record.FilterID = int16(item)
		}
	}
	return endpoint, nil
}

// flyLineRate 职责：将完整量化产线流量按当前机器行拆分；使用：端点容量和 FlowGraph 边分配；边界：不把取整机器余量当作额外流量。
func flyLineRate(production quant.Production, item int, input bool, machineCount int) float64 {
	if machineCount <= 0 || production.MachineCount <= 0 {
		return 0
	}
	rates := production.Outputs
	if input {
		rates = production.Inputs
	}
	for _, rate := range rates {
		if rate.ItemID == item {
			return rate.RatePerMinute * float64(machineCount) / float64(production.MachineCount)
		}
	}
	return 0
}
