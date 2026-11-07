package build

// 文件职责：为小数机器节距联合创建局部物料带与分拣器锚点。
// 使用：dense 机器行的基础候选；边界：不是完整双排密铺，所有候选仍经过相同终检，未知机器类型拒绝。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// denseLineSpacing 职责：提供基础密集单排实验节距；使用：局部机器和运输联合装配；边界：这些值不声明游戏已验证，最终碰撞失败不得退回普通结果。
func denseLineSpacing(kind, extra int) (float64, error) {
	// 炼油厂按270度朝向的7.2米建造盒取5.8格，对撞机按10.4米最大建造盒取8.8格。
	// 这两类没有参考图中的低层轨道，使用独立物料带的紧凑单排，并继续执行完整终检。
	pitch, ok := map[int]float64{1: 2.8, 2: 3.4, 3: 5.8, 4: 6.8, 5: 8.8, 6: 4.8}[kind]
	if !ok {
		return 0, &blueprint.Error{Code: "GEN_E_DENSITY_UNSUPPORTED", Message: fmt.Sprintf("机器类型 %d 尚无密集局部运输候选", kind)}
	}
	return pitch + float64(extra), nil
}

// denseTrackPositions 职责：保留每台机器三个真实带侧槽位锚点并补齐短连接；使用：浮点节距机器行；边界：不将小数锚点吸附整数，不创建长于一格的直线段。
func denseTrackPositions(count int, spacing, start, end float64) []float64 {
	// 外围路由继续使用整数端点；尾端位于最后一个槽位之后，避免截出小于安全间距的孤立短尾。
	end = math.Max(end, math.Ceil(float64(count-1)*spacing+start+2)+2)
	anchors := []float64{0, end}
	for machine := 0; machine < count; machine++ {
		for slot := 0; slot < 3; slot++ {
			anchors = append(anchors, float64(machine)*spacing+start+float64(slot))
		}
	}
	sort.Float64s(anchors)
	positions := []float64{anchors[0]}
	for _, x := range anchors[1:] {
		last := positions[len(positions)-1]
		if x-last < 1e-6 {
			continue
		}
		steps := int(math.Ceil(x - last))
		for step := 1; step <= steps; step++ {
			positions = append(positions, last+(x-last)*float64(step)/float64(steps))
		}
	}
	return positions
}

// denseTrack 职责：把独立物料的浮点锚点落实为真实有向带；使用：dense 局部装配；边界：各物料独立创建，不共享实体。
func (b *builder) denseTrack(positions []float64, y float64) []*placed {
	points := make([]vector, len(positions))
	for index, x := range positions {
		points[index] = vector{x, y, 0}
	}
	return b.byPath(points)
}

// machineBeltAnchor 职责：读取与机器槽位对应的带实体；使用：普通或密集机器臂装配；边界：普通路径保留原索引，dense 按实际锚点匹配而不是截断小数节距。
func (b *builder) machineBeltAnchor(belt []*placed, machine, slot int, spacing, start, pre float64) (*placed, error) {
	if b.adapter == nil || b.adapter.density != densityDense {
		return belt[machine*int(spacing)+int(start)+slot+int(pre)], nil
	}
	x := float64(machine)*spacing + float64(int(start)+slot+int(pre))
	index := sort.Search(len(belt), func(index int) bool { return belt[index].a.x >= x-1e-6 })
	if index == len(belt) || math.Abs(belt[index].a.x-x) > 1e-6 {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "密集机器槽位没有对应的局部带锚点"}
	}
	return belt[index], nil
}
