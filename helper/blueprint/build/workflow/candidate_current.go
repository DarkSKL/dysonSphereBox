package workflow

// 文件职责：持有已验证父结果的不可变候选基线；使用：可选精修；边界：不派发假工具、不改变父实例，不让基线进入候选计数。

import (
	"context"
	"fmt"
	"math"
)

// currentCandidate 职责：冻结完整父结果并绑定真实作用域；使用：includeCurrent 搜索起点；边界：基线快照计入保留预算，取消/克隆/评分失败致命且归还预算。
func (e *executor) currentCandidate(ctx context.Context, parent *workspace, score ScoreFunc, compare Comparator) (result *candidateResult, err error) {
	if parent.region != "" {
		return nil, fmt.Errorf("includeCurrent requires complete root validation")
	}
	parent.instance.mu.Lock()
	valid := parent.instance.validationValid("")
	size, sizeErr := stateSize(parent.instance.slots)
	if parent.counted {
		size, sizeErr = stateCloneSize(parent.instance.slots)
	}
	parent.instance.mu.Unlock()
	if !valid {
		return nil, fmt.Errorf("includeCurrent parent is not fully validated")
	}
	if sizeErr != nil {
		return nil, sizeErr
	}
	if err = e.budget.resize(size); err != nil {
		return nil, err
	}
	baseline := &candidateResult{baseline: true, snapshotBytes: size}
	defer func() {
		if result != baseline {
			e.releaseResult(baseline)
		}
	}()
	baseline.snapshot, err = parent.instance.Freeze()
	if err != nil {
		return nil, err
	}
	origin := baseline.snapshot.origin
	if origin.path == "" {
		return nil, fmt.Errorf("validated baseline has no real result scope")
	}
	baseline.path = origin.path
	baseline.index = origin.ordinal
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if compare == nil && score != nil {
		baseline.score, err = score(baseline.snapshot)
		if err != nil {
			return nil, err
		}
		baseline.score = append([]float64(nil), baseline.score...)
		for _, v := range baseline.score {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("nonfinite baseline score")
			}
		}
	}
	return baseline, nil
}
