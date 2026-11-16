package build

// 文件职责：把单次装配实体转换为引擎固定作用域内的预览值快照。
// 使用：生成的实体提交、平移和候选回退边界；边界：不改变几何、连接、候选排序或正式编码，未启用时不构建场景。

import (
	"context"
	"strconv"
	"sync"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/preview"
)

// generationPreviewKey 职责：隔离任务预览旁路；使用：入口和最终结果发布；边界：不持有全局任务。
type generationPreviewKey struct{}

// generationPreview 职责：维护固定路径候选的通知顺序和已显示场景；使用：v4并发子会话或v3单槽画面；边界：只持有值快照，不分配算法候选或恢复实体。
type generationPreview struct {
	// mu 职责：串行候选事件和通知器；使用：生命周期与增量同序；边界：v3借用根会话的锁。
	mu sync.Mutex
	// root 职责：关联任务通知根；使用：v3共享画面和任务归属；边界：不提供领域状态。
	root *generationPreview
	// protocol 职责：固定当前公开协议；使用：兼容v3/v4；边界：开始后不切换。
	protocol int
	// candidateID 职责：保存完整路径摘要身份；使用：平台和正式结果匹配；边界：不依赖完成次序。
	candidateID string
	// ordinal 职责：保存路径确定的平台顺序；使用：稳定排布；边界：嵌套作用域允许稀疏。
	ordinal uint64
	// path 职责：保存固定引擎作用域；使用：生命周期归属；边界：不拥有调度权。
	path string
	// bound 职责：标记已由外部引擎绑定完整Solve；使用：阻止外层组产生平台；边界：不代表物理验证通过。
	bound bool
	// finished 职责：阻止已终结会话继续发实体变化；使用：增量守卫；边界：正式winner快照另行发布。
	finished bool
	// started 职责：保证计划与开始事务只提交一次；使用：装配开始；边界：开始不等于可行。
	started bool
	// sequence 职责：维护本候选连续事件序号；使用：前端校验；边界：提交后不能因取消丢号。
	sequence uint64
	// revision 职责：维护本候选完整快照版本；使用：best及最终提交共用；边界：每次发布递增，最终结果不得复用best版本。
	revision uint64
	// observedValid 职责：幂等消费完整验证成功；使用：祖先重复succeeded守卫；边界：不阻止父selected纠正当前最佳。
	observedValid bool
	// status 职责：保存valid/rejected/pruned旁路状态；使用：终结幂等；边界：不决定正式winner。
	status string
	// terminal 职责：标记v3共享通知器正在撤销；使用：取消后发出最后删除操作；边界：只在根锁内读写。
	terminal bool
	// active 职责：记录v3共享画面当前会话；使用：只撤销真正显示的失败候选；边界：仅旁路身份，不决定winner。
	active *generationPreview

	// ctx 职责：保留父任务取消；使用：检查点停止发送；边界：不使用局部优化候选的截止来取消整个任务。
	ctx context.Context
	// notifier 职责：输出最小对象操作；使用：可见场景提交；边界：不输出全量蓝图。
	notifier *preview.Notifier
	// nextID 职责：在候选作用域分配对象ID，v3改用根序列；使用：首次显示实体；边界：完整身份包含候选ID，独立于最终记录下标。
	nextID uint64
	// stage 职责：标记当前对象变化所在阶段；使用：进度外壳；边界：不包含回退或失败原因。
	stage string
}

// generationPreviewSnapshot 职责：保留可获选结果的最终渲染值快照；使用：全部候选评估后提交真正的最佳场景；边界：未导出字段不进入正式结果 JSON。
type generationPreviewSnapshot struct {
	// session 职责：保留内部候选通知身份；使用：验证与正式获选发布；边界：不向外暴露或写入领域数据。
	session *generationPreview
	// scene 职责：保存最终记录姿态及稳定实体身份；使用：最终增量比较；边界：只内部持有，不直接全量发送。
	scene preview.Scene
}

// withGenerationPreview 职责：按显式请求和实际观察者创建旁路；使用：Generate 入口；边界：普通生成不分配实体身份、不复制场景或解码蓝图。
func withGenerationPreview(ctx context.Context, request Request) context.Context {
	if !request.EmitPreviewDeltas || !hasProgressObserver(ctx) {
		return ctx
	}
	session := &generationPreview{ctx: ctx, stage: "preview", protocol: 3}
	session.root = session
	if request.PreviewProtocolVersion == 4 {
		session.protocol = 4
		session.root = session
		publishProgress(ctx, Progress{ProtocolVersion: 4, Kind: "search-start", Stage: "preview", Message: "开始候选方案搜索"})
	}
	session.notifier = preview.New(func(frame preview.Frame, operations []preview.Operation) {
		emitContext := ctx
		if session.terminal {
			emitContext = context.WithoutCancel(ctx)
		}
		publishProgress(emitContext, Progress{ProtocolVersion: preview.ProtocolVersion, Kind: preview.Kind, Stage: session.stage, Message: "正在更新生成预览", CoordinateSystem: frame.CoordinateSystem, Operations: operations, PreviewAreas: frame.Areas, PreviewPrimaryAreaIndex: frame.PrimaryAreaIndex})
	})
	return context.WithValue(ctx, generationPreviewKey{}, session)
}

// fork 职责：复用引擎绑定的完整候选会话；使用：单次Solve内部创建装配器；边界：不分配新算法候选或依完成时间猜身份。
func (p *generationPreview) fork() *generationPreview {
	if p == nil || p.bound || p.protocol != 4 {
		return p
	}

	return nil
}

// newObservationPreview 职责：按已确定身份构造旁路子会话；使用：引擎固定路径绑定；边界：不启动平台，不读取实体。
func newObservationPreview(root *generationPreview, id string, ordinal uint64, path string) *generationPreview {
	p := &generationPreview{root: root, ctx: root.ctx, protocol: root.protocol, candidateID: id, ordinal: ordinal, path: path, bound: true, stage: "preview"}
	if root.protocol == 4 {
		p.notifier = preview.New(func(frame preview.Frame, ops []preview.Operation) {
			p.publishLocked(Progress{Kind: preview.Kind, CoordinateSystem: frame.CoordinateSystem, Operations: ops, PreviewAreas: frame.Areas, PreviewPrimaryAreaIndex: frame.PrimaryAreaIndex}, false)
		})
	} else {
		p.notifier = root.notifier
	}
	return p
}

// guard 职责：选择独占候选锁或v3共享通知锁；使用：全部旁路状态访问；边界：不保护领域实体。
func (p *generationPreview) guard() *sync.Mutex {
	if p.protocol != 4 && p.root != nil {
		return &p.root.mu
	}
	return &p.mu
}

// publishLocked 职责：按会话顺序原子提交候选事件；使用：持有guard的调用方；边界：提交前检查取消，已分配序号不能因等待全局发送锁时取消而丢失，强制提交用于开始、完整快照标记及终结撤销。
func (p *generationPreview) publishLocked(event Progress, terminal bool) {
	ctx := p.ctx
	if !terminal && ctx.Err() != nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	event.Operations = append([]preview.Operation(nil), event.Operations...)
	for i := range event.Operations {
		if event.Operations[i].Building != nil {
			value := *event.Operations[i].Building
			event.Operations[i].Building = &value
		}
	}
	event.PreviewAreas = append([]preview.Area(nil), event.PreviewAreas...)
	event.Candidates = append([]CandidatePreviewPlan(nil), event.Candidates...)
	p.sequence++
	event.ProtocolVersion, event.CandidateID, event.CandidateOrdinal = 4, p.candidateID, p.ordinal
	event.CandidateSequence, event.Stage = p.sequence, p.stage
	publishProgress(ctx, event)
}

// publish 职责：串行发布候选快照；使用：最终获选快照分块；边界：不分配候选身份。
func (p *generationPreview) publish(event Progress) {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	p.publishLocked(event, false)
}

// start 职责：仅为已绑定完整Solve的候选创建平台；使用：装配器开始提交实体前；边界：外层组事件不能调用本方法造平台。
func (p *generationPreview) start() {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	if p.protocol != 4 || p.started || p.finished || !p.bound || p.ctx.Err() != nil {
		return
	}
	p.started = true
	publishProgress(context.WithoutCancel(p.ctx), Progress{ProtocolVersion: 4, Kind: "candidate-plan", Stage: "preview", Candidates: []CandidatePreviewPlan{{ID: p.candidateID, Ordinal: p.ordinal}}})
	p.publishLocked(Progress{Kind: "candidate-start", Message: "候选方案开始生成"}, true)
}

// finish 职责：终结完整候选或裁掉已验证落选平台；使用：验证成功、失败、引擎淘汰和任务清理；边界：valid仍可转pruned，失败后不再接受实体变化。
func (p *generationPreview) finish(status string) {
	if p == nil {
		return
	}
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	p.finishLocked(status)
}

// finishLocked 职责：在已有会话事务中提交终结；使用：验证与普通finish；边界：调用方必须持有guard，不能复活已撤销会话。
func (p *generationPreview) finishLocked(status string) {
	if p.status == "rejected" || p.status == "pruned" || p.status == status {
		return
	}
	if status == "valid" && p.ctx.Err() != nil {
		return
	}
	p.finished, p.status = true, status
	if p.protocol == 4 {
		if p.started {
			p.publishLocked(Progress{Kind: "candidate-finish", CandidateStatus: status}, status != "valid")
		}
		p.notifier.Discard()
	} else if status != "valid" {
		if p.root != nil && p.root.active != p {
			return
		}
		if p.root != nil {
			p.root.terminal = true
			defer func() { p.root.terminal = false }()
		}
		p.notifier.Sync(preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
	}
}

// syncScene 职责：在串行边界提交值场景；使用：装配检查点及最终验证姿态；边界：失败候选不能继续写预览。
func (p *generationPreview) syncScene(stage string, scene preview.Scene) {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	if p.finished || p.ctx.Err() != nil {
		return
	}
	p.stage = stage
	if p.protocol != 4 && p.root != nil {
		p.root.stage = stage
		p.root.active = p
	}
	p.notifier.Sync(scene)
}

// patchScene 职责：在串行边界提交真实追加对象；使用：路由事务；边界：完整同步回退由调用者处理。
func (p *generationPreview) patchScene(frame preview.Frame, objects []preview.Object) bool {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	if p.finished || p.ctx.Err() != nil {
		return true
	}
	p.stage = "routing"
	if p.protocol != 4 && p.root != nil {
		p.root.stage = "routing"
		p.root.active = p
	}
	return p.notifier.Patch(frame, objects)
}

// batchCount 职责：读取已交付批次；使用：正式完成事件；边界：与并发生命周期访问串行。
func (p *generationPreview) batchCount() int {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	return p.notifier.BatchCount()
}

// previewFromContext 职责：读取单任务预览会话；使用：Builder 初始化和最终提交；边界：未启用时返回 nil。
func previewFromContext(ctx context.Context) *generationPreview {
	session, _ := ctx.Value(generationPreviewKey{}).(*generationPreview)
	return session
}

// objectID 职责：为首次可见实体分配稳定身份；使用：构建只读渲染快照；边界：实体删除后 ID 不复用，候选试装未显示时不必分配。
func (p *generationPreview) objectID(entity *placed) string {
	lock := p.guard()
	lock.Lock()
	defer lock.Unlock()
	allocator := p
	if p.protocol != 4 && p.root != nil {
		allocator = p.root
	}
	if entity.previewID == "" {
		allocator.nextID++
		entity.previewID = "building-" + strconv.FormatUint(allocator.nextID, 10)
	}
	return entity.previewID
}

// beginPreview 职责：标记真实生成候选可见并撤下前一个候选；使用：实际候选开始；边界：几何尺寸探测的临时 Builder 不调用此入口。
func (b *builder) beginPreview() {
	if b.preview == nil {
		return
	}
	b.previewActive = true
	if b.preview.protocol == 4 && b.preview != b.preview.root {
		b.preview.start()
	}
	b.previewCheckpoint("layout")
}

// previewCheckpoint 职责：在实体提交或平移边界发布变化；使用：真实装配和路线提交；边界：不发送 A* 节点、不调用 records 修改索引或触发正式校验。
func (b *builder) previewCheckpoint(stage string) {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	if b.preview.protocol == 4 {
		if b.preview == b.preview.root && len(b.all) == 0 {
			return
		}
		if b.preview == b.preview.root {
			return
		}
		b.preview.start()
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
		building := renderPreviewBuilding(record, entity.owner, true)
		building.InputObjectID, building.OutputObjectID = ids[entity.input], ids[entity.output]
		objects = append(objects, preview.Object{ID: ids[entity], Building: building})
	}
	b.preview.syncScene(stage, preview.Scene{Objects: objects, Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
	b.previewEntities = ids
	b.previewOrder = append(b.previewOrder[:0], b.all...)
}

// previewLocalCheckpoint 职责：只投影本次机器行及其局部附件，避免每行重复扫描已完成的整厂。
// 使用：调用方保证start之前实体未变；边界：索引或坐标语义不匹配回退完整检查点，原始实体和最终规则不变。
func (b *builder) previewLocalCheckpoint(stage string, start int) {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	if start < 0 || start > len(b.all) || start > len(b.previewOrder) || len(b.previewEntities) != len(b.previewOrder) {
		b.previewCheckpoint(stage)
		return
	}
	if b.previewEntities == nil {
		b.previewEntities = make(map[*placed]string)
	}
	members := make(map[*placed]bool, len(b.all)-start)
	for _, entity := range b.all[start:] {
		if entity == nil {
			b.previewCheckpoint(stage)
			return
		}
		members[entity] = true
		if b.previewEntities[entity] == "" {
			b.previewEntities[entity] = b.preview.objectID(entity)
		}
	}
	objects := make([]preview.Object, 0, len(b.all)-start)
	for _, entity := range b.all[start:] {
		objects = append(objects, b.previewObject(entity))
	}
	p := b.preview
	lock := p.guard()
	lock.Lock()
	if p.finished || p.ctx.Err() != nil {
		lock.Unlock()
		return
	}
	p.stage = stage
	if p.protocol != 4 && p.root != nil {
		p.root.stage = stage
		p.root.active = p
	}
	patched := p.notifier.SyncSuffix(preview.Frame{CoordinateSystem: preview.LayoutCoordinates}, start, objects)
	lock.Unlock()
	if !patched {
		b.previewCheckpoint(stage)
		return
	}
	for _, old := range b.previewOrder[start:] {
		if !members[old] {
			delete(b.previewEntities, old)
		}
	}
	b.previewOrder = append(b.previewOrder[:start], b.all[start:]...)
}

// previewObject 职责：投影一个已分配身份的真实实体；使用：局部装配与路线追加共用；边界：只读几何，未知引用仍为空身份。
func (b *builder) previewObject(entity *placed) preview.Object {
	record := entity.record
	record.Index, record.AreaIndex = -1, 0
	record.LocalOffset = blueprint.Point{X: float32(entity.a.x), Y: float32(entity.a.y), Z: float32(entity.a.z)}
	record.LocalOffset2 = blueprint.Point{X: float32(entity.b.x), Y: float32(entity.b.y), Z: float32(entity.b.z)}
	building := renderPreviewBuilding(record, entity.owner, true)
	building.InputObjectID, building.OutputObjectID = b.previewEntities[entity.input], b.previewEntities[entity.output]
	return preview.Object{ID: b.previewEntities[entity], Building: building}
}

// previewRouteAppend 职责：发布一条已提交路线的新带节点及起点连线；使用：routeTransportWithStrategy 的追加事务末端；边界：必须先完整同步全部端口，裁剪、平移和其它更新继续使用完整检查点。
func (b *builder) previewRouteAppend(base int, source *placed) {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	if base < 0 || base > len(b.all) || len(b.previewEntities) != base || b.previewEntities[source] == "" {
		b.previewCheckpoint("routing")
		return
	}
	for _, entity := range b.all[base:] {
		if entity == nil || b.previewEntities[entity] != "" {
			b.previewCheckpoint("routing")
			return
		}
		b.previewEntities[entity] = b.preview.objectID(entity)
	}
	entities := make([]*placed, 1, len(b.all)-base+1)
	entities[0] = source
	entities = append(entities, b.all[base:]...)
	objects := make([]preview.Object, 0, len(entities))
	for _, entity := range entities {
		objects = append(objects, b.previewObject(entity))
	}
	if !b.preview.patchScene(preview.Frame{CoordinateSystem: preview.LayoutCoordinates}, objects) {
		b.previewCheckpoint("routing")
	} else {
		b.previewOrder = append(b.previewOrder, b.all[base:]...)
	}
}

// rollbackPreview 职责：撤下真实失败或被放弃候选的全部临时对象；使用：候选失败边界；边界：不影响实体库存、告警或失败原因。
func (b *builder) rollbackPreview() {
	if b.preview == nil || !b.previewActive || b.preview.ctx.Err() != nil {
		return
	}
	if b.preview.protocol == 4 {
		b.preview.finish("rejected")
		b.previewActive = false
		return
	}
	b.preview.finish("rejected")
	b.previewActive = false
}

// renderPreviewBuilding 职责：从建筑记录摘取渲染值；使用：临时及最终姿态共用；边界：不复制参数数组和原始编码尾部，只摘取配方与带标记物品两个图标字段，连接身份由调用方填写。
func renderPreviewBuilding(record blueprint.Building, owner int, isTemporary bool) preview.Building {
	return preview.Building{Index: record.Index, IsTemporary: isTemporary, ItemID: record.ItemID, ModelIndex: record.ModelIndex, AreaIndex: record.AreaIndex, LocalOffset: record.LocalOffset, LocalOffset2: record.LocalOffset2, Yaw: record.Yaw, Yaw2: record.Yaw2, Tilt: record.Tilt, Tilt2: record.Tilt2, Pitch: record.Pitch, Pitch2: record.Pitch2, InputFromSlot: record.InputFromSlot, OutputToSlot: record.OutputToSlot, ProductionIndex: owner, RecipeID: max(record.RecipeID, 0), IconItemID: previewBeltMarkerItem(record)}
}

// previewBeltMarkerItem 职责：取出传送带标记参数里的物品；使用：入口与出口的带标记图标；边界：只认传送带且参数不少于二的记录，其他建筑的参数含义不同，一律返回零。
func previewBeltMarkerItem(record blueprint.Building) int32 {
	if !surfaceIsBelt(record.ItemID) || len(record.Parameters) < 2 || record.Parameters[0] <= 0 {
		return 0
	}
	return record.Parameters[0]
}

// previewAreas 职责：把蓝图区域转换为实时预览契约；使用：候选最终提交；边界：原始 Area 记录继续保持扁平结构。
func previewAreas(areas []blueprint.Area) []preview.Area {
	converted := make([]preview.Area, len(areas))
	for index, area := range areas {
		converted[index] = preview.Area{
			Index: area.Index, ParentIndex: area.ParentIndex, TropicAnchor: area.TropicAnchor,
			AreaSegments:      area.AreaSegments,
			AnchorLocalOffset: preview.Point2{X: area.AnchorLocalOffsetX, Y: area.AnchorLocalOffsetY},
			Size:              preview.Point2{X: area.Width, Y: area.Height},
		}
	}
	return converted
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
	if session.protocol == 4 {
		session = result.previewScene.session
		lock := session.guard()
		lock.Lock()
		defer lock.Unlock()
		if err := session.publishSnapshotLocked(ctx, result.previewScene.scene, Progress{Kind: ProgressKindPreviewComplete, WinnerCandidateID: session.candidateID, BatchCount: session.notifier.BatchCount()}); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return previewContextError(err)
		}
		return nil
	}
	session.syncScene("preview", result.previewScene.scene)
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	version := preview.ProtocolVersion
	if session.protocol == 4 {
		version = 4
	}
	winnerID := ""
	if result.previewScene.session != nil {
		winnerID = result.previewScene.session.candidateID
	}
	publishProgress(ctx, Progress{ProtocolVersion: version, Kind: ProgressKindPreviewComplete, CandidateID: winnerID, WinnerCandidateID: winnerID, Stage: "preview", Message: "生成预览已提交正式结果", CoordinateSystem: preview.AreaCoordinates, BuildingCount: result.BuildingCount, BatchCount: session.batchCount()})
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	return nil
}

// clearGenerationPreview 职责：为未成功任务发布撤销终结；使用：任务Finish；边界：取消后只允许已提交事件及最终撤销，不把残留画面视为成功。
func clearGenerationPreview(ctx context.Context) {
	if session := previewFromContext(ctx); session != nil {
		if session.root != nil {
			session = session.root
		}
		if session.protocol == 4 {
			publishProgress(context.WithoutCancel(ctx), Progress{ProtocolVersion: 4, Kind: "search-failed", Stage: "preview", Message: "候选搜索未成功"})
			return
		}
		lock := session.guard()
		lock.Lock()
		session.terminal = true
		session.stage = "preview"
		session.notifier.Sync(preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.LayoutCoordinates}})
		session.terminal = false
		lock.Unlock()
	}
}
