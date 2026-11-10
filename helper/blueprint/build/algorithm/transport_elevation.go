package build

// 文件职责：为独立传送带复用已占用的平面走廊提供软代价。
// 使用：普通与反向合流 A* 共用；边界：相同平面格的不同高度仍是独立实体，不强制高架或放宽占位与连接规则。

import "math"

// denseRouteHeightPenalty 职责：按水平距离累计高空线路的额外代价；使用：超密度寻路及完整接驳比较；边界：是软目标，不降低可达高度或更改碰撞规则。
const denseRouteHeightPenalty = .08

// denseRouteOuterPenalty 职责：计入路线越出接线前实体包络的距离；使用：引导路线利用建筑间空隙；边界：保留外围可达空间，不将原包络当硬墙。
const denseRouteOuterPenalty = .2

// transportStepCost 职责：按连续位移计算主路及分数接驳的同一代价；使用：整数A*、斜接驳和完整路线比较；边界：两端均在走廊才算复用，代价正反对称，不把每个分数节点重复计为一整格。
func (env *transportEnvironment) transportStepCost(from, to vector) float64 {
	horizontal := math.Hypot(from.x-to.x, from.y-to.y)
	weight := 1.
	if env.preferCorridor && !(env.beltCorridors[beltColumn(from)] && env.beltCorridors[beltColumn(to)]) {
		weight = 1.25
	}
	if env.densePorts != nil && !env.densePorts.strictBelts {
		weight += denseRouteHeightPenalty * math.Max(0, (from.z+to.z)/2)
		weight += denseRouteOuterPenalty * (env.transportFootprintOverflow(from) + env.transportFootprintOverflow(to)) / 2
	}
	return horizontal*weight + math.Abs(from.z-to.z)*1.5
}

// transportFootprintOverflow 职责：计算离原实体包络的平面距离；使用：完整路线统一计价；边界：不读取已铺路线的扩展包络，不影响未登记包络的独立寻路。
func (env *transportEnvironment) transportFootprintOverflow(point vector) float64 {
	index := env.densePorts
	if index == nil || !index.footprintReady {
		return 0
	}
	return math.Max(0, index.footprintLow.x-point.x) + math.Max(0, point.x-index.footprintHigh.x) + math.Max(0, index.footprintLow.y-point.y) + math.Max(0, point.y-index.footprintHigh.y)
}

// transportElevationLowerBound 职责：给出水平高度成本与必要升降的联合一致下界；使用：单目标、多目标及反向合流共用；边界：放松障碍后比较原高度与下到地面，不计外绕罚项，保持可采纳性。
func (env *transportEnvironment) transportElevationLowerBound(from, to vector) float64 {
	vertical := math.Abs(from.z-to.z) * 1.5
	if env.densePorts == nil || env.densePorts.strictBelts {
		return vertical
	}
	horizontal := math.Abs(from.x-to.x) + math.Abs(from.y-to.y)
	return math.Min(vertical+denseRouteHeightPenalty*horizontal*math.Min(from.z, to.z), (from.z+to.z)*1.5)
}

// beltColumn 职责：获得真实带格的平面投影；使用：跨高度走廊索引；边界：不代替三维碰撞或连接核验。
func beltColumn(point vector) [2]int {
	return [2]int{int(math.Round(point.x)), int(math.Round(point.y))}
}

// transportHeuristic 职责：给出忽略三维障碍的保守一致下界；使用：正向与反向 A*；边界：势场过期或场外时只使用曼哈顿，不假定未知路径必定复用走廊。
func (env *transportEnvironment) transportHeuristic(from, to vector) float64 {
	horizontal := math.Abs(from.x-to.x) + math.Abs(from.y-to.y)
	if field := env.corridorPotential; env.preferCorridor && field != nil && field.version == env.version {
		left, leftOK := field.distanceAt(from)
		right, rightOK := field.distanceAt(to)
		if leftOK && rightOK {
			horizontal = math.Max(horizontal, math.Abs(left-right))
		}
	}
	return horizontal + env.transportElevationLowerBound(from, to)
}

// transportPathCost 职责：按同一软代价比较已找到的端点候选；使用：消费端合法邻格择优；边界：不以较少节点覆盖能有效复用走廊的路线。
func (env *transportEnvironment) transportPathCost(path []vector) float64 {
	total := 0.
	for i := 1; i < len(path); i++ {
		total += env.transportStepCost(path[i-1], path[i])
	}
	return total
}
