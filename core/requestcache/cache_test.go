package requestcache

// 文件职责：验证缓存边界、请求共享及写后失效的并发行为。
// 使用：go test ./core/requestcache；边界：用内存读取函数控制请求顺序，不访问网络。

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestReadRespectsExpirationForceAndScope(t *testing.T) {
	cache := New(3, 32)
	now := time.Now()
	cache.now = func() time.Time { return now }
	session, cancelSession := context.WithCancel(t.Context())
	defer cancelSession()
	scope := Scope{ID: "server/user", Context: session}
	method := Method{Name: "posts", CacheFor: 30 * time.Second}
	calls := 0
	load := func(context.Context) (Result, error) {
		calls++
		return Result{Data: []byte(fmt.Sprint(calls)), FreshFor: 10 * time.Second}, nil
	}
	read := func(scope Scope, force bool, want string) []byte {
		t.Helper()
		data, err := cache.Read(t.Context(), scope, method, force, load)
		if err != nil || string(data) != want {
			t.Fatalf("read returned %q, %v; want %q", data, err, want)
		}
		return data
	}
	data := read(scope, false, "1")
	data[0] = 'x'
	now = now.Add(9 * time.Second)
	read(scope, false, "1")
	now = now.Add(time.Second)
	read(scope, false, "2")
	read(scope, true, "3")

	otherSession, cancelOther := context.WithCancel(t.Context())
	defer cancelOther()
	read(Scope{ID: scope.ID, Context: otherSession}, false, "4")
	cancelSession()
	if _, err := cache.Read(t.Context(), scope, method, false, load); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended session read error = %v", err)
	}
}

func TestReadHonorsEntryAndByteLimits(t *testing.T) {
	cache := New(2, 5)
	scope := Scope{ID: "session", Context: t.Context()}
	reads := make(map[string]int)
	read := func(value string) {
		t.Helper()
		method := Method{Name: "summary", Parameters: value, CacheFor: time.Minute}
		data, err := cache.Read(t.Context(), scope, method, false, func(context.Context) (Result, error) {
			reads[value]++
			return Result{Data: []byte(value), FreshFor: time.Minute}, nil
		})
		if err != nil || string(data) != value {
			t.Fatalf("read returned %q, %v; want %q", data, err, value)
		}
	}
	read("a")
	read("bb")
	read("a")
	read("cc")
	read("bb") // 条目数超限时，先淘汰较久未访问的 bb。
	read("ddd")
	read("cc") // ddd 进入时会淘汰 cc，bb 和 ddd 正好占满 5 字节。
	read("oversized")
	read("oversized")
	for value, want := range map[string]int{"a": 1, "bb": 2, "cc": 2, "ddd": 1, "oversized": 2} {
		if reads[value] != want {
			t.Errorf("upstream reads for %q = %d; want %d", value, reads[value], want)
		}
	}
}

func TestSharingKeepsRequestWhenOneWaiterCancels(t *testing.T) {
	cache := New(4, 32)
	scope := Scope{ID: "session", Context: t.Context()}
	method := Method{Name: "detail", ShareRequest: true}
	first, cancelFirst := context.WithCancel(t.Context())
	defer cancelFirst()
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	firstResult := make(chan error, 1)
	go func() {
		_, err := cache.Read(first, scope, method, false, func(ctx context.Context) (Result, error) {
			close(started)
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-release:
				return Result{Data: []byte("live")}, nil
			}
		})
		firstResult <- err
	}()
	<-started
	joined := &waitingContext{Context: t.Context(), waiting: make(chan struct{})}
	secondResult := make(chan error, 1)
	go func() {
		data, err := cache.Read(joined, scope, method, false, func(context.Context) (Result, error) {
			return Result{}, errors.New("duplicate upstream request")
		})
		if err == nil && string(data) != "live" {
			err = fmt.Errorf("unexpected data %q", data)
		}
		secondResult <- err
	}()
	<-joined.waiting
	cancelFirst()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("first waiter error = %v", err)
	}

	// 唤醒上游，第二个读取仍应从原请求取得结果。
	unblock()
	if err := <-secondResult; err != nil {
		t.Fatal(err)
	}
}

func TestInvalidationDetachesOldInFlightRead(t *testing.T) {
	cache := New(4, 64)
	scope := Scope{ID: "session", Context: t.Context()}
	source := Source{Method: "reply", Target: "42"}
	method := Method{Name: "detail", Parameters: "42", ShareRequest: true, CacheFor: time.Minute, HitSource: []Source{source}}
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	old := make(chan error, 1)
	go func() {
		_, err := cache.Read(t.Context(), scope, method, false, func(ctx context.Context) (Result, error) {
			close(started)
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-release:
				return Result{Data: []byte("old"), FreshFor: time.Minute}, nil
			}
		})
		old <- err
	}()
	<-started
	cache.Invalidate(scope, source)
	loadNew := func(context.Context) (Result, error) {
		return Result{Data: []byte("new"), FreshFor: time.Minute}, nil
	}
	if data, err := cache.Read(t.Context(), scope, method, false, loadNew); err != nil || string(data) != "new" {
		t.Fatalf("new read returned %q, %v", data, err)
	}
	unblock()
	if err := <-old; !errors.Is(err, ErrInvalidated) {
		t.Fatalf("old request error = %v", err)
	}
	if data, err := cache.Read(t.Context(), scope, method, false, func(context.Context) (Result, error) {
		return Result{}, errors.New("new result was evicted")
	}); err != nil || string(data) != "new" {
		t.Fatalf("cached read returned %q, %v", data, err)
	}
}

// waitingContext 在调用方进入等待时发出信号，避免用定时休眠协调并发测试。
// TestForceStartsItsOwnFlight 验证强刷不合并到普通在途读取，普通请求晚到时也不能覆盖强刷结果。
func TestForceStartsItsOwnFlight(t *testing.T) {
	cache := New(2, 32)
	scope := Scope{ID: "session", Context: t.Context()}
	method := Method{Name: "posts", ShareRequest: true, CacheFor: time.Minute}
	started, finished := make(chan struct{}), make(chan error, 1)
	release := make(chan struct{}, 1)
	defer close(release)
	go func() {
		_, err := cache.Read(t.Context(), scope, method, false, func(ctx context.Context) (Result, error) {
			close(started)
			select {
			case <-release:
				return Result{Data: []byte("old"), FreshFor: time.Minute}, nil
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
		})
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary flight did not start")
	}

	wait, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	data, err := cache.Read(wait, scope, method, true, func(context.Context) (Result, error) {
		return Result{Data: []byte("forced"), FreshFor: time.Minute}, nil
	})
	if err != nil || string(data) != "forced" {
		t.Fatalf("forced read joined ordinary flight: %q %v", data, err)
	}
	release <- struct{}{}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-wait.Done():
		t.Fatal("ordinary flight did not finish")
	}
	data, err = cache.Read(wait, scope, method, false, func(context.Context) (Result, error) {
		t.Error("forced result was overwritten")
		return Result{}, nil
	})
	if err != nil || string(data) != "forced" {
		t.Fatalf("cached result = %q, %v", data, err)
	}
}

type waitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *waitingContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}
