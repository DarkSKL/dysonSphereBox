package build

// 文件职责：按共享总线的物理带宽规划原料入口的门控和回收规模。
// 使用：总线高度估算与实际装配共用同一规划；边界：不减少生产机器或原料需求，超带宽仍须向用户明确告警。

import "math"

// busSupply 职责：保存一个原料入口的可实现门控配置；使用：估高与装配；边界：requested 保留原需求，effective 不超过单条传送带运力。
type busSupply struct {
	requested, effective, remaining float64
	feed                            []transfer
	returnCount                     int
}

// planBusSupply 职责：消除同一单带入口无法利用的重复补料及回收设施；使用：rawSupplement 和总线高度估算；边界：不把受限入口当成满足完整目标产率。
func planBusSupply(need float64, layer int, setting Setting) busSupply {
	capacity := []float64{0, 6, 12, 30}[setting.BeltLv]
	plan := busSupply{requested: need, effective: math.Min(math.Ceil(need*2)/2, capacity)}
	plan.feed, plan.remaining = transferPlan(plan.effective, layer+1, setting.InserterLv)
	table := [][]float64{{}, {1.5, .75, .5}, {3, 1.5, 1}, {6, 3, 2}}
	power := table[min(3, setting.InserterLv)][layer]
	// 回收优先仍保留超过入口运力的一只门控；超出整条带宽的额外门控没有吞吐收益。
	plan.returnCount = int(math.Floor(plan.effective/power)) + 1
	return plan
}
