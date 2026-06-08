package modDependence

import (
	"sync"

	systemHelper "DysonSphereBox/helper/system"
	ModDependenceResponse "DysonSphereBox/model/modDependence/response"
	systemModel "DysonSphereBox/model/system"
)

type headlessStatusSnapshot struct {
	processRunning bool
	processID      int
	portListening  bool
	tcpListening   bool
	udpListening   bool
	hostPort       int
}

var (
	headlessStatusSnapshotMutex sync.Mutex
	headlessStatusSnapshots     = make(map[string]headlessStatusSnapshot)
)

// RecordPackageInstallSuccess 记录 Mod 整合包安装成功事件。
func RecordPackageInstallSuccess(result *PackageInstallResult) {
	if result == nil {
		return
	}

	resultText := "已安装"
	if result.Skipped {
		resultText = "已跳过"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "installer",
		Event:     "installPackage",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "Mod 整合包状态：未安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"packageId":       result.PackageID,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
			"pluginRoot":      result.PluginRoot,
			"message":         result.Message,
		},
	})
}

// RecordPackageInstallFailure 记录 Mod 整合包安装失败事件。
func RecordPackageInstallFailure(pack ModDependenceResponse.ModDependencyPackageDTO, err error) {
	packageName := ""
	if pack.PackageName != nil {
		packageName = *pack.PackageName
	}

	systemHelper.RecordFailure("modDependence", "installer", "installPackage", "Mod 整合包安装失败", err, map[string]interface{}{
		"packageId":   pack.ID,
		"packageName": packageName,
	})
}

// RecordPackageUninstallSuccess 记录 Mod 整合包卸载成功事件。
func RecordPackageUninstallSuccess(result *PackageInstallResult) {
	if result == nil {
		return
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "installer",
		Event:     "uninstallPackage",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "Mod 整合包状态：已安装 -> 已卸载",
		Metadata: map[string]interface{}{
			"packageId":       result.PackageID,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
		},
	})
}

// RecordPackageUninstallFailure 记录 Mod 整合包卸载失败事件。
func RecordPackageUninstallFailure(packageID uint, err error) {
	systemHelper.RecordFailure("modDependence", "installer", "uninstallPackage", "Mod 整合包卸载失败", err, map[string]interface{}{
		"packageId": packageID,
	})
}

// RecordBepInExFailure 记录 BepInEx 操作失败事件。
func RecordBepInExFailure(event string, content string, err error) {
	systemHelper.RecordFailure("modDependence", "bepInEx", event, content, err, nil)
}

// RecordBepInExUninstallSuccess 记录 BepInEx 卸载成功事件。
func RecordBepInExUninstallSuccess(result *BepInExUninstallResult) {
	if result == nil {
		return
	}

	resultText := "已移除"
	if result.Skipped {
		resultText = "已跳过"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "bepInEx",
		Event:     "uninstall",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "BepInEx 状态：已安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"gameDir":        result.GameDir,
			"removed":        result.Removed,
			"skipped":        result.Skipped,
			"removedCount":   len(result.RemovedPaths),
			"clearedRecords": result.ClearedRecords,
			"message":        result.Message,
		},
	})
}

// RecordBepInExInstallResult 记录 BepInEx 安装或更新结果。
func RecordBepInExInstallResult(event string, result *BepInExInstallResult) {
	if result == nil {
		return
	}

	resultText := "已安装"
	if result.Skipped {
		resultText = "已跳过"
	}

	packageVersion := ""
	if result.Package != nil {
		packageVersion = result.Package.Version
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "bepInEx",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "BepInEx 状态：未安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"gameDir":   result.GameDir,
			"bufferDir": result.BufferDir,
			"version":   packageVersion,
			"message":   result.Message,
		},
	})
}

// RecordHeadlessPrepareResult 记录无头服务器环境准备结果。
func RecordHeadlessPrepareResult(result *HeadlessPrepareResult) {
	if result == nil {
		return
	}

	level := systemModel.LogLevelInfo
	if !result.Status.Ready {
		level = systemModel.LogLevelWarn
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "prepare",
		Level:     level,
		Type:      systemModel.LogTypeConfiguration,
		Content:   result.Message,
		Metadata: map[string]interface{}{
			"gameDir":      result.GameDir,
			"changed":      result.Changed,
			"writtenPath":  result.WrittenPath,
			"ready":        result.Status.Ready,
			"issues":       result.Status.Issues,
			"hostPort":     result.Status.HostPort,
			"steamAppIdOk": result.Status.SteamAppIDValid,
			"nebulaReady":  result.Status.NebulaReady,
		},
	})
}

// RecordHeadlessStartResult 记录无头服务器启动动作结果。
func RecordHeadlessStartResult(result *HeadlessStartResult) {
	if result == nil {
		return
	}

	level := systemModel.LogLevelInfo
	if !result.Started && !result.Skipped {
		level = systemModel.LogLevelWarn
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "start",
		Level:     level,
		Type:      systemModel.LogTypeConfiguration,
		Content:   result.Message,
		Metadata: map[string]interface{}{
			"gameDir":          result.GameDir,
			"started":          result.Started,
			"skipped":          result.Skipped,
			"processId":        result.ProcessID,
			"ready":            result.Status.Ready,
			"issues":           result.Status.Issues,
			"startCommandText": result.StartCommandText,
		},
	})
}

// RecordHeadlessStopResult 记录无头服务器停止动作结果。
func RecordHeadlessStopResult(result *HeadlessStopResult) {
	if result == nil {
		return
	}

	level := systemModel.LogLevelInfo
	if !result.Stopped && !result.Skipped {
		level = systemModel.LogLevelWarn
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "stop",
		Level:     level,
		Type:      systemModel.LogTypeConfiguration,
		Content:   result.Message,
		Metadata: map[string]interface{}{
			"gameDir":   result.GameDir,
			"stopped":   result.Stopped,
			"skipped":   result.Skipped,
			"processId": result.ProcessID,
			"ready":     result.Status.Ready,
			"issues":    result.Status.Issues,
		},
	})
}

// RecordHeadlessStatusObservation 记录无头服务器进程与端口的状态变迁。
// 使用：状态刷新、启动和停止后调用；边界：首次观测只建立基线，不写入上线/下线日志，避免应用启动时刷屏。
func RecordHeadlessStatusObservation(status HeadlessEnvironmentStatus, source string) {
	if status.GameDir == "" {
		return
	}

	nextSnapshot := headlessStatusSnapshot{
		processRunning: status.ProcessRunning,
		processID:      status.ProcessID,
		portListening:  status.PortListening,
		tcpListening:   status.TCPListening,
		udpListening:   status.UDPListening,
		hostPort:       status.HostPort,
	}

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

func recordHeadlessProcessTransition(status HeadlessEnvironmentStatus, online bool, source string) {
	event := "processOffline"
	content := "无头服务器进程已下线"
	if online {
		event = "processOnline"
		content = "无头服务器进程已上线"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata: map[string]interface{}{
			"gameDir":   status.GameDir,
			"processId": status.ProcessID,
			"hostPort":  status.HostPort,
			"source":    source,
		},
	})
}

func recordHeadlessPortTransition(status HeadlessEnvironmentStatus, opened bool, source string) {
	event := "portClosed"
	content := "无头服务器端口已关闭"
	if opened {
		event = "portOpen"
		content = "无头服务器端口已开放"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata: map[string]interface{}{
			"gameDir":      status.GameDir,
			"processId":    status.ProcessID,
			"hostPort":     status.HostPort,
			"tcpListening": status.TCPListening,
			"udpListening": status.UDPListening,
			"source":       source,
		},
	})
}

// RecordHeadlessFailure 记录无头服务器环境检测或准备失败事件。
func RecordHeadlessFailure(event string, content string, err error) {
	systemHelper.RecordFailure("modDependence", "headless", event, content, err, nil)
}
