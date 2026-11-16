package workflowjournal

// 文件职责：提供一致快照及游标分页；使用：Wails 只读服务；边界：锁内反序列化生成独立输出，不返回内部引用，不推进任务。

import (
	"encoding/json"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// Describe 职责：限制并包装当前图查询；使用：服务 DescribeWorkflow；边界：图超限整体拒绝，错误文本封顶，不截断节点关系。
func Describe(graph workflow.PlanDescription, err error) Description {
	if err != nil {
		return Description{Code: "configuration_error", Message: bounded(err.Error(), 2048)}
	}
	raw, marshalErr := json.Marshal(graph)
	if marshalErr != nil || len(raw) > 4<<20 {
		return Description{Code: "capacity", Message: "当前冻结图超过公开查询容量"}
	}
	var copy PlanDescription
	if decodeWireJSON(raw, &copy) != nil {
		return Description{Code: "configuration_error", Message: "当前冻结图无法序列化"}
	}
	return Description{Code: "ok", Graph: &copy}
}

// lookup 职责：统一任务查询状态；使用：持锁 Read/Snapshot；边界：返回内部记录仅限包内持锁使用。
func (s *Store) lookup(id string) (*record, string, string) {
	if !validID(id) {
		return nil, "invalid_argument", "任务身份无效"
	}
	if s.closed {
		return nil, "closed", "工作流日志已关闭"
	}
	if entry := s.entries[id]; entry != nil {
		return entry, "ok", ""
	}
	if s.expired(id) {
		return nil, "expired", "任务日志已过期或因容量被移除"
	}
	return nil, "not_found", "未找到任务日志；过期标记超过保留窗口后也返回此状态"
}

// first 职责：计算保留前缀界限；使用：快照和分页；边界：空队列返回最后序号加一。
func (entry *record) first() uint64 {
	if len(entry.events) > 0 {
		return entry.events[0].sequence
	}
	return entry.last + 1
}

// Snapshot 职责：读取任务实际绑定的图与最终状态；使用：任务恢复；边界：过期或未知返回显式 Code，不查询当前配置。
func (s *Store) Snapshot(taskID string) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	entry, code, message := s.lookup(taskID)
	result := Snapshot{Code: code, Message: message, TaskID: bounded(taskID, 128)}
	if entry == nil {
		return result
	}
	var graph PlanDescription
	if len(entry.graph) > 0 {
		_ = decodeWireJSON(entry.graph, &graph)
		result.Graph = &graph
	}
	result.State = entry.state
	result.Fingerprint = entry.fingerprint
	result.TrackingAvailable = len(entry.graph) > 0
	result.TrackingErrorCode = entry.trackingCode
	result.TrackingErrorMessage = entry.trackingMessage
	result.StartedAt = wireTime(entry.started)
	result.FinishedAt = wireTime(entry.finished)
	result.FirstSequence = entry.first()
	result.LastSequence = entry.last
	result.DroppedEvents = entry.dropped
	result.LostEvents = entry.lost
	result.EngineEventsLost = entry.engineLost
	result.EventCount = len(entry.events)
	result.RetainedBytes = entry.bytes
	result.ErrorCode = entry.code
	result.ErrorMessage = entry.message
	return result
}

// Read 职责：返回独立的有界增量事件页；使用：执行中或完成后轮询；边界：limit 必须为一至上限，超前游标拒绝，缺失前缀用 cursor_gap 返回。
func (s *Store) Read(taskID string, after uint64, limit int) Page {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	result := Page{TaskID: bounded(taskID, 128), Events: []Event{}, Next: after}
	if limit < 1 || limit > s.limits.MaxPageSize {
		result.Code = "invalid_argument"
		result.Message = "分页大小超出允许范围"
		return result
	}
	entry, code, message := s.lookup(taskID)
	result.Code = code
	result.Message = message
	if entry == nil {
		return result
	}
	result.FirstSequence = entry.first()
	result.LastSequence = entry.last
	if after > entry.last {
		result.Code = "invalid_cursor"
		result.Message = "游标超过已接收事件上界"
		return result
	}
	if entry.state != "failed-before-start" && len(entry.graph) == 0 {
		result.Code = "tracking_unavailable"
		result.Message = entry.trackingMessage
		result.Complete = entry.state != "running"
		return result
	}
	if after < result.FirstSequence-1 {
		result.Code = "cursor_gap"
		result.Message = "游标之前的事件已淘汰，返回当前保留前缀"
		result.Gap = true
		result.Next = result.FirstSequence - 1
	}
	for _, item := range entry.events {
		if item.sequence <= after {
			continue
		}
		var event Event
		_ = decodeWireJSON(item.raw, &event)
		result.Events = append(result.Events, event)
		result.Next = item.sequence
		if len(result.Events) == limit {
			break
		}
	}
	result.Complete = entry.state != "running" && result.Next == entry.last
	return result
}
