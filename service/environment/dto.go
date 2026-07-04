package environment

// 文件职责：定义环境管理 Wails 服务对前端暴露的 DTO。
// 使用：EnvironmentManagerService 查询接口返回这些结构，前端通过生成的 TypeScript 类型消费。
// 边界：DTO 只表达页面需要的数据，不携带 GORM 标签，不直接暴露数据库模型。

import (
	"encoding/json"
	"time"

	environmentHelper "DysonSphereBox/helper/environment"
)

// EnvironmentOverview 描述环境页顶部和遮罩所需的环境概览。
// 职责：聚合资源库、Mod 目录、蓝图目录、存档目录和只读计数。
// 使用：环境页初始化和点击刷新时调用 GetOverview 获取。
// 边界：计数来自轻量扫描，不代表启用/禁用完整状态复核。
type EnvironmentOverview struct {
	// AppDataRoot 职责：展示客户端环境资源库根目录；使用：诊断和打开目录入口；边界：目录可能尚未创建。
	AppDataRoot string `json:"appDataRoot"`
	// ModRoot 职责：展示游戏侧 BepInEx/plugins 目录解析结果；使用：Mod 标签遮罩和打开目录；边界：路径无效时 Path 为空。
	ModRoot environmentHelper.RootResolution `json:"modRoot"`
	// BlueprintRoot 职责：展示真实蓝图目录解析结果；使用：蓝图标签遮罩和写入确认；边界：needConfirm 时不得直接写入。
	BlueprintRoot environmentHelper.RootResolution `json:"blueprintRoot"`
	// SaveRoot 职责：展示真实存档目录解析结果；使用：存档标签遮罩和扫描；边界：路径缺失时列表为空。
	SaveRoot environmentHelper.RootResolution `json:"saveRoot"`
	// LibraryCounts 职责：展示客户端资源库中 Mod、蓝图和文件夹数量；使用：页头摘要和工具栏计数；边界：只统计当前文件系统。
	LibraryCounts EnvironmentLibraryCounts `json:"libraryCounts"`
	// InitialSync 职责：展示本机真实目录中可纳管资源数量；使用：首次同步提示入口；边界：第一阶段只预览不执行纳管。
	InitialSync EnvironmentInitialSyncPreview `json:"initialSync"`
}

// EnvironmentLibraryCounts 描述客户端资源库扫描计数。
// 职责：保存当前 AppData 资源库里的只读数量摘要。
// 使用：概览和前端工具栏展示。
// 边界：不包含真实游戏目录内尚未纳管的资源。
type EnvironmentLibraryCounts struct {
	// Mods 职责：保存客户端 Mod 资源库条目数量；使用：概览展示；边界：包含不支持裸 DLL 提示项。
	Mods int `json:"mods"`
	// Blueprints 职责：保存客户端蓝图文件数量；使用：概览展示；边界：不包含文件夹。
	Blueprints int `json:"blueprints"`
	// BlueprintFolders 职责：保存客户端蓝图文件夹数量；使用：概览展示；边界：只统计镜像目录树。
	BlueprintFolders int `json:"blueprintFolders"`
	// Saves 职责：保存真实存档目录当前文件数量；使用：概览展示；边界：存档不进入客户端资源库。
	Saves int `json:"saves"`
}

// EnvironmentInitialSyncPreview 描述首次同步可纳管数量。
// 职责：统计游戏真实目录中已有 Mod、蓝图和存档数量。
// 使用：GetOverview 和 PreviewInitialSync 返回给前端提示用户。
// 边界：第一阶段只读预览，不复制文件、不写入同步记录。
type EnvironmentInitialSyncPreview struct {
	// Kind 职责：保存预览范围；使用：区分 all、mod、blueprint、save；边界：不影响实际扫描逻辑之外的状态。
	Kind string `json:"kind"`
	// Mods 职责：保存游戏侧 Mod 目录数量；使用：首次同步提示；边界：只统计一级目录。
	Mods int `json:"mods"`
	// Blueprints 职责：保存真实蓝图根目录下可自动纳管文件数量；使用：首次同步提示；边界：只统计 .txt 和 .bp。
	Blueprints int `json:"blueprints"`
	// Saves 职责：保存真实存档目录下存档文件数量；使用：首次同步提示；边界：不复制存档。
	Saves int `json:"saves"`
	// Message 职责：保存用户可读同步提示；使用：前端提示条展示；边界：不用于业务分支。
	Message string `json:"message"`
}

// EnvironmentInitialSyncRequest 描述首次同步纳管请求。
// 职责：声明用户要纳管的资源类型范围。
// 使用：前端点击“同步本机内容”后调用 RunInitialSync。
// 边界：第一版只支持 all、mod、blueprint、save；未知类型由服务层回退 all。
type EnvironmentInitialSyncRequest struct {
	// Kind 职责：保存同步范围；使用：允许 all、mod、blueprint、save；边界：空值按 all 处理。
	Kind string `json:"kind"`
}

// EnvironmentInitialSyncResult 描述首次同步纳管结果。
// 职责：返回本次纳管成功的数量、失败数量和刷新后的概览。
// 使用：前端同步完成后刷新环境页数据。
// 边界：失败明细以文本返回；真实文件写入失败不会提前标记资源为成功。
type EnvironmentInitialSyncResult struct {
	// Kind 职责：保存本次同步范围；使用：前端展示结果摘要；边界：由服务层归一化。
	Kind string `json:"kind"`
	// SyncedMods 职责：保存成功纳管的 Mod 数量；使用：前端提示；边界：只统计目录型 Mod。
	SyncedMods int `json:"syncedMods"`
	// SyncedBlueprints 职责：保存成功纳管的蓝图文件数量；使用：前端提示；边界：只统计 .txt 和 .bp。
	SyncedBlueprints int `json:"syncedBlueprints"`
	// IndexedSaves 职责：保存成功索引的存档数量；使用：前端提示；边界：存档不复制到资源库。
	IndexedSaves int `json:"indexedSaves"`
	// Failures 职责：保存失败原因列表；使用：前端展示同步异常；边界：不包含完整堆栈。
	Failures []string `json:"failures"`
	// Overview 职责：保存同步后的最新概览；使用：前端同步刷新；边界：概览失败时服务方法直接返回错误。
	Overview EnvironmentOverview `json:"overview"`
	// OperationID 职责：保存首次同步操作记录 ID；使用：前端需要查询证据链时调用 GetOperation；边界：数据库未启用时为 0。
	OperationID uint `json:"operationId"`
}

// EnvironmentOperationResult 描述单个资源写入操作结果。
// 职责：返回启用、禁用、重命名等操作后的资源状态和提示。
// 使用：前端完成用户动作后刷新卡片状态。
// 边界：Message 只用于展示；是否成功由错误返回决定。
type EnvironmentOperationResult struct {
	// Message 职责：保存用户可读操作结果；使用：前端 toast 展示；边界：不用于程序状态判断。
	Message string `json:"message"`
	// OperationID 职责：保存本次写操作记录 ID；使用：前端需要追踪长任务或查询证据时调用 GetOperation；边界：数据库未启用或未记录时为 0。
	OperationID uint `json:"operationId"`
	// Mod 职责：保存 Mod 操作后的卡片数据；使用：EnableMod/DisableMod 返回；边界：非 Mod 操作为空。
	Mod *EnvironmentModItem `json:"mod,omitempty"`
	// Mods 职责：保存 Mod 导入或删除后的列表数据；使用：ImportMod/DeleteMod 返回；边界：非批量 Mod 操作为空。
	Mods []EnvironmentModItem `json:"mods,omitempty"`
	// BlueprintPage 职责：保存蓝图操作后的当前目录数据；使用：蓝图启用、禁用、创建文件夹、重命名后刷新；边界：非蓝图操作为空。
	BlueprintPage *EnvironmentBlueprintPage `json:"blueprintPage,omitempty"`
	// Saves 职责：保存存档操作后的扫描列表；使用：RenameSave 返回；边界：非存档操作为空。
	Saves []EnvironmentSaveItem `json:"saves,omitempty"`
}

// EnvironmentOperation 描述环境管理操作记录。
// 职责：向前端返回 EnvOperation 的可读状态、消息和结构化详情。
// 使用：GetOperation 以及后续操作列表页调用。
// 边界：Detail 保持 JSON 原始结构，不在 DTO 层解释业务字段。
type EnvironmentOperation struct {
	// ID 职责：保存操作记录 ID；使用：前端查询单个操作；边界：本地数据库自增值，不跨设备稳定。
	ID uint `json:"id"`
	// Operation 职责：保存操作动作；使用：展示 import、enable、disable、delete 等动作；边界：不在 DTO 层校验枚举。
	Operation string `json:"operation"`
	// Kind 职责：保存资源类型；使用：区分 mod、blueprint、save 或 all；边界：未知值原样返回。
	Kind string `json:"kind"`
	// ItemID 职责：保存关联资源 ID；使用：定位操作对应卡片；边界：批量操作或存档操作可为空。
	ItemID string `json:"itemId"`
	// Status 职责：保存操作状态；使用：展示 pending、running、success、failed；边界：状态迁移由服务层控制。
	Status string `json:"status"`
	// Message 职责：保存用户可读消息；使用：操作详情展示；边界：结构化证据仍读取 Detail。
	Message string `json:"message"`
	// Detail 职责：保存结构化操作计划和结果；使用：冲突处理、诊断和后续恢复；边界：只保证是合法 JSON 值。
	Detail json.RawMessage `json:"detail"`
	// CreatedAt 职责：保存操作创建时间；使用：前端排序和展示；边界：不代表完成时间。
	CreatedAt time.Time `json:"createdAt"`
}

// EnvironmentManagedStateSummary 描述托管状态复核结果。
// 职责：汇总 RefreshManagedState 对同步记录、目标根和真实文件的复核数量。
// 使用：环境页进入或用户点击“复核状态”时展示。
// 边界：只处理已登记的 mod/blueprint 同步记录，不扫描未纳管文件。
type EnvironmentManagedStateSummary struct {
	// Kind 职责：保存本次复核范围；使用：前端区分 mod、blueprint、all；边界：由服务层归一化。
	Kind string `json:"kind"`
	// CheckedRecords 职责：保存已检查的同步文件记录数量；使用：展示复核覆盖范围；边界：不等同资源数量。
	CheckedRecords int `json:"checkedRecords"`
	// CheckedItems 职责：保存涉及的资源数量；使用：展示复核资源范围；边界：按 itemId 去重统计。
	CheckedItems int `json:"checkedItems"`
	// ChangedItems 职责：保存被更新状态的资源数量；使用：提示用户是否有状态变化；边界：重复问题只计一次资源。
	ChangedItems int `json:"changedItems"`
	// ConflictItems 职责：保存标记为 conflict 的资源数量；使用：提示需要人工处理；边界：根目录变化和 hash 不一致均计入。
	ConflictItems int `json:"conflictItems"`
	// MissingItems 职责：保存标记为 missing 的资源数量；使用：提示目标文件缺失；边界：只代表登记目标不存在。
	MissingItems int `json:"missingItems"`
	// Issues 职责：保存复核发现的问题列表；使用：前端详情或日志展示；边界：数量可能少于记录数量，因为同资源会归并。
	Issues []EnvironmentManagedStateIssue `json:"issues"`
	// OperationID 职责：保存复核操作记录 ID；使用：后续查询复核证据；边界：数据库不可用时为 0。
	OperationID uint `json:"operationId"`
}

// EnvironmentManagedStateIssue 描述单个资源的托管状态问题。
// 职责：记录 root_changed、hash_mismatch、missing_target 等复核问题。
// 使用：RefreshManagedState 返回给前端用于提示和后续冲突处理。
// 边界：不包含完整文件内容，不直接授权删除或覆盖。
type EnvironmentManagedStateIssue struct {
	// ItemID 职责：保存问题所属资源 ID；使用：前端定位资源卡片；边界：同步记录异常时可能为空。
	ItemID string `json:"itemId"`
	// Kind 职责：保存资源类型；使用：区分 Mod 和蓝图；边界：未知值原样返回。
	Kind string `json:"kind"`
	// State 职责：保存服务层建议写入的资源状态；使用：展示 conflict 或 missing；边界：不是临时 syncing 状态。
	State string `json:"state"`
	// Reason 职责：保存机器可读问题原因；使用：后续冲突处理分支；边界：当前只覆盖复核发现的基础原因。
	Reason string `json:"reason"`
	// Message 职责：保存用户可读说明；使用：toast 或详情列表展示；边界：不用于业务判断。
	Message string `json:"message"`
	// RelativePath 职责：保存目标相对路径；使用：帮助用户定位具体文件；边界：不暴露扫描根之外的相对路径。
	RelativePath string `json:"relativePath"`
	// TargetPath 职责：保存登记时的目标路径；使用：诊断旧同步记录；边界：根目录变化时不得据此删除旧文件。
	TargetPath string `json:"targetPath"`
	// ExpectedRoot 职责：保存当前解析出的目标根；使用：解释 root_changed；边界：为空表示当前根不可用。
	ExpectedRoot string `json:"expectedRoot"`
	// RecordedRoot 职责：保存同步记录中的旧目标根；使用：解释 root_changed；边界：不代表当前有效根。
	RecordedRoot string `json:"recordedRoot"`
}

// EnvironmentRecoverySummary 描述环境管理启动恢复结果。
// 职责：汇总未完成 operation 的恢复数量、临时文件清理和托管状态复核结果。
// 使用：RecoverInterruptedOperations 返回给启动流程、环境页进入和后续诊断面板。
// 边界：只表达恢复摘要，不携带文件内容；无未完成操作时 RecoveredOperations 为 0。
type EnvironmentRecoverySummary struct {
	// PendingOperations 职责：保存恢复前发现的未完成操作数量；使用：判断是否发生恢复流程；边界：只统计环境管理 operation。
	PendingOperations int `json:"pendingOperations"`
	// RecoveredOperations 职责：保存成功标记为 recovered 的操作数量；使用：前端或日志展示；边界：不代表所有冲突都已自动解决。
	RecoveredOperations int `json:"recoveredOperations"`
	// FailedOperations 职责：保存恢复失败并标记为 failed 的操作数量；使用：提示用户检查异常；边界：失败详情在 FailedPaths 或 operation 详情中。
	FailedOperations int `json:"failedOperations"`
	// RemovedPaths 职责：保存恢复时清理掉的临时路径；使用：诊断和测试验证；边界：只包含明确属于环境管理的临时文件或目录。
	RemovedPaths []string `json:"removedPaths"`
	// FailedPaths 职责：保存临时路径清理失败列表；使用：提示无法完成自动恢复的路径；边界：只记录路径和错误摘要。
	FailedPaths []string `json:"failedPaths"`
	// ManagedState 职责：保存恢复后的托管状态复核摘要；使用：前端刷新后解释 conflict/missing 变化；边界：为空表示没有执行复核。
	ManagedState *EnvironmentManagedStateSummary `json:"managedState,omitempty"`
}

// EnvironmentPathSelection 描述系统文件或目录选择器结果。
// 职责：把用户选择的一个或多个路径返回给前端写入流程。
// 使用：环境页导入 Mod、蓝图、存档或选择导出目录时调用选择器接口。
// 边界：用户取消时 Paths 为空且 Canceled 为 true；DTO 不校验路径业务合法性。
type EnvironmentPathSelection struct {
	// Paths 职责：保存选择器返回的路径列表；使用：前端传给导入或导出接口；边界：用户取消时为空。
	Paths []string `json:"paths"`
	// Canceled 职责：标记用户是否取消选择；使用：前端避免弹错误提示；边界：只表达选择器行为，不代表路径有效。
	Canceled bool `json:"canceled"`
}

// EnvironmentRootDirectoryResult 描述环境根目录配置结果。
// 职责：返回保存后的根目录解析状态和最新概览。
// 使用：环境页手动选择蓝图或存档目录后刷新页面状态。
// 边界：只处理 blueprint/save 根目录，不改变游戏启动路径或客户端资源库。
type EnvironmentRootDirectoryResult struct {
	// Kind 职责：保存配置的目录类型；使用：前端区分刷新蓝图或存档标签；边界：只允许 blueprint 或 save。
	Kind string `json:"kind"`
	// Root 职责：保存保存后的目录解析结果；使用：前端立即展示新目录状态；边界：不代表目录内资源已同步。
	Root environmentHelper.RootResolution `json:"root"`
	// Overview 职责：保存最新环境概览；使用：前端同步刷新页头和首次同步提示；边界：概览扫描失败时服务层返回错误。
	Overview EnvironmentOverview `json:"overview"`
}

// EnvironmentBlueprintMigrationPreview 描述蓝图移动或重命名前的游戏侧迁移预览。
// 职责：列出已启用蓝图受影响文件、新旧真实路径、冲突数量和是否需要用户确认。
// 使用：环境页拖拽移动或重命名已启用蓝图/文件夹前调用 PreviewBlueprintMigration。
// 边界：只读扫描与同步记录，不移动、不删除、不写数据库。
type EnvironmentBlueprintMigrationPreview struct {
	// ItemID 职责：保存被移动或重命名的蓝图资源 ID；使用：前端确认后继续执行对应动作；边界：由当前资源库扫描生成。
	ItemID string `json:"itemId"`
	// Operation 职责：保存预览操作类型 move 或 rename；使用：前端区分目标参数展示；边界：不作为执行授权。
	Operation string `json:"operation"`
	// TargetParentID 职责：保存移动目标父文件夹 ID；使用：拖拽移动预览；边界：重命名时为空。
	TargetParentID string `json:"targetParentId"`
	// TargetName 职责：保存重命名目标名称；使用：重命名预览；边界：移动时为空。
	TargetName string `json:"targetName"`
	// RequiresConfirmation 职责：标记是否存在已启用蓝图需要迁移确认；使用：前端决定是否弹出预览；边界：false 不代表执行一定成功。
	RequiresConfirmation bool `json:"requiresConfirmation"`
	// EnabledBlueprints 职责：保存受影响的已启用蓝图数量；使用：前端摘要展示；边界：只统计 environment_items 为 enabled 的蓝图。
	EnabledBlueprints int `json:"enabledBlueprints"`
	// Conflicts 职责：保存预览阶段发现的新目标路径冲突数量；使用：前端阻止直接确认迁移；边界：真实执行时仍需再次校验。
	Conflicts int `json:"conflicts"`
	// Entries 职责：保存每个受影响蓝图的迁移路径；使用：前端展示旧路径、新路径和冲突原因；边界：不包含文件内容。
	Entries []EnvironmentBlueprintMigrationEntry `json:"entries"`
}

// EnvironmentBlueprintMigrationEntry 描述单个蓝图文件迁移预览项。
// 职责：保存资源 ID、名称、新旧游戏侧路径和冲突原因。
// 使用：EnvironmentBlueprintMigrationPreview.Entries 渲染迁移预览表。
// 边界：只表达预览结果，不直接驱动删除或覆盖。
type EnvironmentBlueprintMigrationEntry struct {
	// ItemID 职责：保存蓝图文件资源 ID；使用：前端定位卡片；边界：文件夹本身不会出现在条目中。
	ItemID string `json:"itemId"`
	// Name 职责：保存蓝图名称；使用：前端列表展示；边界：来自当前资源库扫描。
	Name string `json:"name"`
	// OldRelativePath 职责：保存旧客户端镜像相对路径；使用：解释迁移来源；边界：slash 格式。
	OldRelativePath string `json:"oldRelativePath"`
	// NewRelativePath 职责：保存新客户端镜像相对路径；使用：解释迁移目标；边界：slash 格式。
	NewRelativePath string `json:"newRelativePath"`
	// OldTargetPath 职责：保存旧游戏侧目标路径；使用：前端打开目录或诊断；边界：路径可能已经不存在。
	OldTargetPath string `json:"oldTargetPath"`
	// NewTargetPath 职责：保存新游戏侧目标路径；使用：前端提示迁移后位置；边界：真实执行前仍需校验。
	NewTargetPath string `json:"newTargetPath"`
	// Conflict 职责：标记预览目标是否存在冲突；使用：前端阻止无预案迁移；边界：只基于当前文件系统瞬时检查。
	Conflict bool `json:"conflict"`
	// ConflictReason 职责：保存冲突原因；使用：前端说明目标已存在等问题；边界：为空表示未发现冲突。
	ConflictReason string `json:"conflictReason"`
}

// EnvironmentBlueprintMigrationRequest 描述用户确认后的蓝图迁移请求。
// 职责：携带预览使用的操作类型、目标父节点和目标名称，供服务层重新复核后执行迁移。
// 使用：环境页迁移预览弹窗确认后调用 ConfirmBlueprintMigration。
// 边界：请求不携带文件路径，真实路径必须由服务层重新扫描和校验生成。
type EnvironmentBlueprintMigrationRequest struct {
	// ItemID 职责：保存被移动或重命名的蓝图资源 ID；使用：服务层定位当前客户端镜像节点；边界：执行前必须重新扫描确认存在。
	ItemID string `json:"itemId"`
	// Operation 职责：保存 move 或 rename；使用：服务层决定目标路径计算方式；边界：其它值直接拒绝。
	Operation string `json:"operation"`
	// TargetParentID 职责：保存移动目标父文件夹 ID；使用：move 操作计算新路径；边界：rename 操作忽略。
	TargetParentID string `json:"targetParentId"`
	// TargetName 职责：保存重命名目标名称；使用：rename 操作计算新路径；边界：move 操作忽略。
	TargetName string `json:"targetName"`
}

// EnvironmentModItem 描述环境页 Mod 卡片数据。
// 职责：向前端提供 Mod 资源库只读摘要。
// 使用：ListMods 返回给模组标签页。
// 边界：启用/禁用接口尚未实现时，State 只表达当前扫描或持久记录状态。
type EnvironmentModItem struct {
	// ID 职责：保存资源 ID；使用：前端列表 key 和后续操作参数；边界：第一阶段由路径派生。
	ID string `json:"id"`
	// Name 职责：保存显示名；使用：卡片主标题；边界：manifest 缺失时为目录名。
	Name string `json:"name"`
	// Version 职责：保存版本号；使用：卡片副标题；边界：缺失时为空。
	Version string `json:"version"`
	// Author 职责：保存作者；使用：详情扩展；边界：缺失时为空。
	Author string `json:"author"`
	// Description 职责：保存说明；使用：详情扩展；边界：不做富文本渲染。
	Description string `json:"description"`
	// State 职责：保存展示状态；使用：卡片边框和状态点；边界：syncing 不写入该字段。
	State string `json:"state"`
	// SizeBytes 职责：保存资源大小；使用：前端格式化展示；边界：目录大小为估算。
	SizeBytes int64 `json:"sizeBytes"`
	// UpdatedAt 职责：保存最新修改时间；使用：前端展示；边界：零值表示未知。
	UpdatedAt time.Time `json:"updatedAt"`
	// LibraryPath 职责：保存资源库相对路径；使用：诊断和后续打开资源目录；边界：不暴露绝对路径。
	LibraryPath string `json:"libraryPath"`
	// Supported 职责：标记第一版是否支持管理；使用：裸 DLL 提示禁用操作；边界：不等同安全校验。
	Supported bool `json:"supported"`
	// Warning 职责：保存扫描警告；使用：前端提示；边界：为空表示无提示。
	Warning string `json:"warning"`
}

// EnvironmentModDetail 描述 Mod 详情弹窗数据。
// 职责：向前端展示 manifest 解析信息、依赖、官网链接和原始 JSON 证据。
// 使用：环境页右键 Mod 选择“查看详情”时通过 GetModDetail 获取。
// 边界：只读客户端资源库镜像，不扫描远端市场，不执行启用或导入。
type EnvironmentModDetail struct {
	// ID 职责：保存 Mod 资源 ID；使用：前端保持详情与卡片关联；边界：由当前资源库相对路径派生。
	ID string `json:"id"`
	// Name 职责：保存显示名；使用：详情标题和基础信息展示；边界：manifest 缺失时为目录名。
	Name string `json:"name"`
	// Version 职责：保存版本号；使用：详情基础信息展示；边界：manifest 缺失时为空。
	Version string `json:"version"`
	// Author 职责：保存作者；使用：详情基础信息展示；边界：manifest 缺失时为空。
	Author string `json:"author"`
	// Description 职责：保存说明文本；使用：详情正文展示；边界：不做 Markdown 渲染。
	Description string `json:"description"`
	// WebsiteURL 职责：保存 manifest 中的网站链接；使用：前端提供可复制或外部打开的文本；边界：不校验远端可达性。
	WebsiteURL string `json:"websiteUrl"`
	// Dependencies 职责：保存 manifest 依赖列表；使用：详情弹窗展示依赖名称；边界：不解析版本约束语义。
	Dependencies []EnvironmentModDependency `json:"dependencies"`
	// LibraryPath 职责：保存资源库相对路径；使用：诊断和定位；边界：不暴露 AppData 绝对根。
	LibraryPath string `json:"libraryPath"`
	// Supported 职责：保存是否支持第一版管理；使用：详情提示裸 DLL 等不可管理资源；边界：不替代后端写入校验。
	Supported bool `json:"supported"`
	// Warning 职责：保存扫描警告；使用：详情提示不可管理原因；边界：为空表示无警告。
	Warning string `json:"warning"`
	// RawManifest 职责：保存 manifest 原始 JSON 文本；使用：详情弹窗展示原始解析信息；边界：缺失时为空对象文本。
	RawManifest string `json:"rawManifest"`
}

// EnvironmentModDependency 描述 Mod manifest 中的单条依赖。
// 职责：把字符串或对象形式的依赖归一化为前端可展示结构。
// 使用：EnvironmentModDetail.Dependencies 渲染依赖列表。
// 边界：不访问远端市场，不判断依赖是否已安装。
type EnvironmentModDependency struct {
	// Name 职责：保存依赖名称或原始字符串；使用：前端列表展示；边界：无法解析时仍保留原文本。
	Name string `json:"name"`
	// Version 职责：保存依赖版本提示；使用：前端辅助展示；边界：字段缺失时为空。
	Version string `json:"version"`
}

// EnvironmentBlueprintPage 描述蓝图目录页数据。
// 职责：返回当前父节点下的蓝图文件夹和蓝图文件，以及面包屑。
// 使用：ListBlueprints 返回给蓝图标签页。
// 边界：第一阶段目录树来自文件系统扫描，不执行移动或重命名。
type EnvironmentBlueprintPage struct {
	// ParentID 职责：保存当前父节点 ID；使用：前端恢复当前目录；边界：根目录为空。
	ParentID string `json:"parentId"`
	// Breadcrumbs 职责：保存从根到当前目录的面包屑；使用：前端导航；边界：路径不存在时可能为空。
	Breadcrumbs []EnvironmentBlueprintBreadcrumb `json:"breadcrumbs"`
	// Items 职责：保存当前目录可见节点；使用：前端网格渲染；边界：已按文件夹优先排序。
	Items []EnvironmentBlueprintItem `json:"items"`
	// TotalBlueprints 职责：保存整个资源库蓝图文件数量；使用：工具栏计数；边界：不含文件夹。
	TotalBlueprints int `json:"totalBlueprints"`
	// TotalFolders 职责：保存整个资源库文件夹数量；使用：工具栏计数；边界：只统计镜像目录树。
	TotalFolders int `json:"totalFolders"`
}

// EnvironmentBlueprintBreadcrumb 描述蓝图目录面包屑节点。
// 职责：保存节点 ID 和展示名。
// 使用：前端点击返回上级目录时传入 ID。
// 边界：根目录由前端固定展示，不需要后端返回。
type EnvironmentBlueprintBreadcrumb struct {
	// ID 职责：保存文件夹节点 ID；使用：前端切换目录；边界：根目录为空。
	ID string `json:"id"`
	// Name 职责：保存文件夹名称；使用：面包屑展示；边界：只显示文件系统名称。
	Name string `json:"name"`
}

// EnvironmentBlueprintItem 描述蓝图文件夹或蓝图文件卡片。
// 职责：向前端提供蓝图目录树节点摘要。
// 使用：EnvironmentBlueprintPage.Items 渲染。
// 边界：第一阶段不携带可写操作计划。
type EnvironmentBlueprintItem struct {
	// ID 职责：保存节点 ID；使用：列表 key 和目录跳转；边界：由相对路径派生。
	ID string `json:"id"`
	// ParentID 职责：保存父节点 ID；使用：前端过滤和调试；边界：根目录为空。
	ParentID string `json:"parentId"`
	// Type 职责：保存 folder 或 blueprint；使用：选择图标和交互；边界：只读扫描结果。
	Type string `json:"type"`
	// Name 职责：保存节点名称；使用：卡片标题；边界：蓝图文件去扩展名。
	Name string `json:"name"`
	// State 职责：保存蓝图展示状态；使用：状态边框；边界：文件夹为空。
	State string `json:"state"`
	// SizeBytes 职责：保存节点大小；使用：前端展示；边界：目录大小为递归估算。
	SizeBytes int64 `json:"sizeBytes"`
	// UpdatedAt 职责：保存更新时间；使用：排序或详情；边界：零值表示未知。
	UpdatedAt time.Time `json:"updatedAt"`
	// RelativePath 职责：保存相对蓝图资源库根路径；使用：后续移动和同步；边界：不包含绝对路径。
	RelativePath string `json:"relativePath"`
	// Extension 职责：保存文件扩展名；使用：提示未知扩展；边界：文件夹为空。
	Extension string `json:"extension"`
	// ChildCount 职责：保存文件夹直接子项数量；使用：文件夹卡片展示；边界：蓝图文件为 0。
	ChildCount int `json:"childCount"`
}

// EnvironmentBlueprintDetail 描述单个蓝图文件详情。
// 职责：把资源库扫描摘要、文件位置、原始编码和解析后的蓝图结构返回给前端。
// 使用：环境页蓝图右键“查看详情”打开 Drawer 时调用 GetBlueprintDetail。
// 边界：只读客户端蓝图镜像文件，不修改文件或启用状态；完整编码仅用于前端嵌入式模型预览器渲染模型。
type EnvironmentBlueprintDetail struct {
	// Item 职责：保存当前蓝图卡片摘要；使用：详情标题、路径和状态展示；边界：来自当前扫描结果，资源被移动后可能变化。
	Item EnvironmentBlueprintItem `json:"item"`
	// LibraryPath 职责：保存相对客户端资源库的路径；使用：诊断和 UI 展示；边界：不暴露 AppData 绝对路径。
	LibraryPath string `json:"libraryPath"`
	// Code 职责：保存原始 BLUEPRINT 编码；使用：前端嵌入式模型预览器调用 loadBlueprint 渲染；边界：只读传输，不用于前端编辑或写回文件。
	Code string `json:"code"`
	// Parsed 职责：保存蓝图编码解析结果；使用：详情 Drawer 展示元信息、区域和建筑统计；边界：解析失败时服务方法直接返回错误。
	Parsed environmentHelper.BlueprintDecodeResult `json:"parsed"`
}

// EnvironmentSaveItem 描述存档卡片数据。
// 职责：向前端提供真实 SaveRoot 文件摘要。
// 使用：ListSaves 和 RefreshSaves 返回。
// 边界：存档操作前仍需后端重新确认文件存在。
type EnvironmentSaveItem struct {
	// ID 职责：保存扫描 ID；使用：前端列表 key 和后续操作参数；边界：更换 SaveRoot 后可能变化。
	ID string `json:"id"`
	// Name 职责：保存显示名；使用：卡片标题；边界：由文件名派生。
	Name string `json:"name"`
	// FileName 职责：保存真实文件名；使用：详情和导出默认名；边界：不包含目录。
	FileName string `json:"fileName"`
	// RelativePath 职责：保存相对 SaveRoot 路径；使用：后续操作定位；边界：不暴露绝对路径。
	RelativePath string `json:"relativePath"`
	// SizeBytes 职责：保存文件大小；使用：前端展示；边界：不代表可写性。
	SizeBytes int64 `json:"sizeBytes"`
	// LastModifiedAt 职责：保存最后修改时间；使用：前端显示最近保存；边界：来自文件系统。
	LastModifiedAt time.Time `json:"lastModifiedAt"`
	// Status 职责：保存扫描状态；使用：前端状态提示；边界：第一阶段不做占用检测。
	Status string `json:"status"`
}
