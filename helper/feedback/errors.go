package feedback

// 文件职责：投影星系请求失败的机读原因和用户提示。
// 使用：Go 请求层和 Wails 服务返回错误，页面从拒绝字符串解析。
// 边界：忽略未经筛选的服务端提示与响应字段。

import "encoding/json"

// failure 生成 Wails 可序列化的错误字符串，只交付本机选定的原因和提示。
func failure(reason, message string) error {
	return &requestError{Reason: reason, Message: message}
}

// requestError 表示星系桥接的稳定错误外壳，由 Wails 转成拒绝字符串。
type requestError struct {
	// Reason 给页面处理鉴权、冲突与重试分支，取值来自首版契约或本机传输状态。
	Reason string `json:"reason"`
	// Message 提供可直接显示的中文信息，不回显服务端原始响应。
	Message string `json:"message"`
}

// Error 将错误外壳编码成 JSON，供 Wails Promise 拒绝时原样传递。
func (err *requestError) Error() string {
	raw, _ := json.Marshal(err)
	return string(raw)
}

// InvalidInput 返回客户端可判定的参数错误，与服务端 invalidInput 使用同一原因。
func InvalidInput(message string) error {
	return failure("invalidInput", message)
}

// businessFailure 将冻结的服务端原因映射为固定消息，未知响应采用统一失败提示。
func businessFailure(code int, raw json.RawMessage) error {
	if code == 7 {
		var data struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(raw, &data) == nil {
			switch data.Reason {
			case "invalidInput":
				return InvalidInput("反馈内容或参数无效")
			case "unavailable":
				return failure(data.Reason, "该反馈已不可用")
			case "versionChanged":
				return failure(data.Reason, "反馈版本已变化，请刷新后再试")
			case "editLocked":
				return failure(data.Reason, "该星星的正文已锁定")
			case "requestConflict":
				return failure(data.Reason, "该请求标识已用于其他内容")
			}
		}
	}
	return failure("requestFailed", "反馈服务请求失败，请稍后重试")
}
