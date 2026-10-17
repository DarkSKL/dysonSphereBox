package announcement

// 文件职责：描述前端可恢复公告失败，不暴露token或正文。
// 使用：HTTP和业务错误转换成Wails可识别的稳定kind。
// 边界：reason只来自协议字段，不解析msg做流程判断。

import "encoding/json"

// Failure 职责：携带稳定错误种类和中文提示；使用：Vue区分重试/鉴权/版本失效；边界：不作为持久已读事实。
type Failure struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Error 职责：编码Wails错误字符串；使用：绑定Promise拒绝；边界：只包含脱敏提示。
func (failure *Failure) Error() string { raw, _ := json.Marshal(failure); return string(raw) }

// fail 职责：创建稳定公告错误；使用：网络与契约流程；边界：不携原始响应。
func fail(kind, message string) error { return &Failure{Kind: kind, Message: message} }
