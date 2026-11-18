package workflow

// 文件职责：执行精确类型绑定和复合字段整体覆盖；使用：注册、编译与调用；边界：只处理参数运输值，不持有领域真源。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

// transport 职责：复制可序列化参数并保留整数；使用：冻结默认值和动态提供器结果；边界：拒绝 null 和非有限值。
func transport(v any) (any, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var out any
	if e = d.Decode(&out); e != nil {
		return nil, e
	}
	if e = noNull(out, ""); e != nil {
		return nil, e
	}
	return out, nil
}

// noNull 职责：拒绝参数中的空值；使用：所有参数来源；边界：缺失字段不等于 null。
func noNull(v any, path string) error {
	if v == nil {
		return fmt.Errorf("%s: null is forbidden", path)
	}
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if e := noNull(v, path+"."+k); e != nil {
				return e
			}
		}
	case []any:
		for i, v := range x {
			if e := noNull(v, fmt.Sprintf("%s[%d]", path, i)); e != nil {
				return e
			}
		}
	}
	return nil
}

// fieldName 职责：从真实字段生成参数名；使用：绑定与 Schema；边界：匿名嵌入与未导出字段禁止注册。
func fieldName(f reflect.StructField) string {
	for _, tag := range []string{"json", "yaml"} {
		if raw := f.Tag.Get(tag); raw != "" {
			if s := strings.Split(raw, ",")[0]; s != "" {
				return s
			}
			return f.Name
		}
	}
	return f.Name
}

// validateArgType 职责：拒绝不明确的参数类型；使用：注册阶段；边界：不接受 interface、函数、任意 map 键及递归类型。
func validateArgType(t reflect.Type, seen map[reflect.Type]bool) error {
	if seen[t] {
		return fmt.Errorf("recursive argument type %v", t)
	}
	seen[t] = true
	defer delete(seen, t)
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return validateArgType(t.Elem(), seen)
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return fmt.Errorf("map key must be string")
		}
		return validateArgType(t.Elem(), seen)
	case reflect.Struct:
		names := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			n := fieldName(f)
			if n == "-" {
				continue
			}
			if f.PkgPath != "" || f.Anonymous {
				return fmt.Errorf("field %s must be exported and explicit", f.Name)
			}
			for _, tag := range []string{"json", "yaml"} {
				parts := strings.Split(f.Tag.Get(tag), ",")
				for _, option := range parts[1:] {
					if option != "" && option != "omitempty" && option != "omitzero" {
						return fmt.Errorf("field %s: unsupported tag option %s", f.Name, option)
					}
				}
			}
			if names[n] {
				return fmt.Errorf("duplicate argument field %s", n)
			}
			names[n] = true
			if e := validateArgType(f.Type, seen); e != nil {
				return e
			}
			for _, tag := range []string{"min", "max"} {
				if s := f.Tag.Get(tag); s != "" {
					ft := f.Type
					for ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					kind := ft.Kind()
					if kind != reflect.String && kind != reflect.Array && kind != reflect.Slice && kind != reflect.Map && !(kind >= reflect.Int && kind <= reflect.Float64 && kind != reflect.Uintptr) {
						return fmt.Errorf("field %s: range tag on unsupported type", n)
					}
					v, e := strconv.ParseFloat(s, 64)
					if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
						return fmt.Errorf("field %s invalid %s", n, tag)
					}
				}
			}
		}
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
	default:
		return fmt.Errorf("unsupported argument type %v", t)
	}
	return nil
}

// bindValue 职责：通过成熟解码器建立独立强类型参数；使用：每次工具调用；边界：Schema 和精确数值 hook 禁止弱转换、溢出、未知字段及 null。
func bindValue(v any, t reflect.Type, path string) (reflect.Value, error) {
	out := reflect.New(t)
	if e := noNull(v, path); e != nil {
		return out.Elem(), e
	}
	schema, e := schemaForType(t)
	if e != nil {
		return out.Elem(), e
	}
	if e = schema.Validate(v); e != nil {
		return out.Elem(), fmt.Errorf("%s: %w", path, e)
	}
	decoder, e := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: out.Interface(), TagName: "json,yaml", ErrorUnused: true, WeaklyTypedInput: false, ZeroFields: true, DisableUnmarshaler: true,
		MatchName: func(a, b string) bool { return a == b }, DecodeHook: mapstructure.DecodeHookFuncType(exactNumberHook),
	})
	if e != nil {
		return out.Elem(), e
	}
	if e = decoder.Decode(v); e != nil {
		return out.Elem(), fmt.Errorf("%s: %w", path, e)
	}
	return out.Elem(), nil
}

// BindDocumentParameter 在执行前按强类型默认值解析一个命名对象常量，使用与工具参数相同的严格绑定规则。
func BindDocumentParameter[T any](doc *Document, key string, defaults T) (T, error) {
	raw, provided := doc.Parameters[key]
	if !provided {
		return defaults, nil
	}
	path := doc.ID + ".parameters." + key
	overrides, ok := raw.(map[string]any)
	if !ok {
		return defaults, fmt.Errorf("%s: expected object", path)
	}
	base, err := transport(defaults)
	if err != nil {
		return defaults, err
	}
	fields, ok := base.(map[string]any)
	if !ok {
		return defaults, fmt.Errorf("%s: expected object defaults", path)
	}
	value, err := bindValue(merge(fields, overrides), reflect.TypeFor[T](), path)
	if err != nil {
		return defaults, err
	}
	return value.Interface().(T), nil
}

// exactNumberHook 职责：在容器解码前保留数值类型与精度；使用：mapstructure hook；边界：窄整数先按目标位数检查，不依赖库内部截断转换。
func exactNumberHook(from, to reflect.Type, v any) (any, error) {
	if to.Kind() == reflect.Pointer {
		return v, nil
	}
	if n, ok := v.(json.Number); ok {
		out := reflect.New(to).Elem()
		switch to.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			x, e := strconv.ParseInt(string(n), 10, to.Bits())
			if e != nil {
				return nil, e
			}
			out.SetInt(x)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			x, e := strconv.ParseUint(string(n), 10, to.Bits())
			if e != nil {
				return nil, e
			}
			out.SetUint(x)
		case reflect.Float32, reflect.Float64:
			x, e := strconv.ParseFloat(string(n), to.Bits())
			if e != nil || math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, fmt.Errorf("finite in-range number required")
			}
			out.SetFloat(x)
		default:
			return nil, fmt.Errorf("number cannot bind to %v", to)
		}
		return out.Interface(), nil
	}
	switch to.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return nil, fmt.Errorf("expected precise json.Number for %v, got %v", to, from)
	case reflect.String:
		if from.Kind() != reflect.String {
			return nil, fmt.Errorf("expected string")
		}
	case reflect.Bool:
		if from.Kind() != reflect.Bool {
			return nil, fmt.Errorf("expected boolean")
		}
	}
	return v, nil
}

// fieldRule 职责：用同一字段 Schema 校验部分默认值；使用：覆盖层与静态字段；边界：跨字段语义由 ToolSpec.Validate 补充。
func fieldRule(f reflect.StructField, v reflect.Value, path string) error {
	if f.Tag.Get("enum") == "" && f.Tag.Get("min") == "" && f.Tag.Get("max") == "" && f.Tag.Get("itemMin") == "" && f.Tag.Get("itemMax") == "" {
		return nil
	}
	schema, e := schemaForField(f)
	if e != nil {
		return e
	}
	value, e := transport(v.Interface())
	if e != nil {
		return e
	}
	if e = schema.Validate(value); e != nil {
		return fmt.Errorf("%s: %w", path, e)
	}
	return nil
}

// merge 职责：按字段覆盖默认值；使用：default -> toolDefaults -> with；边界：不递归合并复合值。
func merge(ms ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range ms {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// checkInputs 职责：绑定显式流程输入；使用：子流程与候选；边界：未知键及缺失必填值拒绝。
func checkInputs(values map[string]any, rules map[string]Parameter) (map[string]any, error) {
	out := map[string]any{}
	for k := range values {
		if _, ok := rules[k]; !ok {
			return nil, fmt.Errorf("input.%s: unknown field", k)
		}
	}
	for k, r := range rules {
		v, ok := values[k]
		if !ok {
			v = r.Default
			ok = v != nil
		}
		if !ok {
			if r.Required {
				return nil, fmt.Errorf("input.%s: required", k)
			}
			continue
		}
		if e := checkParameter(v, r.Type); e != nil {
			return nil, fmt.Errorf("input.%s: %w", k, e)
		}
		out[k] = v
	}
	return out, nil
}

// checkParameter 职责：校验流程输入类型；使用：规则声明；边界：整数精度不丢失。
func checkParameter(v any, kind string) error {
	ok := false
	switch kind {
	case "string":
		_, ok = v.(string)
	case "boolean":
		_, ok = v.(bool)
	case "object":
		_, ok = v.(map[string]any)
	case "array":
		_, ok = v.([]any)
	case "integer":
		if n, yes := v.(json.Number); yes {
			_, e := strconv.ParseInt(string(n), 10, 64)
			if e != nil {
				_, e = strconv.ParseUint(string(n), 10, 64)
			}
			ok = e == nil
		}
	case "number":
		if n, yes := v.(json.Number); yes {
			x, e := n.Float64()
			ok = e == nil && !math.IsInf(x, 0) && !math.IsNaN(x)
		}
	}
	if !ok {
		return fmt.Errorf("expected %s", kind)
	}
	return noNull(v, "input")
}
