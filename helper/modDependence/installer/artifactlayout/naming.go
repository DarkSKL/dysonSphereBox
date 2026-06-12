package artifactlayout

// 文件职责：为 artifact 选择稳定的安装目录名并判断路径是否已有组件目录前缀。
// 使用：ResolveScopedBepInExRelativePath、安装会话清理和历史路径迁移共享这些命名规则。
// 边界：只处理目录名候选和片段比较，不拼接完整目标路径、不访问文件系统。

import (
	"strings"

	"DysonSphereBox/helper/modDependence/installer/shared"
)

// HasArtifactDirectoryPrefix 判断相对路径是否已包含组件目录前缀。
// 职责：避免把已经按组件目录组织的 plugins/patchers 文件重复嵌套。
// 使用：ResolveScopedBepInExRelativePath 调用。
// 边界：只有存在首段和后续路径时才视为可能包含目录前缀。
func HasArtifactDirectoryPrefix(relativePath string, artifact ArtifactDescriptor) bool {
	firstSegment, rest := SplitFirstPathSegment(relativePath)
	if firstSegment == "" || rest == "" {
		return false
	}
	for _, candidate := range DirectoryCandidateNames(artifact) {
		if SamePathSegment(firstSegment, candidate) {
			return true
		}
	}
	return false
}

// DirectoryCandidateNames 返回 artifact 可作为目录名的候选值。
// 职责：按显示名、原名、来源依赖 ID、组件 key 的顺序生成候选。
// 使用：目录前缀识别和 InstallDirName 兜底策略共享。
// 边界：返回值未清洗，比较或落盘前需要调用路径片段清洗。
func DirectoryCandidateNames(artifact ArtifactDescriptor) []string {
	return []string{
		InstallDirName(artifact),
		artifact.DisplayName,
		artifact.Name,
		SourceDependencyID(artifact.ComponentKey),
		artifact.ComponentKey,
	}
}

// SamePathSegment 判断两个路径片段清洗后是否相同。
// 职责：用安装器路径片段规则进行大小写无关比较。
// 使用：目录前缀识别和迁移判断时调用。
// 边界：任一片段清洗为空时返回 false。
func SamePathSegment(left string, right string) bool {
	left = strings.ToLower(shared.SanitizePathSegment(left, ""))
	right = strings.ToLower(shared.SanitizePathSegment(right, ""))
	return left != "" && left == right
}

// InstallDirName 选择 artifact 的最终安装目录名。
// 职责：从显示名、原名、来源依赖 ID、组件 key 中选择第一个可清洗候选。
// 使用：安装规划、会话清理和历史迁移需要生成组件目录时调用。
// 边界：所有候选无效时返回 artifact。
func InstallDirName(artifact ArtifactDescriptor) string {
	for _, candidate := range []string{artifact.DisplayName, artifact.Name, SourceDependencyID(artifact.ComponentKey), artifact.ComponentKey} {
		if sanitized := shared.SanitizePathSegment(candidate, "artifact"); sanitized != "artifact" {
			return sanitized
		}
	}
	return "artifact"
}

// SourceDependencyID 从统一组件 key 中提取来源侧依赖标识。
// 职责：为显示名缺失的依赖提供更短的目录名候选。
// 使用：InstallDirName 与 DirectoryCandidateNames 调用。
// 边界：key 片段不足时返回空字符串，不猜测目录名。
func SourceDependencyID(componentKey string) string {
	parts := strings.Split(componentKey, ":")
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return ""
}
