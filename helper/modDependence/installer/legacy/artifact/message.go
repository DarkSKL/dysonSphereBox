package artifact

// 文件职责：生成旧官方依赖安装结果的用户可读文案。
// 使用：InstallDependency 构造 DependencyInstallResult 时调用 dependencyInstallMessage。
// 边界：只生成展示文本，不影响安装状态、错误处理、审计日志或缓存策略。

// dependencyInstallMessage 生成旧依赖安装结果文案。
// 职责：根据是否命中 zip 缓存返回前端可读说明。
// 使用：InstallDependency 构造 DependencyInstallResult 时调用。
// 边界：只生成展示文案，不影响安装状态和日志等级。
func dependencyInstallMessage(usedCache bool) string {
	if usedCache {
		return "依赖安装完成，已优先使用本地缓存"
	}

	return "依赖安装完成"
}
