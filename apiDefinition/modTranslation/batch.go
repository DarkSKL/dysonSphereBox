package modTranslation

// 文件职责：声明中文批量翻译的服务器契约。
// 使用：翻译 helper 构造请求，市场服务经 Wails 返回批次数据。
// 边界：不发送请求、不持有凭据；条目必须与原文等长同序。

import "encoding/json"

// APIURLTranslateBatch 职责：定义翻译相对路径；使用：由 ServerClient 拼接路由前缀；边界：仅支持认证 POST。
const APIURLTranslateBatch = "/translation/translateBatch"

// MaxBatchSize 职责：限定批次项数；使用：服务入口校验；边界：不能通过拆批在此处自动重试。
const MaxBatchSize = 20

// MaxTextRunes 职责：限定单项码点数；使用：客户端提前排除非法描述；边界：不归一化原文。
const MaxTextRunes = 2000

// MaxRequestBytes 职责：限定编码后请求体；使用：发出请求前检查；边界：与服务器的 128 KiB 上限一致。
const MaxRequestBytes = 128 << 10

// BatchRequest 职责：承载原文列表；使用：编码服务器请求；边界：保留重复项及字节差异。
type BatchRequest struct {
	// Texts 职责：保存待翻译原文；使用：按下标配对响应；边界：最多二十项。
	Texts []string `json:"texts"`
}

// Item 职责：承载单项翻译状态；使用：前端按请求次序显示；边界：失败始终使用客户端原文。
type Item struct {
	// Status 职责：区分 ready/error；使用：决定是否展示译文；边界：未知值按失败处理。
	Status string `json:"status"`
	// Reason 职责：携带条目级原因；使用：提供手动重试提示；边界：不包含供应商详情。
	Reason string `json:"reason"`
	// Data 职责：承载译文或原文；使用：纯文本展示；边界：不作为 HTML。
	Data string `json:"data"`
}

// BatchData 职责：承载有序翻译结果；使用：Wails 返回市场页；边界：长度不符整批失败。
type BatchData struct {
	// Items 职责：保存有序条目；使用：按下标配对原文；边界：不按包标识排序。
	Items []Item `json:"items"`
}

// BatchResponse 职责：描述统一服务器外壳；使用：helper 有界解码；边界：缺失 code 不等于成功。
type BatchResponse struct {
	// Code 职责：描述业务结果；使用：HTTP 200 且零值成功；边界：必须明确返回。
	Code *int `json:"code"`
	// Data 职责：保存批次正文；使用：成功后解码 BatchData；边界：错误正文不返回前端。
	Data json.RawMessage `json:"data"`
	// Msg 职责：描述服务端消息；使用：保持外壳契约；边界：不显示或记录原始消息。
	Msg string `json:"msg"`
}
