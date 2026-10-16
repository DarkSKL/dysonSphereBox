package modDependence

// 文件职责：提供开服工作台的日志读取、导出和存档候选列表绑定。
// 使用：开服页通过 HeadlessService 获取只读日志与存档联想。
// 边界：不提供游戏内踢人或延迟管理，不修改存档，选择取消不落盘。

import (
	"fmt"
	"os"
	"time"

	"DysonSphereBox/core"
	headlessAudit "DysonSphereBox/helper/modDependence/audit/headless"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/headless"
	"DysonSphereBox/helper/modDependence/headless/console"
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ReadServerLog 职责：获取固定日志尾部；使用：开服页自动刷新；边界：只允许 bepInEx/unity 来源，无文件返回空态，不产生轮询审计噪音。
func (service *HeadlessService) ReadServerLog(source string) (*headlessContract.HeadlessLogSnapshot, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	return console.ReadLog(gameDir, source)
}

// ExportServerLog 职责：导出完整源日志；使用：开服页下载按钮；边界：用户在原生对话框确认目标，取消返回空串，禁止覆盖源日志。
func (service *HeadlessService) ExportServerLog(source string) (string, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return "", err
	}
	path, err := console.LogPath(gameDir, source)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("日志尚未生成或不可读取: %w", err)
	}
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("桌面运行时尚未初始化")
	}
	target, err := runtime.SaveFileDialog(core.GLOBAL.RuntimeContext, runtime.SaveDialogOptions{
		Title: "导出服务器日志", DefaultFilename: "server-" + source + "-" + time.Now().Format("20060102-150405") + ".log",
		Filters: []runtime.FileFilter{{DisplayName: "日志文件 (*.log)", Pattern: "*.log"}},
	})
	if err != nil || target == "" {
		return "", err
	}
	if err := console.ExportLog(gameDir, source, target); err != nil {
		headlessAudit.RecordHeadlessFailure("exportLog", "导出服务器日志失败", err)
		return "", err
	}
	return target, nil
}

// ListServerArchives 职责：列出可选启动存档；使用：开服配置的存档输入联想；边界：只读 SaveRoot 直属 .dsv，不导入外部存档，不自动保存配置。
func (service *HeadlessService) ListServerArchives() ([]headlessContract.HeadlessArchiveOption, error) {
	return headless.ListHeadlessArchives()
}
