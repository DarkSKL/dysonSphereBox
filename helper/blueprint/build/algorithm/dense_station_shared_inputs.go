package build

// 文件职责：为共享塔口的原料串供提供独立候选和最终份额身份核验。
// 使用：超密度塔候选及共同物流出口；边界：只聚合同物料、无喷涂原料，不替代真实串供链和配置满负荷带宽检查。

import (
	"context"
	"math"
)

// denseStationSharedInputsKey 职责：隔离共享原料塔口候选；使用：候选上下文；边界：既有独立端口候选继续参与比较。
type denseStationSharedInputsKey struct{}

// denseStationSharedInputVariantOffset 职责：保留共享供给候选与其原布局的对应关系；使用：候选登记及解码；边界：不是公开运输模式编号。
const denseStationSharedInputVariantOffset = 100

// sharesDenseStationInputs 职责：限定共享原料入口只接入具备真实黑盒边界的塔模式；使用：原料网络规划；边界：不把普通塔或配送器当作同一端口协议。
func (b *builder) sharesDenseStationInputs(ctx context.Context) bool {
	enabled, _ := ctx.Value(denseStationSharedInputsKey{}).(bool)
	return enabled && b.adapter != nil && (b.adapter.mode == 3 || b.adapter.mode == 4) && b.directBoundaryUnits != nil
}

// denseStationSharedInputSource 职责：证明规范原料份额对应到同一已提交物理供给；使用：最终塔口聚合计量；边界：此处只证明身份，真实路径、全部份额和配置毛流仍由公共物料网络终检重算。
func (b *builder) denseStationSharedInputSource(route *transportRoute, link *stationTransportLink) (*transportRoute, error) {
	plan := b.denseMaterialNetworks
	if plan == nil || route.from != nil {
		return nil, nil
	}
	binding, ok := plan.bindings[route]
	if !ok {
		return nil, nil
	}
	fail := func() (*transportRoute, error) {
		return nil, transportAllocationError(route.edge, route.rate, "共享原料份额未绑定到已提交的同一真实串供口")
	}
	source, group := binding.source, binding.group
	if !plan.committed || source == nil || source.from != nil || group == nil || group.tapped || binding.target < 0 || binding.target >= len(group.rows) || group.rows[binding.target] != route.to {
		return fail()
	}
	if route.inputRole != recipeInputRole || source.edge.FlowKind != "external-input" || source.edge.ItemID != link.item || source.start != link.head || source.boundary != link.tower || math.Abs(source.rate-link.rate) > 1e-6 {
		return fail()
	}
	for _, member := range plan.members[source] {
		if member == route {
			return source, nil
		}
	}
	return fail()
}
