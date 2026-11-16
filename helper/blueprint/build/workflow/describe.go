package workflow

// 文件职责：导出冻结计划的纯值结构图与有界参数定义；使用：后台方案查询；边界：不导出反射方法、领域状态或可变编译 AST。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	ParameterSchema json.RawMessage `json:"parameterSchema,omitempty"`
}

// NodeDescription 职责：描述编译节点及结构关系；使用：工作流可视化；边界：参数只输出显式允许的字段，所有值均独立序列化。
type NodeDescription struct {
	// Source 职责：保留可读文档或子流程归属；使用：短定义ID定位；边界：不重复整个展开调用栈。
	Source string `json:"source"`
	// SchemaID 职责：引用共享工具规范ID；使用：PlanDescription.Tools；边界：非工具节点为空。
	SchemaID string `json:"schemaID,omitempty"`
	// Control 职责：描述显式候选预算/选择或区域reset；使用：解释控制结构；边界：不包含候选参数列表或动态状态值。
	Control json.RawMessage `json:"control,omitempty"`
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
	Parameters json.RawMessage `json:"parameters,omitempty"`
	// ParameterSchema 职责：白名单真实参数结构；使用：编辑器说明；边界：不输出未授权字段定义。
	ParameterSchema json.RawMessage `json:"parameterSchema,omitempty"`
	// ParameterSources 职责：白名单参数覆盖来源；使用：解释默认优先级；边界：不输出隐藏参数名称。
	ParameterSources map[string]string `json:"parameterSources,omitempty"`
	// ParametersTruncated 职责：标识模板超出输出预算；使用：图缺口展示；边界：不截断非法JSON。
	ParametersTruncated bool `json:"parametersTruncated,omitempty"`
}

// nodeType 职责：映射唯一动作名称；使用：图和运行活动统一；边界：不触发求值。
func nodeType(n *compiledNode) string {
	switch {
	case n.tool != nil:
		return "tool"
	case n.ast.Sequence != nil:
		return "sequence"
	case n.ast.Switch != nil:
		return "switch"
	case n.ast.Call != "":
		return "call"
	case n.ast.Regions != nil:
		return "regions"
	case n.ast.Candidates != nil:
		return "candidates"
	case n.ast.Fail != "":
		return "fail"
	}
	return "unknown"
}

// traceParameters 职责：复制白名单顶层参数；使用：定义和事件；边界：不序列化未授权字段，超出16KiB省略全部值。
func traceParameters(t *tool, value any, sources map[string]string) (json.RawMessage, map[string]string, bool) {
	if t == nil || len(t.spec.TraceFields) == 0 {
		return nil, nil, false
	}
	var m map[string]any
	switch x := value.(type) {
	case map[string]any:
		m = x
	case json.RawMessage:
		decoder := json.NewDecoder(bytes.NewReader(x))
		decoder.UseNumber()
		_ = decoder.Decode(&m)
	}
	selected := map[string]any{}
	origin := map[string]string{}
	for _, name := range t.spec.TraceFields {
		if v, ok := m[name]; ok {
			selected[name] = v
			if source, ok := sources[name]; ok {
				origin[name] = source
			}
		}
	}
	data, err := json.Marshal(selected)
	if err != nil || len(data) > 16<<10 {
		return nil, origin, true
	}
	return data, origin, false
}

// Describe 职责：返回深复制的完整紧凑展开图；使用：后台方案定义接口；边界：Path/ParentPath/Children为短定义ID，真实长运行路径只出现在事件，不能截断或丢节点。
func (p *Plan) Describe() PlanDescription {
	description := PlanDescription{ID: p.id, Fingerprint: p.fingerprint, Nodes: []NodeDescription{}}
	used := map[string]bool{}
	var visit func([]*compiledNode, string, string)
	visit = func(nodes []*compiledNode, parent, relation string) {
		for _, n := range nodes {
			node := NodeDescription{Path: n.definitionID, ParentPath: parent, ID: n.ast.ID, Type: nodeType(n), Relation: relation, Tool: n.ast.Tool, Call: n.ast.Call, Source: definitionSource(n.path)}
			if c := n.ast.Candidates; c != nil {
				node.Control, _ = json.Marshal(map[string]any{"selection": c.Selection, "includeCurrent": c.IncludeCurrent, "parallel": c.Parallel, "scorer": c.Scorer, "comparator": c.Comparator, "timeoutMs": c.TimeoutMs, "settleMs": c.SettleMs, "reserveMs": c.ReserveMs, "groups": c.Groups != nil})
			}
			if c := n.ast.Regions; c != nil {
				node.Control, _ = json.Marshal(map[string]any{"reset": c.Reset})
			}
			if n.tool == nil && len(n.inputs) > 0 {
				properties := map[string]any{}
				required := []string{}
				for name, rule := range n.inputs {
					properties[name] = map[string]any{"type": rule.Type}
					if rule.Required {
						required = append(required, name)
					}
				}
				sort.Strings(required)
				node.ParameterSchema, _ = json.Marshal(map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
			}
			node.Parameters, node.ParameterSources, node.ParametersTruncated = traceParameters(n.tool, n.args, n.sources)
			if n.tool != nil {
				node.SchemaID = n.tool.spec.ID
				if !used[n.tool.spec.ID] {
					used[n.tool.spec.ID] = true
					properties := typeSchema(n.tool.args)["properties"].(map[string]any)
					filtered := map[string]any{}
					for _, name := range n.tool.spec.TraceFields {
						filtered[name] = properties[name]
					}
					schema, _ := json.Marshal(map[string]any{"type": "object", "properties": filtered})
					tool := ToolDescription{ID: n.tool.spec.ID, Implementation: n.tool.spec.Implementation, TraceFields: append([]string(nil), n.tool.spec.TraceFields...)}
					if len(schema) <= 16<<10 {
						tool.ParameterSchema = schema
					} else {
						tool.SchemaTruncated = true
					}
					description.Tools = append(description.Tools, tool)
				}
			}
			for _, child := range n.children {
				node.Children = append(node.Children, child.definitionID)
			}
			labels := make([]string, 0, len(n.cases))
			for name := range n.cases {
				labels = append(labels, name)
			}
			sort.Strings(labels)
			for _, label := range labels {
				for _, child := range n.cases[label] {
					node.Children = append(node.Children, child.definitionID)
				}
			}
			for _, child := range n.other {
				node.Children = append(node.Children, child.definitionID)
			}
			description.Nodes = append(description.Nodes, node)
			childRelation := node.Type
			if childRelation == "candidates" {
				childRelation = "candidate"
			}
			if childRelation == "regions" {
				childRelation = "region"
			}
			visit(n.children, n.definitionID, childRelation)
			for _, label := range labels {
				visit(n.cases[label], n.definitionID, "case:"+label)
			}
			other := "fallback"
			if n.ast.Switch != nil {
				other = "otherwise"
			}
			visit(n.other, n.definitionID, other)
		}
	}
	visit(p.flow, "", "entry")
	sort.Slice(description.Tools, func(a, b int) bool { return description.Tools[a].ID < description.Tools[b].ID })
	return description
}

// definitionSource 职责：提取最邻近显式库调用或入口文档；使用：短ID旁的人类可读定位；边界：不保存完整长路径。
func definitionSource(path string) string {
	if index := strings.LastIndex(path, "/call:"); index >= 0 {
		parts := strings.Split(path[index+6:], "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return strings.SplitN(path, "/", 2)[0]
}

// assignDefinitionIDs 职责：为全部展开节点缓存短稳定ID并检测碰撞；使用：编译结束；边界：事件与图使用同一缓存ID，不省略任何调用实例。
func assignDefinitionIDs(nodes []*compiledNode) error {
	seen := map[string]string{}
	var visit func([]*compiledNode) error
	visit = func(nodes []*compiledNode) error {
		for _, n := range nodes {
			sum := sha256.Sum256([]byte(n.path))
			id := "n" + hex.EncodeToString(sum[:10])
			if previous, ok := seen[id]; ok {
				return fmt.Errorf("duplicate definition identity %s (%s / %s)", id, previous, n.path)
			}
			seen[id] = n.path
			n.definitionID = id
			if e := visit(n.children); e != nil {
				return e
			}
			labels := make([]string, 0, len(n.cases))
			for label := range n.cases {
				labels = append(labels, label)
			}
			sort.Strings(labels)
			for _, label := range labels {
				if e := visit(n.cases[label]); e != nil {
					return e
				}
			}
			if e := visit(n.other); e != nil {
				return e
			}
		}
		return nil
	}
	return visit(nodes)
}
