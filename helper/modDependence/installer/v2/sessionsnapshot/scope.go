package sessionsnapshot

// 文件职责：区分当前安装写入和重装前已经存在的文件清单。
// 使用：manifest 补全和失败记录清理筛选数据库兜底证据。
// 边界：只查询内存证据；缺少会话起点时优先保留旧文件，不能把旧文件当作新文件删除。

import (
	"time"

	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// BelongsToSession 判断文件记录是否由本次重装写入。
// 职责：保护原包未改动文件；使用：重装失败的数据库和物理清理；边界：显式写入证据优先，首次安装不受旧包时间过滤影响。
func BelongsToSession(record installedModel.ModInstalledFile, manifest sessionstate.Manifest) bool {
	if manifest.PreviousPackage == nil {
		return true
	}
	for _, file := range manifest.InstalledFiles {
		if file.PackageKey == record.PackageKey && installpath.Equal(file.TargetPath, record.TargetPath) {
			return true
		}
	}
	started, err := time.Parse(time.RFC3339Nano, manifest.StartedAt)
	return err == nil && !record.CreatedAt.Before(started)
}
