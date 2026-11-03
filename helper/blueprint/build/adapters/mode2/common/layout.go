package common

// 文件职责：维护配送器模式的候选布局规划数据。
// 使用：mode2 的 standard/dense 组合共同消费；边界：不依赖 build 私有 Builder，不执行配送实体装配、寻路或终检。

// SurfaceMode 职责：描述配送候选规划所需的球面语义；使用：候选工厂；边界：不携带完整请求或运行时状态。
type SurfaceMode string

const (
	// SurfacePlanar 职责：标识平面布局；使用：保持基线候选；边界：不启用球面反向组合。
	SurfacePlanar SurfaceMode = "planar"
	// SurfaceAutoTropic 职责：标识自动球面布局；使用：配送反向交错候选；边界：不替代最终球面校验。
	SurfaceAutoTropic SurfaceMode = "autoTropic"
)

// LayoutVariant 职责：保存配送候选的空间策略；使用：build 装配器；边界：不包含建筑或物流实体。
type LayoutVariant struct {
	// HeightLimit 职责：保存列高限制；使用：配送分区排布；边界：必须来自公共容量候选。
	HeightLimit float64
	// GroupRows 职责：标识是否整组排列；使用：保留基线；边界：不改变机器数量。
	GroupRows bool
	// Compact 职责：启用真实轮廓压缩；使用：配送候选比较；边界：不改变配送份额。
	Compact bool
	// SurfacePairs 职责：启用球面反向交错组合；使用：自动球面配送；边界：仍须完整终检。
	SurfacePairs bool
}

// Variants 职责：按确定顺序构建配送候选；使用：mode2 各密度组合；边界：不执行候选，不扩大传入高度预算。
func Variants(baseLimit float64, limits []float64, surfaceMode SurfaceMode, enableSurfacePairs bool) []LayoutVariant {
	variants := []LayoutVariant{{HeightLimit: baseLimit, GroupRows: true}}
	for _, compact := range []bool{false, true} {
		for _, limit := range limits {
			variants = append(variants, LayoutVariant{HeightLimit: limit, Compact: compact})
		}
	}
	if enableSurfacePairs && surfaceMode == SurfaceAutoTropic && len(limits) > 0 {
		variants = append(variants, LayoutVariant{HeightLimit: limits[0], Compact: true, SurfacePairs: true})
	}
	return variants
}
