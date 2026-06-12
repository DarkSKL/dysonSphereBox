package modDependence

// 文件职责：定义外部接口契约和前后端传输 DTO。
// 使用：由 service/helper 在请求远端接口或生成 Wails 类型时引用。
// 边界：不发起网络请求，不读写数据库，不承载业务决策。

// DownloadRecordRequest 表示下载计数记录接口请求体。
// 职责：承载官方下载计数接口所需的资源身份和客户端上下文。
// 使用：官方源整合包开始真实下载前，由后端记录一次公开下载事件。
// 边界：只作为请求 DTO，不校验资源是否真实存在，不写入本地安装记录。
type DownloadRecordRequest struct {
	// ResourceType 职责：标记服务端统计的资源类型；使用：官方下载计数接口区分整合包等资源；边界：不由本地模型推断，调用方必须传入明确值。
	ResourceType string `json:"resourceType"`
	// ResourceID 职责：保存来源侧资源数字 ID；使用：官方源下载计数定位资源；边界：非官方来源不应调用该接口。
	ResourceID uint `json:"resourceId"`
	// ClientID 职责：保存可选客户端身份；使用：后续统计扩展可传入；边界：为空时服务端仍应按匿名事件处理。
	ClientID string `json:"clientId,omitempty"`
	// ClientVersion 职责：保存可选客户端版本；使用：服务端统计不同应用版本下载行为；边界：为空时不阻塞下载计数。
	ClientVersion string `json:"clientVersion,omitempty"`
	// Channel 职责：保存下载渠道；使用：区分 windows、manual 等客户端场景；边界：只作为统计维度，不影响本地安装流程。
	Channel string `json:"channel,omitempty"`
	// Extra 职责：保存下载事件扩展上下文；使用：记录包 key、版本、原始 URL 等排障信息；边界：不得依赖 Extra 作为服务端主键。
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// DownloadRecordResponse 表示下载计数记录接口响应。
// 职责：承载官方下载计数接口的业务状态码、消息和数据。
// 使用：安装器调用下载计数接口后解码该结构判断是否继续。
// 边界：客户端只以 Code 是否为 0 判断是否可以继续下载。
type DownloadRecordResponse struct {
	// Code 职责：保存服务端业务状态码；使用：安装器判断记录是否成功；边界：HTTP 成功但 Code 非 0 时仍视为失败。
	Code int `json:"code"`
	// Data 职责：保存服务端接受事件后的统计结果；使用：成功后可用于日志或提示；边界：失败响应中可能为空值。
	Data DownloadRecordData `json:"data"`
	// Msg 职责：保存服务端业务消息；使用：失败时拼接错误信息；边界：成功时不参与安装决策。
	Msg string `json:"msg"`
}

// DownloadRecordData 表示服务端返回的本次下载事件与累计次数。
// 职责：描述一次下载记录事件和资源累计下载次数。
// 使用：下载记录成功后用于日志、调试或后续展示扩展。
// 边界：不代表本地下载文件已成功完成，只代表远端记录接口已接受事件。
type DownloadRecordData struct {
	// EventID 职责：保存服务端创建的下载事件 ID；使用：日志或排障时回溯服务端记录；边界：不作为本地安装记录 ID。
	EventID uint `json:"eventId"`
	// ResourceType 职责：回显下载事件资源类型；使用：校验和日志展示；边界：不改变本地安装流程。
	ResourceType string `json:"resourceType"`
	// ResourceID 职责：回显下载事件资源 ID；使用：日志展示和排障；边界：不作为统一包 Key。
	ResourceID uint `json:"resourceId"`
	// DownloadCount 职责：保存服务端资源累计下载次数；使用：日志或后续展示扩展；边界：不代表本次 zip 已下载完成。
	DownloadCount int64 `json:"downloadCount"`
}
