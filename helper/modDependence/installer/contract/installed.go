package contract

// 文件职责：定义已安装 Mod 包摘要的跨层契约。
// 使用：v2/query 读取本地安装记录后构造，InstallerService 透出给资源页展示市场安装的模组。
// 边界：只描述展示字段，不执行文件操作、不暴露安装会话 manifest 或绝对目标路径。

import "time"

// InstalledComponentSummary 描述已安装包内单个组件的摘要。
// 职责：承载组件身份、版本和本包内归属文件数量。
// 使用：InstalledPackageSummary.Components 返回给资源页详情弹窗。
// 边界：共享组件可能同时出现在多个包摘要中，FileCount 只统计归属当前包的文件。
type InstalledComponentSummary struct {
	// ComponentKey 职责：保存组件唯一标识；使用：前端列表 key；边界：格式与安装器组件 key 一致。
	ComponentKey string `json:"componentKey"`
	// Namespace 职责：保存组件命名空间；使用：前端展示 Owner；边界：手动导入组件可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存组件名称；使用：显示名缺失时兜底；边界：不作为唯一标识。
	Name string `json:"name"`
	// DisplayName 职责：保存组件显示名；使用：前端展示；边界：为空时前端回退 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存组件实际安装版本；使用：前端展示；边界：不做语义版本比较。
	Version string `json:"version"`
	// FileCount 职责：保存当前包内该组件的文件数量；使用：前端展示；边界：复用的共享组件可能为 0。
	FileCount int `json:"fileCount"`
}

// InstalledPackageSummary 描述本地已安装 Mod 包的摘要。
// 职责：承载资源页展示市场安装模组所需的包信息、插件目录和组件明细。
// 使用：InstallerService.ListInstalledPackages 返回给前端资源页。
// 边界：只包含已安装状态的包；安装中、失败和冲突中的包由进度快照表达。
type InstalledPackageSummary struct {
	// PackageKey 职责：保存统一包 key；使用：前端列表 key 和卸载参数；边界：格式与 RemoteModPackageDTO.Key 一致。
	PackageKey string `json:"packageKey"`
	// Source 职责：保存包来源；使用：前端展示来源标签；边界：取值 thunderstore 或 manual。
	Source string `json:"source"`
	// Namespace 职责：保存包命名空间；使用：前端展示作者兜底；边界：手动导入包可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存包名称；使用：显示名缺失时兜底；边界：不作为唯一标识。
	Name string `json:"name"`
	// DisplayName 职责：保存包显示名；使用：卡片主标题；边界：为空时前端回退 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存安装版本；使用：卡片副标题；边界：不做语义版本比较。
	Version string `json:"version"`
	// Author 职责：保存作者；使用：详情展示；边界：缺失时为空。
	Author string `json:"author"`
	// Description 职责：保存包说明；使用：详情展示；边界：不做富文本渲染。
	Description string `json:"description"`
	// PackageURL 职责：保存来源页面地址；使用：详情展示；边界：手动导入包可能为空。
	PackageURL string `json:"packageUrl"`
	// InstalledAt 职责：保存安装时间；使用：详情展示；边界：零值表示未知。
	InstalledAt time.Time `json:"installedAt" ts_type:"string"`
	// FileCount 职责：保存归属当前包的文件数量；使用：详情展示；边界：不包含其它包共享持有的文件。
	FileCount int `json:"fileCount"`
	// SizeBytes 职责：保存归属当前包的文件总大小；使用：详情展示；边界：以安装记录为准，不实时扫描磁盘。
	SizeBytes int64 `json:"sizeBytes"`
	// PluginDirs 职责：保存写入 BepInEx/plugins 下的一级目录名；使用：详情展示和资源页识别重复纳管；边界：只来自安装记录相对路径。
	PluginDirs []string `json:"pluginDirs"`
	// Components 职责：保存包关联的组件明细；使用：详情展示依赖；边界：主包组件排在最前。
	Components []InstalledComponentSummary `json:"components"`
}
