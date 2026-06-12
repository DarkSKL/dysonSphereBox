package dependency

// 文件职责：汇总无头服务器依赖检测结果是否满足启动前置条件。
// 使用：headless.BuildUnconfiguredHeadlessEnvironmentStatus 和 headless.InspectHeadlessEnvironment 调用 Ready。
// 边界：只归约传入状态，不重新读取磁盘、不检查 BepInEx、进程或端口状态。

import "DysonSphereBox/helper/modDependence/headless/contract"

// Ready 判断所有开服依赖是否安装完整。
// 职责：把依赖列表归约为 NebulaReady 布尔值。
// 使用：headless.BuildUnconfiguredHeadlessEnvironmentStatus 和 headless.InspectHeadlessEnvironment 调用。
// 边界：空列表会返回 true；当前调用方始终传入内置规格生成的列表。
func Ready(dependencies []contract.HeadlessDependencyStatus) bool {
	for _, item := range dependencies {
		if !item.Installed {
			return false
		}
	}

	return true
}
