package modDependence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const installProgressEventName = "modDependence:installProgress"

const (
	installProgressStateRunning     = "running"
	installProgressStateDone        = "done"
	installProgressStateFailed      = "failed"
	installProgressStateInstalled   = "installed"
	installProgressStateUninstalled = "uninstalled"
	installProgressStateConflict    = "conflict"
)

// InstallProgressSnapshot 表示 Mod 安装器最近一次进度快照。
// 用途：前端页面刷新或重新激活后恢复正在安装、失败等临时状态。
// 边界：该快照只保存在当前进程内，应用重启后以 SQLite 安装记录为准。
type InstallProgressSnapshot struct {
	PackageKey     string              `json:"packageKey"`
	Stage          string              `json:"stage"`
	Status         string              `json:"status"`
	Percent        int                 `json:"percent"`
	Current        int64               `json:"current"`
	Total          int64               `json:"total"`
	BytesPerSecond int64               `json:"bytesPerSecond"`
	Message        string              `json:"message"`
	State          string              `json:"state"`
	ErrorMessage   string              `json:"errorMessage"`
	Conflict       *InstallConflictDTO `json:"conflict,omitempty"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

type installProgressPayload struct {
	PackageKey     string              `json:"packageKey"`
	Stage          string              `json:"stage"`
	Status         string              `json:"status"`
	Percent        int                 `json:"percent"`
	Current        int64               `json:"current"`
	Total          int64               `json:"total"`
	BytesPerSecond int64               `json:"bytesPerSecond"`
	Message        string              `json:"message"`
	State          string              `json:"state"`
	ErrorMessage   string              `json:"errorMessage"`
	Conflict       *InstallConflictDTO `json:"conflict,omitempty"`
}

var errInstallCancelled = errors.New("安装已取消")

var installProgressStore = struct {
	sync.RWMutex
	items map[string]InstallProgressSnapshot
}{
	items: make(map[string]InstallProgressSnapshot),
}

var installCancelStore = struct {
	sync.Mutex
	items map[string]installCancellation
}{
	items: make(map[string]installCancellation),
}

type installCancellation struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

type progressReader struct {
	reader      io.Reader
	ctx         context.Context
	packageKey  string
	stage       string
	status      string
	total       int64
	current     int64
	start       int
	end         int
	lastPercent int
	lastEmitAt  time.Time
	startedAt   time.Time
	mutex       sync.Mutex
}

func newProgressReader(ctx context.Context, packageKey string, stage string, status string, reader io.Reader, total int64, start int, end int) *progressReader {
	return &progressReader{
		ctx:         ctx,
		reader:      reader,
		packageKey:  packageKey,
		stage:       stage,
		status:      status,
		total:       total,
		start:       start,
		end:         end,
		startedAt:   time.Now(),
		lastPercent: -1,
	}
}

func (reader *progressReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, errInstallCancelled
	}
	count, err := reader.reader.Read(buffer)
	if err != nil && reader.ctx.Err() != nil {
		err = errInstallCancelled
	}
	if count > 0 {
		reader.mutex.Lock()
		reader.current += int64(count)
		current := reader.current
		total := reader.total
		percent := mapProgress(reader.start, reader.end, progressPercent(current, total))
		bytesPerSecond := reader.bytesPerSecond(current)
		if percent != reader.lastPercent || time.Since(reader.lastEmitAt) >= time.Second {
			reader.lastPercent = percent
			reader.lastEmitAt = time.Now()
			emitInstallProgress(installProgressPayload{
				PackageKey:     reader.packageKey,
				Stage:          reader.stage,
				Status:         reader.status,
				Percent:        percent,
				Current:        current,
				Total:          total,
				BytesPerSecond: bytesPerSecond,
				Message:        reader.status,
			})
		}
		reader.mutex.Unlock()
	}
	return count, err
}

func (reader *progressReader) bytesPerSecond(current int64) int64 {
	elapsed := time.Since(reader.startedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(current) / elapsed)
}

func emitInstallProgress(payload installProgressPayload) {
	if payload.PackageKey == "" {
		return
	}
	payload.Percent = clampProgress(payload.Percent)
	if payload.State == "" {
		payload.State = installProgressStateRunning
	}
	storeInstallProgressSnapshot(payload)
	updatePackageInstallProgress(payload)
	if core.GLOBAL.RuntimeContext == nil {
		return
	}
	runtime.EventsEmit(core.GLOBAL.RuntimeContext, installProgressEventName, payload)
}

func emitRuntimeOnlyProgress(payload installProgressPayload) {
	if payload.PackageKey == "" {
		return
	}
	payload.Percent = clampProgress(payload.Percent)
	if payload.State == "" {
		payload.State = installProgressStateRunning
	}
	storeInstallProgressSnapshot(payload)
	if core.GLOBAL.RuntimeContext == nil {
		return
	}
	runtime.EventsEmit(core.GLOBAL.RuntimeContext, installProgressEventName, payload)
}

func emitPackageProgress(packageKey string, stage string, status string, percent int) {
	state := installProgressStateRunning
	if stage == "done" {
		state = installProgressStateInstalled
	}
	emitInstallProgress(installProgressPayload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    clampProgress(percent),
		Message:    status,
		State:      state,
	})
}

func emitPackageProgressWithState(packageKey string, stage string, status string, percent int, state string) {
	emitInstallProgress(installProgressPayload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    clampProgress(percent),
		Message:    status,
		State:      state,
	})
}

func emitUninstallProgress(packageKey string, stage string, status string, percent int) {
	state := installProgressStateRunning
	if stage == "done" {
		state = installProgressStateUninstalled
	}
	// 卸载会删除安装包记录，进度只需要通知前端；避免 SQLite 写锁与卸载事务互相等待。
	emitRuntimeOnlyProgress(installProgressPayload{
		PackageKey: packageKey,
		Stage:      stage,
		Status:     status,
		Percent:    clampProgress(percent),
		Message:    status,
		State:      state,
	})
}

// MarkPackageProgressFailure 将指定包的最近进度标记为失败并通知前端。
// 使用：Wails 服务捕获安装器错误后调用，避免页面刷新后丢失失败状态。
func MarkPackageProgressFailure(packageKey string, status string, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	emitInstallProgress(installProgressPayload{
		PackageKey:   packageKey,
		Stage:        "failed",
		Status:       status,
		Percent:      100,
		Message:      errorMessage,
		State:        installProgressStateFailed,
		ErrorMessage: errorMessage,
	})
}

func registerInstallCancellation(packageKey string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	installCancelStore.Lock()
	if previous, ok := installCancelStore.items[packageKey]; ok {
		previous.cancel()
	}
	installCancelStore.items[packageKey] = installCancellation{ctx: ctx, cancel: cancel, done: done}
	installCancelStore.Unlock()

	return ctx, func() {
		installCancelStore.Lock()
		if current, ok := installCancelStore.items[packageKey]; ok && current.ctx == ctx {
			delete(installCancelStore.items, packageKey)
		}
		installCancelStore.Unlock()
		close(done)
	}
}

func checkInstallCancelled(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if ctx.Err() != nil {
		return errInstallCancelled
	}
	return nil
}

// CancelRemoteModPackageInstall 取消当前进程内正在执行的 Thunderstore 安装。
// 用途：下载进度长时间不动时，前端可立即取消后台请求并释放卡片操作状态。
// 边界：只能取消当前进程内仍注册的安装；应用重启后的遗留 running 由 RecoverInterruptedRemotePackageInstalls 处理。
func CancelRemoteModPackageInstall(packageKey string) error {
	installCancelStore.Lock()
	item, ok := installCancelStore.items[packageKey]
	installCancelStore.Unlock()
	if !ok {
		return fmt.Errorf("当前没有正在安装的整合包")
	}

	item.cancel()
	select {
	case <-item.done:
	case <-time.After(10 * time.Second):
		err := fmt.Errorf("用户取消安装，后台清理仍在进行")
		emitRuntimeOnlyProgress(installProgressPayload{
			PackageKey:   packageKey,
			Stage:        "cleanup",
			Status:       "正在清理取消安装",
			Percent:      99,
			Message:      err.Error(),
			State:        installProgressStateRunning,
			ErrorMessage: err.Error(),
		})
		return nil
	}
	return nil
}

// RecoverInterruptedRemotePackageInstalls 将上次进程遗留的安装中记录恢复为失败。
// 用途：应用异常退出或用户关闭程序后，清理已经落库的部分安装文件，再把卡片恢复为失败状态。
// 边界：只处理 running 状态；conflict 会保留给用户继续选择覆盖、跳过或失败。
func RecoverInterruptedRemotePackageInstalls() error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	records := make([]modDependenceModel.ModInstalledPackage, 0)
	if err := core.GLOBAL.DB.
		Where("install_state = ?", installProgressStateRunning).
		Find(&records).Error; err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	errorMessage := "上次安装在程序退出时中断，已清理未完成安装文件，请重新安装该整合包。"
	for _, record := range records {
		if err := cleanupInterruptedRemotePackageInstall(record.PackageKey, "安装已中断"); err != nil {
			return err
		}
		storeInstallProgressSnapshot(installProgressPayload{
			PackageKey:   record.PackageKey,
			Stage:        "failed",
			Status:       "安装已中断",
			Percent:      clampProgress(record.InstallProgress),
			Message:      errorMessage,
			State:        installProgressStateFailed,
			ErrorMessage: errorMessage,
		})
	}
	return nil
}

// ListInstallProgressSnapshots 返回本地表或当前进程内仍需前端展示的安装进度快照。
// 边界：只返回 running、failed 与 conflict，installed 由本地安装记录同步，不作为临时进度恢复。
func ListInstallProgressSnapshots() ([]InstallProgressSnapshot, error) {
	persistedSnapshots, err := ListPackageInstallProgressSnapshots()
	if err == nil && len(persistedSnapshots) > 0 {
		return persistedSnapshots, nil
	}

	installProgressStore.RLock()
	defer installProgressStore.RUnlock()

	snapshots := make([]InstallProgressSnapshot, 0, len(installProgressStore.items))
	for _, snapshot := range installProgressStore.items {
		if snapshot.State != installProgressStateRunning && snapshot.State != installProgressStateFailed && snapshot.State != installProgressStateConflict {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, err
}

func storeInstallProgressSnapshot(payload installProgressPayload) {
	installProgressStore.Lock()
	defer installProgressStore.Unlock()

	installProgressStore.items[payload.PackageKey] = InstallProgressSnapshot{
		PackageKey:     payload.PackageKey,
		Stage:          payload.Stage,
		Status:         payload.Status,
		Percent:        payload.Percent,
		Current:        payload.Current,
		Total:          payload.Total,
		BytesPerSecond: payload.BytesPerSecond,
		Message:        payload.Message,
		State:          payload.State,
		ErrorMessage:   payload.ErrorMessage,
		Conflict:       payload.Conflict,
		UpdatedAt:      time.Now(),
	}
}

func progressPercent(current int64, total int64) int {
	if total <= 0 {
		return 0
	}
	return clampProgress(int((current * 100) / total))
}

func mapProgress(start int, end int, percent int) int {
	start = clampProgress(start)
	end = clampProgress(end)
	if end < start {
		end = start
	}
	return clampProgress(start + (end-start)*clampProgress(percent)/100)
}

func clampProgress(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
