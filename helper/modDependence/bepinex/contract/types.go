package contract

// 文件职责：定义 BepInEx 检测、安装和卸载流程的跨层契约类型。
// 使用：由 BepInEx helper、Mod 安装器、无头服务器和 Wails 服务层在传递状态或结果时引用。
// 边界：只定义 DTO 形状，不执行文件检测、下载、安装、卸载、审计或前端交互编排。

// BepInExStatus 表示游戏目录下 BepInEx 注入环境的检测状态。
// 职责：聚合 BepInEx 关键文件、Doorstop 配置和版本读取结果。
// 使用：设置页、Mod 安装器和无头服务器准备流程用该结构展示或判断前置环境是否就绪。
// 边界：只描述本地检测结果，不代表远端最新版本，也不会触发任何修复动作。
type BepInExStatus struct {
	// GameDir 职责：记录本次检测对应的游戏根目录；使用：前端和服务日志展示路径；边界：该字段来自调用方输入或配置归一化结果，不在 DTO 内再次校验。
	GameDir string `json:"gameDir"`
	// Installed 职责：表示所有必需文件和 Doorstop 配置是否就绪；使用：安装器据此决定跳过或继续安装；边界：不代表 Nebula 或其他 Mod 已安装。
	Installed bool `json:"installed"`
	// BepInExDir 职责：记录 BepInEx 子目录路径；使用：前端状态展示和日志定位；边界：目录可能不存在，需结合 Installed 判断。
	BepInExDir string `json:"bepInExDir"`
	// MissingFiles 职责：列出缺失或无效的关键项；使用：服务层返回给前端展示阻塞原因；边界：只包含 BepInEx 和 Doorstop 相关项。
	MissingFiles []string `json:"missingFiles"`

	// CoreExists 职责：标记 BepInEx.dll 是否存在；使用：诊断 BepInEx 核心文件缺失；边界：不校验 DLL 版本兼容性。
	CoreExists bool `json:"coreExists"`
	// PreloaderExists 职责：标记 BepInEx.Preloader.dll 是否存在；使用：判断 Doorstop 目标程序集是否具备；边界：不读取程序集内容。
	PreloaderExists bool `json:"preloaderExists"`
	// WinHTTPExists 职责：标记 winhttp.dll 注入文件是否存在；使用：判断 Windows Doorstop 注入入口是否完整；边界：不校验文件签名。
	WinHTTPExists bool `json:"winHTTPExists"`
	// DoorstopExists 职责：标记 doorstop_config.ini 是否存在；使用：判断是否需要安装或修复配置；边界：存在不代表配置有效。
	DoorstopExists bool `json:"doorstopExists"`
	// DoorstopValid 职责：标记 Doorstop 配置是否启用并指向 BepInEx Preloader；使用：安装器决定是否只修复配置；边界：读取失败时为 false。
	DoorstopValid bool `json:"doorstopValid"`
	// InstalledVersion 职责：记录本地 BepInEx 版本文本；使用：前端展示当前版本；边界：读取不到版本时为空字符串。
	InstalledVersion string `json:"installedVersion"`
}

// BepInExPackage 表示可安装的 BepInEx Thunderstore 包信息。
// 职责：承载远端 BepInEx 包的下载地址、版本号和依赖信息。
// 使用：InstallBepInEx 下载最新 Thunderstore 包并写入安装结果时使用。
// 边界：只描述选中的 active 版本，不包含完整 Thunderstore 包索引。
type BepInExPackage struct {
	// Owner 职责：记录 Thunderstore 包作者；使用：构造缓存文件名和日志；边界：不参与本地目录命名安全校验。
	Owner string `json:"owner"`
	// Name 职责：记录 Thunderstore 包名；使用：展示和缓存命名；边界：必须来自远端索引，不由前端输入。
	Name string `json:"name"`
	// FullName 职责：记录 Thunderstore 版本全名；使用：日志和调试定位版本；边界：不作为唯一数据库键。
	FullName string `json:"fullName"`
	// Version 职责：记录可安装版本号；使用：构造下载缓存和展示版本；边界：不在此处比较语义版本。
	Version string `json:"version"`
	// DownloadURL 职责：记录 zip 下载地址；使用：remote.DownloadPackage 发起 HTTP 下载；边界：为空或不可访问时由下载阶段返回错误。
	DownloadURL string `json:"downloadUrl"`
	// Dependencies 职责：保留远端声明的依赖列表；使用：前端或日志展示包元数据；边界：BepInEx 安装流程不会递归安装这些依赖。
	Dependencies []string `json:"dependencies"`
}

// BepInExInstallResult 表示 BepInEx 安装动作的执行结果。
// 职责：返回安装、跳过或修复 Doorstop 后的结果快照。
// 使用：服务层记录审计日志并透传给前端展示安装反馈。
// 边界：失败会以 error 返回，不用该结构表达部分失败。
type BepInExInstallResult struct {
	// GameDir 职责：记录安装目标游戏目录；使用：前端展示和日志定位；边界：不允许前端据此直接执行文件操作。
	GameDir string `json:"gameDir"`
	// BufferDir 职责：记录下载和解压缓存根目录；使用：调试安装缓存问题；边界：跳过安装时可能为空。
	BufferDir string `json:"bufferDir"`
	// Installed 职责：表示本次调用是否实际写入 BepInEx 文件；使用：服务日志判断状态变化；边界：已安装跳过时为 false。
	Installed bool `json:"installed"`
	// Skipped 职责：表示本次调用是否因已就绪而跳过写入；使用：前端展示“已安装/已修复”提示；边界：跳过不等于失败。
	Skipped bool `json:"skipped"`
	// Message 职责：保存用户可读的安装结果说明；使用：前端提示和审计日志；边界：不用于程序分支判断。
	Message string `json:"message"`
	// Package 职责：记录实际下载的 BepInEx 远端包；使用：日志展示版本；边界：跳过安装或仅修复配置时为空。
	Package *BepInExPackage `json:"package,omitempty"`
	// ZipPath 职责：记录下载缓存 zip 路径；使用：调试下载缓存；边界：只用于本机展示，不作为持久配置。
	ZipPath string `json:"zipPath,omitempty"`
	// ExtractPath 职责：记录解压目录路径；使用：调试包结构问题；边界：安装器后续可清理该目录。
	ExtractPath string `json:"extractPath,omitempty"`
	// Status 职责：保存安装后的最新检测状态；使用：前端刷新 BepInEx 卡片；边界：状态只反映安装完成时刻。
	Status BepInExStatus `json:"status"`
}

// BepInExUninstallResult 表示 BepInEx 卸载后的本地清理结果。
// 职责：返回 BepInEx 注入文件卸载结果。
// 使用：服务层记录卸载审计日志并透传给设置页展示。
// 边界：只删除 BepInEx 注入相关文件，不触碰游戏本体目录。
type BepInExUninstallResult struct {
	// GameDir 职责：记录卸载目标游戏目录；使用：日志和前端展示；边界：路径已在卸载前校验。
	GameDir string `json:"gameDir"`
	// Removed 职责：表示本次调用是否删除过文件；使用：前端区分已卸载和无操作；边界：BepInEx 已安装但无文件可删时同样为 true。
	Removed bool `json:"removed"`
	// Skipped 职责：表示未发现可卸载内容；使用：前端展示跳过提示；边界：跳过不代表错误。
	Skipped bool `json:"skipped"`
	// Message 职责：保存用户可读卸载结果；使用：前端提示和审计日志；边界：不用于业务分支。
	Message string `json:"message"`
	// RemovedPaths 职责：记录已删除的本地路径；使用：审计日志统计和排查；边界：只包含游戏目录内允许删除的目标。
	RemovedPaths []string `json:"removedPaths"`
	// Status 职责：保存卸载后的最新 BepInEx 检测状态；使用：前端刷新状态卡片；边界：状态只反映卸载完成时刻。
	Status BepInExStatus `json:"status"`
}
