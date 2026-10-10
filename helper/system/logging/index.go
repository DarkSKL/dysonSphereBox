package logging

// 文件职责：异步维护统一日志数据库索引。使用：启动后挂接文件采集器；边界：有界队列、不能从业务事务内同步写同一个 SQLite。

import (
	contract "DysonSphereBox/constant/logging"
	"DysonSphereBox/core"
	"DysonSphereBox/core/diagnostics"
	systemModel "DysonSphereBox/model/system"
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"sync"
	"time"
)

// Index 职责：维护有界异步写队列和状态；使用：应用生命周期；边界：写失败仍可从文件恢复。
type Index struct {
	db        *gorm.DB
	queue     chan contract.Entry
	done      chan struct{}
	mu        sync.Mutex
	lastError string
	dropped   int64
}

// active 职责：当前索引；使用：健康查询和退出；边界：启动后只挂接一次。
var active *Index

// Start 职责：回放文件并启动数据库索引；使用：数据库初始化后；边界：先挂接队列，回放与实时记录通过主键去重。
func Start(db *gorm.DB) error {
	idx := &Index{db: db.Session(&gorm.Session{Logger: logger.Discard}), queue: make(chan contract.Entry, 1024), done: make(chan struct{})}
	active = idx
	diagnostics.Default.SetSink(func(e contract.Entry) {
		select {
		case idx.queue <- e:
		default:
			idx.mu.Lock()
			idx.dropped++
			idx.lastError = "日志索引队列已满，证据仍在 JSONL 文件中"
			idx.mu.Unlock()
		}
	})
	go idx.run()
	entries, warning := diagnostics.Default.ReadEntries()
	if warning != "" {
		idx.setError(warning)
	}
	for _, e := range entries {
		if err := idx.save(e); err != nil {
			idx.setError(err.Error())
			return err
		}
	}
	return nil
}

// run 职责：顺序写入索引；使用：后台队列；边界：不会递归记录数据库错误。
func (idx *Index) run() {
	defer close(idx.done)
	for e := range idx.queue {
		if err := idx.save(e); err != nil {
			idx.setError(err.Error())
		} else if core.GLOBAL.RuntimeContext != nil {
			runtime.EventsEmit(core.GLOBAL.RuntimeContext, contract.ChangedEvent)
		}
	}
}

// save 职责：按 ID 幂等入库；使用：回放和实时写入；边界：错误不会抹掉文件证据。
func (idx *Index) save(e contract.Entry) error {
	when, err := time.Parse(time.RFC3339Nano, e.Time)
	if err != nil {
		return err
	}
	data, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	row := systemModel.LogEntry{ID: e.ID, SchemaVersion: e.SchemaVersion, Action: e.Action, Level: e.Level, Source: e.Source, Reason: e.Reason, Namespace: e.Namespace, Data: datatypes.JSON(data), EventTime: when, Handler: e.Handler, SessionID: e.SessionID, OperationID: e.OperationID, AppVersion: e.AppVersion}
	return idx.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// setError 职责：保存索引异常；使用：队列和回放；边界：错误文本统一脱敏。
func (idx *Index) setError(value string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.lastError = diagnostics.CleanText(value)
}

// Health 职责：返回索引状态；使用：前端健康检查；边界：启动前明确显示未初始化。
func Health() (string, int) {
	if active == nil {
		return "数据库日志索引尚未初始化", 0
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	return active.lastError, len(active.queue)
}

// Stop 职责：停止接收并排空索引；使用：关闭数据库之前；边界：所有生产者经 SetSink 串行切断，不关闭正在写入的队列。
func Stop() {
	diagnostics.Default.SetSink(nil)
	if active == nil {
		return
	}
	close(active.queue)
	select {
	case <-active.done:
	case <-time.After(5 * time.Second):
		active.setError("退出时日志索引刷新超时，证据保留在文件")
	}
}

// MigrateLegacy 职责：幂等转换旧日志到新索引；使用：启动迁移；边界：原因和触发者无法还原时明确 Unknown，旧表保留。
func MigrateLegacy(db *gorm.DB) error {
	var lastID uint
	for {
		items := []systemModel.Log{}
		if err := db.Where("id > ?", lastID).Order("id ASC").Limit(200).Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for _, item := range items {
			data := map[string]interface{}{}
			if err := json.Unmarshal(item.Metadata, &data); err != nil {
				data["legacyMetadataError"] = err.Error()
			}
			if data == nil {
				data = map[string]interface{}{}
			}
			data["legacyID"], data["legacyType"] = item.ID, item.Type
			if item.Error != "" {
				data["error"] = map[string]interface{}{"message": item.Error}
			}
			level := contract.Info
			switch item.Level {
			case systemModel.LogLevelDebug:
				level = contract.Debug
			case systemModel.LogLevelWarn:
				level = contract.Warn
			case systemModel.LogLevelError:
				level = contract.Error
			}
			when := time.Unix(0, 0).UTC()
			if item.DateTime != nil {
				when = *item.DateTime
			} else {
				data["legacyTimeUnknown"] = true
			}
			action := item.Content
			if action == "" {
				action = "历史日志（未记录行为描述）"
			}
			namespace := item.Namespace + "." + item.Module + "." + item.Event
			entry := contract.Entry{ID: fmt.Sprintf("legacy-%d", item.ID), SchemaVersion: 2, Action: diagnostics.CleanText(action), Level: level, Source: "legacy." + item.Module, Reason: "历史记录未保存触发原因，无法还原", Namespace: namespace, Data: diagnostics.CleanData(data), Time: when.Format(time.RFC3339Nano), Handler: contract.Unknown, SessionID: "legacy", AppVersion: "unknown"}
			if err := (&Index{db: db}).save(entry); err != nil {
				return err
			}
			lastID = item.ID
		}
	}
}
