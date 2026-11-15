package progress

// 文件职责：按实际读取字节数汇总更新包下载进度。
// 使用：应用更新资产读取时注入回调；边界：不发送桌面事件、不写入文件、不统计校验清单下载。

import (
	"io"
	"time"
)

// Snapshot 职责：描述本次更新包的下载状态；使用：进度回调与前端展示；边界：verifying 仅表示下载完成，安装结果由服务返回。
type Snapshot struct {
	// Stage 职责：区分 downloading 与 verifying；使用：按钮状态文案；边界：不代表更新成功。
	Stage string `json:"stage"`
	// DownloadedBytes 职责：记录实际读取字节数；使用：计算下载比例；边界：不包含 checksum 内容。
	DownloadedBytes int64 `json:"downloadedBytes"`
	// TotalBytes 职责：承载发布元数据中的附件大小；使用：计算百分比；边界：非正数表示大小未知。
	TotalBytes int64 `json:"totalBytes"`
}

// Reporter 职责：接收下载快照；使用：服务注入事件广播；边界：同步调用，不能阻塞下载或修改下载内容。
type Reporter func(Snapshot)

// downloadReader 职责：包装下载流并节流报告；使用：ReadAll 内部读取；边界：不关闭原始流，关闭责任仍属于调用方。
type downloadReader struct {
	// reader 是实际资产流，只由 Read 消费。
	reader io.Reader
	// snapshot 保存累计字节数，失败时不会报告校验阶段。
	snapshot Snapshot
	// report 接收节流后的下载快照，不允许为空。
	report Reporter
	// lastReport 限制下载中的报告频率，不影响最终快照。
	lastReport time.Time
}

// Read 职责：统计本次实际读取的字节；使用：io.ReadAll 驱动；边界：原样返回读取错误，下载中每 100ms 最多报告一次。
func (reader *downloadReader) Read(buffer []byte) (int, error) {
	n, err := reader.reader.Read(buffer)
	reader.snapshot.DownloadedBytes += int64(n)
	if time.Since(reader.lastReport) >= 100*time.Millisecond {
		reader.report(reader.snapshot)
		reader.lastReport = time.Now()
	}
	return n, err
}

// ReadAll 职责：完整读取主更新包并报告进度；使用：ApplyRelease 在校验前调用；边界：无回调时直接读取，读取失败不报告下载完成。
// 逻辑流：1. 报告下载开始 => 2. 累计并节流报告字节数 => 3. 成功后报告校验阶段，失败直接返回原始错误。
func ReadAll(reader io.Reader, totalBytes int64, report Reporter) ([]byte, error) {
	if report == nil {
		return io.ReadAll(reader)
	}
	tracked := &downloadReader{
		reader: reader, report: report, lastReport: time.Now(),
		snapshot: Snapshot{Stage: "downloading", TotalBytes: totalBytes},
	}
	report(tracked.snapshot)
	data, err := io.ReadAll(tracked)
	if err == nil {
		tracked.snapshot.Stage = "verifying"
		report(tracked.snapshot)
	}
	return data, err
}
