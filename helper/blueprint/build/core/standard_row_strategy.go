package build

// 文件职责：校验普通行跨度并向共享行规划交接显式策略。
// 使用：普通配送与塔枚举；边界：不改变机器、容量、间距或碰撞，不参与候选比较。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// standardRowSpanKey 职责：标识一次装配的显式普通行跨度；使用：配送装配局部上下文；边界：零值不覆盖既有默认，不携带请求事实。
type standardRowSpanKey struct{}

// configureStandardRows 职责：为各普通模式组合已有物料排序与同一行阵装箱；使用：单候选装配器初始化；边界：保留原排序，不重复扭曲已组成行阵的原生单元。
func (b *builder) configureStandardRows(ctx context.Context) context.Context {
	order := b.denseUnitOrder
	b.denseUnitOrder = func(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
		if order != nil {
			var err error
			units, err = order(ctx, units)
			if err != nil {
				return nil, err
			}
		}
		return b.standardSpatialRowUnits(ctx, units, 2)
	}
	return context.WithValue(ctx, denseBalancedFrameKey{}, false)
}

// validateStandardRowSpans 职责：限制普通行策略为有限正跨度；使用：枚举参数入口；边界：至多四种不同跨度，空表保留旧默认，重复值拒绝。
func validateStandardRowSpans(spans []float64) error {
	if len(spans) > 4 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "普通行跨度最多配置四种"}
	}
	seen := map[float64]bool{}
	for _, span := range spans {
		if !finitePositive(span) || span > 10000 || seen[span] {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "普通行跨度必须唯一、有限、为正且不超过10000工作格"}
		}
		seen[span] = true
	}
	return nil
}

// standardSpatialRowUnits 职责：为普通功能单元保留可布线行间距并形成有限近方形行阵；使用：普通候选显式行策略；边界：复用共享shelf装箱，不增加实体，仍交由原轮廓装箱和完整终检。
func (b *builder) standardSpatialRowUnits(ctx context.Context, units []denseSpatialUnit, gap int) ([]denseSpatialUnit, error) {
	if len(units) == 0 || gap < 2 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "普通功能行阵需要非空单元及至少两格接线间距"}
	}
	ordered := append([]denseSpatialUnit(nil), units...)
	if b.denseSpatialOrder%2 == 0 {
		sort.SliceStable(ordered, func(i, j int) bool {
			a, c := bounds(ordered[i].entities)
			d, e := bounds(ordered[j].entities)
			return c.y-a.y > e.y-d.y
		})
	}
	sizes := make([]surfaceBlockSize, len(ordered))
	area, maximum := 0., 1
	for i, unit := range ordered {
		low, high := bounds(unit.entities)
		sizes[i] = surfaceBlockSize{width: int(math.Ceil(high.x) - math.Floor(low.x) + 1), height: int(math.Ceil(high.y) - math.Floor(low.y) + 1)}
		area += float64((sizes[i].width + gap) * (sizes[i].height + gap))
		maximum = max(maximum, sizes[i].width)
	}
	height := denseLatitudeHeightLimit(ctx, math.Max(24, math.Ceil(math.Sqrt(area)*1.5)))
	start := max(maximum, int(math.Ceil(math.Sqrt(area)*.8)))
	var best *surfacePackAttempt
	bestScore := math.Inf(1)
	for n := 0; n < 10; n++ {
		width := start + n*max(2, start/10)
		rules := surfacePackRules{gap: gap, widthLimit: width, ctx: ctx}
		band := surfacePackedBand{low: 0, high: int(height) - 1, segments: (width + 4) / 5}
		attempt, err := packSurfaceBands([]surfacePackedBand{band}, []int{0}, sizes, rules, 1)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		w, h := 0, 0
		for i, block := range attempt.blocks {
			w = max(w, block.x+sizes[i].width)
			h = max(h, block.y+sizes[i].height)
		}
		score := layoutShapeCost(float64(w*h), math.Max(float64(w)/float64(h), float64(h)/float64(w)))
		if score < bestScore {
			best, bestScore = attempt, score
		}
	}
	if best == nil {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "普通行阵在当前许可高度内无法装箱"}
	}
	combined := []denseSpatialUnit{}
	byShelf := map[int]int{}
	for i, unit := range ordered {
		low, _ := bounds(unit.entities)
		block := best.blocks[i]
		shift(unit.entities, float64(block.x)-math.Floor(low.x), float64(block.y)-math.Floor(low.y), 0)
		index, ok := byShelf[block.y]
		if !ok {
			index = len(combined)
			byShelf[block.y] = index
			combined = append(combined, denseSpatialUnit{})
		}
		combined[index].entities = append(combined[index].entities, unit.entities...)
		combined[index].rows = append(combined[index].rows, unit.transportRows()...)
		combined[index].ports = append(combined[index].ports, unit.ports...)
	}
	return combined, nil
}
