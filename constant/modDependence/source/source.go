package source

// 文件职责：维护 Mod 依赖领域统一市场来源枚举。
// 使用：由 service/helper/apiDefinition 在比较来源、构造记录或生成 DTO 时引用。
// 边界：只描述来源稳定值，不保存可变运行态，不访问外部资源，不承载流程逻辑。

// ModSource 表示统一 Mod 市场数据来源。
// 职责：描述统一 Mod 市场和安装器中的包来源。
// 使用：前端、Provider 和安装器通过该值区分国内源、Thunderstore 与手动导入来源。
// 边界：只表达来源类型，不代表来源当前可用或包已被信任。
type ModSource string

// 来源常量职责：定义当前统一市场支持的来源枚举。
// 使用：市场查询、DTO 转换、安装记录和前端筛选共同使用这些稳定值。
// 边界：新增来源必须补齐 provider、DTO 转换和安装器兼容逻辑。
const (
	// ModSourceOfficial 表示当前自有接口来源。
	// 职责：标记来自国内官方/自有接口的 Mod 包。
	// 使用：官方源 provider、旧整合包兼容和安装状态映射时使用。
	// 边界：不表示 Thunderstore 包，也不表示本地导入。
	ModSourceOfficial ModSource = "official"
	// ModSourceThunderstore 表示 Thunderstore 来源。
	// 职责：标记来自 Thunderstore 索引的 Mod 包。
	// 使用：Thunderstore provider、依赖解析和 V2 安装器使用。
	// 边界：不直接保证依赖已适配 DSP，过滤逻辑由 provider 执行。
	ModSourceThunderstore ModSource = "thunderstore"
	// ModSourceManual 表示用户手动导入来源。
	// 职责：标记由用户本地选择文件或目录导入的包。
	// 使用：手动导入流程和安装记录区分本地来源时使用。
	// 边界：当前市场查询不会返回远端列表，具体导入能力由后续流程提供。
	ModSourceManual ModSource = "manual"
)
