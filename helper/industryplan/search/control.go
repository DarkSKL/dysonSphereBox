package search

// 文件职责：在可恢复的计算边界暂停长规划；使用：服务持有 Control，算法调用 Checkpoint；边界：只保留当前 goroutine 状态，不写入磁盘。

import (
	"context"
	"sync"
	"time"
)

// Control 职责：管理一轮计算时窗和继续信号；使用：一个规划任务共用；边界：取消仍由父 context 负责。
type Control struct {
	mu        sync.Mutex
	window    time.Duration
	deadline  time.Time
	pausedAt  time.Time
	pausedFor time.Duration
	resume    chan struct{}
	onPause   func()
}

// key 职责：隔离任务控制上下文；使用：WithControl；边界：不暴露跨任务共享对象。
type key struct{}

// New 职责：建立计算时窗；使用：桌面默认三分钟；边界：暂停回调不能阻塞或调用 Checkpoint。
func New(window time.Duration, onPause func()) *Control {
	return &Control{window: window, deadline: time.Now().Add(window), onPause: onPause}
}

// WithControl 职责：将控制器传入算法；使用：父任务创建后包装 context；边界：保留原取消信号。
func WithControl(ctx context.Context, control *Control) context.Context {
	return context.WithValue(ctx, key{}, control)
}

// Controlled 职责：确认父任务是否拥有可恢复时窗；使用：子算法沿用父取消；边界：暂停期间不应被另一个墙钟超时销毁。
func Controlled(ctx context.Context) bool {
	c, _ := ctx.Value(key{}).(*Control)
	return c != nil
}

// ActiveTimer 统计阶段的实际运行时长；暂停中的所有并发检查点共用同一段等待时间。
func ActiveTimer(ctx context.Context) func() time.Duration {
	c, _ := ctx.Value(key{}).(*Control)
	now := time.Now
	if c != nil {
		now = c.activeNow
	}
	start := now()
	return func() time.Duration { return now().Sub(start) }
}

// activeNow 从单调时钟扣除本任务累计等待；暂停时停在当前时窗结束的检查点。
func (c *Control) activeNow() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resume != nil {
		return c.pausedAt.Add(-c.pausedFor)
	}
	return time.Now().Add(-c.pausedFor)
}

// Checkpoint 职责：在时间到后等待玩家继续；使用：矿源、分工、模块及布局候选边界；边界：没有控制器的批处理仅检查取消。
func Checkpoint(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c, _ := ctx.Value(key{}).(*Control)
	if c == nil {
		return nil
	}
	c.mu.Lock()
	notify := false
	now := time.Now()
	if c.resume == nil && now.After(c.deadline) {
		c.pausedAt = now
		c.resume = make(chan struct{})
		notify = true
	}
	wait := c.resume
	c.mu.Unlock()
	if notify && c.onPause != nil {
		c.onPause()
	}
	if wait == nil {
		return nil
	}
	select {
	case <-wait:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Continue 职责：释放全部暂停中的候选并开始下一时窗；使用：服务的继续按钮；边界：重复调用不延长正在运行的时窗。
func (c *Control) Continue() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resume == nil {
		return false
	}
	now := time.Now()
	c.pausedFor += now.Sub(c.pausedAt)
	c.deadline = now.Add(c.window)
	close(c.resume)
	c.resume = nil
	return true
}
