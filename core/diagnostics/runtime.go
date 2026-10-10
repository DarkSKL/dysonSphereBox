package diagnostics

// 文件职责：把 Wails、Go 标准日志和崩溃边界接入统一结构。使用：main 与生命周期；边界：不吞掉 panic，不将普通运行时文本推测为业务结果。

import (
	"DysonSphereBox/constant/logging"
	"fmt"
	"runtime/debug"
	"strings"
)

// RuntimeLogger 职责：实现 Wails 日志接口；使用：options.Logger；边界：不依赖桌面桥接或数据库。
type RuntimeLogger struct{}

// emit 职责：归一化运行时日志；使用：接口方法；边界：来源固定 runtime.wails。
func (RuntimeLogger) emit(level, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	Default.Record(logging.Entry{Action: "桌面运行时异常", Level: level, Source: "runtime.wails", Reason: message, Namespace: "system.runtime.message", Handler: logging.System, Data: map[string]interface{}{"message": message}})
}

// Print 职责：把普通运行时输出交给噪声策略；使用：Wails；边界：框架信息输出被过滤，不重定向文件句柄。
func (r RuntimeLogger) Print(v string) { r.emit(logging.Info, v) }

// Trace 职责：记录详细运行时输出；使用：Wails；边界：作为 Debug。
func (r RuntimeLogger) Trace(v string) { r.emit(logging.Debug, v) }

// Debug 职责：记录调试输出；使用：Wails；边界：不表示故障。
func (r RuntimeLogger) Debug(v string) { r.emit(logging.Debug, v) }

// Info 职责：记录运行时信息；使用：Wails；边界：不表示业务成功。
func (r RuntimeLogger) Info(v string) { r.emit(logging.Info, v) }

// Warning 职责：记录运行时告警；使用：Wails；边界：不强制退出。
func (r RuntimeLogger) Warning(v string) { r.emit(logging.Warn, v) }

// Error 职责：记录运行时错误；使用：Wails；边界：不覆盖原错误。
func (r RuntimeLogger) Error(v string) { r.emit(logging.Error, v) }

// Fatal 职责：记录致命运行时错误；使用：Wails；边界：退出决定留给框架。
func (r RuntimeLogger) Fatal(v string) { r.emit(logging.Fatal, v) }

// StandardWriter 职责：承接 Go 标准日志；使用：log.SetOutput；边界：不截获第三方 stdout。
type StandardWriter struct{}

// Write 职责：转存标准日志；使用：标准库 logger；边界：返回原长度以避免递归错误。
func (StandardWriter) Write(p []byte) (int, error) {
	Default.Record(logging.Entry{Action: "Go 标准日志", Level: logging.Info, Source: "runtime.go", Reason: "标准日志输出", Namespace: "system.runtime.standardLog", Handler: logging.System, Data: map[string]interface{}{"message": strings.TrimSpace(string(p))}})
	return len(p), nil
}

// CapturePanic 职责：在明确边界保留 panic 与堆栈；使用：main/startup 的 defer；边界：记录后重新 panic，不把损坏状态当正常运行。
func CapturePanic(source string) {
	if value := recover(); value != nil {
		Default.Record(logging.Entry{Action: "客户端发生程序异常", Level: logging.Fatal, Source: source, Reason: "运行边界捕获 panic", Namespace: "system.runtime.panic", Handler: logging.System, Data: map[string]interface{}{"result": "failed", "error": map[string]interface{}{"message": fmt.Sprint(value), "stack": string(debug.Stack())}}})
		panic(value)
	}
}
