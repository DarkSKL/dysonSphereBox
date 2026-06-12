package artifact

// 文件职责：定义 V2 安装器内部 artifact 类型。
// 使用：V2 安装解析、下载、解压、复制、会话清理和安装记录保存共享本包类型。
// 边界：不下载文件、不访问数据库、不发送进度事件；DTO 转换和布局辅助方法由同包独立文件维护。

import (
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
)

// Artifact 表示安装器内部待处理的单个组件 artifact。
// 职责：承载下载地址、本地路径、来源和安装归属信息。
// 使用：V2 安装流程解析远端包及依赖后逐个处理。
// 边界：内部计划类型，不直接返回给前端或持久化。
type Artifact struct {
	// PackageKey 职责：记录当前进度事件所属包 key；使用：进度上报和文件清单归属；边界：依赖组件可能使用父包 key。
	PackageKey string
	// ParentPackageKey 职责：保留用户实际点击安装的主包 key；使用：依赖安装时文件清单仍归属主包；边界：普通主包组件可为空或等于 PackageKey。
	ParentPackageKey string
	// ComponentKey 职责：记录当前组件唯一 key；使用：冲突决策、文件清单和组件记录；边界：必须在解析阶段保证稳定。
	ComponentKey string
	// Source 职责：记录组件来源；使用：安装记录和审计日志；边界：不直接决定下载方式，下载方式看 ArtifactKind。
	Source sourceConstant.ModSource
	// Name 职责：记录组件原始名称；使用：目录命名和安装记录；边界：显示优先使用 DisplayName。
	Name string
	// DisplayName 职责：记录组件展示名称；使用：进度文本和前端明细；边界：不作为唯一标识。
	DisplayName string
	// Version 职责：记录组件版本；使用：安装记录和展示；边界：不做语义版本比较。
	Version string
	// DownloadURL 职责：记录远端下载地址；使用：远端 artifact 下载；边界：本地 artifact 可为空。
	DownloadURL string
	// LocalPath 职责：记录手动导入或本地 artifact 路径；使用：本地安装分支；边界：远端 artifact 可为空。
	LocalPath string
	// ArtifactKind 职责：记录 artifact 类型；使用：决定下载、解压和布局策略；边界：未知类型由规划阶段返回错误。
	ArtifactKind artifactConstant.ArtifactKind
}
