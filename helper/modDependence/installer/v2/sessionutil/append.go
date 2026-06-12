package sessionutil

// 文件职责：提供 V2 安装会话使用的纯字符串和路径去重追加工具。
// 使用：安装会话 manifest 维护 planned target dirs、component keys 等列表时调用。
// 边界：不理解安装会话结构、不访问数据库、不执行文件操作；路径相等判断委托 installpath。

import (
	"path/filepath"
	"strings"

	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
)

// AppendUniqueString 追加非空且未出现过的字符串。
// 职责：维护 component key 等简单列表的唯一性。
// 使用：清理 manifest 收集组件 key、缓存清理候选等场景调用。
// 边界：只做 TrimSpace 后的精确比较，不做大小写折叠或路径归一化。
func AppendUniqueString(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

// AppendUniqueTargetDir 追加非空且逻辑路径未出现过的目标目录。
// 职责：维护安装会话计划目标目录列表，兼容 Windows 大小写和斜杠差异。
// 使用：规划文件、清理兜底目录和 manifest 补全时调用。
// 边界：只比较路径字符串，不验证目录是否存在或是否位于游戏目录内。
func AppendUniqueTargetDir(values []string, next string) []string {
	next = strings.TrimSpace(filepath.Clean(next))
	if next == "" || next == "." {
		return values
	}
	for _, value := range values {
		if installpath.Equal(value, next) {
			return values
		}
	}
	return append(values, next)
}

// AppendOrReplacePlannedComponent 追加或替换计划组件目录。
// 职责：按 componentKey 保证计划组件列表唯一，并保留最新目录。
// 使用：解析 artifact 队列、补全清理 manifest 和从 raw 包恢复清理计划时调用。
// 边界：componentKey 或 InstallDir 为空时跳过，不访问文件系统。
func AppendOrReplacePlannedComponent(components []sessionstate.ComponentPlan, next sessionstate.ComponentPlan) []sessionstate.ComponentPlan {
	if strings.TrimSpace(next.ComponentKey) == "" || strings.TrimSpace(next.InstallDir) == "" {
		return components
	}
	result := make([]sessionstate.ComponentPlan, 0, len(components)+1)
	replaced := false
	for _, component := range components {
		if component.ComponentKey == next.ComponentKey {
			result = append(result, next)
			replaced = true
			continue
		}
		result = append(result, component)
	}
	if !replaced {
		result = append(result, next)
	}
	return result
}

// AppendOrReplaceSessionFile 追加或替换会话文件记录。
// 职责：按 packageKey、componentKey 和 targetPath 维护当前会话文件唯一性。
// 使用：复制成功、manifest 补全和失败清理准备阶段调用。
// 边界：只处理内存切片，不校验文件是否存在；路径比较委托 installpath。
func AppendOrReplaceSessionFile(files []sessionstate.File, next sessionstate.File) []sessionstate.File {
	result := make([]sessionstate.File, 0, len(files)+1)
	replaced := false
	for _, file := range files {
		if installpath.Equal(file.TargetPath, next.TargetPath) && file.PackageKey == next.PackageKey && file.ComponentKey == next.ComponentKey {
			result = append(result, next)
			replaced = true
			continue
		}
		result = append(result, file)
	}
	if !replaced {
		result = append(result, next)
	}
	return result
}
