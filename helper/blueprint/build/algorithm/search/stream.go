package search

// 文件职责：复用有界工作槽连续执行候选，避免固定波次等待最慢任务才派发后续工作。
// 使用：大型候选已有独立执行取消上下文时；边界：执行可乱序完成，归约始终按原索引，父请求取消不交付结果。

import (
	"context"
	"sync"
	"sync/atomic"
)

// Stream 职责：候选完成后立即复用空闲槽；使用：共享父寿命及私有优化截止的候选搜索；边界：只归约实际执行完的项，等待所有在途任务退出后按索引归约，不改变并发上限。
func Stream[Candidate, Output any](parent, execution context.Context, candidates []Candidate, workers int, execute func(context.Context, int, Candidate) Output, reduce func(int, Output) error) error {
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(execution)
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	outputs := make([]*Output, len(candidates))
	var next atomic.Int64
	var pending sync.WaitGroup
	for worker := 0; worker < max(1, min(workers, len(candidates))); worker++ {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for ctx.Err() == nil {
				index := int(next.Add(1) - 1)
				if index >= len(candidates) {
					return
				}
				out := execute(ctx, index, candidates[index])
				outputs[index] = &out
			}
		}()
	}
	pending.Wait()
	if err := parent.Err(); err != nil {
		return err
	}
	for index, out := range outputs {
		if err := parent.Err(); err != nil {
			return err
		}
		if out != nil {
			if err := reduce(index, *out); err != nil {
				return err
			}
		}
	}
	return parent.Err()
}
