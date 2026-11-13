package r2modman

// 文件职责：构造 r2modman 托管的重要操作日志。
// 使用：环境服务在选择、解除托管和修改 Profile 后调用。
// 边界：只记录结果，不执行配置保存或 Profile 文件操作。

import (
	"DysonSphereBox/constant/logging"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordOperation 职责：记录直接托管的目标、包及结果；使用：服务层操作完成时；边界：日志失败不改变已提交的文件结果。
func RecordOperation(action, path, name string, err error) {
	data := map[string]interface{}{"profilePath": path, "packageName": name, "action": action}
	if err != nil {
		systemHelper.RecordFailure("environment", "r2modman", action, "r2modman Profile 操作失败", err, data)
		return
	}
	data["result"] = "success"
	systemHelper.RecordLogSilently(systemHelper.LogEntry{Action: "r2modman Profile 操作完成", Level: logging.Info, Source: "backend.environment.r2modman", Namespace: "environment.r2modman." + action, Reason: "用户在资源页执行 Profile 托管操作", Handler: logging.User, Data: data})
}
