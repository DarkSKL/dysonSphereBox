package utils

// 文件职责：提供外部 API 路径模板的字段占位符替换工具。
// 使用：apiDefinition 中定义 `/api/{{FieldName}}` 风格路径后，由 service 或 helper 在发起请求前传入参数结构体生成真实路径。
// 边界：只处理路径模板中的字段替换，不拼接 query，不发起网络请求，不依赖业务包。

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// apiPathFieldExpression 匹配 API 路径模板中的字段占位符。
// 职责：识别 `{{FieldName}}` 格式的路径动态段。
// 使用：ResolveAPIPath 遍历模板占位符时复用。
// 边界：只接受字母、数字和下划线字段名；其它模板文本保持原样。
var apiPathFieldExpression = regexp.MustCompile(`\{\{([A-Za-z0-9_]+)\}\}`)

// ResolveAPIPath 根据参数结构体解析 API 路径模板。
// 职责：将 `/api/{{UserID}}/{{TestID}}` 中的占位符替换为 params 对象同名导出字段的路径安全值。
// 使用：service 或 helper 引用 apiDefinition 中的路径常量后，在发起请求前调用。
// 边界：只读取结构体或结构体指针的导出字段；缺少字段、nil 指针或无法转成字符串时返回错误。
// 逻辑流：1. 扫描模板占位符 => 2. 解引用参数对象 => 3. 逐个读取同名导出字段 => 4. 转为字符串并 PathEscape => 5. 返回替换后的路径。
func ResolveAPIPath(pathTemplate string, params interface{}) (string, error) {
	matches := apiPathFieldExpression.FindAllStringSubmatch(pathTemplate, -1)
	if len(matches) == 0 {
		return pathTemplate, nil
	}

	value, err := dereferenceAPIPathParams(params)
	if err != nil {
		return "", err
	}
	if value.Kind() != reflect.Struct {
		return "", fmt.Errorf("api path params must be a struct or struct pointer")
	}

	resolvedPath := pathTemplate
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		fieldName := match[1]
		fieldValue, err := lookupAPIPathField(value, fieldName)
		if err != nil {
			return "", err
		}

		stringValue, err := stringifyAPIPathField(fieldName, fieldValue)
		if err != nil {
			return "", err
		}
		resolvedPath = strings.ReplaceAll(resolvedPath, match[0], url.PathEscape(stringValue))
	}

	return resolvedPath, nil
}

// dereferenceAPIPathParams 解引用 API 路径参数对象。
// 职责：把 interface 或指针形式的参数还原为可读取的 reflect.Value。
// 使用：ResolveAPIPath 读取字段前调用。
// 边界：nil 参数或 nil 指针返回错误，不创建默认值。
func dereferenceAPIPathParams(params interface{}) (reflect.Value, error) {
	if params == nil {
		return reflect.Value{}, fmt.Errorf("api path params cannot be nil")
	}

	value := reflect.ValueOf(params)
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("api path params cannot be nil")
		}
		value = value.Elem()
	}

	return value, nil
}

// lookupAPIPathField 读取 API 路径参数中的同名导出字段。
// 职责：按模板占位符名称从结构体中定位字段值。
// 使用：ResolveAPIPath 逐个替换占位符时调用。
// 边界：字段不存在或未导出时返回错误，避免静默生成错误 URL。
func lookupAPIPathField(value reflect.Value, fieldName string) (reflect.Value, error) {
	field, ok := value.Type().FieldByName(fieldName)
	if !ok {
		return reflect.Value{}, fmt.Errorf("api path param field %q not found", fieldName)
	}
	if field.PkgPath != "" {
		return reflect.Value{}, fmt.Errorf("api path param field %q is not exported", fieldName)
	}

	return value.FieldByIndex(field.Index), nil
}

// stringifyAPIPathField 将字段值转换为路径段文本。
// 职责：支持字符串、数字、布尔值和 fmt.Stringer 值进入 API 路径模板。
// 使用：ResolveAPIPath 替换占位符前调用。
// 边界：nil 指针、结构体、切片、map 等非路径段基础值返回错误。
func stringifyAPIPathField(fieldName string, value reflect.Value) (string, error) {
	if !value.IsValid() {
		return "", fmt.Errorf("api path param field %q is invalid", fieldName)
	}

	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", fmt.Errorf("api path param field %q is nil", fieldName)
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if stringer, ok := value.Interface().(fmt.Stringer); ok {
			return stringer.String(), nil
		}
	}

	switch value.Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(value.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, value.Type().Bits()), nil
	default:
		return "", fmt.Errorf("api path param field %q cannot be converted to string", fieldName)
	}
}
