package snapshot

// 文件职责：维护 V2 安装进度的进程内快照缓存和快照查询回退。
// 使用：event.Emit、event.RuntimeOnly 写入缓存，服务层 ListInstallProgressSnapshots 读取恢复页面临时状态。
// 边界：内存缓存只在当前进程有效；应用重启后的权威状态来自数据库 installed package 记录。

import (
	"sync"
	"time"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	v2Query "DysonSphereBox/helper/modDependence/installer/v2/query"
)

// store 职责：保存当前进程内最后一次安装进度快照。
// 使用：List 在没有持久化快照时读取，页面刷新后恢复运行时状态。
// 边界：仅进程内有效，应用重启后以数据库 installed package 记录为准。
var store = struct {
	sync.RWMutex
	// items 职责：按 packageKey 保存最后一次运行时进度快照；使用：Store 写入、List 回退读取；边界：应用重启后丢失。
	items map[string]contract.InstallProgressSnapshot
}{
	items: make(map[string]contract.InstallProgressSnapshot),
}

// List 返回本地表或当前进程内仍需前端展示的安装进度快照。
// 职责：优先返回数据库持久化进度，缺失时回退到当前进程内缓存。
// 使用：前端重新进入 Mod 页恢复安装、失败或冲突卡片状态时调用。
// 边界：只返回 running、failed 与 conflict，installed 由本地安装记录同步，不作为临时进度恢复。
func List() ([]contract.InstallProgressSnapshot, error) {
	persistedSnapshots, err := v2Query.ListPackageInstallProgressSnapshots()
	if err == nil && len(persistedSnapshots) > 0 {
		return persistedSnapshots, nil
	}

	store.RLock()
	defer store.RUnlock()

	snapshots := make([]contract.InstallProgressSnapshot, 0, len(store.items))
	for _, item := range store.items {
		if item.State != string(installConstant.StateRunning) && item.State != string(installConstant.StateFailed) && item.State != string(installConstant.StateConflict) {
			continue
		}
		snapshots = append(snapshots, item)
	}
	return snapshots, err
}

// Store 保存当前进程内的最后一次安装进度快照。
// 职责：把运行时事件载荷转换为前端可查询的 InstallProgressSnapshot。
// 使用：event.Emit、event.RuntimeOnly 和中断恢复流程调用。
// 边界：只写内存 map，不持久化到 SQLite；应用重启后会丢失。
func Store(data payload.Payload) {
	store.Lock()
	defer store.Unlock()

	store.items[data.PackageKey] = contract.InstallProgressSnapshot{
		PackageKey:     data.PackageKey,
		Stage:          data.Stage,
		Status:         data.Status,
		Percent:        data.Percent,
		Current:        data.Current,
		Total:          data.Total,
		BytesPerSecond: data.BytesPerSecond,
		Message:        data.Message,
		State:          data.State,
		ErrorMessage:   data.ErrorMessage,
		Conflict:       data.Conflict,
		UpdatedAt:      time.Now(),
	}
}
