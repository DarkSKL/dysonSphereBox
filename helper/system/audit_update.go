package system

// 文件职责：构造更新检查、下载准备及退出交接的审计日志。
// 使用：系统更新服务决定记录时机；边界：不把下载完成或交接成功记录成已安装。

import systemModel "DysonSphereBox/constant/logging"

// RecordUpdateCheckFailure 职责：记录检查失败；使用：更新服务捕获远端错误；边界：不重试或修改版本状态。
func RecordUpdateCheckFailure(version string, err error) {
	RecordFailure("system", "update", "checkAppUpdate", "应用更新检查失败", err, map[string]interface{}{"version": version})
}

// RecordUpdatePrepared 职责：记录新版暂存完成；使用：下载、校验和解压成功后调用；边界：此时旧程序仍在运行。
func RecordUpdatePrepared(version string, result interface{}) {
	RecordLogSilently(LogEntry{
		Namespace: "system.update.prepared", Source: "backend.update",
		Reason: "更新包已下载并校验，等待用户退出", Handler: "User",
		Level: systemModel.Info, Action: "更新准备完成，退出后安装",
		Data: map[string]interface{}{"version": version, "result": result},
	})
}

// RecordUpdateExit 职责：记录退出安装交接；使用：助手确认等待旧进程后调用；边界：此时尚未替换包体，不提前宣告安装成功。
func RecordUpdateExit(version string, result interface{}) {
	RecordLogSilently(LogEntry{
		Namespace: "system.update.handoff", Source: "backend.update",
		Reason: "更新助手已就绪，旧进程退出后替换并自动启动", Handler: "User",
		Level: systemModel.Info, Action: "退出并安装更新",
		Data: map[string]interface{}{"version": version, "result": result},
	})
}

// RecordUpdateSelfFailure 职责：记录更新准备或交接失败；使用：服务捕获错误后调用；边界：不回滚文件，具体恢复由安装助手负责。
func RecordUpdateSelfFailure(version string, err error) {
	RecordFailure("system", "update", "updateAppSelf", "应用自更新失败", err, map[string]interface{}{"version": version})
}
