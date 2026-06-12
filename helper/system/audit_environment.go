package system

// 文件职责：构造游戏目录加载、自动定位和用户选择相关的审计日志。
// 使用：由 EnvironmentService 和 App.SelectDysonSphereDirectory 在路径流程结束后调用。
// 边界：只记录路径流程结果，不查找路径、不保存配置、不校验 DSPGAME.exe 是否存在。

import systemModel "DysonSphereBox/model/system"

// RecordGamePathSuccess 记录游戏目录加载或初始化成功事件。
// 职责：写入游戏目录加载、自动定位或用户选择成功的审计日志。
// 使用：EnvironmentService 和 App.SelectDysonSphereDirectory 在获得有效游戏目录后调用。
// 边界：不校验路径结构，调用方应先完成路径归一化或校验。
func RecordGamePathSuccess(event string, content string, dirPath string, source string) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "environment",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeConfiguration,
		Content:   content,
		Metadata: map[string]interface{}{
			"dirPath": dirPath,
			"source":  source,
		},
	})
}

// RecordGamePathFailure 记录游戏目录加载、初始化或选择失败事件。
// 职责：写入游戏目录加载、自动定位或用户选择失败的审计日志。
// 使用：EnvironmentService 和 App.SelectDysonSphereDirectory 在路径读取、选择或保存失败后调用。
// 边界：只记录错误和候选路径，不尝试修复配置。
func RecordGamePathFailure(event string, content string, err error, dirPath string) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "environment",
		Event:     event,
		Level:     systemModel.LogLevelError,
		Type:      systemModel.LogTypeConfiguration,
		Content:   content,
		Metadata: map[string]interface{}{
			"dirPath": dirPath,
		},
		Error: errorText(err),
	})
}
