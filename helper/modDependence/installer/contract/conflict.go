package contract

// 文件职责：定义远端 Mod 安装冲突处理的前后端传输契约。
// 使用：installer 实现层在复制文件遇到冲突时生成，service 层接收前端选择后再传回 installer 继续执行。
// 边界：只表达冲突上下文和用户选择，不直接读取文件、修改安装会话或执行覆盖/跳过操作。

// InstallConflictAction 表示用户对安装冲突的处理方式。
// 职责：定义安装文件冲突的可选处理动作。
// 使用：前端弹窗选择覆盖、跳过或终止安装后传回安装器继续执行。
// 边界：未知动作会被 ContinueRemoteModPackageInstall 拒绝。
type InstallConflictAction string

const (
	// InstallConflictActionOverwrite 职责：覆盖当前目标文件，并把该文件的管理权转给当前安装包；使用：用户明确选择覆盖冲突时传入；边界：覆盖前会备份旧文件。
	InstallConflictActionOverwrite InstallConflictAction = "overwrite"
	// InstallConflictActionSkip 职责：跳过当前目标文件，不写入当前安装包文件清单；使用：用户选择保留已有文件时传入；边界：可能导致组件部分文件未安装。
	InstallConflictActionSkip InstallConflictAction = "skip"
	// InstallConflictActionFail 职责：终止当前安装，并保留失败状态给用户重新处理；使用：用户放弃处理冲突时传入；边界：会进入失败/清理流程。
	InstallConflictActionFail InstallConflictAction = "fail"
)

// InstallConflictDTO 描述一次需要用户处理的文件冲突。
// 职责：向前端描述目标文件冲突、来源组件和已有文件归属。
// 使用：通过 Wails 事件、安装快照和继续安装接口在前后端之间传递冲突上下文。
// 边界：只描述一次冲突，不代表整个安装会话状态。
type InstallConflictDTO struct {
	// PackageKey 职责：记录发生冲突的安装包 key；使用：前端继续安装时定位会话；边界：必须来自当前安装会话。
	PackageKey string `json:"packageKey"`
	// ComponentKey 职责：记录冲突文件所属组件 key；使用：组件级冲突决策复用；边界：不能为空。
	ComponentKey string `json:"componentKey"`
	// ComponentName 职责：记录冲突组件名称；使用：前端弹窗展示；边界：不作为唯一标识。
	ComponentName string `json:"componentName"`
	// ComponentVersion 职责：记录冲突组件版本；使用：前端展示和审计；边界：不做版本比较。
	ComponentVersion string `json:"componentVersion"`
	// TargetPath 职责：记录冲突目标绝对路径；使用：前端提示和后续决策匹配；边界：仅本机展示，不给前端直接操作。
	TargetPath string `json:"targetPath"`
	// RelativePath 职责：记录冲突目标相对游戏目录路径；使用：数据库匹配和前端简短展示；边界：由规划阶段生成。
	RelativePath string `json:"relativePath"`
	// IncomingSha256 职责：记录即将写入文件哈希；使用：判断是否与已有文件相同；边界：目录冲突时可能为空。
	IncomingSha256 string `json:"incomingSha256"`
	// ExistingSha256 职责：记录现有目标文件哈希；使用：前端说明冲突差异；边界：目录冲突或读取失败时可能为空。
	ExistingSha256 string `json:"existingSha256"`
	// ExistingPackageKey 职责：记录现有文件所属包 key；使用：提示覆盖会影响哪个包；边界：非受管理文件可能为空。
	ExistingPackageKey string `json:"existingPackageKey"`
	// ExistingPackageName 职责：记录现有文件所属包名称；使用：前端展示冲突来源；边界：非受管理文件可能为空。
	ExistingPackageName string `json:"existingPackageName"`
	// ExistingComponentKey 职责：记录现有文件所属组件 key；使用：诊断共享文件；边界：非受管理文件可能为空。
	ExistingComponentKey string `json:"existingComponentKey"`
	// ExistingComponentName 职责：记录现有文件所属组件名称；使用：前端展示冲突来源；边界：非受管理文件可能为空。
	ExistingComponentName string `json:"existingComponentName"`
	// ExistingComponentVersion 职责：记录现有文件所属组件版本；使用：前端展示冲突来源；边界：非受管理文件可能为空。
	ExistingComponentVersion string `json:"existingComponentVersion"`
	// Message 职责：保存用户可读冲突说明；使用：前端弹窗标题或详情；边界：不用于程序分支。
	Message string `json:"message"`
}

// InstallRemotePackageOptions 表示安装器可恢复执行时使用的用户决策。
// 职责：携带用户冲突决策和是否重置安装会话的控制参数。
// 使用：初次安装、继续冲突安装和测试恢复流程传入。
// 边界：冲突决策优先按具体文件匹配；同一组件的后续版本冲突会复用用户选择，避免 DLL/PDB/XML 等文件逐个弹窗。
type InstallRemotePackageOptions struct {
	// Decisions 职责：记录用户已做出的冲突决策；使用：继续安装时复用；边界：为空时遇到冲突会暂停。
	Decisions []InstallConflictDecisionDTO `json:"decisions"`
	// ResetSession 职责：控制是否重置既有安装会话；使用：初次安装时为 true，继续冲突时保持 false；边界：不导出到 JSON。
	ResetSession bool `json:"-"`
}

// InstallConflictDecisionDTO 记录用户对一次组件文件冲突的选择。
// 职责：保存目标文件、组件和处理动作。
// 使用：前端选择冲突处理方式后传回 ContinueRemotePackageInstall。
// 边界：只表达用户选择，不直接执行文件操作。
type InstallConflictDecisionDTO struct {
	// TargetPath 职责：记录决策对应的目标路径；使用：精确匹配同一个冲突文件；边界：为空时可能按组件复用。
	TargetPath string `json:"targetPath"`
	// ComponentKey 职责：记录决策对应组件；使用：复用同组件后续冲突决策；边界：不能为空。
	ComponentKey string `json:"componentKey"`
	// Action 职责：记录用户选择的冲突动作；使用：copyPlannedFiles 决定覆盖、跳过或失败；边界：非法值会被服务入口拒绝。
	Action InstallConflictAction `json:"action"`
}
