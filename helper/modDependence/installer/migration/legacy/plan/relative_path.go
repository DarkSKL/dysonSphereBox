package plan

// 文件职责：计算历史 BepInEx 裸露文件迁移后的相对路径。
// 使用：BuildScopedFileRepairPlans 为每条历史 installed 文件记录生成新目标路径前调用。
// 边界：只处理 BepInEx/plugins 与 BepInEx/patchers 的裸文件；已有子目录或无法识别的记录会被跳过。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"strings"
)

// repairScopedRelativePath 计算历史裸露文件应迁入的新相对路径。
// 职责：识别 BepInEx/plugins 或 BepInEx/patchers 下的裸文件并补上组件目录。
// 使用：构造迁移计划时调用。
// 边界：已有子目录、非 plugins/patchers、无法识别根目录的记录返回 false。
func repairScopedRelativePath(record modDependenceModel.ModInstalledFile) (string, bool) {
	normalizedRelativePath := artifactlayout.NormalizeEntryPath(record.RelativePath)
	root, rest, hasRoot, fromBepInEx := artifactlayout.SplitInstallRoot(normalizedRelativePath)
	if !hasRoot || !fromBepInEx || (root != "plugins" && root != "patchers") {
		return "", false
	}
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}

	artifact := installedFileRecordArtifact(record)
	return artifactlayout.ResolveScopedBepInExRelativePath(root, rest, artifact), true
}

// installedFileRecordArtifact 将 installed 文件记录转成布局算法需要的 artifact 描述。
// 职责：加载组件展示信息，补足历史迁移时的目录命名候选。
// 使用：repairScopedRelativePath 调用。
// 边界：组件记录缺失时仍使用 ComponentKey 兜底，不阻断迁移。
func installedFileRecordArtifact(record modDependenceModel.ModInstalledFile) artifactlayout.ArtifactDescriptor {
	component := modDependenceModel.ModInstalledComponent{}
	if core.GLOBAL.DB != nil && strings.TrimSpace(record.ComponentKey) != "" {
		_ = core.GLOBAL.DB.Where("component_key = ?", record.ComponentKey).Take(&component).Error
	}
	return artifactlayout.ArtifactDescriptor{
		ComponentKey: record.ComponentKey,
		Namespace:    component.Namespace,
		Name:         component.Name,
		DisplayName:  component.DisplayName,
		Version:      component.Version,
	}
}
