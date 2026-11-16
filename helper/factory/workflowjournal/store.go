package workflowjournal

// 文件职责：保存有界并发任务日志和短期过期标记；使用：生成服务登记与查询；边界：不执行工作流，不解析文本推断事件，无后台线程。
// 逻辑流：1. 冻结图并签发写入能力 => 2. 锁内签发日志游标并淘汰旧前缀 => 3. 完成后保留 TTL => 4. 查询时惰性清理并明确过期。

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// storedEvent 职责：保留独立 JSON 和日志序号；使用：容量队列；边界：不保存调用方 map 或 slice。
type storedEvent struct {
	sequence uint64
	raw      []byte
}

// record 职责：保存单个任务的能力身份及有界载荷；使用：Store 锁保护；边界：图固定，终态不可再写。
type record struct {
	id, fingerprint, state, code, message string
	trackingCode, trackingMessage         string
	graph                                 []byte
	events                                []storedEvent
	started, finished                     time.Time
	last, dropped, lost, engineLost       uint64
	bytes                                 int
}

// tombstone 职责：保留短期过期语义；使用：区分未知和曾存在任务；边界：最多两倍 MaxTasks，超出后按 not_found 处理。
type tombstone struct {
	id    string
	until time.Time
}

// Store 职责：持有有界内存日志；使用：服务字段零值或 New；边界：不得在首次使用后复制，不持有内部实体，活动任务满员时拒绝新任务。
type Store struct {
	mu      sync.Mutex
	limits  Limits
	entries map[string]*record
	order   []string
	tombs   []tombstone
	bytes   int
	closed  bool
	// now 职责：隔离测试时钟；使用：包内 TTL 测试；边界：生产使用 UTC 当前时间。
	now func() time.Time
}

// Writer 职责：签发单个任务不可冒用的写入能力；使用：生成生命周期；边界：任务移除或终态后拒绝，不按相同 TaskID 找回新记录。
type Writer struct {
	store *Store
	entry *record
}

// New 职责：创建受限日志；使用：测试或服务定制预算；边界：只能收紧默认预算，零值 Store 同样可用。
func New(limits Limits) (*Store, error) {
	l, err := normalized(limits)
	if err != nil {
		return nil, err
	}
	return &Store{limits: l}, nil
}

// initialize 职责：补齐零值依赖；使用：持锁入口；边界：不启动后台任务。
func (s *Store) initialize() {
	if s.entries == nil {
		s.entries = make(map[string]*record)
		if s.limits.MaxTasks == 0 {
			s.limits, _ = normalized(Limits{})
		}
	}
	if s.now == nil {
		s.now = func() time.Time { return time.Now().UTC() }
	}
}

// Start 职责：冻结图并登记唯一任务；使用：实际计划装载后；边界：同 ID 冲突和活动容量不足返回错误，不覆盖已有任务。
func (s *Store) Start(taskID string, graph workflow.PlanDescription) (*Writer, error) {
	return s.start(taskID, graph, "", "", "")
}

// FailBeforeStart 职责：记录尚未成功装载计划的真实启动失败；使用：服务校验或配置装载失败；边界：无冻结图、不伪造引擎事件，同 ID 仍拒绝。
func (s *Store) FailBeforeStart(taskID, code, message string) error {
	_, err := s.start(taskID, workflow.PlanDescription{}, "failed-before-start", code, message)
	return err
}

// StartUnavailable 职责：在旁路资源不足时保留可查询业务状态；使用：服务对 Start 的容量失败降级；边界：无图无事件，仍拒绝身份冲突和活动任务满员，不阻止调用方继续生成。
func (s *Store) StartUnavailable(taskID, fingerprint, code, message string) (*Writer, error) {
	if fingerprint == "" {
		return nil, fault("invalid_argument", "冻结图指纹为空")
	}
	return s.start(taskID, workflow.PlanDescription{Fingerprint: fingerprint}, "running", code, message)
}

// start 职责：统一准入并冻结任务元数据；使用：Start/FailBeforeStart；边界：提前失败没有图，不接受成功图缺少指纹。
func (s *Store) start(taskID string, graph workflow.PlanDescription, state, code, message string) (*Writer, error) {
	if !validID(taskID) || (state == "" && graph.Fingerprint == "") || len(graph.Fingerprint) > 256 {
		return nil, fault("invalid_argument", "任务身份或冻结图指纹无效")
	}
	taskID = strings.Clone(taskID)
	raw, err := json.Marshal(graph)
	if err != nil {
		return nil, fault("invalid_argument", "冻结图无法序列化")
	}
	if state != "" {
		raw = nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	if s.closed {
		return nil, fault("closed", "工作流日志已关闭")
	}
	if s.entries[taskID] != nil || s.expired(taskID) {
		return nil, fault("conflict", "任务身份已使用，请使用新的任务身份")
	}
	if len(raw) > s.limits.MaxGraphBytes || len(raw) > s.limits.MaxTaskBytes {
		return nil, fault("capacity", "冻结图超过日志容量")
	}
	if len(s.entries) >= s.limits.MaxTasks {
		removed := false
		for _, id := range s.order {
			if s.entries[id].state != "running" {
				s.remove(id)
				removed = true
				break
			}
		}
		if !removed {
			return nil, fault("capacity", "活动任务数量已达上限")
		}
	}
	graphBytes := len(raw)
	for _, entry := range s.entries {
		graphBytes += len(entry.graph)
	}
	if graphBytes > s.limits.MaxTotalBytes {
		return nil, fault("capacity", "冻结图总容量已达上限")
	}
	for s.bytes+len(raw) > s.limits.MaxTotalBytes {
		if !s.dropOldestEvent() {
			return nil, fault("capacity", "日志总容量已达上限")
		}
	}
	entry := &record{id: taskID, fingerprint: bounded(graph.Fingerprint, 256), state: "running", graph: raw, bytes: len(raw), started: s.now()}
	if state != "" {
		entry.state = state
		if state == "running" {
			entry.trackingCode = bounded(code, 128)
			entry.trackingMessage = bounded(message, 2048)
		} else {
			entry.finished = entry.started
			entry.code = bounded(code, 128)
			entry.message = bounded(message, 2048)
		}
	}
	s.entries[taskID] = entry
	s.order = append(s.order, taskID)
	s.bytes += len(raw)
	return &Writer{store: s, entry: entry}, nil
}

// Append 职责：保存真实引擎事件并签发独立游标；使用：ObserveExecution 同步回调；边界：失败只影响日志，不允许跨图或终态写入。
func (w *Writer) Append(event workflow.ExecutionEvent) error {
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
	entry.engineLost = max(entry.engineLost, event.EventsLost)
	if len(entry.graph) == 0 {
		entry.lost++
		return fault("unavailable", "任务执行轨迹未能保存")
	}
	if event.Fingerprint != "" && event.Fingerprint != entry.fingerprint {
		entry.lost++
		return fault("invalid_argument", "事件与任务冻结图不匹配")
	}
	event = sanitize(event)
	event.Fingerprint = entry.fingerprint
	if event.Timestamp.IsZero() {
		event.Timestamp = s.now()
	}
	if entry.last == ^uint64(0) {
		entry.lost++
		return fault("capacity", "日志序号已达上限")
	}
	// 内部按引擎JSON保留精确载荷；读取时再投影为Wails兼容的对象字段。
	value := struct {
		TaskID   string                  `json:"taskId"`
		Sequence uint64                  `json:"sequence"`
		Event    workflow.ExecutionEvent `json:"event"`
	}{TaskID: entry.id, Sequence: entry.last + 1, Event: event}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > s.limits.MaxEventBytes || len(raw)+len(entry.graph) > s.limits.MaxTaskBytes {
		entry.lost++
		return fault("event_too_large", "事件超过日志载荷限制")
	}
	for len(entry.events) >= s.limits.MaxEventsPerTask || entry.bytes+len(raw) > s.limits.MaxTaskBytes {
		s.drop(entry)
	}
	for s.bytes+len(raw) > s.limits.MaxTotalBytes {
		if !s.dropOldestEvent() {
			entry.lost++
			return fault("capacity", "日志总容量已达上限")
		}
	}
	entry.last++
	entry.events = append(entry.events, storedEvent{sequence: entry.last, raw: raw})
	entry.bytes += len(raw)
	s.bytes += len(raw)
	return nil
}

// Finish 职责：将任务不可逆地标记终态；使用：Generate 成功、失败或取消收尾；边界：不把候选成功当主任务成功，重复完成返回 finished。
func (w *Writer) Finish(state, code, message string) error {
	return w.FinishWithLost(state, code, message, 0)
}

// FinishWithLost 职责：保存终态及引擎最终丢失兜底；使用：真实 Generate 返回后；边界：重复完成拒绝，引擎丢失和归档失败可能重叠。
func (w *Writer) FinishWithLost(state, code, message string, engineLost uint64) error {
	if w == nil || w.store == nil {
		return fault("invalid_argument", "写入能力为空")
	}
	if state != "succeeded" && state != "failed" && state != "canceled" {
		return fault("invalid_argument", "任务终态无效")
	}
	s := w.store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialize()
	s.prune()
	if err := w.writable(); err != nil {
		return err
	}
	w.entry.state = state
	w.entry.engineLost = max(w.entry.engineLost, engineLost)
	w.entry.finished = s.now()
	w.entry.code = bounded(code, 128)
	w.entry.message = bounded(message, 2048)
	return nil
}

// writable 职责：校验能力仍指向当前活动记录；使用：持锁写入；边界：旧 writer 无法污染同 ID 的后续记录。
func (w *Writer) writable() error {
	s := w.store
	if s.closed {
		return fault("closed", "工作流日志已关闭")
	}
	if w.entry == nil || s.entries[w.entry.id] != w.entry {
		return fault("expired", "任务日志已移除")
	}
	if w.entry.state != "running" {
		return fault("finished", "任务已经结束")
	}
	return nil
}

// drop 职责：淘汰一个事件前缀并释放引用；使用：单任务及全局预算；边界：调用方须保证有事件。
func (s *Store) drop(entry *record) {
	length := len(entry.events[0].raw)
	entry.events[0] = storedEvent{}
	entry.events = entry.events[1:]
	entry.bytes -= length
	s.bytes -= length
	entry.dropped++
	if len(entry.events) == 0 {
		entry.events = nil
	}
}

// dropOldestEvent 职责：按任务登记顺序释放最早事件；使用：全局字节压力；边界：保留活动任务和冻结图，丢失前缀明确计数。
func (s *Store) dropOldestEvent() bool {
	for _, id := range s.order {
		if entry := s.entries[id]; entry != nil && len(entry.events) > 0 {
			s.drop(entry)
			return true
		}
	}
	return false
}

// expired 职责：检查有界过期标记；使用：Start/查询；边界：标记淘汰后只能返回 not_found。
func (s *Store) expired(id string) bool {
	for _, item := range s.tombs {
		if item.id == id {
			return true
		}
	}
	return false
}

// remove 职责：移除完整任务并保存短期墓碑；使用：TTL 或终态数量淘汰；边界：内部调用，不清理外部文件。
func (s *Store) remove(id string) {
	entry := s.entries[id]
	if entry == nil {
		return
	}
	s.bytes -= entry.bytes
	delete(s.entries, id)
	entry.graph = nil
	entry.events = nil
	entry.bytes = 0
	for index, value := range s.order {
		if value == id {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
	s.tombs = append(s.tombs, tombstone{id: id, until: s.now().Add(s.limits.TTL)})
	if len(s.tombs) > 2*s.limits.MaxTasks {
		copy(s.tombs, s.tombs[1:])
		s.tombs[len(s.tombs)-1] = tombstone{}
		s.tombs = s.tombs[:len(s.tombs)-1]
	}
}

// prune 职责：惰性清理完成任务与过期标记；使用：每个公开入口；边界：读取不会延长 TTL，活动任务由数量和字节预算约束。
func (s *Store) prune() {
	now := s.now()
	kept := s.tombs[:0]
	for _, item := range s.tombs {
		if now.Before(item.until) {
			kept = append(kept, item)
		}
	}
	clear(s.tombs[len(kept):])
	s.tombs = kept
	for id, entry := range s.entries {
		if entry.state != "running" && !now.Before(entry.finished.Add(s.limits.TTL)) {
			s.remove(id)
		}
	}
}

// Close 职责：释放所有保留数据并永久关闭实例；使用：应用退出；边界：幂等，旧 writer 始终被拒绝。
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, entry := range s.entries {
		entry.graph = nil
		entry.events = nil
		entry.bytes = 0
	}
	s.entries = nil
	s.order = nil
	s.tombs = nil
	s.bytes = 0
}
