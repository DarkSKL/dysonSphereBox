package artifactlayout

// 文件职责：按 r2modman 为戴森球计划定义的安装规则，把 artifact 包内相对路径解析为游戏目录下的目标相对路径。
// 使用：installer 安装规划遍历解压文件时调用 ResolveTargetRelativePath；历史迁移修复裸露插件文件时调用 ResolveScopedBepInExRelativePath。
// 边界：只处理路径计算，不访问数据库、不读写文件、不决定安装冲突策略。
// 规则来源：r2modmanPlus ecosystem.json 中 dyson-sphere-program 的 installRules，与 InstallRulePluginInstaller 的匹配方式一致。

import (
	"path/filepath"
	"strings"
)

// InstallRule 描述一条 BepInEx 安装规则。
// 职责：声明规则目录、按扩展名归属的文件类型，以及是否按 Mod 名建立子目录。
// 使用：ResolveTargetRelativePath 匹配目录名或文件扩展名后生成目标路径。
// 边界：Subdir 为 false 的规则对应 R2 的 none 方式，文件直接落到规则目录且不纳入受管文件清单。
type InstallRule struct {
	// Route 职责：规则在游戏目录下的相对目录；使用：拼接目标路径；边界：使用 / 分隔。
	Route string
	// Extensions 职责：按扩展名归属到该规则的文件后缀；使用：包内散落文件匹配；边界：更长的后缀优先，例如 .mm.dll 优先于 .dll。
	Extensions []string
	// Subdir 职责：是否在规则目录下按 Mod 名建立子目录；使用：plugins/core/patchers/monomod 为 true；边界：config 为 false。
	Subdir bool
	// IsDefault 职责：标记未匹配任何规则的文件默认落点；使用：包内资源、说明等文件；边界：只能有一条默认规则。
	IsDefault bool
}

// installRules 对齐 R2 戴森球计划安装规则：plugins 为默认位置，config 不按 Mod 分目录。
var installRules = []InstallRule{
	{Route: "BepInEx/plugins", Extensions: []string{".dll"}, Subdir: true, IsDefault: true},
	{Route: "BepInEx/core", Subdir: true},
	{Route: "BepInEx/patchers", Subdir: true},
	{Route: "BepInEx/monomod", Extensions: []string{".mm.dll"}, Subdir: true},
	{Route: "BepInEx/config", Subdir: false},
}

// Target 表示单个包内文件的安装落点。
// 职责：同时返回目标路径与 R2 tracking method 对应的受管标记。
// 使用：安装规划决定冲突检测、安装记录和卸载行为时调用。
// 边界：Tracked 为 false 的 config 文件仍会复制，但不写入 installed file 清单。
type Target struct {
	// RelativePath 职责：游戏目录下的目标相对路径；使用：与游戏目录拼接；边界：调用方仍需做越界校验。
	RelativePath string
	// Tracked 职责：标记文件是否由安装器跟踪；使用：决定冲突检测、卸载和文件归属记录；边界：对应 R2 的 subdir tracking。
	Tracked bool
}

// ResolveTarget 按 R2 规则解析包内文件的目标相对路径。
// 职责：从最外层目录开始逐级匹配规则目录名，命中后保留其下的相对结构；都未命中时按文件扩展名归属并展平到 Mod 子目录。
// 使用：安装规划遍历解压文件时调用。
// 边界：目录名匹配大小写不敏感；未命中的中间目录会被丢弃，与 R2 的展平行为一致。
// 逻辑流：1. 归一化路径 => 2. 逐级匹配规则目录名 => 3. 未命中则按扩展名选择规则 => 4. 拼接规则目录、Mod 子目录与剩余路径。
func ResolveTarget(relativePath string, artifact ArtifactDescriptor) Target {
	segments := strings.Split(NormalizeEntryPath(relativePath), "/")
	for index := 0; index < len(segments)-1; index++ {
		if rule, ok := ruleForDirectory(segments[index]); ok {
			return rule.target(artifact, segments[index+1:])
		}
	}
	fileName := segments[len(segments)-1]
	return ruleForFile(fileName).target(artifact, []string{fileName})
}

// ResolveTargetRelativePath 返回包内文件的目标相对路径。
// 职责：兼容只需要路径的历史调用方。
// 使用：迁移和其它只处理路径的辅助流程调用。
// 边界：丢弃 Tracked 标记，返回值仍需由调用方做越界校验。
func ResolveTargetRelativePath(relativePath string, artifact ArtifactDescriptor) string {
	return ResolveTarget(relativePath, artifact).RelativePath
}

// ResolveScopedBepInExRelativePath 将 plugins 或 patchers 下的路径归入 Mod 子目录。
// 职责：为直接落在规则目录根部的文件补上 Mod 作用域。
// 使用：历史迁移修复早期裸露在 plugins/patchers 根目录的文件时调用。
// 边界：与 R2 一致，总是在规则目录下再包一层 Mod 目录，不识别包内已有的同名目录。
func ResolveScopedBepInExRelativePath(root string, rest string, artifact ArtifactDescriptor) string {
	parts := []string{"BepInEx", root, InstallDirName(artifact)}
	if rest != "" {
		parts = append(parts, filepath.FromSlash(rest))
	}
	return filepath.Join(parts...)
}

// ruleForDirectory 按目录名查找规则。
// 职责：对齐 R2 按规则目录 basename 匹配包内目录的方式。
// 使用：ResolveTargetRelativePath 逐级匹配目录时调用。
// 边界：大小写不敏感；戴森球规则的 basename 互不重复，无需 R2 的最佳匹配计算。
func ruleForDirectory(directoryName string) (InstallRule, bool) {
	for _, rule := range installRules {
		if strings.EqualFold(pathBase(rule.Route), directoryName) {
			return rule, true
		}
	}
	return InstallRule{}, false
}

// ruleForFile 按扩展名查找规则。
// 职责：选择后缀匹配最长的规则，没有匹配时返回默认规则。
// 使用：ResolveTargetRelativePath 处理散落文件时调用。
// 边界：后缀比较大小写不敏感。
func ruleForFile(fileName string) InstallRule {
	lowerName := strings.ToLower(fileName)
	matched := InstallRule{}
	matchedLength := 0
	for _, rule := range installRules {
		for _, extension := range rule.Extensions {
			if strings.HasSuffix(lowerName, strings.ToLower(extension)) && len(extension) > matchedLength {
				matched = rule
				matchedLength = len(extension)
			}
		}
	}
	if matchedLength > 0 {
		return matched
	}
	for _, rule := range installRules {
		if rule.IsDefault {
			return rule
		}
	}
	return installRules[0]
}

// target 拼接规则目录、Mod 子目录与剩余路径。
// 职责：生成最终目标相对路径。
// 使用：ResolveTargetRelativePath 命中规则后调用。
// 边界：rest 为包内规则目录以下的相对片段。
func (rule InstallRule) target(artifact ArtifactDescriptor, rest []string) Target {
	parts := []string{filepath.FromSlash(rule.Route)}
	if rule.Subdir {
		parts = append(parts, InstallDirName(artifact))
	}
	parts = append(parts, rest...)
	return Target{RelativePath: filepath.Join(parts...), Tracked: rule.Subdir}
}

// pathBase 返回 / 分隔路径的最后一段。
func pathBase(route string) string {
	if index := strings.LastIndex(route, "/"); index >= 0 {
		return route[index+1:]
	}
	return route
}
