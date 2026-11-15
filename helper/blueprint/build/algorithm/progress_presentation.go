package build

// 文件职责：将候选搜索内部诊断与状态栏的用户文案分开。
// 使用：reportProgress发布真实阶段事件；边界：不调整阶段、计数、预览协议或生成策略，不把单候选失败当成整次失败。

// progressPresentation 职责：为候选比较提供不重复计数的简短文案；使用：普通阶段事件；边界：未知阶段沿用原文，原始诊断原样保留供展开查看。
func progressPresentation(stage, original string) (message, detail string) {
	switch stage {
	case "flow-layout":
		message = "正在比较传送带布局"
	case "fly-layout":
		message = "正在比较物流配送布局"
	case "layout-rejected":
		message = "当前候选未通过检查，正在比较其他布局"
	case "polar-rejected":
		message = "当前球面候选未通过检查，正在尝试其他排布"
	default:
		return original, ""
	}
	if message != original {
		detail = original
	}
	return
}
