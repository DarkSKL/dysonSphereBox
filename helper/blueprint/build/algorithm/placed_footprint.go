package build

// 文件职责：让已核对的局部布局在旋转后仍使用正确的宽高预算。
// 使用：实体包络；边界：仅影响布局估算，真正碰撞仍由原生定向盒和最终球面规则核验。

import "math"

// placedFootprint 职责：旋转Yaw零度下已核对的局部外包；使用：非默认朝向的原型；边界：无专用数据时保留原有估算，不改原型建造盒。
func placedFootprint(p *placed) (vector, vector) {
	if p.layoutFootprint == nil {
		return footprint(p.record.ItemID)
	}
	low, high := vector{math.Inf(1), math.Inf(1), 0}, vector{math.Inf(-1), math.Inf(-1), 0}
	s, c := math.Sincos(float64(p.record.Yaw) * math.Pi / 180)
	for _, x := range []float64{p.layoutFootprint[0].x, p.layoutFootprint[1].x} {
		for _, y := range []float64{p.layoutFootprint[0].y, p.layoutFootprint[1].y} {
			rotatedX, rotatedY := c*x+s*y, -s*x+c*y
			low.x, high.x = math.Min(low.x, rotatedX), math.Max(high.x, rotatedX)
			low.y, high.y = math.Min(low.y, rotatedY), math.Max(high.y, rotatedY)
		}
	}
	return low, high
}
