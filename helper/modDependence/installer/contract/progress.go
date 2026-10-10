package contract

// 文件职责：定义 Mod 安装进度快照的跨层契约。
// 使用：installer 进度模块写入快照，service 层在前端刷新或重新进入页面时返回该结构。
// 边界：该文件只定义快照形状，不负责事件发送、进度计算、持久化查询或取消控制。

import "time"

// InstallProgressSnapshot 表示 Mod 安装器最近一次进度快照。
// 职责：承载当前进程和本地持久化状态中仍需前端展示的安装进度。
// 使用：前端页面刷新或重新激活后恢复正在安装、失败、冲突等待处理等临时状态。
// 边界：该快照不是安装记录模型；应用重启后的真实安装状态仍以 SQLite 安装记录为准。
type InstallProgressSnapshot struct {
	// PackageKey 职责：记录进度所属包 key；使用：前端定位卡片和继续冲突安装；边界：与 RemoteModPackageDTO.Key 一致。
	PackageKey string `json:"packageKey"`
	// Stage 职责：记录当前安装阶段；使用：前端展示下载、解压、复制、记录等阶段；边界：只作展示和排查。
	Stage string `json:"stage"`
	// Status 职责：记录阶段状态文本；使用：前端进度条旁展示；边界：不作为状态机唯一来源。
	Status string `json:"status"`
	// Percent 职责：记录整体百分比；使用：前端进度条展示；边界：会被限制在 0 到 100。
	Percent int `json:"percent"`
	// Current 职责：记录当前处理字节或数量；使用：下载读写进度展示；边界：非下载阶段可为 0。
	Current int64 `json:"current"`
	// Total 职责：记录总字节或总数量；使用：下载读写进度展示；边界：未知总量时可为 0。
	Total int64 `json:"total"`
	// BytesPerSecond 职责：记录当前下载速度；使用：前端展示实时速度；边界：非下载阶段为 0。
	BytesPerSecond int64 `json:"bytesPerSecond"`
	// Message 职责：保存用户可读进度说明；使用：前端展示当前动作；边界：不用于业务分支。
	Message string `json:"message"`
	// State 职责：记录安装状态机状态；使用：前端区分 running、failed、conflict 等状态；边界：具体合法值由 installer 进度模块维护。
	State string `json:"state"`
	// ErrorMessage 职责：保存失败或冲突时的错误说明；使用：前端提示和恢复安装排查；边界：成功状态为空。
	ErrorMessage string `json:"errorMessage"`
	// Conflict 职责：保存待处理冲突详情；使用：前端冲突弹窗和继续安装入口；边界：只有 conflict 状态才应存在。
	Conflict *InstallConflictDTO `json:"conflict,omitempty"`
	// UpdatedAt 职责：记录快照更新时间；使用：前端排序和判断状态新旧；边界：运行时快照使用当前进程时间。
	UpdatedAt time.Time `json:"updatedAt" ts_type:"string"`
}
