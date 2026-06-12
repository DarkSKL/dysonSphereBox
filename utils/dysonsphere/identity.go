package dysonsphere

// 文件职责：定义戴森球计划目录探测所需的稳定身份常量。
// 使用：目录归一化、Steam manifest 探测和默认 common 目录兜底逻辑共同读取。
// 边界：只描述戴森球计划本体，不描述 Mod、BepInEx 或无头服务器文件。

// 戴森球计划路径常量组集中描述游戏目录自动识别所需的稳定名称。
// 职责：保存启动程序名、默认安装目录名和 Steam AppID。
// 使用：环境服务、Steam 探测和快捷方式探测共同读取。
// 边界：只描述戴森球计划本体，不描述 Mod、BepInEx 或无头服务器文件。
const (
	// ExeName 是戴森球计划 Windows 启动程序文件名。
	// 职责：作为判断目录是否为游戏根目录的核心文件名。
	// 使用：NormalizeDir 拼接或识别 exe 路径时读取。
	// 边界：仅适用于当前 Windows 版启动文件命名。
	ExeName = "DSPGAME.exe"
	// DirName 是 Steam 默认安装目录名。
	// 职责：作为未能从 manifest 读取安装目录时的 common 目录兜底名称。
	// 使用：FindDir 组合默认 Steam common 路径时读取。
	// 边界：用户自定义安装目录仍优先依赖 manifest 或快捷方式发现。
	DirName = "Dyson Sphere Program"
	// SteamAppID 是戴森球计划在 Steam 中的应用 ID。
	// 职责：定位 Steam appmanifest 文件。
	// 使用：utils.ReadSteamAppInstallDir 读取 appmanifest_1366540.acf 时传入。
	// 边界：只适用于 Steam 版本，不覆盖其它发行渠道。
	SteamAppID = "1366540"
)
