package network

// 文件职责：编排无头服务器网络状态采样并返回前端状态 DTO。
// 使用：headless 环境检测需要返回网络吞吐状态时调用 InspectStatus。
// 边界：只组织采样、速率计算和短窗口维护，不判断端口监听、不控制进程、不访问 Mod 安装状态。

import (
	"DysonSphereBox/helper/modDependence/headless/contract"
)

// InspectStatus 采样系统网卡速率并维护短窗口样本。
// 职责：读取累计计数、计算上下行速率、更新最近样本和峰值。
// 使用：headless.InspectHeadlessEnvironment 每次刷新无头服务器状态时调用。
// 边界：计数器读取失败时返回 Valid=false 并保留旧样本，不使用模拟数据。
// 逻辑流：1. 读取系统累计计数 => 2. 加锁读取上一帧和样本窗口 => 3. 失败则返回旧样本和错误 => 4. 与上一帧求差计算速率 => 5. 追加样本并裁剪窗口 => 6. 返回当前速率、峰值和窗口跨度。
func InspectStatus() contract.HeadlessNetworkStatus {
	counter, err := readHeadlessNetworkCounter()

	headlessNetworkMutex.Lock()
	defer headlessNetworkMutex.Unlock()

	status := contract.HeadlessNetworkStatus{
		Valid:   err == nil,
		Samples: append([]contract.HeadlessNetworkSample(nil), headlessNetworkSamples...),
	}
	if err != nil {
		status.Error = err.Error()
		status.UploadPeakMBps, status.DownloadPeakMBps = networkPeaks(headlessNetworkSamples)
		status.SampleWindowSeconds = networkSampleWindowSeconds(headlessNetworkSamples)
		return status
	}

	uploadMBps := 0.0
	downloadMBps := 0.0
	if headlessNetworkLast != nil {
		elapsedSeconds := counter.capturedAt.Sub(headlessNetworkLast.capturedAt).Seconds()
		if elapsedSeconds > 0 {
			if counter.sentBytes >= headlessNetworkLast.sentBytes {
				status.UploadBytesPerSecond = float64(counter.sentBytes-headlessNetworkLast.sentBytes) / elapsedSeconds
			}
			if counter.receivedBytes >= headlessNetworkLast.receivedBytes {
				status.DownloadBytesPerSecond = float64(counter.receivedBytes-headlessNetworkLast.receivedBytes) / elapsedSeconds
			}
			uploadMBps = bytesPerSecondToMBps(status.UploadBytesPerSecond)
			downloadMBps = bytesPerSecondToMBps(status.DownloadBytesPerSecond)
		}
	}

	status.UploadMBps = uploadMBps
	status.DownloadMBps = downloadMBps
	sample := contract.HeadlessNetworkSample{
		Label:          counter.capturedAt.Format("15:04:05"),
		CapturedAtUnix: counter.capturedAt.Unix(),
		UploadMBps:     uploadMBps,
		DownloadMBps:   downloadMBps,
	}
	headlessNetworkSamples = append(headlessNetworkSamples, sample)
	if len(headlessNetworkSamples) > 12 {
		headlessNetworkSamples = headlessNetworkSamples[len(headlessNetworkSamples)-12:]
	}
	headlessNetworkLast = &counter

	status.Samples = append([]contract.HeadlessNetworkSample(nil), headlessNetworkSamples...)
	status.UploadPeakMBps, status.DownloadPeakMBps = networkPeaks(headlessNetworkSamples)
	status.SampleWindowSeconds = networkSampleWindowSeconds(headlessNetworkSamples)

	return status
}
