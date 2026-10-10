package logging

// 文件职责：持久化单条日志删除与文件降级过滤。使用：LogService、列表和导出；边界：保留原始 JSONL，数据库不可用时拒绝删除。

import (
	"DysonSphereBox/core/diagnostics"
	systemModel "DysonSphereBox/model/system"
	"errors"
	"fmt"
	"gorm.io/gorm"
)

// Delete 职责：软删除指定日志；使用：服务校验 ID 后调用；边界：支持尚未索引的文件记录，重复删除幂等，不删除其它事件。
// 逻辑流：1. 事务查找索引 => 2. 缺失时从文件补入 => 3. 保存删除标记；失败回滚索引变更。
func Delete(db *gorm.DB, id string) error {
	if db == nil {
		return fmt.Errorf("日志数据库尚未就绪，无法删除日志")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var row systemModel.LogEntry
		err := tx.Unscoped().Where("id = ?", id).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			entries, _ := diagnostics.Default.ReadEntries()
			found := false
			for _, entry := range entries {
				if entry.ID == id {
					if err := (&Index{db: tx}).save(entry); err != nil {
						return err
					}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("日志不存在或已过期")
			}
		} else if err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&systemModel.LogEntry{}).Error
	})
}

// deletedIDs 职责：读取文件检索须排除的日志 ID；使用：文件降级和诊断导出；边界：无索引时为空，读取失败显式报错，避免重新展示已删除记录。
func deletedIDs(db *gorm.DB) (map[string]bool, error) {
	if db == nil && active != nil {
		db = active.db
	}
	ids := map[string]bool{}
	if db == nil {
		return ids, nil
	}
	var values []string
	if err := db.Unscoped().Model(&systemModel.LogEntry{}).Where("deleted_at IS NOT NULL").Pluck("id", &values).Error; err != nil {
		return nil, fmt.Errorf("无法读取日志删除状态: %w", err)
	}
	for _, id := range values {
		ids[id] = true
	}
	return ids, nil
}
