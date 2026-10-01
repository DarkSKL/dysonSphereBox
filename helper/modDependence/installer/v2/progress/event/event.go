package event

// 文件职责：发送 V2 安装进度事件并提供安装、卸载和失败状态的便捷入口。
// 使用：V2 安装器各阶段通过 Package、PackageWithState、Uninstall、MarkFailure 更新前端和本地状态。
// 边界：不执行下载、解压、复制或冲突处理；事件发送失败不阻断安装流程。

import (
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/persistence"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/snapshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Emit 发送并持久化安装进度。
// 职责：规范化进度、缓存进程内快照、更新数据库安装状态并向 Wails 前端推送事件。
// 使用：下载、复制、记录和失败等会影响安装记录的阶段调用。
// 边界：packageKey 为空时直接跳过；RuntimeContext 为空时只更新本地状态，不发送事件。
// 逻辑流：1. 校验包 key => 2. 限制百分比并补默认 running 状态 => 3. 写入运行时快照和数据库 => 4. Runtime 可用时发送 Wails 事件。
func Emit(data payload.Payload) {
	if data.PackageKey == "" {
		return
	}
	data.Percent = progresscalc.Clamp(data.Percent)
	if data.State == "" {
		data.State = string(installConstant.StateRunning)
	}
	snapshot.Store(data)
	persistence.UpdatePackageInstallProgress(data)
	if core.GLOBAL.RuntimeContext == nil {
		return
	}
	runtime.EventsEmit(core.GLOBAL.RuntimeContext, installConstant.ProgressEventName, data)
}

// RuntimeOnly 发送仅运行时有效的安装进度。
// 职责：缓存进程内快照并向 Wails 前端推送事件，但不写入数据库安装记录。
// 使用：卸载进度、取消清理提示等不应该持久化为安装状态的场景调用。
// 边界：packageKey 为空时跳过；RuntimeContext 为空时只保留运行时快照。
func RuntimeOnly(data payload.Payload) {
	if data.PackageKey == "" {
		return
	}
	data.Percent = progresscalc.Clamp(data.Percent)
	if data.State == "" {
		data.State = string(installConstant.StateRunning)
	}
	snapshot.Store(data)
	if core.GLOBAL.RuntimeContext == nil {
		return
	}
	runtime.EventsEmit(core.GLOBAL.RuntimeContext, installConstant.ProgressEventName, data)
}

// Package 发送普通安装进度。
// 职责：把阶段、状态和百分比转换为标准安装进度事件。
// 使用：安装器各阶段需要更新前端和数据库状态时调用。
// 边界：stage 为 done 时会转换为 installed 状态，其它阶段默认 running。
func Package(packageKey string, stage string, status string, percent int) {
	state := string(installConstant.StateRunning)
	if stage == "done" {
		state = string(installConstant.StateInstalled)
	}
	Emit(payload.Payload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    progresscalc.Clamp(percent),
		Message:    status,
		State:      state,
	})
}

// PackageWithState 发送指定状态的安装进度。
// 职责：允许调用方显式设置 running、failed、conflict 等状态。
// 使用：冲突、失败或恢复流程需要覆盖默认状态时调用。
// 边界：不校验 state 合法性，调用方必须传入 installer 已知状态常量。
func PackageWithState(packageKey string, stage string, status string, percent int, state string) {
	Emit(payload.Payload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    progresscalc.Clamp(percent),
		Message:    status,
		State:      state,
	})
}

// Uninstall 发送卸载运行时进度。
// 职责：向前端通知卸载阶段变化，并在 done 阶段标记为 uninstalled。
// 使用：远端包卸载文件清理过程中调用。
// 边界：只写运行时快照，不写安装表，避免卸载事务和进度更新争用 SQLite 锁。
func Uninstall(packageKey string, stage string, status string, percent int) {
	state := string(installConstant.StateRunning)
	if stage == "done" {
		state = string(installConstant.StateUninstalled)
	}

	RuntimeOnly(payload.Payload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    progresscalc.Clamp(percent),
		Message:    status,
		State:      state,
	})
}

// MarkFailure 将指定包的最近进度标记为失败并通知前端。
// 职责：把服务层捕获到的错误转换为 failed 进度事件。
// 使用：Wails 服务捕获卸载或安装器错误后调用，避免页面刷新后丢失失败状态。
// 边界：仅更新运行时快照，不能因卸载失败或操作忙碌覆盖仍有效的安装记录；err 为空时错误信息为空。
func MarkFailure(packageKey string, status string, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	RuntimeOnly(payload.Payload{
		PackageKey:   packageKey,
		Stage:        "failed",
		Status:       status,
		Percent:      100,
		Message:      errorMessage,
		State:        string(installConstant.StateFailed),
		ErrorMessage: errorMessage,
	})
}
