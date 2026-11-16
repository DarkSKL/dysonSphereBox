package build

// 文件职责：共享独立物料带之间的过滤分拣器装配。
// 使用：直连、配送及环路接驳；边界：沿用真实所选等级、跨度和过滤，最终容量与动态几何仍需验证。

import (
	"DysonSphereBox/helper/blueprint"
	"math"
)

// beltTransfer 职责：连接两条带上的分拣器；使用：取料/回流；边界：超 3 格不制造无效臂。
func (b *builder) beltTransfer(from, to *placed, level, filter int) (*placed, error) {
	distance := math.Abs(from.a.x-to.a.x) + math.Abs(from.a.y-to.a.y)
	if distance < .9 || distance > 3.01 || from.a.z != to.a.z {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线分拣器跨度超出 1..3 格"}
	}
	p := b.make(2010+level, from.a.x, from.a.y, from.a.z)
	p.a, p.b = from.a, to.a
	p.input = from
	p.output = to
	p.record.InputFromSlot = -1
	p.record.OutputToSlot = -1
	p.record.FilterID = int16(filter)
	yaw := math.Atan2(to.a.x-from.a.x, to.a.y-from.a.y) * 180 / math.Pi
	if yaw < 0 {
		yaw += 360
	}
	p.record.Yaw = float32(yaw)
	p.record.Yaw2 = p.record.Yaw
	p.record.Parameters = []int32{int32(math.Round(distance))}
	return p, nil
}
