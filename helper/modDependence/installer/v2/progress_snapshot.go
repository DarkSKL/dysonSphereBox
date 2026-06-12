package v2

// 文件职责：封装安装进度状态、事件或快照模块 progress_snapshot。
// 使用：由安装器在下载、复制、记录和卸载阶段上报或恢复进度时调用。
// 边界：不执行安装业务本身，不直接决定冲突处理策略。

import (
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	progressPayload "DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	progressSnapshot "DysonSphereBox/helper/modDependence/installer/v2/progress/snapshot"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"fmt"
)

// RecoverInterruptedRemotePackageInstalls 将上次进程遗留的安装中记录恢复为失败。
// 职责：应用异常退出或用户关闭程序后，清理已经落库的部分安装文件，再把卡片恢复为失败状态。
// 使用：App.startup 在数据库初始化和历史路径迁移后调用。
// 边界：只处理 running 状态；conflict 会保留给用户继续选择覆盖、跳过或失败。
// 逻辑流：1. 查询 running 安装记录 => 2. 调用会话清理移除未完成文件 => 3. 写入 failed 运行时快照 => 4. 返回恢复结果给启动流程。
func RecoverInterruptedRemotePackageInstalls() error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	records := make([]modDependenceModel.ModInstalledPackage, 0)
	if err := core.GLOBAL.DB.
		Where("install_state = ?", string(installConstant.StateRunning)).
		Find(&records).Error; err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	errorMessage := "上次安装在程序退出时中断，已清理未完成安装文件，请重新安装该整合包。"
	for _, record := range records {
		if err := cleanupEntry.CleanupInterrupted(record.PackageKey, "安装已中断"); err != nil {
			return err
		}
		progressSnapshot.Store(progressPayload.Payload{
			PackageKey:   record.PackageKey,
			Stage:        "failed",
			Status:       "安装已中断",
			Percent:      progresscalc.Clamp(record.InstallProgress),
			Message:      errorMessage,
			State:        string(installConstant.StateFailed),
			ErrorMessage: errorMessage,
		})
	}
	return nil
}
