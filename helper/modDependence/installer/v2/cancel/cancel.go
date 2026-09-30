package cancel

// 文件职责：维护 V2 远端安装任务的进程内取消注册表与取消检查。
// 使用：安装主流程在开始时调用 Register，下载、解压、复制等长耗时步骤调用 Check，服务取消入口调用 Cancel。
// 边界：只管理当前进程内的 context 与 done 信号，不持久化数据库状态，也不发送 Wails 进度事件。

import (
	"context"
	"fmt"
	"sync"
	"time"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/fileops"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	progressPayload "DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
)

// store 职责：保存当前进程内可取消安装任务的上下文。
// 使用：Register 写入，Cancel 根据 packageKey 读取并触发取消。
// 边界：不跨进程持久化；应用重启后的 running 记录由恢复流程处理。
var store = struct {
	sync.Mutex
	// items 职责：按 packageKey 保存当前进程内注册的安装任务；使用：Register 写入、Cancel 查询和 cleanup 删除；边界：不跨进程持久化。
	items map[string]registration
}{
	items: make(map[string]registration),
}

// registration 表示一个可取消安装任务的运行时句柄。
// 职责：聚合 context、取消函数和任务结束信号。
// 使用：安装开始时注册，取消或完成时释放。
// 边界：只在当前进程有效，不写入数据库或前端契约。
type registration struct {
	// ctx 职责：承载安装任务取消信号；使用：下载、复制等长耗时步骤检查；边界：不得跨任务复用。
	ctx context.Context
	// cancel 职责：触发安装任务取消；使用：Cancel 调用；边界：可多次调用但只生效一次。
	cancel context.CancelFunc
	// done 职责：通知安装任务已完成退出；使用：取消接口等待后台释放资源；边界：由安装任务结束时关闭。
	done chan struct{}
}

// Register 注册当前安装任务的取消句柄。
// 职责：为 packageKey 创建 context、cancel 函数和完成信号，并替换同包旧任务句柄。
// 使用：远端包安装开始时调用，返回的 cleanup 必须在任务退出时执行。
// 边界：同一 packageKey 的旧任务会先被取消；该注册只在当前进程有效。
func Register(packageKey string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	store.Lock()
	if previous, ok := store.items[packageKey]; ok {
		previous.cancel()
	}
	store.items[packageKey] = registration{ctx: ctx, cancel: cancel, done: done}
	store.Unlock()

	return ctx, func() {
		store.Lock()
		if current, ok := store.items[packageKey]; ok && current.ctx == ctx {
			delete(store.items, packageKey)
		}
		store.Unlock()
		close(done)
	}
}

// Check 检查安装上下文是否已经取消。
// 职责：把 context 取消统一转换成 fileops.ErrInstallCancelled。
// 使用：下载、复制、解压规划等长耗时步骤之间调用。
// 边界：ctx 为空时视为不可取消并返回 nil。
func Check(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if ctx.Err() != nil {
		return fileops.ErrInstallCancelled
	}
	return nil
}

// Cancel 取消当前进程内注册的安装任务并等待其释放资源。
// 职责：根据 packageKey 触发 cancel，并等待安装协程执行 cleanup。
// 使用：服务层取消安装入口调用；timeoutCallback 可用于在超时时推送运行时进度。
// 边界：只能取消当前进程内仍注册的任务；超时不会阻塞后台继续清理。
// 逻辑流：1. 查询当前任务句柄 => 2. 调用 cancel => 3. 等待 done 关闭 => 4. 超时执行回调并返回。
func Cancel(packageKey string, timeoutCallback func(error)) error {
	store.Lock()
	item, ok := store.items[packageKey]
	store.Unlock()
	if !ok {
		return fmt.Errorf("当前没有正在安装的整合包")
	}

	item.cancel()
	select {
	case <-item.done:
	case <-time.After(10 * time.Second):
		err := fmt.Errorf("用户取消安装，后台清理仍在进行")
		if timeoutCallback != nil {
			timeoutCallback(err)
		}
	}
	return nil
}

// CancelRemotePackageInstall 取消当前进程内正在执行的 Mod 包安装。
// 职责：根据 packageKey 触发运行中安装任务的 cancel，并在后台清理超时时推送运行时进度。
// 使用：InstallerService.CancelRemotePackageInstall 在前端请求取消安装时调用。
// 边界：只能取消当前进程内仍注册的安装；应用重启后的遗留 running 由恢复流程处理。
// 逻辑流：1. 查询当前任务句柄 => 2. 调用 cancel => 3. 等待 done 关闭 => 4. 超时则推送清理中进度并返回。
func CancelRemotePackageInstall(packageKey string) error {
	return Cancel(packageKey, func(err error) {
		progressEvent.RuntimeOnly(progressPayload.Payload{
			PackageKey:   packageKey,
			Stage:        "cleanup",
			Status:       "正在清理取消安装",
			Percent:      99,
			Message:      err.Error(),
			State:        string(installConstant.StateRunning),
			ErrorMessage: err.Error(),
		})
	})
}
