//go:build windows

package main

// 文件职责：读取Windows为当前进程保留的内存高水位。
// 使用：独立测量进程在Generate返回后取数。
// 边界：是进程启动至读数时的峰值，不是请求独占内存或Go堆峰值。

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processMemoryCounters 职责：对应Win32 PROCESS_MEMORY_COUNTERS布局；使用：GetProcessMemoryInfo；边界：SIZE_T采用uintptr以匹配目标位数。
type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// readProcessMemory 职责：读取当前进程的驻留集与提交量峰值；使用：单次资源报告；边界：调用失败保留原因，不填零冒充测量。
func readProcessMemory() processMemory {
	memory := processMemory{Scope: "process-lifetime-through-measurement", Source: "Windows.GetProcessMemoryInfo"}
	procedure := windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
	if err := procedure.Find(); err != nil {
		memory.Error = err.Error()
		return memory
	}
	var counters processMemoryCounters
	counters.CB = uint32(unsafe.Sizeof(counters))
	success, _, err := procedure.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB))
	if success == 0 {
		memory.Error = fmt.Sprintf("GetProcessMemoryInfo失败: %v", err)
		return memory
	}
	workingSet, commit := uint64(counters.PeakWorkingSetSize), uint64(counters.PeakPagefileUsage)
	memory.PeakWorkingSetBytes, memory.PeakCommitBytes = &workingSet, &commit
	return memory
}
