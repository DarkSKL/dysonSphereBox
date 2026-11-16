package build

// 文件职责：为独立几何业务步骤保存不可变装配快照和中间产物。
// 使用：联合布局完成后交给工作流逐步变换；边界：不跨步骤保存可变builder、context或装配闭包，每次变换恢复独占临时图。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// AssemblySnapshot 职责：封闭保存一个已装配候选；使用：几何工作流的首个产物；边界：尚未执行最终球面、电力或规则步骤，不能编码。
type AssemblySnapshot struct{ data *assemblySnapshotData }

// assemblySnapshotData 职责：拥有装配关系、设置和取消无关的候选值；使用：几何步骤恢复；边界：字段私有且发布后不修改。
type assemblySnapshotData struct {
	evidence               *finalizationEvidence
	controls               assemblyControls
	policy                 *assemblyPolicy
	request                Request
	dataset                *quant.Dataset
	context                geometryContext
	models                 map[int]int16
	warnings               []string
	coaterLinks            map[int][]*placed
	zones                  []Zone
	zoneRanges             []zoneRange
	surfaceBlocks          []surfaceBlockRange
	flyLayoutBlocks        []surfaceBlockRange
	denseStreetRoutes      []*transportRoute
	surfaceBusEncoding     *surfaceEncoding
	connectedPowerEncoding *surfaceEncoding
	connectedBestObjective *LayoutMetrics
	preview                *generationPreview
	previewEntities        map[*placed]string
	bytes                  int64
}

// GeometryDraft 职责：封闭保存几何阶段的当前内容与阶段号；使用：独立反射工具交接；边界：不能跳过阶段或修改已发布快照。
type GeometryDraft struct{ data *geometryDraftData }

// geometryDraftData 职责：持有单一版本的几何内容；使用：下一步独占恢复；边界：records与实体指针关系使用同一克隆器保持内部身份。
type geometryDraftData struct {
	assembly     AssemblySnapshot
	phase        geometryPhase
	surface      *surfaceEncoding
	records      []blueprint.Building
	bindings     []ProductionBinding
	shared       []int
	zones        []Zone
	addedPoles   int
	removedPoles int
	bytes        int64
}

// geometryPhase 职责：声明业务变换的强类型前置阶段；使用：纯核心拒绝乱序；边界：流程顺序由YAML决定，核心只核对契约。
type geometryPhase uint8

const (
	geometryAssembled geometryPhase = iota
	geometryPositioned
	geometryPowerRepaired
	geometryPowerPruned
	geometryRelocated
	geometryMonitorsAligned
	geometryCargoBound
	geometryRangeComputed
	geometryTransportBound
	geometryPrioritiesCommitted
	geometryRatesAnnotated
)

// geometryWork 职责：保存单一步骤的独占计算区；使用：纯变换函数内部；边界：不会作为工作流产物返回。
type geometryWork struct {
	builder      *builder
	request      Request
	dataset      *quant.Dataset
	context      geometryContext
	surface      *surfaceEncoding
	records      []blueprint.Building
	bindings     []ProductionBinding
	shared       []int
	zones        []Zone
	addedPoles   int
	removedPoles int
}

// EstimatedBytes 职责：提供封闭装配状态的保守大小；使用：实例预算；边界：不代表进程峰值。
func (snapshot AssemblySnapshot) EstimatedBytes() int64 {
	if snapshot.data == nil {
		return 0
	}
	return snapshot.data.bytes
}

// EstimatedBytes 职责：提供几何步骤产物大小；使用：实例预算；边界：共享只读数据允许保守重复计量。
func (draft GeometryDraft) EstimatedBytes() int64 {
	if draft.data == nil {
		return 0
	}
	return draft.data.bytes
}

// freezeAssembly 职责：切断全部装配图别名并只保存值语义；使用：联合装配结束；边界：测量探针不能充当真实装配，闭包和旧ctx不会进入快照。
func freezeAssembly(ctx context.Context, b *builder, req Request, dataset *quant.Dataset) (AssemblySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return AssemblySnapshot{}, err
	}
	if b == nil || dataset == nil || b.candidateExtentProbe != nil {
		return AssemblySnapshot{}, fmt.Errorf("完整装配快照缺少真实状态或仍处于测量探针")
	}
	values, err := captureGeometryContext(ctx)
	if err != nil {
		return AssemblySnapshot{}, err
	}
	c := newFinalizationCloner()
	data := captureAssemblyData(b, req, dataset, values, c)
	// 克隆器已经逐项保守计入当前图的值和容器；事务层另计暂存/提交副本，不能在这里再次翻倍。
	data.bytes = c.retainedBytes(4096)
	return AssemblySnapshot{data}, nil
}

// captureAssemblyData 职责：使用同一克隆器保存所有几何阶段所需关系；使用：装配及中间产物冻结；边界：业务规则仍由各独立步骤执行。
func captureAssemblyData(b *builder, req Request, dataset *quant.Dataset, values geometryContext, c *finalizationCloner) *assemblySnapshotData {
	data := &assemblySnapshotData{
		evidence: captureFinalizationEvidence(b, c), controls: captureAssemblyControls(b), policy: cloneAssemblyPolicy(b.policy),
		request: c.buildRequest(req), dataset: copyFinalizationPointer(c, dataset, c.quantDataset), context: values,
		models: b.models, warnings: copyFinalizationSlice(c, b.warnings, c.copyString),
		coaterLinks: copyFinalizationMap(c, b.coaterLinks, func(v int) int { return v }, func(values []*placed) []*placed { return clonePlacedSlice(c, values) }),
		zones:       copyFinalizationSlice(c, b.zones, c.buildZone), zoneRanges: copyFinalizationSlice(c, b.zoneRanges, func(v zoneRange) zoneRange { return v }),
		surfaceBlocks: copyFinalizationSlice(c, b.surfaceBlocks, func(v surfaceBlockRange) surfaceBlockRange { return v }), flyLayoutBlocks: copyFinalizationSlice(c, b.flyLayoutBlocks, func(v surfaceBlockRange) surfaceBlockRange { return v }),
		denseStreetRoutes:  copyFinalizationSlice(c, b.denseStreetRoutes, func(v *transportRoute) *transportRoute { return copyFinalizationPointer(c, v, c.buildTransportRoute) }),
		surfaceBusEncoding: copyFinalizationPointer(c, b.surfaceBusEncoding, c.buildSurfaceEncoding), connectedPowerEncoding: copyFinalizationPointer(c, b.connectedPowerEncoding, c.buildSurfaceEncoding),
		connectedBestObjective: copyFinalizationPointer(c, b.connectedBestObjective, c.buildLayoutMetrics), preview: b.preview,
		previewEntities: copyFinalizationMap(c, b.previewEntities, func(v *placed) *placed { return copyFinalizationPointer(c, v, c.buildPlaced) }, c.copyString),
	}
	return data
}

// clonePlacedSlice 职责：保留实体图内部指针身份；使用：快照关系切片；边界：不把源指针暴露给下一步骤。
func clonePlacedSlice(c *finalizationCloner, values []*placed) []*placed {
	return copyFinalizationSlice(c, values, func(v *placed) *placed { return copyFinalizationPointer(c, v, c.buildPlaced) })
}

// cloneAssemblyPolicy 职责：复制完整纯值装配能力；使用：几何步骤交接；边界：不含函数或调度器。
func cloneAssemblyPolicy(source *assemblyPolicy) *assemblyPolicy {
	if source == nil {
		return nil
	}
	copy := *source
	if source.station != nil {
		station := *source.station
		copy.station = &station
	}
	return &copy
}

// restoreAssembly 职责：从不可变状态恢复本步骤独占图；使用：每个几何变换；边界：仅复用旁路通知身份，领域容器全部隔离。
func (data *assemblySnapshotData) restoreAssembly(ctx context.Context, c *finalizationCloner) *geometryWork {
	b := data.evidence.validationView(c)
	data.controls.apply(b)
	b.policy = cloneAssemblyPolicy(data.policy)
	b.models = data.models
	b.warnings = copyFinalizationSlice(c, data.warnings, c.copyString)
	b.coaterLinks = copyFinalizationMap(c, data.coaterLinks, func(v int) int { return v }, func(values []*placed) []*placed { return clonePlacedSlice(c, values) })
	b.zones = copyFinalizationSlice(c, data.zones, c.buildZone)
	b.zoneRanges = copyFinalizationSlice(c, data.zoneRanges, func(v zoneRange) zoneRange { return v })
	b.surfaceBlocks = copyFinalizationSlice(c, data.surfaceBlocks, func(v surfaceBlockRange) surfaceBlockRange { return v })
	b.flyLayoutBlocks = copyFinalizationSlice(c, data.flyLayoutBlocks, func(v surfaceBlockRange) surfaceBlockRange { return v })
	b.denseStreetRoutes = copyFinalizationSlice(c, data.denseStreetRoutes, func(v *transportRoute) *transportRoute { return copyFinalizationPointer(c, v, c.buildTransportRoute) })
	b.surfaceBusEncoding = copyFinalizationPointer(c, data.surfaceBusEncoding, c.buildSurfaceEncoding)
	b.connectedPowerEncoding = copyFinalizationPointer(c, data.connectedPowerEncoding, c.buildSurfaceEncoding)
	b.connectedBestObjective = copyFinalizationPointer(c, data.connectedBestObjective, c.buildLayoutMetrics)
	b.preview, b.previewEntities = data.preview, copyFinalizationMap(c, data.previewEntities, func(v *placed) *placed { return copyFinalizationPointer(c, v, c.buildPlaced) }, c.copyString)
	b.generationCtx = data.context.apply(ctx)
	return &geometryWork{builder: b, request: c.buildRequest(data.request), dataset: copyFinalizationPointer(c, data.dataset, c.quantDataset), context: data.context}
}

// restoreGeometry 职责：核对阶段并创建独占临时图；使用：各明确业务步骤；边界：不能对原产物做原地修改。
func restoreGeometry(ctx context.Context, draft GeometryDraft, expected geometryPhase) (*geometryWork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if draft.data == nil || draft.data.assembly.data == nil || draft.data.phase != expected {
		return nil, fmt.Errorf("几何步骤缺少正确前置阶段：需要%d", expected)
	}
	c := newFinalizationCloner()
	work := draft.data.assembly.data.restoreAssembly(ctx, c)
	work.surface = copyFinalizationPointer(c, draft.data.surface, c.buildSurfaceEncoding)
	work.records = copyFinalizationSlice(c, draft.data.records, c.blueprintBuilding)
	work.bindings = copyFinalizationSlice(c, draft.data.bindings, c.buildProductionBinding)
	work.shared = copyFinalizationSlice(c, draft.data.shared, func(v int) int { return v })
	work.zones = copyFinalizationSlice(c, draft.data.zones, c.buildZone)
	work.addedPoles, work.removedPoles = draft.data.addedPoles, draft.data.removedPoles
	return work, nil
}

// freezeGeometryWork 职责：原子形成下一阶段的不可变产物；使用：业务步骤成功出口；边界：失败或取消时不发布暂存图。
func freezeGeometryWork(ctx context.Context, work *geometryWork, phase geometryPhase) (GeometryDraft, error) {
	if err := ctx.Err(); err != nil {
		return GeometryDraft{}, err
	}
	c := newFinalizationCloner()
	assembly := captureAssemblyData(work.builder, work.request, work.dataset, work.context, c)
	data := &geometryDraftData{assembly: AssemblySnapshot{assembly}, phase: phase, surface: copyFinalizationPointer(c, work.surface, c.buildSurfaceEncoding), records: copyFinalizationSlice(c, work.records, c.blueprintBuilding), bindings: copyFinalizationSlice(c, work.bindings, c.buildProductionBinding), shared: copyFinalizationSlice(c, work.shared, func(v int) int { return v }), zones: copyFinalizationSlice(c, work.zones, c.buildZone), addedPoles: work.addedPoles, removedPoles: work.removedPoles}
	data.bytes = c.retainedBytes(4096)
	assembly.bytes = data.bytes
	return GeometryDraft{data}, nil
}
