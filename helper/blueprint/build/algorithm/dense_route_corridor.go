package build

// 文件职责：在原生球面长线路穷尽全域展开预算后，补充有限局部走廊候选。
// 使用：密铺端口前后缀之间的整数主路；边界：保留原搜索成功结果、实际碰撞和高度规则，不把局部无解解释为全域无解。

import (
	"context"
	"math"
)

// findDenseNativeDestinationPath 职责：复用同一精确A*在较小平面走廊中重试预算不足的长线；使用：原生密铺全域未找到任何路径之后；边界：每次仍受原展开上限，调用及展开如实累计，任何退出均恢复完整边界及启发式缓存。
func (env *transportEnvironment) findDenseNativeDestinationPath(ctx context.Context, start vector, goals []transportDestination, source, target *placed) (path []vector, selected, expanded, peak, calls int) {
	path, selected, expanded = env.findTransportDestinationPathWithSuffix(ctx, start, goals, source, target)
	peak, calls = expanded, 1
	if len(path) > 0 || ctx.Err() != nil || expanded < transportSearchLimit || env.densePorts == nil || env.densePorts.polarFrame == nil {
		return
	}
	minimum, maximum, potential := env.minimum, env.maximum, env.corridorPotential
	defer func() { env.minimum, env.maximum, env.corridorPotential = minimum, maximum, potential }()
	low, high := start, start
	found := false
	for _, goal := range goals {
		p := goal.point
		if p.x < minimum.x || p.x > maximum.x || p.y < minimum.y || p.y > maximum.y || p.z < 0 || p.z > transportMaxHeight || goal.cost < 0 || math.IsNaN(goal.cost) || math.IsInf(goal.cost, 0) || goal.length < 0 {
			continue
		}
		low.x, low.y = math.Min(low.x, p.x), math.Min(low.y, p.y)
		high.x, high.y = math.Max(high.x, p.x), math.Max(high.y, p.y)
		found = true
	}
	if !found {
		return
	}
	area := (maximum.x - minimum.x + 1) * (maximum.y - minimum.y + 1)
	for _, padding := range []float64{8, 16} {
		if ctx.Err() != nil {
			return
		}
		localLow := vector{math.Max(minimum.x, math.Floor(low.x)-padding), math.Max(minimum.y, math.Floor(low.y)-padding), minimum.z}
		localHigh := vector{math.Min(maximum.x, math.Ceil(high.x)+padding), math.Min(maximum.y, math.Ceil(high.y)+padding), maximum.z}
		if (localHigh.x-localLow.x+1)*(localHigh.y-localLow.y+1) >= area*.75 {
			continue
		}
		env.minimum, env.maximum = localLow, localHigh
		env.corridorPotential = potential
		var steps int
		path, selected, steps = env.findTransportDestinationPathWithSuffix(ctx, start, goals, source, target)
		expanded, peak, calls = expanded+steps, max(peak, steps), calls+1
		if len(path) > 0 {
			return
		}
	}
	return
}
