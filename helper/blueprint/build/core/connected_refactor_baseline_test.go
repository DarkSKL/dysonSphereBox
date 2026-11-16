package build

// 文件职责：冻结重构前直连候选枚举、扩展和街道去重，供临时差分验收。
// 使用：验证真实策略覆盖和同分顺序；边界：仅测试保留旧结构，交由主线完成总验收后清理。
import (
	mode1common "DysonSphereBox/helper/blueprint/build/planning/connected"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"strings"
)

// referenceConnectedVariant 职责：保存既有几何辅助函数的内部候选值；使用：单候选核心与过渡桥转换；边界：新调用方应使用 ConnectedCandidateSpec，不依赖旧调度。
type referenceConnectedVariant struct {
	// LayoutVariant 职责：保留普通候选参数；使用：共同编排；边界：不改变普通模式策略。
	mode1common.LayoutVariant
	// denseOrder 职责：选择密铺排序与距离代价；使用：独立候选；边界：不改变规范物料图。
	denseOrder int
	// denseSpan 职责：约束本候选机器行跨度；使用：容量组内细分；边界：不增加单带容量。
	denseSpan float64
	// refine 职责：启用小数位姿搜索；使用：额外可行候选；边界：不取消整数候选或最终校验。
	refine bool
	// street/streetRotation 职责：选择连续容量段街道并比较整场朝向；使用：独立完整候选；边界：不代替旧全场密铺候选。
	street           bool
	streetRotation   int
	streetGroupLimit int
	// pairedOutputs 职责：标识真实双侧过滤布局；使用：有限多去向候选；边界：旧单侧可行候选继续独立保留。
	pairedOutputs bool
	// capacityFill 职责：选择单带能力优先填满的独立密铺分组；使用：有界布局比较；边界：原均衡候选和请求设置保持不变。
	capacityFill bool
	// producerSplits/splitterModel 职责：选择独立产物分流树及其真实形态；使用：有限几何候选；边界：原过滤候选保留，模型改变必须重新装配并通过完整校验。
	producerSplits bool
	splitterModel  int16
	// bentRows 职责：选择真实短run的L/U折行；使用：独立密铺候选；边界：直线方案保留，不按外包矩形独占折角空间。
	bentRows bool
	// translateCompact 职责：增加独立移近候选；使用：行间空档优化；边界：保留未经平移的可行方案。
	translateCompact bool
}

// referenceConnectedBase 职责：展开普通及密铺基础空间策略；使用：显式枚举与共享规格构造；边界：所有默认仅由默认工厂传入。
func referenceConnectedBase(req Request, limits []float64, options ConnectedSearchOptions) []referenceConnectedVariant {
	surfaceMode := mode1common.SurfacePlanar
	if req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		surfaceMode = mode1common.SurfaceAutoTropic
	}
	planned := referenceConnectedLayouts(surfaceMode, limits, options.Layout)
	variants := make([]referenceConnectedVariant, 0, len(planned))
	for index, variant := range planned {
		candidate := referenceConnectedVariant{LayoutVariant: variant}
		if req.Density == densityDense {
			candidate.denseOrder = options.DenseOrders[index%len(options.DenseOrders)]
			candidate.denseSpan = options.DenseSpans[(index/options.DenseSpanRepeat)%len(options.DenseSpans)]
		}
		variants = append(variants, candidate)
	}
	if req.Density != densityDense {
		return variants
	}
	for _, variant := range append([]referenceConnectedVariant{}, variants...) {
		for _, order := range options.RefineOrders {
			if variant.denseOrder == order {
				variant.refine = true
				variants = append(variants, variant)
				break
			}
		}
	}
	if options.Street && denseConnectedStreetRequestEligible(req) && len(limits) > 0 {
		for _, group := range options.StreetGroups {
			for _, turn := range options.StreetRotations {
				variants = append(variants, referenceConnectedVariant{LayoutVariant: mode1common.LayoutVariant{Limit: limits[0], Compact: true, GlobalPower: true}, street: true, streetGroupLimit: group, streetRotation: turn})
			}
		}
	}
	return variants
}

// referenceConnectedExpand 职责：按明确参数派生容量、分流及折行；使用：真实分组去重前；边界：不递归扩展新候选，不隐藏完整求解。
func referenceConnectedExpand(req Request, limits []float64, variants []referenceConnectedVariant, options ConnectedSearchOptions) []referenceConnectedVariant {
	if req.Mode != 1 || req.Density != densityDense {
		return variants
	}
	if options.CapacityFill {
		original := append([]referenceConnectedVariant{}, variants...)
		for _, order := range options.DenseOrders {
			chosen := -1
			for i, variant := range original {
				if variant.street || variant.pairedOutputs || variant.denseOrder != order {
					continue
				}
				if chosen < 0 || variant.denseSpan > original[chosen].denseSpan || variant.denseSpan == original[chosen].denseSpan && variant.refine && !original[chosen].refine {
					chosen = i
				}
			}
			if chosen >= 0 {
				variant := original[chosen]
				variant.capacityFill = true
				variants = append(variants, variant)
			}
		}
		for _, turn := range options.StreetRotations {
			found := false
			for _, group := range options.CapacityStreetPreference {
				for _, variant := range original {
					if variant.street && !variant.pairedOutputs && variant.streetGroupLimit == group && variant.streetRotation == turn {
						variant.capacityFill = true
						variants = append(variants, variant)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}
	if options.ProducerSplits {
		original := append([]referenceConnectedVariant{}, variants...)
		copied := 0
		balanced := map[int]bool{}
		appendModels := func(variant referenceConnectedVariant) {
			for _, model := range options.SplitterModels {
				variant.producerSplits = true
				variant.splitterModel = model
				variants = append(variants, variant)
			}
		}
		for _, variant := range original {
			if !variant.capacityFill || variant.pairedOutputs || variant.splitterModel != 0 {
				continue
			}
			if copied >= options.SplitterCapacityLimit {
				break
			}
			copied++
			appendModels(variant)
			if !variant.street && !balanced[variant.denseOrder] {
				balanced[variant.denseOrder] = true
				variant.capacityFill = false
				variant.refine = false
				appendModels(variant)
			}
		}
	}
	if options.BentRows && len(limits) > 0 {
		for _, fill := range options.BentCapacityModes {
			for _, span := range options.BentSpans {
				for _, model := range options.SplitterModels {
					variants = append(variants, referenceConnectedVariant{LayoutVariant: mode1common.LayoutVariant{Limit: limits[0], Compact: true, GlobalPower: true}, denseSpan: span, capacityFill: fill, producerSplits: true, splitterModel: model, bentRows: true})
				}
			}
		}
	}
	return variants
}

// referenceConnectedStreetIdentity 职责：保存除已归一街道分组以外全部影响布局的参数；使用：精确去重；边界：不同朝向、跨度、微调和供电策略不能合并。
type referenceConnectedStreetIdentity struct {
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

// referenceConnectedStreetUnique 职责：保持首个等价计划及原顺序；使用：公开候选入口；边界：未知分组键原样保留，空的已知键只表示没有适配产线。
func referenceConnectedStreetUnique(variants []referenceConnectedVariant, keys map[int]string, capacityKeys ...map[int]string) ([]referenceConnectedVariant, int) {
	result := make([]referenceConnectedVariant, 0, len(variants))
	seen := map[referenceConnectedStreetIdentity]bool{}
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
		key := referenceConnectedStreetIdentity{plan: plan, base: variant.LayoutVariant, order: variant.denseOrder, turns: variant.streetRotation, span: variant.denseSpan, refine: variant.refine, pairedOutputs: variant.pairedOutputs, capacityFill: variant.capacityFill, producerSplits: variant.producerSplits, splitterModel: variant.splitterModel, bentRows: variant.bentRows}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, variant)
	}
	return result, len(variants) - len(result)
}

// referenceConnectedSpec 职责：把兼容布局值转成具名规格；使用：旧入口过渡桥；边界：不分配装配器。
func referenceConnectedSpec(req Request, variant referenceConnectedVariant, legacy bool) ConnectedCandidateSpec {
	spec := ConnectedCandidateSpec{Limit: variant.Limit, Compact: variant.Compact, Horizontal: variant.Horizontal, GlobalPower: variant.GlobalPower, LegacyInputs: legacy,
		DenseOrder: variant.denseOrder, DenseSpan: variant.denseSpan, Refine: variant.refine, Street: variant.street, StreetRotation: variant.streetRotation, StreetGroupLimit: variant.streetGroupLimit,
		PairedOutputs: variant.pairedOutputs, CapacityFill: variant.capacityFill, ProducerSplits: variant.producerSplits, SplitterModel: variant.splitterModel, BentRows: variant.bentRows, TranslateCompact: variant.translateCompact,
	}
	if req.Density == densityDense {
		spec.GlobalPower = req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar"
	}
	return spec
}

// Variants 职责：按确定顺序构建直连候选；使用：核心传入完整参数；边界：不补默认、不执行装配。
func referenceConnectedLayouts(surfaceMode mode1common.SurfaceMode, limits []float64, options mode1common.SearchOptions) []mode1common.LayoutVariant {
	variants := make([]mode1common.LayoutVariant, 0, len(options.CompactModes)*len(limits))
	for _, compact := range options.CompactModes {
		start := len(variants)
		for _, limit := range limits {
			variants = append(variants, mode1common.LayoutVariant{Limit: limit, Compact: compact})
		}
		if surfaceMode != mode1common.SurfaceAutoTropic || len(limits) == 0 {
			continue
		}
		if compact && limits[len(limits)-1] > options.WideLimitFactor*limits[0] {
			variants[len(variants)-1] = mode1common.LayoutVariant{Limit: options.WideLimitFactor * limits[0]}
		}
		if compact && options.Horizontal && len(limits) > 1 {
			variants[start] = mode1common.LayoutVariant{Limit: math.Ceil(limits[0] * options.HorizontalFactor), Compact: true, Horizontal: true}
			if options.GlobalPower && len(limits) > 2 {
				variants[start+1] = mode1common.LayoutVariant{Limit: math.Max(options.MinimumPowerLimit, math.Floor(limits[0]*options.PowerFactor/options.PowerQuantum)*options.PowerQuantum), Compact: true, Horizontal: true, GlobalPower: true}
			}
		}
	}
	return variants
}

// referenceConnectedPlanKeys 职责：从同一资格判断和真实容量counts生成每街道组数的规范键；使用：去除完全相同的显式街道计划；边界：auto只在单组时折叠，键不证明几何成功，空键表示没有适配owner。
func (b *builder) referenceConnectedPlanKeys(ctx context.Context, req Request, dataset *quant.Dataset, capacityFill ...bool) (map[int]string, error) {
	probe, err := b.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	probe.denseSpatial, probe.denseMaxSpan = true, 0
	probe.denseCapacityFill = len(capacityFill) > 0 && capacityFill[0]
	probe.flowGraph = b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	rows, err := probe.planTransportRows(ctx, req, dataset)
	if err != nil {
		return nil, err
	}
	order := []int{}
	byOwner := map[int][]*transportRow{}
	for _, row := range rows {
		if len(byOwner[row.owner]) == 0 {
			order = append(order, row.owner)
		}
		byOwner[row.owner] = append(byOwner[row.owner], row)
	}
	keys := map[int]string{}
	for _, limit := range []int{0, 2, -1} {
		var key strings.Builder
		for _, owner := range order {
			group := byOwner[owner]
			if !denseConnectedStreetRowsEligible(group, b.setting) {
				continue
			}
			counts := make([]int, len(group))
			for i, row := range group {
				counts[i] = row.count
			}
			effective := "auto"
			if limit != 0 || len(group) == 1 {
				size := min(len(group), limit)
				if limit <= 0 {
					size = len(group)
				}
				effective = fmt.Sprintf("%d", size)
			}
			fmt.Fprintf(&key, "owner%d:counts%v:perStreet%s;", owner, counts, effective)
		}
		keys[limit] = key.String()
	}
	return keys, nil
}

// referenceConnectedPairedGroups 职责：先核对真实容量行是否存在可压缩过滤组；使用：仅在几何确会变化时增加四向候选；边界：不凭总目标数猜测分支数。
func (b *builder) referenceConnectedPairedGroups(ctx context.Context, req Request, dataset *quant.Dataset) (bool, error) {
	probe, err := b.newCandidateBuilder()
	if err != nil {
		return false, err
	}
	probe.denseSpatial, probe.denseMaxSpan = true, 0
	probe.flowGraph = b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	rows, err := probe.planTransportRows(ctx, req, dataset)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if densePairedOutputEligible(row) {
			return true, nil
		}
	}
	return false, nil
}
