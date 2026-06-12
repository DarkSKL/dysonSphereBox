package system

// 文件职责：维护系统日志写入时的维度和枚举默认值。
// 使用：RecordLog 写入日志模型前调用这些归一化函数。
// 边界：只提供兜底值，不校验业务命名规范、不裁剪非空字符串。

import systemModel "DysonSphereBox/model/system"

const (
	// defaultLogNamespace 职责：定义日志默认命名空间；使用：RecordLog 在调用方未传 namespace 时回退；边界：只用于兜底，不替代业务域命名。
	defaultLogNamespace = "system"
	// defaultLogModule 职责：定义日志默认模块名；使用：RecordLog 在调用方未传 module 时回退；边界：只用于兜底，不替代具体模块命名。
	defaultLogModule = "general"
	// defaultLogEvent 职责：定义日志默认事件名；使用：RecordLog 在调用方未传 event 时回退；边界：只用于兜底，不替代稳定业务事件。
	defaultLogEvent = "operation"
)

// normalizeLogDimension 归一化日志维度值。
// 职责：为空的日志维度提供兜底值。
// 使用：RecordLog 写入 namespace、module、event 前调用。
// 边界：不会裁剪非空字符串，也不校验命名规范。
func normalizeLogDimension(value string, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// normalizeLogLevel 归一化日志等级。
// 职责：为空等级提供默认 info 级别。
// 使用：RecordLog 写入等级前调用。
// 边界：非 0 值原样保留，由模型层枚举约束语义。
func normalizeLogLevel(level systemModel.LogLevel) systemModel.LogLevel {
	if level == 0 {
		return systemModel.LogLevelInfo
	}

	return level
}

// normalizeLogType 归一化日志类型。
// 职责：为空类型提供默认 operation 类型。
// 使用：RecordLog 写入类型前调用。
// 边界：非 0 值原样保留，由模型层枚举约束语义。
func normalizeLogType(logType systemModel.LogType) systemModel.LogType {
	if logType == 0 {
		return systemModel.LogTypeOperation
	}

	return logType
}
