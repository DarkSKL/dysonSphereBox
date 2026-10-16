package diagnostics

// 文件职责：识别没有排障价值的常规操作记录；使用：实时采集、历史索引与诊断导出共用；边界：只排除明确的低级别噪声，保留告警、失败和未知事件。

import (
	"DysonSphereBox/constant/logging"
	"strings"
)

// isRoutineOperation 职责：按业务语义排除自记录、普通读取和重复进度；使用：IsNoise 的统一策略入口；边界：不按日志文本猜测失败，Warn、Error、Fatal 均保留。
func isRoutineOperation(entry logging.Entry) bool {
	if entry.Level != logging.Info && entry.Level != logging.Debug {
		return false
	}
	if entry.Data["result"] == "failed" || entry.Data["success"] == false {
		return false
	}
	switch entry.Namespace {
	case "system.logging.delete", "system.logging.export", "modDependence.headless.exportLog",
		"system.lifecycle.ready", "system.update.checkAppUpdate",
		"system.account.settingsUpdated",
		"modDependence.installer.cleanupRemotePackageInstallStart":
		return true
	case "system.configuration.setConfig":
		return entry.Data["group"] == "System" && entry.Data["key"] == "AppTheme"
	case "system.environment.loadOrInitGamePath":
		return entry.Data["source"] == "configured"
	case "modDependence.installer.stage":
		// 成功与普通运行阶段由服务最终结果概括；取消和冲突仍保留操作语义。
		state, _ := entry.Data["state"].(string)
		return state == "running" || state == "installed" || state == "uninstalled"
	case "modDependence.installer.cleanupRemotePackageInstallItem":
		return entry.Data["success"] == true
	case "modDependence.bepInEx.install", "modDependence.bepInEx.update", "modDependence.bepInEx.uninstall",
		"modDependence.installer.installRemotePackage", "modDependence.installer.uninstallRemotePackage",
		"modDependence.headless.start", "modDependence.headless.stop":
		return entry.Data["skipped"] == true || entry.Action == "BepInEx 安装处理结果：已跳过"
	case "modDependence.headless.prepare":
		return entry.Data["changed"] == false && entry.Data["ready"] == true
	case "environment.resource.refresh_state":
		return entry.Data["result"] == "started" || entry.Data["result"] == "success" && noResourceChanges(entry.Data)
	}
	return strings.HasPrefix(entry.Namespace, "environment.resource.") && entry.Data["result"] == "started"
}

// noResourceChanges 职责：识别资源状态复核无变化的明确证据；使用：复核结果噪声判定；边界：缺失字段或未知数据类型不视为零，存在问题时保留记录。
func noResourceChanges(data map[string]interface{}) bool {
	evidence, ok := data["evidence"].(map[string]interface{})
	if !ok {
		return false
	}
	for _, key := range []string{"changedItems", "conflictItems", "missingItems"} {
		value := evidence[key]
		if value != 0 && value != float64(0) && value != int64(0) {
			return false
		}
	}
	switch issues := data["issues"].(type) {
	case nil:
		return true
	case []string:
		return len(issues) == 0
	case []interface{}:
		return len(issues) == 0
	default:
		return false
	}
}
