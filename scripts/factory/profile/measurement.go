package main

// 文件职责：组合真实生成、操作系统内存高水位和阶段取消测量。
// 使用：独立命令及其边界回归。
// 边界：性能观察不参与候选决策；测量报告不替代游戏放置与吞吐验收。

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
)

// processMemory 职责：保存系统测得的进程生命周期高水位；使用：固定单请求进程对比；边界：不是请求独占分配，缺失值保持nil。
type processMemory struct {
	Scope               string  `json:"scope"`               // 职责：说明峰值覆盖的时间范围；使用：证据解释；边界：不作请求差分。
	Source              string  `json:"source,omitempty"`    // 职责：记录系统取数接口；使用：复核来源；边界：不是采样推测。
	PeakWorkingSetBytes *uint64 `json:"peakWorkingSetBytes"` // 职责：保存驻留集峰值；使用：物理内存观察；边界：包含运行时及请求解析。
	PeakCommitBytes     *uint64 `json:"peakCommitBytes"`     // 职责：保存提交量峰值；使用：虚拟提交观察；边界：不等于Go堆或驻留集。
	Error               string  `json:"error,omitempty"`     // 职责：保存未取到数的原因；使用：失败诊断；边界：缺失不能解释为零。
}

// goMemory 职责：记录生成前后的Go运行时计数；使用：分配成本对比；边界：结束堆大小不是堆峰值。
type goMemory struct {
	HeapBeforeBytes     uint64 `json:"heapBeforeBytes"`     // 职责：记录调用前活跃堆；使用：起点说明；边界：不强制GC。
	HeapAfterBytes      uint64 `json:"heapAfterBytes"`      // 职责：记录返回后活跃堆；使用：终点说明；边界：不冒充峰值。
	TotalAllocatedBytes uint64 `json:"totalAllocatedBytes"` // 职责：记录TotalAlloc增量；使用：累计分配成本；边界：不是同时存活内存。
	GCCount             uint32 `json:"gcCount"`             // 职责：记录调用内GC次数增量；使用：解释分配压力；边界：不改变运行时配置。
}

// cancellationReport 职责：记录取消发出至Generate返回的真实延迟；使用：阶段取消验收；边界：单样本测量不能推广成通用SLA。
type cancellationReport struct {
	RequestedStage string   `json:"requestedStage"`         // 职责：保存指定触发阶段；使用：重放；边界：阶段未达到必须失败。
	DelayMs        float64  `json:"delayMs"`                // 职责：保存进入阶段后的计划延迟；使用：探测循环内部取消；边界：不是响应延迟。
	StageReached   bool     `json:"stageReached"`           // 职责：说明是否真实到达触发阶段；使用：避免假取消证据；边界：不表示延迟期间仍停在该阶段。
	Issued         bool     `json:"issued"`                 // 职责：记录是否实际调用cancel；使用：区分错过与响应；边界：返回后不补发取消。
	StageAtIssue   string   `json:"stageAtIssue,omitempty"` // 职责：保存取消时最近阶段；使用：识别阶段已经切换；边界：不猜测函数栈。
	ResponseMs     *float64 `json:"responseMs"`             // 职责：记录发出取消至调用返回的毫秒；使用：可复核延迟；边界：未发出时为nil。
}

// resourceReport 职责：保存单次重放的完整性能证据；使用：需求14.3的资源与取消测量；边界：失败不携带半份蓝图。
type resourceReport struct {
	SchemaVersion        int                          `json:"schemaVersion"`               // 职责：标识报告契约；使用：离线解析；边界：不是Wails事件版本。
	MeasuredAt           string                       `json:"measuredAt"`                  // 职责：记录完成时间；使用：证据追溯；边界：不参与生成。
	RequestSHA256        string                       `json:"requestSHA256"`               // 职责：固定原始输入内容；使用：样本一致性；边界：包含原文件空白。
	GoVersion            string                       `json:"goVersion"`                   // 职责：固定编译器版本；使用：性能对比；边界：不推断其它机器速度。
	OS                   string                       `json:"os"`                          // 职责：记录目标系统；使用：解释资源接口；边界：非游戏版本。
	Architecture         string                       `json:"architecture"`                // 职责：记录目标位数；使用：ABI复核；边界：不假设固定uintptr大小。
	VCS                  map[string]string            `json:"vcs"`                         // 职责：保存构建中的提交与脏标记；使用：溯源；边界：缺失时为空，仍需工作树证据。
	Mode                 int                          `json:"mode"`                        // 职责：记录公开生成策略；使用：矩阵归属；边界：不扩展产品模式。
	Surface              string                       `json:"surface"`                     // 职责：记录平面或自动球面；使用：矩阵归属；边界：不覆盖所有纬度。
	MachineCount         int                          `json:"machineCount"`                // 职责：记录可信请求的机器总数；使用：固定工作量；边界：不调整量化。
	GenerationDurationMs float64                      `json:"generationDurationMs"`        // 职责：记录Generate外层墙钟；使用：与内部阶段对照；边界：不含报告编码和写盘。
	Outcome              string                       `json:"outcome"`                     // 职责：记录passed/failed/canceled/timed-out；使用：真实调用结果；边界：不根据期望取消伪造状态。
	ErrorCode            string                       `json:"errorCode,omitempty"`         // 职责：保留引擎错误码；使用：失败分类；边界：不改写错误。
	Error                string                       `json:"error,omitempty"`             // 职责：保留引擎错误说明；使用：失败诊断；边界：不把失败转成功。
	VerificationError    string                       `json:"verificationError,omitempty"` // 职责：指出测量本身未满足的条件；使用：命令退出状态；边界：保留原始测量。
	BuildingCount        int                          `json:"buildingCount"`               // 职责：记录成功实体数量；使用：确保没有通过省建筑提速；边界：失败必须为零。
	Areas                int                          `json:"areas"`                       // 职责：记录成功区域数量；使用：样本一致性；边界：不代表全纬度通过。
	BlueprintSHA256      string                       `json:"blueprintSHA256,omitempty"`   // 职责：校对成功产物；使用：与人工包比较；边界：不写出蓝图正文。
	ProcessMemoryBefore  processMemory                `json:"processMemoryBefore"`         // 职责：记录Generate前的进程高水位；使用：区分初始化成本；边界：不能相减为请求峰值。
	ProcessMemoryAfter   processMemory                `json:"processMemoryAfter"`          // 职责：记录Generate后的进程高水位；使用：操作系统峰值证据；边界：包含进程启动和输入解析。
	GoMemory             goMemory                     `json:"goMemory"`                    // 职责：记录运行时分配增量；使用：解释内存压力；边界：不是操作系统内存。
	Cancellation         *cancellationReport          `json:"cancellation,omitempty"`      // 职责：保存指定阶段取消实测；使用：取消矩阵；边界：正常生成不伪造取消数据。
	Performance          *build.GenerationPerformance `json:"performance,omitempty"`       // 职责：保存真实引擎诊断；使用：关联寻路和阶段成本；边界：不删除引擎自身unmeasured边界。
}

// measureGeneration 职责：在一次真实调用周围测量内存和取消；使用：单请求独立进程；边界：不强制GC、不改GOMAXPROCS或路由预算。
func measureGeneration(request build.Request, option options, digest string, generate generateFunc) resourceReport {
	report := resourceReport{
		SchemaVersion: 1, RequestSHA256: digest, GoVersion: runtime.Version(), OS: runtime.GOOS, Architecture: runtime.GOARCH,
		Mode: request.Mode, Surface: "planar", MachineCount: request.Plan.MachineCount, VCS: map[string]string{},
	}
	if request.SurfaceLayout != nil {
		report.Surface = request.SurfaceLayout.Mode
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if strings.HasPrefix(setting.Key, "vcs.") {
				report.VCS[setting.Key] = setting.Value
			}
		}
	}
	parent, deadlineCancel := context.WithTimeout(context.Background(), option.timeout)
	defer deadlineCancel()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var mu sync.Mutex
	var timer *time.Timer
	var issuedAt time.Time
	currentStage, finished := "initialization", false
	if option.cancelStage != "" {
		report.Cancellation = &cancellationReport{RequestedStage: option.cancelStage, DelayMs: float64(option.cancelDelay) / float64(time.Millisecond)}
	}
	ctx = build.WithProgress(ctx, func(progress build.Progress) {
		if progress.Kind != build.ProgressKindPhase {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		currentStage = progress.Stage
		if c := report.Cancellation; c != nil && !c.StageReached && progress.Stage == option.cancelStage {
			c.StageReached = true
			timer = time.AfterFunc(option.cancelDelay, func() {
				mu.Lock()
				defer mu.Unlock()
				if !finished {
					issuedAt = time.Now()
					c.Issued, c.StageAtIssue = true, currentStage
					cancel()
				}
			})
		}
	})
	ctx = build.WithPerformance(ctx, func(performance build.GenerationPerformance) { report.Performance = &performance })
	request.CollectPerformance = true
	report.ProcessMemoryBefore = readProcessMemory()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	result, err := generate(ctx, request)
	returned := time.Now()
	mu.Lock()
	finished = true
	if timer != nil {
		timer.Stop()
	}
	if c := report.Cancellation; c != nil && c.Issued && !returned.Before(issuedAt) {
		response := float64(returned.Sub(issuedAt)) / float64(time.Millisecond)
		c.ResponseMs = &response
	}
	mu.Unlock()
	runtime.ReadMemStats(&after)
	report.ProcessMemoryAfter = readProcessMemory()
	report.GoMemory = goMemory{HeapBeforeBytes: before.HeapAlloc, HeapAfterBytes: after.HeapAlloc, TotalAllocatedBytes: after.TotalAlloc - before.TotalAlloc, GCCount: after.NumGC - before.NumGC}
	report.MeasuredAt, report.GenerationDurationMs = time.Now().UTC().Format(time.RFC3339Nano), float64(returned.Sub(started))/float64(time.Millisecond)
	report.Outcome = "passed"
	if err != nil {
		report.Outcome, report.Error = "failed", err.Error()
		if errors.Is(ctx.Err(), context.Canceled) {
			report.Outcome = "canceled"
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			report.Outcome = "timed-out"
		}
		var failure *blueprint.Error
		if errors.As(err, &failure) {
			report.ErrorCode = failure.Code
		}
		if hasPartialResult(result) {
			report.VerificationError = "失败调用返回了部分蓝图或实体诊断"
		}
	} else {
		report.BuildingCount, report.Areas = result.BuildingCount, result.Areas
		report.BlueprintSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(result.Blueprint)))
	}
	verifyMeasurement(&report)
	return report
}

// hasPartialResult 职责：完整拒绝失败结果中的任何已生成信息；使用：资源报告防泄漏；边界：仅允许初始化空切片与nil的差别。
func hasPartialResult(result build.Result) bool {
	if len(result.Warnings)+len(result.Bindings)+len(result.SharedBuildingIndices)+len(result.Zones) != 0 {
		return true
	}
	result.Warnings, result.Bindings, result.SharedBuildingIndices, result.Zones = nil, nil, nil, nil
	return !reflect.DeepEqual(result, build.Result{})
}

// verifyMeasurement 职责：拒绝把未触发取消、缺失诊断或生成错误当成通过；使用：退出码计算；边界：原始错误和事实仍留在报告中。
func verifyMeasurement(report *resourceReport) {
	var failures []string
	if report.VerificationError != "" {
		failures = append(failures, report.VerificationError)
	}
	if report.Performance == nil || report.Performance.Outcome != report.Outcome {
		failures = append(failures, "缺少与返回状态一致的引擎性能快照")
	}
	if report.OS == "windows" {
		memory := report.ProcessMemoryAfter
		if memory.Error != "" || memory.PeakWorkingSetBytes == nil || memory.PeakCommitBytes == nil || *memory.PeakWorkingSetBytes == 0 || *memory.PeakCommitBytes == 0 {
			failures = append(failures, "未取到Windows进程内存高水位")
		}
	}
	if c := report.Cancellation; c != nil {
		if !c.StageReached || !c.Issued || c.ResponseMs == nil || report.Outcome != "canceled" {
			failures = append(failures, "未完成指定阶段的实际取消，不能作为取消延迟证据")
		}
	} else if report.Outcome != "passed" || report.BuildingCount <= 0 {
		failures = append(failures, "完整重放未成功生成蓝图")
	}
	report.VerificationError = strings.Join(failures, "；")
}
