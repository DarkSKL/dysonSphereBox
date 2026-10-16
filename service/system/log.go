package system

// 文件职责：暴露统一日志查询、前端采集、健康与诊断导出。使用：日志页面及前端异常边界；边界：不向外部服务器上传，不删除原始证据。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core"
	"DysonSphereBox/core/diagnostics"
	systemHelper "DysonSphereBox/helper/system"
	logStore "DysonSphereBox/helper/system/logging"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"strings"
)

// LogService 职责：提供统一诊断日志 API；使用：前端日志模块；边界：服务只编排 helper 和运行时对话框。
type LogService struct{}

// ListLogs 职责：筛选分页日志；使用：日志页与时间线；边界：数据库不可用时明确返回文件降级告警。
func (service *LogService) ListLogs(query systemHelper.LogQuery) (systemHelper.LogListResult, error) {
	return logStore.List(core.GLOBAL.DB, query)
}

// DeleteLog 职责：删除用户指定的单条日志；使用：日志页右键菜单；边界：校验 ID，持久化失败不报告成功，日志管理操作本身不产生日志。
func (service *LogService) DeleteLog(id string) error {
	if strings.TrimSpace(id) == "" || len(id) > 40 || strings.TrimSpace(id) != id {
		return fmt.Errorf("日志标识无效")
	}
	if err := logStore.Delete(core.GLOBAL.DB, id); err != nil {
		return err
	}
	if core.GLOBAL.RuntimeContext != nil {
		runtime.EventsEmit(core.GLOBAL.RuntimeContext, logging.ChangedEvent)
	}
	return nil
}

// CreateLog 职责：采集前端事件；使用：异常捕获和桥接边界；边界：强制 frontend 来源及本机会话，拒绝无效结构和过量数据。
func (service *LogService) CreateLog(entry logging.Entry) error {
	if !strings.HasPrefix(entry.Source, "frontend.") {
		return fmt.Errorf("前端日志来源必须以 frontend. 开头")
	}
	entry.ID, entry.SessionID, entry.AppVersion = "", "", ""
	return diagnostics.Default.Write(entry)
}

// GetLogHealth 职责：查询采集健康和实际路径；使用：日志页及诊断导出；边界：索引未初始化明确显示。
func (service *LogService) GetLogHealth() systemHelper.LogHealth {
	err, pending := logStore.Health()
	return systemHelper.LogHealth{File: diagnostics.Default.Health(), IndexError: err, Pending: pending}
}

// OpenLogDirectory 职责：打开实际日志目录；使用：日志页工具栏；边界：无运行时或目录时拒绝。
func (service *LogService) OpenLogDirectory() error {
	dir := diagnostics.Default.Health().Directory
	if core.GLOBAL.RuntimeContext == nil || dir == "" {
		return fmt.Errorf("日志目录或桌面运行时尚未就绪")
	}
	return logStore.OpenDirectory(dir)
}

// ExportDiagnostics 职责：导出脱敏诊断包；使用：用户报错时；边界：取消不写文件，拒绝覆盖盒子内部数据。
func (service *LogService) ExportDiagnostics(query systemHelper.LogQuery) (string, error) {
	if err := logStore.ValidateQuery(query); err != nil {
		return "", err
	}
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("桌面运行时尚未初始化")
	}
	target, err := runtime.SaveFileDialog(core.GLOBAL.RuntimeContext, runtime.SaveDialogOptions{Title: "导出诊断包", DefaultFilename: "dyson-sphere-box-diagnostics.zip", Filters: []runtime.FileFilter{{DisplayName: "诊断包 (*.zip)", Pattern: "*.zip"}}})
	if err != nil || target == "" {
		return "", err
	}
	if err := logStore.Export(core.GLOBAL.DB, target, query, service.GetLogHealth()); err != nil {
		return "", err
	}
	return target, nil
}
