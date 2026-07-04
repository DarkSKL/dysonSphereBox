package environment

// 文件职责：提供环境管理页目录解析、资源扫描和核心文件同步 Wails 服务。
// 使用：前端环境页通过生成绑定获取概览、列表，并执行首次纳管、启用、禁用和安全重命名。
// 边界：服务方法必须等真实文件操作和数据库提交完成后才返回成功，不执行远端市场查询。

import (
	"DysonSphereBox/core"
	environmentModel "DysonSphereBox/model/environment"
	"DysonSphereBox/utils"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/modDependence/fileops"

	"gorm.io/gorm"
)

// EnvironmentManagerService 提供环境管理页的 Wails 绑定能力。
// 职责：编排目录解析、资源库扫描、首次纳管、启用禁用同步和存档安全操作。
// 使用：环境页进入、切换标签、点击刷新或执行资源操作时调用。
// 边界：不负责远端 Mod 市场安装；冲突时返回错误并保留原文件状态。
type EnvironmentManagerService struct{}

// blueprintMigrationCommitEntry 描述确认迁移阶段单个已启用蓝图的提交计划。
// 职责：保存预览条目、旧同步证据、新文件证据和迁移后的资源身份。
// 使用：confirmBlueprintMigrationWithoutOperation 在复制、移动和事务更新之间传递一致上下文。
// 边界：仅用于服务内部编排，不暴露给前端，也不直接执行文件删除。
type blueprintMigrationCommitEntry struct {
	// PreviewEntry 职责：保存重新生成的迁移预览条目；使用：执行路径和失败提示；边界：执行前仍需结合数据库旧证据复核。
	PreviewEntry EnvironmentBlueprintMigrationEntry
	// OldRecord 职责：保存迁移前的同步记录；使用：旧目标 hash 删除和旧记录定位；边界：找不到记录时迁移拒绝继续。
	OldRecord environmentModel.EnvSyncedFile
	// NewPlan 职责：保存复制到新游戏侧目标后的证据；使用：写入新的 EnvSyncedFile；边界：客户端镜像移动前 SourcePath 仍需重定向。
	NewPlan environmentHelper.SyncedFilePlan
	// NewResource 职责：保存客户端镜像迁移后扫描出的蓝图资源；使用：迁移 EnvItem 身份、父节点和展示字段；边界：扫描不到时数据库不提交成功。
	NewResource environmentHelper.BlueprintResource
}

// blueprintLibraryMigrationResult 描述客户端蓝图镜像节点迁移结果。
// 职责：保存迁移前后节点路径和迁移后应刷新的目录 ID。
// 使用：确认迁移流程在数据库失败时尝试回滚客户端镜像，并在成功后刷新列表。
// 边界：只描述客户端镜像节点，不包含游戏侧文件和同步记录。
type blueprintLibraryMigrationResult struct {
	// OldPath 职责：保存迁移前客户端镜像节点绝对路径；使用：失败回滚；边界：执行成功后通常不存在。
	OldPath string
	// NewPath 职责：保存迁移后客户端镜像节点绝对路径；使用：失败回滚和诊断；边界：路径是否存在由调用方检查。
	NewPath string
	// RefreshParentID 职责：保存迁移成功后前端应刷新的目录 ID；使用：ListBlueprints 调用；边界：根目录为空字符串。
	RefreshParentID string
	// Changed 职责：标记客户端镜像是否真实移动；使用：决定是否需要回滚；边界：false 表示路径未变化。
	Changed bool
}

// GetOverview 返回环境管理概览。
// 职责：聚合 AppData、ModRoot、BlueprintRoot、SaveRoot 和只读计数。
// 使用：环境页初始化和刷新顶部状态时调用。
// 边界：目录不存在时返回空计数和明确状态，不自动创建目录。
// 逻辑流：1. 解析各类根目录 => 2. 扫描客户端资源库和真实存档 => 3. 统计首次同步预览 => 4. 返回前端概览 DTO。
func (service *EnvironmentManagerService) GetOverview() (EnvironmentOverview, error) {
	if _, err := service.RecoverInterruptedOperations(); err != nil {
		return EnvironmentOverview{}, err
	}

	return service.getOverviewWithoutRecovery()
}

// getOverviewWithoutRecovery 构建环境管理概览但不触发恢复。
// 职责：供公开 GetOverview 和内部写操作完成后复用同一概览逻辑。
// 使用：GetOverview 恢复后调用，RunInitialSync 等当前 operation 内部调用。
// 边界：不扫描 pending/running operation，避免误恢复当前正在执行的操作。
func (service *EnvironmentManagerService) getOverviewWithoutRecovery() (EnvironmentOverview, error) {
	resolver := environmentHelper.Resolver{}
	appDataRoot, err := resolver.AppDataRoot()
	if err != nil {
		return EnvironmentOverview{}, err
	}

	modRoot := resolver.ResolveModRoot()
	blueprintRoot := resolver.ResolveBlueprintRoot()
	saveRoot := resolver.ResolveSaveRoot()
	counts, err := service.libraryCounts(resolver, saveRoot)
	if err != nil {
		return EnvironmentOverview{}, err
	}
	preview := service.buildInitialSyncPreview("all", modRoot, blueprintRoot, saveRoot)

	return EnvironmentOverview{
		AppDataRoot:   appDataRoot,
		ModRoot:       modRoot,
		BlueprintRoot: blueprintRoot,
		SaveRoot:      saveRoot,
		LibraryCounts: counts,
		InitialSync:   preview,
	}, nil
}

// ListMods 返回客户端 Mod 资源库列表。
// 职责：扫描 AppData/plugins 一级资源并按关键词过滤。
// 使用：环境页模组标签初始化、搜索和刷新时调用。
// 边界：不会扫描游戏侧 plugins；首次纳管通过 PreviewInitialSync 展示。
func (service *EnvironmentManagerService) ListMods(query string) ([]EnvironmentModItem, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return nil, err
	}
	resources, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return nil, err
	}
	states, err := service.loadItemStates("mod")
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]EnvironmentModItem, 0, len(resources))
	for _, resource := range resources {
		if query != "" && !strings.Contains(strings.ToLower(resource.Name), query) {
			continue
		}
		if state := states[resource.ID]; state != "" {
			resource.State = state
		}
		items = append(items, modResourceToListItem(resource))
	}
	return items, nil
}

// GetModDetail 返回客户端 Mod 资源库详情。
// 职责：按 Mod ID 读取当前资源库扫描结果，并返回 manifest 依赖、官网和原始 JSON。
// 使用：环境页 Mod 右键“查看详情”弹窗调用。
// 边界：只读 AppData/plugins 镜像，不读取远端市场，不修改数据库或文件状态。
func (service *EnvironmentManagerService) GetModDetail(itemId string) (EnvironmentModDetail, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return EnvironmentModDetail{}, err
	}
	resources, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return EnvironmentModDetail{}, err
	}
	for _, resource := range resources {
		if resource.ID == itemId {
			return modResourceToDetail(resource)
		}
	}
	return EnvironmentModDetail{}, fmt.Errorf("未找到 Mod 资源")
}

// ListBlueprints 返回客户端蓝图目录页。
// 职责：扫描 AppData/blueprints 镜像目录并返回指定父节点下的节点。
// 使用：环境页蓝图标签进入文件夹、搜索和刷新时调用。
// 边界：第一阶段目录树来自文件系统，不执行数据库移动或重命名。
func (service *EnvironmentManagerService) ListBlueprints(parentId string, query string) (EnvironmentBlueprintPage, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentBlueprintPage{}, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentBlueprintPage{}, err
	}
	states, err := service.loadItemStates("blueprint")
	if err != nil {
		return EnvironmentBlueprintPage{}, err
	}

	query = strings.ToLower(strings.TrimSpace(query))
	childrenByParent := make(map[string]int)
	byID := make(map[string]environmentHelper.BlueprintResource)
	totalBlueprints := 0
	totalFolders := 0
	for _, resource := range resources {
		byID[resource.ID] = resource
		childrenByParent[resource.ParentID]++
		if resource.Type == "folder" {
			totalFolders++
		} else {
			totalBlueprints++
		}
	}

	items := make([]EnvironmentBlueprintItem, 0)
	for _, resource := range resources {
		if resource.ParentID != parentId {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(resource.Name), query) {
			continue
		}
		if state := states[resource.ID]; state != "" && resource.Type == "blueprint" {
			resource.State = state
		}
		items = append(items, EnvironmentBlueprintItem{
			ID:           resource.ID,
			ParentID:     resource.ParentID,
			Type:         resource.Type,
			Name:         resource.Name,
			State:        resource.State,
			SizeBytes:    resource.SizeBytes,
			UpdatedAt:    resource.UpdatedAt,
			RelativePath: resource.RelativePath,
			Extension:    resource.Extension,
			ChildCount:   childrenByParent[resource.ID],
		})
	}

	return EnvironmentBlueprintPage{
		ParentID:        parentId,
		Breadcrumbs:     buildBreadcrumbs(parentId, byID),
		Items:           items,
		TotalBlueprints: totalBlueprints,
		TotalFolders:    totalFolders,
	}, nil
}

// GetBlueprintDetail 返回客户端蓝图文件详情。
// 职责：按蓝图 ID 定位 AppData/blueprints 中的单个蓝图文件，读取编码并解析为结构化信息。
// 使用：环境页蓝图卡片右键“查看详情”打开 Drawer 时调用。
// 边界：只读客户端镜像文件；文件夹、缺失文件、非 BLUEPRINT 文本或暂不支持格式会返回错误。
// 逻辑流：1. 扫描资源库定位蓝图 => 2. 校验文件仍在蓝图资源库根内 => 3. 读取文本并解析 => 4. 返回资源摘要、原始编码与解析结果。
func (service *EnvironmentManagerService) GetBlueprintDetail(itemId string) (EnvironmentBlueprintDetail, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentBlueprintDetail{}, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentBlueprintDetail{}, err
	}
	resource, ok := findBlueprintResource(resources, itemId)
	if !ok {
		return EnvironmentBlueprintDetail{}, fmt.Errorf("未找到蓝图资源")
	}
	if resource.Type != "blueprint" {
		return EnvironmentBlueprintDetail{}, fmt.Errorf("只能查看蓝图文件详情")
	}

	blueprintPath := filepath.Join(libraryRoot, filepath.FromSlash(resource.RelativePath))
	if !fileops.IsSubPath(libraryRoot, blueprintPath) {
		return EnvironmentBlueprintDetail{}, fmt.Errorf("蓝图路径越界")
	}
	content, err := os.ReadFile(blueprintPath)
	if err != nil {
		return EnvironmentBlueprintDetail{}, fmt.Errorf("读取蓝图文件失败: %w", err)
	}
	blueprintCode := string(content)
	parsed, err := environmentHelper.ParseBlueprintCode(blueprintCode)
	if err != nil {
		return EnvironmentBlueprintDetail{}, err
	}

	return EnvironmentBlueprintDetail{
		Item: EnvironmentBlueprintItem{
			ID:           resource.ID,
			ParentID:     resource.ParentID,
			Type:         resource.Type,
			Name:         resource.Name,
			State:        resource.State,
			SizeBytes:    resource.SizeBytes,
			UpdatedAt:    resource.UpdatedAt,
			RelativePath: resource.RelativePath,
			Extension:    resource.Extension,
		},
		LibraryPath: filepath.ToSlash(filepath.Join("blueprints", filepath.FromSlash(resource.RelativePath))),
		Code:        blueprintCode,
		Parsed:      parsed,
	}, nil
}

// ListSaves 返回真实存档目录列表。
// 职责：扫描 SaveRoot 并返回当前存档文件摘要。
// 使用：环境页存档标签初始化、搜索和刷新时调用。
// 边界：目录未定位时返回空列表；不会复制、删除或重命名存档。
func (service *EnvironmentManagerService) ListSaves(query string) ([]EnvironmentSaveItem, error) {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return []EnvironmentSaveItem{}, nil
	}
	resources, err := environmentHelper.ScanSaves(saveRoot.Path)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]EnvironmentSaveItem, 0, len(resources))
	for _, resource := range resources {
		if query != "" && !strings.Contains(strings.ToLower(resource.Name), query) {
			continue
		}
		items = append(items, EnvironmentSaveItem(resource))
	}
	return items, nil
}

// RefreshSaves 重新扫描真实存档目录。
// 职责：提供语义化刷新入口，当前等价于 ListSaves("")。
// 使用：存档标签点击刷新或存档操作完成后调用。
// 边界：第一阶段不写入 environment_save_index；后续可在这里补最近扫描缓存。
func (service *EnvironmentManagerService) RefreshSaves() ([]EnvironmentSaveItem, error) {
	items, err := service.ListSaves("")
	if err != nil {
		return nil, err
	}
	if err := service.indexSaves(items); err != nil {
		return nil, err
	}
	return items, nil
}

// PreviewInitialSync 返回首次同步预览。
// 职责：按 kind 统计真实游戏目录中可纳管资源数量。
// 使用：环境页首次进入或点击“同步本机内容”前展示预览。
// 边界：只读预览，不复制、不写入数据库、不登记同步文件。
func (service *EnvironmentManagerService) PreviewInitialSync(kind string) (EnvironmentInitialSyncPreview, error) {
	resolver := environmentHelper.Resolver{}
	return service.buildInitialSyncPreview(
		normalizePreviewKind(kind),
		resolver.ResolveModRoot(),
		resolver.ResolveBlueprintRoot(),
		resolver.ResolveSaveRoot(),
	), nil
}

// RunInitialSync 执行首次同步纳管。
// 职责：把游戏真实目录中已有 Mod/蓝图复制进客户端资源库并补同步记录，存档只刷新最近索引。
// 使用：环境页用户确认“同步本机内容”后调用。
// 边界：存档不复制；单个资源失败会记录到 Failures 并继续处理其它资源。
// 逻辑流：1. 归一化同步范围 => 2. Mod 复制进资源库并登记 enabled => 3. 蓝图复制进镜像并登记 enabled => 4. 存档刷新索引 => 5. 返回最新概览。
func (service *EnvironmentManagerService) RunInitialSync(request EnvironmentInitialSyncRequest) (EnvironmentInitialSyncResult, error) {
	kind := normalizePreviewKind(request.Kind)
	operationID, err := service.recordEnvironmentOperationStart("initial_sync", kind, "", map[string]interface{}{
		"kind": kind,
	})
	if err != nil {
		return EnvironmentInitialSyncResult{}, err
	}

	result := EnvironmentInitialSyncResult{Kind: kind, Failures: []string{}}
	resolver := environmentHelper.Resolver{}

	if kind == "all" || kind == "mod" {
		synced, failures := service.syncExistingMods(resolver)
		result.SyncedMods = synced
		result.Failures = append(result.Failures, failures...)
	}
	if kind == "all" || kind == "blueprint" {
		synced, failures := service.syncExistingBlueprints(resolver)
		result.SyncedBlueprints = synced
		result.Failures = append(result.Failures, failures...)
	}
	if kind == "all" || kind == "save" {
		saves, err := service.RefreshSaves()
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("存档索引失败: %v", err))
		} else {
			result.IndexedSaves = len(saves)
		}
	}

	overview, err := service.getOverviewWithoutRecovery()
	if err != nil {
		_ = service.finishEnvironmentOperation(operationID, envOperationStatusFailed, err.Error(), operationDetailPayload{
			Plan:    map[string]interface{}{"kind": kind},
			Failure: map[string]interface{}{"message": err.Error()},
		})
		return EnvironmentInitialSyncResult{}, err
	}
	result.Overview = overview
	result.OperationID = operationID
	status := envOperationStatusSuccess
	message := "首次同步已完成"
	if len(result.Failures) > 0 {
		message = "首次同步完成，部分资源失败"
	}
	if err := service.finishEnvironmentOperation(operationID, status, message, operationDetailPayload{
		Plan: map[string]interface{}{"kind": kind},
		Result: map[string]interface{}{
			"syncedMods":       result.SyncedMods,
			"syncedBlueprints": result.SyncedBlueprints,
			"indexedSaves":     result.IndexedSaves,
			"failureCount":     len(result.Failures),
			"failures":         result.Failures,
		},
	}); err != nil {
		return EnvironmentInitialSyncResult{}, err
	}
	return result, nil
}

// EnableMod 启用客户端资源库中的目录型 Mod。
// 职责：创建环境操作记录，并把 AppData/plugins 下的 Mod 镜像同步到游戏 BepInEx/plugins。
// 使用：环境页 Mod 卡片左键启用时调用。
// 边界：目标文件存在且 hash 不同时返回冲突错误，不静默覆盖未知文件。
func (service *EnvironmentManagerService) EnableMod(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("enable", "mod", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.enableModWithoutOperation(itemId)
	})
}

// enableModWithoutOperation 执行 Mod 启用的真实同步逻辑。
// 职责：复制目录、校验目标文件并保存同步记录。
// 使用：EnableMod 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不发 Wails 事件。
func (service *EnvironmentManagerService) enableModWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	modRoot := resolver.ResolveModRoot()
	if modRoot.Path == "" {
		return EnvironmentOperationResult{}, fmt.Errorf("未配置有效的 Mod 目标目录")
	}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	mods, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	var selected *environmentHelper.ModResource
	for index := range mods {
		if mods[index].ID == itemId {
			selected = &mods[index]
			break
		}
	}
	if selected == nil {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到 Mod 资源")
	}
	if !selected.Supported {
		return EnvironmentOperationResult{}, fmt.Errorf("%s", selected.Warning)
	}

	sourcePath := filepath.Join(filepath.Dir(libraryRoot), filepath.FromSlash(selected.LibraryPath))
	targetPath := filepath.Join(modRoot.Path, filepath.Base(sourcePath))
	plans, err := environmentHelper.CopyDirectoryMirrorSafely(sourcePath, targetPath)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if err := service.saveEnabledItemAndFiles(modResourceToEnvItem(*selected, environmentHelper.EnvironmentStateEnabled, "library"), "mod", modRoot.Path, plans); err != nil {
		return EnvironmentOperationResult{}, err
	}

	items, err := service.ListMods("")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	for index := range items {
		if items[index].ID == itemId {
			return EnvironmentOperationResult{Message: "Mod 已启用", Mod: &items[index]}, nil
		}
	}
	return EnvironmentOperationResult{Message: "Mod 已启用"}, nil
}

// DisableMod 禁用客户端资源库中的目录型 Mod。
// 职责：创建环境操作记录，只删除本系统登记且 hash 未变化的目标文件。
// 使用：环境页 Mod 卡片左键禁用时调用。
// 边界：目标文件被外部修改时拒绝删除并返回错误，避免误删用户文件。
func (service *EnvironmentManagerService) DisableMod(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("disable", "mod", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.disableModWithoutOperation(itemId)
	})
}

// UpdateClientMirrorFromGame 用游戏侧文件更新客户端镜像。
// 职责：在用户确认保留游戏侧改动后，把同步记录中的目标文件覆盖回 AppData 镜像并刷新 hash 证据。
// 使用：环境页冲突资源右键“用游戏侧更新客户端镜像”调用。
// 边界：只处理已有同步记录的 Mod/蓝图；任一文件复制或数据库提交失败都会返回错误，不宣称冲突已解决。
func (service *EnvironmentManagerService) UpdateClientMirrorFromGame(itemId string, kind string) (EnvironmentOperationResult, error) {
	normalizedKind, err := normalizeSyncRecordKind(kind)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.runRecordedEnvironmentOperation("update_client_mirror", normalizedKind, itemId, map[string]interface{}{
		"itemId": itemId,
		"kind":   normalizedKind,
	}, func() (EnvironmentOperationResult, error) {
		return service.updateClientMirrorFromGameWithoutOperation(itemId, normalizedKind)
	})
}

// RestoreGameFromClientMirror 用客户端镜像恢复游戏侧文件。
// 职责：在用户确认以客户端镜像为准后，覆盖游戏侧同步目标并刷新同步记录 hash。
// 使用：环境页冲突资源右键“用客户端镜像恢复游戏侧”调用。
// 边界：只处理已有同步记录的 Mod/蓝图；文件被占用或权限不足时返回失败并保留数据库状态。
func (service *EnvironmentManagerService) RestoreGameFromClientMirror(itemId string, kind string) (EnvironmentOperationResult, error) {
	normalizedKind, err := normalizeSyncRecordKind(kind)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.runRecordedEnvironmentOperation("restore_game_target", normalizedKind, itemId, map[string]interface{}{
		"itemId": itemId,
		"kind":   normalizedKind,
	}, func() (EnvironmentOperationResult, error) {
		return service.restoreGameFromClientMirrorWithoutOperation(itemId, normalizedKind)
	})
}

// UnlinkEnvironmentSyncRecord 解除环境资源同步记录。
// 职责：只删除指定资源的同步文件记录并把 EnvItem 标记为 disabled，不删除客户端镜像或游戏真实文件。
// 使用：环境页冲突资源右键“解除同步记录”二次确认后调用。
// 边界：不做文件删除；数据库提交失败时返回错误并保留原记录。
func (service *EnvironmentManagerService) UnlinkEnvironmentSyncRecord(itemId string, kind string) (EnvironmentOperationResult, error) {
	normalizedKind, err := normalizeSyncRecordKind(kind)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.runRecordedEnvironmentOperation("unlink_sync_record", normalizedKind, itemId, map[string]interface{}{
		"itemId": itemId,
		"kind":   normalizedKind,
	}, func() (EnvironmentOperationResult, error) {
		return service.unlinkEnvironmentSyncRecordWithoutOperation(itemId, normalizedKind)
	})
}

// disableModWithoutOperation 执行 Mod 禁用的真实文件删除逻辑。
// 职责：按同步记录安全删除目标文件并刷新列表。
// 使用：DisableMod 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不发 Wails 事件。
func (service *EnvironmentManagerService) disableModWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	if err := service.disableSyncedItem(itemId, "mod"); err != nil {
		return EnvironmentOperationResult{}, err
	}
	items, err := service.ListMods("")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	for index := range items {
		if items[index].ID == itemId {
			return EnvironmentOperationResult{Message: "Mod 已禁用", Mod: &items[index]}, nil
		}
	}
	return EnvironmentOperationResult{Message: "Mod 已禁用"}, nil
}

// ImportMod 导入目录型 Mod 或 zip 包到客户端资源库。
// 职责：创建环境操作记录，并把用户选择的目录或 zip 包复制到 AppData/plugins。
// 使用：环境页 Mod 工具栏导入本地资源时调用。
// 边界：裸 DLL 返回不支持；zip 只识别目录型 Mod，不写入游戏目录。
func (service *EnvironmentManagerService) ImportMod(paths []string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("import", "mod", "", map[string]interface{}{
		"paths": paths,
	}, func() (EnvironmentOperationResult, error) {
		return service.importModWithoutOperation(paths)
	})
}

// importModWithoutOperation 执行 Mod 导入的真实复制逻辑。
// 职责：识别目录或 zip 包并导入客户端资源库。
// 使用：ImportMod 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不写入游戏目录。
func (service *EnvironmentManagerService) importModWithoutOperation(paths []string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	importTmpRoot, err := resolver.LibraryRoot("environment", "import-tmp", fmt.Sprintf("mod-%d", time.Now().UnixNano()))
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	defer os.RemoveAll(importTmpRoot)

	imported := 0
	for _, sourcePath := range paths {
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			continue
		}
		importSource, targetName, cleanup, err := service.prepareModImportSource(sourcePath, importTmpRoot)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		if _, _, err := environmentHelper.CopyDirectoryAsNameSafely(importSource, libraryRoot, targetName); err != nil {
			return EnvironmentOperationResult{}, err
		}
		imported++
	}
	if imported == 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("没有可导入的 Mod 资源")
	}
	if err := service.upsertScannedModItems(libraryRoot, environmentHelper.EnvironmentStateDefault, "import_dir"); err != nil {
		return EnvironmentOperationResult{}, err
	}
	items, err := service.ListMods("")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: fmt.Sprintf("已导入 %d 个 Mod", imported), Mods: items}, nil
}

// DeleteMod 删除客户端 Mod 资源库项目。
// 职责：创建环境操作记录，已启用时先完整禁用，再删除客户端镜像和数据库记录。
// 使用：环境页 Mod 删除操作调用。
// 边界：禁用失败时停止删除，避免同时丢失游戏侧和客户端侧文件。
func (service *EnvironmentManagerService) DeleteMod(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("delete", "mod", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.deleteModWithoutOperation(itemId)
	})
}

// deleteModWithoutOperation 执行 Mod 删除的真实资源库清理逻辑。
// 职责：必要时先禁用，再删除客户端镜像和数据库记录。
// 使用：DeleteMod 的 operation wrapper 调用。
// 边界：不创建 operation 记录；禁用失败直接返回。
func (service *EnvironmentManagerService) deleteModWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	mods, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	var selected *environmentHelper.ModResource
	for index := range mods {
		if mods[index].ID == itemId {
			selected = &mods[index]
			break
		}
	}
	if selected == nil {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到 Mod 资源")
	}
	states, err := service.loadItemStates("mod")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if states[itemId] == environmentHelper.EnvironmentStateEnabled {
		if err := service.disableSyncedItem(itemId, "mod"); err != nil {
			return EnvironmentOperationResult{}, err
		}
	}
	targetPath := filepath.Join(filepath.Dir(libraryRoot), filepath.FromSlash(selected.LibraryPath))
	if err := environmentHelper.RemovePathWithinRootSafely(libraryRoot, targetPath); err != nil {
		return EnvironmentOperationResult{}, err
	}
	if err := service.deleteEnvItemAndSyncedFiles(itemId, "mod"); err != nil {
		return EnvironmentOperationResult{}, err
	}
	items, err := service.ListMods("")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "Mod 资源已删除", Mods: items}, nil
}

// CreateBlueprintFolder 创建客户端蓝图镜像文件夹。
// 职责：创建环境操作记录，并在 AppData/blueprints 下创建真实目录。
// 使用：蓝图标签点击“新建文件夹”时调用。
// 边界：只创建客户端镜像目录，不写入游戏蓝图目录。
func (service *EnvironmentManagerService) CreateBlueprintFolder(parentId string, name string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("create_folder", "blueprint", parentId, map[string]interface{}{
		"parentId": parentId,
		"name":     name,
	}, func() (EnvironmentOperationResult, error) {
		return service.createBlueprintFolderWithoutOperation(parentId, name)
	})
}

// createBlueprintFolderWithoutOperation 执行蓝图文件夹创建逻辑。
// 职责：校验名称并创建客户端镜像目录。
// 使用：CreateBlueprintFolder 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不写入游戏蓝图目录。
func (service *EnvironmentManagerService) createBlueprintFolderWithoutOperation(parentId string, name string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	parentPath, err := service.blueprintRelativePathByID(parentId)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	safeName, err := safeWindowsName(name)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	targetPath := filepath.Join(libraryRoot, filepath.FromSlash(parentPath), safeName)
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return EnvironmentOperationResult{}, fmt.Errorf("创建蓝图文件夹失败: %w", err)
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图文件夹已创建", BlueprintPage: &page}, nil
}

// ImportBlueprintFiles 导入蓝图文件到客户端镜像目录。
// 职责：创建环境操作记录，并把用户选择的蓝图文件复制到当前蓝图文件夹。
// 使用：环境页蓝图工具栏导入文件时调用。
// 边界：目标同名且内容不同会返回冲突；不会写入游戏蓝图目录。
func (service *EnvironmentManagerService) ImportBlueprintFiles(parentId string, paths []string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("import", "blueprint", parentId, map[string]interface{}{
		"parentId": parentId,
		"paths":    paths,
	}, func() (EnvironmentOperationResult, error) {
		return service.importBlueprintFilesWithoutOperation(parentId, paths)
	})
}

// importBlueprintFilesWithoutOperation 执行蓝图文件导入逻辑。
// 职责：过滤扩展名并复制文件到客户端镜像目录。
// 使用：ImportBlueprintFiles 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不写入游戏蓝图目录。
func (service *EnvironmentManagerService) importBlueprintFilesWithoutOperation(parentId string, paths []string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	parentPath, err := service.blueprintRelativePathByID(parentId)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	targetRoot := filepath.Join(libraryRoot, filepath.FromSlash(parentPath))
	filteredPaths := make([]string, 0, len(paths))
	for _, sourcePath := range paths {
		ext := strings.ToLower(filepath.Ext(sourcePath))
		if ext != ".txt" && ext != ".bp" && ext != ".json" {
			return EnvironmentOperationResult{}, fmt.Errorf("不支持的蓝图文件类型: %s", sourcePath)
		}
		filteredPaths = append(filteredPaths, sourcePath)
	}
	if len(filteredPaths) == 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("没有可导入的蓝图文件")
	}
	if _, err := environmentHelper.ImportFilesIntoRootSafely(filteredPaths, targetRoot); err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: fmt.Sprintf("已导入 %d 个蓝图文件", len(filteredPaths)), BlueprintPage: &page}, nil
}

// QuickImportBlueprint 通过蓝图编码创建客户端蓝图文件。
// 职责：创建环境操作记录，并把用户输入的蓝图编码保存为 .txt 文件。
// 使用：环境页蓝图快速导入入口调用。
// 边界：只写入客户端镜像目录，不自动启用到游戏蓝图目录。
func (service *EnvironmentManagerService) QuickImportBlueprint(parentId string, name string, code string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("quick_import", "blueprint", parentId, map[string]interface{}{
		"parentId": parentId,
		"name":     name,
	}, func() (EnvironmentOperationResult, error) {
		return service.quickImportBlueprintWithoutOperation(parentId, name, code)
	})
}

// quickImportBlueprintWithoutOperation 执行蓝图编码快速导入逻辑。
// 职责：校验名称与编码并写入客户端镜像文件。
// 使用：QuickImportBlueprint 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不保存完整蓝图编码到 operation。
func (service *EnvironmentManagerService) quickImportBlueprintWithoutOperation(parentId string, name string, code string) (EnvironmentOperationResult, error) {
	if strings.TrimSpace(code) == "" {
		return EnvironmentOperationResult{}, fmt.Errorf("蓝图编码不能为空")
	}
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	parentPath, err := service.blueprintRelativePathByID(parentId)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	safeName, err := safeWindowsName(name)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	targetRoot := filepath.Join(libraryRoot, filepath.FromSlash(parentPath))
	if _, err := environmentHelper.WriteTextFileIntoRootSafely(targetRoot, safeName+".txt", code); err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图已快速导入", BlueprintPage: &page}, nil
}

// EnableBlueprint 启用客户端资源库中的单个蓝图。
// 职责：创建环境操作记录，并把客户端蓝图文件同步到真实 BlueprintRoot。
// 使用：蓝图卡片左键启用时调用。
// 边界：蓝图目录 needConfirm 时拒绝写入；目标文件冲突时返回错误。
func (service *EnvironmentManagerService) EnableBlueprint(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("enable", "blueprint", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.enableBlueprintWithoutOperation(itemId)
	})
}

// enableBlueprintWithoutOperation 执行蓝图启用的真实同步逻辑。
// 职责：复制单个蓝图文件并保存同步记录。
// 使用：EnableBlueprint 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不处理文件夹批量启用。
func (service *EnvironmentManagerService) enableBlueprintWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	blueprintRoot := resolver.ResolveBlueprintRoot()
	if blueprintRoot.Path == "" || blueprintRoot.NeedConfirm {
		return EnvironmentOperationResult{}, fmt.Errorf("蓝图目录尚未确认")
	}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	blueprints, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	var selected *environmentHelper.BlueprintResource
	for index := range blueprints {
		if blueprints[index].ID == itemId && blueprints[index].Type == "blueprint" {
			selected = &blueprints[index]
			break
		}
	}
	if selected == nil {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到蓝图资源")
	}

	relativePath := filepath.FromSlash(selected.RelativePath)
	sourcePath := filepath.Join(libraryRoot, relativePath)
	targetPath := filepath.Join(blueprintRoot.Path, relativePath)
	plan, err := environmentHelper.CopySingleFileSafely(sourcePath, targetPath, relativePath)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if err := service.saveEnabledItemAndFiles(blueprintResourceToEnvItem(*selected, environmentHelper.EnvironmentStateEnabled, "library"), "blueprint", blueprintRoot.Path, []environmentHelper.SyncedFilePlan{plan}); err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(selected.ParentID, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图已启用", BlueprintPage: &page}, nil
}

// DisableBlueprint 禁用客户端资源库中的单个蓝图。
// 职责：创建环境操作记录，只删除已登记且 hash 未变化的游戏侧蓝图文件。
// 使用：蓝图卡片左键禁用时调用。
// 边界：目标文件被修改时拒绝删除，避免覆盖玩家在游戏侧的改动。
func (service *EnvironmentManagerService) DisableBlueprint(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("disable", "blueprint", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.disableBlueprintWithoutOperation(itemId)
	})
}

// PreviewBlueprintMigration 预览蓝图移动或重命名对游戏侧已启用文件的影响。
// 职责：生成已启用蓝图的新旧真实路径和潜在冲突清单，供前端在迁移前展示确认。
// 使用：环境页拖拽移动、重命名已启用蓝图或包含已启用蓝图的文件夹前调用。
// 边界：只读扫描和同步记录，不移动、不删除、不写数据库；operation 仅支持 move/rename。
func (service *EnvironmentManagerService) PreviewBlueprintMigration(itemId string, operation string, targetParentId string, targetName string) (EnvironmentBlueprintMigrationPreview, error) {
	return service.previewBlueprintMigrationWithoutOperation(itemId, operation, targetParentId, targetName)
}

// ConfirmBlueprintMigration 执行用户确认后的蓝图迁移。
// 职责：重新生成迁移预览，迁移已启用蓝图的游戏侧文件和同步记录，再移动或重命名客户端镜像。
// 使用：环境页蓝图迁移预览弹窗确认后调用。
// 边界：目标冲突、复制失败、旧目标删除失败都会返回错误；旧目标删除失败时资源进入 conflict，不能伪装成功。
func (service *EnvironmentManagerService) ConfirmBlueprintMigration(request EnvironmentBlueprintMigrationRequest) (EnvironmentOperationResult, error) {
	normalizedOperation := strings.ToLower(strings.TrimSpace(request.Operation))
	return service.runRecordedEnvironmentOperation("migrate", "blueprint", request.ItemID, map[string]interface{}{
		"itemId":         request.ItemID,
		"operation":      normalizedOperation,
		"targetParentId": request.TargetParentID,
		"targetName":     request.TargetName,
	}, func() (EnvironmentOperationResult, error) {
		return service.confirmBlueprintMigrationWithoutOperation(request.ItemID, normalizedOperation, request.TargetParentID, request.TargetName)
	})
}

// disableBlueprintWithoutOperation 执行蓝图禁用的真实删除逻辑。
// 职责：按同步记录安全删除目标文件并刷新目录页。
// 使用：DisableBlueprint 的 operation wrapper 调用。
// 边界：不创建 operation 记录。
func (service *EnvironmentManagerService) disableBlueprintWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	parentId, _ := service.blueprintParentIDByID(itemId)
	if err := service.disableSyncedItem(itemId, "blueprint"); err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图已禁用", BlueprintPage: &page}, nil
}

// RenameBlueprintItem 重命名客户端蓝图文件或文件夹。
// 职责：创建环境操作记录，并在客户端镜像资源库中执行安全重命名；已启用蓝图同步迁移游戏侧文件和同步记录。
// 使用：蓝图右键或内联重命名时调用。
// 边界：目标名称冲突、真实目录未确认或旧目标被外部修改时返回错误，不要求用户先禁用。
func (service *EnvironmentManagerService) RenameBlueprintItem(itemId string, name string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("rename", "blueprint", itemId, map[string]interface{}{
		"itemId": itemId,
		"name":   name,
	}, func() (EnvironmentOperationResult, error) {
		return service.confirmBlueprintMigrationWithoutOperation(itemId, "rename", "", name)
	})
}

// renameBlueprintItemWithoutOperation 执行蓝图重命名的真实文件操作。
// 职责：复用蓝图迁移流程重命名客户端镜像节点，并在已启用时同步游戏侧文件和同步记录。
// 使用：RenameBlueprintItem 的 operation wrapper 调用。
// 边界：不创建 operation 记录；同步冲突、真实目录未确认或旧目标被外部修改时返回错误。
func (service *EnvironmentManagerService) renameBlueprintItemWithoutOperation(itemId string, name string) (EnvironmentOperationResult, error) {
	return service.confirmBlueprintMigrationWithoutOperation(itemId, "rename", "", name)
}

// renameBlueprintLibraryNode 重命名客户端蓝图镜像节点。
// 职责：封装蓝图文件或文件夹的名称校验和真实 os.Rename 调用。
// 使用：确认迁移流程在游戏侧文件和同步记录准备完成后调用。
// 边界：只移动客户端镜像，不写同步记录、不删除游戏侧旧目标；目标名称存在时返回错误。
func (service *EnvironmentManagerService) renameBlueprintLibraryNode(itemId string, name string) (string, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return "", err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return "", err
	}
	var selected *environmentHelper.BlueprintResource
	for index := range resources {
		if resources[index].ID == itemId {
			selected = &resources[index]
			break
		}
	}
	if selected == nil {
		return "", fmt.Errorf("未找到蓝图资源")
	}
	safeName, err := safeWindowsName(name)
	if err != nil {
		return "", err
	}
	oldPath := filepath.Join(libraryRoot, filepath.FromSlash(selected.RelativePath))
	newName := safeName
	if selected.Type == "blueprint" {
		newName += selected.Extension
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	if sameCleanPath(oldPath, newPath) {
		return selected.ParentID, nil
	}
	if pathExists(newPath) {
		return "", fmt.Errorf("目标名称已存在")
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("重命名蓝图资源失败: %w", err)
	}
	return selected.ParentID, nil
}

// MoveBlueprintItem 移动客户端蓝图文件或文件夹。
// 职责：创建环境操作记录，并在 AppData/blueprints 镜像目录内移动蓝图节点；已启用蓝图会同步迁移游戏侧文件和同步记录。
// 使用：环境页蓝图拖拽移动或移动命令调用。
// 边界：不弹出确认流程；目标冲突、复制失败、旧目标删除失败或数据库失败时返回错误并阻止本次移动。
func (service *EnvironmentManagerService) MoveBlueprintItem(itemId string, targetParentId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("move", "blueprint", itemId, map[string]interface{}{
		"itemId":         itemId,
		"targetParentId": targetParentId,
	}, func() (EnvironmentOperationResult, error) {
		return service.confirmBlueprintMigrationWithoutOperation(itemId, "move", targetParentId, "")
	})
}

// moveBlueprintItemWithoutOperation 执行蓝图移动的真实文件操作。
// 职责：校验循环移动和启用状态后移动客户端镜像节点。
// 使用：MoveBlueprintItem 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不迁移游戏侧文件。
func (service *EnvironmentManagerService) moveBlueprintItemWithoutOperation(itemId string, targetParentId string) (EnvironmentOperationResult, error) {
	parentId, changed, err := service.moveBlueprintLibraryNode(itemId, targetParentId, false)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if !changed {
		return EnvironmentOperationResult{Message: "蓝图位置未变化", BlueprintPage: &page}, nil
	}
	return EnvironmentOperationResult{Message: "蓝图资源已移动", BlueprintPage: &page}, nil
}

// moveBlueprintLibraryNode 移动客户端蓝图镜像节点。
// 职责：封装蓝图节点移动的目标解析、循环移动校验、启用状态限制和真实路径移动。
// 使用：普通拖拽移动传入 allowEnabledMigration=false，确认迁移流程传入 true。
// 边界：只处理 AppData/blueprints 内的路径，不更新游戏侧文件或数据库同步证据。
func (service *EnvironmentManagerService) moveBlueprintLibraryNode(itemId string, targetParentId string, allowEnabledMigration bool) (string, bool, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return "", false, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return "", false, err
	}
	selected, ok := findBlueprintResource(resources, itemId)
	if !ok {
		return "", false, fmt.Errorf("未找到蓝图资源")
	}
	if selected.ParentID == targetParentId {
		return targetParentId, false, nil
	}
	if selected.Type == "folder" && blueprintFolderContains(resources, selected.ID, targetParentId) {
		return "", false, fmt.Errorf("不能把文件夹移动到自身或子文件夹中")
	}
	states, err := service.loadItemStates("blueprint")
	if err != nil {
		return "", false, err
	}
	if !allowEnabledMigration && selected.Type == "blueprint" && states[itemId] == environmentHelper.EnvironmentStateEnabled {
		return "", false, fmt.Errorf("已启用蓝图需要先禁用后再移动")
	}
	if !allowEnabledMigration && selected.Type == "folder" && blueprintFolderHasEnabledResource(resources, states, selected.ID) {
		return "", false, fmt.Errorf("文件夹内存在已启用蓝图，需要先禁用后再移动")
	}
	targetParentPath, err := service.blueprintRelativePathByID(targetParentId)
	if err != nil {
		return "", false, err
	}
	sourcePath := filepath.Join(libraryRoot, filepath.FromSlash(selected.RelativePath))
	targetPath := filepath.Join(libraryRoot, filepath.FromSlash(targetParentPath), filepath.Base(sourcePath))
	if err := environmentHelper.MovePathWithinRootSafely(libraryRoot, sourcePath, targetPath); err != nil {
		return "", false, err
	}
	return targetParentId, true, nil
}

// DeleteBlueprintItem 删除客户端蓝图文件或文件夹。
// 职责：创建环境操作记录，已启用蓝图先安全禁用，再删除客户端镜像节点。
// 使用：环境页蓝图删除操作调用。
// 边界：文件夹内存在已启用蓝图时逐个禁用，任一失败则停止删除。
func (service *EnvironmentManagerService) DeleteBlueprintItem(itemId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("delete", "blueprint", itemId, map[string]interface{}{
		"itemId": itemId,
	}, func() (EnvironmentOperationResult, error) {
		return service.deleteBlueprintItemWithoutOperation(itemId)
	})
}

// deleteBlueprintItemWithoutOperation 执行蓝图删除的真实清理逻辑。
// 职责：先禁用已同步蓝图，再删除客户端镜像和数据库记录。
// 使用：DeleteBlueprintItem 的 operation wrapper 调用。
// 边界：不创建 operation 记录；任一禁用失败即停止删除。
func (service *EnvironmentManagerService) deleteBlueprintItemWithoutOperation(itemId string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	selected, ok := findBlueprintResource(resources, itemId)
	if !ok {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到蓝图资源")
	}
	parentId := selected.ParentID
	blueprintIDs := blueprintDescendantFileIDs(resources, itemId)
	if selected.Type == "blueprint" {
		blueprintIDs = []string{selected.ID}
	}
	states, err := service.loadItemStates("blueprint")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	for _, blueprintID := range blueprintIDs {
		if states[blueprintID] == environmentHelper.EnvironmentStateEnabled {
			if err := service.disableSyncedItem(blueprintID, "blueprint"); err != nil {
				return EnvironmentOperationResult{}, err
			}
		}
	}
	targetPath := filepath.Join(libraryRoot, filepath.FromSlash(selected.RelativePath))
	if err := environmentHelper.RemovePathWithinRootSafely(libraryRoot, targetPath); err != nil {
		return EnvironmentOperationResult{}, err
	}
	for _, blueprintID := range blueprintIDs {
		if err := service.deleteEnvItemAndSyncedFiles(blueprintID, "blueprint"); err != nil {
			return EnvironmentOperationResult{}, err
		}
	}
	page, err := service.ListBlueprints(parentId, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图资源已删除", BlueprintPage: &page}, nil
}

// ExportBlueprint 导出客户端蓝图文件到指定目录。
// 职责：创建环境操作记录，并把单个蓝图文件从客户端镜像目录复制到用户指定目录。
// 使用：环境页蓝图导出操作调用。
// 边界：第一版只导出单个蓝图文件，不导出文件夹。
func (service *EnvironmentManagerService) ExportBlueprint(itemId string, targetDir string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("export", "blueprint", itemId, map[string]interface{}{
		"itemId":    itemId,
		"targetDir": targetDir,
	}, func() (EnvironmentOperationResult, error) {
		return service.exportBlueprintWithoutOperation(itemId, targetDir)
	})
}

// exportBlueprintWithoutOperation 执行蓝图导出的真实复制逻辑。
// 职责：定位单个客户端蓝图文件并复制到目标目录。
// 使用：ExportBlueprint 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不修改源文件。
func (service *EnvironmentManagerService) exportBlueprintWithoutOperation(itemId string, targetDir string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	selected, ok := findBlueprintResource(resources, itemId)
	if !ok || selected.Type != "blueprint" {
		return EnvironmentOperationResult{}, fmt.Errorf("只能导出单个蓝图文件")
	}
	sourcePath := filepath.Join(libraryRoot, filepath.FromSlash(selected.RelativePath))
	if _, err := environmentHelper.ImportFilesIntoRootSafely([]string{sourcePath}, targetDir); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图已导出"}, nil
}

// RenameSave 重命名真实存档文件。
// 职责：创建环境操作记录，并保留原扩展名重命名 SaveRoot 中的真实文件。
// 使用：存档卡片重命名时调用。
// 边界：不会跨目录移动，不覆盖同名文件，完成后重新扫描真实 SaveRoot。
func (service *EnvironmentManagerService) RenameSave(saveId string, name string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("rename", "save", saveId, map[string]interface{}{
		"saveId": saveId,
		"name":   name,
	}, func() (EnvironmentOperationResult, error) {
		return service.renameSaveWithoutOperation(saveId, name)
	})
}

// renameSaveWithoutOperation 执行存档重命名的真实文件操作。
// 职责：定位当前存档、校验新名称并重命名真实文件。
// 使用：RenameSave 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不跨目录移动。
func (service *EnvironmentManagerService) renameSaveWithoutOperation(saveId string, name string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到存档目录")
	}
	saves, err := environmentHelper.ScanSaves(saveRoot.Path)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	var selected *environmentHelper.SaveResource
	for index := range saves {
		if saves[index].ID == saveId {
			selected = &saves[index]
			break
		}
	}
	if selected == nil {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到存档文件")
	}
	safeName, err := safeWindowsName(name)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	oldPath := filepath.Join(saveRoot.Path, filepath.FromSlash(selected.RelativePath))
	newPath := filepath.Join(filepath.Dir(oldPath), safeName+filepath.Ext(selected.FileName))
	if strings.EqualFold(oldPath, newPath) {
		items, _ := service.ListSaves("")
		return EnvironmentOperationResult{Message: "存档名称未变化", Saves: items}, nil
	}
	if pathExists(newPath) {
		return EnvironmentOperationResult{}, fmt.Errorf("同名存档已存在")
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return EnvironmentOperationResult{}, fmt.Errorf("重命名存档失败: %w", err)
	}
	items, err := service.RefreshSaves()
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "存档已重命名", Saves: items}, nil
}

// ImportSaves 导入存档文件到真实 SaveRoot。
// 职责：创建环境操作记录，并把用户选择的存档文件复制到游戏存档目录。
// 使用：环境页存档导入操作调用。
// 边界：同名且内容不同会返回冲突；不会覆盖已有存档。
func (service *EnvironmentManagerService) ImportSaves(paths []string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("import", "save", "", map[string]interface{}{
		"paths": paths,
	}, func() (EnvironmentOperationResult, error) {
		return service.importSavesWithoutOperation(paths)
	})
}

// importSavesWithoutOperation 执行存档导入的真实复制逻辑。
// 职责：把外部存档文件复制到当前 SaveRoot 并刷新索引。
// 使用：ImportSaves 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不覆盖不同内容同名文件。
func (service *EnvironmentManagerService) importSavesWithoutOperation(paths []string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到存档目录")
	}
	if len(paths) == 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("没有可导入的存档文件")
	}
	if _, err := environmentHelper.ImportFilesIntoRootSafely(paths, saveRoot.Path); err != nil {
		return EnvironmentOperationResult{}, err
	}
	items, err := service.RefreshSaves()
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: fmt.Sprintf("已导入 %d 个存档", len(paths)), Saves: items}, nil
}

// ExportSave 导出真实存档文件到指定目录。
// 职责：创建环境操作记录，按 saveId 定位当前真实存档并复制到用户指定目录。
// 使用：环境页存档导出操作调用。
// 边界：导出失败不修改源存档；目标同名且内容不同返回冲突。
func (service *EnvironmentManagerService) ExportSave(saveId string, targetDir string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("export", "save", saveId, map[string]interface{}{
		"saveId":    saveId,
		"targetDir": targetDir,
	}, func() (EnvironmentOperationResult, error) {
		return service.exportSaveWithoutOperation(saveId, targetDir)
	})
}

// exportSaveWithoutOperation 执行存档导出的真实复制逻辑。
// 职责：定位当前真实存档并复制到用户指定目录。
// 使用：ExportSave 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不修改源存档。
func (service *EnvironmentManagerService) exportSaveWithoutOperation(saveId string, targetDir string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到存档目录")
	}
	selected, err := service.findSaveResource(saveRoot.Path, saveId)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	sourcePath := filepath.Join(saveRoot.Path, filepath.FromSlash(selected.RelativePath))
	if _, err := environmentHelper.ImportFilesIntoRootSafely([]string{sourcePath}, targetDir); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "存档已导出"}, nil
}

// DeleteSave 删除真实存档文件到应用快照目录。
// 职责：创建环境操作记录，把存档移动到 AppData/environment/snapshots/save 下。
// 使用：环境页存档删除操作在用户明确确认后调用。
// 边界：不永久删除；快照冲突或文件不可移动时返回错误并保留原文件。
func (service *EnvironmentManagerService) DeleteSave(saveId string) (EnvironmentOperationResult, error) {
	return service.runRecordedEnvironmentOperation("delete", "save", saveId, map[string]interface{}{
		"saveId": saveId,
	}, func() (EnvironmentOperationResult, error) {
		return service.deleteSaveWithoutOperation(saveId)
	})
}

// deleteSaveWithoutOperation 执行存档快照删除的真实文件操作。
// 职责：把真实存档移动到应用快照目录并刷新列表。
// 使用：DeleteSave 的 operation wrapper 调用。
// 边界：不创建 operation 记录，不永久删除。
func (service *EnvironmentManagerService) deleteSaveWithoutOperation(saveId string) (EnvironmentOperationResult, error) {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到存档目录")
	}
	selected, err := service.findSaveResource(saveRoot.Path, saveId)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	snapshotRoot, err := resolver.LibraryRoot("environment", "snapshots", "save", fmt.Sprintf("%d", time.Now().UnixNano()))
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	sourcePath := filepath.Join(saveRoot.Path, filepath.FromSlash(selected.RelativePath))
	if _, err := environmentHelper.MoveFileToSnapshotSafely(sourcePath, snapshotRoot, selected.RelativePath); err != nil {
		return EnvironmentOperationResult{}, err
	}
	items, err := service.RefreshSaves()
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "存档已移入应用快照", Saves: items}, nil
}

// updateClientMirrorFromGameWithoutOperation 执行用游戏侧更新客户端镜像的真实文件操作。
// 职责：把同步记录里的游戏侧目标文件重新复制回客户端镜像目录，并用新 hash 覆盖数据库证据。
// 使用：UpdateClientMirrorFromGame 的 operation wrapper 调用。
// 边界：不创建 operation 记录；只支持有同步记录的 Mod 和蓝图文件。
func (service *EnvironmentManagerService) updateClientMirrorFromGameWithoutOperation(itemId string, kind string) (EnvironmentOperationResult, error) {
	records, err := service.loadSyncedFilesByItem(itemId, kind)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if len(records) == 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到同步记录")
	}
	for _, record := range records {
		if !utils.FileExists(record.TargetPath) {
			return EnvironmentOperationResult{}, fmt.Errorf("目标文件不存在，无法回写客户端镜像: %s", record.TargetPath)
		}
		plan, err := environmentHelper.OverwriteSingleFileWithEvidence(record.TargetPath, record.SourcePath, record.RelativePath)
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		record.SHA256 = plan.SHA256
		record.FileSize = plan.FileSize
		if err := service.replaceSyncedFileRecord(record); err != nil {
			return EnvironmentOperationResult{}, err
		}
	}
	if err := service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateEnabled); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.loadConflictOperationResult(itemId, kind, "客户端镜像已用游戏侧更新")
}

// restoreGameFromClientMirrorWithoutOperation 执行用客户端镜像恢复游戏侧的真实文件操作。
// 职责：把客户端镜像覆盖回游戏侧目标文件，并刷新同步记录证据。
// 使用：RestoreGameFromClientMirror 的 operation wrapper 调用。
// 边界：不创建 operation 记录；恢复失败时保留原文件状态并返回错误。
func (service *EnvironmentManagerService) restoreGameFromClientMirrorWithoutOperation(itemId string, kind string) (EnvironmentOperationResult, error) {
	records, err := service.loadSyncedFilesByItem(itemId, kind)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if len(records) == 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("未找到同步记录")
	}
	for _, record := range records {
		if !utils.FileExists(record.SourcePath) {
			return EnvironmentOperationResult{}, fmt.Errorf("客户端镜像不存在，无法恢复游戏侧: %s", record.SourcePath)
		}
		plan, err := environmentHelper.OverwriteSingleFileWithEvidence(record.SourcePath, record.TargetPath, record.RelativePath)
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		record.SHA256 = plan.SHA256
		record.FileSize = plan.FileSize
		if err := service.replaceSyncedFileRecord(record); err != nil {
			return EnvironmentOperationResult{}, err
		}
	}
	if err := service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateEnabled); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.loadConflictOperationResult(itemId, kind, "游戏侧已用客户端镜像恢复")
}

// unlinkEnvironmentSyncRecordWithoutOperation 执行解除同步记录的真实数据库清理逻辑。
// 职责：删除同步文件记录并把资源状态降为 disabled，保留真实文件供用户手动处理。
// 使用：UnlinkEnvironmentSyncRecord 的 operation wrapper 调用。
// 边界：不删除任何真实文件；数据库失败时不应被前端视为成功。
func (service *EnvironmentManagerService) unlinkEnvironmentSyncRecordWithoutOperation(itemId string, kind string) (EnvironmentOperationResult, error) {
	if err := service.deleteEnvSyncedFiles(itemId, kind); err != nil {
		return EnvironmentOperationResult{}, err
	}
	if err := service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateDisabled); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return service.loadConflictOperationResult(itemId, kind, "同步记录已解除")
}

// loadSyncedFilesByItem 读取指定资源的同步文件记录。
// 职责：为冲突恢复和解除同步记录复用记录读取逻辑。
// 使用：UpdateClientMirrorFromGame、RestoreGameFromClientMirror 和 UnlinkEnvironmentSyncRecord 调用。
// 边界：数据库未初始化时返回错误；不修改任何状态。
func (service *EnvironmentManagerService) loadSyncedFilesByItem(itemId string, kind string) ([]environmentModel.EnvSyncedFile, error) {
	if core.GLOBAL.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	records := []environmentModel.EnvSyncedFile{}
	if err := core.GLOBAL.DB.Where("item_id = ? AND kind = ?", itemId, kind).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// deleteEnvSyncedFiles 删除指定资源的同步记录。
// 职责：在解除同步或资源删除时清理环境同步证据。
// 使用：UnlinkEnvironmentSyncRecord 和 deleteEnvItemAndSyncedFiles 调用。
// 边界：只删除同步表记录，不改写 EnvItem。
func (service *EnvironmentManagerService) deleteEnvSyncedFiles(itemId string, kind string) error {
	if core.GLOBAL.DB == nil {
		return nil
	}
	return core.GLOBAL.DB.Where("item_id = ? AND kind = ?", itemId, kind).Delete(&environmentModel.EnvSyncedFile{}).Error
}

// replaceSyncedFileRecord 用最新的 hash 和大小替换单条同步记录。
// 职责：在用户选择用某一侧覆盖另一侧后同步刷新证据。
// 使用：冲突恢复流程在文件覆盖成功后调用。
// 边界：只更新记录证据，不决定资源状态。
func (service *EnvironmentManagerService) replaceSyncedFileRecord(record environmentModel.EnvSyncedFile) error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	now := time.Now()
	record.ID = 0
	record.SyncedAt = now
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("item_id = ? AND kind = ? AND target_path = ?", record.ItemID, record.Kind, record.TargetPath).Delete(&environmentModel.EnvSyncedFile{}).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
}

// loadConflictOperationResult 构造冲突处理后的统一返回值。
// 职责：刷新对应资源列表并返回简短成功消息。
// 使用：冲突恢复和解除同步记录成功后调用。
// 边界：只负责消息与列表刷新，不做额外状态修改。
func (service *EnvironmentManagerService) loadConflictOperationResult(itemId string, kind string, message string) (EnvironmentOperationResult, error) {
	switch kind {
	case "mod":
		items, err := service.ListMods("")
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		for index := range items {
			if items[index].ID == itemId {
				return EnvironmentOperationResult{Message: message, Mod: &items[index]}, nil
			}
		}
		return EnvironmentOperationResult{Message: message, Mods: items}, nil
	case "blueprint":
		parentId, _ := service.blueprintParentIDByID(itemId)
		page, err := service.ListBlueprints(parentId, "")
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		return EnvironmentOperationResult{Message: message, BlueprintPage: &page}, nil
	default:
		return EnvironmentOperationResult{}, fmt.Errorf("不支持的同步记录类型: %s", kind)
	}
}

// OpenModLibraryDir 打开客户端 Mod 资源库目录。
// 职责：在系统文件管理器中打开 AppData/plugins。
// 使用：环境页模组工具栏“打开目录”调用。
// 边界：只打开目录；目录不存在时创建空目录以便用户导入资源。
func (service *EnvironmentManagerService) OpenModLibraryDir() error {
	resolver := environmentHelper.Resolver{}
	path, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return err
	}
	return openDirectory(path)
}

// OpenBlueprintLibraryDir 打开客户端蓝图资源库目录。
// 职责：在系统文件管理器中打开 AppData/blueprints。
// 使用：环境页蓝图工具栏“打开目录”调用。
// 边界：只打开客户端镜像目录，不打开游戏真实蓝图目录。
func (service *EnvironmentManagerService) OpenBlueprintLibraryDir() error {
	resolver := environmentHelper.Resolver{}
	path, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return err
	}
	return openDirectory(path)
}

// OpenSaveLibraryDir 打开客户端存档资源库目录。
// 职责：在系统文件管理器中打开环境管理用于保存存档快照的 AppData/environment/snapshots/save。
// 使用：环境页存档右键“打开资源库目录”调用，用于查看被安全移入快照区的存档文件。
// 边界：只创建并打开客户端快照目录，不打开游戏真实存档目录。
func (service *EnvironmentManagerService) OpenSaveLibraryDir() error {
	resolver := environmentHelper.Resolver{}
	path, err := resolver.LibraryRoot("environment", "snapshots", "save")
	if err != nil {
		return err
	}
	return openDirectory(path)
}

// OpenModRealDir 打开游戏真实 Mod 目录。
// 职责：在系统文件管理器中打开当前游戏目录下的 BepInEx/plugins。
// 使用：环境页模组右键“打开真实资源目录”调用，用于查看实际被游戏加载的 Mod 文件。
// 边界：目录不存在时返回错误，不创建 BepInEx 或 plugins 目录，避免误导用户已安装前置框架。
func (service *EnvironmentManagerService) OpenModRealDir() error {
	resolver := environmentHelper.Resolver{}
	modRoot := resolver.ResolveModRoot()
	if modRoot.Path == "" || !modRoot.Exists {
		return fmt.Errorf("未找到真实 Mod 目录")
	}
	return openExistingDirectory(modRoot.Path)
}

// OpenBlueprintRealDir 打开游戏真实蓝图目录。
// 职责：在系统文件管理器中打开当前解析出的真实 BlueprintRoot。
// 使用：环境页蓝图右键“打开真实资源目录”调用，用于查看游戏实际读取的蓝图文件。
// 边界：目录未确认或不存在时返回错误，不创建默认蓝图目录。
func (service *EnvironmentManagerService) OpenBlueprintRealDir() error {
	resolver := environmentHelper.Resolver{}
	blueprintRoot := resolver.ResolveBlueprintRoot()
	if blueprintRoot.Path == "" || !blueprintRoot.Exists {
		return fmt.Errorf("未找到真实蓝图目录")
	}
	return openExistingDirectory(blueprintRoot.Path)
}

// OpenSaveRealDir 打开游戏真实存档目录。
// 职责：在系统文件管理器中打开当前解析出的 SaveRoot。
// 使用：环境页存档右键“打开真实资源目录”调用，用于查看游戏实际存档文件。
// 边界：目录未定位或不存在时返回错误，不创建存档目录。
func (service *EnvironmentManagerService) OpenSaveRealDir() error {
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	if saveRoot.Path == "" || !saveRoot.Exists {
		return fmt.Errorf("未找到存档目录")
	}
	return openExistingDirectory(saveRoot.Path)
}

// OpenSaveDir 兼容旧前端绑定打开真实存档目录。
// 职责：保留旧方法名并委托给 OpenSaveRealDir。
// 使用：旧版本前端或未刷新绑定时调用。
// 边界：不再承载新菜单语义，新代码应调用 OpenSaveLibraryDir 或 OpenSaveRealDir。
func (service *EnvironmentManagerService) OpenSaveDir() error {
	return service.OpenSaveRealDir()
}

// libraryCounts 统计客户端资源库和真实存档目录数量。
// 职责：为概览 DTO 提供轻量数量摘要。
// 使用：GetOverview 调用。
// 边界：扫描失败返回错误；目录不存在视为 0。
func (service *EnvironmentManagerService) libraryCounts(resolver environmentHelper.Resolver, saveRoot environmentHelper.RootResolution) (EnvironmentLibraryCounts, error) {
	modLibraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return EnvironmentLibraryCounts{}, err
	}
	blueprintLibraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentLibraryCounts{}, err
	}
	mods, err := environmentHelper.ScanMods(modLibraryRoot)
	if err != nil {
		return EnvironmentLibraryCounts{}, err
	}
	blueprints, err := environmentHelper.ScanBlueprints(blueprintLibraryRoot)
	if err != nil {
		return EnvironmentLibraryCounts{}, err
	}
	saves := []environmentHelper.SaveResource{}
	if saveRoot.Path != "" && saveRoot.Exists {
		saves, err = environmentHelper.ScanSaves(saveRoot.Path)
		if err != nil {
			return EnvironmentLibraryCounts{}, err
		}
	}

	counts := EnvironmentLibraryCounts{Mods: len(mods), Saves: len(saves)}
	for _, item := range blueprints {
		if item.Type == "folder" {
			counts.BlueprintFolders++
		} else {
			counts.Blueprints++
		}
	}
	return counts, nil
}

// loadItemStates 读取指定类型资源的持久化状态。
// 职责：把 environment_items 中的状态映射到列表扫描结果。
// 使用：ListMods 和 ListBlueprints 调用。
// 边界：数据库未初始化时返回错误；缺失记录由扫描结果保持 default。
func (service *EnvironmentManagerService) loadItemStates(kind string) (map[string]string, error) {
	items := []environmentModel.EnvItem{}
	if core.GLOBAL.DB == nil {
		return map[string]string{}, nil
	}
	if err := core.GLOBAL.DB.Where("kind = ?", kind).Find(&items).Error; err != nil {
		return nil, err
	}
	states := make(map[string]string, len(items))
	for _, item := range items {
		states[item.ID] = item.State
	}
	return states, nil
}

// upsertScannedModItems 写入已扫描 Mod 的默认资源记录。
// 职责：导入 Mod 后把资源库扫描结果保存到 EnvItem，便于关闭应用后恢复状态。
// 使用：ImportMod 调用。
// 边界：只保存支持的目录型 Mod；已有资源只刷新元数据不覆盖启用状态，不创建同步文件记录，不写入游戏目录。
func (service *EnvironmentManagerService) upsertScannedModItems(libraryRoot string, state string, sourceType string) error {
	if core.GLOBAL.DB == nil {
		return nil
	}
	mods, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return err
	}
	now := time.Now()
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		for _, mod := range mods {
			if !mod.Supported {
				continue
			}
			item := modResourceToEnvItem(mod, state, sourceType)
			existing := environmentModel.EnvItem{}
			err := tx.Where("id = ? AND kind = ?", item.ID, item.Kind).First(&existing).Error
			if err == nil {
				if err := tx.Model(&existing).Updates(map[string]interface{}{
					"name":          item.Name,
					"source_type":   item.SourceType,
					"library_path":  item.LibraryPath,
					"size_bytes":    item.SizeBytes,
					"metadata_json": item.MetadataJSON,
					"updated_at":    now,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			item.CreatedAt = now
			item.UpdatedAt = now
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// deleteEnvItemAndSyncedFiles 删除资源状态和同步记录。
// 职责：在客户端镜像删除后清理环境管理数据库中的资源痕迹。
// 使用：DeleteMod 和 DeleteBlueprintItem 调用。
// 边界：只清理指定 kind 的记录，不删除真实文件。
func (service *EnvironmentManagerService) deleteEnvItemAndSyncedFiles(itemId string, kind string) error {
	if core.GLOBAL.DB == nil {
		return nil
	}
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("item_id = ? AND kind = ?", itemId, kind).Delete(&environmentModel.EnvSyncedFile{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND kind = ?", itemId, kind).Delete(&environmentModel.EnvItem{}).Error
	})
}

// saveEnabledItemAndFiles 保存启用资源和同步文件清单。
// 职责：在同一事务中更新 EnvItem 状态并替换该资源的同步文件记录。
// 使用：EnableMod、EnableBlueprint 和首次纳管流程调用。
// 边界：数据库提交失败时返回错误；调用方已完成的文件复制不会在这里回滚。
func (service *EnvironmentManagerService) saveEnabledItemAndFiles(item environmentModel.EnvItem, kind string, targetRoot string, plans []environmentHelper.SyncedFilePlan) error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	now := time.Now()
	item.UpdatedAt = now
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}

	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Where("item_id = ? AND kind = ?", item.ID, kind).Delete(&environmentModel.EnvSyncedFile{}).Error; err != nil {
			return err
		}
		records := make([]environmentModel.EnvSyncedFile, 0, len(plans))
		for _, plan := range plans {
			records = append(records, environmentModel.EnvSyncedFile{
				ItemID:         item.ID,
				Kind:           kind,
				SourcePath:     plan.SourcePath,
				TargetPath:     plan.TargetPath,
				TargetRootPath: targetRoot,
				RelativePath:   plan.RelativePath,
				SHA256:         plan.SHA256,
				FileSize:       plan.FileSize,
				SyncedAt:       now,
			})
		}
		if len(records) > 0 {
			if err := tx.Create(&records).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// disableSyncedItem 禁用已同步资源。
// 职责：根据同步记录安全删除目标文件、清理空目录并更新资源状态。
// 使用：DisableMod 和 DisableBlueprint 调用。
// 边界：无同步记录时只更新状态；hash 不匹配时返回错误并不删除记录。
func (service *EnvironmentManagerService) disableSyncedItem(itemId string, kind string) error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	records := []environmentModel.EnvSyncedFile{}
	if err := core.GLOBAL.DB.Where("item_id = ? AND kind = ?", itemId, kind).Find(&records).Error; err != nil {
		return err
	}
	currentRoot := service.currentTargetRootForKind(kind)
	if err := service.validateSyncedItemDisablePlan(itemId, kind, currentRoot, records); err != nil {
		return err
	}
	for _, record := range records {
		if err := environmentHelper.RemoveFileIfHashMatches(record.TargetPath, record.SHA256); err != nil {
			_ = service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateConflict)
			return err
		}
		if record.TargetRootPath != "" {
			if err := environmentHelper.RemoveEmptyParents(record.TargetRootPath, filepath.Dir(record.TargetPath)); err != nil {
				return err
			}
		}
	}
	now := time.Now()
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("item_id = ? AND kind = ?", itemId, kind).Delete(&environmentModel.EnvSyncedFile{}).Error; err != nil {
			return err
		}
		update := map[string]interface{}{
			"state":      environmentHelper.EnvironmentStateDisabled,
			"updated_at": now,
		}
		return tx.Model(&environmentModel.EnvItem{}).Where("id = ? AND kind = ?", itemId, kind).Updates(update).Error
	})
}

// validateSyncedItemDisablePlan 预检同步资源禁用计划。
// 职责：在真实删除前完整检查根目录、路径边界和 hash 证据，避免多文件资源删除到一半才发现冲突。
// 使用：disableSyncedItem 在执行 RemoveFileIfHashMatches 前调用。
// 边界：只读文件系统并在冲突时标记 EnvItem.State；不删除文件、不清理同步记录、不提交禁用状态。
func (service *EnvironmentManagerService) validateSyncedItemDisablePlan(itemId string, kind string, currentRoot string, records []environmentModel.EnvSyncedFile) error {
	for _, record := range records {
		if currentRoot == "" || !sameCleanPath(record.TargetRootPath, currentRoot) || !fileops.IsSubPath(currentRoot, record.TargetPath) {
			_ = service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateConflict)
			return fmt.Errorf("目标根目录已变化，拒绝按旧路径删除: %s", record.TargetPath)
		}
		if !utils.FileExists(record.TargetPath) {
			continue
		}
		if strings.TrimSpace(record.SHA256) == "" {
			_ = service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateConflict)
			return fmt.Errorf("缺少同步 hash，拒绝删除: %s", record.TargetPath)
		}
		currentSHA256, _, err := environmentHelper.HashFile(record.TargetPath)
		if err != nil {
			_ = service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateConflict)
			return err
		}
		if currentSHA256 != record.SHA256 {
			_ = service.markEnvItemState(itemId, kind, environmentHelper.EnvironmentStateConflict)
			return fmt.Errorf("目标文件已被修改，拒绝删除: %s", record.TargetPath)
		}
	}
	return nil
}

// prepareModImportSource 准备 Mod 导入源目录。
// 职责：识别目录、zip 包和不支持文件，并返回可复制的目录和资源库目录名。
// 使用：ImportMod 遍历用户路径时调用。
// 边界：裸 DLL 或无法识别 zip 内容时返回错误；清理函数由调用方执行。
func (service *EnvironmentManagerService) prepareModImportSource(sourcePath string, importTmpRoot string) (string, string, func(), error) {
	sourcePath = strings.TrimSpace(sourcePath)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", "", nil, fmt.Errorf("读取导入路径失败: %w", err)
	}
	if info.IsDir() {
		return sourcePath, filepath.Base(sourcePath), nil, nil
	}
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == ".dll" {
		return "", "", nil, fmt.Errorf("第一版暂不支持裸 DLL Mod，请导入目录或 zip 包")
	}
	if ext != ".zip" {
		return "", "", nil, fmt.Errorf("不支持的 Mod 导入类型: %s", sourcePath)
	}
	extractRoot := filepath.Join(importTmpRoot, strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)))
	if err := os.RemoveAll(extractRoot); err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(extractRoot, 0755); err != nil {
		return "", "", nil, fmt.Errorf("创建导入临时目录失败: %w", err)
	}
	if err := environmentHelper.ExtractZipToDirectorySafely(sourcePath, extractRoot); err != nil {
		return "", "", nil, err
	}
	modRoot, targetName, err := discoverModRootFromExtractedZip(extractRoot, sourcePath)
	cleanup := func() {
		_ = os.RemoveAll(extractRoot)
	}
	return modRoot, targetName, cleanup, err
}

// syncExistingMods 纳管游戏侧已有 Mod。
// 职责：把游戏 BepInEx/plugins 中的一级目录复制进客户端资源库并登记为 enabled。
// 使用：RunInitialSync 处理 mod/all 范围时调用。
// 边界：只处理目录型 Mod；单个目录失败会记录失败并继续其它目录。
func (service *EnvironmentManagerService) syncExistingMods(resolver environmentHelper.Resolver) (int, []string) {
	modRoot := resolver.ResolveModRoot()
	if modRoot.Path == "" || !modRoot.Exists {
		return 0, []string{}
	}
	libraryRoot, err := resolver.LibraryRoot("plugins")
	if err != nil {
		return 0, []string{err.Error()}
	}
	entries, err := os.ReadDir(modRoot.Path)
	if err != nil {
		return 0, []string{fmt.Sprintf("扫描 Mod 目录失败: %v", err)}
	}
	synced := 0
	failures := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(modRoot.Path, entry.Name())
		targetPath := filepath.Join(libraryRoot, entry.Name())
		wasManaged := pathExists(targetPath)
		plans, err := environmentHelper.CopyDirectoryMirrorSafely(sourcePath, targetPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		mods, err := environmentHelper.ScanMods(libraryRoot)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		for _, mod := range mods {
			if filepath.Base(filepath.FromSlash(mod.LibraryPath)) != entry.Name() {
				continue
			}
			item := modResourceToEnvItem(mod, environmentHelper.EnvironmentStateEnabled, "detected")
			if err := service.saveEnabledItemAndFiles(item, "mod", modRoot.Path, retargetPlans(plans, targetPath, sourcePath)); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
				continue
			}
			if !wasManaged {
				synced++
			}
			break
		}
	}
	return synced, failures
}

// syncExistingBlueprints 纳管真实蓝图目录中的蓝图文件。
// 职责：把真实 BlueprintRoot 中的 .txt/.bp 文件复制进客户端镜像目录并登记为 enabled。
// 使用：RunInitialSync 处理 blueprint/all 范围时调用。
// 边界：未知扩展不自动纳管；单个文件失败会记录失败并继续其它文件。
func (service *EnvironmentManagerService) syncExistingBlueprints(resolver environmentHelper.Resolver) (int, []string) {
	blueprintRoot := resolver.ResolveBlueprintRoot()
	if blueprintRoot.Path == "" || !blueprintRoot.Exists || blueprintRoot.NeedConfirm {
		return 0, []string{}
	}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return 0, []string{err.Error()}
	}
	synced := 0
	failures := []string{}
	_ = filepath.WalkDir(blueprintRoot.Path, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".bp" {
			return nil
		}
		relativePath, err := filepath.Rel(blueprintRoot.Path, sourcePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
			return nil
		}
		targetPath := filepath.Join(libraryRoot, relativePath)
		wasManaged := pathExists(targetPath)
		plan, err := environmentHelper.CopySingleFileSafely(sourcePath, targetPath, relativePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
			return nil
		}
		blueprints, err := environmentHelper.ScanBlueprints(libraryRoot)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
			return nil
		}
		relativeSlash := filepath.ToSlash(filepath.Clean(relativePath))
		for _, blueprint := range blueprints {
			if blueprint.RelativePath != relativeSlash {
				continue
			}
			item := blueprintResourceToEnvItem(blueprint, environmentHelper.EnvironmentStateEnabled, "detected")
			if err := service.saveEnabledItemAndFiles(item, "blueprint", blueprintRoot.Path, []environmentHelper.SyncedFilePlan{retargetPlan(plan, targetPath, sourcePath)}); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", entry.Name(), err))
				return nil
			}
			if !wasManaged {
				synced++
			}
			break
		}
		return nil
	})
	return synced, failures
}

// indexSaves 写入存档最近扫描索引。
// 职责：把真实存档扫描结果保存到 environment_save_index 作为最近缓存。
// 使用：RefreshSaves 和 RunInitialSync 调用。
// 边界：索引不是存档权威；操作前仍需实时扫描。
func (service *EnvironmentManagerService) indexSaves(items []EnvironmentSaveItem) error {
	if core.GLOBAL.DB == nil {
		return nil
	}
	resolver := environmentHelper.Resolver{}
	saveRoot := resolver.ResolveSaveRoot()
	now := time.Now()
	records := make([]environmentModel.EnvSaveIndex, 0, len(items))
	for _, item := range items {
		records = append(records, environmentModel.EnvSaveIndex{
			ID:             item.ID,
			Name:           item.Name,
			FilePath:       filepath.Join(saveRoot.Path, filepath.FromSlash(item.RelativePath)),
			RelativePath:   item.RelativePath,
			FileSize:       item.SizeBytes,
			LastModifiedAt: item.LastModifiedAt,
			LastSeenAt:     now,
			Status:         item.Status,
		})
	}
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		for _, record := range records {
			if err := tx.Save(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// blueprintRelativePathByID 根据蓝图文件夹 ID 解析相对路径。
// 职责：将前端目录节点 ID 转回客户端镜像相对路径。
// 使用：CreateBlueprintFolder 调用。
// 边界：根目录 ID 为空时返回空路径；找不到节点返回错误。
func (service *EnvironmentManagerService) blueprintRelativePathByID(folderId string) (string, error) {
	if strings.TrimSpace(folderId) == "" {
		return "", nil
	}
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return "", err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return "", err
	}
	for _, resource := range resources {
		if resource.ID == folderId && resource.Type == "folder" {
			return resource.RelativePath, nil
		}
	}
	return "", fmt.Errorf("未找到蓝图文件夹")
}

// blueprintParentIDByID 根据蓝图 ID 返回父文件夹 ID。
// 职责：禁用或重命名后刷新原目录。
// 使用：DisableBlueprint 调用。
// 边界：找不到节点时返回空父节点和错误。
func (service *EnvironmentManagerService) blueprintParentIDByID(itemId string) (string, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return "", err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return "", err
	}
	for _, resource := range resources {
		if resource.ID == itemId {
			return resource.ParentID, nil
		}
	}
	return "", fmt.Errorf("未找到蓝图资源")
}

// previewBlueprintMigrationWithoutOperation 执行蓝图迁移预览计算。
// 职责：基于当前客户端镜像路径、目标参数和同步记录推导游戏侧新旧目标路径。
// 使用：PreviewBlueprintMigration 调用。
// 边界：不创建 operation 记录，不修改文件或数据库；目标冲突仅作为预览证据返回。
func (service *EnvironmentManagerService) previewBlueprintMigrationWithoutOperation(itemId string, operation string, targetParentId string, targetName string) (EnvironmentBlueprintMigrationPreview, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentBlueprintMigrationPreview{}, err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return EnvironmentBlueprintMigrationPreview{}, err
	}
	selected, ok := findBlueprintResource(resources, itemId)
	if !ok {
		return EnvironmentBlueprintMigrationPreview{}, fmt.Errorf("未找到蓝图资源")
	}
	normalizedOperation := strings.ToLower(strings.TrimSpace(operation))
	preview := EnvironmentBlueprintMigrationPreview{
		ItemID:         itemId,
		Operation:      normalizedOperation,
		TargetParentID: targetParentId,
		TargetName:     targetName,
		Entries:        []EnvironmentBlueprintMigrationEntry{},
	}
	states, err := service.loadItemStates("blueprint")
	if err != nil {
		return EnvironmentBlueprintMigrationPreview{}, err
	}
	newBaseRelativePath, err := service.resolveBlueprintMigrationBasePath(resources, selected, normalizedOperation, targetParentId, targetName)
	if err != nil {
		return EnvironmentBlueprintMigrationPreview{}, err
	}
	affectedResources := blueprintMigrationAffectedResources(resources, selected, states)
	if len(affectedResources) == 0 {
		return preview, nil
	}
	blueprintRoot := resolver.ResolveBlueprintRoot()
	if blueprintRoot.Path == "" || blueprintRoot.NeedConfirm {
		return EnvironmentBlueprintMigrationPreview{}, fmt.Errorf("蓝图目录尚未确认")
	}
	for _, blueprintResource := range affectedResources {
		oldRelativePath := filepath.ToSlash(filepath.Clean(blueprintResource.RelativePath))
		newRelativePath := remapBlueprintRelativePath(selected, oldRelativePath, newBaseRelativePath)
		if oldRelativePath == newRelativePath {
			continue
		}
		entry := EnvironmentBlueprintMigrationEntry{
			ItemID:          blueprintResource.ID,
			Name:            blueprintResource.Name,
			OldRelativePath: oldRelativePath,
			NewRelativePath: newRelativePath,
			OldTargetPath:   filepath.Join(blueprintRoot.Path, filepath.FromSlash(oldRelativePath)),
			NewTargetPath:   filepath.Join(blueprintRoot.Path, filepath.FromSlash(newRelativePath)),
		}
		if entry.OldRelativePath != entry.NewRelativePath && pathExists(entry.NewTargetPath) {
			entry.Conflict = true
			entry.ConflictReason = "目标路径已存在"
			preview.Conflicts++
		}
		preview.Entries = append(preview.Entries, entry)
	}
	preview.EnabledBlueprints = len(preview.Entries)
	preview.RequiresConfirmation = preview.EnabledBlueprints > 0
	return preview, nil
}

// confirmBlueprintMigrationWithoutOperation 执行蓝图迁移的文件与数据库提交。
// 职责：按预览条目复制新游戏侧文件、刷新同步记录、移动客户端镜像，并安全删除旧游戏侧文件。
// 使用：ConfirmBlueprintMigration 的 operation wrapper 调用。
// 边界：不创建 operation 记录；失败返回错误，旧文件删除失败时标记 conflict。
func (service *EnvironmentManagerService) confirmBlueprintMigrationWithoutOperation(itemId string, operation string, targetParentId string, targetName string) (EnvironmentOperationResult, error) {
	preview, err := service.previewBlueprintMigrationWithoutOperation(itemId, operation, targetParentId, targetName)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if preview.Conflicts > 0 {
		return EnvironmentOperationResult{}, fmt.Errorf("迁移目标存在冲突，请处理后重试")
	}
	if len(preview.Entries) == 0 {
		return service.applyBlueprintLibraryMigration(itemId, operation, targetParentId, targetName)
	}
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	blueprintRoot := resolver.ResolveBlueprintRoot()
	if blueprintRoot.Path == "" || blueprintRoot.NeedConfirm {
		return EnvironmentOperationResult{}, fmt.Errorf("蓝图目录尚未确认")
	}
	commitEntries := make([]blueprintMigrationCommitEntry, 0, len(preview.Entries))
	for _, entry := range preview.Entries {
		oldRecord, err := service.loadSyncedBlueprintRecord(entry.ItemID, entry.OldTargetPath)
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		if oldRecord.SHA256 == "" {
			_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateConflict)
			return EnvironmentOperationResult{}, fmt.Errorf("缺少旧同步 hash，拒绝迁移: %s", entry.OldTargetPath)
		}
		oldHash, _, err := environmentHelper.HashFileWhenVisible(entry.OldTargetPath)
		if err != nil {
			if os.IsNotExist(err) {
				_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateMissing)
				return EnvironmentOperationResult{}, fmt.Errorf("旧游戏侧蓝图不存在，已标记为不可用，请复核状态后重新启用或解除同步记录: %s", entry.OldTargetPath)
			}
			_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateConflict)
			return EnvironmentOperationResult{}, fmt.Errorf("读取旧游戏侧蓝图失败，拒绝迁移: %w", err)
		}
		if oldHash != oldRecord.SHA256 {
			_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateConflict)
			return EnvironmentOperationResult{}, fmt.Errorf("旧游戏侧蓝图已被修改，拒绝迁移: %s", entry.OldTargetPath)
		}
		sourcePath := filepath.Join(libraryRoot, filepath.FromSlash(entry.OldRelativePath))
		plan, err := environmentHelper.CopySingleFileSafely(sourcePath, entry.NewTargetPath, entry.NewRelativePath)
		if err != nil {
			return EnvironmentOperationResult{}, err
		}
		commitEntries = append(commitEntries, blueprintMigrationCommitEntry{
			PreviewEntry: entry,
			OldRecord:    oldRecord,
			NewPlan:      plan,
		})
	}
	migrationResult, err := service.applyBlueprintLibraryMigrationNode(itemId, operation, targetParentId, targetName)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if err := service.attachBlueprintMigrationNewResources(libraryRoot, commitEntries); err != nil {
		if migrationResult.Changed {
			_ = os.Rename(migrationResult.NewPath, migrationResult.OldPath)
		}
		return EnvironmentOperationResult{}, err
	}
	if err := service.commitBlueprintMigrationRecords(commitEntries, blueprintRoot.Path); err != nil {
		if migrationResult.Changed {
			_ = os.Rename(migrationResult.NewPath, migrationResult.OldPath)
		}
		return EnvironmentOperationResult{}, err
	}
	for _, entry := range preview.Entries {
		oldRecord := environmentModel.EnvSyncedFile{}
		for _, commitEntry := range commitEntries {
			if commitEntry.PreviewEntry.ItemID == entry.ItemID && sameCleanPath(commitEntry.PreviewEntry.OldTargetPath, entry.OldTargetPath) {
				oldRecord = commitEntry.OldRecord
				break
			}
		}
		if err := environmentHelper.RemoveFileIfHashMatches(entry.OldTargetPath, oldRecord.SHA256); err != nil {
			_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateConflict)
			return EnvironmentOperationResult{}, fmt.Errorf("迁移已写入新目标，但清理旧目标失败: %w", err)
		}
		if err := environmentHelper.RemoveEmptyParents(blueprintRoot.Path, filepath.Dir(entry.OldTargetPath)); err != nil {
			_ = service.markEnvItemState(entry.ItemID, "blueprint", environmentHelper.EnvironmentStateConflict)
			return EnvironmentOperationResult{}, fmt.Errorf("迁移已写入新目标，但清理旧目录失败: %w", err)
		}
	}
	page, err := service.ListBlueprints(migrationResult.RefreshParentID, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	return EnvironmentOperationResult{Message: "蓝图迁移已完成", BlueprintPage: &page}, nil
}

// applyBlueprintLibraryMigration 执行客户端蓝图镜像移动或重命名。
// 职责：在游戏侧新目标和同步记录准备完成后，移动 AppData/blueprints 下的真实节点。
// 使用：confirmBlueprintMigrationWithoutOperation 调用。
// 边界：只处理客户端镜像；循环移动和命名冲突仍由既有无 operation 方法校验。
func (service *EnvironmentManagerService) applyBlueprintLibraryMigration(itemId string, operation string, targetParentId string, targetName string) (EnvironmentOperationResult, error) {
	result, err := service.applyBlueprintLibraryMigrationNode(itemId, operation, targetParentId, targetName)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	page, err := service.ListBlueprints(result.RefreshParentID, "")
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	if !result.Changed {
		return EnvironmentOperationResult{Message: "蓝图位置未变化", BlueprintPage: &page}, nil
	}
	return EnvironmentOperationResult{Message: "蓝图资源已迁移", BlueprintPage: &page}, nil
}

// applyBlueprintLibraryMigrationNode 执行客户端蓝图镜像节点迁移。
// 职责：允许确认迁移绕过已启用限制，同时保留名称、循环移动和目标冲突校验。
// 使用：confirmBlueprintMigrationWithoutOperation 和无已启用条目的确认迁移调用。
// 边界：只移动客户端镜像，不写数据库；失败时调用方必须保持弹窗确认态。
func (service *EnvironmentManagerService) applyBlueprintLibraryMigrationNode(itemId string, operation string, targetParentId string, targetName string) (blueprintLibraryMigrationResult, error) {
	beforePath, err := service.blueprintLibraryPathByID(itemId)
	if err != nil {
		return blueprintLibraryMigrationResult{}, err
	}
	switch operation {
	case "move":
		targetParentPath, err := service.blueprintRelativePathByID(targetParentId)
		if err != nil {
			return blueprintLibraryMigrationResult{}, err
		}
		afterPath := filepath.Join(filepath.Dir(beforePath), filepath.Base(beforePath))
		if root, rootErr := (environmentHelper.Resolver{}).LibraryRoot("blueprints"); rootErr == nil {
			afterPath = filepath.Join(root, filepath.FromSlash(targetParentPath), filepath.Base(beforePath))
		}
		refreshParentID, changed, err := service.moveBlueprintLibraryNode(itemId, targetParentId, true)
		if err != nil {
			return blueprintLibraryMigrationResult{}, err
		}
		if !changed {
			afterPath = beforePath
		}
		return blueprintLibraryMigrationResult{OldPath: beforePath, NewPath: afterPath, RefreshParentID: refreshParentID, Changed: changed}, nil
	case "rename":
		afterPath, changed := service.renamedBlueprintLibraryPath(beforePath, targetName)
		refreshParentID, err := service.renameBlueprintLibraryNode(itemId, targetName)
		if err != nil {
			return blueprintLibraryMigrationResult{}, err
		}
		return blueprintLibraryMigrationResult{OldPath: beforePath, NewPath: afterPath, RefreshParentID: refreshParentID, Changed: changed}, nil
	default:
		return blueprintLibraryMigrationResult{}, fmt.Errorf("不支持的蓝图迁移操作: %s", operation)
	}
}

// loadSyncedBlueprintRecord 读取迁移前单个蓝图的旧同步记录。
// 职责：按蓝图资源 ID 和旧目标路径定位唯一同步证据。
// 使用：确认迁移执行前复核旧 hash 和旧目标文件状态。
// 边界：数据库未初始化、记录缺失或同一路径多记录都会返回错误，避免迁移误删。
func (service *EnvironmentManagerService) loadSyncedBlueprintRecord(itemId string, oldTargetPath string) (environmentModel.EnvSyncedFile, error) {
	if core.GLOBAL.DB == nil {
		return environmentModel.EnvSyncedFile{}, fmt.Errorf("database is not initialized")
	}
	records := []environmentModel.EnvSyncedFile{}
	if err := core.GLOBAL.DB.Where("item_id = ? AND kind = ?", itemId, "blueprint").Find(&records).Error; err != nil {
		return environmentModel.EnvSyncedFile{}, err
	}
	for _, record := range records {
		if sameCleanPath(record.TargetPath, oldTargetPath) {
			return record, nil
		}
	}
	return environmentModel.EnvSyncedFile{}, fmt.Errorf("未找到旧蓝图同步记录: %s", oldTargetPath)
}

// attachBlueprintMigrationNewResources 绑定迁移后扫描出的蓝图资源。
// 职责：在客户端镜像已移动后，用新相对路径重新扫描资源 ID、父节点和展示信息。
// 使用：确认迁移提交数据库前调用，确保路径派生 ID 被正确迁移。
// 边界：只读取客户端镜像；找不到新文件时返回错误并由调用方尝试回滚镜像移动。
func (service *EnvironmentManagerService) attachBlueprintMigrationNewResources(libraryRoot string, commitEntries []blueprintMigrationCommitEntry) error {
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return err
	}
	resourcesByRelativePath := make(map[string]environmentHelper.BlueprintResource, len(resources))
	for _, resource := range resources {
		resourcesByRelativePath[filepath.ToSlash(filepath.Clean(resource.RelativePath))] = resource
	}
	for index := range commitEntries {
		newRelativePath := filepath.ToSlash(filepath.Clean(commitEntries[index].PreviewEntry.NewRelativePath))
		resource, ok := resourcesByRelativePath[newRelativePath]
		if !ok || resource.Type != "blueprint" {
			return fmt.Errorf("迁移后未找到客户端蓝图镜像: %s", newRelativePath)
		}
		commitEntries[index].NewResource = resource
		commitEntries[index].NewPlan.SourcePath = filepath.Join(libraryRoot, filepath.FromSlash(newRelativePath))
	}
	return nil
}

// commitBlueprintMigrationRecords 提交蓝图迁移后的数据库记录。
// 职责：把旧路径派生的 EnvItem 和同步记录迁移到新路径派生的资源 ID。
// 使用：确认迁移在客户端镜像移动完成后调用。
// 边界：只提交数据库，不删除旧游戏侧文件；失败时调用方决定是否回滚客户端镜像。
func (service *EnvironmentManagerService) commitBlueprintMigrationRecords(commitEntries []blueprintMigrationCommitEntry, targetRoot string) error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	now := time.Now()
	return core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		for _, commitEntry := range commitEntries {
			previousItem := environmentModel.EnvItem{}
			if err := tx.Where("id = ? AND kind = ?", commitEntry.PreviewEntry.ItemID, "blueprint").First(&previousItem).Error; err != nil {
				return err
			}
			if !sameCleanPath(commitEntry.OldRecord.TargetPath, commitEntry.PreviewEntry.OldTargetPath) {
				return fmt.Errorf("旧同步记录目标路径不一致: %s", commitEntry.PreviewEntry.OldTargetPath)
			}
			if !sameCleanPath(commitEntry.NewPlan.TargetPath, commitEntry.PreviewEntry.NewTargetPath) {
				return fmt.Errorf("新同步记录目标路径不一致: %s", commitEntry.PreviewEntry.NewTargetPath)
			}
			newItem := blueprintResourceToEnvItem(commitEntry.NewResource, environmentHelper.EnvironmentStateEnabled, previousItem.SourceType)
			newItem.OriginPath = previousItem.OriginPath
			newItem.MetadataJSON = previousItem.MetadataJSON
			newItem.CreatedAt = previousItem.CreatedAt
			newItem.UpdatedAt = now
			if newItem.CreatedAt.IsZero() {
				newItem.CreatedAt = now
			}
			if previousItem.ID != newItem.ID {
				if err := tx.Where("id = ? AND kind = ?", newItem.ID, "blueprint").Delete(&environmentModel.EnvItem{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(&newItem).Error; err != nil {
				return err
			}
			if previousItem.ID != newItem.ID {
				if err := tx.Where("id = ? AND kind = ?", previousItem.ID, "blueprint").Delete(&environmentModel.EnvItem{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("item_id = ? AND kind = ? AND target_path = ?", commitEntry.OldRecord.ItemID, "blueprint", commitEntry.OldRecord.TargetPath).Delete(&environmentModel.EnvSyncedFile{}).Error; err != nil {
				return err
			}
			newRecord := environmentModel.EnvSyncedFile{
				ItemID:         newItem.ID,
				Kind:           "blueprint",
				SourcePath:     commitEntry.NewPlan.SourcePath,
				TargetPath:     commitEntry.NewPlan.TargetPath,
				TargetRootPath: targetRoot,
				RelativePath:   commitEntry.NewPlan.RelativePath,
				SHA256:         commitEntry.NewPlan.SHA256,
				FileSize:       commitEntry.NewPlan.FileSize,
				SyncedAt:       now,
			}
			if err := tx.Create(&newRecord).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// blueprintLibraryPathByID 解析当前客户端蓝图镜像节点绝对路径。
// 职责：按扫描 ID 定位蓝图或文件夹在 AppData/blueprints 下的真实路径。
// 使用：确认迁移前记录可回滚路径。
// 边界：不返回游戏侧路径；资源缺失时返回错误。
func (service *EnvironmentManagerService) blueprintLibraryPathByID(itemId string) (string, error) {
	resolver := environmentHelper.Resolver{}
	libraryRoot, err := resolver.LibraryRoot("blueprints")
	if err != nil {
		return "", err
	}
	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		return "", err
	}
	resource, ok := findBlueprintResource(resources, itemId)
	if !ok {
		return "", fmt.Errorf("未找到蓝图资源")
	}
	return filepath.Join(libraryRoot, filepath.FromSlash(resource.RelativePath)), nil
}

// renamedBlueprintLibraryPath 计算重命名后的客户端镜像绝对路径。
// 职责：根据旧路径和用户输入名称推导同目录下的新路径。
// 使用：确认迁移的回滚路径记录。
// 边界：调用前重命名函数已完成名称校验；此处失败时保守返回旧路径和未变化。
func (service *EnvironmentManagerService) renamedBlueprintLibraryPath(oldPath string, targetName string) (string, bool) {
	safeName, err := safeWindowsName(targetName)
	if err != nil {
		return oldPath, false
	}
	info, statErr := os.Stat(oldPath)
	if statErr != nil {
		return oldPath, false
	}
	newName := safeName
	if !info.IsDir() {
		extension := filepath.Ext(oldPath)
		newName += extension
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	return newPath, !sameCleanPath(oldPath, newPath)
}

// resolveBlueprintMigrationBasePath 计算蓝图迁移后的根相对路径。
// 职责：按 move 或 rename 参数推导被操作节点的新相对路径。
// 使用：previewBlueprintMigrationWithoutOperation 调用。
// 边界：只计算路径，不移动文件；非法名称和循环移动返回错误。
func (service *EnvironmentManagerService) resolveBlueprintMigrationBasePath(resources []environmentHelper.BlueprintResource, selected environmentHelper.BlueprintResource, operation string, targetParentId string, targetName string) (string, error) {
	switch operation {
	case "move":
		if selected.Type == "folder" && blueprintFolderContains(resources, selected.ID, targetParentId) {
			return "", fmt.Errorf("不能把文件夹移动到自身或子文件夹中")
		}
		targetParentPath, err := service.blueprintRelativePathByID(targetParentId)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.FromSlash(targetParentPath), filepath.Base(filepath.FromSlash(selected.RelativePath))))), nil
	case "rename":
		safeName, err := safeWindowsName(targetName)
		if err != nil {
			return "", err
		}
		newName := safeName
		if selected.Type == "blueprint" {
			newName += selected.Extension
		}
		return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(selected.RelativePath)), newName))), nil
	default:
		return "", fmt.Errorf("不支持的蓝图迁移预览操作: %s", operation)
	}
}

// findSaveResource 在当前扫描列表中定位存档。
// 职责：每次存档写操作前重新扫描 SaveRoot 并查找 saveId。
// 使用：ExportSave 和 DeleteSave 调用。
// 边界：不使用旧索引作为权威，找不到时返回错误。
func (service *EnvironmentManagerService) findSaveResource(saveRoot string, saveId string) (environmentHelper.SaveResource, error) {
	saves, err := environmentHelper.ScanSaves(saveRoot)
	if err != nil {
		return environmentHelper.SaveResource{}, err
	}
	for _, save := range saves {
		if save.ID == saveId {
			return save, nil
		}
	}
	return environmentHelper.SaveResource{}, fmt.Errorf("未找到存档文件")
}

// buildInitialSyncPreview 构建首次同步预览。
// 职责：按目录解析结果统计真实目录中可纳管资源数量并生成提示文案。
// 使用：GetOverview 和 PreviewInitialSync 调用。
// 边界：不执行纳管；目录缺失时对应数量为 0。
func (service *EnvironmentManagerService) buildInitialSyncPreview(kind string, modRoot environmentHelper.RootResolution, blueprintRoot environmentHelper.RootResolution, saveRoot environmentHelper.RootResolution) EnvironmentInitialSyncPreview {
	preview := EnvironmentInitialSyncPreview{Kind: kind}
	if kind == "all" || kind == "mod" {
		preview.Mods = environmentHelper.CountFirstLevelDirs(modRoot.Path)
	}
	if kind == "all" || kind == "blueprint" {
		preview.Blueprints = environmentHelper.CountBlueprintFiles(blueprintRoot.Path)
	}
	if kind == "all" || kind == "save" {
		saves, _ := environmentHelper.ScanSaves(saveRoot.Path)
		preview.Saves = len(saves)
	}
	preview.Message = fmt.Sprintf("发现本机已有 %d 个 Mod、%d 个蓝图、%d 个存档。", preview.Mods, preview.Blueprints, preview.Saves)
	return preview
}

// buildBreadcrumbs 根据当前文件夹 ID 构建面包屑。
// 职责：从当前目录向上回溯父节点并返回根到当前的节点序列。
// 使用：ListBlueprints 返回目录页时调用。
// 边界：节点缺失时停止回溯，避免错误路径阻断列表。
func buildBreadcrumbs(parentId string, byID map[string]environmentHelper.BlueprintResource) []EnvironmentBlueprintBreadcrumb {
	if parentId == "" {
		return []EnvironmentBlueprintBreadcrumb{}
	}

	stack := []EnvironmentBlueprintBreadcrumb{}
	seen := map[string]struct{}{}
	currentID := parentId
	for currentID != "" {
		if _, ok := seen[currentID]; ok {
			break
		}
		seen[currentID] = struct{}{}
		item, ok := byID[currentID]
		if !ok || item.Type != "folder" {
			break
		}
		stack = append(stack, EnvironmentBlueprintBreadcrumb{ID: item.ID, Name: item.Name})
		currentID = item.ParentID
	}

	for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
		stack[i], stack[j] = stack[j], stack[i]
	}
	return stack
}

// findBlueprintResource 从扫描结果中按 ID 定位蓝图节点。
// 职责：为移动、删除和导出操作复用蓝图资源查找。
// 使用：MoveBlueprintItem、DeleteBlueprintItem 和 ExportBlueprint 调用。
// 边界：只查当前扫描结果，不访问数据库。
func findBlueprintResource(resources []environmentHelper.BlueprintResource, itemId string) (environmentHelper.BlueprintResource, bool) {
	for _, resource := range resources {
		if resource.ID == itemId {
			return resource, true
		}
	}
	return environmentHelper.BlueprintResource{}, false
}

// blueprintFolderContains 判断目标文件夹是否位于源文件夹子树内。
// 职责：防止拖拽移动把文件夹移动到自身或后代目录。
// 使用：MoveBlueprintItem 调用。
// 边界：仅基于扫描 ID 和 ParentID 判断，不读取文件系统。
func blueprintFolderContains(resources []environmentHelper.BlueprintResource, sourceFolderId string, targetFolderId string) bool {
	if targetFolderId == "" {
		return false
	}
	current := targetFolderId
	seen := map[string]struct{}{}
	for current != "" {
		if current == sourceFolderId {
			return true
		}
		if _, ok := seen[current]; ok {
			return false
		}
		seen[current] = struct{}{}
		resource, ok := findBlueprintResource(resources, current)
		if !ok {
			return false
		}
		current = resource.ParentID
	}
	return false
}

// blueprintDescendantFileIDs 收集文件夹子树中的蓝图文件 ID。
// 职责：删除文件夹前定位需要先禁用和清理记录的蓝图资源。
// 使用：DeleteBlueprintItem 调用。
// 边界：只返回 type=blueprint 的文件节点，不包含文件夹节点。
func blueprintDescendantFileIDs(resources []environmentHelper.BlueprintResource, folderId string) []string {
	fileIDs := []string{}
	for _, resource := range resources {
		if resource.ParentID != folderId {
			continue
		}
		if resource.Type == "blueprint" {
			fileIDs = append(fileIDs, resource.ID)
			continue
		}
		fileIDs = append(fileIDs, blueprintDescendantFileIDs(resources, resource.ID)...)
	}
	return fileIDs
}

// blueprintFolderHasEnabledResource 判断文件夹子树内是否存在已启用蓝图。
// 职责：在缺少迁移预览时阻止移动已同步到游戏侧的蓝图子树。
// 使用：MoveBlueprintItem 调用。
// 边界：状态来自数据库映射，缺失记录按未启用处理。
func blueprintFolderHasEnabledResource(resources []environmentHelper.BlueprintResource, states map[string]string, folderId string) bool {
	for _, blueprintID := range blueprintDescendantFileIDs(resources, folderId) {
		if states[blueprintID] == environmentHelper.EnvironmentStateEnabled {
			return true
		}
	}
	return false
}

// blueprintMigrationAffectedResources 收集迁移预览中受影响的已启用蓝图。
// 职责：把单个蓝图或文件夹子树内 enabled 蓝图筛选出来。
// 使用：previewBlueprintMigrationWithoutOperation 生成迁移预览条目。
// 边界：只返回蓝图文件，不返回文件夹；状态缺失按未启用处理。
func blueprintMigrationAffectedResources(resources []environmentHelper.BlueprintResource, selected environmentHelper.BlueprintResource, states map[string]string) []environmentHelper.BlueprintResource {
	affectedResources := []environmentHelper.BlueprintResource{}
	if selected.Type == "blueprint" {
		if states[selected.ID] == environmentHelper.EnvironmentStateEnabled {
			return append(affectedResources, selected)
		}
		return affectedResources
	}
	enabledIDs := map[string]struct{}{}
	for _, blueprintID := range blueprintDescendantFileIDs(resources, selected.ID) {
		if states[blueprintID] == environmentHelper.EnvironmentStateEnabled {
			enabledIDs[blueprintID] = struct{}{}
		}
	}
	for _, resource := range resources {
		if _, ok := enabledIDs[resource.ID]; ok && resource.Type == "blueprint" {
			affectedResources = append(affectedResources, resource)
		}
	}
	return affectedResources
}

// remapBlueprintRelativePath 将迁移前子路径映射到迁移后子路径。
// 职责：保留文件夹迁移时的子树相对层级，单文件迁移直接返回新文件路径。
// 使用：previewBlueprintMigrationWithoutOperation 构造新目标路径。
// 边界：输入路径均为 slash 格式；无法求相对路径时回退新根路径。
func remapBlueprintRelativePath(selected environmentHelper.BlueprintResource, oldRelativePath string, newBaseRelativePath string) string {
	if selected.Type == "blueprint" {
		return filepath.ToSlash(filepath.Clean(newBaseRelativePath))
	}
	childRelativePath, err := filepath.Rel(filepath.FromSlash(selected.RelativePath), filepath.FromSlash(oldRelativePath))
	if err != nil {
		return filepath.ToSlash(filepath.Clean(newBaseRelativePath))
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.FromSlash(newBaseRelativePath), childRelativePath)))
}

// discoverModRootFromExtractedZip 从 zip 解压目录中识别目录型 Mod 根。
// 职责：兼容包根包含 plugins 子目录、单个包根目录或根目录即 Mod 文件集的常见结构。
// 使用：prepareModImportSource 调用。
// 边界：不处理 patchers/config；无法识别时返回错误，不创建资源库项目。
func discoverModRootFromExtractedZip(extractRoot string, sourceZipPath string) (string, string, error) {
	pluginsRoot := filepath.Join(extractRoot, "plugins")
	if isExistingDirectory(pluginsRoot) {
		entries, err := os.ReadDir(pluginsRoot)
		if err != nil {
			return "", "", err
		}
		dirs := []os.DirEntry{}
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry)
			}
		}
		if len(dirs) == 1 {
			return filepath.Join(pluginsRoot, dirs[0].Name()), dirs[0].Name(), nil
		}
		if len(dirs) > 1 {
			return "", "", fmt.Errorf("zip 包含多个 Mod 目录，请分别导入目录")
		}
	}
	entries, err := os.ReadDir(extractRoot)
	if err != nil {
		return "", "", err
	}
	dirs := []os.DirEntry{}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "patchers" && entry.Name() != "config" {
			dirs = append(dirs, entry)
		}
	}
	if len(dirs) == 1 {
		return filepath.Join(extractRoot, dirs[0].Name()), dirs[0].Name(), nil
	}
	if directoryHasRegularFile(extractRoot) {
		name := strings.TrimSuffix(filepath.Base(sourceZipPath), filepath.Ext(sourceZipPath))
		if name == "" {
			name = filepath.Base(extractRoot)
		}
		return extractRoot, name, nil
	}
	return "", "", fmt.Errorf("未在 zip 包中识别到目录型 Mod")
}

// directoryHasRegularFile 判断目录根是否包含普通文件。
// 职责：辅助识别 zip 根目录即 Mod 文件集的结构。
// 使用：discoverModRootFromExtractedZip 调用。
// 边界：只检查当前目录一级文件，不递归。
func directoryHasRegularFile(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return true
		}
	}
	return false
}

// isExistingDirectory 判断路径是否存在且为目录。
// 职责：为服务层导入识别提供轻量目录判断。
// 使用：discoverModRootFromExtractedZip 调用。
// 边界：访问失败、空路径或普通文件均返回 false。
func isExistingDirectory(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// normalizePreviewKind 归一化首次同步预览类型。
// 职责：把前端传入的类型裁剪到受支持范围。
// 使用：PreviewInitialSync 调用。
// 边界：未知值回退 all，避免返回空预览。
func normalizePreviewKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "mod", "blueprint", "save", "all":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return "all"
	}
}

// normalizeSyncRecordKind 归一化同步记录资源类型。
// 职责：限制冲突处理只作用于有 environment_synced_files 记录的 Mod 和蓝图。
// 使用：UpdateClientMirrorFromGame、RestoreGameFromClientMirror 和 UnlinkEnvironmentSyncRecord 调用。
// 边界：存档不进入客户端镜像同步表，传入 save 或未知值会返回错误。
func normalizeSyncRecordKind(kind string) (string, error) {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	switch normalizedKind {
	case "mod", "blueprint":
		return normalizedKind, nil
	default:
		return "", fmt.Errorf("不支持的同步记录类型: %s", kind)
	}
}

// openDirectory 创建并打开指定目录。
// 职责：为环境页目录入口提供统一打开行为。
// 使用：OpenModLibraryDir、OpenBlueprintLibraryDir 和 OpenSaveLibraryDir 调用。
// 边界：只创建客户端资源库目录；调用方需要在真实游戏目录缺失时提前返回错误。
func openDirectory(path string) error {
	if path == "" {
		return fmt.Errorf("目录路径为空")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析目录失败: %w", err)
	}
	return openDirectoryInFileManager(absolutePath)
}

// openExistingDirectory 打开已经存在的目录。
// 职责：为真实游戏目录入口提供不创建目录的打开行为。
// 使用：OpenModRealDir、OpenBlueprintRealDir 和 OpenSaveRealDir 等真实目录入口调用。
// 边界：目录不存在或不是目录时返回错误，不尝试创建或修复。
func openExistingDirectory(path string) error {
	if path == "" {
		return fmt.Errorf("目录路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("检查目录失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析目录失败: %w", err)
	}
	return openDirectoryInFileManager(absolutePath)
}

// openDirectoryInFileManager 使用系统文件管理器打开目录。
// 职责：绕过 Wails URL scheme 限制，直接交给操作系统打开本地目录。
// 使用：openDirectory 和 openExistingDirectory 完成路径校验后调用。
// 边界：只打开目录，不创建、不删除、不定位具体文件；未知平台返回可读错误。
func openDirectoryInFileManager(path string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("explorer.exe", path)
	case "darwin":
		command = exec.Command("open", path)
	case "linux":
		command = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("当前平台不支持打开目录: %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("打开目录失败: %w", err)
	}
	return nil
}

// modResourceToEnvItem 将 Mod 扫描结果转换为持久化资源。
// 职责：统一 Mod 资源 ID、状态、来源和资源库相对路径。
// 使用：EnableMod 和首次纳管流程保存 EnvItem 前调用。
// 边界：不写数据库、不读取同步文件。
func modResourceToEnvItem(resource environmentHelper.ModResource, state string, sourceType string) environmentModel.EnvItem {
	return environmentModel.EnvItem{
		ID:          resource.ID,
		Kind:        "mod",
		Name:        resource.Name,
		State:       state,
		SourceType:  sourceType,
		LibraryPath: resource.LibraryPath,
		SizeBytes:   resource.SizeBytes,
	}
}

// modResourceToListItem 将 Mod 扫描结果转换为列表 DTO。
// 职责：只暴露卡片需要的摘要字段，避免把完整 manifest 带入列表。
// 使用：ListMods 遍历扫描结果时调用。
// 边界：详情字段通过 GetModDetail 单独读取，列表 DTO 不携带原始 JSON。
func modResourceToListItem(resource environmentHelper.ModResource) EnvironmentModItem {
	return EnvironmentModItem{
		ID:          resource.ID,
		Name:        resource.Name,
		Version:     resource.Version,
		Author:      resource.Author,
		Description: resource.Description,
		State:       resource.State,
		SizeBytes:   resource.SizeBytes,
		UpdatedAt:   resource.UpdatedAt,
		LibraryPath: resource.LibraryPath,
		Supported:   resource.Supported,
		Warning:     resource.Warning,
	}
}

// modResourceToDetail 将 Mod 扫描结果转换为详情 DTO。
// 职责：归一化 manifest 原始字段、依赖和官网链接，供前端详情弹窗展示。
// 使用：GetModDetail 找到资源后调用。
// 边界：不访问远端市场，不判断依赖安装状态；manifest 为空时返回空对象文本。
func modResourceToDetail(resource environmentHelper.ModResource) (EnvironmentModDetail, error) {
	rawManifest, err := marshalModManifest(resource.Manifest)
	if err != nil {
		return EnvironmentModDetail{}, err
	}
	return EnvironmentModDetail{
		ID:           resource.ID,
		Name:         resource.Name,
		Version:      resource.Version,
		Author:       resource.Author,
		Description:  resource.Description,
		WebsiteURL:   stringFromGenericManifest(resource.Manifest, "website_url"),
		Dependencies: normalizeModDependencies(resource.Manifest["dependencies"]),
		LibraryPath:  resource.LibraryPath,
		Supported:    resource.Supported,
		Warning:      resource.Warning,
		RawManifest:  rawManifest,
	}, nil
}

// marshalModManifest 序列化 Mod manifest 原始解析信息。
// 职责：生成可读 JSON 文本，用于前端展示和诊断。
// 使用：modResourceToDetail 调用。
// 边界：manifest 为空时返回 "{}"；序列化失败返回错误而不是吞掉证据异常。
func marshalModManifest(manifest map[string]interface{}) (string, error) {
	if len(manifest) == 0 {
		return "{}", nil
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 Mod manifest 失败: %w", err)
	}
	return string(content), nil
}

// normalizeModDependencies 归一化 manifest 依赖字段。
// 职责：兼容字符串数组和对象数组两种常见依赖描述。
// 使用：modResourceToDetail 生成前端依赖列表时调用。
// 边界：无法解析的结构保留为 fmt.Sprint 文本，不丢弃用户诊断信息。
func normalizeModDependencies(value interface{}) []EnvironmentModDependency {
	if value == nil {
		return []EnvironmentModDependency{}
	}
	dependencies := []EnvironmentModDependency{}
	switch typedValue := value.(type) {
	case []interface{}:
		for _, entry := range typedValue {
			dependencies = append(dependencies, normalizeSingleModDependency(entry))
		}
	case []string:
		for _, entry := range typedValue {
			dependencies = append(dependencies, EnvironmentModDependency{Name: entry})
		}
	default:
		dependencies = append(dependencies, EnvironmentModDependency{Name: fmt.Sprint(typedValue)})
	}
	return dependencies
}

// normalizeSingleModDependency 归一化单条 Mod 依赖。
// 职责：从字符串或对象字段中提取依赖名称和版本。
// 使用：normalizeModDependencies 遍历依赖数组时调用。
// 边界：不假设固定市场格式，缺失字段时保留原始对象文本。
func normalizeSingleModDependency(value interface{}) EnvironmentModDependency {
	switch typedValue := value.(type) {
	case string:
		return EnvironmentModDependency{Name: typedValue}
	case map[string]interface{}:
		name := firstGenericManifestString(typedValue, []string{"name", "id", "package", "dependency"})
		version := firstGenericManifestString(typedValue, []string{"version", "version_number", "versionNumber"})
		if name == "" {
			name = fmt.Sprint(typedValue)
		}
		return EnvironmentModDependency{Name: name, Version: version}
	default:
		return EnvironmentModDependency{Name: fmt.Sprint(typedValue)}
	}
}

// firstGenericManifestString 从多个候选键读取首个字符串字段。
// 职责：兼容不同 manifest 字段命名。
// 使用：normalizeSingleModDependency 提取依赖名称和版本时调用。
// 边界：只接受字符串值，非字符串返回空。
func firstGenericManifestString(manifest map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if value := stringFromGenericManifest(manifest, key); value != "" {
			return value
		}
	}
	return ""
}

// stringFromGenericManifest 从泛型 manifest 中读取字符串字段。
// 职责：避免服务层直接做重复类型断言。
// 使用：Mod 详情转换函数读取 website_url 和依赖字段时调用。
// 边界：缺失或非字符串字段返回空字符串。
func stringFromGenericManifest(manifest map[string]interface{}, key string) string {
	if len(manifest) == 0 {
		return ""
	}
	value, ok := manifest[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

// blueprintResourceToEnvItem 将蓝图扫描结果转换为持久化资源。
// 职责：统一蓝图资源 ID、父节点、状态和相对目录。
// 使用：EnableBlueprint 和首次纳管流程保存 EnvItem 前调用。
// 边界：文件夹资源不应调用该转换函数作为启用资源。
func blueprintResourceToEnvItem(resource environmentHelper.BlueprintResource, state string, sourceType string) environmentModel.EnvItem {
	relativeDir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(resource.RelativePath)))
	if relativeDir == "." {
		relativeDir = ""
	}
	return environmentModel.EnvItem{
		ID:                   resource.ID,
		Kind:                 "blueprint",
		Name:                 resource.Name,
		ParentID:             resource.ParentID,
		State:                state,
		SourceType:           sourceType,
		LibraryPath:          filepath.ToSlash(filepath.Join("blueprints", filepath.FromSlash(resource.RelativePath))),
		BlueprintRelativeDir: relativeDir,
		SizeBytes:            resource.SizeBytes,
	}
}

// retargetPlans 重新定位首次纳管后的同步证据。
// 职责：首次纳管复制方向是游戏到资源库，需要把同步记录改回资源库源文件到游戏目标文件。
// 使用：syncExistingMods 保存同步记录前调用。
// 边界：只替换路径字段，不重新计算 hash。
func retargetPlans(plans []environmentHelper.SyncedFilePlan, libraryRoot string, gameRoot string) []environmentHelper.SyncedFilePlan {
	retargeted := make([]environmentHelper.SyncedFilePlan, 0, len(plans))
	for _, plan := range plans {
		relativePath := filepath.FromSlash(plan.RelativePath)
		retargeted = append(retargeted, environmentHelper.SyncedFilePlan{
			SourcePath:   filepath.Join(libraryRoot, relativePath),
			TargetPath:   filepath.Join(gameRoot, relativePath),
			RelativePath: plan.RelativePath,
			SHA256:       plan.SHA256,
			FileSize:     plan.FileSize,
		})
	}
	return retargeted
}

// retargetPlan 重新定位单文件首次纳管后的同步证据。
// 职责：把游戏到资源库复制产生的证据转换为资源库到游戏的同步记录。
// 使用：syncExistingBlueprints 保存同步记录前调用。
// 边界：只替换源和目标路径，不重新计算 hash。
func retargetPlan(plan environmentHelper.SyncedFilePlan, libraryPath string, gamePath string) environmentHelper.SyncedFilePlan {
	return environmentHelper.SyncedFilePlan{
		SourcePath:   libraryPath,
		TargetPath:   gamePath,
		RelativePath: plan.RelativePath,
		SHA256:       plan.SHA256,
		FileSize:     plan.FileSize,
	}
}

// safeWindowsName 校验 Windows 文件名安全规则。
// 职责：阻止空名称、非法字符、路径穿越、尾随空格点号和保留设备名。
// 使用：RenameSave、RenameBlueprintItem 和 CreateBlueprintFolder 调用。
// 边界：只校验单段文件名，不处理扩展名或目录拼接。
func safeWindowsName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("名称不能为空")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("名称不能是特殊路径")
	}
	if strings.TrimRight(name, " .") != name {
		return "", fmt.Errorf("名称不能以空格或点号结尾")
	}
	if strings.ContainsAny(name, `<>:"/\|?*`) {
		return "", fmt.Errorf("名称包含 Windows 不允许的字符")
	}
	upperName := strings.ToUpper(name)
	reserved := map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	}
	if _, ok := reserved[upperName]; ok {
		return "", fmt.Errorf("名称不能使用 Windows 保留设备名")
	}
	return name, nil
}

// pathExists 判断文件或目录是否存在。
// 职责：在重命名前检查目标名称是否冲突。
// 使用：RenameSave 和 RenameBlueprintItem 调用。
// 边界：访问失败且不是不存在时保守视为存在。
func pathExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return true
}

// deterministicID 基于相对路径生成与 helper 扫描一致的稳定 ID。
// 职责：供测试或服务内部需要时复用 ID 规则。
// 使用：当前作为扩展保留；扫描 helper 已直接生成列表 ID。
// 边界：只保证同一前缀和路径稳定，不作为安全凭证。
func deterministicID(prefix string, relativePath string) string {
	sum := sha1.Sum([]byte(strings.ToLower(filepath.ToSlash(relativePath))))
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}
