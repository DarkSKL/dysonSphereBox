package forum

// 文件职责：映射社区论坛请求的机读错误原因与中文提示文案。
// 使用：由 helper/forum 和 service/forum 构造错误，供前端按 reason 处理鉴权、重试和冲突分支。
// 边界：固定对外错误外壳，不向前端透传未受控的底层诊断或服务端堆栈文本。

import "encoding/json"

// requestError 表示论坛桥接的稳定错误外壳，由 Wails 序列化为拒绝字符串。
// 职责：向前端交付结构化的失败原因和可读中文提示。
// 使用：failure 与 InvalidInput 构造，前端解析 JSON 后按 reason 分派提示。
// 边界：不携带敏感令牌或本机堆栈。
type requestError struct {
	// Reason 职责：机读错误原因码；使用：前端区分登录过期、资源下架、冲突与可重试故障；边界：固定集合。
	Reason string `json:"reason"`
	// Message 职责：可直接向用户展示的中文说明；使用：弹窗或提示条；边界：非空中文文本。
	Message string `json:"message"`
}

// Error 将错误外壳编码成 JSON 字符串。
// 职责：实现 Go error 接口，使 Wails 拒绝字符串保留结构化原因。
// 使用：由 Go 运行时与 Wails 桥接自动调用。
// 边界：仅序列化 Reason 和 Message。
func (err *requestError) Error() string {
	raw, _ := json.Marshal(err)
	return string(raw)
}

// failure 构造论坛桥接的统一结构化错误。
// 职责：封装错误原因和中文提示。
// 使用：helper/forum 内部各网络、文件与业务校验失败路径调用。
// 边界：reason 与 message 应由本机固定枚举与文案生成。
func failure(reason, message string) error {
	return &requestError{Reason: reason, Message: message}
}

// InvalidInput 返回客户端可判定的参数或文件校验错误。
// 职责：统一服务层和辅助层的入参错误格式。
// 使用：ForumService 入口参数检查和文件校验调用。
// 边界：使用 invalidInput 原因码，前端不自动清空草稿。
func InvalidInput(message string) error {
	return failure("invalidInput", message)
}

// ImportFailure 构造本地资源导入失败错误，并识别同名异内容冲突。
// 职责：把环境管理导入错误转成前端可识别的冲突或普通导入失败。
// 使用：ForumService 导入蓝图、存档或模组失败时调用。
// 边界：冲突类错误保留原本地文件，提示用户检查同名文件。
func ImportFailure(reason, message string) error {
	return failure(reason, message)
}

// businessFailure 把服务端论坛业务失败码与 reason 映射为稳定客户端错误。
// 职责：对照服务端 failureMessages 还原清晰可控的用户提示。
// 使用：DoJSON、DoMultipart 与下载失败分支解析 JSON 响应时调用。
// 边界：未知失败码回退为通用请求失败，不回显不可信原始字段。
func businessFailure(code int, raw json.RawMessage) error {
	if code == 7 {
		var data struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(raw, &data) == nil {
			switch data.Reason {
			case "invalid":
				return failure("invalid", "提交内容或正文资源标签不符合要求，请检查后重试")
			case "notFound":
				return failure("notFound", "目标内容不存在、已隐藏或资源已下架")
			case "categoryDisabled":
				return failure("categoryDisabled", "该分类已停用，请重新选择其他分类")
			case "saveTooLarge":
				return failure("saveTooLarge", "存档超过 512 MiB 限制")
			case "fileTooLarge":
				return failure("fileTooLarge", "上传文件超过该类型的大小限制")
			case "quotaExceeded":
				return failure("quotaExceeded", "当前账号托管文件已超过 2 GiB 容量上限")
			case "alreadyHandled":
				return failure("alreadyHandled", "该举报已处理")
			case "conflict":
				return failure("conflict", "请求标识已用于其他内容，或目标状态已变化")
			case "unavailable":
				return failure("unavailable", "论坛服务暂不可用，请稍后重试")
			}
		}
	}
	return failure("requestFailed", "论坛服务请求失败，请稍后重试")
}
