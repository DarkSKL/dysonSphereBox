package uninstall

// 文件职责：生成 V2 卸载文件结果的用户可读文案。
// 使用：正常卸载流程构造结果明细时调用 FileMessage。
// 边界：只生成展示文本，不影响共享判断、删除决策或审计日志。

// FileMessage 根据共享状态生成卸载文件结果文案。
// 职责：统一卸载结果中单文件的用户可读说明。
// 使用：正常卸载流程构造 DependencyInstallResult 时调用。
// 边界：只生成展示文本，不影响删除决策。
func FileMessage(shared bool) string {
	if shared {
		return "文件仍被其它包引用，已保留"
	}
	return "文件已移除"
}
