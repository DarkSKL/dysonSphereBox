//go:build !windows

package main

// 文件职责：保留非Windows测量的明确边界。
// 使用：允许资源命令跨平台编译及检查取消延迟。
// 边界：尚未接入其它平台的进程峰值接口，不能以零值充当证据。

// readProcessMemory 职责：报告当前平台缺少内存高水位来源；使用：非Windows资源报告；边界：不伪造峰值。
func readProcessMemory() processMemory {
	return processMemory{Scope: "process-lifetime-through-measurement", Error: "此平台尚未接入进程内存高水位接口"}
}
