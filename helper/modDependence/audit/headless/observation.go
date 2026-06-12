package headless

// 文件职责：记录无头服务器进程与端口状态的观测变迁日志。
// 使用：由 HeadlessService 在状态刷新、启动、停止或设置保存后传入最新环境状态。
// 边界：首次观测只建立内存基线，不写入上线/下线日志；不启动进程、不检查端口、不持久化快照。

import headlessContract "DysonSphereBox/helper/modDependence/headless/contract"

// RecordHeadlessStatusObservation 记录无头服务器进程与端口的状态变迁。
// 职责：比较同一游戏目录的新旧无头服务器状态并记录进程/端口变化日志。
// 使用：状态刷新、启动和停止后调用。
// 边界：首次观测只建立基线，不写入上线/下线日志，避免应用启动时刷屏；游戏目录为空时忽略。
// 逻辑流：1. 从状态 DTO 提取快照 => 2. 加锁读取旧快照并写入新快照 => 3. 首次观测直接返回 => 4. 进程状态变化时记录进程日志 => 5. 端口状态变化时记录端口日志。
func RecordHeadlessStatusObservation(status headlessContract.HeadlessEnvironmentStatus, source string) {
	if status.GameDir == "" {
		return
	}

	nextSnapshot := snapshotFromStatus(status)

	headlessStatusSnapshotMutex.Lock()
	previousSnapshot, hasPrevious := headlessStatusSnapshots[status.GameDir]
	headlessStatusSnapshots[status.GameDir] = nextSnapshot
	headlessStatusSnapshotMutex.Unlock()

	if !hasPrevious {
		return
	}

	if previousSnapshot.processRunning != nextSnapshot.processRunning {
		recordHeadlessProcessTransition(status, nextSnapshot.processRunning, source)
	}
	if previousSnapshot.portListening != nextSnapshot.portListening {
		recordHeadlessPortTransition(status, nextSnapshot.portListening, source)
	}
}
