package system

// 文件职责：提供系统日志构造时的错误文本归一化能力。
// 使用：审计日志 helper 在写入 LogEntry.Error 字段前调用，避免 nil error 解引用。
// 边界：只转换错误文本，不生成业务兜底文案、不包装错误、不记录日志。

// errorText 将可空错误转换为日志文本。
// 职责：避免审计日志构造时对 nil error 解引用。
// 使用：失败日志构造函数写入 Error 字段前调用。
// 边界：nil 返回空字符串，不生成兜底错误文案。
func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
