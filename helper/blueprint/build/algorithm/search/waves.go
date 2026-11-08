package search

import (
	"context"
	"runtime"
	"sync"
)

// MaxCandidateWorkers 单任务候选并发数的默认上限。
const MaxCandidateWorkers = 10

// Concurrency 职责：计算单任务候选并发上限；使用：候选组派发；边界：不修改全局运行时设置，至少保留一个逻辑核心给界面与其他任务。
func Concurrency(candidates int) int {
	return max(1, min(candidates, MaxCandidateWorkers, max(1, runtime.GOMAXPROCS(0)-1)))
}

// Waves 职责：按固定波次执行候选并顺序归约；使用：模式候选组；边界：execute 独占可变状态，reduce 仅在整波退出后执行，取消后等待所有已启动任务退出。
func Waves[Candidate, Output any](ctx context.Context, candidates []Candidate, workers int, execute func(context.Context, int, Candidate) Output, reduce func(int, Output) error) error {
	workers = max(1, workers)
	for start := 0; start < len(candidates); start += workers {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+workers, len(candidates))
		outputs := make([]Output, end-start)
		var pending sync.WaitGroup
		for index := start; index < end; index++ {
			if ctx.Err() != nil {
				break
			}
			pending.Add(1)
			go func() {
				defer pending.Done()
				outputs[index-start] = execute(ctx, index, candidates[index])
			}()
		}
		pending.Wait()
		if err := ctx.Err(); err != nil {
			return err
		}
		for offset, output := range outputs {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := reduce(start+offset, output); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
