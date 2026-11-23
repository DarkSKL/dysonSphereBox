package factory

import (
	"context"
	"testing"
	"time"
)

// TestGenerationQueueOrdering 模拟运行中、等待中和最后一次提交的任务，确认过期请求无法重夺执行槽。
func TestGenerationQueueOrdering(t *testing.T) {
	var tasks GenerationTasks
	started, release := make(chan struct{}), make(chan struct{})
	tasks.RunAsync(context.Background(), func(ctx context.Context) { close(started); <-release })
	<-started
	middle, newest := make(chan error, 1), make(chan error, 1)
	tasks.RunAsync(context.Background(), func(ctx context.Context) { middle <- ctx.Err() })
	tasks.RunAsync(context.Background(), func(ctx context.Context) { newest <- ctx.Err() })
	select {
	case err := <-middle:
		if err != context.Canceled {
			t.Fatalf("旧任务仍可运行 %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("等待中的任务没有取消")
	}
	select {
	case <-newest:
		t.Fatal("两个有效任务同时执行")
	default:
	}
	close(release)
	select {
	case err := <-newest:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("新任务没有得到执行槽")
	}
	if err := tasks.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
