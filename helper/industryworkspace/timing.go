package industryworkspace

// 文件职责：按本机有效计算时间与实际分工估算布局剩余时间；使用：任务轮询；边界：布局重排和扩产可改变预估范围。

import (
	"math"
	"time"

	"DysonSphereBox/helper/industryplan"
)

// TimeEstimate 是当前目标或本轮扩产的预计剩余计算时间，单位为秒。
type TimeEstimate struct {
	MinSeconds int64 `json:"minSeconds"`
	MaxSeconds int64 `json:"maxSeconds"`
}

// SetState 只累计运行时长。暂停与恢复使用同一个计时器，切换页面不影响计时。
func (t *Task) SetState(state string, now time.Time) {
	if t.State == state {
		return
	}
	if !t.activeSince.IsZero() {
		t.activeTime += now.Sub(t.activeSince)
		t.activeSince = time.Time{}
	}
	if state == "running" {
		t.activeSince = now
	}
	t.State = state
}

func (t *Task) elapsed(now time.Time) time.Duration {
	if t.activeSince.IsZero() {
		return t.activeTime
	}
	return t.activeTime + now.Sub(t.activeSince)
}

// Observe 更新当前分工的剩余工作量。反复核验同一前缀只累计花费的时间，不重复计算完成量。
func (t *Task) Observe(p industryplan.Progress, now time.Time) {
	t.Progress = p
	if p.Phase == "expand" {
		t.Expanding = true
		t.roundStart = t.elapsed(now)
		t.workDone, t.workTotal, t.furthest = 0, 0, 0
	}
	if p.WorkTotal > 0 {
		t.workTotal = p.WorkTotal
		t.simpleLayout = p.SimpleLayout
		t.workDone = min(p.WorkDone, p.WorkTotal)
		t.furthest = max(t.furthest, t.workDone)
	}
}

// Snapshot 给页面当前时刻的估时。首次按设备规模粗估，之后按本轮实际进展修正。
func (t Task) Snapshot(now time.Time) Task {
	elapsed := t.elapsed(now)
	t.ElapsedSeconds = int64(elapsed / time.Second)
	t.Estimate = nil
	if t.workTotal == 0 || t.State != "running" && t.State != "paused" {
		return t
	}
	// 一个标准块作为初始样本权重。真存档 2000 白矩阵的 9390 个工作单位约耗时 2850 秒，单进单出的短线约每单位 0.15 秒。
	const priorMachines = 32
	prior := .30
	if t.simpleLayout {
		prior = .15
	}
	seconds := (elapsed - t.roundStart).Seconds()
	secondsPerUnit := math.Max(prior, seconds/priorMachines)
	if t.furthest > 0 {
		secondsPerUnit = (seconds + priorMachines*prior) / (float64(t.furthest) + priorMachines)
	}
	secondsLeft := float64(max(1, t.workTotal-t.workDone)) * secondsPerUnit
	t.Estimate = &TimeEstimate{
		MinSeconds: max(1, int64(math.Ceil(secondsLeft/2))),
		MaxSeconds: max(1, int64(math.Ceil(secondsLeft*2))),
	}
	return t
}
