package server

// 文件职责：定义服务器连接测试返回给系统服务和前端绑定的结果 DTO。
// 使用：service/system/server.go 通过 Wails 服务返回 ProbeResult。
// 边界：只描述测试结论，不包含响应体原文或账号数据。

// ProbeResult 是服务器连接测试的结果。
// 职责：承载地址是否可用、响应耗时与面向用户的说明文字。
// 使用：全局设置页“测试连接”按钮展示测试结论。
// 边界：OK 只表示地址返回了账号服务的统一响应结构，不代表登录等接口权限配置正确。
type ProbeResult struct {
	// OK 职责：标记目标地址是否为可用的账号服务；使用：前端决定成功或失败样式；边界：网络失败和结构不符都为 false。
	OK bool `json:"ok"`
	// LatencyMs 职责：记录从发出请求到读完响应的耗时毫秒数；使用：前端展示连接耗时；边界：失败时为已等待的时间。
	LatencyMs int64 `json:"latencyMs"`
	// Message 职责：保存面向用户的测试结论；使用：前端直接展示；边界：不承载机器可读状态，判断应使用 OK。
	Message string `json:"message"`
}
