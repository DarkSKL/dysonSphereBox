package contract

// 文件职责：定义无头服务器准备、启动和停止动作的返回契约。
// 使用：HeadlessService 的 Prepare/Start/Stop 方法返回这些 DTO 给前端。
// 边界：只描述动作结果，不执行文件写入、不启动或停止进程、不刷新状态。

// HeadlessPrepareResult 表示无头服务器环境准备动作的执行结果。
// 职责：返回基础环境准备动作的写入结果和最新状态。
// 使用：HeadlessService.PrepareEnvironment 调用后交给前端展示。
// 边界：只准备基础环境，不自动安装 Nebula 整合包。
type HeadlessPrepareResult struct {
	// GameDir 职责：记录准备目标游戏目录；使用：审计日志和前端展示；边界：准备失败时不会返回该结构。
	GameDir string `json:"gameDir"`
	// Changed 职责：表示是否写入或创建了基础文件；使用：前端区分“已准备”和“已存在”；边界：依赖缺失不影响该字段。
	Changed bool `json:"changed"`
	// Message 职责：保存用户可读准备结果；使用：前端提示和审计日志；边界：不用于业务分支。
	Message string `json:"message"`
	// WrittenPath 职责：记录本次写入的路径列表；使用：日志排查准备动作；边界：只包含基础文件，不包含依赖安装文件。
	WrittenPath []string `json:"writtenPath"`
	// Status 职责：保存准备后的最新环境状态；使用：前端刷新状态；边界：只代表准备完成时刻。
	Status HeadlessEnvironmentStatus `json:"status"`
}

// HeadlessStartResult 表示无头服务器启动动作的执行结果。
// 职责：返回启动、跳过或启动后状态检测结果。
// 使用：HeadlessService.StartEnvironment 调用后展示启动反馈。
// 边界：环境未就绪时不会启动游戏进程；已运行时返回跳过而不是重复启动。
type HeadlessStartResult struct {
	// GameDir 职责：记录启动目标游戏目录；使用：审计日志和前端展示；边界：未就绪跳过时仍保留准备流程返回的目录。
	GameDir string `json:"gameDir"`
	// Started 职责：表示本次调用是否新启动进程；使用：前端状态提示；边界：进程已存在时为 false。
	Started bool `json:"started"`
	// Skipped 职责：表示启动被安全跳过；使用：区分已运行或环境未就绪；边界：跳过不代表错误。
	Skipped bool `json:"skipped"`
	// ProcessID 职责：记录相关进程 PID；使用：前端展示和停止流程参考；边界：未启动时可能为 0。
	ProcessID int `json:"processId"`
	// Message 职责：保存用户可读启动结果；使用：前端提示和审计日志；边界：不用于业务判断。
	Message string `json:"message"`
	// StartCommandText 职责：保存启动命令文本；使用：前端展示启动参数；边界：不作为实际执行入口。
	StartCommandText string `json:"startCommandText"`
	// Status 职责：保存启动后的最新环境状态；使用：前端刷新进程和端口状态；边界：只代表启动后短暂检测时刻。
	Status HeadlessEnvironmentStatus `json:"status"`
}

// HeadlessStopResult 表示无头服务器停止动作的执行结果。
// 职责：返回停止、跳过或停止后状态检测结果。
// 使用：HeadlessService.StopEnvironment 调用后展示停止反馈。
// 边界：只停止确认带有 -nebula-server 参数的 DSPGAME.exe 进程。
type HeadlessStopResult struct {
	// GameDir 职责：记录停止目标游戏目录；使用：审计日志和前端展示；边界：不会用于停止非匹配目录进程。
	GameDir string `json:"gameDir"`
	// Stopped 职责：表示本次调用是否成功发送并确认停止；使用：前端状态提示；边界：发送命令后仍运行时为 false。
	Stopped bool `json:"stopped"`
	// Skipped 职责：表示没有可停止进程而跳过；使用：前端展示无操作结果；边界：跳过不代表错误。
	Skipped bool `json:"skipped"`
	// ProcessID 职责：记录被停止或尝试停止的进程 PID；使用：审计日志定位；边界：未运行时为 0。
	ProcessID int `json:"processId"`
	// HostPort 职责：记录停止目标端口；使用：日志说明端口释放对象；边界：端口状态以 Status 为准。
	HostPort int `json:"hostPort"`
	// Message 职责：保存用户可读停止结果；使用：前端提示和审计日志；边界：不用于业务判断。
	Message string `json:"message"`
	// Status 职责：保存停止后的最新环境状态；使用：前端刷新进程和端口状态；边界：只代表停止后短暂检测时刻。
	Status HeadlessEnvironmentStatus `json:"status"`
}
