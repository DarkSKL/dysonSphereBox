package build

// 文件职责：将真实L/U折行作为有限独立候选加入直连搜索。
// 使用：在两种短run跨度、两种容量分组与三种真实分流器间比较；边界：十二个候选均需完整接线、供电和球面终检，不强制用更差的折行替换直线。

import mode1common "DysonSphereBox/helper/blueprint/build/adapters/mode1/common"

// appendDenseBentVariants 职责：保留已验证的直角复合单元组合并比较其实际成本；使用：公开mode1密铺入口；边界：不对普通模式、塔或配送改写机器排布，不递归扩展候选。
func appendDenseBentVariants(req Request, limits []float64, variants []connectedLayoutVariant) []connectedLayoutVariant {
	if req.Mode != 1 || req.Density != densityDense || len(limits) == 0 {
		return variants
	}
	for _, variant := range variants {
		if variant.bentRows {
			return variants
		}
	}
	for _, fill := range []bool{false, true} {
		for _, span := range []float64{12, 18} {
			for _, model := range []int16{38, 39, 40} {
				variants = append(variants, connectedLayoutVariant{
					LayoutVariant: mode1common.LayoutVariant{Limit: limits[0], Compact: true, GlobalPower: true},
					denseSpan:     span, capacityFill: fill, producerSplits: true, splitterModel: model, bentRows: true,
				})
			}
		}
	}
	return variants
}
