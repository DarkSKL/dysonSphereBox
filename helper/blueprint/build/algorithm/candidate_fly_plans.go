package build

// 文件职责：按实际模块范围、最小预算和取整结果合并配送候选；使用：普通/超密度配送；边界：只合并同拓扑、同排序、同容量预留的有效预算，测量失败不剪枝。

import (
	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/quant"
	"context"
	"errors"
	"math"
)

// errCandidateExtentReady 职责：中止独立模块测量；使用：占位寻路开始前；边界：不是生成失败或缓存的无解证明。
var errCandidateExtentReady = errors.New("候选实际模块预算测量完成")

// candidateExtentProbe 职责：保存倍率唯一影响的实际模块边界；使用：相同局部拓扑只测一次；边界：不保存实体。
type candidateExtentProbe struct {
	area, shortest float64
	byproducts     bool
}

// flyInput 职责：保留候选原参数和显式容量预留；使用：过滤后独立于原数组下标；边界：预算键不回写原尺度。
type flyInput struct {
	scale         float64
	order         int
	nativeReserve bool
}

// measureFlyExtent 职责：装配必要局部模块以得到真实占位预算；使用：尺度等价判定；边界：黑盒街道另有预算变换，不在此归一化。
func (b *builder) measureFlyExtent(ctx context.Context, req Request, dataset *quant.Dataset, option denseFlyLayoutVariant, input flyInput) (candidateExtentProbe, bool) {
	probe := candidateExtentProbe{}
	if option.blackbox && option.street {
		return probe, false
	}
	candidate, err := b.newCandidateBuilder()
	if err != nil {
		return probe, false
	}
	candidate.preview = nil
	if option.ground {
		ctx = context.WithValue(ctx, denseGroundTracksKey{}, true)
	}
	if option.street {
		clearance := option.clearance
		if clearance == 0 {
			clearance = .04
		}
		ctx = context.WithValue(ctx, denseStreetFoldClearanceKey{}, clearance)
	}
	candidate.generationCtx, candidate.flowGraph = ctx, b.flowGraph
	candidate.denseSpatial, candidate.denseSpatialOrder, candidate.flyCompact = true, input.order, true
	candidate.nativeCapacityReserve = input.nativeReserve
	candidate.denseStreetGroupLimit = option.groups
	candidate.logistics = newLogisticsSummary(*b.flowGraph, candidate.setting)
	candidate.candidateExtentProbe = &probe
	if option.blackbox {
		err = candidate.assembleDenseFlyBlackbox(ctx, req, dataset, 1, false)
	} else {
		err = candidate.assembleDenseFlyLayout(ctx, req, dataset, 1, option.street)
	}
	return probe, errors.Is(err, errCandidateExtentReady)
}

// height 职责：复现装配器中尺度唯一影响的预算公式；使用：测得实际实体范围后；边界：浮点运算顺序与原式一致。
func (p candidateExtentProbe) height(req Request, blackbox bool, scale float64) float64 {
	if !blackbox {
		return math.Max(16, math.Ceil(math.Sqrt(p.area)*scale))
	}
	height := math.Max(24, math.Ceil(math.Sqrt(float64(req.Plan.MachineCount)*32)*scale))
	if p.byproducts {
		scale = math.Max(scale, 1.2)
	}
	return math.Max(height, math.Max(p.shortest+4, math.Ceil(math.Sqrt(p.area)*scale)))
}

// normalizeDenseFlyInputs 职责：保留每种实际预算的首个参数；使用：创建预览及派发之前；边界：排序、容量和地面拓扑不同不会合并。
func (b *builder) normalizeDenseFlyInputs(ctx context.Context, req Request, dataset *quant.Dataset, option denseFlyLayoutVariant, inputs []flyInput) []flyInput {
	type group struct {
		order   int
		reserve bool
	}
	type measurement struct {
		probe candidateExtentProbe
		ok    bool
	}
	measured := map[group]measurement{}
	type key struct {
		group
		budget float64
		raw    bool
	}
	out := candidates.StableUnique(inputs, func(input flyInput) key {
		g := group{input.order, input.nativeReserve}
		m, found := measured[g]
		if !found {
			m.probe, m.ok = b.measureFlyExtent(ctx, req, dataset, option, input)
			measured[g] = m
		}
		if !m.ok {
			return key{g, input.scale, true}
		}
		return key{g, denseLatitudeHeightLimit(ctx, m.probe.height(req, option.blackbox, input.scale)), false}
	})
	recordCandidatePlans(ctx, "dense-fly", len(inputs), len(out), 0)
	return out
}

// standardFlyOption 职责：保存普通配送的完整策略；使用：预算规范化和派发共用；边界：材料排序和消费者优先始终保留。
type standardFlyOption struct {
	scale, latitudeFactor        float64
	order                        int
	materialOrder, consumerFirst bool
}

// normalizeStandardFlyOptions 职责：合并取整后相同的纬向/总预算；使用：普通配送候选；边界：显式限高和未限高仍是不同策略。
func (b *builder) normalizeStandardFlyOptions(ctx context.Context, req Request, dataset *quant.Dataset, options []standardFlyOption) []standardFlyOption {
	type measurement struct {
		probe candidateExtentProbe
		ok    bool
	}
	measured := map[int]measurement{}
	type key struct {
		height, latitude        float64
		order                   int
		material, consumer, raw bool
	}
	out := candidates.StableUnique(options, func(option standardFlyOption) key {
		m, found := measured[option.order]
		if !found {
			m.probe, m.ok = b.measureFlyExtent(ctx, req, dataset, denseFlyLayoutVariant{blackbox: true}, flyInput{scale: 1, order: option.order})
			measured[option.order] = m
		}
		k := key{height: option.scale, latitude: option.latitudeFactor, order: option.order, material: option.materialOrder, consumer: option.consumerFirst, raw: !m.ok}
		if m.ok {
			k.height = denseLatitudeHeightLimit(ctx, m.probe.height(req, true, option.scale))
			if option.latitudeFactor > 0 {
				k.latitude = math.Max(24, math.Ceil(math.Sqrt(float64(req.Plan.MachineCount)*32)*option.latitudeFactor))
			}
		}
		return k
	})
	recordCandidatePlans(ctx, "standard-fly", len(options), len(out), 0)
	return out
}
