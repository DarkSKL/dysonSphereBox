package headless

// 文件职责：集中定义无头服务器环境检测与启动命令使用的稳定常量。
// 使用：environment、prepare、settings、process_control 等同包流程读取这些值。
// 边界：只保存常量，不读取配置文件、不检测进程、不写入磁盘；Nebula 配置相关值复用共享包，避免与 gameport 重复维护。

import "DysonSphereBox/helper/modDependence/nebula"

const (
	// headlessSteamAppID 职责：保存戴森球计划 Steam AppID；使用：写入和校验 steam_appid.txt；边界：不代表用户 Steam 安装路径。
	headlessSteamAppID = "1366540"
	// defaultNebulaHostPort 职责：提供 Nebula 默认主机端口；使用：nebula.cfg 缺失或端口解析失败时兜底；边界：不保证端口未被占用。
	defaultNebulaHostPort = nebula.DefaultHostPort
	// dysonSphereProcessName 职责：定义戴森球计划进程名；使用：进程查询和启动命令构造；边界：只匹配 Windows 可执行名。
	dysonSphereProcessName = "DSPGAME.exe"
	// nebulaServerArgument 职责：定义 Nebula 无头服务器启动参数；使用：识别和启动无头进程；边界：不用于普通游戏进程。
	nebulaServerArgument = "-nebula-server"
	// nebulaConfigRelative 职责：定义 Nebula 配置文件相对路径；使用：读取和写入 HostPort；边界：与 gameport 共用 nebula.ConfigRelative。
	nebulaConfigRelative = nebula.ConfigRelative
	// nebulaSettingsSection 职责：定义 nebula.cfg 中保存端口的配置节名；使用：解析和更新 HostPort；边界：与 gameport 共用 nebula.SettingsSection。
	nebulaSettingsSection = nebula.SettingsSection
	// headlessSteamAppIDFile 职责：定义开服基础文件名；使用：准备环境时写入游戏目录；边界：只写入 AppID，不修改 Steam 客户端。
	headlessSteamAppIDFile = "steam_appid.txt"
	// headlessOutputLogFile 职责：定义 Unity 输出日志名；使用：状态 DTO 返回排错路径；边界：不创建或读取该日志。
	headlessOutputLogFile = "output_log.txt"
	// headlessBepInExLogFile 职责：定义 BepInEx 日志相对路径；使用：状态 DTO 返回排错路径；边界：不创建或读取该日志。
	headlessBepInExLogFile = "BepInEx/LogOutput.log"
	// headlessPluginRoot 职责：定义 BepInEx 插件根目录；使用：依赖检测和基础目录创建；边界：不负责安装具体 Mod。
	headlessPluginRoot = "BepInEx/plugins"
	// headlessDefaultSaveMode 职责：定义未选择存档时的启动参数；使用：构造默认启动命令；边界：实际加载哪个存档由游戏/Nebula 决定。
	headlessDefaultSaveMode = "-load-latest"
)
