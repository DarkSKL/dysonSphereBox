package workflow

// 文件职责：定义唯一可执行 YAML 语法和有限预算；使用：配置加载与编译；边界：不执行工具，不提供脚本或跳转。

// Limits 职责：约束解析、展开和执行资源；使用：零值取保守默认；边界：负值非法。
type Limits struct {
	// ConfigBytes 职责：配置字节上限；使用：加载前；边界：默认一 MiB。
	ConfigBytes int
	// Depth 职责：YAML 与流程嵌套上限；使用：加载及编译；边界：默认三十二。
	Depth int
	// Nodes 职责：展开节点上限；使用：编译；边界：默认四千零九十六。
	Nodes int
	// CallDepth 职责：子流程展开深度；使用：编译；边界：默认十六。
	CallDepth int
	// Candidates 职责：任务累计候选上限；使用：执行；边界：默认二百五十六。
	Candidates int
	// Parallel 职责：并发上限；使用：候选调度；边界：默认十。
	Parallel int
	// Steps 职责：累计工具调用预算；使用：执行；边界：默认六万五千五百三十六。
	Steps int
	// StateBytes 职责：所有活动候选状态保留字节上限；使用：克隆前与提交前；边界：默认二百五十六 MiB，依赖领域大小器。
	StateBytes int64
	// Regions 职责：一次遍历的区域上限；使用：动态列表；边界：默认二百五十六。
	Regions int
}

// limits 职责：归一化上限；使用：加载与运行；边界：显式负值拒绝。
func limits(l Limits) (Limits, error) {
	p := []*int{&l.ConfigBytes, &l.Depth, &l.Nodes, &l.CallDepth, &l.Candidates, &l.Parallel, &l.Steps, &l.Regions}
	defaults := []int{1 << 20, 32, 4096, 16, 256, 10, 65536, 256}
	for i, v := range p {
		if *v < 0 {
			return l, errConfig("negative limit")
		}
		if *v == 0 {
			*v = defaults[i]
		}
	}
	if l.StateBytes < 0 {
		return l, errConfig("negative byte limit")
	}
	if l.StateBytes == 0 {
		l.StateBytes = 256 << 20
	}
	return l, nil
}

// Document 职责：承载方案族 AST；使用：Load 后 Compile；边界：编译重新复制，不持有调用方可变配置。
type Document struct {
	// SchemaVersion 职责：固定语法版本；使用：加载；边界：仅支持一。
	SchemaVersion int `json:"schemaVersion"`
	// ID 职责：方案族标识；使用：诊断与指纹；边界：不能为空。
	ID string `json:"id"`
	// Parameters 职责：命名常量；使用：$param 引用；边界：不是隐式工具默认值。
	Parameters map[string]any `json:"parameters,omitempty"`
	// ToolDefaults 职责：方案级参数覆盖；使用：工具绑定；边界：复合字段整体覆盖。
	ToolDefaults map[string]map[string]any `json:"toolDefaults,omitempty"`
	// Flow 职责：唯一入口；使用：引擎；边界：必须非空。
	Flow []Node `json:"flow,omitempty"`
	// Subflows 职责：私有类型化流程；使用：call；边界：禁止递归。
	Subflows map[string]Subflow `json:"subflows,omitempty"`
}

// Parameter 职责：描述子流程或候选输入语义；使用：编译和运行；边界：仅支持 string/integer/number/boolean/object/array。
type Parameter struct {
	// Type 职责：输入类型；使用：显式校验；边界：不隐式转换。
	Type string `json:"type"`
	// Required 职责：区分缺失与零；使用：合并后校验；边界：false 不等于缺失。
	Required bool `json:"required,omitempty"`
	// Default 职责：缺失时的默认值；使用：调用绑定；边界：null 不合法。
	Default any `json:"default,omitempty"`
}

// Subflow 职责：定义私有流程及显式输入；使用：call 展开；边界：不捕获调用方 $input。
type Subflow struct {
	// Inputs 职责：参数契约；使用：调用点；边界：拒绝额外参数。
	Inputs map[string]Parameter `json:"inputs,omitempty"`
	// Flow 职责：流程体；使用：保留固定作用域；边界：受展开预算约束。
	Flow []Node `json:"flow"`
}

// Node 职责：表达一个互斥调度动作；使用：方案 flow；边界：不允许多种动作混用。
type Node struct {
	// ID 职责：稳定追踪节点；使用：错误路径；边界：同一列表内唯一且非空。
	ID string `json:"id"`
	// Tool 职责：业务工具 ID；使用：白名单注册；边界：不表示 Go 字段路径。
	Tool string `json:"tool,omitempty"`
	// With 职责：显式字段覆盖；使用：工具和 call；边界：其他节点不能使用。
	With map[string]any `json:"with,omitempty"`
	// Sequence 职责：嵌套顺序；使用：结构化分组；边界：有限列表。
	Sequence []Node `json:"sequence,omitempty"`
	// Switch 职责：唯一分支选择；使用：枚举或布尔值；边界：必须 otherwise。
	Switch *Switch `json:"switch,omitempty"`
	// Call 职责：私有子流程引用；使用：显式入参；边界：不能递归。
	Call string `json:"call,omitempty"`
	// Regions 职责：稳定区域遍历；使用：列表或显式提供器；边界：不得嵌套区域。
	Regions *Regions `json:"regions,omitempty"`
	// Candidates 职责：隔离有界选优；使用：同一候选流程；边界：嵌套共享全部任务预算。
	Candidates *Candidates `json:"candidates,omitempty"`
	// Fail 职责：显式失败分支；使用：otherwise；边界：配置失败不会触发候选回退。
	Fail string `json:"fail,omitempty"`
}

// Switch 职责：描述穷尽分支；使用：编译求共同产物；边界：不得省略 otherwise。
type Switch struct {
	// Value 职责：分支值表达式；使用：string 或 bool；边界：不执行表达式脚本。
	Value any `json:"value"`
	// Cases 职责：枚举分支；使用：字符串键；边界：空流程不产生新产物。
	Cases map[string][]Node `json:"cases"`
	// Otherwise 职责：未匹配分支；使用：显式失败或流程；边界：字段必填。
	Otherwise []Node `json:"otherwise"`
}

// Regions 职责：定义有限区域及流程；使用：引擎；边界：重复或空区域 ID 拒绝。
type Regions struct {
	// Reset 职责：允许显式安全重绑定区域集合；使用：新几何已使旧区域产物及验证失效后；边界：默认拒绝变化，存在有效旧产物或当前版本区域验证时仍拒绝。
	Reset bool `json:"reset,omitempty"`
	// Items 职责：区域列表；使用：[]string 或引用；边界：由内核检查数量。
	Items any `json:"items"`
	// Flow 职责：每个区域流程；使用：固定 Region；边界：不自动拼接领域产物。
	Flow []Node `json:"flow"`
}

// Candidates 职责：定义独立候选组；使用：强类型提供器动态枚举；边界：只有已声明无解允许备用。
type Candidates struct {
	// IncludeCurrent 职责：把已完整验证父结果作为先于新项的不可变基线；使用：可选精修；边界：仅根作用域 best/items，允许空 items，不派发虚构候选。
	IncludeCurrent bool `json:"includeCurrent,omitempty"`
	// Items 职责：参数对象数组；使用：静态或 $state 引用；边界：每次独立绑定。
	Items any `json:"items,omitempty"`
	// Groups 职责：描述有限失败依赖组表；使用：静态数组或显式状态提供器；边界：与 Items 互斥，开始前整体冻结校验。
	Groups any `json:"groups,omitempty"`
	// Inputs 职责：候选参数契约；使用：$input；边界：与子流程同样严格。
	Inputs map[string]Parameter `json:"inputs,omitempty"`
	// Flow 职责：每个候选完整流程；使用：隔离实例；边界：获选前必须验证。
	Flow []Node `json:"flow"`
	// Parallel 职责：请求并发；使用：有界调度；边界：不得超过任务上限。
	Parallel int `json:"parallel,omitempty"`
	// Scorer 职责：指定只读评分器；使用：RunOptions.Scorers；边界：空值使用默认 Score。
	Scorer string `json:"scorer,omitempty"`
	// Selection 职责：指定选优或有序首解；使用：best/first；边界：默认 best，first 只保证不启动后续波。
	Selection string `json:"selection,omitempty"`
	// Comparator 职责：指定双方只读比较器；使用：无法拆成向量的领域规则；边界：与 scorer 互斥。
	Comparator string `json:"comparator,omitempty"`
	// TimeoutMs 职责：限定本组优化时间；使用：局部截止；边界：父任务取消永远优先失败。
	TimeoutMs int `json:"timeoutMs,omitempty"`
	// ReserveMs 职责：在父截止前预留后续汇合验证与编码时间；使用：完整候选优化节点；边界：非负，缺少父截止时忽略，不创建或延长任务总时限。
	ReserveMs int `json:"reserveMs,omitempty"`
	// SettleMs 职责：首个验证成功后的优化宽限；使用：避免无收益搜索；边界：零表示无额外宽限截止。
	SettleMs int `json:"settleMs,omitempty"`
	// TimeoutFailure 职责：声明无成功候选时的截止无解码；使用：局部回退；边界：未声明则局部超时致命。
	TimeoutFailure string `json:"timeoutFailure,omitempty"`
	// AdvanceOn 职责：限制有序尝试的推进失败码；使用：first 且 parallel 为一；边界：空列表允许任意已声明无解，过滤读取完整错误树。
	AdvanceOn []string `json:"advanceOn,omitempty"`
	// Fallback 职责：全部无解后的备用流程；使用：父工作区；边界：任何致命错误均禁止回退。
	Fallback []Node `json:"fallback,omitempty"`
	// FallbackOn 职责：限制备用流程的触发失败码；使用：几何错误重排；边界：空列表沿用任意已声明无解，致命错误与父取消永不触发。
	FallbackOn []string `json:"fallbackOn,omitempty"`
}

// CandidateGroup 职责：运输有界候选参数及前序失败条件；使用：方案静态组表或强类型状态提供器；边界：Items 仅为配置参数，不是领域状态容器。
type CandidateGroup struct {
	// ID 职责：稳定组身份；使用：条件依赖和轨迹；边界：同表唯一且非空。
	ID string `json:"id"`
	// Items 职责：保存本组候选参数对象；使用：既有 flow/inputs；边界：允许空组表示全部剪枝，空组不构造真实失败。
	Items []map[string]any `json:"items"`
	// AfterNoSuccess 职责：列出允许触发本组的前组；使用：任一前组无成功且有非排除真实失败时启用；边界：只允许指向此前组。
	AfterNoSuccess []string `json:"afterNoSuccess,omitempty"`
	// ExcludeFailureCodes 职责：排除不能触发备用的真实失败码；使用：前组失败树过滤；边界：汇总包装及局部截止不冒充真实算法失败。
	ExcludeFailureCodes []string `json:"excludeFailureCodes,omitempty"`
}
