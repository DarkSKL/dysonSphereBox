package workflow

// 文件职责：展开显式流程库并静态校验类型和共同产物；使用：任务创建前；边界：不求值动态实例属性，执行时再次核对。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CompileOptions 职责：提供可信初始产物和显式库；使用：Compile；边界：不会隐式搜索或热加载其他文件。
type CompileOptions struct {
	// Initial 职责：声明入口已有产物；使用：静态依赖检查；边界：运行仍须实际播种。
	Initial []Artifact
	// Libraries 职责：冻结共享方案库；使用：call familyID/subflowID；边界：ID 唯一、跨库递归同样拒绝。
	Libraries []*Document
	// Limits 职责：设置编译和默认运行预算；使用：整个计划；边界：零值取默认。
	Limits Limits
}

// Plan 职责：保存冻结可执行计划；使用：Run；边界：不导出可变 AST 或注册目录。
type Plan struct {
	registry    *Registry
	flow        []*compiledNode
	limits      Limits
	fingerprint string
	id          string
	initial     []Artifact
}

// Fingerprint 职责：返回全依赖身份；使用：追踪；边界：包含冻结库、工具契约和实现身份。
func (p *Plan) Fingerprint() string { return p.fingerprint }

// compiledNode 职责：缓存节点绑定和静态参数模板；使用：引擎；边界：执行过程不修改此结构。
type compiledNode struct {
	definitionID string
	ast          Node
	path         string
	tool         *tool
	args         map[string]any
	sources      map[string]string
	children     []*compiledNode
	cases        map[string][]*compiledNode
	other        []*compiledNode
	inputs       map[string]Parameter
	with         map[string]any
	items        any
	groups       any
	scorer       string
}

// compileState 职责：记录每条路径必然有效的产物及依赖；使用：分支汇合；边界：不可达失败路径不贡献产物。
type compileState struct {
	available map[*artifactDef]bool
	deps      map[*artifactDef]map[*artifactDef]bool
	validated bool
	live      bool
	evidence  map[*artifactDef]bool
}

// copy 职责：分离路径分析状态；使用：分支；边界：依赖集合仅由本次编译使用。
func (s compileState) copy() compileState {
	a := map[*artifactDef]bool{}
	d := map[*artifactDef]map[*artifactDef]bool{}
	for k, v := range s.available {
		a[k] = v
	}
	for k, v := range s.deps {
		x := map[*artifactDef]bool{}
		for dep, b := range v {
			x[dep] = b
		}
		d[k] = x
	}
	evidence := map[*artifactDef]bool{}
	for key, value := range s.evidence {
		evidence[key] = value
	}
	return compileState{available: a, deps: d, validated: s.validated, live: s.live, evidence: evidence}
}

// compiler 职责：维护有限展开上下文；使用：Compile；边界：不保存实例状态。
type compiler struct {
	plan  *Plan
	docs  map[string]*Document
	nodes int
	calls map[string]bool
}

// Compile 职责：构建可执行强契约计划；使用：Load 之后；边界：错误配置不创建领域实体。
func Compile(doc *Document, r *Registry, options CompileOptions) (*Plan, error) {
	if doc == nil || r == nil {
		return nil, errConfig("document and registry required")
	}
	l, e := limits(options.Limits)
	if e != nil {
		return nil, e
	}
	p := &Plan{registry: r, limits: l, id: doc.ID, initial: append([]Artifact(nil), options.Initial...)}
	c := &compiler{plan: p, docs: map[string]*Document{}, calls: map[string]bool{}}
	var frozen [][]byte
	totalBytes := 0
	for _, source := range append([]*Document{doc}, options.Libraries...) {
		if source == nil {
			return nil, errConfig("nil library")
		}
		b, e := json.Marshal(source)
		if e != nil {
			return nil, e
		}
		if len(b) > l.ConfigBytes {
			return nil, errConfig("configuration byte budget exceeded")
		}
		totalBytes += len(b)
		if totalBytes > l.ConfigBytes {
			return nil, errConfig("complete library byte budget exceeded")
		}
		var copy Document
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.UseNumber()
		if e = d.Decode(&copy); e != nil {
			return nil, e
		}
		value, e := transport(&copy)
		if e != nil {
			return nil, e
		}
		if e = r.documentSchema.Validate(value); e != nil {
			return nil, fmt.Errorf("document %s schema: %w", copy.ID, e)
		}
		if copy.SchemaVersion != 1 || copy.ID == "" || c.docs[copy.ID] != nil {
			return nil, errConfig("invalid or duplicate document id")
		}
		c.docs[copy.ID] = &copy
		frozen = append(frozen, b)
	}
	doc = c.docs[doc.ID]
	if len(doc.Flow) == 0 {
		return nil, errConfig("entry flow required")
	}
	state := compileState{available: map[*artifactDef]bool{}, deps: map[*artifactDef]map[*artifactDef]bool{}, live: true}
	for _, a := range options.Initial {
		d, e := artifactDefinition(a)
		if e != nil {
			return nil, e
		}
		state.available[d] = true
	}
	// 所有声明默认值都单独检查，避免被节点覆盖后隐藏非法配置。
	for _, owner := range c.docs {
		for id, m := range owner.ToolDefaults {
			t := r.tools[id]
			if t == nil {
				return nil, errConfig("unknown toolDefaults " + id)
			}
			for name, value := range m {
				field, ok := argField(t.args, name)
				if !ok {
					return nil, errConfig("toolDefaults unknown field " + name)
				}
				bound, dynamic, e := c.template(value, field.Type, nil, owner, t)
				if e != nil {
					return nil, e
				}
				if !dynamic {
					if e = t.checkPartial(map[string]any{name: bound}, "toolDefaults"); e != nil {
						return nil, e
					}
				}
			}
		}
		for _, sub := range owner.Subflows {
			if e = checkRules(sub.Inputs); e != nil {
				return nil, e
			}
		}
	}
	if e = c.validateDeclarations(); e != nil {
		return nil, e
	}
	p.flow, _, e = c.flow(doc.Flow, doc, nil, state, doc.ID, 0, 0, false)
	if e != nil {
		return nil, e
	}
	if e = assignDefinitionIDs(p.flow); e != nil {
		return nil, e
	}
	sort.Slice(frozen, func(i, j int) bool { return string(frozen[i]) < string(frozen[j]) })
	b, _ := json.Marshal([]any{frozen, r.fingerprint, artifactNames(options.Initial), l})
	h := sha256.Sum256(b)
	p.fingerprint = hex.EncodeToString(h[:])
	return p, nil
}

// checkRules 职责：检查流程输入契约；使用：编译；边界：默认值必须满足声明类型。
func checkRules(rules map[string]Parameter) error {
	for name, r := range rules {
		if name == "" {
			return errConfig("empty input name")
		}
		switch r.Type {
		case "string", "boolean", "integer", "number", "object", "array":
		default:
			return errConfig("unknown input type " + r.Type)
		}
		if r.Default != nil {
			if e := checkParameter(r.Default, r.Type); e != nil {
				return fmt.Errorf("input %s default: %w", name, e)
			}
		}
	}
	return nil
}

// flow 职责：逐条展开并传递依赖状态；使用：编译递归；边界：节点与深度预算覆盖所有分支。
func (c *compiler) flow(nodes []Node, doc *Document, inputs map[string]Parameter, state compileState, path string, depth, calls int, inRegion bool) ([]*compiledNode, compileState, error) {
	if depth > c.plan.limits.Depth || calls > c.plan.limits.CallDepth {
		return nil, state, errConfig(path + ": expansion depth budget exceeded")
	}
	out := make([]*compiledNode, 0, len(nodes))
	ids := map[string]bool{}
	for _, n := range nodes {
		if n.ID == "" || ids[n.ID] {
			return nil, state, errConfig(path + ": empty or duplicate node id")
		}
		ids[n.ID] = true
		c.nodes++
		if c.nodes > c.plan.limits.Nodes {
			return nil, state, errConfig("expanded node budget exceeded")
		}
		node, next, e := c.node(n, doc, inputs, state, path+"/"+n.ID, depth, calls, inRegion)
		if e != nil {
			return nil, state, e
		}
		out = append(out, node)
		state = next
	}
	return out, state, nil
}

// node 职责：校验单个互斥动作；使用：flow；边界：引擎负责实际动态分支和候选调度。
func (c *compiler) node(n Node, doc *Document, inputs map[string]Parameter, s compileState, path string, depth, calls int, inRegion bool) (*compiledNode, compileState, error) {
	cn := &compiledNode{ast: n, path: path}
	fail := func(e error) (*compiledNode, compileState, error) { return nil, s, fmt.Errorf("%s: %w", path, e) }
	count := 0
	for _, b := range []bool{n.Tool != "", n.Sequence != nil, n.Switch != nil, n.Call != "", n.Regions != nil, n.Candidates != nil, n.Fail != ""} {
		if b {
			count++
		}
	}
	if count != 1 {
		return fail(errConfig("exactly one node action required"))
	}
	if n.With != nil && n.Tool == "" && n.Call == "" {
		return fail(errConfig("with is only valid for tool/call"))
	}
	var expressions []any
	if n.With != nil {
		expressions = append(expressions, n.With)
	}
	if n.Tool != "" {
		expressions = append(expressions, doc.ToolDefaults[n.Tool])
	}
	if n.Switch != nil {
		expressions = append(expressions, n.Switch.Value)
	}
	if n.Regions != nil {
		expressions = append(expressions, n.Regions.Items)
	}
	if n.Candidates != nil {
		expressions = append(expressions, n.Candidates.Items)
		expressions = append(expressions, n.Candidates.Groups)
	}
	for _, value := range expressions {
		if e := c.checkStateReferences(value, s); e != nil {
			return fail(e)
		}
	}
	switch {
	case n.Tool != "":
		t := c.plan.registry.tools[n.Tool]
		if t == nil {
			return fail(errConfig("unknown tool " + n.Tool))
		}
		cn.tool = t
		for _, a := range t.spec.Requires {
			if !s.available[a.artifact()] && s.live {
				return fail(errConfig("missing required artifact " + a.artifact().name))
			}
		}
		if !inRegion {
			for _, a := range t.spec.Reads {
				if a.artifact().options.Regional {
					return fail(errConfig("root reads require AllRegions capability"))
				}
			}
			for _, a := range t.spec.Writes {
				if a.artifact().options.Regional {
					return fail(errConfig("root cannot write regional artifact"))
				}
			}
		} else if len(t.spec.AllRegions) > 0 {
			return fail(errConfig("region tool cannot request AllRegions"))
		}
		if t.spec.Role == Encode && s.live && !s.validated {
			return fail(errConfig("encoding path is not validated"))
		}
		if t.spec.Role == Encode && inRegion {
			return fail(errConfig("encoding requires root scope"))
		}
		cn.sources = map[string]string{}
		for name := range t.spec.Defaults {
			cn.sources[name] = "defaults"
		}
		for name := range doc.ToolDefaults[n.Tool] {
			cn.sources[name] = "toolDefaults"
		}
		for name := range n.With {
			cn.sources[name] = "with"
		}
		merged := merge(t.spec.Defaults, doc.ToolDefaults[n.Tool], n.With)
		for name, value := range merged {
			kind, target, is, _ := reference(value)
			if is {
				cn.sources[name] += (":" + kind + ":" + target)
			}
		}
		bound, dynamic, e := c.template(merged, t.args, inputs, doc, t)
		if e != nil {
			return fail(e)
		}
		cn.args = bound.(map[string]any)
		for _, name := range t.spec.Required {
			if _, ok := cn.args[name]; !ok {
				return fail(errConfig("required argument " + name))
			}
		}
		if !dynamic {
			if _, e = t.bind(cn.args, path); e != nil {
				return fail(e)
			}
		}
		s = applyContract(s, t.spec)
	case n.Sequence != nil:
		var e error
		cn.children, s, e = c.flow(n.Sequence, doc, inputs, s, path, depth+1, calls, inRegion)
		if e != nil {
			return fail(e)
		}
	case n.Fail != "":
		s.live = false
	case n.Switch != nil:
		sw := n.Switch
		if sw.Otherwise == nil || len(sw.Cases) == 0 {
			return fail(errConfig("switch requires cases and otherwise"))
		}
		v, typ, e := c.expression(sw.Value, inputs, doc, nil)
		if e != nil {
			return fail(e)
		}
		if typ != nil && typ.Kind() != reflect.String && typ.Kind() != reflect.Bool {
			return fail(errConfig("switch value must be string or boolean"))
		}
		if typ == nil {
			switch v.(type) {
			case string, bool:
			default:
				return fail(errConfig("switch value must be string or boolean"))
			}
		}
		cn.items = v
		cn.cases = map[string][]*compiledNode{}
		var branches []compileState
		names := make([]string, 0, len(sw.Cases))
		for name := range sw.Cases {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			flow, next, e := c.flow(sw.Cases[name], doc, inputs, s.copy(), path+"/case:"+name, depth+1, calls, inRegion)
			if e != nil {
				return fail(e)
			}
			cn.cases[name] = flow
			branches = append(branches, next)
		}
		cn.other, s, e = c.flow(sw.Otherwise, doc, inputs, s.copy(), path+"/otherwise", depth+1, calls, inRegion)
		if e != nil {
			return fail(e)
		}
		branches = append(branches, s)
		s = commonState(branches)
	case n.Call != "":
		owner := doc
		id := n.Call
		if parts := strings.SplitN(id, "/", 2); len(parts) == 2 {
			owner = c.docs[parts[0]]
			id = parts[1]
		}
		if owner == nil {
			return fail(errConfig("unknown flow library"))
		}
		sub, ok := owner.Subflows[id]
		if !ok {
			return fail(errConfig("unknown subflow " + n.Call))
		}
		full := owner.ID + "/" + id
		if c.calls[full] {
			return fail(errConfig("recursive subflow " + full))
		}
		with, e := c.inputTemplate(n.With, sub.Inputs, inputs, doc)
		if e != nil {
			return fail(e)
		}
		cn.with = with
		cn.inputs = sub.Inputs
		c.calls[full] = true
		cn.children, s, e = c.flow(sub.Flow, owner, sub.Inputs, s, path+"/call:"+full, depth+1, calls+1, inRegion)
		delete(c.calls, full)
		if e != nil {
			return fail(e)
		}
	case n.Regions != nil:
		if inRegion {
			return fail(errConfig("nested regions forbidden"))
		}
		var typ reflect.Type
		var e error
		cn.items, typ, e = c.expression(n.Regions.Items, inputs, doc, nil)
		if e != nil {
			return fail(e)
		}
		if typ != nil {
			if typ.Kind() != reflect.Slice || typ.Elem().Kind() != reflect.String {
				return fail(errConfig("regions provider must return []string"))
			}
		} else if _, e = regionItems(cn.items, c.plan.limits.Regions); e != nil {
			return fail(e)
		}
		cn.children, s, e = c.flow(n.Regions.Flow, doc, inputs, s, path+"/region", depth+1, calls, true)
		if e != nil {
			return fail(e)
		}
		s.validated = false
	case n.Candidates != nil:
		spec := n.Candidates
		if spec.IncludeCurrent {
			if spec.Selection == "first" || spec.Groups != nil {
				return fail(errConfig("includeCurrent supports best/items only"))
			}
			if inRegion {
				return fail(errConfig("includeCurrent requires complete root validation"))
			}
			if s.live && !s.validated {
				return fail(errConfig("includeCurrent parent is not fully validated"))
			}
		}
		if spec.Selection != "" && spec.Selection != "best" && spec.Selection != "first" {
			return fail(errConfig("selection must be best or first"))
		}
		if spec.TimeoutMs < 0 || spec.SettleMs < 0 || spec.TimeoutMs > 86400000 || spec.SettleMs > 86400000 {
			return fail(errConfig("candidate time budget must be between zero and one day"))
		}
		if spec.ReserveMs < 0 {
			return fail(errConfig("candidate reserveMs must be nonnegative"))
		}
		if spec.Scorer != "" && spec.Comparator != "" {
			return fail(errConfig("scorer and comparator are mutually exclusive"))
		}
		if len(spec.AdvanceOn) > 0 && (spec.Selection != "first" || spec.Parallel != 1) {
			return fail(errConfig("advanceOn requires selection first and parallel 1"))
		}
		if spec.Parallel < 0 || spec.Parallel > c.plan.limits.Parallel {
			return fail(errConfig("candidate parallel exceeds limit"))
		}
		if e := checkRules(spec.Inputs); e != nil {
			return fail(e)
		}
		cn.inputs = spec.Inputs
		cn.scorer = spec.Scorer
		if (spec.Items == nil) == (spec.Groups == nil) {
			return fail(errConfig("candidates requires exactly one of items or groups"))
		}
		var e error
		if spec.Groups != nil {
			if spec.Selection == "first" || len(spec.AdvanceOn) > 0 || spec.SettleMs > 0 {
				return fail(errConfig("groups supports best selection without advanceOn or settleMs"))
			}
			var typ reflect.Type
			cn.groups, typ, e = c.expression(spec.Groups, inputs, doc, nil)
			if e != nil {
				return fail(e)
			}
			if typ != nil {
				if typ != reflect.TypeFor[[]CandidateGroup]() {
					return fail(errConfig("groups provider must return []workflow.CandidateGroup"))
				}
			} else {
				groups, e := decodeCandidateGroups(cn.groups, c.plan.limits.Candidates)
				if e != nil {
					return fail(e)
				}
				for j := range groups {
					for k, item := range groups[j].Items {
						groups[j].Items[k], e = c.inputTemplate(item, spec.Inputs, inputs, doc)
						if e != nil {
							return fail(e)
						}
					}
				}
				cn.groups, e = transport(groups)
				if e != nil {
					return fail(e)
				}
			}
		} else {
			var typ reflect.Type
			cn.items, typ, e = c.expression(spec.Items, inputs, doc, nil)
			if e != nil {
				return fail(e)
			}
			if typ != nil {
				if typ.Kind() != reflect.Slice {
					return fail(errConfig("candidate provider must return slice"))
				}
				if e = checkCandidateType(typ.Elem(), spec.Inputs); e != nil {
					return fail(e)
				}
			} else {
				items, ok := cn.items.([]any)
				if !ok || len(items) == 0 && !spec.IncludeCurrent || len(items) > c.plan.limits.Candidates {
					return fail(errConfig("candidate list must be nonempty and bounded"))
				}
				for index, item := range items {
					m, ok := item.(map[string]any)
					if !ok {
						return fail(errConfig("candidate item must be object"))
					}
					if items[index], e = c.inputTemplate(m, spec.Inputs, inputs, doc); e != nil {
						return fail(e)
					}
				}
			}
		}
		var candidateState compileState
		cn.children, candidateState, e = c.flow(spec.Flow, doc, spec.Inputs, s.copy(), path+"/candidate", depth+1, calls, inRegion)
		if e != nil {
			return fail(e)
		}
		if candidateState.live && !candidateState.validated {
			return fail(errConfig("candidate must finish with validated content"))
		}
		if spec.IncludeCurrent {
			candidateState = commonState([]compileState{s, candidateState})
		}
		if spec.Fallback != nil {
			var fallback compileState
			cn.other, fallback, e = c.flow(spec.Fallback, doc, inputs, s.copy(), path+"/fallback", depth+1, calls, inRegion)
			if e != nil {
				return fail(e)
			}
			if spec.IncludeCurrent {
				s = candidateState
			} else {
				s = commonState([]compileState{candidateState, fallback})
			}
		} else {
			s = candidateState
		}
	}
	return cn, s, nil
}

// applyContract 职责：模拟内容及依赖失效；使用：静态路径验证；边界：不把仅声明但未实际写入视为运行成功。
func applyContract(s compileState, spec ToolSpec) compileState {
	s = s.copy()
	previous := map[*artifactDef]map[*artifactDef]bool{}
	for _, artifact := range spec.Provides {
		if contains(spec.Reads, artifact) {
			previous[artifact.artifact()] = s.deps[artifact.artifact()]
		}
	}
	changed := map[*artifactDef]bool{}
	for _, a := range append(append([]Artifact{}, spec.Writes...), spec.Invalidates...) {
		changed[a.artifact()] = true
		if !a.artifact().options.Diagnostic {
			s.validated = false
		}
	}
	invalid := map[*artifactDef]bool{}
	for _, a := range spec.Invalidates {
		invalid[a.artifact()] = true
	}
	for change := true; change; {
		change = false
		for d, deps := range s.deps {
			if invalid[d] {
				continue
			}
			for dep := range deps {
				if changed[dep] || invalid[dep] {
					invalid[d] = true
					change = true
					break
				}
			}
		}
	}
	for d := range invalid {
		delete(s.available, d)
		delete(s.deps, d)
	}
	for d := range s.evidence {
		if changed[d] || invalid[d] {
			s.validated = false
		}
	}
	for _, a := range spec.Provides {
		d := a.artifact()
		s.available[d] = true
		s.deps[d] = map[*artifactDef]bool{}
		for dependency := range previous[d] {
			if dependency != d {
				s.deps[d][dependency] = true
			}
		}
		for _, req := range spec.Requires {
			if req.artifact() != d {
				s.deps[d][req.artifact()] = true
			}
		}
	}
	for _, a := range spec.Consumes {
		delete(s.available, a.artifact())
	}
	if spec.Role == Validate {
		s.validated = true
		s.evidence = map[*artifactDef]bool{}
		for _, a := range spec.Requires {
			s.evidence[a.artifact()] = true
		}
	}
	return s
}

// commonState 职责：保留所有可达路径共同有效产物；使用：分支和备用汇合；边界：依赖取并集以保守失效。
func commonState(states []compileState) compileState {
	var out compileState
	found := false
	for _, s := range states {
		if !s.live {
			continue
		}
		if !found {
			out = s.copy()
			found = true
			continue
		}
		out.validated = out.validated && s.validated
		if out.evidence == nil {
			out.evidence = map[*artifactDef]bool{}
		}
		for key := range s.evidence {
			out.evidence[key] = true
		}
		for d := range out.available {
			if !s.available[d] {
				delete(out.available, d)
				continue
			}
			if out.deps[d] == nil {
				out.deps[d] = map[*artifactDef]bool{}
			}
			for dep := range s.deps[d] {
				out.deps[d][dep] = true
			}
		}
		for d, deps := range s.deps {
			if out.deps[d] == nil {
				out.deps[d] = map[*artifactDef]bool{}
			}
			for dep := range deps {
				out.deps[d][dep] = true
			}
		}
	}
	if !found {
		return compileState{available: map[*artifactDef]bool{}, deps: map[*artifactDef]map[*artifactDef]bool{}, live: false}
	}
	return out
}
