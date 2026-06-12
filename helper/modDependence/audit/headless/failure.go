package headless

// 文件职责：记录无头服务器服务方法失败审计日志。
// 使用：service/modDependence 在无头服务器状态、设置、准备、启动或停止流程捕获错误后调用。
// 边界：只记录日志，不吞掉错误，不修改进程或配置文件状态。

import systemHelper "DysonSphereBox/helper/system"

// RecordHeadlessFailure 记录无头服务器环境检测或准备失败事件。
// 职责：统一写入无头服务器状态、设置、准备、启动或停止失败日志。
// 使用：service/modDependence 在无头服务器服务方法捕获错误后调用。
// 边界：只记录日志，不吞掉错误，不修改进程或配置文件状态。
func RecordHeadlessFailure(event string, content string, err error) {
	systemHelper.RecordFailure("modDependence", "headless", event, content, err, nil)
}
