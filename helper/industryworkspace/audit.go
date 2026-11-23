package industryworkspace

import (
	"DysonSphereBox/constant/logging"
	systemhelper "DysonSphereBox/helper/system"
)

// RecordOperation 记录本机规划的操作结果，日志只保留规模、状态与任务编号。
func RecordOperation(event, action string, data map[string]interface{}) {
	systemhelper.RecordLogSilently(systemhelper.LogEntry{
		Action: action, Namespace: "industryplan.workspace." + event, Source: "backend.industryplan",
		Level: logging.Info, Handler: logging.User, Reason: "玩家在跨星球产业线工作区执行操作", Data: data,
	})
}
