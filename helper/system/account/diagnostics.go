package account

// 文件职责：补齐账号接口中传输层无法观察的业务与解析错误。使用：账号请求和会话持久化边界；边界：不记录密码、token、资料正文或成功读取，网络传输错误由共享采集器记录。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
)

// recordRequestFailure 职责：记录接口、失败阶段与原始错误链；使用：账号 helper 的错误分支；边界：接口路径是静态契约，不包含请求内容，不能替代 HTTP 传输证据。
func recordRequestFailure(method, path, stage string, status, code int, err error) {
	diagnostics.Default.Record(logging.Entry{Action: "账号接口处理失败", Level: logging.Error, Source: "backend.account", Reason: "账号接口的业务校验、解析或会话持久化返回错误", Namespace: "system.account.requestFailed", Handler: logging.Unknown, Data: map[string]interface{}{"method": method, "path": path, "stage": stage, "statusCode": status, "businessCode": code, "result": "failed", "error": diagnostics.ErrorData(err)}})
}

// RecordStateChange 职责：保留账号操作完成后的关键状态；使用：服务确认结果后调用；边界：不记录成功读取、账号资料或认证凭据。
func RecordStateChange(event, action string) {
	diagnostics.Default.Record(logging.Entry{Action: action, Level: logging.Info, Source: "backend.account", Reason: "账号服务确认操作完成", Namespace: "system.account." + event, Handler: logging.User, Data: map[string]interface{}{"result": "success"}})
}
