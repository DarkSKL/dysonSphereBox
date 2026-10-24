package multiplayer

// 文件职责：把联机会话、端口、链路和中继的状态变化写入盒子统一日志，供掉线后复盘。
// 使用：控制器、成员链路与策略应用在状态发生变化时调用；边界：只记录状态事实，不记录 token、私钥与对端地址明细。

import (
	"time"

	loggingConstant "DysonSphereBox/constant/logging"
	systemHelper "DysonSphereBox/helper/system"
)

// recordState 写入一条联机状态事实。
// 职责：统一 namespace 前缀、来源与触发者；使用：状态变化点；边界：落盘失败不改变业务流程。
func recordState(level, namespace, action, reason string, data map[string]interface{}) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "multiplayer." + namespace,
		Source:    "backend.multiplayer",
		Reason:    reason,
		Action:    action,
		Level:     level,
		Handler:   loggingConstant.System,
		Data:      data,
	})
}

// recordStateAsync 异步写入一条状态事实。
// 职责：避免同步写日志阻塞链路状态机或 socket 事件循环；使用：高频或事件回调路径；边界：只保证尽量写入，不保证顺序。
func recordStateAsync(level, namespace, action, reason string, data map[string]interface{}) {
	go recordState(level, namespace, action, reason, data)
}

// durationMs 返回非负毫秒数；职责：日志时长字段；使用：会话关闭与拨号；边界：负值归零。
func durationMs(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	value := time.Since(start).Milliseconds()
	if value < 0 {
		return 0
	}
	return value
}
