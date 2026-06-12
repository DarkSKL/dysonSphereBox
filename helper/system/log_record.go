package system

// 文件职责：提供系统操作日志的统一写入能力。
// 使用：service/system、helper/system 和各业务审计 helper 在记录重要操作时调用 RecordLog 或静默变体。
// 边界：不查询日志、不删除日志、不决定某个业务流程是否必须记录日志。

import (
	"errors"
	"fmt"
	"time"

	"DysonSphereBox/core"
	systemModel "DysonSphereBox/model/system"
)

// RecordLog 将重要操作记录到 SQLite。
// 职责：归一化日志维度、序列化 metadata，并写入系统日志表。
// 使用：需要感知日志写入失败的服务方法或 helper 直接调用。
// 边界：调用方若是业务流程埋点，可使用 RecordLogSilently；需要前端感知失败时才直接处理返回错误。
// 逻辑流：1. 校验数据库 => 2. 归一化 namespace/module/event/level/type => 3. 序列化 metadata => 4. 写入日志表。
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
// 职责：调用 RecordLog 并吞掉日志写入错误。
// 使用：业务主流程已完成，不希望审计失败影响用户操作结果时调用。
// 边界：日志失败不会阻断原操作；若需要排查，可临时改用 RecordLog 获取错误。
func RecordLogSilently(entry LogEntry) {
	_ = RecordLog(entry)
}

// RecordOperation 记录普通操作成功日志。
// 职责：为成功操作构造 info/operation 级别日志并静默写入。
// 使用：系统配置、安装、更新等 helper 在业务成功后调用。
// 边界：不返回写入错误，重要失败路径应使用 RecordFailure。
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
// 职责：为失败操作构造 error/operation 级别日志并静默写入。
// 使用：服务或 helper 捕获业务错误后调用。
// 边界：err 为空时 Error 字段为空，但仍按失败级别记录。
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
