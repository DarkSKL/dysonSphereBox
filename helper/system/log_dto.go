package system

// 文件职责：转换统一日志索引为外部契约。使用：日志服务查询；边界：单条损坏数据仍可查看，不能阻断整页。

import (
	"DysonSphereBox/constant/logging"
	systemModel "DysonSphereBox/model/system"
	"encoding/json"
	"time"
)

// ToLogEntries 职责：批量转换持久化记录；使用：日志列表；边界：JSON 解码失败保留错误证据。
func ToLogEntries(items []systemModel.LogEntry) []logging.Entry {
	out := make([]logging.Entry, 0, len(items))
	for _, item := range items {
		data := map[string]interface{}{}
		if err := json.Unmarshal(item.Data, &data); err != nil {
			data = map[string]interface{}{"decodeError": err.Error()}
		}
		out = append(out, logging.Entry{ID: item.ID, SchemaVersion: item.SchemaVersion, Action: item.Action, Level: item.Level, Source: item.Source, Reason: item.Reason, Namespace: item.Namespace, Data: data, Time: item.EventTime.Format(time.RFC3339Nano), Handler: item.Handler, SessionID: item.SessionID, OperationID: item.OperationID, AppVersion: item.AppVersion})
	}
	return out
}
