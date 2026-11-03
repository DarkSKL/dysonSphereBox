package build

// 文件职责：把旧装配器实体转换为独立预览通知器的值快照。
// 使用：生成的实体提交、平移和候选回退边界；边界：不改变几何、连接、候选排序或正式编码，未启用时不构建场景。

import (
	"context"
	"strconv"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/preview"
)

// generationPreviewKey 职责：隔离任务预览旁路；使用：入口和最终结果发布；边界：不持有全局任务。
type generationPreviewKey struct{}

// generationPreview 职责：共享同一任务各候选的身份分配与已显示场景；使用：单任务串行生成；边界：只持有值快照，不参与算法状态恢复。
type generationPreview struct {
	// ctx 职责：保留父任务取消；使用：检查点停止发送；边界：不使用局部优化候选的截止来取消整个任务。
	ctx context.Context
	// notifier 职责：输出最小对象操作；使用：可见场景提交；边界：不输出全量蓝图。
	notifier *preview.Notifier
	// nextID 职责：为所有候选分配任务内不复用的对象 ID；使用：首次显示实体；边界：独立于最终记录下标。
	nextID uint64
	// stage 职责：标记当前对象变化所在阶段；使用：进度外壳；边界：不包含回退或失败原因。
	stage string
}

// generationPreviewSnapshot 职责：保留可获选结果的最终渲染值快照；使用：全部候选评估后提交真正的最佳场景；边界：未导出字段不进入正式结果 JSON。
type generationPreviewSnapshot struct {
	// scene 职责：保存最终记录姿态及稳定实体身份；使用：最终增量比较；边界：只内部持有，不直接全量发送。
	scene preview.Scene
}

// withGenerationPreview 职责：按显式请求和实际观察者创建旁路；使用：Generate 入口；边界：普通生成不分配实体身份、不复制场景或解码蓝图。
func withGenerationPreview(ctx context.Context, request Request) context.Context {
	if !request.EmitPreviewDeltas || !hasProgressObserver(ctx) {
		return ctx
	}
	session := &generationPreview{ctx: ctx, stage: "preview"}
	session.notifier = preview.New(func(frame preview.Frame, operations []preview.Operation) {
		publishProgress(ctx, Progress{ProtocolVersion: preview.ProtocolVersion, Kind: preview.Kind, Stage: session.stage, Message: "正在更新生成预览", CoordinateSystem: frame.CoordinateSystem, Operations: operations, PreviewAreas: frame.Areas, PreviewPrimaryAreaIndex: frame.PrimaryAreaIndex})
	})
	return context.WithValue(ctx, generationPreviewKey{}, session)
}

// previewFromContext 职责：读取单任务预览会话；使用：Builder 初始化和最终提交；边界：未启用时返回 nil。
func previewFromContext(ctx context.Context) *generationPreview {
	session, _ := ctx.Value(generationPreviewKey{}).(*generationPreview)
	return session
}

// objectID 职责：为首次可见实体分配稳定身份；使用：构建只读渲染快照；边界：实体删除后 ID 不复用，候选试装未显示时不必分配。
func (p *generationPreview) objectID(entity *placed) string {
	if entity.previewID == "" {
		p.nextID++
		entity.previewID = "building-" + strconv.FormatUint(p.nextID, 10)
	}
	return entity.previewID
}

// beginPreview 职责：标记真实生成候选可见并撤下前一个候选；使用：实际候选开始；边界：几何尺寸探测的临时 Builder 不调用此入口。
func (b *builder) beginPreview() {
	if b.preview == nil {
		return
	}
	b.previewActive = true
	b.previewCheckpoint("layout")
}

// previewCheckpoint 职责：在实体提交或平移边界发布变化；使用：真实装配和路线提交；边界：不发送 A* 节点、不调用 records 修改索引或触发正式校验。
func (b *builder) previewCheckpoint(stage string) {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	ids := make(map[*placed]string, len(b.all))
	for _, entity := range b.all {
		if entity != nil {
			ids[entity] = b.preview.objectID(entity)
		}
	}
	objects := make([]preview.Object, 0, len(b.all))
	for _, entity := range b.all {
		if entity == nil {
			continue
		}
		record := entity.record
		record.Index, record.AreaIndex = -1, 0
		record.LocalOffset = blueprint.Point{X: float32(entity.a.x), Y: float32(entity.a.y), Z: float32(entity.a.z)}
		record.LocalOffset2 = blueprint.Point{X: float32(entity.b.x), Y: float32(entity.b.y), Z: float32(entity.b.z)}
		building := renderPreviewBuilding(record, entity.owner)
		building.InputObjectID, building.OutputObjectID = ids[entity.input], ids[entity.output]
		objects = append(objects, preview.Object{ID: ids[entity], Building: building})
	}
	b.preview.stage = stage
	b.preview.notifier.Sync(preview.Scene{Objects: objects, Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
}

// rollbackPreview 职责：撤下真实失败或被放弃候选的全部临时对象；使用：候选失败边界；边界：不影响实体库存、告警或失败原因。
func (b *builder) rollbackPreview() {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	b.preview.stage = "preview"
	b.preview.notifier.Sync(preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
	b.previewActive = false
}

// renderPreviewBuilding 职责：从建筑记录摘取渲染值；使用：临时及最终姿态共用；边界：不复制参数和原始编码尾部，连接身份由调用方填写。
func renderPreviewBuilding(record blueprint.Building, owner int) preview.Building {
	return preview.Building{Index: record.Index, ItemID: record.ItemID, ModelIndex: record.ModelIndex, AreaIndex: record.AreaIndex, LocalOffset: record.LocalOffset, LocalOffset2: record.LocalOffset2, Yaw: record.Yaw, Yaw2: record.Yaw2, Tilt: record.Tilt, Tilt2: record.Tilt2, Pitch: record.Pitch, Pitch2: record.Pitch2, InputFromSlot: record.InputFromSlot, OutputToSlot: record.OutputToSlot, ProductionIndex: owner}
}

// finalPreviewSnapshot 职责：为通过严格编码的候选保留最终坐标与稳定身份；使用：候选结果择优；边界：不解码蓝图串，也不把尚未获选候选标为任务完成。
func (b *builder) finalPreviewSnapshot(records []blueprint.Building, areas []blueprint.Area, primary int32) *generationPreviewSnapshot {
	if b.preview == nil || !b.previewActive || len(records) != len(b.all) {
		return nil
	}
	ids := make(map[int32]string, len(records))
	for index, record := range records {
		ids[record.Index] = b.preview.objectID(b.all[index])
	}
	objects := make([]preview.Object, 0, len(records))
	for index, record := range records {
		building := renderPreviewBuilding(record, b.all[index].owner)
		building.InputObjectID, building.OutputObjectID = ids[record.InputObjIdx], ids[record.OutputObjIdx]
		objects = append(objects, preview.Object{ID: ids[record.Index], Building: building})
	}
	return &generationPreviewSnapshot{scene: preview.Scene{Objects: objects, Frame: preview.Frame{CoordinateSystem: preview.AreaCoordinates, Areas: append([]blueprint.Area(nil), areas...), PrimaryAreaIndex: primary}}}
}

// commitGenerationPreview 职责：把真正获选的正式候选差量提交并标记流结束；使用：Generate 成功返回前；边界：完成不取代正式调用结果，取消时不返回预览成功。
func commitGenerationPreview(ctx context.Context, result Result) error {
	session := previewFromContext(ctx)
	if session == nil {
		return nil
	}
	if result.previewScene == nil || len(result.previewScene.scene.Objects) != result.BuildingCount {
		return &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "正式候选缺少完整的增量预览快照"}
	}
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	session.stage = "preview"
	session.notifier.Sync(result.previewScene.scene)
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	publishProgress(ctx, Progress{ProtocolVersion: preview.ProtocolVersion, Kind: ProgressKindPreviewComplete, Stage: "preview", Message: "生成预览已提交正式结果", CoordinateSystem: preview.AreaCoordinates, BuildingCount: result.BuildingCount, BatchCount: session.notifier.BatchCount()})
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	return nil
}

// clearGenerationPreview 职责：为未成功任务撤下已发出的临时对象；使用：统一入口失败清理；边界：取消后事件停止，由前端任务生命周期清理视图。
func clearGenerationPreview(ctx context.Context) {
	if session := previewFromContext(ctx); session != nil && ctx.Err() == nil {
		session.stage = "preview"
		session.notifier.Sync(preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
	}
}
