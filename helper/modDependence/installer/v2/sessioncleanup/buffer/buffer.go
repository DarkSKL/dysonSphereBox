package buffer

// 文件职责：清理 V2 安装会话产生的下载、解压缓存和成功安装后的回滚备份。
// 使用：失败/中断清理完成后删除 partial 与 extract 目录，安装成功提交后删除 BackupPath。
// 边界：只处理缓存与备份文件，不修改 manifest、不删除游戏目标文件、不写数据库记录。
// 逻辑流：1. 从 manifest 汇总组件 key => 2. 定位 downloads partial 和 extract 目录 => 3. 校验路径位于 bufferDir 内 => 4. 删除缓存或跳过并记录日志。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	installerShared "DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/logging"
	cleanupShared "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
	"fmt"
	"os"
	"path/filepath"
)

// CleanupSessionBuffers 清理安装会话产生的下载和解压缓存。
// 职责：按包 key 和组件 key 删除 partial 下载文件与 extract 目录。
// 使用：sessioncleanup/entry.CleanupInterrupted 在核心清理完成后调用。
// 边界：BepInEx 基础依赖缓存跳过；路径不在 bufferDir 内时只记录跳过日志。
func CleanupSessionBuffers(bufferDir string, packageKey string, manifest sessionstate.Manifest) {
	for _, componentKey := range sessionutil.AppendUniqueString(cleanupShared.ComponentKeys(manifest), packageKey) {
		if v2Artifact.IsBepInExDependencyKey(componentKey) {
			continue
		}
		segment := installerShared.SanitizePathSegment(componentKey, "artifact")
		targets := []string{
			filepath.Join(bufferDir, "downloads", segment+".zip.partial"),
			filepath.Join(bufferDir, fmt.Sprintf("extract-%s", segment)),
		}
		for _, target := range targets {
			if !fileops.IsSubPath(bufferDir, target) {
				logging.RecordItem(packageKey, "buffer", target, "skip", "缓存路径不在安装缓存目录内，跳过清理", nil)
				continue
			}
			err := os.RemoveAll(target)
			if os.IsNotExist(err) {
				err = nil
			}
			logging.RecordItem(packageKey, "buffer", target, "remove", "移除本次未完成安装的下载或解压缓存", err)
		}
	}
}

// CleanupSessionBackups 清理成功安装后遗留的回滚备份文件。
// 职责：读取当前 manifest 并删除其中记录的 BackupPath。
// 使用：会话清理完成后调用。
// 边界：删除失败会被忽略；函数只处理备份文件，不删除 manifest 或目标文件。
func CleanupSessionBackups(packageKey string) error {
	manifest := sessionsnapshot.LoadManifest(packageKey)
	CleanupManifestBackups(manifest)
	return nil
}

// CleanupManifestBackups 删除已完成会话中的新旧文件备份。
// 职责：统一清理覆盖文件和旧版本残留文件的备份。
// 使用：数据库提交成功或失败恢复完成后调用。
// 边界：删除失败仅保留缓存，不把已提交安装视为失败，不删除游戏文件。
func CleanupManifestBackups(manifest sessionstate.Manifest) {
	files := append(append([]sessionstate.File(nil), manifest.InstalledFiles...), manifest.RemovedVersionFiles...)
	files = append(files, manifest.UntrackedFiles...)
	for _, file := range files {
		if file.BackupPath != "" {
			_ = os.Remove(file.BackupPath)
		}
	}
}
