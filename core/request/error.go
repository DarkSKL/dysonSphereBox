package request

// 文件职责：区分请求阶段失败，供领域层映射用户提示。
// 使用：认证、HTTP 适配和业务接口。
// 边界：通用错误文本不输出路由参数、响应内容或凭据。

import (
	"encoding/json"
	"fmt"
)

// ErrorKind 职责：识别失败阶段；使用：业务错误映射；边界：HTTP 状态和业务码分别保存。
type ErrorKind string

// 失败类别覆盖参数、会话、传输和响应校验，HTTP 与业务失败可携带协议字段。
const (
	InvalidInput     ErrorKind = "invalidInput"
	BuildFailed      ErrorKind = "build"
	Unavailable      ErrorKind = "unavailable"
	AuthRequired     ErrorKind = "authRequired"
	Unauthorized     ErrorKind = "unauthorized"
	PasswordRequired ErrorKind = "passwordRequired"
	Stale            ErrorKind = "stale"
	TransportFailed  ErrorKind = "transport"
	ReadFailed       ErrorKind = "read"
	InvalidEnvelope  ErrorKind = "envelope"
	InvalidData      ErrorKind = "data"
	HTTPFailed       ErrorKind = "http"
	BusinessFailed   ErrorKind = "business"
)

// Error 职责：提供可映射的请求失败；使用：Go 业务层判断错误种类；边界：Message 与 Data 不进入默认错误文本。
type Error struct {
	Kind    ErrorKind
	Status  int
	Code    int
	Message string
	Data    json.RawMessage
	// Protocol 标识失败响应带有可识别的外壳。
	Protocol bool
}

// Error 职责：生成只包含阶段和码值的错误；使用：日志和内部诊断；边界：业务按需提供本地用户提示。
func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("请求失败 %s (HTTP %d, code %d)", e.Kind, e.Status, e.Code)
	}
	return fmt.Sprintf("请求失败 %s", e.Kind)
}
