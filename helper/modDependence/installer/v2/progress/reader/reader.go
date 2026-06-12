package reader

// 文件职责：提供带取消检查和安装进度上报能力的下载流包装器。
// 使用：artifact 下载阶段把 HTTP 响应体包装成 Reader，再交给文件写入逻辑复制到缓存文件。
// 边界：不负责关闭底层流、不校验下载哈希、不决定缓存命中或文件落盘路径。

import (
	"context"
	"io"
	"sync"
	"time"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
)

// Reader 表示带安装进度上报能力的下载流包装器。
// 职责：包装 io.Reader，在读取字节时计算百分比、速度并节流发送进度事件。
// 使用：artifact 下载阶段把远端响应体包成 Reader 后交给文件写入函数。
// 边界：只负责读取和进度上报，不负责关闭底层流、校验哈希或保存文件。
type Reader struct {
	// reader 职责：保存底层数据流；使用：Read 委托读取真实字节；边界：不负责 Close。
	reader io.Reader
	// ctx 职责：保存取消上下文；使用：Read 前后检查安装是否取消；边界：取消时返回 fileops.ErrInstallCancelled。
	ctx context.Context
	// packageKey 职责：记录进度所属包 key；使用：event.Emit 路由事件；边界：为空会导致进度被跳过。
	packageKey string
	// stage 职责：记录下载阶段名；使用：前端展示当前阶段；边界：只作展示。
	stage string
	// status 职责：记录下载状态文本；使用：前端展示和快照消息；边界：不作为状态机。
	status string
	// total 职责：记录下载总字节；使用：计算百分比和速度展示；边界：未知时可为 0。
	total int64
	// current 职责：记录已读取字节；使用：计算进度和速度；边界：由 mutex 保护。
	current int64
	// start 职责：记录映射到总安装进度的起始百分比；使用：progresscalc.Map 计算整体进度；边界：会被 progresscalc.Clamp 限制。
	start int
	// end 职责：记录映射到总安装进度的结束百分比；使用：progresscalc.Map 计算整体进度；边界：小于 start 时按 start 处理。
	end int
	// lastPercent 职责：记录上次发送的百分比；使用：节流重复事件；边界：初始为 -1 以确保首次变化可发送。
	lastPercent int
	// lastEmitAt 职责：记录上次发送时间；使用：超过 1 秒时允许同百分比刷新速度；边界：只在当前 reader 内有效。
	lastEmitAt time.Time
	// startedAt 职责：记录下载开始时间；使用：计算平均 bytesPerSecond；边界：不做暂停补偿。
	startedAt time.Time
	// mutex 职责：保护 current、lastPercent 和 lastEmitAt；使用：Read 中更新进度状态；边界：不保护底层 reader 的并发安全。
	mutex sync.Mutex
}

// New 创建带进度上报的 reader。
// 职责：绑定取消上下文、包 key、阶段、状态、总大小和进度映射区间。
// 使用：下载 artifact 前包装 HTTP 响应体。
// 边界：不校验 reader 是否为 nil，调用方应保证传入可读流。
func New(ctx context.Context, packageKey string, stage string, status string, source io.Reader, total int64, start int, end int) *Reader {
	return &Reader{
		ctx:         ctx,
		reader:      source,
		packageKey:  packageKey,
		stage:       stage,
		status:      status,
		total:       total,
		start:       start,
		end:         end,
		startedAt:   time.Now(),
		lastPercent: -1,
	}
}

// Read 读取下载流并按节流规则发送安装进度。
// 职责：从底层 reader 读取数据、累计字节、计算整体进度和下载速度。
// 使用：由 io.Copy 或文件写入函数间接调用。
// 边界：上下文取消时返回 fileops.ErrInstallCancelled，避免继续写入部分下载文件。
// 逻辑流：1. 读取前检查取消 => 2. 委托底层 reader 读取 => 3. 累计字节并计算百分比/速度 => 4. 按百分比变化或 1 秒间隔发送进度。
func (reader *Reader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, fileops.ErrInstallCancelled
	}
	count, err := reader.reader.Read(buffer)
	if err != nil && reader.ctx.Err() != nil {
		err = fileops.ErrInstallCancelled
	}
	if count > 0 {
		reader.mutex.Lock()
		reader.current += int64(count)
		current := reader.current
		total := reader.total
		percent := progresscalc.Map(reader.start, reader.end, progresscalc.Percent(current, total))
		bytesPerSecond := reader.bytesPerSecond(current)
		if percent != reader.lastPercent || time.Since(reader.lastEmitAt) >= time.Second {
			reader.lastPercent = percent
			reader.lastEmitAt = time.Now()
			event.Emit(payload.Payload{
				PackageKey:     reader.packageKey,
				Stage:          reader.stage,
				Status:         reader.status,
				Percent:        percent,
				Current:        current,
				Total:          total,
				BytesPerSecond: bytesPerSecond,
				Message:        reader.status,
			})
		}
		reader.mutex.Unlock()
	}
	return count, err
}

// bytesPerSecond 计算当前 reader 的平均下载速度。
// 职责：根据开始时间和已读字节估算每秒字节数。
// 使用：Read 发送进度事件前调用。
// 边界：耗时小于等于 0 时返回 0，避免除零。
func (reader *Reader) bytesPerSecond(current int64) int64 {
	elapsed := time.Since(reader.startedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(current) / elapsed)
}
