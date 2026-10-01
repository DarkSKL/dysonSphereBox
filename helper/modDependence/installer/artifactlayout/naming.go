package artifactlayout

// 文件职责：为 artifact 选择稳定的安装目录名（对齐 r2modman 的 Owner-Name 命名）。
// 使用：ResolveTargetRelativePath、安装会话清理和历史路径迁移共享这些命名规则。
// 边界：只处理目录名候选和片段比较，不拼接完整目标路径、不访问文件系统。

import (
	"strings"

	"DysonSphereBox/helper/modDependence/installer/shared"
)

// InstallDirName 选择 artifact 的最终安装目录名。
// 职责：优先使用与 r2modman 一致的 Owner-Name 目录名，其次从显示名、原名、来源依赖 ID、组件 key 中选择第一个可清洗候选。
// 使用：安装规划、会话清理和历史迁移需要生成组件目录时调用。
// 边界：所有候选无效时返回 artifact。
func InstallDirName(artifact ArtifactDescriptor) string {
	for _, candidate := range []string{ModName(artifact), artifact.DisplayName, artifact.Name, SourceDependencyID(artifact.ComponentKey), artifact.ComponentKey} {
		if sanitized := shared.SanitizePathSegment(candidate, "artifact"); sanitized != "artifact" {
			return sanitized
		}
	}
	return "artifact"
}

// ModName 返回 artifact 的 Owner-Name 形式 Mod 名。
// 职责：对齐 r2modman 的 mod.getName()，作为插件子目录名与“同一 Mod”判断依据。
// 使用：InstallDirName 优先调用；命名空间缺失时从 thunderstore:owner:name:version 形式的 key 解析。
// 边界：无法得到命名空间时返回空字符串，由调用方回退到其它候选。
func ModName(artifact ArtifactDescriptor) string {
	namespace := strings.TrimSpace(artifact.Namespace)
	name := strings.TrimSpace(artifact.Name)
	if namespace == "" || name == "" {
		keyNamespace, keyName, ok := SplitThunderstoreKey(artifact.ComponentKey)
		if !ok {
			return ""
		}
		namespace, name = keyNamespace, keyName
	}
	return namespace + "-" + name
}

// SplitThunderstoreKey 从 thunderstore:owner:name:version 形式的统一 key 中解析 owner 与 name。
// 职责：为目录命名和同一 Mod 判断提供不依赖数据库字段的解析。
// 使用：ModName 与安装器的同 Mod 识别调用。
// 边界：非 Thunderstore key 或片段不足时返回 false。
func SplitThunderstoreKey(componentKey string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(componentKey), ":")
	if len(parts) != 4 || parts[0] != "thunderstore" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// SourceDependencyID 从统一组件 key 中提取来源侧依赖标识。
// 职责：为显示名缺失的依赖提供更短的目录名候选。
// 使用：InstallDirName 调用。
// 边界：key 片段不足时返回空字符串，不猜测目录名。
func SourceDependencyID(componentKey string) string {
	parts := strings.Split(componentKey, ":")
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return ""
}
