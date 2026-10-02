package contract

// 文件职责：定义无头服务器环境聚合检测结果契约。
// 使用：headless helper 构造 HeadlessEnvironmentStatus，service/modDependence 返回给开服页。
// 边界：DTO 本身只读，不读取文件、不启动进程、不检查端口、不修改 Nebula 配置。

import bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"

// HeadlessEnvironmentStatus 表示开服所需本地环境的聚合检测结果。
// 职责：聚合 BepInEx、Nebula 依赖、steam_appid、进程、端口、网络和启动命令状态。
// 使用：开服页状态刷新、准备环境、启动停止后统一返回。
// 边界：该状态只读，不会修改文件；实时进程和网络状态只代表本次采样。
type HeadlessEnvironmentStatus struct {
	// UptimeSeconds 职责：记录真实进程运行秒数；使用：控制台运行时长；边界：创建时间未知或未运行时为零。
	UptimeSeconds int64 `json:"uptimeSeconds"`
	// GameDir 职责：记录当前游戏目录；使用：前端展示和审计日志定位；边界：未配置状态下为空。
	GameDir string `json:"gameDir"`
	// PluginRoot 职责：记录 BepInEx 插件根目录；使用：依赖检测和前端展示；边界：目录可能尚未创建。
	PluginRoot string `json:"pluginRoot"`
	// BepInExStatus 职责：嵌入 BepInEx 前置环境检测结果；使用：判断开服是否被前置框架阻塞；边界：不触发安装。
	BepInExStatus bepinexContract.BepInExStatus `json:"bepInExStatus"`
	// Dependencies 职责：保存 Nebula 开服依赖状态列表；使用：前端展示缺失项；边界：不包含市场搜索结果。
	Dependencies []HeadlessDependencyStatus `json:"dependencies"`
	// NebulaReady 职责：表示所有内置 Nebula 依赖是否齐全；使用：计算 Ready 和前端状态灯；边界：不保证联机运行一定成功。
	NebulaReady bool `json:"nebulaReady"`
	// SteamAppIDPath 职责：记录 steam_appid.txt 路径；使用：准备环境和前端展示；边界：文件可能不存在或内容错误。
	SteamAppIDPath string `json:"steamAppIdPath"`
	// SteamAppIDExists 职责：表示 steam_appid.txt 是否存在；使用：前端区分缺失和内容错误；边界：不代表内容正确。
	SteamAppIDExists bool `json:"steamAppIdExists"`
	// SteamAppIDValid 职责：表示 steam_appid.txt 内容是否为 1366540；使用：Ready 计算；边界：读取失败时为 false。
	SteamAppIDValid bool `json:"steamAppIdValid"`
	// HostPort 职责：记录 Nebula 配置中的主机端口；使用：端口探测和启动提示；边界：配置缺失时使用默认端口。
	HostPort int `json:"hostPort"`
	// NebulaConfigPath 职责：记录 nebula.cfg 路径；使用：设置页和状态页展示；边界：依赖未安装时该路径可能不存在。
	NebulaConfigPath string `json:"nebulaConfigPath"`
	// ServerArchive 职责：记录启动存档参数；使用：构建启动命令；边界：不验证存档文件。
	ServerArchive string `json:"serverArchive"`
	// ProcessRunning 职责：表示目标游戏目录下无头服务器进程是否运行；使用：前端启动/停止按钮状态；边界：只匹配带无头参数的 DSPGAME.exe。
	ProcessRunning bool `json:"processRunning"`
	// ProcessID 职责：记录匹配到的进程 PID；使用：停止流程和日志展示；边界：未运行时为 0。
	ProcessID int `json:"processId"`
	// TCPListening 职责：表示目标端口 TCP 是否监听；使用：网络状态和审计日志；边界：不代表 UDP 状态。
	TCPListening bool `json:"tcpListening"`
	// UDPListening 职责：表示目标端口 UDP 是否监听；使用：网络状态和审计日志；边界：不代表 TCP 状态。
	UDPListening bool `json:"udpListening"`
	// PortListening 职责：表示目标端口是否有任一协议监听；使用：Ready 问题诊断；边界：进程运行但端口未监听会形成 issue。
	PortListening bool `json:"portListening"`
	// StartCommand 职责：保存实际启动命令切片；使用：服务层启动进程；边界：前端展示应使用 StartCommandText。
	StartCommand []string `json:"startCommand"`
	// StartCommandText 职责：保存用户可读启动命令；使用：前端展示复制；边界：不用于执行命令。
	StartCommandText string `json:"startCommandText"`
	// BepInExLogPath 职责：记录 BepInEx 日志路径；使用：前端提示排错位置；边界：文件可能尚未生成。
	BepInExLogPath string `json:"bepInExLogPath"`
	// OutputLogPath 职责：记录 Unity 输出日志路径；使用：前端提示排错位置；边界：文件可能尚未生成。
	OutputLogPath string `json:"outputLogPath"`
	// Network 职责：保存当前网卡速率状态；使用：前端网络折线图；边界：读取失败时 Valid=false。
	Network HeadlessNetworkStatus `json:"network"`
	// Issues 职责：列出阻塞开服或需要关注的问题；使用：前端展示待处理事项；边界：只来自本地检测规则。
	Issues []string `json:"issues"`
	// Ready 职责：表示开服基础环境是否满足启动条件；使用：服务层决定是否允许启动；边界：不保证运行后联机逻辑无错误。
	Ready bool `json:"ready"`
}
