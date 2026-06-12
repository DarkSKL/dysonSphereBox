package network

// 文件职责：维护无头服务器网络采样所需的进程内状态和计数 DTO。
// 使用：InspectStatus 加锁更新 headlessNetworkLast 与 headlessNetworkSamples，并用计数 DTO 计算速率。
// 边界：状态只存在当前进程内，不持久化、不跨进程共享、不代表端口或进程健康状态。

import (
	"sync"
	"time"

	"DysonSphereBox/helper/modDependence/headless/contract"
)

var (
	// headlessNetworkMutex 职责：保护网络采样状态并发访问；使用：InspectStatus 更新样本前加锁；边界：不保护其它无头服务器状态。
	headlessNetworkMutex sync.Mutex
	// headlessNetworkLast 职责：保存上一帧网卡累计计数器；使用：下一次采样计算速率；边界：应用启动后首次为空。
	headlessNetworkLast *headlessNetworkCounter
	// headlessNetworkSamples 职责：保存最近网络速率样本；使用：前端折线图和峰值计算；边界：只保存在当前进程内。
	headlessNetworkSamples []contract.HeadlessNetworkSample
)

// headlessNetworkCounter 表示一次系统网卡累计字节计数。
// 职责：保存接收、发送累计字节和采样时间。
// 使用：InspectStatus 与上一帧相减计算速率。
// 边界：只用于内存计算，不直接暴露给前端。
type headlessNetworkCounter struct {
	// receivedBytes 职责：记录系统累计接收字节；使用：计算下载速率；边界：计数器回绕时不计算负速率。
	receivedBytes uint64
	// sentBytes 职责：记录系统累计发送字节；使用：计算上传速率；边界：计数器回绕时不计算负速率。
	sentBytes uint64
	// capturedAt 职责：记录采样时间；使用：计算两次采样间隔；边界：本地时间异常会导致速率归零。
	capturedAt time.Time
}

// headlessPowerShellNetworkCounter 表示 PowerShell 网卡统计输出项。
// 职责：承载 Get-NetAdapterStatistics 返回的 JSON 字段。
// 使用：readPowerShellNetworkCounter 反序列化单项或数组输出。
// 边界：只读取 ReceivedBytes 和 SentBytes，不区分具体网卡。
type headlessPowerShellNetworkCounter struct {
	// ReceivedBytes 职责：记录单个网卡累计接收字节；使用：聚合为总下载计数；边界：字段缺失时为 0。
	ReceivedBytes uint64 `json:"ReceivedBytes"`
	// SentBytes 职责：记录单个网卡累计发送字节；使用：聚合为总上传计数；边界：字段缺失时为 0。
	SentBytes uint64 `json:"SentBytes"`
}
