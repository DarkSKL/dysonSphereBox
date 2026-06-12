package network

// 文件职责：提供网络采样结果的单位换算与样本窗口统计。
// 使用：InspectStatus 在返回前计算 MB/s、峰值和窗口跨度。
// 边界：只处理内存样本中的数值计算，不读取系统命令、不修改采样状态、不做展示格式化。

import "DysonSphereBox/helper/modDependence/headless/contract"

// bytesPerSecondToMBps 将字节每秒转换为 MB/s。
// 职责：统一网络速率展示单位换算。
// 使用：InspectStatus 计算前端展示值时调用。
// 边界：非正值返回 0，避免展示负速率。
func bytesPerSecondToMBps(value float64) float64 {
	if value <= 0 {
		return 0
	}

	return value / 1024 / 1024
}

// networkPeaks 计算样本窗口中的上传和下载峰值。
// 职责：从最近样本中提取峰值用于状态展示。
// 使用：InspectStatus 成功或失败返回前调用。
// 边界：样本为空时返回 0,0。
func networkPeaks(samples []contract.HeadlessNetworkSample) (float64, float64) {
	uploadPeak := 0.0
	downloadPeak := 0.0
	for _, sample := range samples {
		if sample.UploadMBps > uploadPeak {
			uploadPeak = sample.UploadMBps
		}
		if sample.DownloadMBps > downloadPeak {
			downloadPeak = sample.DownloadMBps
		}
	}

	return uploadPeak, downloadPeak
}

// networkSampleWindowSeconds 计算当前样本窗口跨度秒数。
// 职责：告诉前端折线图覆盖的时间范围。
// 使用：InspectStatus 返回状态前调用。
// 边界：样本不足、时间无效或乱序时返回 0。
func networkSampleWindowSeconds(samples []contract.HeadlessNetworkSample) int {
	if len(samples) <= 1 {
		return 0
	}

	first := samples[0].CapturedAtUnix
	last := samples[len(samples)-1].CapturedAtUnix
	if first <= 0 || last <= first {
		return 0
	}

	return int(last - first)
}
