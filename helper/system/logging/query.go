package logging

// 文件职责：构造统一日志筛选与数据库失效后的文件检索。使用：LogService；边界：关键词为字面量，降级明确告警。

import (
	contract "DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"slices"
	"strings"
	"time"
)

// ValidateQuery 职责：校验时间范围；使用：列表和导出；边界：非法时间拒绝查询。
func ValidateQuery(q systemHelper.LogQuery) error {
	if len(q.Namespaces) > 64 || len(q.Levels) > 5 || len(q.Handlers) > 3 {
		return fmt.Errorf("日志筛选选项过多")
	}
	for _, value := range q.Levels {
		switch value {
		case contract.Debug, contract.Info, contract.Warn, contract.Error, contract.Fatal:
		default:
			return fmt.Errorf("无效日志级别: %s", value)
		}
	}
	for _, value := range q.Handlers {
		switch value {
		case contract.User, contract.System, contract.Unknown:
		default:
			return fmt.Errorf("无效日志触发者: %s", value)
		}
	}
	for _, value := range q.Namespaces {
		if strings.TrimSpace(value) == "" || len(value) > 180 {
			return fmt.Errorf("无效日志模块")
		}
	}
	var from, to time.Time
	var err error
	if q.From != "" {
		from, err = time.Parse(time.RFC3339Nano, q.From)
		if err != nil {
			return fmt.Errorf("起始时间无效: %w", err)
		}
	}
	if q.To != "" {
		to, err = time.Parse(time.RFC3339Nano, q.To)
		if err != nil {
			return fmt.Errorf("结束时间无效: %w", err)
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return fmt.Errorf("起始时间不能晚于结束时间")
	}
	return nil
}

// ApplyQuery 职责：叠加参数化筛选；使用：列表和导出；边界：不执行查询。
func ApplyQuery(db *gorm.DB, q systemHelper.LogQuery) *gorm.DB {
	if q.Namespaces != nil {
		if len(q.Namespaces) == 0 {
			db = db.Where("1 = 0")
		} else {
			conditions := make([]string, 0, len(q.Namespaces))
			args := make([]interface{}, 0, len(q.Namespaces)*3)
			for _, namespace := range q.Namespaces {
				conditions = append(conditions, "namespace = ? OR substr(namespace, 1, ?) = ?")
				args = append(args, namespace, len(namespace)+1, namespace+".")
			}
			db = db.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}
	} else if q.Namespace != "" {
		db = db.Where("(namespace = ? OR substr(namespace, 1, ?) = ?)", q.Namespace, len(q.Namespace)+1, q.Namespace+".")
	}
	if q.Levels != nil {
		db = db.Where("level IN ?", q.Levels)
	} else if q.Level != "" {
		db = db.Where("level = ?", q.Level)
	}
	if q.Handlers != nil {
		db = db.Where("handler IN ?", q.Handlers)
	} else if q.Handler != "" {
		db = db.Where("handler = ?", q.Handler)
	}
	for column, value := range map[string]string{"source": q.Source, "session_id": q.SessionID, "operation_id": q.OperationID} {
		if value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	if q.Keyword != "" {
		db = db.Where("instr(action, ?) > 0 OR instr(reason, ?) > 0 OR instr(data, ?) > 0 OR instr(namespace, ?) > 0", q.Keyword, q.Keyword, q.Keyword, q.Keyword)
	}
	if q.From != "" {
		t, _ := time.Parse(time.RFC3339Nano, q.From)
		db = db.Where("event_time >= ?", t)
	}
	if q.To != "" {
		t, _ := time.Parse(time.RFC3339Nano, q.To)
		db = db.Where("event_time <= ?", t)
	}
	return db
}

// Match 职责：实现与索引等价的文件筛选；使用：降级检索；边界：不把关键词当正则。
func Match(e contract.Entry, q systemHelper.LogQuery) bool {
	if q.Namespaces != nil {
		matched := false
		for _, namespace := range q.Namespaces {
			if e.Namespace == namespace || strings.HasPrefix(e.Namespace, namespace+".") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	} else if q.Namespace != "" && e.Namespace != q.Namespace && !strings.HasPrefix(e.Namespace, q.Namespace+".") {
		return false
	}
	if q.Levels != nil {
		if !slices.Contains(q.Levels, e.Level) {
			return false
		}
	} else if q.Level != "" && e.Level != q.Level {
		return false
	}
	if q.Handlers != nil {
		if !slices.Contains(q.Handlers, e.Handler) {
			return false
		}
	} else if q.Handler != "" && e.Handler != q.Handler {
		return false
	}
	if (q.Source != "" && e.Source != q.Source) || (q.SessionID != "" && e.SessionID != q.SessionID) || (q.OperationID != "" && e.OperationID != q.OperationID) {
		return false
	}
	when, err := time.Parse(time.RFC3339Nano, e.Time)
	if err != nil {
		return false
	}
	if q.From != "" {
		from, _ := time.Parse(time.RFC3339Nano, q.From)
		if when.Before(from) {
			return false
		}
	}
	if q.To != "" {
		to, _ := time.Parse(time.RFC3339Nano, q.To)
		if when.After(to) {
			return false
		}
	}
	if q.Keyword != "" {
		data, _ := json.Marshal(e.Data)
		if !strings.Contains(e.Action+e.Reason+e.Namespace+string(data), q.Keyword) {
			return false
		}
	}
	return true
}

// List 职责：查询索引或文件降级；使用：日志页；边界：查询失败不会返回虚假的空列表。
func List(db *gorm.DB, q systemHelper.LogQuery) (systemHelper.LogListResult, error) {
	if err := ValidateQuery(q); err != nil {
		return systemHelper.LogListResult{}, err
	}
	page, size := systemHelper.NormalizeLogPage(q.Page, q.PageSize)
	result := systemHelper.LogListResult{Items: []contract.Entry{}, Page: page, PageSize: size}
	indexError, _ := Health()
	if db != nil && indexError == "" {
		query := ApplyQuery(db.Session(&gorm.Session{Logger: logger.Discard}).Model(&systemModel.LogEntry{}), q)
		if err := query.Count(&result.Total).Error; err == nil {
			rows := []systemModel.LogEntry{}
			if err := query.Order("event_time DESC, id DESC").Limit(size).Offset((page - 1) * size).Find(&rows).Error; err == nil {
				result.Items = systemHelper.ToLogEntries(rows)
				return result, nil
			} else {
				indexError = err.Error()
			}
		} else {
			indexError = err.Error()
		}
	}
	deleted, err := deletedIDs(db)
	if err != nil {
		return result, err
	}
	items, warning := diagnostics.Default.ReadEntries()
	result.Warning = "日志索引不可用，正在查看保留的文件记录。" + diagnostics.CleanText(indexError) + warning
	matched := []contract.Entry{}
	for _, item := range items {
		if !deleted[item.ID] && !diagnostics.IsNoise(item) && Match(item, q) {
			matched = append(matched, item)
		}
	}
	result.Total = int64(len(matched))
	start := (page - 1) * size
	if start < len(matched) {
		end := start + size
		if end > len(matched) {
			end = len(matched)
		}
		result.Items = matched[start:end]
	}
	return result, nil
}
