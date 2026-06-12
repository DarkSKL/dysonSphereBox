package updater

// 文件职责：集中保存 DysonSphereBox 应用自更新的远端仓库与资产配置。
// 使用：更新检查、Release 应用、校验器创建和平台资产解析模块读取这些常量。
// 边界：仅承载当前应用自更新元数据，不用于 Mod 下载、插件安装或其它远端资源。

// 应用更新常量组集中描述 DysonSphereBox 自更新所需的远端仓库与校验资源。
// 职责：保存 Gitee 仓库、API、校验清单和压缩包内命令名等稳定配置。
// 使用：由检查更新、下载资产、校验文件和解压可执行文件的 helper 共同读取。
// 边界：仅承载当前应用的更新元数据，不用于 Mod 下载、插件安装或其它远端资源。
const (
	// appUpdateOwner 表示 Gitee 仓库所属用户。
	// 职责：定位自更新 Release 所在的仓库命名空间。
	// 使用：repository 构造 go-selfupdate 仓库标识时读取。
	// 边界：不参与 API 鉴权，不描述本地应用作者信息。
	appUpdateOwner = "Dark-Skl"
	// appUpdateRepo 表示 Gitee 仓库名称。
	// 职责：定位自更新 Release 所在的具体仓库。
	// 使用：repository 构造远端更新仓库 slug 时读取。
	// 边界：只适用于应用自更新，不用于 Mod 市场仓库。
	appUpdateRepo = "dyson-sphere-box"
	// appUpdateAPIBaseURL 表示 Gitee API 基础地址。
	// 职责：为 gitee.Source 提供默认 Release API 根路径。
	// 使用：client.New 初始化更新源时传入。
	// 边界：只保存基础地址，不拼接仓库路径和请求参数。
	appUpdateAPIBaseURL = "https://gitee.com/api/v5"
	// appUpdateChecksum 表示 Release 附带的校验清单文件名。
	// 职责：指定 go-selfupdate 校验链中唯一的 checksum 文件。
	// 使用：client.New 与 apply.ApplyRelease 创建校验器时读取。
	// 边界：缺失时视为更新资产不可信并返回错误，不自动跳过校验。
	appUpdateChecksum = "checksums.txt"
	// appUpdateCommand 表示压缩包内应用可执行文件的基础名称。
	// 职责：让解压逻辑在 Release 资产中找到实际命令文件。
	// 使用：archiveExecutableName 根据平台补齐可执行文件名时读取。
	// 边界：不代表当前进程路径，当前路径由 go-selfupdate 解析。
	appUpdateCommand = "DysonSphereBox"
)
