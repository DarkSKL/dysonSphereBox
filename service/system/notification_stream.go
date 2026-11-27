package system

// 职责：管理全应用唯一通知连接；使用：桌面启动与退出；边界：页面只消费刷新提示，无权创建连接。

import (
	"context"
	"math/rand/v2"
	"time"

	systemConstant "DysonSphereBox/constant/system"
	"DysonSphereBox/helper/system/account"
	"DysonSphereBox/helper/system/notification"
)

// StartNotifications 职责：启动唯一通知观察者；使用：桌面基础设施初始化后；边界：没有可信会话时只等待。
func (group *ServiceGroup) StartNotifications(parent context.Context, emit func(string, string)) {
	s := &group.NotificationService
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		return
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.done = make(chan struct{})
	go func(ctx context.Context, done chan struct{}) {
		defer close(done)
		s.watchSessions(ctx, emit)
	}(s.ctx, s.done)
}

// StopNotifications 职责：取消流、退避与收件箱请求；使用：退出前；边界：不删除持久化账号凭据。
func (group *ServiceGroup) StopNotifications() {
	s := &group.NotificationService
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		s.cancel()
		<-s.done
	}
}

// 职责：顺序处理每个可用账号；使用：唯一后台协程；边界：旧连接完成清理后才捕获新作用域。
func (s *NotificationService) watchSessions(ctx context.Context, emit func(string, string)) {
	for ctx.Err() == nil {
		changed := account.VerifiedSessionChanges()
		session, err := notification.Capture("")
		if err == nil {
			s.keepConnected(ctx, session, emit)
			if ctx.Err() == nil {
				emit(session.Key, systemConstant.NotificationInvalidated)
			}
			continue
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

// 职责：在可信会话内串行重连；使用：观察者取得可用账号后；边界：退出打断读流和退避，重连重新读取最新凭据。
func (s *NotificationService) keepConnected(parent context.Context, initial notification.Session, emit func(string, string)) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(initial.Context, cancel)
	defer stop()
	backoff := time.Second
	for ctx.Err() == nil && initial.Context.Err() == nil {
		session, err := notification.Capture(initial.Key)
		if err != nil || session.Context != initial.Context {
			return
		}
		started := time.Now()
		session.Stream(ctx, func(event string) {
			if ctx.Err() == nil && session.Context.Err() == nil {
				emit(session.Key, event)
			}
		})
		if time.Since(started) >= 30*time.Second {
			backoff = time.Second
		}
		delay := backoff - backoff/4 + rand.N(backoff/4)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}
