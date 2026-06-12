package headless

// 文件职责：维护无头服务器状态观测所需的内存快照结构。
// 使用：RecordHeadlessStatusObservation 从环境状态中提取快照并与上一次状态比较。
// 边界：快照只存在当前进程内，不持久化、不作为前端 DTO、不用于控制进程。

import headlessContract "DysonSphereBox/helper/modDependence/headless/contract"

// headlessStatusSnapshot 保存一次无头服务器观测的关键状态。
// 职责：捕获进程、端口、协议和目标端口信息，用于比较状态变迁。
// 使用：RecordHeadlessStatusObservation 在每次状态刷新后写入缓存并与旧值比较。
// 边界：不保存完整环境状态，不作为前端展示 DTO，也不跨应用重启持久化。
type headlessStatusSnapshot struct {
	// processRunning 表示上次观测时 DSPGAME.exe 无头进程是否运行。
	// 职责：用于判断进程上线或下线事件。
	// 使用：RecordHeadlessStatusObservation 与新快照比较。
	// 边界：只代表观测时刻，不保证之后仍保持该状态。
	processRunning bool
	// processID 表示上次观测到的无头服务器进程 PID。
	// 职责：在状态变更日志中提供定位信息。
	// 使用：recordHeadlessProcessTransition 和端口变更日志间接展示。
	// 边界：进程不存在时为 0，不作为后续控制进程的唯一凭据。
	processID int
	// portListening 表示目标端口是否有任一协议监听。
	// 职责：用于判断端口开放或关闭事件。
	// 使用：RecordHeadlessStatusObservation 与新快照比较。
	// 边界：只表达 TCP/UDP 任一监听状态，不区分协议细节。
	portListening bool
	// tcpListening 表示目标端口 TCP 是否监听。
	// 职责：保留端口状态的协议细节。
	// 使用：recordHeadlessPortTransition 写入日志元数据。
	// 边界：不单独触发日志，触发条件仍是 portListening 总状态变化。
	tcpListening bool
	// udpListening 表示目标端口 UDP 是否监听。
	// 职责：保留端口状态的协议细节。
	// 使用：recordHeadlessPortTransition 写入日志元数据。
	// 边界：不单独触发日志，触发条件仍是 portListening 总状态变化。
	udpListening bool
	// hostPort 表示观测时的无头服务器目标端口。
	// 职责：让状态变更日志说明端口对象。
	// 使用：日志内容和元数据展示。
	// 边界：端口配置变更本身不在此结构中单独记录。
	hostPort int
}

// snapshotFromStatus 从环境状态中提取观测快照。
// 职责：保留进程和端口变迁判断需要的最小字段。
// 使用：RecordHeadlessStatusObservation 每次状态刷新时调用。
// 边界：不保存完整状态，不做日志写入，也不读取旧快照。
func snapshotFromStatus(status headlessContract.HeadlessEnvironmentStatus) headlessStatusSnapshot {
	return headlessStatusSnapshot{
		processRunning: status.ProcessRunning,
		processID:      status.ProcessID,
		portListening:  status.PortListening,
		tcpListening:   status.TCPListening,
		udpListening:   status.UDPListening,
		hostPort:       status.HostPort,
	}
}
