package entry

// 文件职责：编排 V2 远端包失败、中断或取消后的安装会话清理流程。
// 使用：安装主流程捕获失败、取消检查和启动恢复流程在需要清理未完成安装时调用。
// 边界：不直接实现文件删除、目录清理、缓存清理或日志写入；这些步骤分别委托 sessioncleanup 子包。

import (
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Buffer "DysonSphereBox/helper/modDependence/installer/v2/buffer"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	cleanupBuffer "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/buffer"
	cleanupDirectory "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/directory"
	cleanupFiles "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/files"
	"DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/logging"
	cleanupManifest "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/manifest"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
	"time"
)

// CleanupInterrupted 清理中断、失败或取消的远端包安装会话。
// 职责：恢复/删除本次会话写入的文件和数据库记录，并把安装状态标记为 failed。
// 使用：启动恢复、安装失败分支和取消安装分支调用。
// 边界：找不到安装记录时视为无需清理；无法获取游戏目录或缓存目录时记录跳过日志而不中断已完成的核心清理。
// 逻辑流：1. 读取安装记录与 manifest => 2. 补全清理证据 => 3. 清理文件和数据库记录 => 4. 标记 failed 并恢复旧包 => 5. 清理目录和缓存 => 6. 记录清理结果。
func CleanupInterrupted(packageKey string, status string) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	record := installedModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	// 已恢复的旧主包不再拥有待清理会话，重复调用必须保留成功安装的文件。
	if record.InstallState == string(installConstant.StateInstalled) || record.InstallState == "" {
		return nil
	}
	manifest := sessionstate.Manifest{}
	if len(record.InstallManifest) > 0 {
		if err := json.Unmarshal(record.InstallManifest, &manifest); err != nil {
			return err
		}
	}
	manifest = cleanupManifest.Enrich(db, record, manifest)
	if strings.TrimSpace(manifest.CleanedAt) != "" {
		logging.RecordItem(packageKey, "session", "", "skip", "清理会话已完成，跳过重复清理", nil)
		return nil
	}
	logging.RecordStart(record, status, manifest)
	if err := cleanupFiles.CleanupFiles(db, record.PackageKey, manifest); err != nil {
		logging.RecordDone(record.PackageKey, status, manifest, err)
		return err
	}
	manifest.CleanedAt = time.Now().Format(time.RFC3339Nano)
	// failed 标记与旧主包恢复必须一起提交，恢复失败不能留下已完成清理的标记。
	if err := db.Transaction(func(tx *gorm.DB) error {
		manifestJSON, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		if err := tx.Model(&installedModel.ModInstalledPackage{}).Where("package_key = ?", record.PackageKey).
			Updates(map[string]interface{}{"install_state": string(installConstant.StateFailed), "install_stage": "failed", "install_status": status, "install_progress": progresscalc.Clamp(record.InstallProgress), "error_message": status, "install_manifest": string(manifestJSON)}).Error; err != nil {
			return err
		}
		return sessionsnapshot.RestorePreviousPackage(tx, record.PackageKey, manifest)
	}); err != nil {
		logging.RecordDone(record.PackageKey, status, manifest, err)
		return err
	}
	cleanupBuffer.CleanupManifestBackups(manifest)
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err == nil {
		cleanupDirectory.CleanupPlannedDirs(db, gameDir, record.PackageKey, manifest)
		v2Uninstall.CleanupEmptyManagedDirs(filepath.Join(gameDir, "BepInEx"))
	} else {
		logging.RecordItem(record.PackageKey, "gameDir", "", "skip", "未获取到游戏目录，跳过目录兜底清理", err)
	}
	if bufferDir, err := v2Buffer.Dir(); err == nil {
		cleanupBuffer.CleanupSessionBuffers(bufferDir, record.PackageKey, manifest)
	} else {
		logging.RecordItem(record.PackageKey, "buffer", "", "skip", "未获取到安装缓存目录，跳过缓存兜底清理", err)
	}
	logging.RecordDone(record.PackageKey, status, manifest, nil)
	return nil
}

// CleanupFailed 在安装失败后执行会话清理并返回原错误。
// 职责：把普通失败和取消失败映射为对应状态，并串联清理错误。
// 使用：安装主流程和 artifact 处理阶段捕获错误后调用。
// 边界：清理失败时返回包装错误；清理成功时保留原始错误供上层判断。
func CleanupFailed(packageKey string, err error) error {
	status := "安装失败"
	if v2InstallError.IsCancelled(err) {
		status = "安装已取消"
	}
	if cleanupErr := CleanupInterrupted(packageKey, status); cleanupErr != nil {
		return fmt.Errorf("%w；清理失败：%v", err, cleanupErr)
	}
	return err
}

// CheckCancelled 检查取消并在取消时触发会话清理。
// 职责：把取消检查和失败清理绑定，避免后续步骤继续写文件。
// 使用：安装主流程在每个长耗时阶段之间调用。
// 边界：未取消时返回 nil；取消时返回清理后的错误链。
func CheckCancelled(ctx context.Context, packageKey string) error {
	if err := cancel.Check(ctx); err != nil {
		return CleanupFailed(packageKey, err)
	}
	return nil
}
