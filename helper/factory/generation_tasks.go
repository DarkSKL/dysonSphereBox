package factory

// 文件职责：实现最新生成请求优先的取消与串行准入。
// 使用：生成服务复核方案后调用；边界：协作取消，不强杀 goroutine，旧引擎释放执行槽后新任务才运行。

import (
	"context"
	"sync"
)

// GenerationTasks 职责：管理一个服务实例的生成任务；使用：零值可用；边界：使用后不可复制，等待中的请求也会被更新请求替代。
type GenerationTasks struct {
	mu      sync.Mutex
	current *generationTask
	slot    chan struct{}
	closed  bool
}

// generationTask 职责：保存可取消任务身份；使用：区分新旧清理；边界：仅在管理器锁下替换身份。
type generationTask struct {
	cancel context.CancelFunc
}

// Begin 职责：取消上一个请求并等待独占执行槽；使用：调用后必须 defer finish 并先检查返回上下文；边界：被替代的等待者不执行计算，finish 幂等且不清除新任务。
func (tasks *GenerationTasks) Begin(parent context.Context) (ctx context.Context, finish func()) {
	ctx, cancel := context.WithCancel(parent)
	task := &generationTask{cancel: cancel}
	tasks.mu.Lock()
	if tasks.closed {
		tasks.mu.Unlock()
		cancel()
		return ctx, func() {}
	}
	if tasks.slot == nil {
		tasks.slot = make(chan struct{}, 1)
	}
	if tasks.current != nil {
		tasks.current.cancel()
	}
	tasks.current = task
	slot := tasks.slot
	tasks.mu.Unlock()

	acquired := false
	select {
	case slot <- struct{}{}:
		acquired = true
	case <-ctx.Done():
	}
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			cancel()
			tasks.mu.Lock()
			if tasks.current == task {
				tasks.current = nil
			}
			tasks.mu.Unlock()
			if acquired {
				<-slot
			}
		})
	}
}

// Shutdown 职责：停止接受新任务并协作取消当前任务；使用：应用关闭；边界：在截止时间内等待执行槽真正释放，不强杀 worker 或提前释放旧任务槽。
func (tasks *GenerationTasks) Shutdown(ctx context.Context) error {
	tasks.mu.Lock()
	tasks.closed = true
	if tasks.current != nil {
		tasks.current.cancel()
	}
	slot := tasks.slot
	tasks.mu.Unlock()
	if slot == nil {
		return nil
	}
	select {
	case slot <- struct{}{}:
		<-slot
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
