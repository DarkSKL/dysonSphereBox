package build

// 文件职责：直接生成完整直连规格并按明确阶段派生空间策略。
// 使用：候选枚举传入已经验证的搜索参数和实际双侧输出资格。
// 边界：不装配实体，不用变体编号传递策略，不按阶段复制整张候选表。

import (
	"slices"

	"DysonSphereBox/helper/blueprint/build/planning/connected"
)

// planConnectedCandidates 职责：按基础、微调、街道、容量、分流、折行、双侧和移近顺序生成完整规格。
// 使用：公开枚举在真实资格探测后调用；边界：保留同分首次出现顺序，Compact 在派生移近策略后才可归一。
// 逻辑流：基础布局与配置排序/跨度配对 => 按阶段固定前缀扩展 => 完整规格交给唯一归一化去重入口。
func planConnectedCandidates(req Request, limits []float64, options ConnectedSearchOptions, paired bool) []ConnectedCandidateSpec {
	surfaceMode := connected.SurfacePlanar
	if req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		surfaceMode = connected.SurfaceAutoTropic
	}
	layouts := connected.Variants(surfaceMode, limits, options.Layout)
	budget := len(layouts)
	if req.Density == densityDense {
		budget += len(layouts) * 2
		if options.Street {
			budget += len(options.StreetGroups) * len(options.StreetRotations)
		}
		if options.CapacityFill {
			budget += len(options.DenseOrders) + len(options.StreetRotations)
		}
		if options.ProducerSplits {
			budget += (min(options.SplitterCapacityLimit, len(options.DenseOrders)+len(options.StreetRotations)) + min(options.SplitterCapacityLimit, len(options.DenseOrders))) * len(options.SplitterModels)
		}
		if options.BentRows {
			budget += len(options.BentCapacityModes) * len(options.BentSpans) * len(options.SplitterModels)
		}
		if options.PairedOutputs && paired {
			budget += len(options.StreetRotations)
		}
	}
	planned := make([]ConnectedCandidateSpec, 0, budget)
	appendCandidate := func(spec ConnectedCandidateSpec) {
		if req.Density == densityDense {
			// 全场密铺不读取水平回填；供电由实际球面模式决定，不能留到派发后才覆盖。
			spec.Horizontal = false
			spec.GlobalPower = req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar"
		}
		planned = append(planned, spec)
	}
	for position, layout := range layouts {
		spec := ConnectedCandidateSpec{Limit: layout.Limit, Compact: layout.Compact, Horizontal: layout.Horizontal, GlobalPower: layout.GlobalPower}
		if req.Density == densityDense {
			// 两个配置循环仅负责有界配对；排序值是实际几何策略，不是可解码的复合候选编号。
			spec.DenseOrder = options.DenseOrders[position%len(options.DenseOrders)]
			spec.DenseSpan = options.DenseSpans[(position/options.DenseSpanRepeat)%len(options.DenseSpans)]
		}
		appendCandidate(spec)
	}
	if req.Mode != 1 || req.Density != densityDense {
		if req.Mode == 1 && options.SharedRows && len(limits) > 0 {
			orders := options.SharedRowOrders
			if orders == nil {
				// 旧 Go 调用没有此字段；显式空数组则关闭共享行扩展。
				orders = defaultSharedRowOrders()
			}
			for _, span := range options.DenseSpans {
				for _, order := range orders {
					appendCandidate(ConnectedCandidateSpec{Limit: limits[0], Compact: true, SpatialRows: true, DenseSpan: span, DenseOrder: order, GlobalPower: req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar"})
				}
			}
		}
		return planned
	}
	for _, spec := range planned[:len(planned)] {
		if slices.Contains(options.RefineOrders, spec.DenseOrder) {
			spec.Refine = true
			appendCandidate(spec)
		}
	}
	if options.Street && denseConnectedStreetRequestEligible(req) && len(limits) > 0 {
		for _, group := range options.StreetGroups {
			for _, turn := range options.StreetRotations {
				appendCandidate(ConnectedCandidateSpec{Limit: limits[0], Compact: true, Street: true, StreetGroupLimit: group, StreetRotation: turn})
			}
		}
	}
	if options.CapacityFill {
		appendConnectedCapacityCandidates(planned, options, appendCandidate)
	}
	if options.ProducerSplits {
		copied := 0
		balanced := map[int]bool{}
		appendModels := func(spec ConnectedCandidateSpec) {
			spec.ProducerSplits = true
			for _, model := range options.SplitterModels {
				spec.SplitterModel = model
				appendCandidate(spec)
			}
		}
		for _, spec := range planned[:len(planned)] {
			if !spec.CapacityFill || spec.PairedOutputs || spec.SplitterModel != 0 {
				continue
			}
			if copied >= options.SplitterCapacityLimit {
				break
			}
			copied++
			appendModels(spec)
			if !spec.Street && !balanced[spec.DenseOrder] {
				balanced[spec.DenseOrder] = true
				spec.CapacityFill, spec.Refine = false, false
				appendModels(spec)
			}
		}
	}
	if options.BentRows && len(limits) > 0 {
		for _, fill := range options.BentCapacityModes {
			for _, span := range options.BentSpans {
				for _, model := range options.SplitterModels {
					appendCandidate(ConnectedCandidateSpec{Limit: limits[0], Compact: true, DenseSpan: span, CapacityFill: fill, ProducerSplits: true, SplitterModel: model, BentRows: true})
				}
			}
		}
	}
	if options.PairedOutputs && paired {
		for _, spec := range planned[:len(planned)] {
			if spec.Street && spec.StreetGroupLimit == 0 && !spec.CapacityFill {
				spec.PairedOutputs = true
				appendCandidate(spec)
			}
		}
	}
	if options.TranslateCompact {
		for _, spec := range planned[:len(planned)] {
			if spec.Compact && !spec.Street && !spec.Refine && !spec.CapacityFill && !spec.ProducerSplits && !spec.BentRows {
				spec.TranslateCompact = true
				appendCandidate(spec)
			}
		}
	}
	return planned
}

// appendConnectedCapacityCandidates 职责：单次扫描收集各排序最大跨度与各街道组，再按配置顺序派生容量候选。
// 使用：基础与微调阶段结束后调用；边界：同跨度优先微调，完全相同保留首个，派生结果不回灌选择集合。
func appendConnectedCapacityCandidates(planned []ConnectedCandidateSpec, options ConnectedSearchOptions, appendCandidate func(ConnectedCandidateSpec)) {
	byOrder := map[int]ConnectedCandidateSpec{}
	streets := map[[2]int]ConnectedCandidateSpec{}
	for _, spec := range planned {
		if spec.PairedOutputs {
			continue
		}
		if spec.Street {
			key := [2]int{spec.StreetRotation, spec.StreetGroupLimit}
			if _, exists := streets[key]; !exists {
				streets[key] = spec
			}
			continue
		}
		previous, exists := byOrder[spec.DenseOrder]
		if !exists || spec.DenseSpan > previous.DenseSpan || spec.DenseSpan == previous.DenseSpan && spec.Refine && !previous.Refine {
			byOrder[spec.DenseOrder] = spec
		}
	}
	orders := map[int]bool{}
	for _, order := range options.DenseOrders {
		if orders[order] {
			continue
		}
		orders[order] = true
		if spec, exists := byOrder[order]; exists {
			spec.CapacityFill = true
			appendCandidate(spec)
		}
	}
	turns := map[int]bool{}
	for _, turn := range options.StreetRotations {
		if turns[turn] {
			continue
		}
		turns[turn] = true
		for _, group := range options.CapacityStreetPreference {
			if spec, exists := streets[[2]int{turn, group}]; exists {
				spec.CapacityFill = true
				appendCandidate(spec)
				break
			}
		}
	}
}
