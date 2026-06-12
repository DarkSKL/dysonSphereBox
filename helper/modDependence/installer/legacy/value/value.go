package value

// 文件职责：提供旧官方整合包 DTO 和 legacy 模型可空字段的读取与克隆工具。
// 使用：legacy 安装编排、artifact 安装、store 转换和 cleanup 清理结果需要读取可空字段时调用。
// 边界：只处理基础指针值，不访问数据库、不读写文件、不做业务默认值推断。

import "strings"

// String 读取可空字符串指针并清理首尾空白。
// 职责：统一旧 DTO 与 legacy 模型可空字符串的读取方式。
// 使用：安装结果展示、目录命名、下载校验和卸载结果构造时调用。
// 边界：nil 返回空字符串，不做默认值填充。
func String(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

// CloneString 克隆可空字符串指针。
// 职责：避免 DTO 指针直接被持久化模型共享。
// 使用：DTO 转模型时复制字符串字段。
// 边界：nil 保持 nil，不修改原始值。
func CloneString(value *string) *string {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}

// CloneInt 克隆可空 int 指针。
// 职责：避免 DTO 指针直接被持久化模型共享。
// 使用：DTO 转模型时复制状态字段。
// 边界：nil 保持 nil，不做零值替换。
func CloneInt(value *int) *int {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}

// CloneInt64 克隆可空 int64 指针。
// 职责：避免 DTO 指针直接被持久化模型共享。
// 使用：DTO 转模型时复制大小、下载量和可信度字段。
// 边界：nil 保持 nil，不做零值替换。
func CloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}
