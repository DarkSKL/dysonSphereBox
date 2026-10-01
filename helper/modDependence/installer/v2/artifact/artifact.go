package artifact

// 文件职责：定义 V2 安装器内部 artifact 类型。
// 使用：V2 安装解析、下载、解压、复制、会话清理和安装记录保存共享本包类型。
// 边界：不下载文件、不访问数据库、不发送进度事件；DTO 转换和布局辅助方法由同包独立文件维护。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
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
	// Namespace 职责：记录来源命名空间（Thunderstore owner）；使用：生成 R2 同款 Owner-Name 目录名与同 Mod 判断；边界：manual 来源可能为空。
	Namespace string
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
	// ReuseExisting 职责：标记是否复用本地组件；使用：建立引用并跳过下载和复制；边界：主包与整合包只复用相同版本，手动来源保持可重复导入。
	ReuseExisting bool
	// Dependencies 职责：保存当前组件版本的直接依赖；使用：复用组件时继续建立传递依赖关联；边界：nil 表示历史记录尚无清单，空切片表示已知没有依赖。
	Dependencies []modDependenceResponse.RemoteModDependencyDTO
}
