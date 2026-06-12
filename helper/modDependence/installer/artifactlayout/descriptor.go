package artifactlayout

// 文件职责：定义 artifact 布局算法需要的最小描述信息。
// 使用：安装器从 v2/artifact.Artifact 转换，迁移流程从 installed 记录和组件记录转换。
// 边界：只承载命名候选字段，不包含下载地址、安装状态、数据库标签或文件系统行为。

// ArtifactDescriptor 描述布局算法需要的 artifact 命名信息。
// 职责：以最小字段承载目录命名候选值。
// 使用：安装器从 v2/artifact.Artifact 转换，迁移流程从 installed 记录和组件记录转换。
// 边界：不包含下载地址、来源、版本规则以外的业务状态，也不直接持久化。
type ArtifactDescriptor struct {
	// ComponentKey 职责：记录组件唯一 key；使用：目录命名兜底和前缀识别；边界：可为空但会降低命名质量。
	ComponentKey string
	// Name 职责：记录组件原始名称；使用：DisplayName 为空时作为目录名候选；边界：会经过路径片段清洗。
	Name string
	// DisplayName 职责：记录用户可读组件名称；使用：优先作为插件目录名候选；边界：不保证唯一。
	DisplayName string
	// Version 职责：记录组件版本；使用：保留给调用方扩展诊断；边界：当前布局算法不直接拼接版本。
	Version string
}
