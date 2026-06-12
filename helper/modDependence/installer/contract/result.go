package contract

// 文件职责：定义 Mod 安装、卸载流程返回给 service 和前端的统一结果契约。
// 使用：installer 实现层在完成旧官方整合包或 Thunderstore/V2 包处理后构造，service 层直接作为 Wails 返回值透出。
// 边界：只描述调用结果和展示字段，不执行文件操作、数据库写入、审计记录或进度事件推送。

import bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"

// DependencyInstallResult 表示单个 Mod 依赖的安装或卸载结果。
// 职责：承载单个依赖组件在安装或卸载中的结果明细。
// 使用：PackageInstallResult.Dependencies 返回给前端展示每个依赖的状态。
// 边界：路径仅用于本机状态展示，不参与前端拼接文件操作。
type DependencyInstallResult struct {
	// Key 职责：记录统一来源依赖 key；使用：Thunderstore/V2 依赖结果展示和日志定位；边界：旧官方依赖可能为空。
	Key string `json:"key"`
	// ID 职责：记录旧官方依赖数字 ID；使用：旧整合包安装结果展示；边界：V2 依赖可能为 0。
	ID uint `json:"id"`
	// Name 职责：记录依赖显示名称；使用：前端展示安装明细；边界：为空时调用方应回退到 Key 或路径。
	Name string `json:"name"`
	// Version 职责：记录依赖版本文本；使用：前端展示本次处理版本；边界：不做语义版本比较。
	Version string `json:"version"`
	// PluginDir 职责：记录依赖写入或移除的插件目录；使用：前端和日志排查本地文件位置；边界：不允许前端据此直接删除文件。
	PluginDir string `json:"pluginDir"`
	// ZipPath 职责：记录依赖下载或使用的 zip 路径；使用：调试下载缓存；边界：本机路径不作为持久配置。
	ZipPath string `json:"zipPath"`
	// Installed 职责：表示该依赖是否在本次调用中完成安装；使用：前端显示成功状态；边界：跳过或卸载时为 false。
	Installed bool `json:"installed"`
	// Removed 职责：表示该依赖是否在本次调用中被移除；使用：前端显示卸载状态；边界：共享依赖保留时为 false。
	Removed bool `json:"removed"`
	// Skipped 职责：表示该依赖是否被安全跳过；使用：前端区分共享保留或无操作；边界：跳过不代表错误。
	Skipped bool `json:"skipped"`
	// Message 职责：保存用户可读结果说明；使用：前端明细提示和日志；边界：不用于业务分支判断。
	Message string `json:"message"`
}

// PackageInstallResult 表示整合包安装器的整体执行结果。
// 职责：承载旧官方整合包和 V2 统一包安装/卸载的整体结果。
// 使用：InstallerService 的安装、卸载、继续冲突接口返回给前端。
// 边界：失败通过 error 返回；该结构只表示成功、跳过或卸载完成结果。
type PackageInstallResult struct {
	// PackageKey 职责：记录 V2 统一包 key；使用：前端按来源统一映射安装状态；边界：旧官方整合包可能为空。
	PackageKey string `json:"packageKey"`
	// PackageID 职责：记录旧官方整合包数字 ID；使用：兼容旧市场卡片状态；边界：V2 包可能为 0。
	PackageID uint `json:"packageId"`
	// PackageName 职责：记录整合包显示名称；使用：前端结果提示和审计日志；边界：不作为唯一标识。
	PackageName string `json:"packageName"`
	// GameDir 职责：记录安装目标游戏目录；使用：前端状态展示和日志定位；边界：不允许前端据此直接操作文件。
	GameDir string `json:"gameDir"`
	// PluginRoot 职责：记录 BepInEx 插件根目录；使用：前端展示安装位置；边界：目录可能因卸载或跳过而未改变。
	PluginRoot string `json:"pluginRoot"`
	// Installed 职责：表示本次调用是否完成安装；使用：前端更新卡片安装态；边界：跳过、卸载或失败时为 false。
	Installed bool `json:"installed"`
	// Uninstalled 职责：表示本次调用是否完成卸载；使用：前端更新卡片卸载态；边界：安装流程中为 false。
	Uninstalled bool `json:"uninstalled"`
	// Skipped 职责：表示本次调用是否无须执行主要文件操作；使用：前端展示“已跳过”；边界：跳过不代表失败。
	Skipped bool `json:"skipped"`
	// Message 职责：保存用户可读整体结果；使用：前端提示和审计日志；边界：不用于程序分支判断。
	Message string `json:"message"`
	// Dependencies 职责：保存依赖组件处理结果；使用：前端展示明细；边界：可能为空，取决于安装包内容或卸载路径。
	Dependencies []DependencyInstallResult `json:"dependencies"`
	// BepInExStatus 职责：保存操作后的 BepInEx 前置状态；使用：前端同步前置框架状态；边界：只代表操作完成时刻。
	BepInExStatus bepinexContract.BepInExStatus `json:"bepInExStatus"`
}
