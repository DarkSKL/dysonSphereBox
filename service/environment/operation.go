package environment

// 文件职责：维护环境管理操作记录、Wails 事件和托管状态复核。
// 使用：EnvironmentManagerService 的写入流程调用记录助手，环境页通过 GetOperation 与 RefreshManagedState 查询证据。
// 边界：不执行具体 Mod、蓝图、存档复制删除；具体文件操作仍在 manager.go 与 helper/environment 中完成。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core"
	"DysonSphereBox/core/diagnostics"
	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/modDependence/fileops"
	environmentModel "DysonSphereBox/model/environment"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// 环境操作状态常量集中描述 operation 生命周期。
// 职责：统一 environment_operations.status 的稳定文本。
// 使用：记录创建、完成、失败以及前端查询时复用。
// 边界：不替代数据库事务；失败恢复状态后续可继续扩展。
const (
	// envOperationStatusPending 职责：表示操作记录已创建但尚未执行；使用：需要先落操作计划的流程；边界：当前同步方法创建后立即切到 running。
	envOperationStatusPending = "pending"
	// envOperationStatusRunning 职责：表示操作正在执行；使用：前端展示同步中状态；边界：进程异常退出时后续启动恢复负责复核。
	envOperationStatusRunning = "running"
	// envOperationStatusCommitted 职责：表示文件阶段已提交但尚需复核；使用：后续更完整事务流程扩展；边界：当前恢复流程会把它纳入未完成状态复核。
	envOperationStatusCommitted = "committed"
	// envOperationStatusRollback 职责：表示操作进入回滚阶段；使用：后续快照恢复流程扩展；边界：当前恢复流程会把它纳入未完成状态复核。
	envOperationStatusRollback = "rollback"
	// envOperationStatusSuccess 职责：表示操作已完成；使用：前端展示成功和服务层审计；边界：只说明当前方法返回成功。
	envOperationStatusSuccess = "success"
	// envOperationStatusFailed 职责：表示操作失败；使用：前端展示失败和后续冲突恢复；边界：不自动回滚文件，回滚由具体流程负责。
	envOperationStatusFailed = "failed"
	// envOperationStatusRecovered 职责：表示启动恢复已复核并收束旧操作；使用：恢复 pending/running/rollback 操作后写入；边界：不代表冲突自动解决。
	envOperationStatusRecovered = "recovered"
)

// 环境操作事件常量集中描述 Wails 事件名称。
// 职责：统一前后端 operation 事件通道。
// 使用：recordEnvironmentOperationStart、finishEnvironmentOperation 和 emitEnvironmentOperationEvent 调用。
// 边界：RuntimeContext 不存在时静默跳过，不影响测试和命令行构建。
const (
	// environmentOperationStartedEvent 职责：通知前端操作开始；使用：资源写入流程创建记录后发出；边界：不保证前端一定订阅。
	environmentOperationStartedEvent = "environment:operation-started"
	// environmentOperationProgressEvent 职责：通知前端操作进度；使用：状态复核或批量操作扩展时发出；边界：当前只发送粗粒度阶段。
	environmentOperationProgressEvent = "environment:operation-progress"
	// environmentOperationCompletedEvent 职责：通知前端操作完成；使用：写入流程成功后发出；边界：不携带大体积列表数据。
	environmentOperationCompletedEvent = "environment:operation-completed"
	// environmentOperationFailedEvent 职责：通知前端操作失败；使用：写入流程返回错误时发出；边界：错误详情仍以 operation.Detail 为准。
	environmentOperationFailedEvent = "environment:operation-failed"
)

// operationDetailPayload 描述写入 operation.detail_json 的通用结构。
// 职责：保存操作计划、结果、失败原因和附加证据。
// 使用：operation 记录助手和 RefreshManagedState 复核结果写入。
// 边界：字段为宽松 JSON 映射，具体业务只写自己拥有的证据。
type operationDetailPayload struct {
	// Plan 职责：保存执行前计划；使用：启动恢复和诊断；边界：不得保存大体积文件内容。
	Plan map[string]interface{} `json:"plan,omitempty"`
	// Result 职责：保存执行后结果摘要；使用：操作详情展示；边界：不替代具体资源列表接口。
	Result map[string]interface{} `json:"result,omitempty"`
	// Failure 职责：保存失败原因；使用：排错和后续恢复；边界：不保存堆栈。
	Failure map[string]interface{} `json:"failure,omitempty"`
	// Issues 职责：保存复核问题；使用：RefreshManagedState 证据链；边界：只记录摘要，不授权自动覆盖。
	Issues []EnvironmentManagedStateIssue `json:"issues,omitempty"`
}

// environmentOperationEventPayload 描述发送给前端的操作事件负载。
// 职责：提供前端识别操作 ID、动作、状态和消息的最小信息。
// 使用：emitEnvironmentOperationEvent 发送 Wails 事件时构建。
// 边界：事件丢失不影响数据库 operation 记录作为权威。
type environmentOperationEventPayload struct {
	// ID 职责：保存操作记录 ID；使用：前端后续调用 GetOperation；边界：数据库不可用时可为 0。
	ID uint `json:"id"`
	// Operation 职责：保存操作动作；使用：前端展示当前动作；边界：原样透传。
	Operation string `json:"operation"`
	// Kind 职责：保存资源类型；使用：前端筛选标签页；边界：原样透传。
	Kind string `json:"kind"`
	// ItemID 职责：保存关联资源 ID；使用：前端标记操作中的卡片；边界：批量操作可为空。
	ItemID string `json:"itemId"`
	// Status 职责：保存操作状态；使用：前端更新进度；边界：不替代 GetOperation。
	Status string `json:"status"`
	// Message 职责：保存用户可读消息；使用：前端提示；边界：不用于业务判断。
	Message string `json:"message"`
}

// GetOperation 查询单个环境管理操作。
// 职责：按 ID 返回 operation 表中的状态、消息和 detail_json。
// 使用：前端在收到 operationId 或事件后查询完整证据。
// 边界：数据库未初始化或记录不存在时返回错误，不扫描文件系统。
func (service *EnvironmentManagerService) GetOperation(id uint) (EnvironmentOperation, error) {
	if id == 0 {
		return EnvironmentOperation{}, fmt.Errorf("操作 ID 不能为空")
	}
	if core.GLOBAL.DB == nil {
		return EnvironmentOperation{}, fmt.Errorf("database is not initialized")
	}

	record := environmentModel.EnvOperation{}
	if err := core.GLOBAL.DB.First(&record, "id = ?", id).Error; err != nil {
		return EnvironmentOperation{}, err
	}
	return operationRecordToDTO(record), nil
}

// RecoverInterruptedOperations 恢复未完成的环境管理操作。
// 职责：扫描 pending/running/committed/rollback 操作，清理临时文件，复核同步状态并收束 operation。
// 使用：应用启动后和环境页进入时调用，保证 UI 不停留在不确定状态。
// 边界：只删除明确的 .dsbox 临时痕迹，不覆盖用户资源；冲突通过 RefreshManagedState 变为 conflict/missing。
// 逻辑流：1. 查询未完成 operation => 2. 清理 import-tmp 与 .dsbox 临时文件 => 3. 复核托管状态 => 4. 按清理结果标记 recovered 或 failed。
func (service *EnvironmentManagerService) RecoverInterruptedOperations() (EnvironmentRecoverySummary, error) {
	if core.GLOBAL.DB == nil {
		return EnvironmentRecoverySummary{}, fmt.Errorf("database is not initialized")
	}

	pendingOperations, err := service.listRecoverableEnvironmentOperations()
	if err != nil {
		return EnvironmentRecoverySummary{}, err
	}
	summary := EnvironmentRecoverySummary{
		PendingOperations: len(pendingOperations),
		RemovedPaths:      []string{},
		FailedPaths:       []string{},
	}
	if len(pendingOperations) == 0 {
		return summary, nil
	}

	cleanupSummary := environmentHelper.CleanupInterruptedTemporaryFiles(service.buildTemporaryCleanupTarget())
	summary.RemovedPaths = cleanupSummary.RemovedPaths
	summary.FailedPaths = cleanupSummary.FailedPaths

	managedState, stateErr := service.refreshManagedStateWithoutOperation("all")
	if stateErr == nil {
		summary.ManagedState = &managedState
	} else {
		summary.FailedPaths = append(summary.FailedPaths, fmt.Sprintf("托管状态复核失败: %v", stateErr))
	}

	if len(summary.FailedPaths) > 0 || stateErr != nil {
		summary.FailedOperations = len(pendingOperations)
		for _, operation := range pendingOperations {
			_ = service.finishEnvironmentOperation(operation.ID, envOperationStatusFailed, "环境操作恢复失败", operationDetailPayload{
				Plan: map[string]interface{}{
					"recoveredFrom": operation.Status,
				},
				Failure: map[string]interface{}{
					"failedPaths": summary.FailedPaths,
				},
				Issues: managedState.Issues,
			})
		}
		return summary, fmt.Errorf("环境操作恢复失败")
	}

	for _, operation := range pendingOperations {
		if err := service.finishEnvironmentOperation(operation.ID, envOperationStatusRecovered, "环境操作已恢复", operationDetailPayload{
			Plan: map[string]interface{}{
				"recoveredFrom": operation.Status,
			},
			Result: map[string]interface{}{
				"removedPathCount": len(summary.RemovedPaths),
				"checkedRecords":   managedState.CheckedRecords,
				"changedItems":     managedState.ChangedItems,
				"conflictItems":    managedState.ConflictItems,
				"missingItems":     managedState.MissingItems,
			},
			Issues: managedState.Issues,
		}); err != nil {
			summary.FailedOperations++
			return summary, err
		}
		summary.RecoveredOperations++
	}
	return summary, nil
}

// RefreshManagedState 复核已托管资源与真实目标文件的一致性。
// 职责：比较当前目标根、同步记录根、目标文件存在性和 hash，必要时标记资源为 missing 或 conflict。
// 使用：环境页进入、用户点击刷新或操作失败后调用。
// 边界：不会删除或覆盖任何真实文件；根目录变化时只标记冲突并保留旧绝对路径。
// 逻辑流：1. 创建复核 operation => 2. 解析当前目标根 => 3. 遍历同步记录核对 root/hash/存在性 => 4. 更新资源状态 => 5. 写入复核摘要。
func (service *EnvironmentManagerService) RefreshManagedState(kind string) (EnvironmentManagedStateSummary, error) {
	normalizedKind := normalizeManagedStateKind(kind)
	operationID, err := service.recordEnvironmentOperationStart("refresh_state", normalizedKind, "", map[string]interface{}{
		"kind": normalizedKind,
	})
	if err != nil {
		return EnvironmentManagedStateSummary{}, err
	}

	summary, runErr := service.refreshManagedStateWithoutOperation(normalizedKind)
	summary.OperationID = operationID
	if runErr != nil {
		_ = service.finishEnvironmentOperation(operationID, envOperationStatusFailed, runErr.Error(), operationDetailPayload{
			Plan:    map[string]interface{}{"kind": normalizedKind},
			Failure: map[string]interface{}{"message": runErr.Error()},
		})
		return EnvironmentManagedStateSummary{}, runErr
	}

	if err := service.finishEnvironmentOperation(operationID, envOperationStatusSuccess, "托管状态已复核", operationDetailPayload{
		Plan: map[string]interface{}{"kind": normalizedKind},
		Result: map[string]interface{}{
			"checkedRecords": summary.CheckedRecords,
			"checkedItems":   summary.CheckedItems,
			"changedItems":   summary.ChangedItems,
			"conflictItems":  summary.ConflictItems,
			"missingItems":   summary.MissingItems,
		},
		Issues: summary.Issues,
	}); err != nil {
		return EnvironmentManagedStateSummary{}, err
	}
	return summary, nil
}

// recordEnvironmentOperationStart 创建并启动环境操作记录。
// 职责：把操作计划写入 environment_operations，并发出 started/progress 事件。
// 使用：写入类 Wails 方法进入真实文件操作前调用。
// 边界：数据库不可用时返回错误；不包裹具体业务事务。
func (service *EnvironmentManagerService) recordEnvironmentOperationStart(operation string, kind string, itemID string, plan map[string]interface{}) (uint, error) {
	if core.GLOBAL.DB == nil {
		return 0, fmt.Errorf("database is not initialized")
	}

	detailJSON, err := marshalOperationDetail(operationDetailPayload{Plan: plan})
	if err != nil {
		return 0, err
	}
	record := environmentModel.EnvOperation{
		Operation:  operation,
		Kind:       kind,
		ItemID:     itemID,
		Status:     envOperationStatusPending,
		Message:    "操作已创建",
		DetailJSON: detailJSON,
		CreatedAt:  time.Now(),
	}
	if err := core.GLOBAL.DB.Create(&record).Error; err != nil {
		return 0, err
	}
	if err := core.GLOBAL.DB.Model(&environmentModel.EnvOperation{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"status":  envOperationStatusRunning,
		"message": "操作执行中",
	}).Error; err != nil {
		return 0, err
	}

	service.emitEnvironmentOperationEvent(environmentOperationStartedEvent, record.ID, operation, kind, itemID, envOperationStatusRunning, "操作执行中")
	service.emitEnvironmentOperationEvent(environmentOperationProgressEvent, record.ID, operation, kind, itemID, envOperationStatusRunning, "正在处理文件与状态")
	return record.ID, nil
}

// runRecordedEnvironmentOperation 包装带统一结果 DTO 的环境写操作。
// 职责：为启用、禁用、导入、删除、重命名等流程统一创建和收尾 operation 记录。
// 使用：EnvironmentManagerService 的公开写入方法以闭包传入真实业务逻辑。
// 边界：不改变业务闭包的文件和数据库行为；收尾记录失败会作为方法错误返回。
func (service *EnvironmentManagerService) runRecordedEnvironmentOperation(operation string, kind string, itemID string, plan map[string]interface{}, action func() (EnvironmentOperationResult, error)) (EnvironmentOperationResult, error) {
	operationID, err := service.recordEnvironmentOperationStart(operation, kind, itemID, plan)
	if err != nil {
		return EnvironmentOperationResult{}, err
	}

	result, actionErr := action()
	if actionErr != nil {
		_ = service.finishEnvironmentOperation(operationID, envOperationStatusFailed, actionErr.Error(), operationDetailPayload{
			Plan:    plan,
			Failure: map[string]interface{}{"message": actionErr.Error()},
		})
		return EnvironmentOperationResult{}, actionErr
	}

	result = service.attachOperationIDToResult(result, operationID)
	message := result.Message
	if strings.TrimSpace(message) == "" {
		message = "操作已完成"
	}
	if err := service.finishEnvironmentOperation(operationID, envOperationStatusSuccess, message, operationDetailPayload{
		Plan: plan,
		Result: map[string]interface{}{
			"message": message,
		},
	}); err != nil {
		return EnvironmentOperationResult{}, err
	}
	return result, nil
}

// finishEnvironmentOperation 完成环境操作记录。
// 职责：把操作状态、消息和结构化结果写回 operation 表并发出完成或失败事件。
// 使用：写入类方法 defer 或显式分支调用。
// 边界：operationID 为 0 时直接跳过，避免测试或旧流程崩溃。
func (service *EnvironmentManagerService) finishEnvironmentOperation(operationID uint, status string, message string, detail operationDetailPayload) error {
	if operationID == 0 || core.GLOBAL.DB == nil {
		return nil
	}
	detailJSON, err := marshalOperationDetail(detail)
	if err != nil {
		return err
	}
	if err := core.GLOBAL.DB.Model(&environmentModel.EnvOperation{}).Where("id = ?", operationID).Updates(map[string]interface{}{
		"status":      status,
		"message":     message,
		"detail_json": detailJSON,
	}).Error; err != nil {
		return err
	}

	record := environmentModel.EnvOperation{}
	if err := core.GLOBAL.DB.First(&record, "id = ?", operationID).Error; err == nil {
		level, handler := logging.Info, logging.Unknown
		if status == envOperationStatusFailed {
			level = logging.Error
		}
		if status == envOperationStatusRecovered {
			handler = logging.System
		}
		diagnostics.Default.Record(logging.Entry{Action: message, Level: level, Source: "backend.environment", Reason: "环境操作收尾状态已写入，文件复核证据见 data", Namespace: "environment.resource." + record.Operation, Handler: handler, OperationID: fmt.Sprintf("environment-%d", record.ID), Data: map[string]interface{}{"result": status, "kind": record.Kind, "itemId": record.ItemID, "plan": detail.Plan, "evidence": detail.Result, "error": detail.Failure, "issues": detail.Issues, "durationMs": time.Since(record.CreatedAt).Milliseconds()}})
		eventName := environmentOperationCompletedEvent
		if status == envOperationStatusFailed {
			eventName = environmentOperationFailedEvent
		}
		service.emitEnvironmentOperationEvent(eventName, record.ID, record.Operation, record.Kind, record.ItemID, status, message)
	}
	return nil
}

// attachOperationIDToResult 把操作 ID 写入统一结果 DTO。
// 职责：避免各业务方法重复设置 OperationID。
// 使用：withEnvironmentOperation 包装返回值时调用。
// 边界：result 按值传递，不修改其它状态。
func (service *EnvironmentManagerService) attachOperationIDToResult(result EnvironmentOperationResult, operationID uint) EnvironmentOperationResult {
	result.OperationID = operationID
	return result
}

// emitEnvironmentOperationEvent 发送环境操作 Wails 事件。
// 职责：在 Wails RuntimeContext 可用时通知前端 operation 状态变化。
// 使用：operation 生命周期助手调用。
// 边界：测试、构建或无前端上下文时静默跳过，不影响数据库结果。
func (service *EnvironmentManagerService) emitEnvironmentOperationEvent(eventName string, operationID uint, operation string, kind string, itemID string, status string, message string) {
	if core.GLOBAL.RuntimeContext == nil {
		return
	}
	runtime.EventsEmit(core.GLOBAL.RuntimeContext, eventName, environmentOperationEventPayload{
		ID:        operationID,
		Operation: operation,
		Kind:      kind,
		ItemID:    itemID,
		Status:    status,
		Message:   message,
	})
}

// refreshManagedStateWithoutOperation 执行不包含 operation 生命周期的状态复核。
// 职责：复用给 RefreshManagedState 和测试，专注同步记录复核与状态写入。
// 使用：RefreshManagedState 创建 operation 后调用。
// 边界：不发事件、不写 operation；发现问题只更新 EnvItem 状态。
func (service *EnvironmentManagerService) refreshManagedStateWithoutOperation(kind string) (EnvironmentManagedStateSummary, error) {
	if core.GLOBAL.DB == nil {
		return EnvironmentManagedStateSummary{}, fmt.Errorf("database is not initialized")
	}

	records := []environmentModel.EnvSyncedFile{}
	query := core.GLOBAL.DB.Model(&environmentModel.EnvSyncedFile{})
	if kind != "all" {
		query = query.Where("kind = ?", kind)
	}
	if err := query.Find(&records).Error; err != nil {
		return EnvironmentManagedStateSummary{}, err
	}

	summary := EnvironmentManagedStateSummary{Kind: kind, CheckedRecords: len(records), Issues: []EnvironmentManagedStateIssue{}}
	checkedItems := map[string]struct{}{}
	itemStateByID := map[string]string{}
	resolver := environmentHelper.Resolver{}
	currentRoots := map[string]string{
		"mod":       resolver.ResolveModRoot().Path,
		"blueprint": resolver.ResolveBlueprintRoot().Path,
	}

	for _, record := range records {
		checkedItems[record.ItemID] = struct{}{}
		issue, hasIssue := service.inspectSyncedRecord(record, currentRoots[record.Kind])
		if !hasIssue {
			continue
		}
		summary.Issues = append(summary.Issues, issue)
		if previousState := itemStateByID[record.ItemID]; previousState == environmentHelper.EnvironmentStateConflict {
			continue
		}
		itemStateByID[record.ItemID] = issue.State
	}

	summary.CheckedItems = len(checkedItems)
	if len(itemStateByID) == 0 {
		return summary, nil
	}

	now := time.Now()
	if err := core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		for itemID, state := range itemStateByID {
			if err := tx.Model(&environmentModel.EnvItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
				"state":      state,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return EnvironmentManagedStateSummary{}, err
	}

	summary.ChangedItems = len(itemStateByID)
	for _, state := range itemStateByID {
		if state == environmentHelper.EnvironmentStateConflict {
			summary.ConflictItems++
		}
		if state == environmentHelper.EnvironmentStateMissing {
			summary.MissingItems++
		}
	}
	return summary, nil
}

// listRecoverableEnvironmentOperations 查询需要启动恢复的环境操作。
// 职责：筛选 pending、running、committed、rollback 状态的 operation。
// 使用：RecoverInterruptedOperations 调用。
// 边界：只读取环境管理 operation 表，不处理系统日志。
func (service *EnvironmentManagerService) listRecoverableEnvironmentOperations() ([]environmentModel.EnvOperation, error) {
	records := []environmentModel.EnvOperation{}
	statuses := []string{
		envOperationStatusPending,
		envOperationStatusRunning,
		envOperationStatusCommitted,
		envOperationStatusRollback,
	}
	if err := core.GLOBAL.DB.Where("status IN ?", statuses).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// buildTemporaryCleanupTarget 构造环境恢复临时文件清理范围。
// 职责：统一解析 AppData、Mod、蓝图和存档根目录，供 helper 在明确根目录内清理。
// 使用：RecoverInterruptedOperations 调用。
// 边界：解析失败的路径留空，helper 会跳过；不创建目录。
func (service *EnvironmentManagerService) buildTemporaryCleanupTarget() environmentHelper.TemporaryCleanupTarget {
	resolver := environmentHelper.Resolver{}
	appDataRoot, _ := resolver.AppDataRoot()
	modRoot := resolver.ResolveModRoot()
	blueprintRoot := resolver.ResolveBlueprintRoot()
	saveRoot := resolver.ResolveSaveRoot()
	return environmentHelper.TemporaryCleanupTarget{
		AppDataRoot:   appDataRoot,
		ModRoot:       modRoot.Path,
		BlueprintRoot: blueprintRoot.Path,
		SaveRoot:      saveRoot.Path,
	}
}

// inspectSyncedRecord 复核单条同步记录。
// 职责：检查当前目标根、登记目标根、目标路径和 hash 是否仍一致。
// 使用：refreshManagedStateWithoutOperation 遍历同步文件时调用。
// 边界：只返回问题描述，不更新数据库，不删除任何文件。
func (service *EnvironmentManagerService) inspectSyncedRecord(record environmentModel.EnvSyncedFile, currentRoot string) (EnvironmentManagedStateIssue, bool) {
	issue := EnvironmentManagedStateIssue{
		ItemID:       record.ItemID,
		Kind:         record.Kind,
		RelativePath: record.RelativePath,
		TargetPath:   record.TargetPath,
		ExpectedRoot: currentRoot,
		RecordedRoot: record.TargetRootPath,
	}
	if currentRoot == "" {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "missing_current_root"
		issue.Message = "当前目标根目录不可用，已标记冲突"
		return issue, true
	}
	if !sameCleanPath(record.TargetRootPath, currentRoot) {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "root_changed"
		issue.Message = "目标根目录已变化，旧同步记录需要人工确认"
		return issue, true
	}
	if !service.syncedRecordTargetMatchesCurrentRoot(record, currentRoot) {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "target_path_mismatch"
		issue.Message = "同步目标路径与当前根目录不一致，已标记冲突"
		return issue, true
	}
	if _, err := os.Stat(record.TargetPath); err != nil {
		if os.IsNotExist(err) {
			issue.State = environmentHelper.EnvironmentStateMissing
			issue.Reason = "missing_target"
			issue.Message = "目标文件缺失，已标记不可用"
			return issue, true
		}
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "target_stat_failed"
		issue.Message = fmt.Sprintf("无法读取目标文件状态: %v", err)
		return issue, true
	}
	if strings.TrimSpace(record.SHA256) == "" {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "missing_hash"
		issue.Message = "同步记录缺少 hash，已标记冲突"
		return issue, true
	}
	currentSHA256, _, err := environmentHelper.HashFile(record.TargetPath)
	if err != nil {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "hash_failed"
		issue.Message = fmt.Sprintf("计算目标文件 hash 失败: %v", err)
		return issue, true
	}
	if currentSHA256 != record.SHA256 {
		issue.State = environmentHelper.EnvironmentStateConflict
		issue.Reason = "hash_mismatch"
		issue.Message = "目标文件已被外部修改，已标记冲突"
		return issue, true
	}
	return EnvironmentManagedStateIssue{}, false
}

// syncedRecordTargetMatchesCurrentRoot 判断同步记录的目标路径是否仍位于当前目标根。
// 职责：兼容 Mod 目录型资源和蓝图单文件资源的相对路径规则差异。
// 使用：inspectSyncedRecord 在 hash 复核前确认记录路径仍属于当前根目录。
// 边界：Mod 旧记录可能只保存相对 Mod 目录的 RelativePath，因此以 TargetPath 根目录边界为准；蓝图继续按 RelativePath 严格复核。
func (service *EnvironmentManagerService) syncedRecordTargetMatchesCurrentRoot(record environmentModel.EnvSyncedFile, currentRoot string) bool {
	if !fileops.IsSubPath(currentRoot, record.TargetPath) {
		return false
	}
	if record.Kind == "mod" {
		return true
	}
	expectedTarget := filepath.Join(currentRoot, filepath.FromSlash(record.RelativePath))
	return sameCleanPath(record.TargetPath, expectedTarget)
}

// markEnvItemState 更新单个资源状态。
// 职责：在安全禁用或状态复核发现异常时写入 EnvItem.State。
// 使用：disableSyncedItem 与 refreshManagedStateWithoutOperation 调用。
// 边界：数据库不可用时返回错误；不清理同步记录。
func (service *EnvironmentManagerService) markEnvItemState(itemID string, kind string, state string) error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return core.GLOBAL.DB.Model(&environmentModel.EnvItem{}).Where("id = ? AND kind = ?", itemID, kind).Updates(map[string]interface{}{
		"state":      state,
		"updated_at": time.Now(),
	}).Error
}

// currentTargetRootForKind 返回指定资源类型当前解析目标根。
// 职责：为禁用和复核提供同一套当前根目录判断。
// 使用：disableSyncedItem 和 inspectSyncedRecord 相关流程调用。
// 边界：只支持 mod、blueprint；其它类型返回空字符串。
func (service *EnvironmentManagerService) currentTargetRootForKind(kind string) string {
	resolver := environmentHelper.Resolver{}
	switch kind {
	case "mod":
		return resolver.ResolveModRoot().Path
	case "blueprint":
		return resolver.ResolveBlueprintRoot().Path
	default:
		return ""
	}
}

// operationRecordToDTO 将持久化 operation 转为前端 DTO。
// 职责：隔离 GORM 模型和 Wails DTO，保留 detail_json 原始结构。
// 使用：GetOperation 调用。
// 边界：空 detail 返回 null，避免前端解析空字符串。
func operationRecordToDTO(record environmentModel.EnvOperation) EnvironmentOperation {
	detail := json.RawMessage("null")
	if len(record.DetailJSON) > 0 {
		detail = json.RawMessage(record.DetailJSON)
	}
	return EnvironmentOperation{
		ID:        record.ID,
		Operation: record.Operation,
		Kind:      record.Kind,
		ItemID:    record.ItemID,
		Status:    record.Status,
		Message:   record.Message,
		Detail:    detail,
		CreatedAt: record.CreatedAt,
	}
}

// marshalOperationDetail 序列化操作详情。
// 职责：把结构化计划、结果和失败摘要转为 datatypes.JSON。
// 使用：创建和完成 operation 时调用。
// 边界：序列化失败返回错误；空结构会写入合法 JSON 对象。
func marshalOperationDetail(detail operationDetailPayload) (datatypes.JSON, error) {
	content, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("序列化操作详情失败: %w", err)
	}
	return datatypes.JSON(content), nil
}

// normalizeManagedStateKind 归一化托管状态复核范围。
// 职责：只允许 all、mod、blueprint 三种复核范围。
// 使用：RefreshManagedState 调用。
// 边界：未知值回退 all，避免误过滤。
func normalizeManagedStateKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "mod", "blueprint", "all":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return "all"
	}
}

// sameCleanPath 判断两个路径清理后是否相同。
// 职责：兼容 Windows 大小写路径比较，避免根目录变化误判。
// 使用：复核同步记录和禁用目标根时调用。
// 边界：空路径只有同时为空才相等。
func sameCleanPath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	return strings.EqualFold(left, right)
}
