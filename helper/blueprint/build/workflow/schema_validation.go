package workflow

// 文件职责：通过成熟 JSON Schema 引擎验证参数和工作流结构；使用：注册、编译和动态绑定；边界：全部资源只存在内存，外部 URL 与文件加载一律拒绝。

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// denySchemaLoader 职责：关闭隐式网络与文件依赖；使用：所有 Schema 编译器；边界：标准元 Schema 由库自带嵌入资源处理。
type denySchemaLoader struct{}

// Load 职责：拒绝外部资源访问；使用：jsonschema 编译器；边界：不会尝试磁盘或网络回退。
func (denySchemaLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema resource forbidden: %s", url)
}

// argumentSchemas 职责：按真实 Go 类型缓存只读规范；使用：每次独立参数绑定；边界：不缓存参数值或领域对象。
var argumentSchemas sync.Map

// fieldSchemas 职责：缓存同一真实字段约束；使用：部分默认值校验；边界：键只包含类型和静态标签，不含执行值。
var fieldSchemas sync.Map

// fieldSchemaKey 职责：标识不可变字段规范；使用：缓存；边界：字段名称不参与值校验规则。
type fieldSchemaKey struct {
	typ reflect.Type
	tag reflect.StructTag
}

// schemaForField 职责：复用完整参数中的相同字段规范；使用：默认值与覆盖层；边界：不重新实现枚举和范围比较。
func schemaForField(f reflect.StructField) (*jsonschema.Schema, error) {
	key := fieldSchemaKey{f.Type, f.Tag}
	if value, ok := fieldSchemas.Load(key); ok {
		return value.(*jsonschema.Schema), nil
	}
	schema, e := compileSchema(fieldTypeSchema(f))
	if e != nil {
		return nil, e
	}
	actual, _ := fieldSchemas.LoadOrStore(key, schema)
	return actual.(*jsonschema.Schema), nil
}

// compileSchema 职责：在内存编译规范；使用：类型和文档 Schema；边界：错误规范本身也拒绝。
func compileSchema(document any) (*jsonschema.Schema, error) {
	normalized, e := transport(document)
	if e != nil {
		return nil, e
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(denySchemaLoader{})
	compiler.DefaultDraft(jsonschema.Draft2020)
	const location = "https://workflow.invalid/schema.json"
	if e = compiler.AddResource(location, normalized); e != nil {
		return nil, e
	}
	return compiler.Compile(location)
}

// schemaForType 职责：取得真实参数类型的缓存规范；使用：bindValue；边界：数字精度及字段语义仍由严格 hook/规则补齐。
func schemaForType(t reflect.Type) (*jsonschema.Schema, error) {
	if value, ok := argumentSchemas.Load(t); ok {
		return value.(*jsonschema.Schema), nil
	}
	schema, e := compileSchema(typeSchema(t))
	if e != nil {
		return nil, e
	}
	actual, _ := argumentSchemas.LoadOrStore(t, schema)
	return actual.(*jsonschema.Schema), nil
}

// compileDocumentSchema 职责：验证并缓存唯一工作流格式；使用：NewRegistry；边界：生成 Schema 必须先通过成熟编译器。
func compileDocumentSchema(r *Registry) (*jsonschema.Schema, error) {
	data, e := GenerateSchema(r)
	if e != nil {
		return nil, e
	}
	value, e := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	return compileSchema(value)
}
