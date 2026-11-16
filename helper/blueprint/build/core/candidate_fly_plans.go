package build

// 文件职责：按实际模块范围、最小预算和取整结果合并配送候选。
// 使用：普通/密铺配送枚举核心；边界：只测局部模块，不寻路、终检或编码，测量失败保留原策略。

import (
	"context"
	"errors"
	"math"
	"strings"
)

// errCandidateExtentReady 职责：中止独立模块测量；使用：占位寻路开始前；边界：不是生成失败或无解证明。
var errCandidateExtentReady = errors.New("候选实际模块预算测量完成")

// candidateExtentProbe 职责：保存倍率唯一影响的实际模块边界；使用：同策略局部测量；边界：不保存实体。
type candidateExtentProbe struct {
	// area, shortest 职责：保存实际总面积和最短模块边；使用：复现高度公式；边界：仅探针成功时可信。
	area, shortest float64
	// byproducts 职责：标识联产优先合流占位；使用：保留其最小尺度；边界：不改变生产流。
	byproducts bool
}

// height 职责：复现装配器中尺度唯一影响的预算公式；使用：测得实际范围后；边界：浮点运算顺序与原式一致。
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

// distributorBudgetIdentity 职责：定义候选预算等价所必需的全部策略；使用：局部测量缓存与稳定去重；边界：拓扑、排序、预留、物料顺序和触发关系不同不能合并。
type distributorBudgetIdentity struct {
	topology                                    DistributorTopology
	group, after, excluded                      string
	noSuccess, failure                          bool
	order, groups                               int
	ground, street, reserve, material, consumer bool
	clearance, latitude, budget, rowSpan        float64
	groupRows, compact, pairs, raw              bool
	rowPacking                                  bool
}

// distributorIdentity 职责：提取不可混淆的策略身份；使用：规范化之前；边界：不把枚举编号当作几何身份。
func distributorIdentity(spec DistributorCandidateSpec, req Request) distributorBudgetIdentity {
	latitude := spec.LatitudeFactor
	if latitude > 0 {
		latitude = math.Max(24, math.Ceil(math.Sqrt(float64(req.Plan.MachineCount)*32)*latitude))
	}
	return distributorBudgetIdentity{topology: spec.Topology, group: spec.GroupID,
		after: strings.Join(spec.Fallback.AfterGroups, "\x00"), excluded: spec.Fallback.ExcludedFailureCode,
		noSuccess: spec.Fallback.RequireNoSuccess, failure: spec.Fallback.RequireFailure,
		order: spec.Order, groups: spec.Groups, ground: spec.Ground, street: spec.Street,
		reserve: spec.NativeReserve, material: spec.MaterialOrder, consumer: spec.ConsumerFirst,
		clearance: spec.Clearance, latitude: latitude, rowSpan: spec.RowSpan, rowPacking: spec.RowPacking, groupRows: spec.GroupRows, compact: spec.Compact, pairs: spec.SurfacePairs}
}

// measureDistributorExtent 职责：用同一规格装配必要模块以测真实预算；使用：枚举去重；边界：黑盒街道预算另有变换，保守不合并其不同尺度。
func measureDistributorExtent(ctx context.Context, input *PreparedInput, spec DistributorCandidateSpec) (candidateExtentProbe, bool) {
	probe := candidateExtentProbe{}
	if spec.Topology == DistributorDenseBlackbox && spec.Street {
		return probe, false
	}
	spec.Scale = 1
	b, ctx, err := newDistributorAssembly(ctx, input, spec)
	if err != nil {
		return probe, false
	}
	b.candidateExtentProbe = &probe
	err = assembleDistributorSpec(ctx, b, input, spec)
	return probe, errors.Is(err, errCandidateExtentReady)
}

// normalizeDistributorCandidateSpecs 职责：稳定保留每种实际预算的首个规格；使用：枚举出口；边界：不回写尺度，失败测量不剪枝，取消必须中止。
func normalizeDistributorCandidateSpecs(ctx context.Context, input *PreparedInput, specs []DistributorCandidateSpec) ([]DistributorCandidateSpec, error) {
	type measurement struct {
		probe candidateExtentProbe
		ok    bool
	}
	measured := map[distributorBudgetIdentity]measurement{}
	seen := map[distributorBudgetIdentity]bool{}
	out := make([]DistributorCandidateSpec, 0, len(specs))
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := distributorIdentity(spec, input.request)
		if spec.Topology == DistributorInternalMode5 {
			key.budget = spec.HeightLimit
		} else {
			m, found := measured[key]
			if !found {
				m.probe, m.ok = measureDistributorExtent(ctx, input, spec)
				measured[key] = m
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key.budget, key.raw = spec.Scale, !m.ok
			if m.ok {
				key.budget = denseLatitudeHeightLimit(ctx, m.probe.height(input.request, spec.Topology != DistributorDenseDetached, spec.Scale))
			}
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, spec)
		}
	}
	recordCandidatePlans(ctx, "distributor", len(specs), len(out), 0)
	return out, nil
}
