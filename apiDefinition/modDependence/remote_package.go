package modDependence

// 文件职责：定义外部接口契约和前后端传输 DTO。
// 使用：由 service/helper 在请求远端接口或生成 Wails 类型时引用。
// 边界：不发起网络请求，不读写数据库，不承载业务决策。

import (
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
)

// RemotePackageListData 表示 Thunderstore 等来源适配后的 Mod 包分页数据。
// 职责：承载统一来源市场适配后的分页结果。
// 使用：前端 Mod 页只消费该结构，不再关心不同来源的原始响应形状。
// 边界：只描述列表结果，不保存缓存状态，不代表包已安装。
type RemotePackageListData struct {
	// List 职责：保存当前分页内的统一远端包 DTO 列表；使用：前端市场列表直接渲染；边界：空列表不代表来源不可用，需结合 Total 和错误判断。
	List []RemoteModPackageDTO `json:"list"`
	// Total 职责：保存符合查询条件的远端包总数；使用：前端分页器计算页数；边界：Thunderstore 本地分页由 helper 计算，官方源沿用远端返回值。
	Total int `json:"total"`
	// Page 职责：保存当前页码；使用：前端同步分页状态；边界：由服务层归一化后返回，调用方不应假设原始入参一定有效。
	Page int `json:"page"`
	// PageSize 职责：保存当前分页大小；使用：前端分页器和列表请求复用；边界：实际值可能被后端上限裁剪。
	PageSize int `json:"pageSize"`
	// Source 职责：标记当前列表对应的统一来源；使用：前端区分 official、thunderstore 或 manual 结果；边界：不代表每个包已安装或可直接下载。
	Source sourceConstant.ModSource `json:"source"`
}

// RemoteModDependencyDTO 表示 Thunderstore 等来源适配后的依赖组件。
// 职责：描述统一来源包中的单个依赖组件。
// 使用：安装器解析依赖链、前端展示依赖明细时使用。
// 边界：Key 是跨来源稳定身份；Raw 保留原始依赖声明，便于排查来源解析问题。
type RemoteModDependencyDTO struct {
	// Key 职责：保存跨来源稳定依赖身份；使用：安装器递归解析和本地记录关联；边界：不得用旧数字 ID 替代该字段。
	Key string `json:"key"`
	// Source 职责：标记依赖来自哪个市场来源；使用：Provider 和安装器选择解析策略；边界：未知来源应由上层返回错误。
	Source sourceConstant.ModSource `json:"source"`
	// SourceDependencyID 职责：保存来源侧依赖原始 ID；使用：调试、兼容旧官方依赖和回溯来源数据；边界：为空时仍以 Key 为准。
	SourceDependencyID string `json:"sourceDependencyId"`
	// Namespace 职责：保存来源侧命名空间；使用：Thunderstore 依赖匹配和展示作者/团队信息；边界：官方源可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存来源侧机器友好名称；使用：生成路径、key 或调试显示；边界：展示优先使用 DisplayName。
	Name string `json:"name"`
	// DisplayName 职责：保存前端展示名称；使用：依赖列表和安装明细展示；边界：为空时调用方可回退到 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存依赖版本文本；使用：安装记录和依赖展示；边界：不在 DTO 层解析语义化版本。
	Version string `json:"version"`
	// Description 职责：保存依赖说明文本；使用：前端详情和搜索匹配；边界：不做 Markdown 或 HTML 清洗。
	Description string `json:"description"`
	// DownloadURL 职责：保存依赖 artifact 下载地址；使用：安装器下载远端 zip；边界：本地导入依赖可能为空。
	DownloadURL string `json:"downloadUrl"`
	// PackageURL 职责：保存来源包页面地址；使用：前端跳转或调试；边界：不参与下载安装。
	PackageURL string `json:"packageUrl"`
	// ArtifactKind 职责：标记依赖 artifact 类型；使用：安装器决定 zip 解压、本地 zip 或本地目录流程；边界：空值按远端 zip 兼容处理。
	ArtifactKind artifactConstant.ArtifactKind `json:"artifactKind"`
	// LocalPath 职责：保存手动导入依赖的本地路径；使用：manual 或本地 artifact 安装时读取；边界：远端来源通常为空，路径校验由安装器完成。
	LocalPath string `json:"localPath"`
	// Raw 职责：保留来源侧原始依赖声明；使用：排查依赖解析问题或兼容 Thunderstore 字符串格式；边界：不作为本地唯一身份。
	Raw string `json:"raw"`
	// Deprecated 职责：标记依赖是否被来源侧废弃；使用：前端提示和过滤扩展；边界：不代表本地安装状态。
	Deprecated bool `json:"deprecated"`
}

// RemoteModPackageDTO 表示 Thunderstore 等来源适配后的可安装 Mod 包。
// 职责：描述统一来源市场中可安装的主包及其依赖信息。
// 使用：前端市场卡片展示和 V2 安装器安装入参均使用该结构。
// 边界：保留 LegacyID 只用于兼容旧国内源状态；新逻辑必须以 Key 为唯一身份。
type RemoteModPackageDTO struct {
	// Key 职责：保存跨来源稳定包身份；使用：前端安装状态映射、安装器主键和冲突恢复；边界：新逻辑不得依赖 LegacyID 做唯一身份。
	Key string `json:"key"`
	// LegacyID 职责：保存旧官方源数字 ID；使用：兼容旧安装流程、官方下载计数和历史状态映射；边界：Thunderstore 与 manual 来源可能为 0。
	LegacyID uint `json:"legacyId"`
	// Source 职责：标记包来自哪个统一来源；使用：服务层和 helper 选择查询、安装或刷新策略；边界：未知来源由调用方返回错误。
	Source sourceConstant.ModSource `json:"source"`
	// SourcePackageID 职责：保存来源侧原始包 ID；使用：调试、回溯来源数据和兼容不同市场；边界：本地唯一性仍以 Key 为准。
	SourcePackageID string `json:"sourcePackageId"`
	// Namespace 职责：保存来源侧命名空间；使用：Thunderstore 包识别和作者/团队展示；边界：官方源可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存来源侧机器友好名称；使用：key 生成、路径规划或调试；边界：用户界面优先展示 DisplayName。
	Name string `json:"name"`
	// DisplayName 职责：保存用户可读包名；使用：市场卡片、进度文本和安装结果展示；边界：为空时调用方可回退到 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存可安装版本文本；使用：安装记录、依赖解析和展示；边界：不在 DTO 层解析语义化版本。
	Version string `json:"version"`
	// Description 职责：保存包说明文本；使用：市场搜索、卡片摘要和详情展示；边界：不做 Markdown 或 HTML 清洗。
	Description string `json:"description"`
	// Author 职责：保存作者或团队展示名；使用：市场卡片和搜索匹配；边界：Thunderstore 来源可由 Namespace 补充。
	Author string `json:"author"`
	// DownloadURL 职责：保存主 artifact 下载地址；使用：安装器下载远端 zip；边界：本地导入或仅依赖包场景可能为空。
	DownloadURL string `json:"downloadUrl"`
	// PackageURL 职责：保存来源详情页地址；使用：前端跳转或调试；边界：不参与文件下载。
	PackageURL string `json:"packageUrl"`
	// CoverURL 职责：保存封面或图标地址；使用：市场卡片展示；边界：图片可用性由前端加载结果决定。
	CoverURL string `json:"coverUrl"`
	// DownloadCount 职责：保存来源侧下载次数；使用：市场热度展示；边界：不代表本地下载计数。
	DownloadCount int64 `json:"downloadCount"`
	// DependencyCount 职责：保存来源侧依赖数量；使用：市场卡片摘要；边界：实际安装依赖以 Dependencies 列表为准。
	DependencyCount int64 `json:"dependencyCount"`
	// PackageSize 职责：保存来源侧包大小；使用：展示和进度估算扩展；边界：不作为下载完整性校验依据。
	PackageSize int64 `json:"packageSize"`
	// Tags 职责：保存来源侧标签；使用：市场展示和后续筛选扩展；边界：标签语义由来源决定。
	Tags []string `json:"tags"`
	// GameVersions 职责：保存来源侧适配游戏版本原始结构；使用：前端展示或后续筛选；边界：DTO 层不解析具体格式。
	GameVersions interface{} `json:"gameVersions"`
	// Trustworthiness 职责：保存来源侧可信度或评分信号；使用：市场展示和排序扩展；边界：不作为安全校验依据。
	Trustworthiness int64 `json:"trustworthiness"`
	// UpdateTime 职责：保存来源更新时间文本；使用：前端展示和排序；边界：DTO 层不解析时间格式。
	UpdateTime string `json:"updateTime"`
	// Deprecated 职责：标记包是否被来源侧废弃；使用：前端提示和过滤；边界：不代表本地安装是否可用。
	Deprecated bool `json:"deprecated"`
	// ArtifactKind 职责：标记主 artifact 类型；使用：安装器选择远端 zip、本地 zip 或本地目录流程；边界：空值按远端 zip 兼容处理。
	ArtifactKind artifactConstant.ArtifactKind `json:"artifactKind"`
	// LocalPath 职责：保存手动导入包的本地路径；使用：manual 或本地 artifact 安装时读取；边界：远端来源通常为空，路径校验由安装器完成。
	LocalPath string `json:"localPath"`
	// Dependencies 职责：保存该包需要安装的统一依赖列表；使用：安装器递归解析和前端依赖展示；边界：空列表代表无显式依赖而不是解析失败。
	Dependencies []RemoteModDependencyDTO `json:"dependencies"`
	// RawDependencySource 职责：标记依赖列表来自哪个来源格式；使用：调试和兼容不同市场解析结果；边界：不参与本地身份判断。
	RawDependencySource string `json:"rawDependencySource"`
}
