package system

// 文件职责：提供版本检查、下载准备与退出安装交接的 Wails 服务。
// 使用：更新入口下载新版，Wails 关闭钩子交接已经准备好的更新。
// 边界：准备期间不替换程序；实际安装、旧文件清理和重启由独立助手执行。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	systemConstant "DysonSphereBox/constant/system"
	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	appUpdater "DysonSphereBox/helper/updater"
	updaterApply "DysonSphereBox/helper/updater/apply"
	"DysonSphereBox/helper/updater/progress"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// UpdateService 职责：管理更新准备与退出交接；使用：由系统服务组注册一次；边界：互斥保护同一程序的更新任务，注册时不读写数据库。
type UpdateService struct {
	// mu 职责：串行化下载和退出交接；使用：各更新操作尝试加锁；边界：忙碌时返回明确错误，不重复下载。
	mu sync.Mutex
	// prepared 职责：持有校验后的安装文件；使用：关闭钩子交接；边界：为空时正常退出。
	prepared *updaterApply.PreparedUpdate
	// result 职责：复用本次准备结果；使用：重复调用返回同一结果；边界：不表示新版已经运行。
	result *appUpdater.UpdateResult
	// handedOff 职责：避免重复启动更新助手；使用：交接成功后置位；边界：失败时允许重试。
	handedOff bool
}

// GetAppVersion 职责：返回构建版本；使用：版本展示；边界：不访问远端。
func (service *UpdateService) GetAppVersion() string {
	return core.Version
}

// CheckAppUpdate 职责：检查最新 Release；使用：设置页和标题栏；边界：只检查，网络失败记录后返回。
func (service *UpdateService) CheckAppUpdate() (*appUpdater.CheckResult, error) {
	result, err := appUpdater.CheckAppUpdate(context.Background())
	if err != nil {
		systemHelper.RecordUpdateCheckFailure(core.Version, err)
		return nil, err
	}
	return result, nil
}

// UpdateAppSelf 职责：下载校验并暂存新版；使用：前端确认下载后调用；边界：当前包体保持不变，重复调用复用准备结果。
// 逻辑流：1. 防止重复任务 => 2. 下载并广播进度 => 3. 保存暂存结果 => 4. 记录准备完成，等待用户退出。
func (service *UpdateService) UpdateAppSelf() (*appUpdater.UpdateResult, error) {
	if !service.mu.TryLock() {
		return nil, appUpdater.ErrUpdateInProgress
	}
	defer service.mu.Unlock()
	if service.prepared != nil {
		return service.result, nil
	}
	result, prepared, err := appUpdater.PrepareAppUpdate(context.Background(), func(snapshot progress.Snapshot) {
		if core.GLOBAL.RuntimeContext != nil {
			runtime.EventsEmit(core.GLOBAL.RuntimeContext, systemConstant.AppUpdateProgressEvent, snapshot)
		}
	})
	if err != nil {
		systemHelper.RecordUpdateSelfFailure(core.Version, err)
		return nil, err
	}
	service.result, service.prepared = result, prepared
	if result.Prepared {
		systemHelper.RecordUpdatePrepared(core.Version, result)
	}
	return result, nil
}

// PrepareAppExitUpdate 职责：在关闭前交接已准备的更新；使用：App.beforeClose 统一覆盖标题栏、退出按钮和系统关闭；边界：下载中阻止退出；助手交接失败取消本轮安装并返回提示，允许正常退出。
// 逻辑流：1. 检查任务状态 => 2. 启动等待旧进程的助手 => 3. 确认交接后允许正常关闭。
func (service *UpdateService) PrepareAppExitUpdate() error {
	if !service.mu.TryLock() {
		return appUpdater.ErrUpdateInProgress
	}
	defer service.mu.Unlock()
	if service.prepared == nil || service.handedOff {
		return nil
	}
	if err := service.prepared.StartAfterExit(os.Getpid()); err != nil {
		cleanupErr := service.prepared.Discard()
		service.prepared, service.result = nil, nil
		err = fmt.Errorf("更新助手未能启动，本次退出不安装更新，当前版本保持不变：%w", errors.Join(err, cleanupErr))
		systemHelper.RecordUpdateSelfFailure(core.Version, err)
		return err
	}
	service.handedOff = true
	systemHelper.RecordUpdateExit(core.Version, service.result)
	return nil
}
