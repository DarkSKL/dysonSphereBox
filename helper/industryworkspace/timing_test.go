package industryworkspace

// 文件职责：验证布局重排与暂停对玩家看到的预计时间的影响；使用：任务状态回归。

import (
	"testing"
	"time"

	"DysonSphereBox/helper/industryplan"
)

func TestSmallSingleRecipeTimeEstimate(t *testing.T) {
	start := time.Unix(100, 0)
	var task Task
	task.SetState("running", start)
	task.Observe(industryplan.Progress{Phase: "blueprint", WorkTotal: 23, SimpleLayout: true}, start)
	// 真实铁块 600/min、23 个工作单位在本机约三秒完成，初次范围应覆盖秒级短线。
	initial := task.Snapshot(start).Estimate
	if initial == nil || initial.MinSeconds > 3 || initial.MaxSeconds < 3 || initial.MaxSeconds >= 12 {
		t.Fatalf("短线首次估时不符：%+v", initial)
	}
	if later := task.Snapshot(start.Add(time.Minute)).Estimate; later.MinSeconds <= initial.MinSeconds {
		t.Fatalf("首轮长时间没有进展时估算应随实际耗时修正：%+v", later)
	}
}

func TestPlanningTimeTracksActiveWork(t *testing.T) {
	start := time.Unix(100, 0)
	later := func(seconds int) time.Time { return start.Add(time.Duration(seconds) * time.Second) }
	var task Task
	task.SetState("running", start)
	if got := task.Snapshot(start); got.Estimate != nil {
		t.Fatal("量化前还不知道建设规模")
	}
	task.Observe(industryplan.Progress{Phase: "quant", WorkTotal: 130}, start)
	initial := task.Snapshot(start)
	if initial.Estimate == nil || initial.Estimate.MinSeconds <= 0 || initial.Estimate.MaxSeconds <= initial.Estimate.MinSeconds {
		t.Fatalf("量化后应立即给出初估：%+v", initial.Estimate)
	}
	p := industryplan.Progress{Phase: "blueprint", WorkTotal: 130, WorkDone: 65}
	task.Observe(p, later(130))
	measured := task.Snapshot(later(130))
	if measured.Estimate == nil || measured.Estimate.MinSeconds <= 33 || measured.Estimate.MaxSeconds <= initial.Estimate.MaxSeconds {
		t.Fatalf("实际速度低于初估时应按本机进度上调剩余用时：%+v", measured.Estimate)
	}
	task.SetState("paused", later(130))
	paused := task.Snapshot(later(730))
	if paused.ElapsedSeconds != 130 || *paused.Estimate != *measured.Estimate {
		t.Fatalf("暂停期间计时或估时发生变化：%+v", paused)
	}
	task.SetState("running", later(730))
	task.Observe(p, later(750))
	repeated := task.Snapshot(later(750))
	if repeated.ElapsedSeconds != 150 || repeated.Estimate.MinSeconds < measured.Estimate.MinSeconds {
		t.Fatalf("重核相同前缀时重复计算了进展：%+v", repeated)
	}
	task.Observe(industryplan.Progress{Phase: "expand"}, later(750))
	task.Observe(industryplan.Progress{Phase: "quant", WorkTotal: 130}, later(751))
	expansion := task.Snapshot(later(751))
	if !expansion.Expanding || *expansion.Estimate != *initial.Estimate || expansion.ElapsedSeconds != 151 {
		t.Fatalf("扩产应重新估当前轮次，并保留总用时：%+v", expansion)
	}
	task.SetState("cancelling", later(751))
	task.SetState("cancelled", later(755))
	if got := task.Snapshot(later(760)); got.Estimate != nil || got.ElapsedSeconds != 151 {
		t.Fatalf("结束后仍显示剩余时间或继续计时：%+v", got)
	}
}
