package workflowjournal

// 文件职责：提供独立于引擎序列化存储形式的前端传输类型。
// 使用：Wails自动生成正确的JSON对象参数和时间字符串；边界：不把json.RawMessage错误暴露为TypeScript number[]。

import (
	"bytes"
	"encoding/json"
	"time"
)

// PlanDescription 职责：描述一个冻结入口展开图；使用：跟踪服务；边界：每次返回独立可变副本，不共享计划内部引用。
type PlanDescription struct {
	// Tools 职责：按工具ID共享白名单参数规范与实现信息；使用：SchemaID引用；边界：不随每个展开节点复制大型规范。
	Tools []ToolDescription `json:"tools,omitempty"`
	// ID 职责：标识方案入口；使用：列表及根图；边界：不含运行身份。
	ID string `json:"id"`
	// Fingerprint 职责：标识所有冻结依赖；使用：运行图关联；边界：不随任务状态变化。
	Fingerprint string `json:"fingerprint"`
	// Nodes 职责：稳定展开节点列表；使用：父子及控制边重建；边界：工具及 call 均显式保留。
	Nodes []NodeDescription `json:"nodes"`
}

// ToolDescription 职责：描述静态工具的可公开契约；使用：图中SchemaID查询；边界：仅输出TraceFields允许字段，不暴露接收器或领域数据。
type ToolDescription struct {
	// SchemaTruncated 职责：标记规范超过单条输出预算；使用：明确静态说明缺口；边界：工具身份仍保留，不能产生悬空SchemaID。
	SchemaTruncated bool `json:"schemaTruncated,omitempty"`
	// ID 职责：稳定工具ID及SchemaID；使用：去重关联；边界：工具契约版本显式存在于ID。
	ID string `json:"id"`
	// Implementation 职责：构建实现身份；使用：复现；边界：不含代码地址。
	Implementation string `json:"implementation,omitempty"`
	// TraceFields 职责：公开参数白名单；使用：规范解释；边界：复制独立列表。
	TraceFields []string `json:"traceFields,omitempty"`
	// ParameterSchema 职责：白名单参数结构；使用：所有引用节点共享；边界：16KiB规范输出上限。
	ParameterSchema map[string]any `json:"parameterSchema,omitempty"`
}

// NodeDescription 职责：描述编译节点及结构关系；使用：工作流可视化；边界：参数只输出显式允许的字段，所有值均独立序列化。
type NodeDescription struct {
	// Source 职责：保留可读文档或子流程归属；使用：短定义ID定位；边界：不重复整个展开调用栈。
	Source string `json:"source"`
	// SchemaID 职责：引用共享工具规范ID；使用：PlanDescription.Tools；边界：非工具节点为空。
	SchemaID string `json:"schemaID,omitempty"`
	// Control 职责：描述显式候选预算/选择或区域reset；使用：解释控制结构；边界：不包含候选参数列表或动态状态值。
	Control map[string]any `json:"control,omitempty"`
	// Path 职责：稳定编译路径；使用：DefinitionPath关联；边界：候选与区域运行路径另行记录。
	Path string `json:"path"`
	// ParentPath 职责：父编译节点；使用：图层级；边界：入口子节点为空。
	ParentPath string `json:"parentPath,omitempty"`
	// ID 职责：方案原始节点ID；使用：展示；边界：仅列表内唯一。
	ID string `json:"id"`
	// Type 职责：动作类别；使用：控制形状；边界：不猜测工具内部业务。
	Type string `json:"type"`
	// Relation 职责：sequence/call/candidate/region/case/otherwise/fallback关系；使用：重建控制边；边界：分支标签保持明确。
	Relation string `json:"relation"`
	// Tool 职责：稳定工具ID；使用：工具节点；边界：不暴露接收器。
	Tool string `json:"tool,omitempty"`
	// Call 职责：明确子流程引用；使用：call节点；边界：展开体另列节点。
	Call string `json:"call,omitempty"`
	// Children 职责：稳定直接子节点路径；使用：图边；边界：保留原列表顺序。
	Children []string `json:"children,omitempty"`
	// Parameters 职责：白名单有效参数模板；使用：定义解释；边界：动态引用保留文本且16KiB封顶。
	Parameters map[string]any `json:"parameters,omitempty"`
	// ParameterSchema 职责：白名单真实参数结构；使用：编辑器说明；边界：不输出未授权字段定义。
	ParameterSchema map[string]any `json:"parameterSchema,omitempty"`
	// ParameterSources 职责：白名单参数覆盖来源；使用：解释默认优先级；边界：不输出隐藏参数名称。
	ParameterSources map[string]string `json:"parameterSources,omitempty"`
	// ParametersTruncated 职责：标识模板超出输出预算；使用：图缺口展示；边界：不截断非法JSON。
	ParametersTruncated bool `json:"parametersTruncated,omitempty"`
}

// ExecutionEvent 职责：记录真实节点及控制作用域的增量活动；使用：ObserveExecution；边界：纯 JSON 深复制 DTO，观察失败不改变业务。
type ExecutionEvent struct {
	// Sequence 职责：单 Run 串行发射序号；使用：journal 排序；边界：包含观察失败造成的缺口。
	Sequence uint64 `json:"sequence"`
	// Timestamp 职责：记录发射时刻；使用：持续时间展示；边界：序号才是可靠顺序。
	Timestamp string `json:"timestamp"`
	// Fingerprint 职责：关联冻结定义；使用：图关联；边界：不包含运行状态。
	Fingerprint string `json:"fingerprint"`
	// InstanceID 职责：关联唯一任务实例；使用：候选隔离解释；边界：不授予访问实例权限。
	InstanceID string `json:"instanceID"`
	// Path 职责：实际动态作用域路径；使用：区分候选/区域；边界：不能作为工具调用入口。
	Path string `json:"path"`
	// DefinitionPath 职责：关联 PlanDescription 节点；使用：动态活动投影；边界：控制活动可关联其父定义。
	DefinitionPath string `json:"definitionPath"`
	// ParentPath 职责：实际运行父作用域；使用：重建控制树；边界：根事件为空。
	ParentPath string `json:"parentPath,omitempty"`
	// Type 职责：区分 run/tool/sequence/call/switch/candidates/regions/candidate/region/group；使用：渲染节点；边界：不表达业务完成百分比。
	Type string `json:"type"`
	// Status 职责：区分 started/completed/failed/selected/skipped；使用：增量状态；边界：候选成功不等于主任务成功。
	Status string `json:"status"`
	// Tool 职责：稳定业务工具 ID；使用：定位；边界：控制节点可为空。
	Tool string `json:"tool,omitempty"`
	// Candidate 职责：固定候选序号；使用：运行身份；边界：根作用域为负一。
	Candidate int `json:"candidate"`
	// Region 职责：固定区域 ID；使用：区域汇合；边界：根作用域为空。
	Region string `json:"region,omitempty"`
	// RevisionBefore 职责：活动开始内容版本；使用：事务解释；边界：不包含领域值。
	RevisionBefore uint64 `json:"revisionBefore"`
	// RevisionAfter 职责：活动结束内容版本；使用：提交证据；边界：诊断更新不推进版本。
	RevisionAfter uint64 `json:"revisionAfter"`
	// EffectiveParameters 职责：只含工具白名单顶层策略参数；使用：解释真实调用；边界：最多16KiB，超限省略。
	EffectiveParameters map[string]any `json:"effectiveParameters,omitempty"`
	// ParameterSources 职责：白名单参数的覆盖来源；使用：解释优先级；边界：不输出其他字段名。
	ParameterSources map[string]string `json:"parameterSources,omitempty"`
	// DurationMs 职责：活动墙钟时长；使用：性能观察；边界：候选并行耗时不可相加为总时长。
	DurationMs float64 `json:"durationMs"`
	// Selection 职责：记录分支、区域或组状态选择；使用：控制活动；边界：不包含实体数据。
	Selection string `json:"selection,omitempty"`
	// Failure 职责：有界错误文本；使用：诊断；边界：不附 panic 栈或内部对象。
	Failure string `json:"failure,omitempty"`
	// FailureCode 职责：失败分类；使用：取消/领域/内核解释；边界：不改变实际恢复决策。
	FailureCode string `json:"failureCode,omitempty"`
	// ParametersTruncated 职责：表示参数超限未输出；使用：UI 明确缺口；边界：不截断为非法 JSON。
	ParametersTruncated bool `json:"parametersTruncated,omitempty"`
	// EventsLost 职责：此前观察错误或 panic 数量；使用：任务终态展示；边界：最终回调自身失败由 RunResult 兜底。
	EventsLost uint64 `json:"eventsLost,omitempty"`
	// Diagnostics 职责：有界记录工具显式实际参数与规则适用性；使用：终态解释；边界：最多64项及16KiB，工具不能放入领域实体。
	Diagnostics []string `json:"diagnostics,omitempty"`
	// DiagnosticsTruncated 职责：表示诊断达到输出上限；使用：明确展示缺口；边界：不改变实际规则结果。
	DiagnosticsTruncated bool `json:"diagnosticsTruncated,omitempty"`
}

// decodeWireJSON 职责：从封闭日志字节创建独立前端副本；使用：图与事件读取；边界：对象内数字保留原精确表示，不经float64中转。
func decodeWireJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}

// wireTime 职责：把内部时间转成明确的RFC3339Nano字段；使用：Wails DTO；边界：未完成时间为空字符串，不暴露time.Time内部表示。
func wireTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
