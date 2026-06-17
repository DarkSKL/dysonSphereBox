package thunderstore

// 文件职责：定义 Thunderstore DSP 包索引接口的原始响应 DTO。
// 使用：由 helper/modDependence/marketplace 与 BepInEx 包解析逻辑解码 Thunderstore 包索引。
// 边界：只描述外部 API 响应结构，不发起 HTTP 请求、不筛选 DSP 适配包、不执行安装逻辑。

const (
	// ChunkedIndexURL 是 Thunderstore 戴森球计划新版分片包索引入口。
	// 职责：集中保存 package-listing-index 主链路 URL，供索引适配器读取 chunk 列表。
	// 使用：helper/modDependence/marketplace/thunderstoreindex 的新版 provider 发起索引请求时使用。
	// 边界：只描述远端接口地址，不解压 gzip、不拉取 chunk、不决定缓存或失败兜底策略。
	ChunkedIndexURL = "https://thunderstore.io/c/dyson-sphere-program/api/v1/package-listing-index/"
	// LegacyIndexURL 是 Thunderstore 戴森球计划旧版完整包索引接口地址。
	// 职责：保留旧 api/v1/package 链路作为新版索引失败后的兼容兜底。
	// 使用：helper/modDependence/marketplace/thunderstoreindex 的旧版 provider 在新版 provider 失败后使用。
	// 边界：该接口当前可能返回 404，不能作为业务层直接依赖的主链路。
	LegacyIndexURL = "https://thunderstore.io/c/dyson-sphere-program/api/v1/package/"
	// IndexURL 是 Thunderstore 戴森球计划旧索引接口兼容别名。
	// 职责：兼容尚未迁移的内部调用点，语义等同 LegacyIndexURL。
	// 使用：新增代码应优先通过 thunderstoreindex 适配器选择 provider，不应直接使用该别名请求远端。
	// 边界：保留该常量不代表旧接口可用，失败兜底必须由适配器承担。
	IndexURL = LegacyIndexURL
)

// Package 表示 Thunderstore 包索引中的单个包。
// 职责：承载 Thunderstore 包级元数据和版本列表。
// 使用：市场 Provider 转换统一 DTO、BepInEx 解析最新包版本时读取。
// 边界：该结构是远端原始响应形状，不代表包已经通过 DSP 适配过滤。
type Package struct {
	// Owner 是 Thunderstore 包命名空间。
	// 职责：标识包所属团队或作者。
	// 使用：生成统一包 key、匹配 BepInEx 作者、解析依赖命名空间时读取。
	// 边界：不保证该命名空间仍然活跃或可信。
	Owner string `json:"owner"`
	// Name 是 Thunderstore 包名称。
	// 职责：标识包在命名空间内的名称。
	// 使用：生成统一包 key、匹配依赖和展示标题时读取。
	// 边界：不包含版本信息，版本由 VersionNumber 字段提供。
	Name string `json:"name"`
	// FullName 是 Thunderstore 返回的完整包名。
	// 职责：保存来源侧完整标识。
	// 使用：调试、展示或兼容 Thunderstore 原始命名时读取。
	// 边界：统一内部身份仍以 source/namespace/name/version 组合为准。
	FullName string `json:"full_name"`
	// PackageURL 是 Thunderstore 包页面地址。
	// 职责：提供包详情跳转链接。
	// 使用：统一 DTO 的 PackageURL 字段映射。
	// 边界：不参与下载，下载地址来自版本字段。
	PackageURL string `json:"package_url"`
	// DateUpdated 是 Thunderstore 包更新时间文本。
	// 职责：保留来源更新时间。
	// 使用：统一 DTO 更新时间或排序扩展时读取。
	// 边界：不在 DTO 层解析时间格式。
	DateUpdated string `json:"date_updated"`
	// IsDeprecated 表示包是否被 Thunderstore 标记为废弃。
	// 职责：支持市场过滤废弃包。
	// 使用：Provider 根据 IncludeDeprecated 决定是否展示。
	// 边界：只反映来源标记，不代表本地安装状态。
	IsDeprecated bool `json:"is_deprecated"`
	// Categories 是 Thunderstore 分类标签。
	// 职责：承载来源分类信息。
	// 使用：统一 DTO Tags 字段映射和搜索扩展。
	// 边界：不保证分类一定与 DSP Mod 语义一致。
	Categories []string `json:"categories"`
	// RatingScore 是 Thunderstore 评分。
	// 职责：保留来源侧可信度或热度信号。
	// 使用：统一 DTO Trustworthiness 字段映射。
	// 边界：不作为安全校验依据。
	RatingScore int64 `json:"rating_score"`
	// Versions 是该包的版本列表。
	// 职责：承载可安装版本和依赖声明。
	// 使用：市场选择最新 active 版本、BepInEx 选择最新包、依赖递归解析时读取。
	// 边界：版本是否 active 和是否适配 DSP 由 helper 逻辑判断。
	Versions []Version `json:"versions"`
}

// Version 表示 Thunderstore 包索引中的单个版本。
// 职责：承载版本号、下载地址、依赖声明和版本级元数据。
// 使用：市场 DTO 转换、依赖递归解析和 BepInEx 下载时读取。
// 边界：只描述来源响应，不负责下载 zip 或校验压缩包内容。
type Version struct {
	// IsActive 表示版本是否仍处于可用状态。
	// 职责：支持过滤废弃或隐藏版本。
	// 使用：市场 Provider 与 BepInEx 版本选择只使用 active 版本。
	// 边界：active 不代表文件下载一定成功。
	IsActive bool `json:"is_active"`
	// DateCreated 是版本创建时间文本。
	// 职责：支持选择最新 active 版本。
	// 使用：BepInEx 解析最新版本和市场排序扩展时读取。
	// 边界：DTO 层不解析格式，解析由调用方负责。
	DateCreated string `json:"date_created"`
	// FullName 是 Thunderstore 版本完整名。
	// 职责：保留来源侧版本完整标识。
	// 使用：BepInExPackage.FullName 和调试日志映射。
	// 边界：内部统一 key 仍由 helper 规则生成。
	FullName string `json:"full_name"`
	// Description 是版本说明。
	// 职责：为统一 DTO 提供展示描述。
	// 使用：市场卡片和详情展示读取。
	// 边界：不做 Markdown 清洗或安全过滤。
	Description string `json:"description"`
	// Icon 是版本图标地址。
	// 职责：为统一 DTO 提供封面图来源。
	// 使用：市场卡片展示读取。
	// 边界：图标可用性由前端加载结果决定。
	Icon string `json:"icon"`
	// VersionNumber 是 Thunderstore 版本号。
	// 职责：标识可安装的具体版本。
	// 使用：统一 DTO Version 字段、安装记录和 BepInEx 下载文件名读取。
	// 边界：不解析语义化版本，仅保留来源文本。
	VersionNumber string `json:"version_number"`
	// DownloadURL 是版本 zip 下载地址。
	// 职责：提供安装器或 BepInEx 下载包的远端 URL。
	// 使用：统一 DTO DownloadURL 和 BepInExPackage.DownloadURL 映射。
	// 边界：不保证地址可达，下载错误由 helper 处理。
	DownloadURL string `json:"download_url"`
	// Downloads 是版本下载次数。
	// 职责：保留来源热度数据。
	// 使用：统一 DTO DownloadCount 字段映射。
	// 边界：不参与本地下载计数。
	Downloads int64 `json:"downloads"`
	// FileSize 是版本压缩包大小。
	// 职责：提供安装前展示和进度估算的来源数据。
	// 使用：统一 DTO PackageSize 字段映射。
	// 边界：不作为下载完整性校验依据。
	FileSize int64 `json:"file_size"`
	// WebsiteURL 是版本相关网站地址。
	// 职责：保留来源侧附加链接。
	// 使用：后续详情页或调试展示可读取。
	// 边界：当前不参与安装流程。
	WebsiteURL string `json:"website_url"`
	// Dependencies 是 Thunderstore 原始依赖声明列表。
	// 职责：提供递归解析依赖包的原始字符串。
	// 使用：市场转换和安装器解析依赖链时读取。
	// 边界：字符串格式错误时由 helper 解析逻辑兜底处理。
	Dependencies []string `json:"dependencies"`
}
