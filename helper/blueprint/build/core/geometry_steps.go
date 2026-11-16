package build

// 文件职责：将每项最终几何业务操作作为独立纯变换提供给工作流。
// 使用：定位、补电、裁剪、重定位、标注及冻结工具逐步调用；边界：不决定下一节点，不校验完整规则链，不编码。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
)

// PositionGeometry 职责：为已装配实体确定真实表面坐标与记录；使用：几何工作流首步；边界：不补电、不编码，局部锚点求解只改变当前显式候选。
func PositionGeometry(ctx context.Context, snapshot AssemblySnapshot) (GeometryDraft, error) {
	if snapshot.data == nil {
		return GeometryDraft{}, fmt.Errorf("缺少装配快照")
	}
	if err := ctx.Err(); err != nil {
		return GeometryDraft{}, err
	}
	work := snapshot.data.restoreAssembly(ctx, newFinalizationCloner())
	b, req := work.builder, work.request
	ctx = b.generationCtx
	b.recordProductionRateBasis(req.Plan)
	if b.policy != nil && b.policy.foldSurfaceBus {
		if err := prepareBusGeometry(ctx, b, req); err != nil {
			return GeometryDraft{}, err
		}
	}
	if len(b.all) > generationBuildingBudget(ctx) {
		return GeometryDraft{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("建筑数量超过本次预算 %d，请拆分生成", generationBuildingBudget(ctx))}
	}
	if len(b.all) > 100000 {
		b.warnings = append(b.warnings, "建筑超过 10 万，生成和游戏内粘贴可能较慢；建议按产物拆分，并关闭实时预览")
	}
	var surface *surfaceEncoding
	if req.SurfaceLayout != nil {
		var err error
		reportProgress(ctx, "surface", "正在适配行星网格与纬带", 0, 0)
		surface, err = b.prepareSurfaceEncodingContext(ctx, req.SurfaceLayout)
		if err != nil {
			return GeometryDraft{}, err
		}
	} else {
		minimumBefore, _ := bounds(b.all)
		shift(b.all, -minimumBefore.x, -minimumBefore.y, 0)
	}
	if ctx.Err() != nil {
		return GeometryDraft{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成已取消或超过当前任务预算"}
	}
	reportProgress(ctx, "records", "正在整理建筑与产线归属", 0, 0)
	records, bindings, shared, err := b.records()
	if err != nil {
		return GeometryDraft{}, err
	}
	if surface != nil {
		if err := surface.applyRecords(b.all, records); err != nil {
			return GeometryDraft{}, err
		}
		reportProgress(ctx, "collision", "正在检查球面锚点与建筑碰撞", 0, 0)
		records, err = b.validateNativeSurfaceCandidates(ctx, req, surface, records)
		if err != nil && b.policy != nil && b.policy.connectedSurfaceAnchor {
			surface, records, bindings, shared, err = prepareConnectedSurfaceAnchor(ctx, b, req, surface, records, err)
		}
		if err != nil && surface != nil && surfaceAnchorRetryable(err) && !surface.packed && len(b.surfaceBlocks) > 0 {
			reportProgress(ctx, "surface-retry", "正在重排产线并复查球面布局", 0, 0)
			// 矩形容量通过但所有锚点几何失败时，仍可按完整块重排，而不是把容量可行误当成唯一方案。
			surface, err = b.preparePackedSurfaceEncoding(ctx, req.SurfaceLayout)
			if err == nil {
				records, bindings, shared, err = b.records()
			}
			if err == nil {
				err = surface.applyRecords(b.all, records)
			}
			if err == nil {
				err = surface.validateAnchorCandidates(ctx, records)
			}
		}
		if err != nil {
			return GeometryDraft{}, err
		}
	}

	work.surface, work.records, work.bindings, work.shared = surface, records, bindings, shared
	return freezeGeometryWork(ctx, work, geometryPositioned)
}

// RepairGeometryPower 职责：按最终坐标补齐覆盖与内部电网；使用：定位之后；边界：不裁剪、不运行其它业务步骤。
func RepairGeometryPower(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryPositioned)
	if err != nil {
		return GeometryDraft{}, err
	}
	b, surface, records, bindings, shared := work.builder, work.surface, work.records, work.bindings, work.shared
	ctx = b.generationCtx
	reportProgress(ctx, "power", "正在按最终坐标补齐覆盖并连接内部电网", 0, 0)
	addedPoles, err := b.repairFinalPower(ctx, surface, records)
	if err != nil {
		return GeometryDraft{}, err
	}
	if addedPoles > 0 {
		records, bindings, shared, err = b.records()
		if err != nil {
			return GeometryDraft{}, err
		}
		if surface != nil {
			if err := surface.applyRecords(b.all, records); err != nil {
				return GeometryDraft{}, err
			}
		}
	}

	work.surface, work.records, work.bindings, work.shared, work.addedPoles = surface, records, bindings, shared, addedPoles
	return freezeGeometryWork(ctx, work, geometryPowerRepaired)
}

// PruneGeometryPower 职责：在保留真实覆盖和连通的条件下裁剪电杆；使用：补电之后；边界：不省略最终电力规则。
func PruneGeometryPower(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryPowerRepaired)
	if err != nil {
		return GeometryDraft{}, err
	}
	b, surface, records, bindings, shared := work.builder, work.surface, work.records, work.bindings, work.shared
	addedPoles := work.addedPoles
	ctx = b.generationCtx
	reportProgress(ctx, "power", "正在按最终覆盖与电网连接去除冗余感应塔", 0, 0)
	removedPoles, err := b.pruneFinalPower(ctx, surface, records)
	if err != nil {
		return GeometryDraft{}, err
	}
	if removedPoles > 0 {
		records, bindings, shared, err = b.records()
		if err != nil {
			return GeometryDraft{}, err
		}
		if surface != nil {
			if err := surface.applyRecords(b.all, records); err != nil {
				return GeometryDraft{}, err
			}
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-power-repair", Status: "passed", Message: fmt.Sprintf("按最终坐标补建 %d 座地面感应塔，消费者覆盖与内部单网随后共同终检", addedPoles)})
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-power-prune", Status: "passed", Message: fmt.Sprintf("按最终坐标去除 %d 座冗余感应塔，保留消费者覆盖和原电网连接", removedPoles)})
	}

	work.surface, work.records, work.bindings, work.shared, work.removedPoles = surface, records, bindings, shared, removedPoles
	return freezeGeometryWork(ctx, work, geometryPowerPruned)
}

// RelocateGeometry 职责：完成当前固定工厂的表面重定位；使用：电力几何固定后；边界：不切换模式、运输等级或其它工厂候选。
func RelocateGeometry(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryPowerPruned)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	work.surface, work.records, err = work.builder.relocateDenseSurface(ctx, work.request, work.surface, work.records)
	if err != nil {
		return GeometryDraft{}, err
	}
	work.zones = work.builder.finalProductionZones()
	if work.surface != nil && work.surface.relocation != nil {
		work.zones = work.builder.relocatedProductionZones(work.records)
	}
	return freezeGeometryWork(ctx, work, geometryRelocated)
}

// AlignGeometryMonitors 职责：按实际连接统一监测器方向；使用：重定位之后；边界：不将方向修正当作监测规则通过。
func AlignGeometryMonitors(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	if draft.data != nil {
		hasMonitor := false
		for _, record := range draft.data.records {
			if record.ItemID == 2030 {
				hasMonitor = true
				break
			}
		}
		if !hasMonitor {
			return advanceUnchangedGeometry(ctx, draft, geometryRelocated, geometryMonitorsAligned)
		}
	}
	work, err := restoreGeometry(ctx, draft, geometryRelocated)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	if err := work.builder.alignFlowMonitorDirections(ctx, work.surface, work.records, true); err != nil {
		return GeometryDraft{}, err
	}
	return freezeGeometryWork(ctx, work, geometryMonitorsAligned)
}

// BindGeometryCargo 职责：按实际表面重新绑定货物锚点；使用：监测方向固定后；边界：不改变生产需求或越过冻结内容写入。
func BindGeometryCargo(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryMonitorsAligned)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	if err := work.builder.rebindReferenceCargo(ctx, work.surface, work.records); err != nil {
		return GeometryDraft{}, err
	}
	return freezeGeometryWork(ctx, work, geometryCargoBound)
}

// ComputeGeometryRange 职责：计算固定内容的连续安全起点范围；使用：货物锚点固定后；边界：范围结果在文档冻结前写入，不替代最终规则。
func ComputeGeometryRange(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryCargoBound)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	if err := work.builder.finalizeDenseRelocationRange(ctx, work.request, work.surface, work.records); err != nil {
		return GeometryDraft{}, err
	}
	return freezeGeometryWork(ctx, work, geometryRangeComputed)
}

// BindGeometryTransport 职责：把真实记录编号绑定到模式运输证据；使用：表面范围固定后；边界：不编码，不替代后续独立物流守恒检查。
func BindGeometryTransport(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	if draft.data != nil && draft.data.assembly.data != nil {
		policy := draft.data.assembly.data.policy
		if policy == nil || policy.records != recordFly {
			return advanceUnchangedGeometry(ctx, draft, geometryRangeComputed, geometryTransportBound)
		}
	}
	work, err := restoreGeometry(ctx, draft, geometryRangeComputed)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	if work.builder.policy != nil && work.builder.policy.records == recordFly {
		if err := work.builder.bindFlyTransportRecords(work.builder.policy.mode); err != nil {
			return GeometryDraft{}, err
		}
	}
	return freezeGeometryWork(ctx, work, geometryTransportBound)
}

// CommitGeometryPriorities 职责：提交真实副产物优先输入关系；使用：记录绑定之后；边界：不得生成没有物料来源的优先端点。
func CommitGeometryPriorities(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	if draft.data != nil && draft.data.assembly.data != nil && draft.data.assembly.data.evidence != nil && len(draft.data.assembly.data.evidence.byproductInputs) == 0 {
		return advanceUnchangedGeometry(ctx, draft, geometryTransportBound, geometryPrioritiesCommitted)
	}
	work, err := restoreGeometry(ctx, draft, geometryTransportBound)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	work.builder.bindByproductPriorityInputs()
	return freezeGeometryWork(ctx, work, geometryPrioritiesCommitted)
}

// AnnotateGeometryRates 职责：把已经核对的生产速率写入实际带参数；使用：优先关系提交后；边界：注解属于最终文档内容，必须先于冻结。
func AnnotateGeometryRates(ctx context.Context, draft GeometryDraft) (GeometryDraft, error) {
	work, err := restoreGeometry(ctx, draft, geometryPrioritiesCommitted)
	if err != nil {
		return GeometryDraft{}, err
	}
	ctx = work.builder.generationCtx
	if err := work.builder.finalizeProductionRateAnnotations(work.records); err != nil {
		return GeometryDraft{}, err
	}
	return freezeGeometryWork(ctx, work, geometryRatesAnnotated)
}

// FreezeGeometry 职责：构造唯一待校验文档并封闭实际关系图；使用：所有变换完成后；边界：不执行完整校验，也不授予编码权限。
func FreezeGeometry(ctx context.Context, draft GeometryDraft) (GeometrySnapshot, error) {
	work, err := restoreGeometry(ctx, draft, geometryRatesAnnotated)
	if err != nil {
		return GeometrySnapshot{}, err
	}
	b, req, dataset := work.builder, work.request, work.dataset
	surface, records, bindings, shared, zones := work.surface, work.records, work.bindings, work.shared, work.zones
	ctx = b.generationCtx
	minimum, maximum := bounds(b.all)
	width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
	if width > 32767 || height > 32767 {
		return GeometrySnapshot{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "蓝图尺寸超过区域整数上限"}
	}
	var surfacePlan *SurfacePlan
	areas := []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: int16(width), Height: int16(height)}}
	primaryArea := int32(0)
	if surface != nil {
		surfacePlan, areas = surface.plan, surface.areas
		primaryArea = int32(surfacePlan.PrimaryAreaIndex)
		width, height = float64(surfacePlan.LayoutWidth), float64(surfacePlan.LayoutHeight)
	}
	name := req.Name
	if name == "" {
		name = "量化产线"
	}
	dragWidth, dragHeight := int32(width), int32(height)
	if surface != nil {
		dragWidth, dragHeight = int32(areas[primaryArea].Width), 0
		for _, area := range areas {
			dragHeight += int32(area.Height)
		}
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Patch: 1, Name: name, GameVersion: dataset.Info.GameVersion, Description: "戴森球盒子生成；需确认所有原料及副产物出口", DragBoxSize: blueprint.PointInt{X: dragWidth, Y: dragHeight}, PrimaryAreaIndex: primaryArea, CursorTargetArea: primaryArea, Areas: areas, Buildings: records}
	if req.Density == densityDense {
		document.Description += "；密铺按整图单次粘贴规则生成，已有场景接入和连续拖拽复制需另行验证"
		document.Description += densityLatitudePlacementHint(surfacePlan)
	}
	// 封面按用户目标顺序展示产物，不以中间原料、设备或产线排序代替目标。
	iconItems := make([]int, 0, len(req.Plan.Targets))
	for _, target := range req.Plan.Targets {
		iconItems = append(iconItems, target.ItemID)
	}
	document, err = blueprint.WithItemIcons(document, iconItems)
	if err != nil {
		return GeometrySnapshot{}, err
	}
	return freezeFinalGeometry(ctx, b, req, dataset, surface, document, bindings, shared, zones)
}
