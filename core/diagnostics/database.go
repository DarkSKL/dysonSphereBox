package diagnostics

// 文件职责：把数据库错误与慢查询接入诊断。使用：GORM 配置；边界：不记录 SQL 和参数，避免密码 Token 与业务正文泄漏。

import (
	"DysonSphereBox/constant/logging"
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// DatabaseLogger 职责：实现 GORM 日志接口；使用：数据库初始化；边界：只写文件，不递归使用数据库。
type DatabaseLogger struct{}

// LogMode 职责：保持错误采集；使用：GORM Session；边界：不打开 SQL 明文记录。
func (d DatabaseLogger) LogMode(logger.LogLevel) logger.Interface { return d }

// Info 职责：忽略 SQL 常规输出；使用：GORM；边界：成功业务由服务层记录。
func (DatabaseLogger) Info(context.Context, string, ...interface{}) {}

// Warn 职责：忽略带参数的格式化输出；使用：GORM；边界：Trace 捕获慢查询。
func (DatabaseLogger) Warn(context.Context, string, ...interface{}) {}

// Error 职责：记录数据库基础设施错误类别；使用：GORM；边界：不格式化潜在敏感参数。
func (DatabaseLogger) Error(_ context.Context, _ string, _ ...interface{}) {
	Default.RecordEvidence(logging.Entry{Action: "数据库内部错误", Level: logging.Error, Source: "backend.database", Reason: "GORM 报告基础设施错误", Namespace: "system.database.internalError", Handler: logging.System, Data: map[string]interface{}{"result": "failed"}})
}

// Trace 职责：记录查询失败与慢查询；使用：GORM 操作回调；边界：不调用返回 SQL 的 fc，忽略正常未找到记录。
func (DatabaseLogger) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	elapsed := time.Since(begin)
	if err == nil && elapsed < time.Second {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	level, event, reason := logging.Warn, "slowQuery", "数据库操作超过一秒"
	data := map[string]interface{}{"durationMs": elapsed.Milliseconds()}
	if err != nil {
		level, event, reason = logging.Error, "queryFailed", "数据库操作返回错误"
		data["error"] = ErrorData(err)
		data["result"] = "failed"
	}
	Default.RecordEvidence(logging.Entry{Action: "数据库操作", Level: level, Source: "backend.database", Reason: reason, Namespace: "system.database." + event, Handler: logging.System, Data: data})
}
