package search

// 文件职责：验证规划检查点暂停、原栈继续和取消；使用：短时间窗并发回归；边界：用通道确认等待状态，不轮询或依赖长睡眠。

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestCheckpointContinueSameStack 职责：确认到期检查点保留已完成工作；使用：继续计算回归；边界：Continue 不重启工作函数，也不延长正在运行的窗口。
func TestCheckpointContinueSameStack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	paused := make(chan struct{}, 1)
	control := New(5*time.Millisecond, func() { paused <- struct{}{} })
	waitControlWindow(t, ctx, control)
	var starts atomic.Int32
	result := make(chan int, 1)
	errResult := make(chan error, 1)
	go func() {
		starts.Add(1)
		completed := 19
		if err := Checkpoint(WithControl(ctx, control)); err != nil {
			errResult <- err
			return
		}
		completed += 23
		result <- completed
	}()
	select {
	case <-paused:
	case <-ctx.Done():
		t.Fatal("到期后未通知暂停")
	}
	select {
	case value := <-result:
		t.Fatalf("继续前已经越过检查点：%d", value)
	default:
	}
	if !control.Continue() {
		t.Fatal("已暂停的检查点无法继续")
	}
	select {
	case value := <-result:
		if value != 42 || starts.Load() != 1 {
			t.Fatalf("继续未保留原栈状态：结果 %d，进入次数 %d", value, starts.Load())
		}
	case err := <-errResult:
		t.Fatalf("继续失败：%v", err)
	case <-ctx.Done():
		t.Fatal("继续没有释放检查点")
	}
	deadline := control.deadline
	if control.Continue() || !control.deadline.Equal(deadline) {
		t.Fatal("重复继续延长了运行中的窗口")
	}
}

// TestConcurrentCheckpointsSharePause 职责：确认并发候选共用一次暂停并一并继续；使用：布局并发回归；边界：所有候选进入等待选择后才发继续信号。
func TestConcurrentCheckpointsSharePause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	const workers = 8
	var pauses atomic.Int32
	control := New(5*time.Millisecond, func() { pauses.Add(1) })
	waitControlWindow(t, ctx, control)
	waiting := make(chan struct{}, workers)
	finished := make(chan error, workers)
	controlled := WithControl(checkpointWaitContext{Context: ctx, waiting: waiting}, control)
	for i := 0; i < workers; i++ {
		go func() { finished <- Checkpoint(controlled) }()
	}
	for i := 0; i < workers; i++ {
		select {
		case <-waiting:
		case <-ctx.Done():
			t.Fatal("并发候选没有全部进入同一暂停")
		}
	}
	if pauses.Load() != 1 {
		t.Fatalf("一次暂停重复通知 %d 次", pauses.Load())
	}
	select {
	case err := <-finished:
		t.Fatalf("继续前已有候选越过检查点：%v", err)
	default:
	}
	if !control.Continue() {
		t.Fatal("共用暂停不能继续")
	}
	for i := 0; i < workers; i++ {
		select {
		case err := <-finished:
			if err != nil {
				t.Fatalf("候选继续失败：%v", err)
			}
		case <-ctx.Done():
			t.Fatal("一次继续没有释放全部候选")
		}
	}
	if pauses.Load() != 1 {
		t.Fatal("释放同一批候选时又创建了暂停")
	}
}

// TestActiveTimerExcludesPause 使用确定的暂停时间，验证阶段计时不会把玩家离开的时长计入运行。
func TestActiveTimerExcludesPause(t *testing.T) {
	control := New(time.Hour, nil)
	ctx := WithControl(t.Context(), control)
	control.pausedAt = time.Now().Add(-time.Hour)
	control.resume = make(chan struct{})
	elapsed := ActiveTimer(ctx)
	if got := elapsed(); got != 0 {
		t.Fatalf("暂停中的时钟仍在运行：%v", got)
	}
	if !control.Continue() {
		t.Fatal("样例没有恢复运行")
	}
	if got := elapsed(); got < 0 || got >= time.Hour {
		t.Fatalf("计时器包含了暂停的一小时：%v", got)
	}
}

// TestActiveTimeoutAfterContinue 校验在暂停中创建的子时窗，继续后用完实际计算预算才超时，父任务仍可使用。
func TestActiveTimeoutAfterContinue(t *testing.T) {
	const budget = 35 * time.Millisecond
	paused := make(chan struct{})
	control := New(time.Hour, func() { close(paused) })
	control.deadline = time.Now().Add(-time.Second)
	parent := WithControl(t.Context(), control)
	resumed := make(chan error, 1)
	go func() { resumed <- Checkpoint(parent) }()
	<-paused
	trial, stop := WithActiveTimeout(parent, budget)
	defer stop()
	active := ActiveTimer(parent)
	select {
	case <-trial.Done():
		t.Fatal("暂停耗尽了试摆时窗", context.Cause(trial))
	case <-time.After(2 * budget):
	}
	control.Continue()
	if err := <-resumed; err != nil {
		t.Fatal(err)
	}
	select {
	case <-trial.Done():
		if context.Cause(trial) != context.DeadlineExceeded || active() < budget || parent.Err() != nil {
			t.Fatalf("实际计算 %s，试摆结束原因 %v，父任务 %v", active(), context.Cause(trial), parent.Err())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("实际试摆预算耗尽仍未停止")
	}
}

// TestCancellationWakesPausedCheckpoint 职责：确认用户取消能够唤醒暂停任务；使用：取消按钮回归；边界：无需先调用 Continue。
func TestCancellationWakesPausedCheckpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	paused := make(chan struct{}, 1)
	control := New(5*time.Millisecond, func() { paused <- struct{}{} })
	waitControlWindow(t, ctx, control)
	finished := make(chan error, 1)
	go func() { finished <- Checkpoint(WithControl(ctx, control)) }()
	select {
	case <-paused:
	case <-ctx.Done():
		t.Fatal("检查点未进入暂停")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("暂停任务取消后返回 %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("取消未唤醒暂停任务")
	}
}

// checkpointWaitContext 职责：观察检查点进入等待选择；使用：并发测试握手；边界：原样保留父上下文的取消通道和行为。
type checkpointWaitContext struct {
	context.Context
	waiting chan<- struct{}
}

// Done 职责：报告等待选择已取得取消通道；使用：确认各候选已捕获共用暂停；边界：通知通道预留每个候选的容量，不阻塞 Checkpoint。
func (c checkpointWaitContext) Done() <-chan struct{} {
	c.waiting <- struct{}{}
	return c.Context.Done()
}

// waitControlWindow 职责：等待短计算窗口到期；使用：检查点测试启动前；边界：只等待一次计时器，不睡眠轮询内部状态。
func waitControlWindow(t *testing.T, ctx context.Context, control *Control) {
	t.Helper()
	timer := time.NewTimer(time.Until(control.deadline) + time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal("测试窗口未能到期")
	}
}
