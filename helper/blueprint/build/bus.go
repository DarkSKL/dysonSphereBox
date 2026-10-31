package build

// 文件职责：混带环形总线、过滤取料、回流限速与原料补料；使用：mode 6；边界：不模拟逐帧堵带，超带宽必须显式告警。
// 依照 MainLineBuildInfo/MainBelt/InsertBlcok/TBlock 的算法意图独立实现；修复未满负荷静默截断和空数组崩溃，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"fmt"
	"math"
	"sort"
)

// transfer 职责：保存分拣器限速组合；使用：总线门控；边界：不把白爪当成可降速蓝爪。
type transfer struct {
	// level 职责：等级；使用：创建；边界：1..3。
	level int
	// length 职责：跨度；使用：端点与参数；边界：1..3。
	length int
	// power 职责：每秒运力；使用：分解；边界：不含未知集装升级。
	power float64
}

// transferPlan 职责：按短跨度及高等级优先分解运力；使用：回流限速；边界：剩余无法表示时返回真实缺口供 UI 警告。
func transferPlan(need float64, minLength, maxLevel int) ([]transfer, float64) {
	out := []transfer{}
	remaining := need
	maxLevel = min(4, maxLevel)
	if minLength < 1 {
		minLength = 1
	}
	if minLength > 3 || maxLevel < 1 {
		return out, remaining
	}
	for length := minLength; length <= 3; length++ {
		for lv := maxLevel; lv >= 1; lv-- {
			capacity, err := transport.InserterCapacityPerMinute(lv, length)
			if err != nil {
				continue
			}
			power := capacity / 60
			if power <= 0 || math.IsNaN(power) || math.IsInf(power, 0) {
				continue
			}
			for remaining >= power-.0001 {
				out = append(out, transfer{level: lv, length: length, power: power})
				remaining -= power
				if len(out) > 10000 {
					return out, remaining
				}
			}
		}
	}
	return out, remaining
}

// beltTransfer 职责：连接两条带上的分拣器；使用：取料/回流；边界：超 3 格不制造无效臂。
func (b *builder) beltTransfer(from, to *placed, level, filter int) (*placed, error) {
	distance := math.Abs(from.a.x-to.a.x) + math.Abs(from.a.y-to.a.y)
	if distance < .9 || distance > 3.01 || from.a.z != to.a.z {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线分拣器跨度超出 1..3 格"}
	}
	p := b.make(2010+level, from.a.x, from.a.y, from.a.z)
	p.a, p.b = from.a, to.a
	p.input = from
	p.output = to
	p.record.InputFromSlot = -1
	p.record.OutputToSlot = -1
	p.record.FilterID = int16(filter)
	yaw := math.Atan2(to.a.x-from.a.x, to.a.y-from.a.y) * 180 / math.Pi
	if yaw < 0 {
		yaw += 360
	}
	p.record.Yaw = float32(yaw)
	p.record.Yaw2 = p.record.Yaw
	p.record.Parameters = []int32{int32(math.Round(distance))}
	return p, nil
}

// limiter 职责：将生产输出限速送到返回带；使用：中间产品回流；边界：组合残余以告警返回，不丢失建筑或需求。
func (b *builder) limiter(output, back *placed, need float64) ([]*placed, []*placed, []*placed, error) {
	plan, remaining := transferPlan(math.Ceil(need*2)/2, 1, b.setting.InserterLv)
	if len(plan) == 0 {
		return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "回流限速无法表示需求"}
	}
	if remaining > 1e-6 {
		b.warnings = append(b.warnings, fmt.Sprintf("回流门控尚有 %.3f/s 运力缺口", remaining))
	}
	points := []vector{}
	targets := []vector{}
	x := 0.
	length := 1
	for _, tr := range plan {
		if len(points) == 0 {
			points = append(points, vector{0, -1, 0})
			x = 1
		}
		for length < tr.length {
			points = append(points, vector{x, -float64(length), 0})
			length++
			points = append(points, vector{x, -float64(length), 0})
			x++
		}
		v := vector{x, -float64(tr.length), 0}
		points = append(points, v)
		targets = append(targets, v)
		x++
	}
	sign := 1.
	if back.a.y < output.a.y {
		sign = -1
	}
	offset := vector{output.a.x + 1, output.a.y + sign, 0}
	for i := range points {
		points[i].x += offset.x
		points[i].y = points[i].y*sign + offset.y
	}
	feed := b.byPath(points)
	returnRail := b.straight(offset.x, offset.y, int(x), 2)
	reverse(returnRail)
	connect(output, feed[0])
	connect(returnRail[0], back)
	gateArms := make([]*placed, 0, len(plan))
	for i, tr := range plan {
		v := targets[i]
		v.x += offset.x
		v.y = v.y*sign + offset.y
		var source *placed
		for _, p := range feed {
			if p.a == v {
				source = p
				break
			}
		}
		target := returnRail[int(math.Round(v.x-offset.x))]
		arm, err := b.beltTransfer(source, target, tr.level, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		gateArms = append(gateArms, arm)
	}
	return feed, returnRail, gateArms, nil
}

// busRing 职责：保存闭合总线两侧可连接带；使用：物品调度；边界：所有物品在分拣器侧过滤。
type busRing struct {
	// left 职责：补料侧；使用：原矿补给；边界：按整数 y 查询。
	left map[int]*placed
	// right 职责：产线侧；使用：取料/回流；边界：按整数 y 查询。
	right map[int]*placed
	// belts 职责：保存闭合环路有向带序；使用：验证环路闭合与生成lane索引；边界：不含环外分支。
	belts []*placed
}

// ring 职责：创建反向闭合矩形带；使用：最多三条嵌套总线；边界：不生成开环或零长度边。
func (b *builder) ring(layer, height int) busRing {
	left, right := layer, 5-layer
	bottom, top := layer, height+5-layer
	points := []vector{}
	for y := top; y >= bottom; y-- {
		points = append(points, vector{float64(left), float64(y), 0})
	}
	for x := left + 1; x <= right; x++ {
		points = append(points, vector{float64(x), float64(bottom), 0})
	}
	for y := bottom + 1; y <= top; y++ {
		points = append(points, vector{float64(right), float64(y), 0})
	}
	for x := right - 1; x > left; x-- {
		points = append(points, vector{float64(x), float64(top), 0})
	}
	belts := b.byPath(points)
	connect(belts[len(belts)-1], belts[0])
	r := busRing{left: map[int]*placed{}, right: map[int]*placed{}, belts: belts}
	for _, p := range belts {
		if int(p.a.x) == left {
			r.left[int(p.a.y)] = p
		}
		if int(p.a.x) == right {
			r.right[int(p.a.y)] = p
		}
	}
	return r
}

// rawSupplement 职责：按受控补料和优先回收拼接 T 形带；使用：原料总线入口；边界：输入为明确标记的外部供料口。
func (b *builder) rawSupplement(item int, need float64, layer int, ring busRing, yStart int) (int, error) {
	supply := planBusSupply(need, layer, b.setting)
	plan, remaining := supply.feed, supply.remaining
	if len(plan) == 0 {
		return yStart, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "原料补料运力无法表示"}
	}
	if remaining > 1e-6 {
		b.warnings = append(b.warnings, fmt.Sprintf("物品 %d 补料有 %.3f/s 缺口", item, remaining))
	}
	if supply.requested > supply.effective+1e-6 {
		return yStart, &blueprint.Error{Code: "GEN_E_BELT_CAPACITY", Message: fmt.Sprintf("物品 %d 需求 %.2f/min 超过当前总线单入口 %.2f/min，不能返回欠供蓝图", item, supply.requested*60, supply.effective*60)}
	}
	maxY := yStart + len(plan) + 4
	inputY := maxY
	center := b.make(2000+b.setting.BeltLv, -1, float64(inputY), 0)
	external := b.make(2000+b.setting.BeltLv, -2, float64(inputY), 0)
	external.record.Parameters = []int32{int32(item), int32(math.Ceil(need * 60))}
	connect(external, center)
	previous := center
	lowest := inputY
	feedArms := []*placed{}
	feedPath := []*placed{external, center}
	for i, tr := range plan {
		y := inputY - i - 1
		x := layer - tr.length
		points := manhattan(previous.a, vector{float64(x), float64(y), 0}, false)
		segment := b.byPath(points[1:])
		connect(previous, segment[0])
		feedPath = append(feedPath, segment...)
		previous = segment[len(segment)-1]
		lowest = y
		target := ring.left[y]
		if target == nil {
			return yStart, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线高度不足以连接补料"}
		}
		arm, err := b.beltTransfer(previous, target, tr.level, 0)
		if err != nil {
			return yStart, err
		}
		feedArms = append(feedArms, arm)
	}
	// 返回支路的实际跨度固定为层下标+1，按真实运力配足，避免只按长度一估计。
	lv := min(4, b.setting.InserterLv)
	backCount := supply.returnCount
	var previousReturn *placed
	returnArms := []*placed{}
	for i := 0; i < backCount; i++ {
		y := inputY + i + 1
		source := ring.left[y]
		if source == nil {
			return yStart, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线高度不足以回收原料"}
		}
		p := b.make(2000+b.setting.BeltLv, -1, float64(y), 0)
		connect(p, center)
		if i > 0 {
			connect(p, previousReturn)
		}
		previousReturn = p
		arm, err := b.beltTransfer(source, p, lv, item)
		if err != nil {
			return yStart, err
		}
		returnArms = append(returnArms, arm)
		if y > maxY {
			maxY = y
		}
	}
	if *b.setting.NeedPower {
		b.make(2201, -4, float64(inputY), 0)
	}
	if layerIndex, ok := b.busLayers[item]; ok {
		// 仅登记入口实际创建的带与分拣器，避免把其它原料路线误并入本路线。
		b.busLinks = append(b.busLinks, busTransportLink{kind: "raw-input", owner: -1, item: item, layer: layerIndex, rate: need * 60, endpoint: external, ring: ring.left[lowest], ringPath: ring.belts, parts: append(feedArms, returnArms...), row: feedPath})
	}
	_ = lowest
	return maxY + 3, nil
}

// flowEdgesForOwner 职责：汇总指定产线流出某类规范边的计划份额；使用：回流门控不得采用机器额定能力；边界：仅按当前行机器数比例拆分既有计划边。
func flowEdgesForOwner(graph *quant.FlowGraph, owner int, kind string, share float64) []quant.ItemRate {
	if graph == nil || share <= 0 {
		return nil
	}
	ownerNode := ""
	for _, node := range graph.Nodes {
		if node.Kind == "production" && node.ProductionIndex == owner {
			ownerNode = node.ID
			break
		}
	}
	if ownerNode == "" {
		return nil
	}
	rates := map[int]float64{}
	for _, edge := range graph.Edges {
		if edge.SourceNodeID == ownerNode && edge.FlowKind == kind {
			rates[edge.ItemID] += edge.RatePerMinute * share
		}
	}
	items := make([]int, 0, len(rates))
	for item, rate := range rates {
		if rate > 0 {
			items = append(items, item)
		}
	}
	sort.Ints(items)
	out := make([]quant.ItemRate, 0, len(items))
	for _, item := range items {
		out = append(out, quant.ItemRate{ItemID: item, RatePerMinute: rates[item]})
	}
	return out
}

// addBusBoundaryLinks 职责：为目标和剩余流建立逐物品过滤出口；使用：mode6 流边核验；边界：不再复用同一条未过滤末端冒充多个物品出口。
func (b *builder) addBusBoundaryLinks(l *line, share float64, gateFeed []*placed) error {
	if l == nil || len(l.output) == 0 || b.flowGraph == nil {
		return nil
	}
	// mode6 的输出带允许混带，但边界端口必须逐物品分开。这里从输出干线上的不同
	// belt cell 分别取料，避免把同一端点的 Parameters 反复改写成最后一个物品。
	flows := []struct {
		kind string
		item int
		rate float64
	}{}
	for _, kind := range []string{"target-output", "leftover"} {
		for _, flow := range flowEdgesForOwner(b.flowGraph, l.output[0].owner, kind, share) {
			if flow.RatePerMinute <= 0 {
				continue
			}
			flows = append(flows, struct {
				kind string
				item int
				rate float64
			}{kind: kind, item: flow.ItemID, rate: flow.RatePerMinute})
		}
	}
	if len(flows) == 0 {
		return nil
	}
	// 输出带可能短于副产物/目标出口数量；先把干线延伸到足够的独立取料格，
	// 每个过滤臂保持 1 格跨度，避免把多个臂重叠到同一格而无法通过碰撞预检。
	if len(l.output) < len(flows) {
		last := l.output[len(l.output)-1]
		extra := b.straight(last.a.x+1, last.a.y, len(flows)-len(l.output), 2)
		connect(last, extra[0])
		l.output = append(l.output, extra...)
	}
	for i, flow := range flows {
		source := l.output[len(l.output)-1-i]
		branch := b.make(2000+b.setting.BeltLv, source.a.x, source.a.y+1, source.a.z)
		arm, err := b.beltTransfer(source, branch, b.setting.InserterLv, flow.item)
		if err != nil {
			return err
		}
		// 分支带本身就是开放端点：各 source 位于不同干线格，因此 branch
		// 也天然不重叠；再额外横向延伸会与相邻物品分支争用格子。
		branch.record.Parameters = []int32{int32(flow.item), int32(math.Ceil(flow.rate))}
		b.busLinks = append(b.busLinks, busTransportLink{
			kind: flow.kind, owner: l.output[0].owner, item: flow.item, layer: -1,
			rate: flow.rate, endpoint: branch, parts: []*placed{arm},
			row: []*placed{branch},
		})
	}
	// 保留一个未被边界过滤臂占用的独立尾端；若该产线还有内部回流，
	// limiter 将从此尾端取料，避免与目标/剩余出口共用同一 belt cell。
	last := l.output[len(l.output)-1]
	handoff := b.straight(last.a.x+1, last.a.y, 1, 2)
	connect(last, handoff[0])
	l.output = append(l.output, handoff...)
	_ = gateFeed
	return nil
}
