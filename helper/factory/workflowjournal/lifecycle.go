package workflowjournal

// 文件职责：支持服务在输入校验前占用身份、在真实配置装载后绑定图；使用：Generate 生命周期；边界：不接触请求实体，容量不足只关闭跟踪，不执行或取消业务。

import (
	"encoding/json"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// Reserve 职责：在任何取消旧任务或校验输入之前原子占用新身份；使用：服务 Generate 入口；边界：无图无事件，必须以 BindPlan 或 FinishBeforeStart 完结，同 ID 拒绝。
func (s *Store) Reserve(taskID string) (*Writer, error) {
	return s.start(taskID, workflow.PlanDescription{}, "running", "pending", "等待装载实际冻结计划")
}

// BindPlan 职责：将已预留任务绑定到实际执行的冻结图；使用：WithWorkflowStart 回调；边界：只能绑定一次，容量不足保留指纹及不可用说明，不能因此改变业务结果。
func (w *Writer) BindPlan(graph workflow.PlanDescription) error {
	if w == nil || w.store == nil {
		return fault("invalid_argument", "写入能力为空")
	}
	raw, marshalErr := json.Marshal(graph)
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	if err := w.writable(); err != nil {
		return err
	}
	entry := w.entry
	if entry.fingerprint != "" || entry.trackingCode != "pending" {
		return fault("conflict", "任务已经绑定冻结图")
	}
	entry.fingerprint = bounded(graph.Fingerprint, 256)
	if marshalErr != nil || graph.Fingerprint == "" || len(graph.Fingerprint) > 256 {
		entry.trackingCode = "invalid_argument"
		entry.trackingMessage = "冻结图无效，跟踪不可用"
		return fault("invalid_argument", entry.trackingMessage)
	}
	graphBytes := len(raw)
	for _, other := range s.entries {
		graphBytes += len(other.graph)
	}
	if len(raw) > s.limits.MaxGraphBytes || len(raw) > s.limits.MaxTaskBytes || graphBytes > s.limits.MaxTotalBytes {
		entry.trackingCode = "capacity"
		entry.trackingMessage = "冻结图超过日志容量，生成继续但执行轨迹不可用"
		return fault("capacity", entry.trackingMessage)
	}
	for s.bytes+len(raw) > s.limits.MaxTotalBytes {
		if !s.dropOldestEvent() {
			entry.trackingCode = "capacity"
			entry.trackingMessage = "日志总容量不足，执行轨迹不可用"
			return fault("capacity", entry.trackingMessage)
		}
	}
	entry.graph = raw
	entry.bytes += len(raw)
	s.bytes += len(raw)
	entry.trackingCode = ""
	entry.trackingMessage = ""
	return nil
}

// FinishBeforeStart 职责：结束尚未成功装载方案的预留任务；使用：服务输入校验、取消等待或配置装载失败；边界：无图无引擎事件，已绑定计划时拒绝。
func (w *Writer) FinishBeforeStart(code, message string) error {
	if w == nil || w.store == nil {
		return fault("invalid_argument", "写入能力为空")
	}
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	if err := w.writable(); err != nil {
		return err
	}
	entry := w.entry
	if entry.fingerprint != "" || len(entry.graph) > 0 {
		return fault("conflict", "任务已经开始执行冻结方案")
	}
	entry.state = "failed-before-start"
	entry.finished = s.now()
	entry.code = bounded(code, 128)
	entry.message = bounded(message, 2048)
	entry.trackingCode = ""
	entry.trackingMessage = ""
	return nil
}
