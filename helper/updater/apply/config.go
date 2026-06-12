package apply

// 文件职责：定义应用更新资产应用流程需要的配置契约。
// 使用：updater 根包调用 ApplyRelease 时传入命令名和 checksum 文件名。
// 边界：只保存执行参数，不读取项目全局配置、不访问远端 Release、不修改本地文件。

// Config 描述 Release 资产应用流程的稳定配置。
// 职责：承载压缩包内命令名和校验清单文件名。
// 使用：ApplyRelease 下载、校验和解压更新资产时读取。
// 边界：字段为空会导致校验或解压失败，调用方应传入项目配置常量。
type Config struct {
	// CommandName 是压缩包内应用可执行文件的基础名称。
	// 职责：让解压逻辑在 Release 资产中找到实际命令文件。
	// 使用：archiveExecutableName 根据平台补齐可执行文件名时读取。
	// 边界：不代表当前进程路径，当前路径由 go-selfupdate 解析。
	CommandName string
	// ChecksumName 是 Release 附带的校验清单文件名。
	// 职责：指定 go-selfupdate 校验链中唯一的 checksum 文件。
	// 使用：validateReleaseAsset 创建校验器并匹配校验链时读取。
	// 边界：缺失时视为更新资产不可信并返回错误，不自动跳过校验。
	ChecksumName string
}
