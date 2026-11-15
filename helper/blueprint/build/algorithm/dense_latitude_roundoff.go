package build

// 文件职责：消除原生纬向整数边界的浮点尾差。
// 使用：生成Area包络前；边界：限于原版球面的float64舍入量级，不放宽实际几何校验。

import "math"

// normalizeDenseLatitudeRoundoff 职责：消除仿射坐标运算产生的零边界尾差；使用：原生Area整数包络计算；边界：仅允许当前纬向量级的八个float64 ULP，实际负坐标和实体越界仍拒绝。
func normalizeDenseLatitudeRoundoff(all []*placed, low, high float64) (float64, float64) {
	scale := math.Max(1, math.Max(math.Abs(low), math.Abs(high)))
	if math.IsInf(scale, 0) || math.IsNaN(scale) {
		return low, high
	}
	// 原版纬向一圈最多500格，不让非法巨大坐标扩大数值容差。
	scale = math.Min(scale, 500)
	epsilon := (math.Nextafter(scale, math.Inf(1)) - scale) * 8
	if low < 0 && low >= -epsilon {
		low = 0
		for _, p := range all {
			if p.a.y < 0 && p.a.y >= -epsilon {
				p.a.y = 0
			}
			if p.b.y < 0 && p.b.y >= -epsilon {
				p.b.y = 0
			}
		}
	}
	if math.Abs(high-math.Round(high)) <= epsilon {
		high = math.Round(high)
	}
	return low, high
}
