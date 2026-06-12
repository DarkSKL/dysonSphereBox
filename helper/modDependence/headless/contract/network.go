package contract

// 文件职责：定义无头服务器网络速率采样的返回契约。
// 使用：network.InspectStatus 构造 HeadlessNetworkStatus，并由环境状态 DTO 聚合返回前端。
// 边界：只描述采样结果，不读取系统计数器、不计算速率、不保存历史到数据库。

// HeadlessNetworkSample 表示一次真实网卡速率采样点。
// 职责：记录一次网络速率采样的时间标签和上下行速率。
// 使用：HeadlessNetworkStatus.Samples 提供给前端折线图。
// 边界：首次检测没有上一帧时速率为 0，样本窗口只保存在当前进程内。
type HeadlessNetworkSample struct {
	// Label 职责：保存用于图表横轴的时间文本；使用：前端折线图展示；边界：不用于排序。
	Label string `json:"label"`
	// CapturedAtUnix 职责：保存采样 Unix 秒级时间；使用：计算样本窗口长度；边界：不是数据库持久化时间。
	CapturedAtUnix int64 `json:"capturedAtUnix"`
	// UploadMBps 职责：记录上传速率 MB/s；使用：前端图表和峰值计算；边界：计数器失败时为 0。
	UploadMBps float64 `json:"uploadMBps"`
	// DownloadMBps 职责：记录下载速率 MB/s；使用：前端图表和峰值计算；边界：计数器失败时为 0。
	DownloadMBps float64 `json:"downloadMBps"`
}

// HeadlessNetworkStatus 表示基于系统网卡计数器计算出的网络速率。
// 职责：聚合当前速率、峰值、样本窗口和读取错误。
// 使用：InspectHeadlessEnvironment 每次状态刷新时返回给开服页。
// 边界：读取失败时保持 Valid=false，不使用模拟数据。
type HeadlessNetworkStatus struct {
	// Valid 职责：表示本次网络计数器读取是否成功；使用：前端决定是否展示错误；边界：失败时仍保留历史样本。
	Valid bool `json:"valid"`
	// UploadBytesPerSecond 职责：记录上传字节速率；使用：精确数值展示或换算；边界：首次采样为 0。
	UploadBytesPerSecond float64 `json:"uploadBytesPerSecond"`
	// DownloadBytesPerSecond 职责：记录下载字节速率；使用：精确数值展示或换算；边界：首次采样为 0。
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	// UploadMBps 职责：记录上传 MB/s；使用：前端主要展示值；边界：来自字节速率换算。
	UploadMBps float64 `json:"uploadMBps"`
	// DownloadMBps 职责：记录下载 MB/s；使用：前端主要展示值；边界：来自字节速率换算。
	DownloadMBps float64 `json:"downloadMBps"`
	// UploadPeakMBps 职责：记录当前样本窗口上传峰值；使用：前端显示峰值；边界：仅基于内存样本窗口。
	UploadPeakMBps float64 `json:"uploadPeakMBps"`
	// DownloadPeakMBps 职责：记录当前样本窗口下载峰值；使用：前端显示峰值；边界：仅基于内存样本窗口。
	DownloadPeakMBps float64 `json:"downloadPeakMBps"`
	// SampleWindowSeconds 职责：记录样本窗口跨度；使用：前端说明图表覆盖时长；边界：样本不足时为 0。
	SampleWindowSeconds int `json:"sampleWindowSeconds"`
	// Samples 职责：保存最近网络速率样本；使用：前端绘制折线图；边界：当前实现最多保留 12 个样本。
	Samples []HeadlessNetworkSample `json:"samples"`
	// Error 职责：保存计数器读取失败原因；使用：前端或日志展示；边界：Valid=true 时为空。
	Error string `json:"error"`
}
