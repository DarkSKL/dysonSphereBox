package build

// 文件职责：发布旁路当前最佳及完整正式坐标快照。
// 使用：完整验证成功、父作用域选优和撤销；边界：仅保留一个渲染快照与评分，不保留领域实体，不参与算法选择。

import (
	"context"
	"strings"

	"DysonSphereBox/helper/blueprint/build/preview"
)

// observationBest 职责：保存唯一当前最佳的不可变场景和真实评分；使用：旁路稳定比较；边界：不引用VerifiedSnapshot及其实体证据。
type observationBest struct {
	// snapshot 职责：保存独立渲染值及会话身份；使用：恢复已释放场景；边界：不保存建筑参数或实体指针。
	snapshot *generationPreviewSnapshot
	// metrics 职责：保存完整验证的共同排序指标；使用：BetterLayoutCandidate；边界：不估算路线。
	metrics LayoutMetrics
	// facilities 职责：记录真实物流设施数量；使用：与BetterAdjacentCandidate相同的共同排序；边界：只计物流塔及配送器。
	facilities int
}

// observationInScope 职责：匹配完整路径作用域；使用：父作用域撤销；边界：不将同名前缀的兄弟作用域混淆。
func observationInScope(path, scope string) bool {
	return scope == "" || path == scope || strings.HasPrefix(path, scope+"/")
}

// observationFacilities 职责：从真实验证文档计数设施；使用：共同评分；边界：与BetterAdjacentCandidate一致，不按候选类型推测。
func observationFacilities(verified VerifiedSnapshot) int {
	count := 0
	for _, record := range verified.geometry.data.document.Buildings {
		if surfaceIsStation(record.ItemID) || record.ItemID == 2107 {
			count++
		}
	}
	return count
}

// offerBestLocked 职责：按共同排序及稳定路径更新唯一最佳；使用：持有lifecycle和候选guard的验证/选优事务；边界：force仅用于引擎撤销旧best后的纠正，不改写引擎决定。
func (state *candidateObservations) offerBestLocked(ctx context.Context, verified VerifiedSnapshot, force bool) bool {
	snapshot := verified.geometry.data.previewScene
	if snapshot == nil || snapshot.session == nil || snapshot.session.protocol != 4 || !LayoutShapeAccepted(verified.Metrics()) ||
		snapshot.scene.Frame.CoordinateSystem != preview.AreaCoordinates || len(snapshot.scene.Objects) != len(verified.geometry.data.document.Buildings) {
		return false
	}
	p := snapshot.session
	metrics, facilities := verified.Metrics(), observationFacilities(verified)
	if !validLayoutCandidateCost(metrics, facilities) || !p.started || p.status == "pruned" || p.status == "rejected" || ctx.Err() != nil {
		return false
	}
	if best := state.best; best != nil {
		previous := best.snapshot.session
		if previous == p {
			return true
		}
		if !force && !BetterLayoutCandidate(metrics, facilities, best.metrics, best.facilities) {
			if BetterLayoutCandidate(best.metrics, best.facilities, metrics, facilities) || p.path > previous.path || (p.path == previous.path && p.candidateID >= previous.candidateID) {
				return false
			}
		}
	}
	// 逻辑流：完整复制值场景 => 分块发布 => 提交best标记 => 替换唯一缓存；失败只留下未提交块，任务撤销负责清理。
	copy := newFinalizationCloner().previewCopy(snapshot)
	if err := p.publishSnapshotLocked(ctx, copy.scene, Progress{Kind: "candidate-best", Message: "当前最佳方案已更新"}); err != nil {
		return false
	}
	state.best = &observationBest{snapshot: copy, metrics: metrics, facilities: facilities}
	return true
}

// clearBestInScopeLocked 职责：在撤销前清除作用域内最佳；使用：持有lifecycle的拒绝、取消和失败事务；边界：任务级事件不带candidateId，重复clear无事件。
func (state *candidateObservations) clearBestInScopeLocked(scope string) {
	if state.best == nil || !observationInScope(state.best.snapshot.session.path, scope) {
		return
	}
	state.best = nil
	if state.root != nil && state.root.protocol == 4 {
		publishProgress(context.WithoutCancel(state.root.ctx), Progress{ProtocolVersion: 4, Kind: "candidate-best-cleared", Stage: "preview", Message: "当前最佳方案已撤销"})
	}
}

// reconcileBestLocked 职责：在父selected裁剪前恢复空缺或纠正即将失效的best；使用：持有lifecycle的引擎选优事务；边界：不覆盖其它作用域的best，无合格选中快照时明确clear，绝不恢复已pruned会话。
func (state *candidateObservations) reconcileBestLocked(ctx context.Context, scope string, verified VerifiedSnapshot) {
	winner := geometryObservationSession(verified.Geometry())
	force := false
	if best := state.best; best != nil {
		if !observationInScope(best.snapshot.session.path, scope) || winner == best.snapshot.session {
			return
		}
		force = true
	}
	if winner != nil && observationOwnsSession(ctx, winner) {
		if _, err := FreezeValidatedGeometry(ctx, verified.Geometry(), verified.Validation()); err == nil {
			lock := winner.guard()
			lock.Lock()
			replaced := state.offerBestLocked(ctx, verified, force)
			lock.Unlock()
			if replaced {
				return
			}
		}
	}
	state.clearBestInScopeLocked(scope)
}

// publishSnapshotLocked 职责：为完整正式场景发布独立版本事务；使用：持有guard的best和最终结果；边界：标记仅在全部块送达后发布，每次调用独占新revision，取消不能复用旧版本。
func (p *generationPreview) publishSnapshotLocked(ctx context.Context, scene preview.Scene, marker Progress) error {
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	p.revision++
	chunks := (len(scene.Objects) + preview.BatchSize - 1) / preview.BatchSize
	for start := 0; start < len(scene.Objects); start += preview.BatchSize {
		if err := ctx.Err(); err != nil {
			return previewContextError(err)
		}
		end := min(start+preview.BatchSize, len(scene.Objects))
		operations := make([]preview.Operation, 0, end-start)
		for _, object := range scene.Objects[start:end] {
			building := object.Building
			operations = append(operations, preview.Operation{Type: preview.Add, ObjectID: object.ID, Building: &building})
		}
		p.publishLocked(Progress{Kind: "preview-snapshot", SnapshotRevision: p.revision, ChunkIndex: start / preview.BatchSize, ChunkCount: chunks, CoordinateSystem: scene.Frame.CoordinateSystem, Operations: operations, PreviewAreas: scene.Frame.Areas, PreviewPrimaryAreaIndex: scene.Frame.PrimaryAreaIndex}, false)
	}
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	marker.SnapshotRevision, marker.ChunkCount, marker.BuildingCount = p.revision, chunks, len(scene.Objects)
	marker.CoordinateSystem, marker.PreviewAreas, marker.PreviewPrimaryAreaIndex = scene.Frame.CoordinateSystem, scene.Frame.Areas, scene.Frame.PrimaryAreaIndex
	// 完整块已送达且取消检查通过后，标记成为已提交事务；回调中取消不能让后台漏记已公开的best。
	p.publishLocked(marker, true)
	return nil
}
