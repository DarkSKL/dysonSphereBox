package shared

// 文件职责：提供 V2 安装会话清理流程共享的 manifest 读取辅助。
// 使用：目录清理、缓存清理等子流程需要统一计算组件 key 集合时调用。
// 边界：只读取 manifest 内存结构，不访问数据库、不验证组件是否仍然存在。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
)

// ComponentKeys 汇总会话清理涉及的组件 key。
// 职责：合并已安装组件和计划组件，供目录清理判断共享关系或缓存清理定位目录。
// 使用：session cleanup 的目录与缓存清理流程调用。
// 边界：只读取 manifest，不校验组件是否仍在数据库中。
func ComponentKeys(manifest sessionstate.Manifest) []string {
	keys := make([]string, 0, len(manifest.InstalledComponents)+len(manifest.PlannedComponents))
	for _, key := range manifest.InstalledComponents {
		keys = sessionutil.AppendUniqueString(keys, key)
	}
	for _, component := range manifest.PlannedComponents {
		keys = sessionutil.AppendUniqueString(keys, component.ComponentKey)
	}
	return keys
}
