package build

// 文件职责：将配送箱体分布到机器行端部的南北两侧，缩短逐物料接驳。
// 使用：配送模式紧凑候选；边界：保留独立物料、真实箱体臂与机器数量，自供剂闭环由原专用流程处理。

import (
	"math"
	"sort"
)

// flyPeripheralBlocks 职责：按物料带轨位向外扇出两组配送设施；使用：mode2 普通布局候选；边界：任一实体路径失败拒绝整个候选，不跳过碰撞或端口检查。
func (b *builder) flyPeripheralBlocks(l *line, begin int) error {
	ports := make([]flyIndependentPort, 0, len(l.inputs)+len(l.outputs)+1)
	lowY, highY, right := math.Inf(1), math.Inf(-1), math.Inf(-1)
	for index, row := range l.inputs {
		reverse(row)
		ports = append(ports, flyIndependentPort{item: l.recipe.Items[index], input: true, link: row[len(row)-1]})
	}
	for index, row := range l.outputs {
		ports = append(ports, flyIndependentPort{item: l.recipe.Results[index], link: row[len(row)-1]})
	}
	for _, port := range ports {
		lowY, highY = math.Min(lowY, port.link.a.y), math.Max(highY, port.link.a.y)
		right = math.Max(right, port.link.a.x)
	}
	if l.coaterInput != nil {
		entry := l.coaterInput
		points := manhattan(entry.a, vector{entry.a.x - 1, entry.a.y, 0}, true)
		points = append(points, manhattan(points[len(points)-1], vector{entry.a.x - 1, math.Min(lowY, entry.a.y) - 3, 0}, false)[1:]...)
		points = append(points, manhattan(points[len(points)-1], vector{right, points[len(points)-1].y, 0}, true)[1:]...)
		if err := b.validateFlyPathCandidate(points, entry, nil, b.all[begin:]); err != nil {
			return err
		}
		path := b.byPath(points[1:])
		reverse(path)
		connect(path[0], entry)
		ports = append(ports, flyIndependentPort{item: productionSprayItem(l.info), input: true, link: path[len(path)-1], role: sprayInputRole})
		lowY = math.Min(lowY, path[len(path)-1].a.y)
	}
	sort.SliceStable(ports, func(i, j int) bool { return ports[i].link.a.y < ports[j].link.a.y })
	// 由最外侧轨道向中间处理，内侧轨道在更远的横向位置折返，互不截断。
	split := (len(ports) + 1) / 2
	for side := 0; side < 2; side++ {
		count := split
		if side == 1 {
			count = len(ports) - split
		}
		for depth := 0; depth < count; depth++ {
			if err := b.checkGeneration("配送设施双侧接驳"); err != nil {
				return err
			}
			index, y := depth, lowY-4
			if side == 1 {
				index, y = len(ports)-1-depth, highY+4
			}
			port := ports[index]
			rate, err := flyPortRate(l.info, l.recipe, port, len(l.machines))
			if err != nil {
				return err
			}
			endpoint, err := b.flyAtRateApproach(port.item, port.link, port.input, right+5+5*float64(depth), y, 2, begin, rate, true)
			if err != nil {
				return err
			}
			port.link.record.Parameters = []int32{int32(port.item), 0}
			b.flyEndpoints = append(b.flyEndpoints, endpoint)
		}
	}
	return nil
}
