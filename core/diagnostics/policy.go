package diagnostics

// 文件职责：识别已确认的框架噪声与可合并故障。使用：采集、历史索引清理和文件查询；边界：不按等级笼统丢弃业务事件或未知错误。

import (
	"DysonSphereBox/constant/logging"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// webViewReadyPattern 职责：精确识别 WebView2 的成功初始化输出；使用：标准日志过滤；边界：其它 WebView 错误与未知文本全部保留。
var webViewReadyPattern = regexp.MustCompile(`^(?:\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? )?\[WebView2\] Environment created successfully$`)

// IsNoise 职责：排除没有业务状态信息的自动输出；使用：实时采集和历史回放；边界：致命错误、业务失败和未识别的运行时错误始终保留。
func IsNoise(e logging.Entry) bool {
	if e.Level == logging.Fatal {
		return false
	}
	switch e.Source {
	case "runtime.wails":
		if e.Level == logging.Debug || e.Level == logging.Info {
			return true
		}
		message, _ := e.Data["message"].(string)
		switch strings.TrimSpace(message) {
		case "websocket: close sent", "Websocket write message failed websocket: close sent", "[ExternalAssetHandler] Proxy error: context canceled",
			"Unknown message from front end: runtime:ready", "process message error: runtime:ready -> Unknown message from front end: runtime:ready":
			// runtime:ready 是框架内置握手，桌面端正常处理；开发端误交给通用消息分发器时产生这两种固定输出。
			return true
		}
	case "runtime.go":
		message, _ := e.Data["message"].(string)
		return e.Level == logging.Info && webViewReadyPattern.MatchString(strings.TrimSpace(message))
	case "frontend.bridge":
		// 历史桥接“started/returned”仅表示函数被调用或 Promise 返回，不能证明业务成功。
		result, _ := e.Data["result"].(string)
		return e.Level == logging.Info && (result == "started" || result == "returned")
	case "backend.environment":
		return e.Namespace == "system.environment.loadOrInitGamePath" && e.Level == logging.Info && e.Data["source"] == "configured"
	}
	return false
}

// repeatKey 职责：给自动故障生成稳定指纹；使用：短时间合并；边界：用户操作、安装阶段、进程变化、Fatal 与不同错误不合并。
func repeatKey(e logging.Entry) string {
	if e.OperationID != "" || e.Handler == logging.User || (e.Level != logging.Warn && e.Level != logging.Error) {
		return ""
	}
	switch e.Source {
	case "runtime.wails", "runtime.go", "backend.http", "backend.database", "frontend.runtime", "frontend.bridge":
	default:
		return ""
	}
	data := make(map[string]interface{}, len(e.Data))
	for key, value := range e.Data {
		if key != "durationMs" {
			data[key] = value
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(e.Source+"\x00"+e.Namespace+"\x00"+e.Level+"\x00"+e.Action+"\x00"+e.Reason+"\x00"+string(encoded))))
}
