package workflowjournal

// 文件职责：定义有界工作流日志的公开传输契约；使用：Wails 查询和生成任务接线；边界：仅含引擎明确事件，不携带实体或工作区。

// Event 职责：给真实引擎事件附加日志游标；使用：分页恢复；边界：Sequence 是日志接收序号，内部 Event.Sequence 保留引擎序号。
type Event struct {
	// TaskID 职责：关联服务任务；使用：过滤；边界：不作为权限凭证。
	TaskID string `json:"taskId"`
	// Sequence 职责：提供严格递增游标；使用：Read 的 after；边界：不复用引擎并发发送序号。
	Sequence uint64 `json:"sequence"`
	// Event 职责：保存真实引擎事件；使用：展示节点及动态路径；边界：参数经过白名单和大小限制。
	Event ExecutionEvent `json:"event"`
}

// Snapshot 职责：返回任务状态及当时冻结的图；使用：任务恢复；边界：不把当前配置图冒充历史任务图。
type Snapshot struct {
	// Code 职责：区分 ok/not_found/expired/closed/invalid_argument；使用：查询分派；边界：不是生成结果码。
	Code string `json:"code"`
	// Message 职责：说明查询结果；使用：诊断；边界：文本有界。
	Message string `json:"message"`
	// TaskID 职责：回显任务身份；使用：路由；边界：最长一百二十八字节。
	TaskID string `json:"taskId"`
	// State 职责：记录 running/succeeded/failed/canceled/failed-before-start；使用：轮询终止；边界：提前失败无图，正常终态由任务所有者 Finish 确认。
	State string `json:"state"`
	// Fingerprint 职责：绑定冻结方案及配置；使用：历史核对；边界：不依赖查询时配置。
	Fingerprint string `json:"fingerprint"`
	// TrackingAvailable 职责：说明该任务是否成功保存冻结图和增量轨迹；使用：容量降级提示；边界：业务成功也可能为 false，不能宣称记录完整。
	TrackingAvailable bool `json:"trackingAvailable"`
	// TrackingErrorCode 职责：记录旁路不可用原因；使用：容量或关闭降级诊断；边界：独立于最终业务错误。
	TrackingErrorCode string `json:"trackingErrorCode,omitempty"`
	// TrackingErrorMessage 职责：说明旁路不可用原因；使用：诊断；边界：最长二千零四十八字节，不含实体。
	TrackingErrorMessage string `json:"trackingErrorMessage,omitempty"`
	// StartedAt 职责：记录登记时间；使用：时序；边界：UTC。
	StartedAt string `json:"startedAt"`
	// FinishedAt 职责：记录终止时间；使用：TTL；边界：运行中为零值。
	FinishedAt string `json:"finishedAt"`
	// FirstSequence 职责：声明最早可读序号；使用：缺口恢复；边界：无事件时为 LastSequence 加一。
	FirstSequence uint64 `json:"firstSequence"`
	// LastSequence 职责：声明已接收序号；使用：分页上界；边界：含已淘汰事件。
	LastSequence uint64 `json:"lastSequence"`
	// DroppedEvents 职责：计量容量淘汰；使用：不完整提示；边界：不是引擎执行失败数。
	DroppedEvents uint64 `json:"droppedEvents"`
	// LostEvents 职责：计量校验或大小拒绝；使用：日志完整性判断；边界：不改变业务结果。
	LostEvents uint64 `json:"lostEvents"`
	// EngineEventsLost 职责：保留引擎最终观察丢失数；使用：含最终回调失败的完整性判断；边界：可与 LostEvents 重叠，不应直接相加。
	EngineEventsLost uint64 `json:"engineEventsLost"`
	// EventCount 职责：返回现存事件数；使用：容量诊断；边界：非总执行数。
	EventCount int `json:"eventCount"`
	// RetainedBytes 职责：统计图和事件 JSON 字节；使用：预算诊断；边界：Go 固定元数据另由任务和事件数界定。
	RetainedBytes int `json:"retainedBytes"`
	// ErrorCode 职责：保存最终业务错误码；使用：失败或取消恢复；边界：最长一百二十八字节。
	ErrorCode string `json:"errorCode,omitempty"`
	// ErrorMessage 职责：保存最终简短原因；使用：失败解释；边界：最长二千零四十八字节。
	ErrorMessage string `json:"errorMessage,omitempty"`
	// Graph 职责：保存任务实际冻结图；使用：动态 DefinitionPath 对照；边界：深复制，不返回内部计划。
	Graph *PlanDescription `json:"graph,omitempty"`
}

// Page 职责：返回 after 之后的有界事件页；使用：执行中轮询；边界：cursor_gap 仍返回保留数据，after 越界直接拒绝。
type Page struct {
	// Code 职责：返回 ok/cursor_gap/not_found/expired/invalid_cursor/invalid_argument/closed；使用：恢复分支；边界：cursor_gap 不代表完整记录。
	Code string `json:"code"`
	// Message 职责：解释分页状态；使用：诊断；边界：不承载引擎错误堆栈。
	Message string `json:"message"`
	// TaskID 职责：回显任务；使用：过滤；边界：不跨任务合并。
	TaskID string `json:"taskId"`
	// Events 职责：保存深复制事件；使用：增量显示；边界：最多 MaxPageSize 条。
	Events []Event `json:"events"`
	// Next 职责：标记已返回的最后游标；使用：下次 after；边界：无新事件时保持原游标。
	Next uint64 `json:"next"`
	// FirstSequence 职责：给出最早可读序号；使用：缺口提示；边界：淘汰后可大于一。
	FirstSequence uint64 `json:"firstSequence"`
	// LastSequence 职责：声明查询瞬间上界；使用：分页完成判断；边界：运行中可能继续增长。
	LastSequence uint64 `json:"lastSequence"`
	// Gap 职责：显式声明旧游标丢失前缀；使用：界面提示；边界：不得补造事件。
	Gap bool `json:"gap"`
	// Complete 职责：声明任务已结束且该页已读到末尾；使用：停止轮询；边界：不表示任务成功或记录无缺口。
	Complete bool `json:"complete"`
}

// Description 职责：承载当前冻结配置图查询；使用：服务 DescribeWorkflow；边界：并非某个已运行任务图。
type Description struct {
	// Code 职责：传递 ok 或配置错误；使用：查询分派；边界：不启动引擎。
	Code string `json:"code"`
	// Message 职责：保存有界说明；使用：诊断；边界：不包含完整配置文件。
	Message string `json:"message"`
	// Graph 职责：返回当前可执行图；使用：新任务预览；边界：历史绑定应查询 Snapshot。
	Graph *PlanDescription `json:"graph,omitempty"`
}
