package factory

// 文件职责：验证生成任务的最新请求优先、独占执行和清理幂等。
// 使用：生成服务回归；边界：只测试上下文与槽位，不启动蓝图引擎。

import (
	"context"
	"testing"
	"time"
)

// waitContextDone 职责：等待请求被取消；使用：断言任务接替顺序；边界：超时即失败，避免测试永久阻塞。
func waitContextDone(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("任务未在限定时间内取消")
	}
}

// waitResult 职责：等待 Begin 返回；使用：检测槽位是否仍被占用；边界：超时失败而不泄漏 goroutine。
func waitResult(t *testing.T, ch <-chan beginResult) beginResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("任务未在限定时间内获得执行槽")
		return beginResult{}
	}
}

type beginResult struct {
	ctx    context.Context
	finish func()
}

// TestGenerationTasksLatestWinsAndFinishIdempotent 职责：覆盖活跃任务、等待任务和新任务的竞态；使用：防止旧 finish 清除新身份；边界：不要求取消底层 goroutine。
func TestGenerationTasksLatestWinsAndFinishIdempotent(t *testing.T) {
	var tasks GenerationTasks
	ctx1, finish1 := tasks.Begin(context.Background())
	if ctx1.Err() != nil {
		t.Fatal("首个任务不应立即取消")
	}
	defer finish1()

	secondCh := make(chan beginResult, 1)
	go func() {
		ctx, finish := tasks.Begin(context.Background())
		secondCh <- beginResult{ctx: ctx, finish: finish}
	}()
	// Begin2 已登记为 current，但仍等待首任务释放槽；第三个请求应立即取消它。
	deadline := time.After(3 * time.Second)
	var firstTask *generationTask
	for {
		tasks.mu.Lock()
		current := tasks.current
		tasks.mu.Unlock()
		if current != nil && ctx1.Err() != nil {
			firstTask = current
			break
		}
		select {
		case <-deadline:
			t.Fatal("第二个任务未登记")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	thirdCh := make(chan beginResult, 1)
	go func() {
		ctx, finish := tasks.Begin(context.Background())
		thirdCh <- beginResult{ctx: ctx, finish: finish}
	}()
	second := waitResult(t, secondCh)
	waitContextDone(t, second.ctx)
	// 以任务身份而非定时假设等待第三个请求登记。
	deadline = time.After(3 * time.Second)
	for {
		tasks.mu.Lock()
		current := tasks.current
		tasks.mu.Unlock()
		if current != nil && current != firstTask {
			break
		}
		select {
		case <-deadline:
			t.Fatal("第三个任务未替换等待任务")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	// 完成被替代的等待请求不应释放第三个请求正在等候的槽。
	second.finish()
	second.finish()

	select {
	case third := <-thirdCh:
		third.finish()
		t.Fatal("第三个任务在首任务结束前错误地运行")
	default:
	}
	select {
	case <-time.After(30 * time.Millisecond):
	}
	finish1()
	finish1()
	third := waitResult(t, thirdCh)
	if third.ctx.Err() != nil {
		t.Fatalf("最新任务不应被旧任务 finish 取消: %v", third.ctx.Err())
	}
	third.finish()
	third.finish()

	// 清理后可再次取得槽位，证明旧身份不会卡住管理器。
	ctx4, finish4 := tasks.Begin(context.Background())
	if ctx4.Err() != nil {
		t.Fatalf("清理后新任务未运行: %v", ctx4.Err())
	}
	finish4()
}

// TestGenerationTasksParentCancellationReleasesWaiter 职责：验证父上下文取消不会误取执行槽；使用：窗口关闭/请求超时；边界：取消等待者仍需幂等 finish。
func TestGenerationTasksParentCancellationReleasesWaiter(t *testing.T) {
	var tasks GenerationTasks
	_, finish1 := tasks.Begin(context.Background())
	defer finish1()
	parent, cancel := context.WithCancel(context.Background())
	resultCh := make(chan beginResult, 1)
	go func() {
		ctx, finish := tasks.Begin(parent)
		resultCh <- beginResult{ctx: ctx, finish: finish}
	}()
	cancel()
	result := waitResult(t, resultCh)
	if result.ctx.Err() == nil {
		t.Fatal("父上下文取消后等待任务仍为可运行")
	}
	result.finish()
	result.finish()
	finish1()
}
