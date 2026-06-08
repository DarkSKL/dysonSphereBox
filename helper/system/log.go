package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"DysonSphereBox/core"
	systemModel "DysonSphereBox/model/system"

	"gorm.io/datatypes"
)

const (
	defaultLogNamespace = "system"
	defaultLogModule    = "general"
	defaultLogEvent     = "operation"
)

// LogEntry 描述一次需要持久化的重要操作。
// 使用：服务层或 helper 层把业务语义转换成 LogEntry，再交给 RecordLog 统一入库。
type LogEntry struct {
	// Namespace 是日志所属业务域，例如 system 或 modDependence。
	Namespace string
	// Module 是业务域下的具体模块，例如 configuration、installer。
	Module string
	// Event 是可稳定检索的事件名，例如 setConfig、installPackage。
	Event string
	// Level 是日志重要等级；为 0 时默认按普通信息处理。
	Level systemModel.LogLevel
	// Type 是日志业务分类；为 0 时默认按普通操作处理。
	Type systemModel.LogType
	// Content 是面向人阅读的日志摘要。
	Content string
	// Metadata 是结构化扩展信息，必须能被 json.Marshal 序列化。
	Metadata map[string]interface{}
	// Error 是失败详情；成功日志保持为空。
	Error string
}

// RecordLog 将重要操作记录到 SQLite。
// 边界：调用方若是业务流程埋点，可使用 RecordLogSilently；需要前端感知失败时才直接处理返回错误。
func RecordLog(entry LogEntry) error {
	if core.GLOBAL.DB == nil {
		return errors.New("database is not initialized")
	}

	now := time.Now()
	logRecord := systemModel.Log{
		Namespace: normalizeLogDimension(entry.Namespace, defaultLogNamespace),
		Module:    normalizeLogDimension(entry.Module, defaultLogModule),
		Event:     normalizeLogDimension(entry.Event, defaultLogEvent),
		Level:     normalizeLogLevel(entry.Level),
		Type:      normalizeLogType(entry.Type),
		Content:   entry.Content,
		Error:     entry.Error,
		DateTime:  &now,
	}

	metadata, err := marshalMetadata(entry.Metadata)
	if err != nil {
		return err
	}
	logRecord.Metadata = metadata

	if err := core.GLOBAL.DB.Create(&logRecord).Error; err != nil {
		return fmt.Errorf("create system log: %w", err)
	}

	return nil
}

// RecordLogSilently 静默记录日志，适合业务主流程完成后的审计埋点。
// 边界：日志失败不会阻断原操作；若需要排查，可临时改用 RecordLog 获取错误。
func RecordLogSilently(entry LogEntry) {
	_ = RecordLog(entry)
}

// RecordOperation 记录普通操作成功日志。
func RecordOperation(namespace string, module string, event string, content string, metadata map[string]interface{}) {
	RecordLogSilently(LogEntry{
		Namespace: namespace,
		Module:    module,
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata:  metadata,
	})
}

// RecordFailure 记录操作失败日志。
func RecordFailure(namespace string, module string, event string, content string, err error, metadata map[string]interface{}) {
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}

	RecordLogSilently(LogEntry{
		Namespace: namespace,
		Module:    module,
		Event:     event,
		Level:     systemModel.LogLevelError,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata:  metadata,
		Error:     errorText,
	})
}

func normalizeLogDimension(value string, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func normalizeLogLevel(level systemModel.LogLevel) systemModel.LogLevel {
	if level == 0 {
		return systemModel.LogLevelInfo
	}

	return level
}

func normalizeLogType(logType systemModel.LogType) systemModel.LogType {
	if logType == 0 {
		return systemModel.LogTypeOperation
	}

	return logType
}

func marshalMetadata(metadata map[string]interface{}) (datatypes.JSON, error) {
	if len(metadata) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal system log metadata: %w", err)
	}

	return datatypes.JSON(data), nil
}
