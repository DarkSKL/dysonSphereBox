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

// storageSlotPoint 职责：把 2101 小型储物仓的已核对槽位方向映射为局部整数格锚点；使用：配送箱体分拣器候选；边界：仅覆盖槽位4/10，最终端口以版本化slotPoses修正，未知槽位拒绝。
func storageSlotPoint(center vector, yaw float32, slot int) (vector, bool) {
	offset, ok := map[int]vector{4: {x: 1}, 10: {x: -1}}[slot]
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
		endpoint, err := b.fly(row.item, row.link, true, linkX+18+6*float64(depth), beginY+3*float64(depth), mode, begin)
		if err != nil {
			return err
		}
		endpoint.rate = flyLineRate(l.info, endpoint.item, true, len(l.machines))
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	// 输出带可能承载多个配方产物。每个产物都必须有独立的过滤配送端点；
	// 不能只登记 MainItemID，否则 leftover/副产物边会在收尾阶段被误报为缺少端口。
	// 这些端点仍共享机器输出带，但由各自的箱体分拣器 FilterID 进行逐物品取货，
	// 上层会按实际计划流量分配端点并检查共享带容量。
	// 输出带先延伸成一条共享干线；每个产物从独立干线格通过带分拣器
	// 分出自己的支路。这样多个配送器不会反复覆盖同一个 belt.output，
	// 也不会因为不同产物各自从同一格起步而产生重叠带段。
	trunk := l.output[len(l.output)-1]
	outputBaseY := beginY + 3*float64(len(order)) + 6
	for outputIndex, item := range l.recipe.Results {
		if err := b.checkGeneration("配送输出端点"); err != nil {
			return err
		}
		trunkPoints := manhattan(trunk.a, vector{trunk.a.x + 6, trunk.a.y, trunk.a.z}, true)
		trunkPath := b.byPath(trunkPoints[1:])
		if len(trunkPath) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配送输出干线没有可用带节点"}
		}
		// byPath 只连接新建节点；先把原输出带尾接入干线，避免
		// 过滤支路从一条脱离机器输出的孤立带开始。
		connect(trunk, trunkPath[0])
		trunk = trunkPath[len(trunkPath)-1]
		// 每个过滤支路偏移两格，满足已核对的分拣器 1..3 格跨度；
		// 横向干线已经为每个产物预留独立格，不再用逐项递增的纵向偏移
		// 造成第四格以上的非法臂距。
		branch := b.make(2000+b.setting.BeltLv, trunk.a.x, trunk.a.y+2, trunk.a.z)
		filter, err := b.beltTransfer(trunk, branch, b.setting.InserterLv, item)
		if err != nil {
			return err
		}
		// 输出支路使用输入端点之后的独立 lane；若仍从 18 格起步，
		// 会与最后一条输入配送竖轨共线并触发无连接交叉。
		endpoint, err := b.fly(item, branch, false, branch.a.x+18+6*float64(len(order)+outputIndex), outputBaseY+3*float64(len(l.recipe.Results)-1-outputIndex), mode, begin)
		if err != nil {
			return err
		}
		endpoint.filters = append(endpoint.filters, filter)
		endpoint.rate = flyLineRate(l.info, endpoint.item, false, len(l.machines))
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	return nil
}

// fly 职责：创建一个物品的箱体/配送器/桥接带；使用：行末连接；边界：生成后容量和运力由上层提示。
func (b *builder) fly(item int, link *placed, input bool, x, y float64, mode, begin int) (*flyTransportEndpoint, error) {
	endpoint := &flyTransportEndpoint{owner: b.owner, item: item, input: input, link: link}
	// 每个端点单元相对其真实 link 固定落在右侧，避免按整条产线
	// 累加 x 把配送器推到数千格外并产生不可接线的长尾。
	offset := math.Max(16, math.Min(64, x-link.a.x))
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
		path := manhattan(link.a, flySplitterApproach(link.a, splitter.a), input)
		if len(path) < 2 {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "四向分流器与机器带端点重合，无法建立实体接驳带"}
		}
		if err := b.validateFlyPathCandidate(path, link, splitter, b.all[begin:]); err != nil {
			return nil, err
		}
		belts := b.byPath(path[1:])
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
	// 箱体统一朝北，槽位 10 位于西侧。把带端放在槽位外两格，
	// 由分拣器跨接，避免接驳带穿入箱体建造盒。
	boxPoint, ok := storageSlotPoint(box.a, box.record.Yaw, 10)
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "配送箱体缺少槽位 10 几何"}
	}
	away := vector{x: boxPoint.x - box.a.x, y: boxPoint.y - box.a.y}
	length := math.Hypot(away.x, away.y)
	if length < 1e-6 {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "配送箱体槽位方向无效"}
	}
	beltEnd := vector{boxPoint.x + away.x/length*2, boxPoint.y + away.y/length*2, 0}
	// 输入先横向到独立竖轨；输出先上行到所有输入设施之外再横向。
	// 多产物输出的纬向轨位逆序，后续竖轨不会截断已生成的横向支路。
	points := manhattan(link.a, beltEnd, input)
	if len(points) < 2 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("配送器与机器带端点没有可用接驳带：端点(%.1f,%.1f)，槽位外端(%.1f,%.1f)", link.a.x, link.a.y, beltEnd.x, beltEnd.y)}
	}
	if err := b.validateFlyPathCandidate(points, link, nil, b.all[begin:]); err != nil {
		return nil, err
	}
	points = points[1:]
	belts := b.byPath(points)
	endpoint.box, endpoint.delivery, endpoint.belts = box, delivery, belts
	if input {
		reverse(belts)
		// reverse 只重写有向引用，切片首节点仍靠近机器带。
		connect(belts[0], link)
	} else {
		connect(link, belts[0])
	}
	end := belts[len(belts)-1]
	arm := b.make(2010+b.setting.InserterLv, facilityX, 0, 0)
	endpoint.arm = arm
	endpoint.arms = append(endpoint.arms, arm)
	if err := flyArmGeometry(arm, box, end, 10, input); err != nil {
		return nil, err
	}
	if !input {
		arm.record.FilterID = int16(item)
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
