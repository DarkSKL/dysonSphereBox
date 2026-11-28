package candidates

// 文件职责：提供单请求内有界的成功结果复用；使用：纯校验及精确状态指纹；边界：不缓存失败，不共享跨请求状态，不修改返回值。

import (
	"context"
	"sync"
)

// Counts 职责：区分校验请求、实际执行和复用；使用：性能事实；边界：等待失败后的重新执行不冒充命中。
type Counts struct {
	Requested int `json:"requested"` // 职责：记录调用次数；使用：覆盖分母；边界：包括取消。
	Executed  int `json:"executed"`  // 职责：记录真实工作次数；使用：节省对比；边界：失败也计入。
	Reused    int `json:"reused"`    // 职责：记录成功复用次数；使用：命中证据；边界：不计失败。
	Uncached  int `json:"uncached"`  // 职责：记录容量已满的独立执行；使用：内存边界解释；边界：仍完整检查。
}

// entry 职责：同步同状态正在执行的工作；使用：并发重复请求；边界：关闭done之后值不可再改。
type entry[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// Memo 职责：限制缓存条目并合并并发成功请求；使用：每次Generate独立实例；边界：V须由调用方按不可变值保存，读取后需要修改时自行复制。
type Memo[V any] struct {
	mutex   sync.Mutex
	limit   int
	entries map[[32]byte]*entry[V]
	counts  Counts
}

// NewMemo 职责：创建有界请求缓存；使用：请求开始；边界：非正上限表示不缓存，不回退为无界。
func NewMemo[V any](limit int) *Memo[V] {
	return &Memo[V]{limit: limit, entries: map[[32]byte]*entry[V]{}}
}

// Do 职责：复用相同状态的完整成功结果；使用：指纹已包含全部语义输入时；边界：执行者失败后等待者重新执行，任一调用取消都不取消其他调用。
func (m *Memo[V]) Do(ctx context.Context, key [32]byte, work func() (V, error)) (V, error) {
	return m.DoRetaining(ctx, key, work, nil)
}

// DoRetaining 合并同键在途计算；keep 只决定成功值是否继续驻留，超出大小的值仍交付当前调用方。
func (m *Memo[V]) DoRetaining(ctx context.Context, key [32]byte, work func() (V, error), keep func(V) bool) (V, error) {
	var zero V
	m.mutex.Lock()
	m.counts.Requested++
	m.mutex.Unlock()
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		m.mutex.Lock()
		if old := m.entries[key]; old != nil {
			m.mutex.Unlock()
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-old.done:
			}
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			if old.err != nil {
				continue
			}
			m.mutex.Lock()
			m.counts.Reused++
			m.mutex.Unlock()
			return old.value, nil
		}
		m.counts.Executed++
		if len(m.entries) >= m.limit {
			m.counts.Uncached++
			m.mutex.Unlock()
			value, err := work()
			if err == nil {
				err = ctx.Err()
			}
			return value, err
		}
		current := &entry[V]{done: make(chan struct{})}
		m.entries[key] = current
		m.mutex.Unlock()
		value, err := work()
		if err == nil {
			err = ctx.Err()
		}
		retain := err == nil && (keep == nil || keep(value))
		m.mutex.Lock()
		current.value, current.err = value, err
		if !retain {
			delete(m.entries, key)
		}
		close(current.done)
		m.mutex.Unlock()
		return value, err
	}
}

// Counts 职责：返回同步快照；使用：请求结束诊断；边界：不返回内部映射。
func (m *Memo[V]) Counts() Counts { m.mutex.Lock(); defer m.mutex.Unlock(); return m.counts }

// StableUnique 职责：按完整有效键保留首次出现的候选；使用：各模式共同规划；边界：键的等价性由模式证明，不按输出面积猜测。
func StableUnique[V any, K comparable](values []V, key func(V) K) []V {
	seen := map[K]bool{}
	out := make([]V, 0, len(values))
	for _, value := range values {
		k := key(value)
		if !seen[k] {
			seen[k] = true
			out = append(out, value)
		}
	}
	return out
}
