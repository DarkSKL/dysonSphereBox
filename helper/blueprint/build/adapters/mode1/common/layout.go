package common

// 文件职责：维护直连带模式的候选布局规划数据。
// 使用：mode1 的 standard/dense 组合共同消费；边界：不依赖 build 私有 Builder，不执行装配、寻路或最终规则。

import "math"

// SurfaceMode 职责：描述候选规划所需的球面布局语义；使用：避免候选包依赖完整生成请求；边界：只接受已归一化的模式名。
type SurfaceMode string

const (
	// SurfacePlanar 职责：标识平面布局；使用：保持旧候选顺序；边界：不触发球面优化。
	SurfacePlanar SurfaceMode = "planar"
	// SurfaceAutoTropic 职责：标识自动球面布局；使用：增加轮廓与全局供电候选；边界：不保证游戏 Physics 可放置。
	SurfaceAutoTropic SurfaceMode = "autoTropic"
)

// LayoutVariant 职责：保存直连候选的几何策略；使用：build 装配器；边界：只描述候选，不持有实体或结果。
type LayoutVariant struct {
	// Limit 职责：保存候选高度限制；使用：机器行排布；边界：必须来自公共容量候选。
	Limit float64
	// Compact 职责：启用真实轮廓压缩；使用：直连候选比较；边界：不改变产线或物流账。
	Compact bool
	// Horizontal 职责：启用水平轮廓回填；使用：自动球面候选；边界：不移动已提交的最终候选。
	Horizontal bool
	// GlobalPower 职责：延后统一补电；使用：独立候选；边界：不改变用户供电开关。
	GlobalPower bool
}

// Variants 职责：按确定顺序构建直连候选；使用：mode1 各密度组合；边界：不扩大传入高度预算，不执行候选。
func Variants(surfaceMode SurfaceMode, limits []float64) []LayoutVariant {
	variants := make([]LayoutVariant, 0, 2*len(limits))
	for _, compact := range []bool{false, true} {
		for _, limit := range limits {
			variants = append(variants, LayoutVariant{Limit: limit, Compact: compact})
		}
	}
	if surfaceMode == SurfaceAutoTropic && len(limits) > 0 && limits[len(limits)-1] > 2*limits[0] {
		variants[len(variants)-1] = LayoutVariant{Limit: 2 * limits[0]}
	}
	if surfaceMode == SurfaceAutoTropic && len(limits) > 1 {
		variants[len(limits)] = LayoutVariant{Limit: math.Ceil(limits[0] * 1.6), Compact: true, Horizontal: true}
		if len(limits) > 2 {
			variants[len(limits)+1] = LayoutVariant{Limit: math.Max(10, math.Floor(limits[0]*4/3/10)*10), Compact: true, Horizontal: true, GlobalPower: true}
		}
	}
	return variants
}
