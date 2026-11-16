package build

// 文件职责：从封闭定稿或验证产物读取旁路身份并终结生命周期。
// 使用：生成工具的验证结果与引擎事件适配；边界：不更改定稿、证据、记录或编码内容，不持有第二份领域状态。

import (
	"context"
	"fmt"
)

// geometryObservationSession 职责：读取定稿附带的内部旁路会话；使用：验证与淘汰；边界：不对外暴露通知器或可变实体。
func geometryObservationSession(geometry GeometrySnapshot) *generationPreview {
	if geometry.data == nil || geometry.data.previewScene == nil {
		return nil
	}
	return geometry.data.previewScene.session
}

// CompleteValidatedObservation 职责：真实完整验证通过后先更新best再标记valid；使用：引擎succeeded事件；边界：v3保持原行为，祖先重复成功不再发布，已撤销会话不能复活。
func CompleteValidatedObservation(ctx context.Context, verified VerifiedSnapshot) error {
	if _, err := FreezeValidatedGeometry(ctx, verified.Geometry(), verified.Validation()); err != nil {
		return err
	}
	if session := geometryObservationSession(verified.Geometry()); session != nil {
		state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
		if state == nil {
			return fmt.Errorf("验证场景不属于当前有效任务观察作用域")
		}
		state.lifecycle.Lock()
		defer state.lifecycle.Unlock()
		if !observationOwnsSession(ctx, session) {
			return fmt.Errorf("验证场景不属于当前有效任务观察作用域")
		}
		if session.protocol != 4 {
			session.syncScene("rules", verified.geometry.data.previewScene.scene)
			session.finish("valid")
			return nil
		}
		lock := session.guard()
		lock.Lock()
		defer lock.Unlock()
		if session.observedValid || session.status == "pruned" || session.status == "rejected" || ctx.Err() != nil {
			return nil
		}
		session.observedValid = true
		session.stage = "rules"
		session.notifier.Sync(verified.geometry.data.previewScene.scene)
		state.offerBestLocked(ctx, verified, false)
		session.finishLocked("valid")
	}
	return nil
}

// RejectGeometryObservation 职责：撤下验证失败或取消定稿对应的真实场景；使用：Validate工具错误分支；边界：不修改实体，取消后仍发送协议终结以撤销平台。
func RejectGeometryObservation(ctx context.Context, geometry GeometrySnapshot) {
	state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
	if state == nil {
		return
	}
	state.lifecycle.Lock()
	defer state.lifecycle.Unlock()
	if session := geometryObservationSession(geometry); session != nil && observationOwnsSession(ctx, session) {
		state.clearBestInScopeLocked(session.path)
		session.finish("rejected")
	}
}

// observationOwnsSession 职责：校验旁路任务归属和生命周期；使用：封闭产物事件入口；边界：跨任务或已结束任务不能改写场景状态。
func observationOwnsSession(ctx context.Context, session *generationPreview) bool {
	state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
	if state == nil || session == nil || session.root != state.root {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return !state.closed
}

// publishObservationWinner 职责：按已编码产物的身份裁掉其余平台；使用：PublishResult正式发布之前；边界：不按完成时间或最近succeeded推测winner，不编码其它候选。
func publishObservationWinner(ctx context.Context, result Result) error {
	state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
	if state == nil || state.root == nil {
		return nil
	}
	state.lifecycle.Lock()
	defer state.lifecycle.Unlock()
	if result.previewScene == nil || result.previewScene.session == nil {
		return fmt.Errorf("获选编码产物缺少旁路会话")
	}
	winner := result.previewScene.session
	if !observationOwnsSession(ctx, winner) {
		return fmt.Errorf("正式场景不属于当前有效任务观察作用域")
	}
	if winner.root != state.root {
		return fmt.Errorf("获选场景不属于本生成任务")
	}
	lock := winner.guard()
	lock.Lock()
	failed := winner.status == "rejected" || winner.status == "pruned"
	lock.Unlock()
	if failed {
		return fmt.Errorf("获选产物的旁路会话已被引擎撤销")
	}
	winner.finish("valid")
	if state.best != nil && state.best.snapshot.session != winner {
		state.clearBestInScopeLocked("")
	}
	for _, entry := range state.matching("") {
		if entry.preview != winner {
			entry.preview.finish("pruned")
		}
	}
	return nil
}
