package workflow

// 文件职责：构建只读反射白名单及稳定属性目录；使用：适配层初始化一次；边界：目录不保存任务状态，接收器必须由调用方保证只读且并发安全。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
)

// Role 职责：限制验证、编码和诊断的版本行为；使用：ToolSpec；边界：非内容角色不得写内容产物。
type Role string

// 角色常量职责：表达固定提交语义；使用：工具注册；边界：不允许运行时改变角色。
const (
	// Transform 职责：允许内容变更；使用：规划装配；边界：提交使旧验证失效。
	Transform Role = "transform"
	// Validate 职责：绑定成功验证到当前版本；使用：最终检查；边界：不可写内容。
	Validate Role = "validate"
	// Encode 职责：要求已验证的相同版本；使用：编码；边界：不可写内容。
	Encode Role = "encode"
	// Diagnostic 职责：写报告而不标记通过；使用：追踪；边界：不可写内容。
	Diagnostic Role = "diagnostic"
)

// ToolSpec 职责：显式绑定真实工具及能力契约；使用：NewRegistry；边界：未列出的方法永远不能从 YAML 调用。
type ToolSpec struct {
	// TraceFields 职责：明确允许旁路输出的顶层策略参数；使用：后台跟踪；边界：缺省不输出，不能指向领域状态，名称需为真实参数字段。
	TraceFields []string
	// ID 职责：稳定业务标识；使用：YAML tool；边界：格式 domain.action@version。
	ID string
	// Receiver 职责：已构造无状态结构体指针；使用：绑定 Method；边界：拒绝 typed nil。
	Receiver any
	// Method 职责：明确导出入口名；使用：反射缓存；边界：不自动枚举方法。
	Method string
	// Implementation 职责：实现构建身份；使用：指纹；边界：应随算法实现版本更新。
	Implementation string
	// Reads 职责：允许读取的产物；使用：受限步骤；边界：不允许字符串路径访问。
	Reads []Artifact
	// Writes 职责：允许暂存的产物；使用：受限步骤；边界：必须同时声明 Provides。
	Writes []Artifact
	// AllRegions 职责：根步骤读取所有目标区域；使用：跨区汇合；边界：不允许区域内读取邻区。
	AllRegions []Artifact
	// Requires 职责：进入工具前必须有效的产物；使用：静态与动态校验；边界：区域键在根步骤要求所有目标区域。
	Requires []Artifact
	// Provides 职责：成功必须提交的产物；使用：编译依赖；边界：禁止空声明成功。
	Provides []Artifact
	// Invalidates 职责：显式使产物失效；使用：保守依赖清理；边界：依赖闭包也自动失效。
	Invalidates []Artifact
	// Consumes 职责：成功后释放输入值并保留版本证明；使用：线性不可变阶段；边界：必须 Requires 且可读，不得同时 Provides/Invalidates，失败取消不消费。
	Consumes []Artifact
	// Defaults 职责：版本级默认参数；使用：优先级最底层；边界：同样严格校验。
	Defaults map[string]any
	// Required 职责：合并后必填顶层字段；使用：区分零值；边界：也可用 required 标签。
	Required []string
	// Validate 职责：补充强类型跨字段约束；使用：静态完整值及运行调用；边界：必须纯函数，可用泛型 ArgumentValidator 包装。
	Validate func(any) error
	// FailureCodes 职责：声明允许候选回退的无解码；使用：工具错误归类；边界：其他错误均致命。
	FailureCodes []string
	// Role 职责：指定提交角色；使用：最终验证编码；边界：零值为 Transform。
	Role Role
}

// ArgumentValidator 职责：包装强类型参数语义校验；使用：ToolSpec.Validate；边界：类型不符立即失败。
func ArgumentValidator[T any](fn func(T) error) func(any) error {
	return func(v any) error {
		x, ok := v.(T)
		if !ok {
			return fmt.Errorf("validator argument type mismatch")
		}
		return fn(x)
	}
}

// StateProvider 职责：显式注册可读实例属性；使用：$state；边界：仅获得所声明产物的只读句柄，不得保存步骤。
type StateProvider struct {
	// Name 职责：稳定属性名；使用：$state；边界：不反射遍历领域对象。
	Name string
	// Type 职责：真实返回类型；使用：编译动态引用校验；边界：运行再次精确核对。
	Type reflect.Type
	// Requires 职责：属性依赖产物；使用：权限和版本校验；边界：工具参数引用时必须拥有这些读取权限。
	Requires []Artifact
	// Resolve 职责：计算只读属性；使用：步骤开始前；边界：错误或 panic 致命，不允许调度。
	Resolve func(context.Context, *StepContext) (any, error)
}

// tool 职责：缓存已核对的方法值与类型；使用：编译绑定；边界：不缓存执行参数。
type tool struct {
	spec   ToolSpec
	method reflect.Value
	args   reflect.Type
}

// Registry 职责：保存冻结工具和属性目录；使用：多个计划共享；边界：内部字段不导出。
type Registry struct {
	tools          map[string]*tool
	states         map[string]StateProvider
	artifacts      map[string]*artifactDef
	fingerprint    string
	documentSchema *jsonschema.Schema
}

// toolID 职责：固定业务 ID 语法；使用：注册；边界：主版本不能为零。
var toolID = regexp.MustCompile(`^[a-z][a-z0-9_-]*\.[a-z][a-z0-9_.-]*@[1-9][0-9]*$`)

// NewRegistry 职责：原子构建白名单；使用：程序启动；边界：任一签名、权限或默认值错误都会拒绝整个目录。
func NewRegistry(specs []ToolSpec, providers []StateProvider) (*Registry, error) {
	r := &Registry{tools: map[string]*tool{}, states: map[string]StateProvider{}, artifacts: map[string]*artifactDef{}}
	add := func(list []Artifact) error {
		seen := map[*artifactDef]bool{}
		for _, a := range list {
			d, e := artifactDefinition(a)
			if e != nil {
				return e
			}
			if seen[d] {
				return fmt.Errorf("duplicate artifact %s", d.name)
			}
			seen[d] = true
			if prev := r.artifacts[d.name]; prev != nil && prev != d {
				return fmt.Errorf("artifact name collision %s", d.name)
			}
			r.artifacts[d.name] = d
		}
		return nil
	}
	for _, s := range specs {
		if !toolID.MatchString(s.ID) || r.tools[s.ID] != nil {
			return nil, fmt.Errorf("invalid or duplicate tool ID %s", s.ID)
		}
		rv := reflect.ValueOf(s.Receiver)
		if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
			return nil, fmt.Errorf("%s: receiver must be non-nil struct pointer", s.ID)
		}
		m := rv.MethodByName(s.Method)
		if !m.IsValid() {
			return nil, fmt.Errorf("%s: unknown method %s", s.ID, s.Method)
		}
		mt := m.Type()
		if mt.IsVariadic() || mt.NumIn() != 3 || mt.In(0) != reflect.TypeFor[context.Context]() || mt.In(1) != reflect.TypeFor[*StepContext]() || mt.In(2).Kind() != reflect.Struct || mt.NumOut() != 1 || mt.Out(0) != reflect.TypeFor[error]() {
			return nil, fmt.Errorf("%s: invalid tool signature", s.ID)
		}
		if e := validateArgType(mt.In(2), map[reflect.Type]bool{}); e != nil {
			return nil, fmt.Errorf("%s: %w", s.ID, e)
		}
		for _, list := range [][]Artifact{s.Reads, s.Writes, s.AllRegions, s.Requires, s.Provides, s.Invalidates, s.Consumes} {
			if e := add(list); e != nil {
				return nil, e
			}
		}
		s.Reads = append([]Artifact(nil), s.Reads...)
		s.Writes = append([]Artifact(nil), s.Writes...)
		s.AllRegions = append([]Artifact(nil), s.AllRegions...)
		s.Requires = append([]Artifact(nil), s.Requires...)
		s.Provides = append([]Artifact(nil), s.Provides...)
		s.Invalidates = append([]Artifact(nil), s.Invalidates...)
		s.Consumes = append([]Artifact(nil), s.Consumes...)
		s.Required = append([]string(nil), s.Required...)
		s.TraceFields = append([]string(nil), s.TraceFields...)
		traceNames := map[string]bool{}
		for _, name := range s.TraceFields {
			if _, ok := argField(mt.In(2), name); !ok || traceNames[name] {
				return nil, fmt.Errorf("%s: invalid or duplicate trace field %s", s.ID, name)
			}
			traceNames[name] = true
		}
		s.FailureCodes = append([]string(nil), s.FailureCodes...)
		if s.Role == "" {
			s.Role = Transform
		}
		if s.Role != Transform && s.Role != Validate && s.Role != Encode && s.Role != Diagnostic {
			return nil, fmt.Errorf("%s: unknown role", s.ID)
		}
		for _, a := range s.Requires {
			if !contains(s.Reads, a) && !contains(s.AllRegions, a) {
				return nil, fmt.Errorf("%s: requires lacks read permission", s.ID)
			}
		}
		for _, a := range s.Consumes {
			if !contains(s.Requires, a) || (!contains(s.Reads, a) && !contains(s.AllRegions, a)) {
				return nil, fmt.Errorf("%s: consumes must require read permission", s.ID)
			}
			if contains(s.Provides, a) || contains(s.Invalidates, a) {
				return nil, fmt.Errorf("%s: consumes cannot provide or invalidate the same artifact", s.ID)
			}
		}
		for _, a := range s.AllRegions {
			if !a.artifact().options.Regional {
				return nil, fmt.Errorf("%s: AllRegions requires regional key", s.ID)
			}
		}
		for _, a := range s.Writes {
			if !contains(s.Provides, a) {
				return nil, fmt.Errorf("%s: write must declare provides", s.ID)
			}
			if s.Role != Transform && !a.artifact().options.Diagnostic {
				return nil, fmt.Errorf("%s: non-transform writes content", s.ID)
			}
		}
		for _, a := range s.Provides {
			if !contains(s.Writes, a) {
				return nil, fmt.Errorf("%s: provides lacks write permission", s.ID)
			}
		}
		if s.Role != Transform {
			for _, a := range s.Invalidates {
				if !a.artifact().options.Diagnostic {
					return nil, fmt.Errorf("%s: non-transform invalidates content", s.ID)
				}
			}
		}
		if s.Defaults == nil {
			s.Defaults = map[string]any{}
		}
		v, e := transport(s.Defaults)
		if e != nil {
			return nil, e
		}
		s.Defaults = v.(map[string]any)
		t := &tool{s, m, mt.In(2)}
		if e = t.checkPartial(s.Defaults, "defaults"); e != nil {
			return nil, fmt.Errorf("%s: %w", s.ID, e)
		}
		for _, name := range s.Required {
			if _, ok := argField(t.args, name); !ok {
				return nil, fmt.Errorf("%s: required unknown field %s", s.ID, name)
			}
		}
		complete := true
		for _, name := range s.Required {
			if _, ok := s.Defaults[name]; !ok {
				complete = false
			}
		}
		for j := 0; j < t.args.NumField(); j++ {
			f := t.args.Field(j)
			if f.Tag.Get("required") == "true" {
				if _, ok := s.Defaults[fieldName(f)]; !ok {
					complete = false
				}
			}
		}
		if complete {
			if _, e := t.bind(s.Defaults, "defaults"); e != nil {
				return nil, fmt.Errorf("%s: %w", s.ID, e)
			}
		}
		r.tools[s.ID] = t
	}
	for _, p := range providers {
		if p.Name == "" || p.Type == nil || p.Resolve == nil {
			return nil, fmt.Errorf("invalid state provider")
		}
		if _, ok := r.states[p.Name]; ok {
			return nil, fmt.Errorf("duplicate state provider %s", p.Name)
		}
		if e := validateProviderType(p.Type); e != nil {
			return nil, fmt.Errorf("state %s: %w", p.Name, e)
		}
		if e := add(p.Requires); e != nil {
			return nil, e
		}
		p.Requires = append([]Artifact(nil), p.Requires...)
		r.states[p.Name] = p
	}
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	var identity []any
	for _, n := range names {
		t := r.tools[n]
		identity = append(identity, []any{n, t.spec.Method, reflect.TypeOf(t.spec.Receiver).String(), t.spec.Implementation, t.args.String(), typeSchema(t.args), t.spec.Defaults, t.spec.Required, t.spec.FailureCodes, t.spec.Role, artifactNames(t.spec.Reads), artifactNames(t.spec.Writes), artifactNames(t.spec.Requires), artifactNames(t.spec.Provides), artifactNames(t.spec.Invalidates), artifactNames(t.spec.AllRegions), artifactNames(t.spec.Consumes), t.spec.TraceFields})
	}
	names = names[:0]
	for n := range r.states {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := r.states[n]
		identity = append(identity, []any{n, p.Type.String(), typeSchema(p.Type), artifactNames(p.Requires)})
	}
	b, _ := json.Marshal(identity)
	h := sha256.Sum256(b)
	r.fingerprint = hex.EncodeToString(h[:])
	var err error
	r.documentSchema, err = compileDocumentSchema(r)
	if err != nil {
		return nil, fmt.Errorf("workflow schema: %w", err)
	}
	return r, nil
}

// validateProviderType 职责：允许显式组参数运输 DTO；使用：状态提供器注册；边界：其他提供器仍遵守真实强类型限制，工具参数不放开 any。
func validateProviderType(t reflect.Type) error {
	if t == reflect.TypeFor[[]CandidateGroup]() {
		return nil
	}
	return validateArgType(t, map[reflect.Type]bool{})
}

// contains 职责：按键身份核对能力；使用：权限检查；边界：同名不同键无权访问。
func contains(list []Artifact, a Artifact) bool {
	for _, x := range list {
		if x.artifact() == a.artifact() {
			return true
		}
	}
	return false
}

// artifactNames 职责：稳定呈现契约；使用：指纹；边界：只输出描述，不输出值。
func artifactNames(list []Artifact) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		d := a.artifact()
		out = append(out, fmt.Sprintf("%s:%s:%t:%t:%t", d.name, d.typ, d.options.Regional, d.options.Diagnostic, d.sharedClone))
	}
	sort.Strings(out)
	return out
}

// argField 职责：定位精确参数名；使用：编译；边界：禁止大小写近似匹配。
func argField(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if fieldName(f) == name && name != "-" {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// checkPartial 职责：检查覆盖层字段与静态值；使用：默认值注册；边界：此阶段不要求完整参数。
func (t *tool) checkPartial(m map[string]any, path string) error {
	for n, v := range m {
		f, ok := argField(t.args, n)
		if !ok {
			return fmt.Errorf("%s.%s: unknown field", path, n)
		}
		x, e := bindValue(v, f.Type, path+"."+n)
		if e != nil {
			return e
		}
		if e = fieldRule(f, x, path+"."+n); e != nil {
			return e
		}
	}
	return nil
}

// bind 职责：每次建立独立强类型参数；使用：反射调用前；边界：完整校验失败不调用工具。
func (t *tool) bind(m map[string]any, path string) (reflect.Value, error) {
	for _, n := range t.spec.Required {
		if _, ok := m[n]; !ok {
			return reflect.Value{}, fmt.Errorf("%s.%s: required", path, n)
		}
	}
	v, e := bindValue(m, t.args, path)
	if e != nil {
		return v, e
	}
	if t.spec.Validate != nil {
		if e = validateArguments(t.spec.Validate, v.Interface()); e != nil {
			return v, fmt.Errorf("%s: %w", path, e)
		}
	}
	return v, nil
}

// validateArguments 职责：隔离纯参数语义校验器故障；使用：静态与动态参数绑定；边界：panic 作为致命错误返回并携带栈。
func validateArguments(validate func(any) error, value any) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("argument validator panic: %v\n%s", p, debug.Stack())
		}
	}()
	return validate(value)
}
