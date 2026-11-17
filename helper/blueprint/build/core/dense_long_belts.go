package build

// 文件职责：把密铺全局路线的自由直线段重采样为以1.5个名义赤道格长为米制目标的真实带链。
// 使用：寻路完成、创建实体前；边界：保留端部三跳与转角，失败保留原路，不改变带等级或吞吐能力。

import (
	"context"
	"math"
)

// denseLongBeltGridSpan 职责：给出自由直带的目标节点间距；使用：几何重采样；边界：仍受米制距离、球面终检和实际节点占位限制。
const denseLongBeltGridSpan = 1.5

// denseLongBeltTargetMeters 职责：将名义赤道格一次换算为物理采样目标；使用：所有纬度自由直带；边界：不代表当前原生坐标格数，仍受denseRouteMaximumSpan硬约束。
const denseLongBeltTargetMeters = denseLongBeltGridSpan * planarGridLength

// denseLongBeltLongitudeFactor 职责：按实际选区给出经向物理格宽的保守上界；使用：长带重采样前；边界：旧自动请求覆盖全星球，不以赤道格宽假设高纬或高架安全。
func denseLongBeltLongitudeFactor(ctx context.Context) float64 {
	if ctx != nil {
		if root, _ := ctx.Value(densityPolarRootKey{}).(bool); root {
			return math.Inf(1)
		}
	}
	low, high := -250, 250
	nativeScale := 1.
	if ctx != nil {
		if zone, _ := ctx.Value(densityLatitudeKey{}).(string); zone != "" {
			if band, _, err := densityContextBand(ctx, zone, defaultSurfaceGridSegment); err == nil {
				low, high = band.low, band.high
				if selection, ok := densityLatitudeSelectionFor(zone); ok {
					if expand, _ := ctx.Value(denseAdjacentLatitudeKey{}).(bool); expand {
						bands := denseLatitudeAllowedBands(selection, true, denseAdjacentContextDepth(ctx))
						if len(bands) > 0 {
							low = min(low, bands[0].low)
							high = max(high, bands[len(bands)-1].high)
						}
					}
					if selection.tier >= 4 {
						// 无真实frame时不能假定某一子带段数；用所有可能缩放的上界，极点未知则停止重采样。
						pole := max(math.Abs(float64(low)), math.Abs(float64(high))) + 1
						if pole >= 250 {
							return math.Inf(1)
						}
						nativeScale = 1 / math.Cos(pole*math.Pi/500)
					}
				}
			}
		}
	}
	factor := 0.
	for latitude := max(-250, low-1); latitude <= min(250, high+1); latitude++ {
		near := max(0, math.Abs(float64(latitude))-1)
		factor = math.Max(factor, math.Cos(near*2*math.Pi/1000)*200/float64(longitudeSegmentCount(latitude, 200)))
	}
	return factor * nativeScale
}

// denseLongBeltAxis 职责：识别同高度、同向的正交直段；使用：合并连续自由段；边界：斜线、升降和退回均不合并。
func denseLongBeltAxis(a, c vector) (axis int, direction float64) {
	if math.Abs(a.z-c.z) > 1e-8 {
		return -1, 0
	}
	if math.Abs(a.y-c.y) <= 1e-8 && math.Abs(a.x-c.x) > 1e-8 {
		return 0, math.Copysign(1, c.x-a.x)
	}
	if math.Abs(a.x-c.x) <= 1e-8 && math.Abs(a.y-c.y) > 1e-8 {
		return 1, math.Copysign(1, c.y-a.y)
	}
	return -1, 0
}

// compactDenseLongRoute 职责：减少自由直段的带节点并复核整条接线；使用：所有密铺公共路由；边界：输入不含源、含目标，原实体及已建立尾链不移动，已有端口连接许可跳数保持不变。
func (env *transportEnvironment) compactDenseLongRoute(points []vector, source, target *placed, longitudeFactors ...float64) ([]vector, int) {
	if env.densePorts == nil || source == nil || target == nil || len(points) < 9 {
		return points, 0
	}
	full := append([]vector{source.a}, points...)
	longitudeFactor := 1.
	if len(longitudeFactors) > 0 {
		longitudeFactor = longitudeFactors[0]
	}
	endLimit := len(full) - 4
	result := append([]vector(nil), full[:4]...)
	changes := []denseLongBeltChange{}
	for start := 3; start < endLimit; {
		end := start + 1
		axis, direction := denseLongBeltAxis(full[start], full[end])
		if axis >= 0 {
			for end < endLimit {
				nextAxis, nextDirection := denseLongBeltAxis(full[end], full[end+1])
				if nextAxis != axis || nextDirection != direction {
					break
				}
				end++
			}
		}
		first, last := full[start], full[end]
		distance := math.Abs(last.x-first.x) + math.Abs(last.y-first.y)
		// 使用真实坐标度量换算米制目标；原生float32投影留1毫米计算余量，最终仍逐段执行原米制硬检查。
		metersPerGrid, margin := env.denseLongBeltMetersPerGrid(first, last, axis, longitudeFactor)
		if !finitePositive(metersPerGrid) {
			return points, 0
		}
		span := (math.Min(denseLongBeltTargetMeters, denseRouteMaximumSpan) - margin) / metersPerGrid
		count := max(1, int(math.Ceil(distance/span-1e-9)))
		if axis >= 0 && count < end-start {
			changeStart := len(result)
			// 等分末段避免留下过短尾节；实际格数由当前纬线和高度的米制格宽决定。
			for n := 1; n <= count; n++ {
				fraction := float64(n) / float64(count)
				point := vector{first.x + (last.x-first.x)*fraction, first.y + (last.y-first.y)*fraction, first.z}
				if n == count {
					point = last
				}
				result = append(result, point)
			}
			changes = append(changes, denseLongBeltChange{start, end, append([]vector(nil), result[changeStart:]...)})
		} else {
			result = append(result, full[start+1:end+1]...)
		}
		start = end
	}
	result = append(result, full[endLimit+1:]...)
	if len(result) >= len(full) {
		return points, 0
	}
	if !env.denseLongBeltCandidateClear(result, source, target) {
		return env.denseLongBeltPartial(full, changes, source, target)
	}
	return result[1:], len(full) - len(result)
}

// denseLongBeltMetersPerGrid 职责：给出当前直段实际坐标的弧长度量和计算余量。
// 使用：重采样前；边界：真实frame优先且不混用子Area步长，未知非原生缩放由保守factor约束，最终world检查不省略。
func (env *transportEnvironment) denseLongBeltMetersPerGrid(first, last vector, axis int, longitudeFactor float64) (float64, float64) {
	radius := 200.4 + max(first.z, last.z)*1.3333333
	if frame := env.densePorts.polarFrame; frame != nil {
		if axis == 0 {
			latitude := float64(float32(frame.start)*frame.latitudeStep + float32(first.y)*frame.latitudeStep)
			return radius * math.Abs(math.Cos(latitude)) * float64(frame.longitudeStep), .001
		}
		return radius * float64(frame.latitudeStep), .001
	}
	meters := radius * 2 * math.Pi / 1000
	if axis == 0 {
		meters *= longitudeFactor
	}
	return meters, 0
}

// denseLongBeltChange 职责：保存可独立撤回的自由直段采样；使用：有界混合候选；边界：原端点索引和坐标不变，不包含端三跳或拐点迁移。
type denseLongBeltChange struct {
	start, end int
	points     []vector
}

// denseLongBeltCandidateClear 职责：对混合路线执行相同完整预留和物理检查；使用：每个拟接受候选；边界：不以局部成功替代整链净空及自距复核。
func (env *transportEnvironment) denseLongBeltCandidateClear(points []vector, source, target *placed) bool {
	for _, point := range points[1 : len(points)-1] {
		if env.reservedByOther(point, source, target) {
			return false
		}
	}
	return env.densePathClear(points, source, target)
}

// denseLongBeltPartial 职责：全量重采样失败后，在最多八次完整检查内保留部分可用直段。
// 使用：一个直段失败不应抹掉其它直段的合法收益；边界：每次接受的是完整混合路线，失败区间恢复原段，不改变间距、实体上限或A*预算。
func (env *transportEnvironment) denseLongBeltPartial(full []vector, changes []denseLongBeltChange, source, target *placed) ([]vector, int) {
	if len(changes) < 2 {
		return full[1:], 0
	}
	enabled := make([]bool, len(changes))
	middle := len(changes) / 2
	stack := [][2]int{{middle, len(changes)}, {0, middle}}
	var best []vector
	for probes := 0; probes < 8 && len(stack) > 0; probes++ {
		interval := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for i := interval[0]; i < interval[1]; i++ {
			enabled[i] = true
		}
		// 全量候选已由调用方证明失败。仍占用原预算位置并走相同拆分，省去重复构造和整链检查，不扩大搜索机会。
		knownFailure := true
		for _, active := range enabled {
			knownFailure = knownFailure && active
		}
		if knownFailure {
			for i := interval[0]; i < interval[1]; i++ {
				enabled[i] = false
			}
			if interval[1]-interval[0] > 1 {
				mid := (interval[0] + interval[1]) / 2
				stack = append(stack, [2]int{mid, interval[1]}, [2]int{interval[0], mid})
			}
			continue
		}
		candidate := []vector{}
		cursor := 0
		for i, change := range changes {
			candidate = append(candidate, full[cursor:change.start+1]...)
			if enabled[i] {
				candidate = append(candidate, change.points...)
			} else {
				candidate = append(candidate, full[change.start+1:change.end+1]...)
			}
			cursor = change.end + 1
		}
		candidate = append(candidate, full[cursor:]...)
		if env.denseLongBeltCandidateClear(candidate, source, target) {
			best = candidate
			continue
		}
		for i := interval[0]; i < interval[1]; i++ {
			enabled[i] = false
		}
		if interval[1]-interval[0] > 1 {
			mid := (interval[0] + interval[1]) / 2
			stack = append(stack, [2]int{mid, interval[1]}, [2]int{interval[0], mid})
		}
	}
	if len(best) == 0 {
		return full[1:], 0
	}
	return best[1:], len(full) - len(best)
}
