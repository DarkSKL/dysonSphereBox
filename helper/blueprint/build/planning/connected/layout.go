package connected

// 文件职责：维护直连带模式的候选布局规划数据。
// 使用：mode1 的 standard/dense 组合共同消费；边界：不依赖 build 私有 Builder，不执行装配、寻路或最终规则。

import (
	"fmt"
	"math"
)

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

// SearchOptions 职责：显式描述普通布局的候选生成参数；使用：由上层默认工厂或配置提供；边界：本包不补隐藏默认值。
type SearchOptions struct {
	// CompactModes 职责：选择矩形及轮廓策略；使用：按配置顺序展开；边界：空集合不生成候选。
	CompactModes []bool `json:"compactModes" yaml:"compactModes"`
	// WideLimitFactor 职责：限制自动纬带末尾宽排候选；使用：乘首个高度；边界：不改变设备容量。
	WideLimitFactor float64 `json:"wideLimitFactor" yaml:"wideLimitFactor" min:"0" unit:"ratio"`
	// Horizontal 职责：允许水平轮廓候选；使用：自动纬带；边界：平面忽略。
	Horizontal bool `json:"horizontal" yaml:"horizontal"`
	// HorizontalFactor 职责：给出水平候选高度倍率；使用：乘首个高度；边界：不推导新默认。
	HorizontalFactor float64 `json:"horizontalFactor" yaml:"horizontalFactor" min:"0" unit:"ratio"`
	// GlobalPower 职责：允许水平统一供电候选；使用：自动纬带；边界：须同时启用 Horizontal，不改变用户供电需求。
	GlobalPower bool `json:"globalPower" yaml:"globalPower"`
	// PowerFactor 职责：给出统一供电高度倍率；使用：按量子下取整；边界：不是额定设备能力。
	PowerFactor float64 `json:"powerFactor" yaml:"powerFactor" min:"0" unit:"ratio"`
	// PowerQuantum 职责：给出供电候选高度取整粒度；使用：下取整；边界：调用方须保证正数。
	PowerQuantum float64 `json:"powerQuantum" yaml:"powerQuantum" min:"0" unit:"grid"`
	// MinimumPowerLimit 职责：给出统一供电候选最低软高度；使用：布局规划；边界：不作为安全规则。
	MinimumPowerLimit float64 `json:"minimumPowerLimit" yaml:"minimumPowerLimit" min:"0" unit:"grid"`
}

// ValidateSearchOptions 职责：纯校验直连布局倍率与工作格量子；使用：核心参数和工具参数共同调用；边界：空 CompactModes 表示禁用基础布局，不填默认值或检查真实纬带资格。
func ValidateSearchOptions(options SearchOptions) error {
	for _, entry := range []struct {
		name  string
		value float64
	}{
		{"wideLimitFactor", options.WideLimitFactor}, {"horizontalFactor", options.HorizontalFactor},
		{"powerFactor", options.PowerFactor}, {"powerQuantum", options.PowerQuantum}, {"minimumPowerLimit", options.MinimumPowerLimit},
	} {
		if entry.value <= 0 || math.IsNaN(entry.value) || math.IsInf(entry.value, 0) {
			return fmt.Errorf("layout.%s 必须为有限正数（倍率无量纲，高度和量子单位为工作格）", entry.name)
		}
	}
	if options.GlobalPower && !options.Horizontal {
		return fmt.Errorf("layout.globalPower 需要启用 horizontal")
	}
	return nil
}

// Variants 职责：按配置顺序直接构造普通几何种子；使用：核心组合完整规格；边界：不先生成再按位置回写，不补默认、不执行装配；重复种子留给完整策略配对后统一去重。
func Variants(surfaceMode SurfaceMode, limits []float64, options SearchOptions) []LayoutVariant {
	variants := make([]LayoutVariant, 0, len(options.CompactModes)*len(limits))
	for _, compact := range options.CompactModes {
		for index, limit := range limits {
			variant := LayoutVariant{Limit: limit, Compact: compact}
			if compact && surfaceMode == SurfaceAutoTropic {
				switch {
				case options.Horizontal && len(limits) > 1 && index == 0:
					variant.Limit, variant.Horizontal = math.Ceil(limits[0]*options.HorizontalFactor), true
				case options.Horizontal && options.GlobalPower && len(limits) > 2 && index == 1:
					variant.Limit = math.Max(options.MinimumPowerLimit, math.Floor(limits[0]*options.PowerFactor/options.PowerQuantum)*options.PowerQuantum)
					variant.Horizontal, variant.GlobalPower = true, true
				case index == len(limits)-1 && limit > options.WideLimitFactor*limits[0]:
					variant = LayoutVariant{Limit: options.WideLimitFactor * limits[0]}
				}
			}
			variants = append(variants, variant)
		}
	}
	return variants
}
