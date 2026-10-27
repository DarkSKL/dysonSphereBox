package quant

// 文件职责：定义稳定量化错误；使用：纯引擎向服务报告失败；边界：不把失败伪装成部分求解。

// Error 职责：保存错误码与诊断；使用：服务映射；边界：不包含文件或账号信息。
type Error struct {
	// Code 职责：错误分类；使用：前端分支；边界：稳定字符串。
	Code string
	// Message 职责：中文原因；使用：直接展示；边界：无蓝图原文。
	Message string
}

// Error 职责：标准错误文本；使用：日志和链路；边界：不改变分类。
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// qerror 职责：构建引擎错误；使用：验证/求解失败；边界：无副作用。
func qerror(code, message string) error { return &Error{code, message} }
