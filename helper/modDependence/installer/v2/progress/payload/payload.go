package payload

// 文件职责：定义 V2 安装进度事件的运行时载荷。
// 使用：安装、卸载、下载进度、冲突和失败流程通过 Payload 传递状态给事件发送、快照缓存和数据库同步模块。
// 边界：只定义进度载荷形状，不发送 Wails 事件、不写数据库、不读取安装记录。

import "DysonSphereBox/helper/modDependence/installer/contract"

// Payload 表示一次待发送或待保存的安装进度事件。
// 职责：承载运行时进度事件中的阶段、速度、状态、错误和冲突信息。
// 使用：event.Emit、snapshot.Store 和 reader.New 构造 Wails 事件及快照。
// 边界：只服务 V2 安装进度链路；对外查询契约使用 contract.InstallProgressSnapshot。
type Payload struct {
	// PackageKey 职责：记录进度所属包 key；使用：事件路由和快照覆盖；边界：不能为空，否则无法索引快照。
	PackageKey string `json:"packageKey"`
	// Stage 职责：记录当前阶段；使用：前端展示进度所在步骤；边界：只作展示。
	Stage string `json:"stage"`
	// Status 职责：记录阶段状态；使用：进度条文字和持久化状态；边界：不作为唯一状态机。
	Status string `json:"status"`
	// Percent 职责：记录整体百分比；使用：进度条展示；边界：写入前通过 progresscalc.Clamp 限制。
	Percent int `json:"percent"`
	// Current 职责：记录当前字节或数量；使用：下载速度展示；边界：非下载阶段可为 0。
	Current int64 `json:"current"`
	// Total 职责：记录总字节或数量；使用：下载速度展示；边界：未知时可为 0。
	Total int64 `json:"total"`
	// BytesPerSecond 职责：记录实时速度；使用：前端展示下载速率；边界：非下载阶段为 0。
	BytesPerSecond int64 `json:"bytesPerSecond"`
	// Message 职责：记录用户可读说明；使用：事件提示和失败恢复；边界：不用于程序分支。
	Message string `json:"message"`
	// State 职责：记录安装状态机值；使用：前端区分 running、failed、conflict 等状态；边界：空值由发送阶段补 running。
	State string `json:"state"`
	// ErrorMessage 职责：记录失败说明；使用：失败快照和审计排查；边界：成功流程为空。
	ErrorMessage string `json:"errorMessage"`
	// Conflict 职责：记录待处理冲突；使用：前端冲突弹窗；边界：仅 conflict 状态应携带。
	Conflict *contract.InstallConflictDTO `json:"conflict,omitempty"`
}
