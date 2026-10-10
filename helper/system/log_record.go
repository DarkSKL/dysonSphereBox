package system

// 文件职责：把业务事实交给统一诊断采集器。使用：各业务审计 helper；边界：不访问数据库，落盘失败不改变业务结果但可查询健康状态。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
	"errors"
)

// RecordLog 职责：写入统一日志并返回落盘错误；使用：主动诊断；边界：错误内容归入 data.error。
func RecordLog(entry LogEntry) error { return diagnostics.Default.Write(toEntry(entry)) }

// toEntry 职责：转换业务日志为统一契约；使用：内部写入；边界：不丢弃原始错误。
func toEntry(entry LogEntry) logging.Entry {
	data := diagnostics.CleanData(entry.Data)
	if entry.Error != "" {
		data["error"] = diagnostics.ErrorData(errors.New(entry.Error))
	}
	return logging.Entry{Action: entry.Action, Level: entry.Level, Source: entry.Source, Reason: entry.Reason, Namespace: entry.Namespace, Data: data, Handler: entry.Handler, OperationID: entry.OperationID, SessionID: entry.SessionID}
}

// RecordLogSilently 职责：不改变业务结果地记录事实；使用：业务埋点；边界：不再吞错，失败输出 stderr 并维护健康状态。
func RecordLogSilently(entry LogEntry) { diagnostics.Default.Record(toEntry(entry)) }

// RecordOperation 职责：记录明确完成的普通操作；使用：服务层完成后；边界：具体结果依据业务验证，不能代替就绪检查。
func RecordOperation(namespace, module, event, action string, data map[string]interface{}) {
	RecordLogSilently(LogEntry{Action: action, Level: logging.Info, Namespace: namespace + "." + module + "." + event, Source: "backend." + module, Reason: "服务调用完成，结果由业务层返回", Handler: logging.User, Data: data})
}

// RecordFailure 职责：记录业务失败的原始错误链；使用：服务捕获返回错误；边界：未知发起者标记 Unknown，不推测根因。
func RecordFailure(namespace, module, event, action string, err error, data map[string]interface{}) {
	clean := diagnostics.CleanData(data)
	clean["result"], clean["error"] = "failed", diagnostics.ErrorData(err)
	RecordLogSilently(LogEntry{Action: action, Level: logging.Error, Namespace: namespace + "." + module + "." + event, Source: "backend." + module, Reason: "业务调用返回错误，原始错误见 data.error", Handler: logging.Unknown, Data: clean})
}
