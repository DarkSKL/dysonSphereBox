package workflow

// 文件职责：定义带克隆边界的领域产物键；使用：由算法适配层注册真实领域类型；边界：内核不解释领域对象或暴露其内部引用。

import (
	"fmt"
	"reflect"
)

// ArtifactOptions 职责：声明产物作用域和版本类别；使用：创建键；边界：诊断产物不推进内容版本。
type ArtifactOptions struct {
	// Regional 职责：限定区域产物；使用：区域工具；边界：根步骤需声明 AllRegions 才可读取。
	Regional bool
	// Diagnostic 职责：标记报告等非内容产物；使用：验证与编码；边界：不能作为领域内容修改的替身。
	Diagnostic bool
}

// Artifact 职责：提供不可伪造的产物契约引用；使用：工具权限和依赖声明；边界：不提供领域值。
type Artifact interface{ artifact() *artifactDef }

// artifactDefinition 职责：安全解包封闭产物键；使用：注册边界；边界：拒绝接口中的 typed nil 键。
func artifactDefinition(a Artifact) (*artifactDef, error) {
	if a == nil {
		return nil, fmt.Errorf("nil artifact key")
	}
	value := reflect.ValueOf(a)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil, fmt.Errorf("typed nil artifact key")
	}
	d := a.artifact()
	if d == nil {
		return nil, fmt.Errorf("zero artifact key")
	}
	return d, nil
}

// artifactDef 职责：保存内部类型擦除操作；使用：实例克隆和校验；边界：仅注册时构造。
type artifactDef struct {
	name    string
	typ     reflect.Type
	options ArtifactOptions
	clone   func(any) (any, error)
	size    func(any) int64
	// sharedClone 职责：声明克隆仅复制封闭不可变句柄；使用：暂存预算；边界：只能由专用构造器设置，可变产物仍须预留两份。
	sharedClone bool
}

// ArtifactKey 职责：把领域类型绑定到固定产物槽；使用：泛型 Read/Stage；边界：零值键无效。
type ArtifactKey[T any] struct{ def *artifactDef }

// artifact 职责：取得封闭描述；使用：内核契约；边界：不导出可变定义。
func (k ArtifactKey[T]) artifact() *artifactDef { return k.def }

// Ref 职责：抹除键的值类型以描述权限；使用：ToolSpec；边界：不能用字符串构造访问权限。
func (k ArtifactKey[T]) Ref() Artifact { return k }

// NewArtifactKey 职责：注册领域克隆与容量计算；使用：领域初始化；边界：克隆器须深复制可变成员且不修改输入，大小器须纯读取并保守估算保留字节，不得返回负值。
func NewArtifactKey[T any](name string, clone func(T) (T, error), size func(T) int64, options ArtifactOptions) (ArtifactKey[T], error) {
	if name == "" || clone == nil || size == nil {
		return ArtifactKey[T]{}, fmt.Errorf("artifact requires name, cloner and size")
	}
	d := &artifactDef{name: name, typ: reflect.TypeFor[T](), options: options}
	d.clone = func(v any) (any, error) {
		x, ok := v.(T)
		if !ok {
			return nil, fmt.Errorf("artifact %s type mismatch", name)
		}
		return clone(x)
	}
	d.size = func(v any) int64 { return size(v.(T)) }
	return ArtifactKey[T]{d}, nil
}

// NewImmutableArtifactKey 职责：登记可安全按值共享的封闭不可变产物；使用：领域快照注册。
// 边界：所有可达内容在发布后必须只读，公开读取须由领域接口复制；请求、可写切片和映射禁止使用。容量仍完整计入每个保留槽，仅不虚构第二份克隆图。
func NewImmutableArtifactKey[T any](name string, size func(T) int64, options ArtifactOptions) (ArtifactKey[T], error) {
	key, err := NewArtifactKey(name, func(value T) (T, error) { return value, nil }, size, options)
	if err == nil {
		key.def.sharedClone = true
	}
	return key, err
}

// safeClone 职责：把领域克隆失败转换为内核错误；使用：事务和候选复制；边界：panic 不作为可回退无解。
func safeClone(d *artifactDef, v any) (out any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("artifact %s clone panic: %v", d.name, p)
		}
	}()
	out, err = d.clone(v)
	if err != nil {
		return nil, fmt.Errorf("artifact %s clone: %w", d.name, err)
	}
	if reflect.TypeOf(out) != d.typ {
		return nil, fmt.Errorf("artifact %s clone type mismatch", d.name)
	}
	return out, nil
}

// safeSize 职责：隔离领域大小计算器故障；使用：候选预算；边界：负值及 panic 均失败。
func safeSize(d *artifactDef, v any) (n int64, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("artifact %s size panic: %v", d.name, p)
		}
	}()
	n = d.size(v)
	if n < 0 {
		return 0, fmt.Errorf("artifact %s negative size", d.name)
	}
	return
}
