package artifact

// 文件职责：提供 artifact 到安装布局算法的适配方法。
// 使用：安装规划、会话清理和目录候选生成需要 artifactlayout 输入时调用。
// 边界：只做纯字段转换和目录名生成，不访问文件系统、不读取数据库、不发送进度。

import (
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	"strings"
)

// Descriptor 将内部 artifact 转为布局算法输入。
// 职责：隔离 installer 内部安装状态与 artifactlayout 纯路径算法。
// 使用：安装规划、会话清理和目录命名需要调用 artifactlayout 时使用。
// 边界：只复制命名字段，不传递下载地址、来源、进度或安装状态。
func (artifact Artifact) Descriptor() artifactlayout.ArtifactDescriptor {
	return artifactlayout.ArtifactDescriptor{
		ComponentKey: artifact.ComponentKey,
		Namespace:    artifact.Namespace,
		Name:         artifact.Name,
		DisplayName:  artifact.DisplayName,
		Version:      artifact.Version,
	}
}

// InstallDirName 返回 artifact 的最终安装目录名。
// 职责：委托 artifactlayout 统一生成组件目录名称。
// 使用：会话清理和布局候选需要根据组件信息生成目录时调用。
// 边界：不访问文件系统，所有候选无效时返回 artifact。
func (artifact Artifact) InstallDirName() string {
	return artifactlayout.InstallDirName(artifact.Descriptor())
}

// InstallRecordPackageKey 返回文件清单应写入的安装包 key。
// 职责：确定依赖组件文件应归属到主包还是自身包。
// 使用：复制文件、会话清单和安装记录保存时调用。
// 边界：依赖组件的 componentKey 独立保留，但 packageKey 必须指向主包，避免主包卸载找不到依赖文件。
func (artifact Artifact) InstallRecordPackageKey() string {
	if strings.TrimSpace(artifact.ParentPackageKey) != "" {
		return artifact.ParentPackageKey
	}
	return artifact.PackageKey
}

// SourceDependencyID 从统一 key 中提取来源侧依赖标识。
// 职责：从 componentKey 中提取较短的来源依赖 ID。
// 使用：目录名兜底和候选值生成时调用。
// 边界：显示名缺失时作为目录名兜底，避免直接生成 thunderstore-owner-name-1.0.0 这类完整 key 目录。
func (artifact Artifact) SourceDependencyID() string {
	return artifactlayout.SourceDependencyID(artifact.ComponentKey)
}
