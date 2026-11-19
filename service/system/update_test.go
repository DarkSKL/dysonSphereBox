package system

// 文件职责：验证更新任务的重复操作与关闭时机。
// 使用：go test ./service/system；边界：不访问网络、数据库或真实桌面。

import (
	appUpdater "DysonSphereBox/helper/updater"
	updaterApply "DysonSphereBox/helper/updater/apply"
	"testing"
)

// TestUpdateServiceLifecycle 职责：验证空闲关闭、下载中禁止关闭及复用已准备结果；使用：更新服务回归；边界：已准备文件交接由助手集成测试覆盖。
func TestUpdateServiceLifecycle(t *testing.T) {
	service := &UpdateService{}
	if err := service.PrepareAppExitUpdate(); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	_, updateErr := service.UpdateAppSelf()
	exitErr := service.PrepareAppExitUpdate()
	service.mu.Unlock()
	if updateErr == nil || exitErr == nil {
		t.Fatal("busy update must reject duplicate downloads and exit")
	}
	expected := &appUpdater.UpdateResult{Prepared: true, TargetVersion: "v2.0.0"}
	service.prepared, service.result = &updaterApply.PreparedUpdate{}, expected
	actual, err := service.UpdateAppSelf()
	if err != nil || actual != expected {
		t.Fatalf("prepared result was not reused: %v", err)
	}
	service.handedOff = true
	if err := service.PrepareAppExitUpdate(); err != nil {
		t.Fatal("repeated close started another handoff", err)
	}
}
