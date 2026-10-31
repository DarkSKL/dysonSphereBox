package build

// 文件职责：按共享总线的物理带宽规划原料入口的门控和回收规模。
// 使用：总线高度估算与实际装配共用同一规划；边界：不减少生产机器或原料需求，超带宽仍须向用户明确告警。

import (
	"DysonSphereBox/helper/transport"
	"math"
)

// busSupply 职责：保存一个原料入口的可实现门控配置；使用：估高与装配；边界：requested 保留原需求，effective 不超过单条传送带运力。
type busSupply struct {
	requested, effective, remaining float64
	feed                            []transfer
	returnCount                     int
}

// planBusSupply 职责：消除同一单带入口无法利用的重复补料及回收设施；使用：rawSupplement 和总线高度估算；边界：不把受限入口当成满足完整目标产率。
func planBusSupply(need float64, layer int, setting Setting) busSupply {
	perMinute, _ := transport.BeltCapacityPerMinute(setting.BeltLv)
	capacity := perMinute / 60
	plan := busSupply{requested: need, effective: math.Min(math.Ceil(need*2)/2, capacity)}
	plan.feed, plan.remaining = transferPlan(plan.effective, layer+1, setting.InserterLv)
	returnLevel := min(4, setting.InserterLv)
	returnCapacity, err := transport.InserterCapacityPerMinute(returnLevel, layer+1)
	if err != nil || returnCapacity <= 0 {
		plan.returnCount = 0
	} else {
		// 回收臂按固定真实跨度核算；多臂才能覆盖单入口门控的有效流量。
		plan.returnCount = int(math.Ceil(plan.effective/(returnCapacity/60) - 1e-9))
	}
	return plan
}
