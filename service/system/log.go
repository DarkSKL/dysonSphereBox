package system

import (
	"errors"
	"fmt"
	"time"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// LogService 提供前端可调用的系统日志读写能力。
// 用途：展示重要操作审计记录，或由前端在确有必要时主动补充用户操作日志。
type LogService struct{}

// ListLogs 按条件分页返回系统日志。
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
