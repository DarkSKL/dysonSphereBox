package environment

// 文件职责：集中定义 r2modman 托管配置与 Profile 路径约定。
// 使用：环境服务保存选择，Profile 辅助层检索和校验目录。
// 边界：常量不代表目录存在，也不授权写入检索结果。

const (
	// R2ConfigGroup 职责：标记托管配置分组；使用：ConfigUnit 读写；边界：不更改游戏目录配置。
	R2ConfigGroup = "Environment"
	// R2ProfilePathKey 职责：记录用户选中的 Profile；使用：持久化直接托管目标；边界：每次操作重新校验路径。
	R2ProfilePathKey = "R2modmanProfilePath"
	// R2GameDirectory 职责：限定戴森球计划的数据目录名；使用：Profile 发现与校验；边界：不扫描其他游戏的 Profile。
	R2GameDirectory = "DysonSphereProgram"
)
