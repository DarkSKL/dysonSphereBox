package v2

// 文件职责：维护 V2 安装主流程专用的测试钩子。
// 使用：由安装集成测试在需要精确卡住阶段边界时设置，生产流程只在非 nil 时触发。
// 边界：不得承载安装计划、文件操作或持久化类型；生产运行默认 nil，不改变真实安装行为。

// installRemotePackageTestHooks 只在集成测试中使用，用来把取消精确卡在安装阶段边界。
// 职责：提供安装阶段后的测试钩子，便于验证取消和回滚边界。
// 使用：集成测试设置对应函数；生产代码只在非 nil 时调用。
// 边界：生产环境保持 nil，不改变真实安装流程。
var installRemotePackageTestHooks struct {
	// AfterPlannedFiles 职责：在计划文件写入会话后触发；使用：测试安装取消时机；边界：生产环境为 nil。
	AfterPlannedFiles func(packageKey string, componentKey string)
	// AfterArtifactFiles 职责：在组件文件处理完成后触发；使用：测试部分组件完成后的取消恢复；边界：生产环境为 nil。
	AfterArtifactFiles func(packageKey string, componentKey string)
	// AfterSaveArtifact 职责：在组件记录保存后触发；使用：测试数据库记录和文件状态一致性；边界：生产环境为 nil。
	AfterSaveArtifact func(packageKey string, componentKey string)
}
