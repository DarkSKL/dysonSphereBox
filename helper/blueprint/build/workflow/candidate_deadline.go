package workflow

// 文件职责：统一计算候选优化的绝对局部截止；使用：普通列表与条件组；边界：只缩短优化窗口，不设置新的任务总截止。

import (
	"context"
	"time"
)

// candidateCutoff 职责：取显式优化时限与父截止预留的较早者；使用：每个候选节点开始时计算；边界：无父截止忽略 reserve，超出 Duration 可表达的预留按预算耗尽处理，避免整数溢出。
func candidateCutoff(ctx context.Context, spec *Candidates, started time.Time) time.Time {
	var cutoff time.Time
	if spec.TimeoutMs > 0 {
		cutoff = started.Add(time.Duration(spec.TimeoutMs) * time.Millisecond)
	}
	if spec.ReserveMs > 0 {
		if parent, ok := ctx.Deadline(); ok {
			reserved := started
			if int64(spec.ReserveMs) <= int64((time.Duration(1<<63-1))/time.Millisecond) {
				reserved = parent.Add(-time.Duration(spec.ReserveMs) * time.Millisecond)
			}
			if cutoff.IsZero() || reserved.Before(cutoff) {
				cutoff = reserved
			}
		}
	}
	return cutoff
}
