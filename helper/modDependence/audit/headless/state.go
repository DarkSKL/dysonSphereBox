package headless

// 文件职责：保存无头服务器审计观测所需的进程和端口内存快照状态。
// 使用：RecordHeadlessStatusObservation 在比较同一游戏目录的新旧状态时读取和更新。
// 边界：只作为审计降噪基线，不是运行状态权威来源；应用重启后自动丢失。

import "sync"

// 无头服务器状态快照缓存职责：记录最近一次观测到的无头服务器状态。
// 使用：RecordHeadlessStatusObservation 在状态刷新、启动和停止后比较新旧状态。
// 边界：只用于进程/端口变更审计，不持久化，不作为服务运行状态的权威来源。
var (
	// headlessStatusSnapshotMutex 保护无头服务器状态快照缓存的并发访问。
	// 职责：避免前端轮询和服务操作同时刷新状态时产生 map 并发读写。
	// 使用：RecordHeadlessStatusObservation 更新 headlessStatusSnapshots 前后加锁。
	// 边界：只保护本文件内的状态快照，不保护系统日志写入或进程探测。
	headlessStatusSnapshotMutex sync.Mutex
	// headlessStatusSnapshots 按游戏目录缓存最近一次无头服务器状态快照。
	// 职责：为状态变迁日志提供前后对比基线。
	// 使用：RecordHeadlessStatusObservation 读取旧快照并写入新快照。
	// 边界：首次观测只建立基线，不写入上线/下线日志，避免应用启动时刷屏。
	headlessStatusSnapshots = make(map[string]headlessStatusSnapshot)
)
