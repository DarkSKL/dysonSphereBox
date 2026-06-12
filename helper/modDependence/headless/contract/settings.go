package contract

// 文件职责：定义无头服务器设置保存相关的跨层契约类型。
// 使用：headless helper 构造设置结果，service/modDependence 作为 Wails 返回值传给前端。
// 边界：只定义 DTO 形状，不写 nebula.cfg、不校验存档、不启动进程。

// HeadlessSettings 表示开服页可编辑的服务器启动配置。
// 职责：承载端口、启动存档和保存后的环境状态。
// 使用：HeadlessService.SaveSettings 返回给前端设置弹窗和状态卡片。
// 边界：端口同步到 nebula.cfg，存档仅写入本地配置，不校验存档实际存在。
type HeadlessSettings struct {
	// HostPort 职责：记录 Nebula 主机监听端口；使用：前端设置框和启动命令展示；边界：端口合法性由保存流程校验。
	HostPort int `json:"hostPort"`
	// ServerArchive 职责：记录启动时要加载的存档名或默认模式；使用：构建 DSPGAME.exe 启动参数；边界：不检查存档文件是否存在。
	ServerArchive string `json:"serverArchive"`
	// NebulaConfigPath 职责：记录 nebula.cfg 文件路径；使用：前端展示配置写入位置；边界：路径可能在未安装依赖时不存在。
	NebulaConfigPath string `json:"nebulaConfigPath"`
	// Status 职责：保存设置后的最新环境状态；使用：前端无需再次刷新即可更新卡片；边界：仅代表保存完成后的瞬时状态。
	Status HeadlessEnvironmentStatus `json:"status"`
}
