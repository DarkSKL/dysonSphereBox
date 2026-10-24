package nebula

// 文件职责：集中定义 Nebula 配置文件的稳定路径、设置节名与默认主机端口。
// 使用：headless 开服流程读写 HostPort；gameport 探测游戏实际主机端口时读取同一配置。
// 边界：只保存常量，不读取文件、不校验端口状态、不修改配置内容。

const (
	// ConfigRelative 职责：定义 nebula.cfg 相对游戏目录的路径；使用：读取和写入 HostPort；边界：依赖未安装时文件可能不存在。
	ConfigRelative = "BepInEx/config/nebula.cfg"
	// SettingsSection 职责：定义 nebula.cfg 中保存端口等设置的节名；使用：解析和更新 HostPort；边界：其它配置节不在本模块维护。
	SettingsSection = "Nebula - Settings"
	// DefaultHostPort 职责：提供 Nebula 默认主机端口；使用：配置缺失或解析失败时兜底；边界：不保证端口已被游戏监听。
	DefaultHostPort = 8469
)
