package build

// 文件职责：把密铺全局路线的自由直线段重采样为最长1.5格的真实带链。
// 使用：寻路完成、创建实体前；边界：保留端部三跳与转角，失败保留原路，不改变带等级或吞吐能力。

import (
	"context"
	"math"
)

// denseLongBeltGridSpan 职责：给出自由直带的目标节点间距；使用：几何重采样；边界：仍受米制距离、球面终检和实际节点占位限制。
const denseLongBeltGridSpan = 1.5

// denseLongBeltLongitudeFactor 职责：按实际选区给出经向物理格宽的保守上界；使用：长带重采样前；边界：旧自动请求覆盖全星球，不以赤道格宽假设高纬或高架安全。
func denseLongBeltLongitudeFactor(ctx context.Context) float64 {
	if ctx != nil {
		if root, _ := ctx.Value(densityPolarRootKey{}).(bool); root {
			return 4
		}
	}
	low, high := -250, 250
	nativeScale := 1.
	if ctx != nil {
		if zone, _ := ctx.Value(densityLatitudeKey{}).(string); zone != "" {
			if band, _, err := densityContextBand(ctx, zone, defaultSurfaceGridSegment); err == nil {
				low, high = band.low, band.high
				if selection, ok := densityLatitudeSelectionFor(zone); ok && selection.tier >= 4 {
					pole := math.Max(math.Abs(float64(low)), math.Abs(float64(high)))
					nativeScale = float64(band.segments) / 200 / math.Cos(pole*math.Pi/500)
				}
			}
		}
	}
	factor := 0.
	for latitude := low; latitude <= high; latitude++ {
		factor = math.Max(factor, math.Cos(float64(latitude)*2*math.Pi/1000)*200/float64(longitudeSegmentCount(latitude, 200)))
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
		// 使用弧长上界保护高架和纬带格宽；选区变窄时不能把平面合法误当作球面长度合法。
		metersPerGrid := (200.4 + math.Max(first.z, last.z)*1.3333333) * 2 * math.Pi / 1000
		if axis == 0 {
			factor := longitudeFactor
			if frame := env.densePorts.polarFrame; frame != nil {
				// 原生极区路线已定位：沿本条纬线计算真实弧长，避免统一最坏格距使长带失效。
				latitude := (float64(frame.start) + first.y) * float64(frame.latitudeStep)
				factor = math.Abs(math.Cos(latitude)) * float64(frame.longitudeStep) / (2 * math.Pi / 1000)
			}
			metersPerGrid *= factor
		}
		span := math.Min(denseLongBeltGridSpan, denseRouteMaximumSpan/metersPerGrid)
		count := max(1, int(math.Ceil(distance/span-1e-9)))
		if axis >= 0 && count < end-start {
			// 等分末段避免留下过短尾节；长度整除时直接得到1.5格节点间距。
			for n := 1; n <= count; n++ {
				fraction := float64(n) / float64(count)
				point := vector{first.x + (last.x-first.x)*fraction, first.y + (last.y-first.y)*fraction, first.z}
				if n == count {
					point = last
				}
				result = append(result, point)
			}
		} else {
			result = append(result, full[start+1:end+1]...)
		}
		start = end
	}
	result = append(result, full[endLimit+1:]...)
	if len(result) >= len(full) {
		return points, 0
	}
	for _, point := range result[1 : len(result)-1] {
		if env.reservedByOther(point, source, target) {
			return points, 0
		}
	}
	if !env.densePathClear(result, source, target) {
		return points, 0
	}
	return result[1:], len(full) - len(result)
}
