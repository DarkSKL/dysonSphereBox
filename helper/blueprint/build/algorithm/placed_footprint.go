package build

// 文件职责：让已核对的局部布局在旋转后仍使用正确的宽高预算。
// 使用：实体包络；边界：仅影响布局估算，真正碰撞仍由原生定向盒和最终球面规则核验。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// prototypeLayoutFootprint 职责：从所选原型的真实建造盒计算零度局部占地；使用：原生球面排布与炼油参考单元；边界：仅提供布局估算，未知模型失败，最终仍按球面三维盒校验。
func prototypeLayoutFootprint(item, model int16) (*[2]vector, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	shapes, err := catalog.Shapes(item, model)
	if err != nil {
		return nil, err
	}
	low, high := vector{math.Inf(1), math.Inf(1), 0}, vector{math.Inf(-1), math.Inf(-1), 0}
	encoding := newPlanarRuleEncoding()
	for _, shape := range shapes {
		box, err := encoding.colliderBox(vector{0, .2, 0}, 0, shape)
		if err != nil {
			return nil, err
		}
		a, c := box.Bounds()
		low.x = math.Min(low.x, a[0]/planarGridLength)
		low.y = math.Min(low.y, a[2]/planarGridLength)
		high.x = math.Max(high.x, c[0]/planarGridLength)
		high.y = math.Max(high.y, c[2]/planarGridLength)
	}
	return &[2]vector{low, high}, nil
}

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
