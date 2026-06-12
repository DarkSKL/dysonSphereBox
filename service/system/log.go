package system

// 文件职责：提供系统操作日志分页查询、前端补充记录和历史清理的 Wails 服务。
// 使用：日志页筛选审计记录，或前端记录后端无法感知的重要交互时调用。
// 边界：业务关键日志优先由后端 helper 自动写入；本服务不替代具体业务流程埋点。

import (
	"errors"
	"fmt"
	"time"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// LogService 提供前端可调用的系统日志读写能力。
// 职责：分页查询、创建和清理系统操作日志。
// 使用：日志页展示审计记录，或前端在后端无法感知的重要交互中主动记录日志。
// 边界：业务操作日志优先由后端 helper 自动记录，前端不应替代后端关键流程埋点。
type LogService struct{}

// ListLogs 按条件分页返回系统日志。
// 职责：根据 namespace/module/event/level/type 条件分页读取日志。
// 使用：日志页筛选、分页和刷新列表时调用。
// 边界：数据库未初始化时返回错误；metadata 转换失败会透传错误。
// 逻辑流：1. 校验数据库 => 2. 规范化分页 => 3. 叠加筛选条件 => 4. 统计总数并分页查询 => 5. 转换 DTO 返回。
func (service *LogService) ListLogs(query systemHelper.LogQuery) (systemHelper.LogListResult, error) {
	if core.GLOBAL.DB == nil {
		return systemHelper.LogListResult{}, errors.New("database is not initialized")
	}

	page, pageSize := systemHelper.NormalizeLogPage(query.Page, query.PageSize)
	dbQuery := core.GLOBAL.DB.Model(&systemModel.Log{})

	if query.Namespace != "" {
		dbQuery = dbQuery.Where("namespace = ?", query.Namespace)
	}
	if query.Module != "" {
		dbQuery = dbQuery.Where("module = ?", query.Module)
	}
	if query.Event != "" {
		dbQuery = dbQuery.Where("event = ?", query.Event)
	}
	if query.Level != 0 {
		dbQuery = dbQuery.Where("level = ?", query.Level)
	}
	if query.Type != 0 {
		dbQuery = dbQuery.Where("type = ?", query.Type)
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return systemHelper.LogListResult{}, fmt.Errorf("count system logs: %w", err)
	}

	items := make([]systemModel.Log, 0, pageSize)
	err := dbQuery.
		Order("date_time DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&items).
		Error
	if err != nil {
		return systemHelper.LogListResult{}, fmt.Errorf("list system logs: %w", err)
	}

	logItems, err := systemHelper.ToLogDTOs(items)
	if err != nil {
		return systemHelper.LogListResult{}, err
	}

	return systemHelper.LogListResult{
		Items:    logItems,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateLog 主动创建一条系统日志。
// 使用：前端只应记录后端无法感知的重要交互；后端业务操作优先使用 helper 内部埋点。
// 职责：把前端提交的日志请求转换为统一 LogEntry。
// 边界：不补充业务上下文；调用方必须传入清晰 namespace/module/event。
func (service *LogService) CreateLog(request systemHelper.CreateLogRequest) error {
	return systemHelper.RecordLog(systemHelper.LogEntry{
		Namespace: request.Namespace,
		Module:    request.Module,
		Event:     request.Event,
		Level:     request.Level,
		Type:      request.Type,
		Content:   request.Content,
		Metadata:  request.Metadata,
		Error:     request.Error,
	})
}

// DeleteLogsBefore 删除指定时间之前的系统日志。
// 职责：按时间边界清理历史系统日志。
// 使用：后续日志清理设置或维护工具触发时调用。
// 边界：用于后续日志清理设置；before 为空时拒绝执行，避免误删全部审计记录。
func (service *LogService) DeleteLogsBefore(before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, errors.New("before time cannot be zero")
	}
	if core.GLOBAL.DB == nil {
		return 0, errors.New("database is not initialized")
	}

	result := core.GLOBAL.DB.
		Where("date_time < ?", before).
		Delete(&systemModel.Log{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete system logs: %w", result.Error)
	}

	return result.RowsAffected, nil
}
