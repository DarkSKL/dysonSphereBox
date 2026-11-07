package build

// 文件职责：构造只在工厂边界使用配送器的完整黑盒候选。
// 使用：普通与超密度的全场布局；边界：内部真实带路替代机队运输，外部仍保留单物料配送设施，不修改公开模式。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseFlyBlackboxState 职责：收集真实边界箱体及分流证据；使用：候选收尾；边界：不把内部物料边登记为远程配送。
type denseFlyBlackboxState struct {
	facilities []*denseFlyBoundaryFacility
	stores     []*denseFlyBoundaryStore
}

// denseFlyBoundaryStore 职责：登记一个单物料真实库存与最多两面对置接口；使用：同物料边界份额共享箱体；边界：每面独立带不超过带宽，六个箱臂槽位全局唯一，不共享不同配送物品或方向。
type denseFlyBoundaryStore struct {
	box, delivery *placed
	item          int
	input         bool
	ports         []*denseFlyBoundaryFacility
	entities      []*placed
	spatialPorts  []denseStreetPort
}

// denseFlyBoundaryFacility 职责：关联一个单物料配送器、真实箱臂和所承载的边界份额；使用：分组、占位及独立核验；边界：输入与输出用途不得互相抵消。
type denseFlyBoundaryFacility struct {
	store    *denseFlyBoundaryStore
	endpoint *flyTransportEndpoint
	routes   []*transportRoute
	entities []*placed
	// branches 职责：保存输入分流支路的全部真实前缀；使用：验证每个过滤臂均抵达所属route.start；边界：单去向不需要过滤支路。
	branches map[*transportRoute][]*placed
	// monitorHeads 职责：登记全部分流臂汇入后的真实监测前缀；使用：避免监测部分并联臂流量；边界：实体同时属于本模块库存及规范有向路径。
	monitorHeads map[*transportRoute]*placed
}

// configureDenseFlyBlackbox 职责：给当前配送候选组合内部直连能力与外部箱体证明；使用：装配前；边界：复制适配器值，不修改注册表或把结果标为mode1。
func (b *builder) configureDenseFlyBlackbox() error {
	if b.adapter == nil || b.adapter.mode != 2 {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: "配送黑盒只适用于模式2候选"}
	}
	adapter := *b.adapter
	adapter.lineAssembly = lineAssemblyCapabilities{reverseOutputRows: true, writeInputParameters: true}
	adapter.connectedRows = connectedRowsCapabilities{prepareConnectedSelfSpray: true}
	adapter.supportsMachineRecycle, adapter.flySurfacePairs = true, false
	adapter.finalizeRecords = finalizeDenseFlyBlackbox
	adapter.logistics = logisticsFinalization{validateConnectedRoutes: true, validateSelfSprays: true, validateEdgeConservation: true}
	b.adapter = &adapter
	b.directBoundaryUnits = b.prepareDenseFlyBoundaryUnits
	b.directBoundaryLaneLimit = math.Min(b.logistics.BeltCapacityPerMinute, 3*inserterCapacityPerMinute(b.setting.InserterLv, 1))
	b.denseFlyBlackbox = &denseFlyBlackboxState{}
	b.connectedCompactBelts, b.connectedGlobalPower, b.densePairedOutputs = true, true, true
	return nil
}

// assembleDenseFlyBlackbox 职责：完成内部机器行、边界设施、真实路线及供电；使用：独立密铺候选；边界：所有阶段结束后仍交给原finish执行全量终检。
func (b *builder) assembleDenseFlyBlackbox(ctx context.Context, req Request, dataset *quant.Dataset, scale float64, street bool) error {
	if err := b.configureDenseFlyBlackbox(); err != nil {
		return err
	}
	b.denseStreetCandidate = street
	b.directBoundaryExtentScale = scale
	b.denseMaxSpan = 12
	if b.denseSpatialOrder%2 == 1 {
		b.denseMaxSpan = 18
	}
	if b.adapter.density == densityStandard {
		// 普通模式保持机器/臂几何，只把默认短机器行合并为较长的完整布局单元。
		b.denseMaxSpan = 24
		if b.denseSpatialOrder%2 == 1 {
			b.denseMaxSpan = 32
		}
	}
	// 机器数只给有限搜索边长，输入短轨、过滤臂及真实箱体同样需要进入该空间；评分仍按最终实际包络收紧。
	limit := math.Max(24, math.Ceil(math.Sqrt(float64(req.Plan.MachineCount)*32)*scale))
	if err := b.prepareConnectedRows(ctx, req, dataset, limit, true); err != nil {
		return err
	}
	if err := b.routeTransport(ctx); err != nil {
		return err
	}
	if err := b.commitDenseConnectedStreets(); err != nil {
		return err
	}
	if err := b.trimConnectedTails(); err != nil {
		return err
	}
	b.surfaceBlocks, b.zoneRanges = nil, nil
	if req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar" {
		return b.completeConnectedSurfacePower(ctx, req)
	}
	if err := b.completeBoundaryFlowMonitors(ctx, req.Mode); err != nil {
		return err
	}
	return b.completeConnectedPower(ctx)
}

// denseFlyFacilityCount 职责：统计实际存活的配送器；使用：已通过全部终检的候选比较；边界：不从理论端口数估计，也不把箱体臂或内部带计为物流设施。
func denseFlyFacilityCount(entities []*placed) int {
	count := 0
	for _, entity := range entities {
		if entity.record.ItemID == 2107 {
			count++
		}
	}
	return count
}

// betterDenseFlyCandidate 职责：在完整矩形约束内先减实际配送器，再比较空间和线路；使用：密铺配送选优；边界：只比较已通过全部规则的结果，不用设施节省掩盖长条布局。
func betterDenseFlyCandidate(a LayoutMetrics, facilitiesA int, c LayoutMetrics, facilitiesC int) bool {
	aRect, cRect := a.EnvelopeAspectRatio <= 2+1e-9, c.EnvelopeAspectRatio <= 2+1e-9
	if aRect != cRect {
		return aRect
	}
	if facilitiesA != facilitiesC {
		return facilitiesA < facilitiesC
	}
	return betterDenseCandidate(a, c)
}
