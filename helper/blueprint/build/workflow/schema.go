package workflow

// 文件职责：从真实参数类型和唯一 AST 生成 JSON Schema；使用：配置编辑器与离线校验；边界：依赖、预算及跨字段语义仍由 Compile 核对。

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

// typeSchema 职责：从注册类型生成字段结构；使用：工具参数和目录指纹；边界：不反向生成领域类型。
func typeSchema(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	s := map[string]any{}
	switch t.Kind() {
	case reflect.Interface:
		s["not"] = map[string]any{"type": "null"}
	case reflect.Struct:
		props := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := fieldName(f)
			if name == "-" {
				continue
			}
			child := fieldTypeSchema(f)
			props[name] = child
			if f.Tag.Get("required") == "true" {
				required = append(required, name)
			}
		}
		s["type"] = "object"
		s["properties"] = props
		s["additionalProperties"] = false
		if len(required) > 0 {
			s["required"] = required
		}
	case reflect.Map:
		s["type"] = "object"
		s["additionalProperties"] = typeSchema(t.Elem())
	case reflect.Slice, reflect.Array:
		s["type"] = "array"
		s["items"] = typeSchema(t.Elem())
		if t.Kind() == reflect.Array {
			s["minItems"] = t.Len()
			s["maxItems"] = t.Len()
		}
	case reflect.String:
		s["type"] = "string"
	case reflect.Bool:
		s["type"] = "boolean"
	case reflect.Float32, reflect.Float64:
		s["type"] = "number"
	default:
		s["type"] = "integer"
		bits := t.Bits()
		maximum := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		minimum := big.NewInt(0)
		if t.Kind() >= reflect.Int && t.Kind() <= reflect.Int64 {
			maximum.Rsh(maximum, 1)
			minimum.Neg(maximum)
		}
		maximum.Sub(maximum, big.NewInt(1))
		s["minimum"] = json.Number(minimum.String())
		s["maximum"] = json.Number(maximum.String())
	}
	return s
}

// expressionSchema 职责：允许显式引用替代参数值；使用：YAML Schema；边界：引用仍需要目录与类型校验。
func expressionSchema(s map[string]any) map[string]any {
	if props, ok := s["properties"].(map[string]any); ok {
		out := map[string]any{}
		for k, v := range props {
			out[k] = expressionSchema(v.(map[string]any))
		}
		s["properties"] = out
	}
	if items, ok := s["items"].(map[string]any); ok {
		s["items"] = expressionSchema(items)
	}
	if additional, ok := s["additionalProperties"].(map[string]any); ok {
		s["additionalProperties"] = expressionSchema(additional)
	}
	return map[string]any{"anyOf": []any{s, map[string]any{"$ref": "#/$defs/reference"}}}
}

// GenerateSchema 职责：生成与执行格式一致的 Schema；使用：配置工具；边界：不是编译器的替代品。
func GenerateSchema(r *Registry) ([]byte, error) {
	return json.MarshalIndent(documentSchema(r), "", "  ")
}

// GenerateSchemaWithParameter 为指定文档的命名常量补充类型提示，库内其余参数仍使用通用语法。
func GenerateSchemaWithParameter[T any](r *Registry, document, parameter string) ([]byte, error) {
	return schemaWithParameterProperties(r, document, map[string]any{parameter: typeSchema(reflect.TypeFor[T]())})
}

// GenerateSchemaWithParameters 按主文档命名常量的强类型字段导出提示，允许同一文件另行声明普通常量。
func GenerateSchemaWithParameters[T any](r *Registry, document string) ([]byte, error) {
	props := typeSchema(reflect.TypeFor[T]())["properties"].(map[string]any)
	return schemaWithParameterProperties(r, document, props)
}

func schemaWithParameterProperties(r *Registry, document string, props map[string]any) ([]byte, error) {
	root := documentSchema(r)
	root["if"] = map[string]any{"properties": map[string]any{"id": map[string]any{"const": document}}}
	parameterRules := map[string]any{"properties": props}
	root["then"] = map[string]any{"properties": map[string]any{"parameters": parameterRules}}
	return json.MarshalIndent(root, "", "  ")
}

func documentSchema(r *Registry) map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	str := map[string]any{"type": "string", "minLength": 1}
	nodes := map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/node"}}
	refVariants := []any{}
	for _, name := range []string{"$param", "$input", "$state"} {
		refVariants = append(refVariants, obj(map[string]any{name: str}, name))
	}
	param := obj(map[string]any{"type": map[string]any{"enum": []string{"string", "integer", "number", "boolean", "object", "array"}}, "required": map[string]any{"type": "boolean"}, "default": map[string]any{"not": map[string]any{"type": "null"}}}, "type")
	inputs := map[string]any{"type": "object", "additionalProperties": param}
	with := map[string]any{"type": "object"}
	variants := []any{}
	action := func(name string, definition any, extras map[string]any) map[string]any {
		props := map[string]any{"id": str, name: definition}
		for k, v := range extras {
			props[k] = v
		}
		return obj(props, "id", name)
	}
	if r != nil {
		names := make([]string, 0, len(r.tools))
		for name := range r.tools {
			names = append(names, name)
		}
		sortStrings(names)
		for _, name := range names {
			t := r.tools[name]
			s := typeSchema(t.args)
			delete(s, "required")
			variants = append(variants, action("tool", map[string]any{"const": name}, map[string]any{"with": expressionSchema(s)}))
		}
	}
	stringList := map[string]any{"type": "array", "items": str}
	groupSchema := obj(map[string]any{"id": str, "items": map[string]any{"type": "array", "items": with}, "afterNoSuccess": stringList, "excludeFailureCodes": stringList}, "id", "items")
	reserveSchema := typeSchema(reflect.TypeFor[int]())
	reserveSchema["minimum"] = 0
	candidateSchema := obj(map[string]any{
		"items":          expressionSchema(map[string]any{"type": "array", "items": with, "minItems": 0}),
		"includeCurrent": map[string]any{"type": "boolean"},
		"groups":         map[string]any{"anyOf": []any{map[string]any{"type": "array", "items": groupSchema}, map[string]any{"$ref": "#/$defs/reference"}}},
		"inputs":         inputs, "flow": nodes, "parallel": map[string]any{"type": "integer", "minimum": 0}, "scorer": str, "comparator": str,
		"selection": map[string]any{"enum": []string{"best", "first"}}, "timeoutMs": map[string]any{"type": "integer", "minimum": 0, "maximum": 86400000},
		"reserveMs": reserveSchema,
		"settleMs":  map[string]any{"type": "integer", "minimum": 0, "maximum": 86400000}, "timeoutFailure": str, "advanceOn": stringList, "fallback": nodes, "fallbackOn": stringList,
	}, "flow")
	candidateSchema["oneOf"] = []any{map[string]any{"required": []string{"items"}, "not": map[string]any{"required": []string{"groups"}}}, map[string]any{"required": []string{"groups"}, "not": map[string]any{"required": []string{"items"}}}}
	candidateSchema["if"] = map[string]any{"required": []string{"groups"}}
	candidateSchema["then"] = map[string]any{"properties": map[string]any{"selection": map[string]any{"const": "best"}, "settleMs": map[string]any{"const": 0}, "advanceOn": map[string]any{"maxItems": 0}}}
	candidateSchema["allOf"] = []any{map[string]any{"if": map[string]any{"required": []string{"includeCurrent"}, "properties": map[string]any{"includeCurrent": map[string]any{"const": true}}}, "then": map[string]any{"not": map[string]any{"required": []string{"groups"}}, "properties": map[string]any{"selection": map[string]any{"const": "best"}}}, "else": map[string]any{"properties": map[string]any{"items": map[string]any{"minItems": 1}}}}}
	variants = append(variants, action("sequence", nodes, nil), action("fail", str, nil), action("call", str, map[string]any{"with": with}),
		action("switch", obj(map[string]any{"value": map[string]any{}, "cases": map[string]any{"type": "object", "additionalProperties": nodes}, "otherwise": nodes}, "value", "cases", "otherwise"), nil),
		action("regions", obj(map[string]any{"items": expressionSchema(map[string]any{"type": "array", "items": str, "minItems": 1}), "flow": nodes, "reset": map[string]any{"type": "boolean"}}, "items", "flow"), nil), action("candidates", candidateSchema, nil))

	root := obj(map[string]any{"schemaVersion": map[string]any{"const": 1}, "id": str, "parameters": with, "toolDefaults": map[string]any{"type": "object", "additionalProperties": with}, "flow": nodes, "subflows": map[string]any{"type": "object", "additionalProperties": obj(map[string]any{"inputs": inputs, "flow": nodes}, "flow")}}, "schemaVersion", "id")
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$defs"] = map[string]any{"reference": map[string]any{"oneOf": refVariants}, "node": map[string]any{"oneOf": variants}}
	return root
}

// sortStrings 职责：稳定 Schema 输出顺序；使用：工具目录；边界：不影响业务分支顺序。
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// fieldTypeSchema 职责：集中生成字段枚举及范围规范；使用：完整类型和默认覆盖层；边界：验证交给 JSON Schema 引擎，不维护第二套字段规则。
func fieldTypeSchema(f reflect.StructField) map[string]any {
	child := typeSchema(f.Type)
	if unit := f.Tag.Get("unit"); unit != "" {
		child["x-unit"] = unit
	}
	fieldType := f.Type
	for fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}
	if values := f.Tag.Get("enum"); values != "" {
		enum := []any{}
		for _, v := range strings.Split(values, ",") {
			var literal any = v
			kind := fieldType.Kind()
			if kind == reflect.Bool {
				if parsed, e := strconv.ParseBool(v); e == nil {
					literal = parsed
				}
			} else if kind >= reflect.Int && kind <= reflect.Float64 {
				literal = json.Number(v)
			}
			x, e := bindValue(literal, f.Type, fieldName(f))
			if e == nil {
				enum = append(enum, x.Interface())
			} else {
				enum = append(enum, v)
			}
		}
		child["enum"] = enum
	}
	// 数组长度与元素数值独立校验；静态列表即使与动态 $state 参数混用也能在编译时定位错误。
	if items, ok := child["items"].(map[string]any); ok {
		bounds := map[string]any{}
		if min := f.Tag.Get("itemMin"); min != "" {
			bounds["minimum"] = json.Number(min)
		}
		if max := f.Tag.Get("itemMax"); max != "" {
			bounds["maximum"] = json.Number(max)
		}
		if len(bounds) > 0 {
			items["allOf"] = []any{bounds}
		}
	}
	constraints := map[string]any{}
	for _, tag := range []string{"min", "max"} {
		if v := f.Tag.Get(tag); v != "" {
			key := "minimum"
			if tag == "max" {
				key = "maximum"
			}
			kind := fieldType.Kind()
			if kind == reflect.String {
				key = "minLength"
				if tag == "max" {
					key = "maxLength"
				}
			} else if kind == reflect.Slice || kind == reflect.Array {
				key = "minItems"
				if tag == "max" {
					key = "maxItems"
				}
			} else if kind == reflect.Map {
				key = "minProperties"
				if tag == "max" {
					key = "maxProperties"
				}
			}
			constraints[key] = json.Number(v)
		}
	}
	if len(constraints) > 0 {
		child["allOf"] = []any{constraints}
	}
	return child
}
