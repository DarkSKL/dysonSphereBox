package build

// 文件职责：标识直连候选在入口策略生效前失败的阶段；使用：工作流判断切换入口是否可能改变结果。
// 边界：保留原始错误及取消身份，不缓存失败、不证明其它布局无解，也不参与物理规则。

// ConnectedPreparationError 职责：说明共同设备/端口排布尚未成功；使用：直连核心包裹prepareConnectedRows失败。
// 边界：只有适配层确认原因为已声明领域无解后才可参与调度；未知错误仍为致命错误。
type ConnectedPreparationError struct {
	// Cause 职责：保留原始分类和原因链；使用：errors.Is/As及诊断；边界：不替换取消或内核故障。
	Cause error
}

// Error 职责：保持原始失败文本；使用：正常error展示；边界：阶段类别由工作流适配层明确发布。
func (e *ConnectedPreparationError) Error() string { return e.Cause.Error() }

// Unwrap 职责：保持原始错误身份；使用：取消与领域错误检查；边界：阶段标记本身不授予重试权限。
func (e *ConnectedPreparationError) Unwrap() error { return e.Cause }
