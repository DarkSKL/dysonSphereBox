package build

// 文件职责：把引擎提供的完整路径绑定到候选旁路身份和计数。
// 使用：Solve工具入口及引擎生命周期观察；边界：不导入工作流，不创建、重试或选择算法候选，不保存实体图。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// candidateObservationKey 职责：绑定任务旁路目录；使用：工具上下文；边界：不跨任务共享。
type candidateObservationKey struct{}

// observedCandidate 职责：保存单个完整Solve的旁路状态；使用：并发计数及生命周期；边界：仅持有通知器、计时器和身份值，不持有领域产物。
type observedCandidate struct {
	// path 职责：保存引擎完整作用域；使用：父组撤销匹配；边界：不表示当前算法目标。
	path string
	// preview 职责：保存旁路子会话；使用：事件分流；边界：未启用预览时为空。
	preview *generationPreview
	// performance 职责：保存独占候选计数；使用：Solve及Validate计时；边界：不参与评分。
	performance *generationPerformance
	// finish 职责：幂等汇总候选计数；使用：引擎终结；边界：不保存或执行算法回调。
	finish func(error)
}

// candidateObservations 职责：维护任务内旁路身份目录；使用：引擎事件适配；边界：目录不是候选调度器或领域状态真源。
type candidateObservations struct {
	// lifecycle 职责：串行最佳发布、选区撤销及任务收尾；使用：旁路事件事务；边界：不保护或调度算法实体。
	lifecycle sync.Mutex
	// best 职责：仅保留一个最佳的不可变渲染值与评分；使用：局部成功和父作用域修正；边界：不保留VerifiedSnapshot或实体图。
	best *observationBest
	// mu 职责：保护旁路目录；使用：并发绑定和读取；边界：发送事件前释放。
	mu sync.Mutex
	// entries 职责：按固定路径保存旁路会话；使用：生命周期匹配；边界：不存实体或产物。
	entries map[string]*observedCandidate
	// root 职责：保留任务通知入口；使用：协议兼容与最终发布；边界：无预览时为空。
	root *generationPreview
	// closed 职责：阻止结束后的新绑定；使用：任务退出；边界：不取消算法执行。
	closed bool
}

// candidateObservationPath 职责：从工具路径提取最近引擎候选作用域；使用：Solve与事件匹配；边界：不遍历工作流或猜测当前winner。
func candidateObservationPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || len(path) > 8192 || strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("候选观察路径为空或无效")
	}
	if i := strings.LastIndex(path, "/candidate:"); i >= 0 {
		end := i + len("/candidate:")
		for end < len(path) && path[end] >= '0' && path[end] <= '9' {
			end++
		}
		if end == i+len("/candidate:") {
			return "", fmt.Errorf("候选观察路径缺少稳定序号")
		}
		path = path[:end]
	}
	return path, nil
}

// candidateObservationIdentity 职责：从稳定完整路径计算唯一ID及稳定平台序号；使用：并发开始前；边界：序号允许稀疏，绝不使用完成时序计数。
func candidateObservationIdentity(path string, ordinal int) (string, uint64, error) {
	if ordinal < 0 {
		return "", 0, fmt.Errorf("候选观察序号不能为负")
	}
	digest := sha256.Sum256([]byte(path))
	// 嵌套组用有序配对展开，避免不同外层分支的同一下标占用同一平台；数值限制在JS安全整数内。
	value := uint64(0)
	count := 0
	last := uint64(0)
	for _, part := range strings.Split(path, "/") {
		if !strings.HasPrefix(part, "candidate:") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimPrefix(part, "candidate:"), 10, 32)
		if err != nil {
			return "", 0, err
		}
		if count == 0 {
			value = n
		} else {
			sum := value + n
			if sum > 100000000 {
				return "", 0, fmt.Errorf("候选观察嵌套序号超出安全范围")
			}
			value = sum*(sum+1)/2 + n
		}
		last = n
		count++
	}
	if count == 0 {
		value = uint64(ordinal)
	} else if last != uint64(ordinal) {
		return "", 0, fmt.Errorf("候选观察路径与引擎序号不一致")
	}
	return "candidate-" + hex.EncodeToString(digest[:]), value + 1, nil
}

// WithCandidateObservation 职责：绑定引擎已确定的完整Solve作用域；使用：每个Solve工具前传StepContext的Node和零基Candidate；边界：不为运输或球面外层组创建平台，同一路径重复绑定幂等。
func WithCandidateObservation(ctx context.Context, path string, ordinal int) (context.Context, error) {
	canonical, err := candidateObservationPath(path)
	if err != nil {
		return ctx, err
	}
	id, order, err := candidateObservationIdentity(canonical, ordinal)
	if err != nil {
		return ctx, err
	}
	state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
	if state == nil {
		return ctx, nil
	}
	if err := ctx.Err(); err != nil {
		return ctx, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closed {
		return ctx, fmt.Errorf("生成旁路已经结束")
	}
	entry := state.entries[canonical]
	if entry == nil {
		local, finish := beginCandidatePerformance(ctx, canonical, ordinal)
		entry = &observedCandidate{path: canonical, performance: performanceFromContext(local), finish: finish}
		if root := state.root; root != nil {
			entry.preview = newObservationPreview(root, id, order, canonical)
		}
		state.entries[canonical] = entry
	}
	if entry.preview != nil {
		ctx = context.WithValue(ctx, generationPreviewKey{}, entry.preview)
	}
	if entry.performance != nil {
		ctx = context.WithValue(ctx, performanceKey{}, entry.performance)
	}
	return ctx, nil
}

// matching 职责：按引擎路径读取完整候选旁路；使用：父组拒绝及局部选优；边界：只返回目录副本，不持锁发送事件。
func (state *candidateObservations) matching(prefix string) []*observedCandidate {
	state.mu.Lock()
	defer state.mu.Unlock()
	keys := []string{}
	for path := range state.entries {
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/") {
			keys = append(keys, path)
		}
	}
	sort.Strings(keys)
	result := make([]*observedCandidate, 0, len(keys))
	for _, path := range keys {
		result = append(result, state.entries[path])
	}
	return result
}

// ObserveCandidate 职责：适配引擎started/succeeded/rejected/selected值事件；使用：RunOptions.Observe，成功事件传Snapshot中的VerifiedSnapshot；边界：started不创建平台，selected只反映引擎决定，不推断winner。
func ObserveCandidate(ctx context.Context, kind, path string, ordinal int, verified *VerifiedSnapshot, failure error) error {
	state, _ := ctx.Value(candidateObservationKey{}).(*candidateObservations)
	if state == nil {
		return nil
	}
	state.mu.Lock()
	closed := state.closed
	state.mu.Unlock()
	if closed {
		return nil
	}
	canonical, err := candidateObservationPath(path)
	if err != nil {
		return err
	}
	switch kind {
	case "started":
		return nil
	case "succeeded":
		if verified == nil {
			return nil
		}
		if err := CompleteValidatedObservation(ctx, *verified); err != nil {
			return err
		}
		for _, entry := range state.matching(canonical) {
			if entry.path == canonical {
				entry.finish(nil)
			}
		}
	case "rejected":
		state.lifecycle.Lock()
		defer state.lifecycle.Unlock()
		state.clearBestInScopeLocked(canonical)
		if failure == nil {
			failure = fmt.Errorf("引擎拒绝候选")
		}
		for _, entry := range state.matching(canonical) {
			entry.preview.finish("rejected")
			entry.finish(failure)
		}
	case "selected":
		if verified == nil {
			return nil
		}
		state.lifecycle.Lock()
		defer state.lifecycle.Unlock()
		winner := geometryObservationSession(verified.Geometry())
		parent := canonical
		if i := strings.LastIndex(parent, "/candidate:"); i >= 0 {
			parent = parent[:i]
		}
		state.reconcileBestLocked(ctx, parent, *verified)
		for _, entry := range state.matching(parent) {
			if entry.preview != winner {
				entry.preview.finish("pruned")
			}
		}
	default:
		return fmt.Errorf("未知候选观察事件：%s", kind)
	}
	return nil
}

// close 职责：在引擎退出后终结全部旁路计时和失败场景；使用：任务Finish；边界：不提前取消或等待算法工作，等待属于引擎。
func (state *candidateObservations) close(failure error) {
	state.lifecycle.Lock()
	defer state.lifecycle.Unlock()
	state.mu.Lock()
	state.closed = true
	state.mu.Unlock()
	if failure != nil {
		state.clearBestInScopeLocked("")
	}
	for _, entry := range state.matching("") {
		entry.finish(failure)
		if failure != nil {
			entry.preview.finish("rejected")
		}
	}
	state.best = nil
}
