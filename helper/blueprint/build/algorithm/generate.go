package build

// 文件职责：验证并编排布局和自检；使用：factory Generate；边界：不信任前端修改的 plan，服务需另行复核来源指纹。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// generationTimeout 职责：限制单次完整蓝图生成占用时间；使用：所有布局、接驳与预检共享同一截止时间；边界：允许复杂多目标方案更长搜索，但取消仍立即生效。
const generationTimeout = 4 * time.Minute

// coatSupply 职责：连接同组喷涂机供剂带；使用：有喷涂的布局；边界：默认暴露供给口，不伪造已存在的增产剂。
func (b *builder) coatSupply(lines []*line) error {
	created := false
	for _, l := range lines {
		if err := b.checkGeneration("喷涂供剂干线"); err != nil {
			return err
		}
		if l == nil {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂供剂缺少已装配机器行"}
		}
		if len(l.coaters) == 0 {
			continue
		}
		if l.coaterInput != nil {
			continue
		}
		if len(l.machines) == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂机器行没有任何真实机器，拒绝输出装饰喷涂网络", l.recipe.ID)}
		}
		// 原料切片保持配方逻辑顺序，底侧物理槽位已反转；供剂干线
		// 必须按喷涂机真实Y排序，不能把逻辑首尾当作递增几何区间。
		coaters := append([]*placed(nil), l.coaters...)
		for _, coater := range coaters {
			if coater == nil {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂机器行存在空喷涂端点"}
			}
		}
		sort.SliceStable(coaters, func(i, j int) bool { return coaters[i].a.y < coaters[j].a.y })
		first, last := coaters[0], coaters[len(coaters)-1]
		for _, coater := range coaters {
			if math.Abs(coater.a.x-first.a.x) > 1e-6 {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的喷涂口横坐标不一致，当前局部供剂干线无法连接", l.recipe.ID)}
			}
		}
		// 入口降到地面，再用一个水平格加一层的真实斜坡进入供剂轨；
		// 各模式可以统一连接地面端点，不要求箱体臂跨层斜插。
		startY := first.a.y - 1
		for _, row := range append(append([][]*placed{}, l.inputs...), l.outputRows()...) {
			if len(row) > 0 {
				startY = math.Min(startY, row[0].a.y-1)
			}
		}
		points := []vector{{first.a.x - 3, startY, 0}, {first.a.x - 2, startY, 0}}
		for y := startY; y <= last.a.y; y++ {
			points = append(points, vector{first.a.x - 1, y, 1})
		}
		owner := b.owner
		b.owner = l.machines[0].owner
		belts := b.byPath(points)
		b.owner = owner
		// 塔接驳从l.all读取占位；供剂入口必须同步登记到本行独立切片，
		// 避免在共享底层数组上append覆盖随后产线或让斜坡成为隐形障碍。
		l.all = append(append([]*placed(nil), l.all...), belts...)
		id := productionSprayItem(l.info)
		if id == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂设施没有合法增产剂等级", l.recipe.ID)}
		}
		belts[0].record.Parameters = []int32{int32(id), 0}
		l.coaterInput = belts[0]
		b.coaterLinks[id] = append(b.coaterLinks[id], belts[0])
		created = true
	}
	if created {
		b.warnings = append(b.warnings, "喷涂机供剂已按物流诊断中的物料来源接入；标记为外部供给的端口需持续提供增产剂，供剂不足会降低产能")
	}
	return nil
}

// Generate 职责：生成并严格自检蓝图；使用：服务对已验证方案调用；边界：超时/布局失败不返回部分串，显式预览失败也不会返回未完整标记的结果。
func Generate(parent context.Context, req Request) (result Result, generateErr error) {
	parent, performance := beginGenerationPerformance(parent)
	ctx, cancel := context.WithTimeout(parent, generationTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, generationTaskIDKey{}, req.TaskID)
	ctx = withGenerationPreview(ctx, req)
	defer func() {
		if generateErr != nil {
			clearGenerationPreview(ctx)
		}
	}()
	// 后注册的性能收尾先执行，避免主动清理把真实失败误记为取消。
	defer func() {
		snapshot := performance.finish(ctx, result, generateErr)
		if generateErr == nil && req.CollectPerformance {
			result.Performance = &snapshot
		}
	}()
	result = Result{Warnings: []string{}, Bindings: []ProductionBinding{}, SharedBuildingIndices: []int{}}
	reportProgress(ctx, "validate", "正在校验生成参数", 0, 0)
	if req.Mode < 1 || req.Mode > 6 {
		return result, &blueprint.Error{Code: "GEN_E_MODE", Message: "策略必须为 1 到 6"}
	}
	req.Density = normalizeDensity(req.Density)
	adapter, err := resolveModeAdapter(req.Mode, req.Density)
	if err != nil {
		return result, err
	}
	surface, err := normalizeSurfaceLayout(req.SurfaceLayout)
	if err != nil {
		return result, err
	}
	req.SurfaceLayout = surface
	prepared, err := prepareGeneration(ctx, req, adapter)
	if err != nil {
		return result, err
	}
	prepared.builder.beginPreview()
	generated, err := adapter.generate(ctx, prepared.builder, prepared.request, prepared.dataset)
	return publishFinalBlueprintProgress(ctx, prepared.request, prepared.dataset, generated, err)
}

// normalizeDensity 职责：规范组合适配器密度键；使用：生成入口和测试；边界：空值兼容旧请求，未知值明确失败由解析器处理。
func normalizeDensity(value string) string {
	if value == "" {
		return densityStandard
	}
	return value
}

// preparedGeneration 职责：承载通过公共准备阶段的规范请求、数据集和装配上下文；使用：模式适配器阶段；边界：只在单次生成内使用，不跨请求缓存。
type preparedGeneration struct {
	request Request
	dataset *quant.Dataset
	builder *builder
}

// prepareGeneration 职责：完成模式共用的请求准备、物料图和容量预检；使用：统一 Generate 入口；边界：不选择具体布局、不装配建筑或物流路线。
func prepareGeneration(ctx context.Context, request Request, adapter *modeAdapter) (preparedGeneration, error) {
	prepared := preparedGeneration{request: request}
	setting, err := normalizeSetting(request.Setting)
	if err != nil {
		return prepared, err
	}
	prepared.request.Setting = setting
	prepared.dataset, err = quant.LoadDataset(request.Plan.Dataset)
	if err != nil {
		return prepared, err
	}
	if prepared.request.Plan.DataVersion != prepared.dataset.Info.DataVersion || prepared.request.Plan.PlanID == "" {
		return prepared, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "数据集已变化，请重新量化"}
	}
	if len(prepared.request.Plan.Productions) == 0 {
		return prepared, &blueprint.Error{Code: "BP_E_EMPTY", Message: "当前方案没有可生成的制造产线"}
	}
	prepared.request.Plan, err = normalizeProductionRateBasis(ctx, prepared.request.Plan, prepared.dataset)
	if err != nil {
		return prepared, err
	}
	prepared.builder, err = newBuilder(setting)
	if err != nil {
		return prepared, err
	}
	prepared.builder.adapter = adapter
	prepared.builder.preview = previewFromContext(ctx)
	prepared.builder.generationCtx = ctx
	reportProgress(ctx, "flow", "正在构建守恒物料图与强连通分量", 0, 0)
	graph, err := quant.BuildFlowGraphContext(ctx, prepared.request.Plan)
	if err != nil {
		return prepared, err
	}
	prepared.builder.flowGraph = &graph
	recordPerformanceFlow(ctx, graph)
	prepared.builder.logistics = newLogisticsSummary(graph, setting)
	reportProgress(ctx, "capacity", "正在核验物料与设备端口容量", 0, 0)
	for _, production := range prepared.request.Plan.Productions {
		if err := ctx.Err(); err != nil {
			return prepared, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "设备端口能力核验已取消或超时"}
		}
		if err := validateProductionCapacity(production, prepared.dataset, setting); err != nil {
			return prepared, err
		}
		recipe, recipeErr := datasetRecipeForProduction(prepared.dataset, production)
		if recipeErr != nil {
			return prepared, recipeErr
		}
		if !adapter.supportsMachineRecycle && recipeHasMachineRecycle(recipe) {
			return prepared, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的机器内部回流当前只支持直连带与总线模式；配送/物流塔净流端点不能代替本地毛流回收", production.RecipeID)}
		}
	}
	return prepared, nil
}

// publishFinalBlueprintProgress 职责：只为最终成功蓝图发布预览事件；使用：Generate 各模式选择完成后；边界：候选布局和失败结果永不泄露为动画结果。
func publishFinalBlueprintProgress(ctx context.Context, request Request, dataset *quant.Dataset, result Result, generationErr error) (Result, error) {
	if generationErr != nil {
		return result, generationErr
	}
	if err := commitGenerationPreview(ctx, result); err != nil {
		return Result{}, err
	}
	result.previewScene = nil
	if err := emitBlueprintProgress(ctx, request, result, dataset); err != nil {
		return Result{}, err
	}
	return result, nil
}

// finish 职责：构建文档、归属和严格编码；使用：所有模式共同出口；边界：只有 Go 自检通过才返回串。
func (b *builder) finish(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	result := Result{}
	b.recordProductionRateBasis(req.Plan)
	if b.adapter != nil && b.adapter.prepareFinish != nil {
		if err := b.adapter.prepareFinish(ctx, b, req); err != nil {
			return result, err
		}
	}
	if len(b.all) > blueprint.MaxBuildingCount {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑超过 20 万，请拆分生成"}
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
			return result, err
		}
	} else {
		minimumBefore, _ := bounds(b.all)
		shift(b.all, -minimumBefore.x, -minimumBefore.y, 0)
	}
	if ctx.Err() != nil {
		return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过 4 分钟"}
	}
	reportProgress(ctx, "records", "正在整理建筑与产线归属", 0, 0)
	records, bindings, shared, err := b.records()
	if err != nil {
		return result, err
	}
	if surface != nil {
		if err := surface.applyRecords(b.all, records); err != nil {
			return result, err
		}
		reportProgress(ctx, "collision", "正在检查球面锚点与建筑碰撞", 0, 0)
		err = surface.validateAnchorCandidates(ctx, records)
		if err != nil && b.adapter != nil && b.adapter.retrySurfaceAnchor != nil {
			surface, records, bindings, shared, err = b.adapter.retrySurfaceAnchor(ctx, b, req, surface, records, err)
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
			return result, err
		}
	}
	reportProgress(ctx, "power", "正在按最终坐标补齐覆盖并连接内部电网", 0, 0)
	addedPoles, err := b.repairFinalPower(ctx, surface, records)
	if err != nil {
		return result, err
	}
	if addedPoles > 0 {
		records, bindings, shared, err = b.records()
		if err != nil {
			return result, err
		}
		if surface != nil {
			if err := surface.applyRecords(b.all, records); err != nil {
				return result, err
			}
		}
	}
	reportProgress(ctx, "power", "正在按最终覆盖与电网连接去除冗余感应塔", 0, 0)
	removedPoles, err := b.pruneFinalPower(ctx, surface, records)
	if err != nil {
		return result, err
	}
	if removedPoles > 0 {
		records, bindings, shared, err = b.records()
		if err != nil {
			return result, err
		}
		if surface != nil {
			if err := surface.applyRecords(b.all, records); err != nil {
				return result, err
			}
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-power-repair", Status: "passed", Message: fmt.Sprintf("按最终坐标补建 %d 座地面感应塔，消费者覆盖与内部单网随后共同终检", addedPoles)})
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "final-power-prune", Status: "passed", Message: fmt.Sprintf("按最终坐标去除 %d 座冗余感应塔，保留消费者覆盖和原电网连接", removedPoles)})
	}
	zones := []Zone{}
	for _, r := range b.zoneRanges {
		if r.end <= r.begin {
			continue
		}
		zone := b.zones[r.zone]
		minimum, maximum := bounds(b.all[r.begin:r.end])
		zone.MinX, zone.MinY, zone.MaxX, zone.MaxY = minimum.x, minimum.y, maximum.x, maximum.y
		zones = append(zones, zone)
	}
	minimum, maximum := bounds(b.all)
	width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
	if width > 32767 || height > 32767 {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "蓝图尺寸超过区域整数上限"}
	}
	var surfacePlan *SurfacePlan
	areas := []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: int16(width), Height: int16(height)}}
	primaryArea := int32(0)
	if surface != nil {
		surfacePlan, areas = surface.plan, surface.areas
		primaryArea = int32(surfacePlan.PrimaryAreaIndex)
		width, height = float64(surfacePlan.LayoutWidth), float64(surfacePlan.LayoutHeight)
	}
	reportProgress(ctx, "rules", "正在核验最终建筑、路线、槽位和供电规则", 0, 0)
	ruleAudit, err := validateFinalPlanetRules(ctx, req, surface, records)
	if err != nil {
		return result, err
	}
	// 各模式在最终物流汇总前补齐自己的实体绑定证据。
	if err := finalizeModeRecords(ctx, b, records); err != nil {
		return result, err
	}
	logistics, err := finalizeModeLogistics(b, records, areas)
	if err != nil {
		return result, err
	}
	if err := b.finalizeProductionRateAnnotations(records); err != nil {
		return result, err
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
	reportProgress(ctx, "encode", "正在编码并校验蓝图", 0, 0)
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		return result, err
	}
	if len(records) > 6000 {
		b.warnings = append(b.warnings, "建筑超过 6000，预览可能较慢，可折叠预览后复制导出")
	}
	return Result{Blueprint: encoded.Blueprint, BuildingCount: len(records), Areas: len(areas), Warnings: b.warnings, SourcePlanID: req.Plan.PlanID, Bindings: bindings, SharedBuildingIndices: shared, Zones: zones, SurfacePlan: surfacePlan, LogisticsSummary: logistics, RuleAudit: ruleAudit, previewScene: b.finalPreviewSnapshot(records, areas, primaryArea)}, nil
}

// surfaceAnchorRetryable 职责：判断球面锚点预检失败是否可以换到邻圈重试；使用：Generate 候选循环；边界：未知原型、超时和未验证错误不被软化。
func surfaceAnchorRetryable(err error) bool {
	failure, ok := err.(*blueprint.Error)
	if !ok {
		return false
	}
	return failure.Code == "GEN_E_COLLISION" || failure.Code == "GEN_E_SURFACE_ROUTING"
}
