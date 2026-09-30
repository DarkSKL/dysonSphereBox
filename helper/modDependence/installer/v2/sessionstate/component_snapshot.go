package sessionstate

// 文件职责：定义覆盖安装前的组件记录快照结构。
// 使用：失败清理恢复组件表和包组件关联时读取 ComponentSnapshot。
// 边界：只保存组件模型字段，不保存文件清单、不下载组件、不包含 package-component 关系。

import (
	"time"

	"gorm.io/datatypes"
)

// ComponentSnapshot 表示安装前组件记录快照。
// 职责：保存可恢复的 installed component 字段。
// 使用：失败清理恢复组件表时调用。
// 边界：只保存组件模型字段，不包含 package-component 关系。
type ComponentSnapshot struct {
	// ComponentKey 职责：保存历史组件唯一标识；使用：失败恢复组件记录和关联关系；边界：不能为空才可恢复组件。
	ComponentKey string `json:"componentKey"`
	// Source 职责：保存历史组件来源；使用：恢复组件记录；边界：只恢复快照值，不重新校验来源。
	Source string `json:"source"`
	// SourceDependencyID 职责：保存历史来源侧依赖标识；使用：恢复组件记录；边界：为空时仍以 ComponentKey 为准。
	SourceDependencyID string `json:"sourceDependencyId"`
	// Namespace 职责：保存历史来源命名空间；使用：恢复展示和诊断字段；边界：manual 来源可能为空。
	Namespace string `json:"namespace"`
	// Name 职责：保存历史组件名；使用：恢复组件记录；边界：前端展示可使用 DisplayName。
	Name string `json:"name"`
	// DisplayName 职责：保存历史显示名称；使用：恢复安装明细和冲突提示字段；边界：为空时调用方可回退 Name。
	DisplayName string `json:"displayName"`
	// Version 职责：保存历史组件版本；使用：恢复组件记录；边界：不解析语义化版本。
	Version string `json:"version"`
	// Description 职责：保存历史组件说明；使用：恢复诊断字段；边界：不做文本清洗。
	Description string `json:"description"`
	// DownloadURL 职责：保存历史组件下载地址；使用：恢复诊断字段；边界：恢复时不重新下载。
	DownloadURL string `json:"downloadUrl"`
	// PackageURL 职责：保存历史组件来源页面；使用：恢复展示或诊断字段；边界：不参与清理。
	PackageURL string `json:"packageUrl"`
	// ArtifactKind 职责：保存历史组件产物类型；使用：恢复诊断字段；边界：不驱动清理逻辑。
	ArtifactKind string `json:"artifactKind"`
	// LocalPath 职责：保存历史本地产物路径；使用：恢复手动导入诊断字段；边界：路径可能已不存在。
	LocalPath string `json:"localPath"`
	// Raw 职责：保存历史来源原始 JSON；使用：恢复组件记录；边界：不作为文件归属判断依据。
	Raw datatypes.JSON `json:"raw"`
	// UpdatedAt 职责：保存历史组件更新时间；使用：恢复组件记录时间；边界：不代表远端更新时间。
	UpdatedAt time.Time `json:"updatedAt"`
}
