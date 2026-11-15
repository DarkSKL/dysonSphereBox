package build

// 文件职责：在生成几何前消除由实际容量组决定的重复街道候选。
// 使用：公开密铺候选规划；边界：只消除已证明参数等价的计划，不缓存失败或剪去不同布局。

import mode1common "DysonSphereBox/helper/blueprint/build/adapters/mode1/common"

// denseStreetVariantIdentity 职责：保存除已归一街道分组以外全部影响布局的参数；使用：精确去重；边界：不同朝向、跨度、微调和供电策略不能合并。
type denseStreetVariantIdentity struct {
	// plan 职责：保存实际容量分组键；使用：比对语义等价计划；边界：不是几何通过证明。
	plan string
	// base 职责：保留公共布局参数；使用：完整身份；边界：任何参数不同均不可复用。
	base mode1common.LayoutVariant
	// order/turns 职责：保存排序与朝向；使用：区分独立几何候选；边界：不能因分组相同而省略朝向。
	order, turns int
	// span 职责：保存额外跨度约束；使用：行分组身份；边界：不同限制单独保留。
	span float64
	// refine 职责：标识连续位置微调；使用：候选身份；边界：与整数候选不合并。
	refine bool
	// pairedOutputs 职责：区分单侧与双侧过滤几何；使用：候选完整身份；边界：不能因机器分组相同而合并两种出货拓扑。
	pairedOutputs bool
	// capacityFill 职责：区分均衡与容量优先分组；使用：完整去重身份；边界：即使某次counts相同也不把不同策略误合并。
	capacityFill bool
	// producerSplits/splitterModel 职责：保留分流拓扑与真实模型身份；使用：街道去重；边界：容量组相同不代表端口、体积及路线相同。
	producerSplits bool
	splitterModel  int16
	// bentRows 职责：区分折行实体变换；使用：完整策略身份；边界：不和同容量的直线计划合并。
	bentRows bool
}

// deduplicateDenseStreetVariants 职责：保持首个等价计划及原顺序；使用：公开候选入口；边界：未知分组键原样保留，空的已知键只表示没有适配产线。
func deduplicateDenseStreetVariants(variants []connectedLayoutVariant, keys map[int]string, capacityKeys ...map[int]string) ([]connectedLayoutVariant, int) {
	result := make([]connectedLayoutVariant, 0, len(variants))
	seen := map[denseStreetVariantIdentity]bool{}
	for _, variant := range variants {
		if !variant.street {
			result = append(result, variant)
			continue
		}
		strategyKeys := keys
		if variant.capacityFill {
			strategyKeys = nil
			if len(capacityKeys) > 0 {
				strategyKeys = capacityKeys[0]
			}
		}
		plan, known := strategyKeys[variant.streetGroupLimit]
		if !known {
			result = append(result, variant)
			continue
		}
		if plan == "" {
			continue
		}
		key := denseStreetVariantIdentity{plan: plan, base: variant.LayoutVariant, order: variant.denseOrder, turns: variant.streetRotation, span: variant.denseSpan, refine: variant.refine, pairedOutputs: variant.pairedOutputs, capacityFill: variant.capacityFill, producerSplits: variant.producerSplits, splitterModel: variant.splitterModel, bentRows: variant.bentRows}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, variant)
	}
	return result, len(variants) - len(result)
}
