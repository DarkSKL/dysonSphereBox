package diagnostics

// 文件职责：记录会话是否正常收束。使用：启动与正常退出；边界：仅把遗留标记解释为未确认正常结束，不能推断崩溃或断电根因。

import (
	"DysonSphereBox/constant/logging"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// BeginSession 职责：检查上次退出证据并创建当前会话标记；使用：文件初始化后；边界：只处理自身固定命名的标记，遗留证据不反复报告。
func (s *Store) BeginSession() {
	health := s.Health()
	if health.Directory == "" {
		return
	}
	paths, err := filepath.Glob(filepath.Join(health.Directory, "session-*.pending"))
	if err != nil {
		return
	}
	for _, path := range paths {
		data := map[string]interface{}{}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err = json.Unmarshal(content, &data); err != nil {
			continue
		}
		s.Record(logging.Entry{Action: "检测到未确认正常结束的客户端会话", Level: logging.Warn, Source: "backend.lifecycle", Reason: "遗留运行标记存在，无法区分强制终止、崩溃或断电", Namespace: "system.lifecycle.uncleanSession", Handler: logging.System, Data: map[string]interface{}{"previousSession": data, "result": "unknown"}})
		if err := os.Remove(path); err != nil {
			s.RecordEvidence(logging.Entry{Action: "清理会话标记失败", Level: logging.Warn, Source: "backend.lifecycle", Reason: "删除已记录的历史运行标记返回错误", Namespace: "system.lifecycle.markerFailed", Handler: logging.System, Data: map[string]interface{}{"error": ErrorData(err)}})
		}
	}
	content, _ := json.Marshal(map[string]interface{}{"sessionId": health.SessionID, "startedAt": time.Now().Format(time.RFC3339Nano), "pid": os.Getpid()})
	if err := os.WriteFile(filepath.Join(health.Directory, "session-"+health.SessionID+".pending"), content, 0600); err != nil {
		s.RecordEvidence(logging.Entry{Action: "创建会话标记失败", Level: logging.Warn, Source: "backend.lifecycle", Reason: "无法保存本次运行标记", Namespace: "system.lifecycle.markerFailed", Handler: logging.System, Data: map[string]interface{}{"error": ErrorData(err)}})
	}
}

// EndSession 职责：确认正常退出后移除本次标记；使用：Wails shutdown 完成后；边界：main 异常退出不调用。
func (s *Store) EndSession() {
	health := s.Health()
	if health.Directory == "" {
		return
	}
	path := filepath.Join(health.Directory, "session-"+health.SessionID+".pending")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		s.RecordEvidence(logging.Entry{Action: "结束会话标记失败", Level: logging.Warn, Source: "backend.lifecycle", Reason: "正常退出标记删除返回错误", Namespace: "system.lifecycle.markerFailed", Handler: logging.System, Data: map[string]interface{}{"error": ErrorData(err)}})
	}
}
