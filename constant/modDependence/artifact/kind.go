package artifact

// 文件职责：维护 Mod 依赖领域安装产物类型枚举。
// 使用：由 helper/apiDefinition 在构造 DTO、解析安装计划和区分本地/远端产物时引用。
// 边界：只描述产物类型稳定值，不保存可变运行态，不访问外部资源，不承载流程逻辑。

// ArtifactKind 表示安装器获取 Mod 产物的方式。
// 职责：描述安装器应如何取得一个待安装组件。
// 使用：远端包 DTO、安装计划和本地记录通过该值区分下载或本地导入流程。
// 边界：远端 zip 需要先下载，本地 zip 与本地目录由手动导入流程提供。
type ArtifactKind string

// 产物来源常量职责：定义安装器支持的产物获取方式。
// 使用：安装器选择下载、解压或本地目录布局解析策略时比较这些值。
// 边界：只表达产物来源，不代表文件已通过安全校验或已经安装。
const (
	// ArtifactKindRemoteZip 表示通过 URL 下载 zip 包。
	// 职责：标记组件产物需要从远端 URL 下载。
	// 使用：安装器在执行前先下载并校验 zip。
	// 边界：不包含下载地址本身，下载失败由安装流程处理。
	ArtifactKindRemoteZip ArtifactKind = "remote_zip"
	// ArtifactKindLocalZip 表示用户选择的本地 zip 包。
	// 职责：标记组件产物来自用户本地 zip 文件。
	// 使用：手动导入流程传入本地压缩包时使用。
	// 边界：不保证 zip 内容结构有效，后续布局解析负责校验。
	ArtifactKindLocalZip ArtifactKind = "local_zip"
	// ArtifactKindLocalDir 表示用户选择的本地目录。
	// 职责：标记组件产物来自用户本地目录。
	// 使用：手动导入目录或开发调试安装时使用。
	// 边界：不保证目录内包含 BepInEx 目标结构，安装计划负责判断。
	ArtifactKindLocalDir ArtifactKind = "local_dir"
)
