package thunderstoreindex

// 文件职责：记录 Thunderstore 索引适配器的 provider 失败与兜底诊断日志。
// 使用：LoadPackages 在某条远端索引链路失败但仍会继续尝试其它链路时调用。
// 边界：只写系统日志，不改变索引加载结果、不触发重试、不向服务层暴露日志写入错误。

import (
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// recordProviderFailure 记录单个 Thunderstore 索引 provider 失败事件。
// 职责：在新旧链路互相兜底时留下失败来源和当前优先模式，便于排查 404、gzip 或 chunk 异常。
// 使用：LoadPackages 捕获 provider.Load 错误后调用。
// 边界：日志写入失败会被静默吞掉，不影响后续 provider 或缓存兜底。
func recordProviderFailure(providerName string, preferredMode string, err error) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "thunderstoreIndex",
		Event:     "providerFailure",
		Level:     systemModel.LogLevelWarn,
		Type:      systemModel.LogTypeOperation,
		Content:   "Thunderstore 索引链路失败，正在尝试兜底",
		Metadata: map[string]interface{}{
			"provider":      providerName,
			"preferredMode": preferredMode,
		},
		Error: errorText(err),
	})
}

// recordCacheFallback 记录远端索引失败后使用本地缓存的事件。
// 职责：标记当前市场数据来自缓存兜底，保留用户可排查的来源线索。
// 使用：LoadPackages 在所有远端 provider 失败但成功读取缓存时调用。
// 边界：只记录日志，不修改缓存内容、不改变返回数据。
func recordCacheFallback(preferredMode string, failures []string) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "thunderstoreIndex",
		Event:     "cacheFallback",
		Level:     systemModel.LogLevelWarn,
		Type:      systemModel.LogTypeOperation,
		Content:   "Thunderstore 远端索引不可用，已使用本地缓存兜底",
		Metadata: map[string]interface{}{
			"preferredMode": preferredMode,
			"failures":      append([]string(nil), failures...),
		},
	})
}

// errorText 把错误转换为可写入日志的文本。
// 职责：避免日志构造处重复判断 nil 错误。
// 使用：recordProviderFailure 写入 Error 字段时调用。
// 边界：只返回错误文本，不包装错误、不参与主流程返回。
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
