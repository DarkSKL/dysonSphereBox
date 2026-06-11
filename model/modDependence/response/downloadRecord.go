package response

// DownloadRecordRequest 表示下载计数记录接口请求体。
// 用途：官方源整合包开始真实下载前，由后端记录一次公开下载事件。
type DownloadRecordRequest struct {
	ResourceType  string                 `json:"resourceType"`
	ResourceID    uint                   `json:"resourceId"`
	ClientID      string                 `json:"clientId,omitempty"`
	ClientVersion string                 `json:"clientVersion,omitempty"`
	Channel       string                 `json:"channel,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// DownloadRecordResponse 表示下载计数记录接口响应。
// 边界：客户端只以 Code 是否为 0 判断是否可以继续下载。
type DownloadRecordResponse struct {
	Code int                `json:"code"`
	Data DownloadRecordData `json:"data"`
	Msg  string             `json:"msg"`
}

// DownloadRecordData 表示服务端返回的本次下载事件与累计次数。
type DownloadRecordData struct {
	EventID       uint   `json:"eventId"`
	ResourceType  string `json:"resourceType"`
	ResourceID    uint   `json:"resourceId"`
	DownloadCount int64  `json:"downloadCount"`
}
