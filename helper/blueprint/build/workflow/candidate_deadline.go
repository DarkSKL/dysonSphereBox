package workflow

// 文件职责：统一计算候选优化的绝对局部截止；使用：普通列表与条件组；边界：只缩短优化窗口，不设置新的任务总截止。

import (
	"context"
	"time"
)

// candidateCutoff 职责：划分单带试摆、邻带搜索和收尾时间；使用：候选组启动时计算；边界：可暂停任务沿用父控制器，普通工作台沿用配置时限。
func (e *executor) candidateCutoff(ctx context.Context, spec *Candidates, started time.Time) time.Time {
	if e.options.UseParentDeadline {
		return time.Time{}
	}
	timeout, reserve := int64(spec.TimeoutMs), int64(spec.ReserveMs)
	parent, hasDeadline := ctx.Deadline()
	if hasDeadline && e.options.FirstVerifiedLayout && spec.Selection == "first" {
		// 可迁星试摆的父时窗可能短于单带时限或收尾预留。给单带三分之一、给收尾八分之一，其余时间可尝试真实邻带。
		if remaining := parent.Sub(started).Milliseconds(); remaining > 0 {
			if timeout >= remaining {
				timeout = max(1, remaining/3)
			}
			if reserve >= remaining {
				reserve = remaining / 8
			}
		}
	}
	var cutoff time.Time
	if timeout > 0 {
		cutoff = started.Add(time.Duration(timeout) * time.Millisecond)
	}
	if reserve > 0 && hasDeadline {
		reserved := started
		if reserve <= int64((time.Duration(1<<63-1))/time.Millisecond) {
			reserved = parent.Add(-time.Duration(reserve) * time.Millisecond)
		}
		if cutoff.IsZero() || reserved.Before(cutoff) {
			cutoff = reserved
		}
	}
	return cutoff
}
