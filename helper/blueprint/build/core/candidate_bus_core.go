package build

// 文件职责：提供内部总线模式的显式规格和一次求解。
// 使用：内部mode6方案族的工具入口；边界：不执行备用、并发、选择或编码，保留所有真实环路与物料规则。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// BusCandidateSpec 职责：表示是否使用有限拓扑收紧；使用：明确单次总线候选；边界：不包含第二次自动执行。
type BusCandidateSpec struct {
	Tighten bool `json:"tighten"`
}

// EnumerateBusCandidates 职责：列出基线与被允许的平面收紧规格；使用：引擎调度；边界：球面不枚举平面收紧。
func EnumerateBusCandidates(ctx context.Context, input *PreparedInput, allowTighten bool) ([]BusCandidateSpec, error) {
	if input == nil || input.request.Mode != 6 {
		return nil, fmt.Errorf("总线规格仅支持内部模式6")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := []BusCandidateSpec{{}}
	if allowTighten && input.request.SurfaceLayout == nil {
		result = append(result, BusCandidateSpec{Tighten: true})
	}
	return result, nil
}

// AssembleBusCandidate 职责：只装配一个显式规格并冻结装配图；使用：工作流装配节点；边界：后续定位、补电、绑定、规则和编码由独立节点执行。
func AssembleBusCandidate(ctx context.Context, input *PreparedInput, spec BusCandidateSpec) (AssemblySnapshot, error) {
	if input == nil || input.request.Mode != 6 {
		return AssemblySnapshot{}, fmt.Errorf("总线求解仅支持内部模式6")
	}
	ctx = input.algorithmContext(ctx)
	b, err := input.newAssembly(ctx, previewFromContext(ctx).fork())
	if err != nil {
		return AssemblySnapshot{}, err
	}
	prepared, err := b.assembleBusVariant(ctx, input.request, input.dataset, spec.Tighten)
	if err == nil && !prepared {
		err = &blueprint.Error{Code: "GEN_I_DENSITY_DOMINATED", Message: "当前总线规格没有可执行的拓扑收紧"}
	}
	if err != nil {
		b.rollbackPreview()
		return AssemblySnapshot{}, err
	}
	geometry, err := freezeAssembly(ctx, b, input.request, input.dataset)
	if err != nil {
		b.rollbackPreview()
	}
	return geometry, err
}

// assembleBusVariant 职责：装配一个明确的共享环路并核对规范边；使用：单候选核心；边界：不进入最终冻结校验或编码。
func (b *builder) assembleBusVariant(ctx context.Context, req Request, dataset *quant.Dataset, tighten bool) (bool, error) {
	b.beginPreview()
	prepared, err := b.prepareBusFabricVariant(ctx, req, dataset, tighten)
	if err != nil || !prepared {
		return prepared, err
	}
	if err := b.routeTransport(ctx); err != nil {
		return true, err
	}
	for _, entry := range b.busFabric {
		if entry.source != nil && entry.route.from == nil {
			entry.route.boundary = entry.source.boundary
		}
	}
	checks := b.logistics.Checks[:0]
	for _, check := range b.logistics.Checks {
		if check.RuleID != "mode1-internal-connectivity" {
			checks = append(checks, check)
		}
	}
	b.logistics.Checks = checks
	b.transportRoutes = nil
	return true, b.validateBusFabric()
}
