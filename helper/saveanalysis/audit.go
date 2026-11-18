package saveanalysis

// 文件职责：构建存档分析操作日志。
// 使用：服务层决定授权、撤销和解析结果的记录时机；边界：不记录存档内容、玩家身份或文件绝对路径。

import (
	"DysonSphereBox/constant/logging"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordEvent 职责：写入只读分析操作结果；使用：服务层关键操作；边界：日志失败不改变分析结果。
func RecordEvent(event, action string, err error) {
	if err != nil {
		systemHelper.RecordFailure("environment", "saveAnalysis", event, action, err, nil)
		return
	}
	systemHelper.RecordLogSilently(systemHelper.LogEntry{Action: action, Level: logging.Info, Namespace: "environment.saveAnalysis." + event, Source: "backend.saveAnalysis", Reason: "玩家发起只读存档分析操作", Handler: logging.User})
}
