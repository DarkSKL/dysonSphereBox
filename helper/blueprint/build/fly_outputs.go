package build

// 文件职责：把多产物机器的独立物料带接入互不混料的配送端点银行。
// 使用：mode2/5多产物机器行；边界：最多六种机器物料和一个供剂入口，仅改变局部路径，不假造机队吞吐。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// flyIndependentPort 职责：保存独立机器物料口及其配送方向；使用：真实轨位排序；边界：每个产物只能引用自己的机器出货带。
type flyIndependentPort struct {
	item  int
	input bool
	link  *placed
}

// flyIndependentOutputBlocks 职责：按真实轨位给所有原料和产物分配有限阶梯通道；使用：多产物行末配送；边界：不假设出货带都在机器上方，禁止不同物品共享实体带。
func (b *builder) flyIndependentOutputBlocks(l *line, mode, begin int) error {
	rows := l.outputRows()
	if len(rows) != len(l.recipe.Results) || len(l.outputs) == 0 || len(l.inputs) != len(l.recipe.Items) {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的多产物配送缺少逐物品独立机器出货带", l.recipe.ID)}
	}
	ports := make([]flyIndependentPort, 0, len(l.inputs)+len(rows)+1)
	seen := map[*placed]bool{}
	lowY, highY, right := math.Inf(1), math.Inf(-1), math.Inf(-1)
	register := func(item int, input bool, row []*placed) error {
		if len(row) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 物品 %d 缺少真实机器物料带", l.recipe.ID, item)}
		}
		for _, belt := range row {
			if belt == nil || seen[belt] || !surfaceIsBelt(belt.record.ItemID) {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 不同物料口重用出货带或包含无效实体", l.recipe.ID)}
			}
			seen[belt] = true
			lowY, highY = math.Min(lowY, belt.a.y), math.Max(highY, belt.a.y)
			right = math.Max(right, belt.a.x)
		}
		if input {
			reverse(row)
		}
		ports = append(ports, flyIndependentPort{item: item, input: input, link: row[len(row)-1]})
		return nil
	}
	for index, row := range l.inputs {
		if err := register(l.recipe.Items[index], true, row); err != nil {
			return err
		}
	}
	for _, item := range l.recipe.Results {
		if err := register(item, false, l.outputForItem(item)); err != nil {
			return err
		}
	}
	if l.coaterInput != nil {
		entry := l.coaterInput
		// 喷涂入口在机器带西侧先降至全部真实轨位之外，避免任意底侧产物
		// 被旧的固定两格供剂横带截断；这里只增加供剂自己的独立路线。
		points := manhattan(entry.a, vector{entry.a.x - 1, entry.a.y, 0}, true)
		points = append(points, manhattan(points[len(points)-1], vector{entry.a.x - 1, math.Min(lowY, entry.a.y) - 3, 0}, false)[1:]...)
		points = append(points, manhattan(points[len(points)-1], vector{right, points[len(points)-1].y, 0}, true)[1:]...)
		if err := b.validateFlyPathCandidate(points, entry, nil, b.all[begin:]); err != nil {
			return err
		}
		path := b.byPath(points[1:])
		reverse(path)
		connect(path[0], entry)
		ports = append(ports, flyIndependentPort{item: productionSprayItem(l.info), input: true, link: path[len(path)-1]})
	}
	if len(ports) > 7 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "多产物配送超过六种机器物料加一个供剂入口的有限通道预算"}
	}
	sort.SliceStable(ports, func(i, j int) bool { return ports[i].link.a.y > ports[j].link.a.y })
	baseOffset, step, clearance := 18., 6., 8.
	if b.flyCompact {
		baseOffset, step, clearance = 13, 5, 4
	}
	for index, port := range ports {
		if err := b.checkGeneration("多产物独立配送端点"); err != nil {
			return err
		}
		x := right + baseOffset + step*float64(index)
		y := highY + clearance + 3*float64(len(ports)-1-index)
		rate := flyLineRate(l.info, port.item, port.input, len(l.machines))
		// 从高到低的物料口逐一向更远的竖轨展开，设施高度反向递减；
		// 横段只经过尚未开始的低轨，竖段不会穿过已装配的高轨设施。
		endpoint, err := b.flyAtRateApproach(port.item, port.link, port.input, x, y, mode, begin, rate, true)
		if err != nil {
			return err
		}
		port.link.record.Parameters = []int32{int32(port.item), 0}
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	return nil
}
