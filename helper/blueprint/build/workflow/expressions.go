package workflow

// 文件职责：解析显式参数引用与动态提供器；使用：编译模板及每次步骤绑定；边界：不把字符串当 Go 字段路径，不执行任意表达式。

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// reference 职责：识别唯一引用对象；使用：参数展开；边界：引用和其他字段混写立即拒绝。
func reference(v any) (kind, path string, ok bool, err error) {
	m, yes := v.(map[string]any)
	if !yes {
		return
	}
	for k, x := range m {
		if strings.HasPrefix(k, "$") {
			if len(m) != 1 {
				return "", "", false, fmt.Errorf("reference must have exactly one key")
			}
			if k != "$param" && k != "$input" && k != "$state" {
				return "", "", false, fmt.Errorf("unknown reference %s", k)
			}
			s, yes := x.(string)
			if !yes || s == "" {
				return "", "", false, fmt.Errorf("reference path must be nonempty string")
			}
			return k, s, true, nil
		}
	}
	return
}

// lookup 职责：在参数运输树上读取显式路径；使用：$param/$input；边界：不接受领域对象和反射路径。
func lookup(root map[string]any, path string) (any, error) {
	var v any = root
	for _, part := range strings.Split(path, ".") {
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[part]
			if !ok {
				return nil, fmt.Errorf("missing reference %s", path)
			}
		case []any:
			i, e := strconv.Atoi(part)
			if e != nil || i < 0 || i >= len(x) {
				return nil, fmt.Errorf("missing reference %s", path)
			}
			v = x[i]
		default:
			return nil, fmt.Errorf("invalid reference %s", path)
		}
	}
	if v == nil {
		return nil, fmt.Errorf("null reference %s", path)
	}
	return v, nil
}

// parameterType 职责：把流程参数语义映射成静态类型；使用：动态引用检查；边界：object/array 内部结构由运行期严格绑定。
func parameterType(kind string) reflect.Type {
	switch kind {
	case "string":
		return reflect.TypeFor[string]()
	case "boolean":
		return reflect.TypeFor[bool]()
	case "integer":
		return reflect.TypeFor[int64]()
	case "number":
		return reflect.TypeFor[float64]()
	case "object", "array":
		return reflect.TypeFor[any]()
	}
	return nil
}

// expression 职责：冻结常量并检查动态引用目录；使用：编译；边界：动态值不会跨步骤缓存。
func (c *compiler) expression(v any, inputs map[string]Parameter, doc *Document, t *tool) (any, reflect.Type, error) {
	kind, path, is, e := reference(v)
	if e != nil {
		return nil, nil, e
	}
	if !is {
		return v, nil, nil
	}
	switch kind {
	case "$param":
		x, e := lookup(doc.Parameters, path)
		if e != nil {
			return nil, nil, e
		}
		if _, _, ok, _ := reference(x); ok {
			return nil, nil, errConfig("parameters must contain literal values")
		}
		return x, nil, nil
	case "$input":
		parts := strings.Split(path, ".")
		r, ok := inputs[parts[0]]
		if !ok {
			return nil, nil, errConfig("unknown input " + path)
		}
		typ := parameterType(r.Type)
		if len(parts) > 1 {
			if r.Type != "object" && r.Type != "array" {
				return nil, nil, errConfig("input path traverses scalar " + path)
			}
			typ = reflect.TypeFor[any]()
		}
		return v, typ, nil
	case "$state":
		p, ok := c.plan.registry.states[path]
		if !ok {
			return nil, nil, errConfig("unknown state provider " + path)
		}
		if t != nil {
			for _, a := range p.Requires {
				if !contains(t.spec.Reads, a) && !contains(t.spec.AllRegions, a) {
					return nil, nil, errConfig("state provider exceeds tool read permission " + path)
				}
			}
		}
		return v, p.Type, nil
	}
	return nil, nil, errConfig("invalid reference")
}

// compatible 职责：检查动态引用的大类与结构；使用：静态类型验证；边界：窄整数溢出由运行实际值再次检查。
func compatible(source, target reflect.Type) bool {
	if target == reflect.TypeFor[any]() {
		return true
	}
	if source == reflect.TypeFor[any]() {
		return true
	}
	for source.Kind() == reflect.Pointer {
		source = source.Elem()
	}
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	number := func(k reflect.Kind) bool { return k >= reflect.Int && k <= reflect.Float64 && k != reflect.Uintptr }
	if number(source.Kind()) && number(target.Kind()) {
		if (source.Kind() == reflect.Float32 || source.Kind() == reflect.Float64) && target.Kind() != reflect.Float32 && target.Kind() != reflect.Float64 {
			return false
		}
		return true
	}
	if source.Kind() != target.Kind() {
		if source.Kind() == reflect.Struct && target.Kind() == reflect.Map && target.Key().Kind() == reflect.String {
			return true
		}
		return false
	}
	switch source.Kind() {
	case reflect.Struct:
		for i := 0; i < source.NumField(); i++ {
			f := source.Field(i)
			name := fieldName(f)
			if name == "-" {
				continue
			}
			tf, ok := argField(target, name)
			if !ok || !compatible(f.Type, tf.Type) {
				return false
			}
		}
		return true
	case reflect.Slice, reflect.Array:
		return compatible(source.Elem(), target.Elem())
	case reflect.Map:
		return source.Key().Kind() == reflect.String && target.Key().Kind() == reflect.String && compatible(source.Elem(), target.Elem())
	}
	return source.Kind() == target.Kind()
}

// freezeExpression 职责：递归冻结运输树中的命名常量；使用：列表与流程对象；边界：动态引用只保留显式模板。
func (c *compiler) freezeExpression(v any, inputs map[string]Parameter, doc *Document) (any, error) {
	x, typ, e := c.expression(v, inputs, doc, nil)
	if e != nil {
		return nil, e
	}
	if typ != nil {
		return x, nil
	}
	kind, _, is, e := reference(v)
	if e != nil {
		return nil, e
	}
	if is && kind == "$param" {
		return x, nil
	}
	switch value := x.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range value {
			bound, e := c.freezeExpression(v, inputs, doc)
			if e != nil {
				return nil, e
			}
			out[k] = bound
		}
		return out, nil
	case []any:
		out := make([]any, len(value))
		for j, v := range value {
			bound, e := c.freezeExpression(v, inputs, doc)
			if e != nil {
				return nil, e
			}
			out[j] = bound
		}
		return out, nil
	}
	return x, nil
}

// checkStateReferences 职责：校验属性提供器的静态前置产物；使用：每个节点表达式；边界：不是动态产物存在性检查的替代。
func (c *compiler) checkStateReferences(v any, s compileState) error {
	kind, name, is, e := reference(v)
	if e != nil {
		return e
	}
	if is {
		if kind == "$state" {
			p, ok := c.plan.registry.states[name]
			if !ok {
				return errConfig("unknown state provider " + name)
			}
			for _, a := range p.Requires {
				if s.live && !s.available[a.artifact()] {
					return errConfig("state provider requires artifact " + a.artifact().name)
				}
			}
		}
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		for _, child := range x {
			if e := c.checkStateReferences(child, s); e != nil {
				return e
			}
		}
	case []any:
		for _, child := range x {
			if e := c.checkStateReferences(child, s); e != nil {
				return e
			}
		}
	}
	return nil
}

// template 职责：校验静态参数并保留动态模板；使用：工具绑定；边界：复合值不做递归默认合并。
func (c *compiler) template(v any, target reflect.Type, inputs map[string]Parameter, doc *Document, t *tool) (any, bool, error) {
	x, typ, e := c.expression(v, inputs, doc, t)
	if e != nil {
		return nil, false, e
	}
	if typ != nil {
		if !compatible(typ, target) {
			return nil, false, fmt.Errorf("dynamic reference type %v incompatible with %v", typ, target)
		}
		return x, true, nil
	}
	v = x
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	dynamic := false
	switch target.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("expected argument object for %v", target)
		}
		out := map[string]any{}
		for name, value := range m {
			f, ok := argField(target, name)
			if !ok {
				return nil, false, fmt.Errorf("argument.%s: unknown field", name)
			}
			bound, d, e := c.template(value, f.Type, inputs, doc, t)
			if e != nil {
				return nil, false, fmt.Errorf("argument.%s: %w", name, e)
			}
			if !d {
				r, e := bindValue(bound, f.Type, "argument."+name)
				if e != nil {
					return nil, false, e
				}
				if e = fieldRule(f, r, "argument."+name); e != nil {
					return nil, false, e
				}
			}
			out[name] = bound
			dynamic = dynamic || d
		}
		if t != nil {
			for i := 0; i < target.NumField(); i++ {
				f := target.Field(i)
				if f.Tag.Get("required") == "true" {
					if _, ok := out[fieldName(f)]; !ok {
						return nil, false, fmt.Errorf("argument.%s: required", fieldName(f))
					}
				}
			}
		}
		return out, dynamic, nil
	case reflect.Map:
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("expected argument map")
		}
		out := map[string]any{}
		for name, value := range m {
			bound, d, e := c.template(value, target.Elem(), inputs, doc, t)
			if e != nil {
				return nil, false, e
			}
			out[name] = bound
			dynamic = dynamic || d
		}
		return out, dynamic, nil
	case reflect.Slice, reflect.Array:
		a, ok := v.([]any)
		if !ok {
			return nil, false, fmt.Errorf("expected argument array")
		}
		if target.Kind() == reflect.Array && len(a) != target.Len() {
			return nil, false, fmt.Errorf("argument array length mismatch")
		}
		out := make([]any, len(a))
		for i, value := range a {
			bound, d, e := c.template(value, target.Elem(), inputs, doc, t)
			if e != nil {
				return nil, false, e
			}
			out[i] = bound
			dynamic = dynamic || d
		}
		return out, dynamic, nil
	}
	_, e = bindValue(v, target, "argument")
	return v, false, e
}

// inputTemplate 职责：检查显式子流程参数并冻结常量；使用：call 与静态候选；边界：不得隐式捕获外层输入。
func (c *compiler) inputTemplate(values map[string]any, rules map[string]Parameter, inputs map[string]Parameter, doc *Document) (map[string]any, error) {
	out := map[string]any{}
	for name, v := range values {
		r, ok := rules[name]
		if !ok {
			return nil, fmt.Errorf("input.%s: unknown", name)
		}
		x, typ, e := c.expression(v, inputs, doc, nil)
		if e != nil {
			return nil, e
		}
		if typ == nil {
			if e = checkParameter(x, r.Type); e != nil {
				return nil, fmt.Errorf("input.%s: %w", name, e)
			}
		} else if !compatible(typ, parameterType(r.Type)) && parameterType(r.Type) != reflect.TypeFor[any]() {
			return nil, fmt.Errorf("input.%s: dynamic type mismatch", name)
		}
		frozen, e := c.freezeExpression(x, inputs, doc)
		if e != nil {
			return nil, e
		}
		out[name] = frozen
	}
	for name, r := range rules {
		if _, ok := out[name]; !ok {
			if r.Default != nil {
				out[name] = r.Default
			} else if r.Required {
				return nil, fmt.Errorf("input.%s: required", name)
			}
		}
	}
	return out, nil
}

// checkCandidateType 职责：检查动态候选集合元素契约；使用：编译提供器；边界：元素必须是明确结构体或字符串键对象。
func checkCandidateType(t reflect.Type, rules map[string]Parameter) error {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map && t.Key().Kind() == reflect.String {
		return nil
	}
	if t.Kind() != reflect.Struct {
		return errConfig("candidate item must be struct or object")
	}
	seen := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := fieldName(f)
		if name == "-" {
			continue
		}
		r, ok := rules[name]
		if !ok {
			return errConfig("candidate field not declared " + name)
		}
		seen[name] = true
		if typ := parameterType(r.Type); typ != reflect.TypeFor[any]() && !compatible(f.Type, typ) {
			return errConfig("candidate field type mismatch " + name)
		}
	}
	for name, r := range rules {
		if r.Required && r.Default == nil && !seen[name] {
			return errConfig("candidate missing field " + name)
		}
	}
	return nil
}

// resolve 职责：冻结本次调用引用值；使用：引擎；边界：每次复制、无跨步骤缓存，属性仅调用显式提供器。
func resolve(ctx context.Context, r *Registry, v any, inputs map[string]any, i *BlueprintInstance, region string, candidate int, path string, toolStep *StepContext) (any, error) {
	kind, name, is, e := reference(v)
	if e != nil {
		return nil, e
	}
	if is {
		switch kind {
		case "$input":
			x, e := lookup(inputs, name)
			if e != nil {
				return nil, e
			}
			return transport(x)
		case "$param":
			return nil, errConfig("uncompiled parameter reference")
		case "$state":
			p, ok := r.states[name]
			if !ok {
				return nil, errConfig("unknown state " + name)
			}
			spec := ToolSpec{ID: "state:" + name, Reads: p.Requires, Requires: p.Requires, Role: Diagnostic}
			if region == "" {
				for _, a := range p.Requires {
					if a.artifact().options.Regional {
						spec.AllRegions = append(spec.AllRegions, a)
					}
				}
				var reads []Artifact
				for _, a := range spec.Reads {
					if !a.artifact().options.Regional {
						reads = append(reads, a)
					}
				}
				spec.Reads = reads
			}
			if toolStep != nil {
				for _, a := range p.Requires {
					if !contains(toolStep.spec.Reads, a) && !contains(toolStep.spec.AllRegions, a) {
						return nil, fmt.Errorf("state provider exceeds tool permissions")
					}
				}
			}
			step, e := beginStep(i, spec, region, candidate, path+"/$state:"+name)
			if e != nil {
				return nil, e
			}
			defer step.expire()
			x, e := p.Resolve(ctx, step)
			if e != nil {
				return nil, e
			}
			step.mu.Lock()
			violation := step.violation
			deps := copyDeps(step.reads)
			step.mu.Unlock()
			if violation != nil {
				return nil, violation
			}
			if reflect.TypeOf(x) != p.Type {
				return nil, fmt.Errorf("state %s returned %T, expected %v", name, x, p.Type)
			}
			if toolStep != nil {
				toolStep.mu.Lock()
				for id, version := range deps {
					toolStep.reads[id] = version
				}
				toolStep.mu.Unlock()
			}
			return transport(x)
		}
	}
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, value := range x {
			bound, e := resolve(ctx, r, value, inputs, i, region, candidate, path, toolStep)
			if e != nil {
				return nil, fmt.Errorf("%s.%s: %w", path, k, e)
			}
			out[k] = bound
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for j, value := range x {
			bound, e := resolve(ctx, r, value, inputs, i, region, candidate, path, toolStep)
			if e != nil {
				return nil, e
			}
			out[j] = bound
		}
		return out, nil
	}
	return v, nil
}

// regionItems 职责：核对动态区域列表；使用：编译静态值与运行；边界：非空、唯一、有限且顺序稳定。
func regionItems(v any, max int) ([]string, error) {
	a, ok := v.([]any)
	if !ok || len(a) == 0 || len(a) > max {
		return nil, fmt.Errorf("regions require nonempty bounded array")
	}
	out := make([]string, len(a))
	seen := map[string]bool{}
	for j, x := range a {
		s, ok := x.(string)
		if !ok || s == "" || seen[s] {
			return nil, fmt.Errorf("invalid or duplicate region")
		}
		seen[s] = true
		out[j] = s
	}
	return out, nil
}
