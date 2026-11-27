package requestcache

// 文件职责：按请求定义复用业务读取，管理会话内缓存、并发共享与写后失效。
// 使用：客户端各业务域提供请求键、读取函数和写入事件。
// 边界：只保存已验证的响应数据，不负责鉴权、HTTP 执行或持久化。

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrInvalidated 职责：标识读取期间发生写入失效；使用：由调用方重新读取；边界：旧数据不随错误返回。
var ErrInvalidated = errors.New("query invalidated during read")

// Scope 职责：隔离请求数据和生命周期；使用：传入已验证会话的标识与可取消上下文；边界：标识不包含凭据。
type Scope struct {
	ID      string
	Context context.Context
}

// Source 职责：描述使查询失效的业务写入；使用：读方法声明规则，写方法提供实际目标；边界：规则的空目标匹配同名写入的任意目标。
type Source struct {
	Method string
	Target string
}

// Method 职责：定义读取的身份和复用策略；使用：同名方法使用稳定的策略及已归一化的参数；边界：CacheFor 为零时可共享在途读取。
type Method struct {
	Name         string
	Parameters   string
	CacheFor     time.Duration
	ShareRequest bool
	HitSource    []Source
}

// Result 职责：承载业务成功且校验后的数据与剩余有效期；使用：由读取函数交给 Cache；边界：FreshFor 为零时只交付给本次调用。
type Result struct {
	Data     []byte
	FreshFor time.Duration
}

type scopeKey struct {
	id   string
	done <-chan struct{}
}

func (scope Scope) key() scopeKey {
	return scopeKey{id: scope.ID, done: scope.Context.Done()}
}

type queryKey struct {
	scope      scopeKey
	name       string
	parameters string
}

type query struct {
	key         queryKey
	ctx         context.Context
	hitSource   []Source
	data        []byte
	expiresAt   time.Time
	recent      *list.Element
	shared      singleflight.Group
	users       int
	started     uint64
	applied     uint64
	invalidated bool
}

// Cache 职责：管理有容量上限的进程内请求缓存；使用：按应用服务器请求域复用；边界：会话结束后清理对应缓存，数据以副本交付。
type Cache struct {
	mu         sync.Mutex
	queries    map[queryKey]*query
	scopes     map[scopeKey]struct{}
	recent     list.List
	bytes      int
	maxEntries int
	maxBytes   int
	now        func() time.Time
}

// New 职责：创建带条目和字节上限的缓存；使用：业务读取开始前初始化；边界：任一上限小于等于零时只支持在途共享。
func New(maxEntries, maxBytes int) *Cache {
	return &Cache{
		queries:    make(map[queryKey]*query),
		scopes:     make(map[scopeKey]struct{}),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		now:        time.Now,
	}
}

// Read 职责：先查有效数据，再共享或启动一次读取；使用：force 跳过已保存的数据；边界：共享读取以会话上下文为父，load 自行设置超时。
func (cache *Cache) Read(caller context.Context, scope Scope, method Method, force bool, load func(context.Context) (Result, error)) ([]byte, error) {
	if err := caller.Err(); err != nil {
		return nil, err
	}
	key := queryKey{scope: scope.key(), name: method.Name, parameters: method.Parameters}
	cache.mu.Lock()
	if err := scope.Context.Err(); err != nil {
		cache.mu.Unlock()
		return nil, err
	}
	current := cache.queries[key]
	if current == nil {
		cache.watchScope(key.scope, scope.Context)
		current = &query{key: key, ctx: scope.Context, hitSource: slices.Clone(method.HitSource)}
		cache.queries[key] = current
	}
	if current.recent != nil {
		if !cache.now().Before(current.expiresAt) {
			cache.removeData(current)
		} else if method.CacheFor > 0 && !force {
			cache.recent.MoveToFront(current.recent)
			data := bytes.Clone(current.data)
			cache.mu.Unlock()
			return data, nil
		}
	}
	current.users++
	cache.mu.Unlock()
	defer cache.release(current)

	if !method.ShareRequest {
		ctx, cancel := context.WithCancel(caller)
		defer cancel()
		stop := context.AfterFunc(scope.Context, cancel)
		defer stop()
		data, err := cache.fetch(ctx, current, method.CacheFor, load)
		return cache.deliver(caller, current, data, err)
	}

	// 每条查询记录持有自己的合并组，失效后新读取会进入新的组。
	ready := current.shared.DoChan(strconv.FormatBool(force), func() (any, error) {
		return cache.fetch(scope.Context, current, method.CacheFor, load)
	})
	select {
	case <-caller.Done():
		return nil, caller.Err()
	case result := <-ready:
		if result.Err != nil {
			return cache.deliver(caller, current, nil, result.Err)
		}
		return cache.deliver(caller, current, result.Val.([]byte), nil)
	}
}

// Invalidate 职责：丢弃同一会话匹配写入来源的查询；使用：业务写入成功后调用；边界：失效前的在途响应不能填充新缓存。
func (cache *Cache) Invalidate(scope Scope, source Source) {
	if source.Method == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	key := scope.key()
	for _, current := range cache.queries {
		if current.key.scope != key {
			continue
		}
		for _, rule := range current.hitSource {
			if rule.Method == source.Method && (rule.Target == "" || rule.Target == source.Target) {
				cache.discard(current)
				break
			}
		}
	}
}

// watchScope 让会话取消时清除本作用域的数据，调用方持有缓存锁。
func (cache *Cache) watchScope(scope scopeKey, ctx context.Context) {
	if _, exists := cache.scopes[scope]; exists {
		return
	}
	cache.scopes[scope] = struct{}{}
	context.AfterFunc(ctx, func() {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		for _, current := range cache.queries {
			if current.key.scope == scope {
				cache.discard(current)
			}
		}
		delete(cache.scopes, scope)
	})
}

// fetch 在原查询仍有效时读取，并按请求开始顺序接纳成功结果。
func (cache *Cache) fetch(ctx context.Context, current *query, limit time.Duration, load func(context.Context) (Result, error)) ([]byte, error) {
	cache.mu.Lock()
	if cache.queries[current.key] != current {
		cache.mu.Unlock()
		return nil, ErrInvalidated
	}
	current.users++
	current.started++
	order := current.started
	cache.mu.Unlock()
	defer cache.release(current)

	result, err := load(ctx)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !current.invalidated && current.ctx.Err() == nil && ctx.Err() == nil && order > current.applied {
		current.applied = order
		if err == nil {
			cache.save(current, result.Data, min(limit, result.FreshFor))
		} else {
			cache.removeData(current)
		}
	}
	return result.Data, err
}

// deliver 在共享结果交给调用方前核对其会话与查询是否仍有效。
func (cache *Cache) deliver(caller context.Context, current *query, data []byte, err error) ([]byte, error) {
	if err := caller.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := current.ctx.Err(); err != nil {
		return nil, err
	}
	if current.invalidated {
		return nil, ErrInvalidated
	}
	return bytes.Clone(data), nil
}

// save 替换一个查询的缓存，按最近使用顺序淘汰空间不足的条目。
func (cache *Cache) save(current *query, data []byte, ttl time.Duration) {
	cache.removeData(current)
	if ttl <= 0 || cache.maxEntries <= 0 || cache.maxBytes <= 0 || len(data) > cache.maxBytes {
		return
	}
	for cache.recent.Len() >= cache.maxEntries || len(data) > cache.maxBytes-cache.bytes {
		oldest := cache.recent.Back().Value.(*query)
		cache.removeData(oldest)
		cache.prune(oldest)
	}
	current.data = data
	current.expiresAt = cache.now().Add(ttl)
	current.recent = cache.recent.PushFront(current)
	cache.bytes += len(data)
}

// removeData 移除缓存数据，保留还在执行的读取。
func (cache *Cache) removeData(current *query) {
	if current.recent == nil {
		return
	}
	cache.recent.Remove(current.recent)
	cache.bytes -= len(current.data)
	current.recent = nil
	current.data = nil
}

// prune 清理无缓存且无活动请求的记录，旧请求不能删除新记录。
func (cache *Cache) prune(current *query) {
	if current.users == 0 && current.recent == nil && cache.queries[current.key] == current {
		delete(cache.queries, current.key)
	}
}

// release 结束一次读取或等待，在最后一个使用方离开时回收空记录。
func (cache *Cache) release(current *query) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	current.users--
	cache.prune(current)
}

// discard 断开旧查询供后续读取开始新请求，保留旧记录供等待者发现失效。
func (cache *Cache) discard(current *query) {
	current.invalidated = true
	cache.removeData(current)
	delete(cache.queries, current.key)
}
