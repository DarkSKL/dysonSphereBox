package distributor

// 文件职责：维护配送器模式的候选布局规划数据。
// 使用：mode2 的 standard/dense 组合共同消费；边界：不依赖 build 私有 Builder，不执行配送实体装配、寻路或终检。

import (
	"fmt"
	"math"
)

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
	HeightLimit float64 `json:"heightLimit" min:"0" unit:"grid"`
	// GroupRows 职责：标识是否整组排列；使用：保留基线；边界：不改变机器数量。
	GroupRows bool `json:"groupRows"`
	// Compact 职责：启用真实轮廓压缩；使用：配送候选比较；边界：不改变配送份额。
	Compact bool `json:"compact"`
	// SurfacePairs 职责：启用球面反向交错组合；使用：自动球面配送；边界：仍须完整终检。
	SurfacePairs bool `json:"surfacePairs"`
}

// FieldStrategy 职责：描述全场配送的尺度、排序及纬向策略；使用：普通和密铺枚举核心；边界：仅保存参数，不创建实体或挑选获胜者。
type FieldStrategy struct {
	// Scale 职责：控制真实模块面积推导的高度倍率；使用：全场装箱；边界：必须为有限正数。
	Scale float64 `json:"scale" min:"0" unit:"ratio"`
	// Order 职责：选择稳定模块排序；使用：装配器；边界：合法值为 0..3。
	Order int `json:"order" min:"0" max:"3"`
	// LatitudeFactor 职责：控制普通配送纬向预算；使用：显式限高；边界：零表示不单独限高。
	LatitudeFactor float64 `json:"latitudeFactor" min:"0" unit:"ratio"`
	// MaterialOrder 职责：启用物料邻接排序；使用：普通配送；边界：不改变规范流量。
	MaterialOrder bool `json:"materialOrder"`
	// ConsumerFirst 职责：选择消费者优先物料顺序；使用：MaterialOrder 为真时；边界：不改变生产设备数量。
	ConsumerFirst bool `json:"consumerFirst"`
	// NativeReserve 职责：启用原生纬带容量预留；使用：密铺原生候选；边界：不同预留策略不得合并。
	NativeReserve bool `json:"nativeReserve"`
}

// ValidateFieldStrategy 职责：校验全场配送的无量纲倍率和排序关系；使用：工具静态参数及核心规格共同调用；边界：不决定普通/密铺资格，不创建装配器或修改参数。
func ValidateFieldStrategy(strategy FieldStrategy) error {
	if strategy.Scale <= 0 || math.IsNaN(strategy.Scale) || math.IsInf(strategy.Scale, 0) {
		return fmt.Errorf("scale 必须为有限正倍率")
	}
	if strategy.LatitudeFactor < 0 || math.IsNaN(strategy.LatitudeFactor) || math.IsInf(strategy.LatitudeFactor, 0) {
		return fmt.Errorf("latitudeFactor 必须为有限非负倍率，零表示不限高")
	}
	if strategy.Order < 0 || strategy.Order > 3 {
		return fmt.Errorf("order 必须为 0..3")
	}
	if strategy.ConsumerFirst && !strategy.MaterialOrder {
		return fmt.Errorf("consumerFirst 需要启用 materialOrder")
	}
	return nil
}

// ValidateLayoutVariant 职责：校验内部mode5列高；使用：显式高度表及核心规格共同调用；边界：单位为工作格，球面交错能力必须另由真实输入核对。
func ValidateLayoutVariant(variant LayoutVariant) error {
	if variant.HeightLimit <= 0 || math.IsNaN(variant.HeightLimit) || math.IsInf(variant.HeightLimit, 0) {
		return fmt.Errorf("heightLimit 必须为有限正工作格数")
	}
	return nil
}

// StandardFieldStrategies 职责：集中提供普通配送默认表；使用：默认搜索选项；边界：每次返回独立切片，显式覆盖由调用方负责。
func StandardFieldStrategies() []FieldStrategy {
	return []FieldStrategy{
		{Scale: 1, Order: 2},
		{Scale: 1.2, Order: 3},
		{Scale: 1, LatitudeFactor: .7, Order: 3},
		{Scale: 1, LatitudeFactor: .85, Order: 3},
		{Scale: 1.2, LatitudeFactor: .75, Order: 2},
		{Scale: 1, LatitudeFactor: .85, Order: 3, MaterialOrder: true},
		{Scale: 1.2, LatitudeFactor: .7, Order: 3, MaterialOrder: true, ConsumerFirst: true},
		{Scale: 1.2, Order: 3, MaterialOrder: true},
	}
}

// DenseFieldStrategies 职责：集中提供密铺默认表和原生容量替代；使用：默认搜索选项；边界：原生策略只替换最后两个排序，不复制整张默认表。
func DenseFieldStrategies(native bool) []FieldStrategy {
	strategies := []FieldStrategy{{Scale: .85, Order: 2}, {Scale: .85, Order: 3}, {Scale: 1, Order: 2}, {Scale: 1, Order: 3}, {Scale: 1.2, Order: 2}, {Scale: 1.2, Order: 3}}
	if native {
		strategies[4].Order, strategies[5].Order = 0, 1
		strategies[4].NativeReserve, strategies[5].NativeReserve = true, true
	}
	return strategies
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
