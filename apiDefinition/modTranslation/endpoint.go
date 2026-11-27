package modTranslation

// 文件职责：声明盒子服务器的批量翻译请求策略。
// 使用：Mod 市场翻译 helper 与请求目录。
// 边界：翻译实时提交并要求已验证会话。

import (
	"net/http"
	"time"

	api "DysonSphereBox/apiDefinition"
)

// TranslateBatchAPI 职责：定义有大小和时限约束的批量翻译；使用：市场翻译；边界：POST 请求不复用结果。
var TranslateBatchAPI = &api.ApiDesc{
	ID: "translation.batch", Description: "提交批量翻译", URL: APIURLTranslateBatch, Method: http.MethodPost,
	Auth: api.AuthVerified, Timeout: 30 * time.Second, RequestLimit: MaxRequestBytes,
	Envelope: api.EnvelopePolicy{RenewOnHTTPError: true},
}
