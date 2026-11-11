package build

// 文件职责：在大型原生直连候选已有完整可行结果后，有界结束额外比较。
// 使用：连续候选工作槽的私有执行上下文；边界：不修改父请求，不把取消中的局部实体作为成功结果。

import (
	"context"
	"sync"
	"time"
)

// connectedFeasibleWave 职责：管理候选执行取消及成功后宽限计时器；使用：大型原生直连候选；边界：只取消执行上下文，调度器继续用父上下文归约完整结果。
type connectedFeasibleWave struct {
	// parent/ctx/cancel 职责：区分请求寿命和私有执行寿命；使用：候选执行与父请求错误检查；边界：不取消parent。
	parent context.Context
	ctx    context.Context
	cancel context.CancelFunc
	// once/mutex/timer/closed 职责：保证首个成功只启动一次计时且清理不与启动竞争；使用：并发候选完成及串行归约之后；边界：关闭后不允许重新启动计时器。
	once   sync.Once
	mutex  sync.Mutex
	timer  *time.Timer
	closed bool
}

// newConnectedFeasibleWave 职责：严格限制有界可行收口到大型原生密铺mode1；使用：每个入口候选组开始前；边界：其他请求返回nil，保留原执行上下文与搜索语义。
func newConnectedFeasibleWave(parent context.Context, req Request) *connectedFeasibleWave {
	root, _ := parent.Value(densityPolarRootKey{}).(bool)
	if !root || req.Mode != 1 || req.Density != densityDense || req.Plan.MachineCount < denseNativeLargeMachineThreshold {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	return &connectedFeasibleWave{parent: parent, ctx: ctx, cancel: cancel}
}

// connectedFeasibleWaveGrace 职责：计算首个完整成功后的等待时间；使用：计时器建立前；边界：通常五秒，父截止不足十秒时为返回结果预留五秒，不延长父截止。
func connectedFeasibleWaveGrace(parent context.Context, now time.Time) time.Duration {
	grace := 5 * time.Second
	if deadline, ok := parent.Deadline(); ok {
		if remaining := deadline.Sub(now); remaining < 10*time.Second {
			grace = max(time.Duration(0), remaining-5*time.Second)
		}
	}
	return grace
}

// complete 职责：在结果及诊断均已赋值后启动唯一宽限计时器；使用：候选finish完整成功的返回分支；边界：不读取或修改候选结果，关闭后不创建计时器。
func (wave *connectedFeasibleWave) complete() {
	if wave == nil {
		return
	}
	wave.once.Do(func() {
		wave.mutex.Lock()
		defer wave.mutex.Unlock()
		if !wave.closed {
			wave.timer = time.AfterFunc(connectedFeasibleWaveGrace(wave.parent, time.Now()), wave.cancel)
		}
	})
}

// close 职责：结束候选组私有执行并停止未触发的宽限计时器；使用：调度器等待全部执行及归约返回之后；边界：与complete并发安全、可重复调用，不影响父请求。
func (wave *connectedFeasibleWave) close() {
	if wave == nil {
		return
	}
	wave.mutex.Lock()
	wave.closed = true
	if wave.timer != nil {
		wave.timer.Stop()
	}
	wave.mutex.Unlock()
	wave.cancel()
}
