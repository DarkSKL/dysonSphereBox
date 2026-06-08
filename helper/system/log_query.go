package system

import (
	"encoding/json"
	"fmt"
	"time"

	systemModel "DysonSphereBox/model/system"
)

const (
	defaultLogPage     = 1
	defaultLogPageSize = 50
	maxLogPageSize     = 200
)

// LogQuery 描述系统日志分页查询条件。
type LogQuery struct {
	// Namespace 按日志命名空间筛选；为空时不过滤。
	Namespace string `json:"namespace"`
	// Module 按模块筛选；为空时不过滤。
	Module string `json:"module"`
	// Event 按事件筛选；为空时不过滤。
	Event string `json:"event"`
	// Level 按日志等级筛选；为 0 时不过滤。
	Level systemModel.LogLevel `json:"level"`
	// Type 按日志类型筛选；为 0 时不过滤。
	Type systemModel.LogType `json:"type"`
	// Page 是从 1 开始的页码；小于 1 时使用默认值。
	Page int `json:"page"`
	// PageSize 是每页数量；超过上限时会被限制。
	PageSize int `json:"pageSize"`
}

// LogListResult 表示系统日志分页查询结果。
type LogListResult struct {
	// Items 是当前页日志列表。
	Items []LogDTO `json:"items"`
	// Total 是符合条件的日志总数。
	Total int64 `json:"total"`
	// Page 是实际使用的页码。
	Page int `json:"page"`
	// PageSize 是实际使用的每页数量。
	PageSize int `json:"pageSize"`
}

// LogDTO 表示前端可安全消费的系统日志。
// 边界：DateTime 使用字符串，Metadata 使用普通 map，避免 Wails 生成弱类型或暴露 GORM 持久化细节。
type LogDTO struct {
	// ID 是日志主键。
	ID uint `json:"id"`
	// Namespace 是日志所属业务域。
	Namespace string `json:"namespace"`
	// Module 是业务域下的具体模块。
	Module string `json:"module"`
	// Event 是稳定事件名。
	Event string `json:"event"`
	// Level 是日志重要等级。
	Level systemModel.LogLevel `json:"level"`
	// Type 是日志业务分类。
	Type systemModel.LogType `json:"type"`
	// Content 是面向用户或维护者阅读的日志摘要。
	Content string `json:"content"`
	// Metadata 是结构化扩展信息。
	Metadata map[string]interface{} `json:"metadata"`
	// Error 是失败详情；成功日志保持为空。
	Error string `json:"error"`
	// DateTime 是日志发生时间，使用 RFC3339Nano 字符串。
	DateTime string `json:"dateTime"`
}

// CreateLogRequest 描述前端主动创建日志的请求。
type CreateLogRequest struct {
	// Namespace 是日志所属业务域；为空时由 helper 使用默认值。
	Namespace string `json:"namespace"`
	// Module 是业务域下的具体模块；为空时由 helper 使用默认值。
	Module string `json:"module"`
	// Event 是可稳定检索的事件名；为空时由 helper 使用默认值。
	Event string `json:"event"`
	// Level 是日志重要等级；为 0 时按普通信息记录。
	Level systemModel.LogLevel `json:"level"`
	// Type 是日志业务分类；为 0 时按普通操作记录。
	Type systemModel.LogType `json:"type"`
	// Content 是面向人阅读的日志摘要。
	Content string `json:"content"`
	// Metadata 是结构化扩展信息。
	Metadata map[string]interface{} `json:"metadata"`
	// Error 是失败详情；成功日志保持为空。
	Error string `json:"error"`
}

// ToLogDTOs 将持久化模型转换为前端 DTO。
func ToLogDTOs(items []systemModel.Log) ([]LogDTO, error) {
	results := make([]LogDTO, 0, len(items))
	for index := range items {
		item, err := toLogDTO(items[index])
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, nil
}

func toLogDTO(item systemModel.Log) (LogDTO, error) {
	metadata := make(map[string]interface{})
	if len(item.Metadata) > 0 {
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
			return LogDTO{}, fmt.Errorf("decode system log metadata: %w", err)
		}
	}

	dateTime := ""
	if item.DateTime != nil {
		dateTime = item.DateTime.Format(time.RFC3339Nano)
	}

	return LogDTO{
		ID:        item.ID,
		Namespace: item.Namespace,
		Module:    item.Module,
		Event:     item.Event,
		Level:     item.Level,
		Type:      item.Type,
		Content:   item.Content,
		Metadata:  metadata,
		Error:     item.Error,
		DateTime:  dateTime,
	}, nil
}

// NormalizeLogPage 归一化日志分页参数。
func NormalizeLogPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = defaultLogPage
	}
	if pageSize < 1 {
		pageSize = defaultLogPageSize
	}
	if pageSize > maxLogPageSize {
		pageSize = maxLogPageSize
	}

	return page, pageSize
}
