package build

// 文件职责：混带环形总线、过滤取料、回流限速与原料补料；使用：mode 6；边界：不模拟逐帧堵带，超带宽必须显式告警。
// 依照 MainLineBuildInfo/MainBelt/InsertBlcok/TBlock 的算法意图独立实现；修复未满负荷静默截断和空数组崩溃，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
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
	table := [][]float64{{}, {1.5, .75, .5}, {3, 1.5, 1}, {6, 3, 2}}
	out := []transfer{}
	remaining := need
	maxLevel = min(3, maxLevel)
	for length := minLength; length <= 3; length++ {
		for lv := maxLevel; lv >= 1; lv-- {
			power := table[lv][length-1]
			if power == .75 {
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
func (b *builder) limiter(output, back *placed, need float64) error {
	plan, remaining := transferPlan(math.Ceil(need*2)/2, 1, b.setting.InserterLv)
	if len(plan) == 0 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "回流限速无法表示需求"}
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
		if _, err := b.beltTransfer(source, target, tr.level, 0); err != nil {
			return err
		}
	}
	return nil
}

// busRing 职责：保存闭合总线两侧可连接带；使用：物品调度；边界：所有物品在分拣器侧过滤。
type busRing struct {
	// left 职责：补料侧；使用：原矿补给；边界：按整数 y 查询。
	left map[int]*placed
	// right 职责：产线侧；使用：取料/回流；边界：按整数 y 查询。
	right map[int]*placed
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
	r := busRing{left: map[int]*placed{}, right: map[int]*placed{}}
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
		b.warnings = append(b.warnings, fmt.Sprintf("物品 %d 需求 %.2f/s，单带补料上限 %.2f/s；已避免生成无吞吐收益的额外门控，当前混带总线不能达到目标产率", item, supply.requested, supply.effective))
	}
	maxY := yStart + len(plan) + 4
	inputY := maxY
	center := b.make(2000+b.setting.BeltLv, -1, float64(inputY), 0)
	external := b.make(2000+b.setting.BeltLv, -2, float64(inputY), 0)
	external.record.Parameters = []int32{int32(item), int32(math.Ceil(need * 60))}
	connect(external, center)
	previous := center
	lowest := inputY
	for i, tr := range plan {
		y := inputY - i - 1
		x := layer - tr.length
		points := manhattan(previous.a, vector{float64(x), float64(y), 0}, false)
		segment := b.byPath(points[1:])
		connect(previous, segment[0])
		previous = segment[len(segment)-1]
		lowest = y
		target := ring.left[y]
		if target == nil {
			return yStart, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线高度不足以连接补料"}
		}
		if _, err := b.beltTransfer(previous, target, tr.level, 0); err != nil {
			return yStart, err
		}
	}
	// 返回支路的实际跨度固定为层下标+1，按真实运力配足，避免只按长度一估计。
	lv := min(3, b.setting.InserterLv)
	backCount := supply.returnCount
	var previousReturn *placed
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
		if _, err := b.beltTransfer(source, p, lv, item); err != nil {
			return yStart, err
		}
		if y > maxY {
			maxY = y
		}
	}
	if *b.setting.NeedPower {
		b.make(2201, -4, float64(inputY), 0)
	}
	_ = lowest
	return maxY + 3, nil
}

// generateBus 职责：装配三圈调度、过滤取料和受限回流；使用：mode 6；边界：总带宽不足时返回非空 warnings，不宣称模拟通过。
func (b *builder) generateBus(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	result := Result{}
	reportProgress(ctx, "bus-plan", "正在规划混带总线与物料调度", 0, 0)
	targets := map[int]float64{}
	for _, target := range req.Plan.Targets {
		targets[target.ItemID] += target.RatePerMinute
	}
	needed := map[int]float64{}
	for _, raw := range req.Plan.RawInputs {
		needed[raw.ItemID] += math.Ceil(raw.RatePerMinute/30) / 2
	}
	for _, p := range req.Plan.Productions {
		internal := math.Max(0, p.OutputCount-targets[p.MainItemID])
		if internal > 1e-8 {
			needed[p.MainItemID] += math.Ceil(internal/30) / 2
		}
	}
	type needItem struct {
		id   int
		rate float64
	}
	// 多产物配方的副产物也要进入总线，按同一圈分配，避免氢等物品没有调度。
	parent := map[int]int{}
	var root func(int) int
	root = func(id int) int {
		p, ok := parent[id]
		if !ok {
			parent[id] = id
			return id
		}
		if p != id {
			parent[id] = root(p)
		}
		return parent[id]
	}
	for _, p := range req.Plan.Productions {
		ri := dataset.RecipeByID[p.RecipeID]
		r := dataset.Recipes[ri]
		mainCount := 1.
		for i, id := range r.Results {
			if id == p.MainItemID {
				mainCount = r.ResultCounts[i]
			}
		}
		for i, id := range r.Results {
			if id != p.MainItemID {
				rate := math.Max(0, p.OutputCount*r.ResultCounts[i]/mainCount-targets[id])
				if rate > 0 {
					needed[id] += math.Ceil(rate/30) / 2
				}
				parent[root(id)] = root(p.MainItemID)
			}
		}
	}
	items := []needItem{}
	total := 0.
	for id, rate := range needed {
		items = append(items, needItem{id, rate})
		total += rate
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].rate != items[j].rate {
			return items[i].rate > items[j].rate
		}
		return items[i].id < items[j].id
	})
	capacity := []float64{0, 6, 12, 30}[b.setting.BeltLv]
	layers := max(1, min(3, int(math.Ceil(total/capacity))))
	loads := make([]float64, layers)
	schedule := map[int]int{}
	groups := map[int][]needItem{}
	groupOrder := []int{}
	for _, item := range items {
		key := root(item.id)
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], item)
	}
	for _, key := range groupOrder {
		load := 0.
		for _, item := range groups[key] {
			load += item.rate
		}
		for j := 0; j < layers; j++ {
			if loads[j]+load <= capacity || j == layers-1 {
				for _, item := range groups[key] {
					schedule[item.id] = j
				}
				loads[j] += load
				break
			}
		}
	}
	// 同配方的主/副产物必须共用一圈；主产物完全外供时也保留显式调度，不能依赖 map 零值。
	for _, p := range req.Plan.Productions {
		r := dataset.Recipes[dataset.RecipeByID[p.RecipeID]]
		layer := -1
		for _, id := range r.Results {
			if value, ok := schedule[id]; ok {
				layer = value
				break
			}
		}
		if layer >= 0 {
			for _, id := range r.Results {
				schedule[id] = layer
			}
		}
	}
	for j, load := range loads {
		if load > capacity+1e-8 {
			b.warnings = append(b.warnings, fmt.Sprintf("混带总线第 %d 圈需求 %.2f/s 超过 %.2f/s 带宽，需拆分目标或升级带速", j+1, load, capacity))
		}
	}
	allLines := []*line{}
	backs := map[*line][]*placed{}
	nextY := 3.
	for owner, p := range req.Plan.Productions {
		reportProgress(ctx, "production", "正在铺设"+dataset.ByID[p.MainItemID].Name+"产线", owner+1, len(req.Plan.Productions))
		if ctx.Err() != nil {
			return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成总线超时"}
		}
		if p.MachineCount <= 0 || p.MachineCount > 10000 {
			return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "机器数量超限"}
		}
		ri, ok := dataset.RecipeByID[p.RecipeID]
		if !ok {
			return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "配方不存在"}
		}
		recipe := dataset.Recipes[ri]
		counts, err := lineCounts(p, recipe, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return result, err
		}
		b.owner = owner
		for i, count := range counts {
			if ctx.Err() != nil {
				return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "总线装配超过截止时间"}
			}
			l, err := b.buildLine(p, recipe, count, 6, i, i == len(counts)-1)
			if err != nil {
				return result, err
			}
			needReturn := false
			for _, id := range recipe.Results {
				if needed[id] > 0 {
					needReturn = true
				}
			}
			if needReturn {
				backY := l.output[0].a.y + 1
				if b.setting.UseMixed || len(recipe.Items) > 5 {
					backY = l.output[0].a.y - 1
				}
				back := b.straight(l.output[0].a.x, backY, len(l.output), 2)
				reverse(back)
				backs[l] = back
				l.all = append(l.all, back...)
			}
			minimum, maximum := bounds(l.all)
			shift(l.all, 6-l.inputs[0][0].a.x, nextY-minimum.y, 0)
			nextY += math.Ceil(maximum.y-minimum.y) + 2
			allLines = append(allLines, l)
			if len(b.all) > blueprint.MaxBuildingCount {
				return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线建筑超过 20 万，请拆分目标"}
			}
		}
	}
	// 补料 T 形控制区也计入总线高度，避免原实现原料多时越界。
	rawHeight := 3.
	for _, raw := range req.Plan.RawInputs {
		layer := schedule[raw.ItemID]
		supply := planBusSupply(raw.RatePerMinute/60, layer, b.setting)
		rawHeight += float64(len(supply.feed) + supply.returnCount + 7)
	}
	height := int(math.Max(nextY+5, rawHeight))
	if height > 20000 {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线高度超限，请拆分目标"}
	}
	if len(b.all)+layers*(2*height+12) > blueprint.MaxBuildingCount {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线建筑超过 20 万，请拆分目标"}
	}
	b.owner = -1
	rings := make([]busRing, layers)
	for j := range rings {
		reportProgress(ctx, "bus-rings", "正在铺设总线环路", j+1, layers)
		rings[j] = b.ring(j, height)
	}
	for index, l := range allLines {
		reportProgress(ctx, "bus-connect", "正在连接总线取料与回流", index+1, len(allLines))
		b.owner = -1
		for i, row := range l.inputs {
			item := l.recipe.Items[i]
			layer, ok := schedule[item]
			if !ok {
				return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线缺少原料调度"}
			}
			source := rings[layer].right[int(math.Round(row[0].a.y))]
			if source == nil {
				return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线取料坐标无对应带"}
			}
			if _, err := b.beltTransfer(source, row[0], b.setting.InserterLv, item); err != nil {
				return result, err
			}
		}
		if back := backs[l]; len(back) > 0 {
			layer := schedule[l.info.MainItemID]
			target := rings[layer].right[int(math.Round(back[0].a.y))]
			if _, err := b.beltTransfer(back[0], target, b.setting.InserterLv, 0); err != nil {
				return result, err
			}
			share := float64(len(l.machines)) / float64(l.info.MachineCount)
			flow := 0.
			for _, id := range l.recipe.Results {
				flow += needed[id]
			}
			rate := flow * share
			if err := b.limiter(l.output[len(l.output)-1], back[len(back)-1], rate); err != nil {
				return result, err
			}
			if targets[l.info.MainItemID] > 0 {
				b.warnings = append(b.warnings, fmt.Sprintf("物品 %d 同时外供与内部消耗：导出时需保留总线回流运力", l.info.MainItemID))
			}
		} else {
			l.output[len(l.output)-1].record.Parameters = []int32{int32(l.info.MainItemID), 0}
		}
		if len(l.recipe.Results) > 1 {
			b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 存在副产物，请从输出带分流，避免总线积压", l.recipe.ID))
		}
	}
	b.owner = -1
	rawY := 3
	reportProgress(ctx, "bus-supply", "正在补充原料入口与喷涂供给", 0, 0)
	for _, raw := range req.Plan.RawInputs {
		var err error
		layer := schedule[raw.ItemID]
		rawY, err = b.rawSupplement(raw.ItemID, raw.RatePerMinute/60, layer, rings[layer], rawY)
		if err != nil {
			return result, err
		}
	}
	b.coatSupply(allLines)

	minimum, _ := bounds(b.all)
	shift(b.all, -minimum.x, -minimum.y, 0)
	if len(b.all) > blueprint.MaxBuildingCount {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线建筑超过 20 万，请拆分目标"}
	}
	return b.finish(ctx, req, dataset)
}
