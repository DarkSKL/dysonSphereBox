package build

// 文件职责：从最终指针图提取物流证据、端点与空间指标。
// 使用：统一编号后编码前；边界：不把理论 lane 数写成不存在的实体带，不因表面解码成功省略流边检查。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
)

// footprintForMetrics 职责：按原有保守 footprint 和分拣器两端统计格网并集；使用：空间指标；边界：只用于度量，不代替碰撞目录。
func footprintForMetrics(p *placed) (vector, vector) {
	low, high := footprint(p.record.ItemID)
	return vector{math.Min(p.a.x+low.x, p.b.x), math.Min(p.a.y+low.y, p.b.y), p.a.z}, vector{math.Max(p.a.x+high.x, p.b.x), math.Max(p.a.y+high.y, p.b.y), p.a.z}
}

// finalRecordFootprintArea 职责：按最终编码坐标及区域身份度量格网足迹；使用：球面重定位和多区域结果；边界：沿用保守轮廓估计，不作为真实碰撞或地表面积证明。
func finalRecordFootprintArea(records []blueprint.Building) float64 {
	cells := map[[3]int]bool{}
	for _, record := range records {
		p := placed{record: record, a: vector{float64(record.LocalOffset.X), float64(record.LocalOffset.Y), float64(record.LocalOffset.Z)}, b: vector{float64(record.LocalOffset2.X), float64(record.LocalOffset2.Y), float64(record.LocalOffset2.Z)}}
		low, high := footprintForMetrics(&p)
		for x := int(math.Floor(low.x)); x <= int(math.Floor(high.x)); x++ {
			for y := int(math.Floor(low.y)); y <= int(math.Floor(high.y)); y++ {
				cells[[3]int{int(record.AreaIndex), x, y}] = true
			}
		}
	}
	return float64(len(cells))
}
