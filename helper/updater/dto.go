package updater

// 文件职责：定义应用自更新 helper 返回给系统服务和前端绑定的结果 DTO。
// 使用：service/system/update.go 通过 Wails 服务返回 CheckResult 与 UpdateResult。
// 边界：只描述更新检查和应用结果，不包含下载流、校验文件内容或本地写入细节。

// CheckResult 是前端展示应用更新状态的数据结构。
// 职责：承载当前版本、最新版本、目标资产和发布说明等检查结果。
// 使用：由系统更新服务返回给前端更新页面展示是否存在可用版本。
// 边界：只包含检查结果和目标 Release 元数据，不包含下载内容或本地写入状态。
type CheckResult struct {
	// CurrentVersion 是当前运行中的应用版本。
	// 职责：帮助前端对比本地版本和远端版本。
	// 使用：CheckAppUpdate 从 core.Version 写入。
	// 边界：dev 或空版本会被视为可更新，但字段本身不做比较。
	CurrentVersion string `json:"currentVersion"`
	// LatestVersion 是远端 Release 版本。
	// 职责：描述被检测到的目标版本号。
	// 使用：前端提示用户将要更新到哪个版本。
	// 边界：未发现 Release 时为空，不代表检测失败。
	LatestVersion string `json:"latestVersion"`
	// HasUpdate 表示远端版本是否新于当前版本。
	// 职责：作为前端展示更新按钮和服务层继续下载的布尔判断。
	// 使用：CheckAppUpdate 通过 release.IsNewer 计算。
	// 边界：只表示版本比较结果，不保证资产下载和校验一定成功。
	HasUpdate bool `json:"hasUpdate"`
	// AssetName 是匹配当前平台的 Release 资产名称。
	// 职责：展示和日志定位即将下载的更新包。
	// 使用：由 go-selfupdate 选择资产后写入。
	// 边界：字段为空时前端不应直接触发下载。
	AssetName string `json:"assetName"`
	// AssetURL 是匹配资产的下载地址。
	// 职责：提供前端展示和调试远端资源定位信息。
	// 使用：检查结果展示，不由前端直接下载。
	// 边界：实际下载仍由后端 gitee.Source 执行并校验。
	AssetURL string `json:"assetUrl"`
	// ReleaseName 是 Gitee Release 展示名称。
	// 职责：提供用户可读的发布标题。
	// 使用：前端更新弹窗或设置页展示。
	// 边界：为空时不影响更新流程，可回退展示版本号。
	ReleaseName string `json:"releaseName"`
	// ReleaseNotes 是 Gitee Release 发布说明。
	// 职责：提供更新内容摘要。
	// 使用：前端展示给用户决定是否更新。
	// 边界：不解析 Markdown，不参与版本判断。
	ReleaseNotes string `json:"releaseNotes"`
	// ReleaseURL 是 Gitee Release 页面地址。
	// 职责：提供用户跳转查看完整发布信息的入口。
	// 使用：前端可作为外链展示。
	// 边界：为空时不阻塞更新下载。
	ReleaseURL string `json:"releaseUrl"`
	// PublishedAt 是格式化后的发布时间。
	// 职责：向前端提供本地时区友好的发布时间文本。
	// 使用：CheckAppUpdate 调用 release.FormatTime 写入。
	// 边界：远端时间缺失时为空字符串。
	PublishedAt string `json:"publishedAt"`
	// OS 是当前运行平台的操作系统标识。
	// 职责：帮助前端说明本次检查匹配的目标平台。
	// 使用：从 runtime.GOOS 写入。
	// 边界：不用于前端选择资产，资产选择由 go-selfupdate 完成。
	OS string `json:"os"`
	// Arch 是当前运行平台的 CPU 架构标识。
	// 职责：帮助前端说明本次检查匹配的目标架构。
	// 使用：从 runtime.GOARCH 写入。
	// 边界：不用于前端选择资产，资产选择由 go-selfupdate 完成。
	Arch string `json:"arch"`
}

// UpdateResult 是执行自更新后的返回结果。
// 职责：承载自更新是否写入成功、目标版本和检查快照。
// 使用：由系统更新服务返回给前端，用于提示用户是否需要重启。
// 边界：写入成功不代表当前进程已切换版本，新版本需要重启后生效。
type UpdateResult struct {
	// Updated 表示本轮是否实际写入了新的可执行文件。
	// 职责：区分无需更新和已完成写入两种结果。
	// 使用：前端据此显示“已是最新”或“重启生效”。
	// 边界：为 true 仍不代表当前进程已运行新版本。
	Updated bool `json:"updated"`
	// TargetVersion 是本轮更新目标版本。
	// 职责：告知前端重启后预计运行的版本。
	// 使用：UpdateAppSelf 根据 Release 或检查结果写入。
	// 边界：无可用更新时可能为空，不能作为成功更新的唯一判断。
	TargetVersion string `json:"targetVersion"`
	// Message 是面向用户的更新结果提示。
	// 职责：提供简短的操作反馈。
	// 使用：前端直接展示给用户。
	// 边界：不承载机器可读状态，机器判断应使用 Updated。
	Message string `json:"message"`
	// Check 是执行更新前的检查快照。
	// 职责：保留版本、资产和发布时间等上下文。
	// 使用：前端在更新完成后继续展示目标 Release 信息。
	// 边界：不记录下载校验细节或写入路径。
	Check CheckResult `json:"check"`
}
