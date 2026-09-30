package sessionstate

// 文件职责：定义覆盖安装前的主包快照结构。
// 使用：覆盖安装失败、取消或中断清理时用 PackageSnapshot 恢复此前主包记录。
// 边界：只保存恢复所需字段，不重新下载包、不校验远端版本、不作为前端 DTO。

import (
	"time"

	"gorm.io/datatypes"
)

// PackageSnapshot 表示安装前主包记录快照。
// 职责：保存可恢复的 installed package 字段。
// 使用：覆盖安装失败或取消时恢复此前安装状态。
// 边界：只用于内部 manifest，不作为前端 DTO。
type PackageSnapshot struct {
	// Source 职责：保存历史主包来源；使用：失败恢复 installed package 时写回；边界：只恢复快照值，不重新校验来源。
	Source string `json:"source"`
	// SourcePackageID 职责：保存历史来源侧包标识；使用：恢复主包来源字段；边界：为空时仍以 PackageKey 定位记录。
	SourcePackageID string `json:"sourcePackageId"`
	// Namespace 职责：保存历史来源命名空间；使用：恢复展示与诊断字段；边界：manual 来源可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存历史来源包名；使用：恢复主包记录；边界：前端展示可使用 DisplayName。
	Name string `json:"name"`
	// DisplayName 职责：保存历史显示名称；使用：恢复前端展示字段；边界：为空时调用方可回退 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存历史安装版本；使用：恢复已安装版本展示；边界：不解析语义化版本。
	Version string `json:"version"`
	// Description 职责：保存历史包说明；使用：恢复主包记录；边界：不做文本清洗。
	Description string `json:"description"`
	// Author 职责：保存历史作者；使用：恢复展示字段；边界：为空不影响恢复。
	Author string `json:"author"`
	// DownloadURL 职责：保存历史下载地址；使用：恢复诊断字段；边界：恢复时不重新下载。
	DownloadURL string `json:"downloadUrl"`
	// PackageURL 职责：保存历史来源页面地址；使用：恢复展示或跳转字段；边界：不参与文件清理。
	PackageURL string `json:"packageUrl"`
	// CoverURL 职责：保存历史封面地址；使用：恢复前端卡片展示；边界：图片可用性由前端兜底。
	CoverURL string `json:"coverUrl"`
	// ArtifactKind 职责：保存历史产物类型；使用：恢复诊断字段；边界：不驱动清理逻辑。
	ArtifactKind string `json:"artifactKind"`
	// LocalPath 职责：保存历史本地产物路径；使用：恢复手动导入诊断字段；边界：路径可能已不存在。
	LocalPath string `json:"localPath"`
	// DownloadCount 职责：保存历史下载次数快照；使用：恢复展示字段；边界：不代表恢复时的远端实时数据。
	DownloadCount int64 `json:"downloadCount"`
	// DependencyCount 职责：保存历史依赖数量快照；使用：恢复展示字段；边界：关联关系由其它表恢复。
	DependencyCount int64 `json:"dependencyCount"`
	// PackageSize 职责：保存历史包大小；使用：恢复展示字段；边界：不作为文件完整性校验。
	PackageSize int64 `json:"packageSize"`
	// Tags 职责：保存历史标签 JSON；使用：恢复主包记录；边界：不解析标签语义。
	Tags datatypes.JSON `json:"tags"`
	// GameVersions 职责：保存历史适配游戏版本 JSON；使用：恢复主包记录；边界：不解析版本结构。
	GameVersions datatypes.JSON `json:"gameVersions"`
	// Trustworthiness 职责：保存历史可信度值；使用：恢复展示字段；边界：不作为安全校验。
	Trustworthiness int64 `json:"trustworthiness"`
	// InstallState 职责：保存历史安装状态；使用：失败恢复时还原旧安装状态；边界：当前失败会话状态由清理流程单独记录。
	InstallState string `json:"installState"`
	// InstallStage 职责：保存历史安装阶段；使用：恢复进度快照字段；边界：只恢复旧记录，不推进新状态。
	InstallStage string `json:"installStage"`
	// InstallStatus 职责：保存历史安装状态文案；使用：恢复前端可读状态；边界：不用于清理分支。
	InstallStatus string `json:"installStatus"`
	// InstallProgress 职责：保存历史安装进度；使用：恢复旧进度字段；边界：不代表当前失败清理进度。
	InstallProgress int `json:"installProgress"`
	// ErrorMessage 职责：保存历史错误文案；使用：恢复旧记录失败信息；边界：为空不影响恢复。
	ErrorMessage string `json:"errorMessage"`
	// CurrentSpeed 职责：保存历史下载速度；使用：恢复旧进度字段；边界：通常恢复为历史快照值。
	CurrentSpeed int64 `json:"currentSpeed"`
	// Raw 职责：保存历史来源原始 JSON；使用：恢复冲突继续或诊断字段；边界：不暴露给用户编辑。
	Raw datatypes.JSON `json:"raw"`
	// InstalledAt 职责：保存历史安装时间；使用：恢复主包记录时间；边界：不代表恢复执行时间。
	InstalledAt time.Time `json:"installedAt"`
	// UpdatedAt 职责：保存历史更新时间；使用：恢复主包记录时间；边界：不代表远端更新时间。
	UpdatedAt time.Time `json:"updatedAt"`
}
