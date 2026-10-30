package quant

// 文件职责：执行隔离的 P5a 单配方喷涂候选搜索；使用：纯 Go 调用与离线验证；边界：不接服务缓存、Wails、蓝图或持久化，不承诺全局最优。

import (
	"context"
	"errors"
	"sort"
	"time"
)

// SuggestState 职责：区分搜索终态；使用：调用者判断评估完整性；边界：非 completed 不允许推荐。
type SuggestState string

// 搜索终态职责：描述预算/取消边界；使用：SuggestResult；边界：不代表单次 Solve 的错误类型。
const (
	// SuggestCompleted 职责：表示计划已全部执行；使用：推荐门禁；边界：仍需检查失败及物理门禁。
	SuggestCompleted SuggestState = "completed"
	// SuggestLimited 职责：表示八项上限截断；使用：显示评估而非推荐；边界：不推测遗漏项。
	SuggestLimited SuggestState = "limited"
	// SuggestCanceled 职责：表示父 context 取消；使用：结束计算；边界：不推荐已完成的部分候选。
	SuggestCanceled SuggestState = "canceled"
	// SuggestTimeout 职责：表示父或总 context 到期；使用：结束计算；边界：合作式而非硬实时。
	SuggestTimeout SuggestState = "timeout"
)

// SuggestChoice 职责：标识一项单配方变更；使用：评估结果及显式应用方；边界：索引为零基，不改变建筑选择。
type SuggestChoice struct {
	// RecipeIndex 职责：授权配方下标；使用：定位 SchemeForRecipe；边界：不是 Recipe.ID。
	RecipeIndex int
	// Mode 职责：候选模式；使用：变更 ProductionMode；边界：仅 0、1、2。
	Mode int
	// Points 职责：候选点数；使用：变更 IncreasePoints；边界：仅 0、1、2、4。
	Points int
}

// SuggestCandidate 职责：保留一次成功评估；使用：对比制造机器整数数；边界：Result 不是可信生成方案。
type SuggestCandidate struct {
	// SuggestChoice 职责：保存本次单项变更；使用：显式对照；边界：不会自动应用。
	SuggestChoice
	// Result 职责：保留真实求解响应；使用：机器、功率及原料对照；边界：功率不作为评分。
	Result Result
	// Assessable 职责：记录物理账本门禁；使用：全计划推荐判定；边界：不证明游戏真实性。
	Assessable bool
}

// SuggestFailure 职责：保留一次候选错误；使用：按枚举顺序观察失败；边界：不包含未执行候选。
type SuggestFailure struct {
	// SuggestChoice 职责：标识失败变更；使用：按枚举序诊断；边界：不是成功候选。
	SuggestChoice
	// Code 职责：保存求解错误码；使用：错误分支；边界：未知 Go error 归 Q_E_INTERNAL。
	Code string
}

// SuggestResult 职责：承载基线、计数与终态；使用：内部 Go 候选调用；边界：不是 Wails DTO 或保存格式。
type SuggestResult struct {
	// State 职责：搜索终态；使用：禁止部分结果推荐；边界：非法输入或基线错误使用 Go error。
	State SuggestState
	// Eligible 职责：合法去重候选总数；使用：核对授权范围；边界：不含基线。
	Eligible int
	// Planned 职责：计划执行数量；使用：全成功门禁；边界：最多八项。
	Planned int
	// Attempted 职责：已调用候选数量；使用：核对部分终态；边界：不含基线。
	Attempted int
	// Succeeded 职责：成功返回的候选数；使用：评估计数；边界：物理不可判定仍算 Solve 成功。
	Succeeded int
	// Failed 职责：候选错误数；使用：阻断推荐；边界：不含 context 中断的在途调用。
	Failed int
	// Omitted 职责：上限省略数；使用：解释 limited；边界：不是失败数。
	Omitted int
	// Baseline 职责：原请求求解结果；使用：整数机器数比较；边界：取消前可能尚无基线。
	Baseline *Result
	// BaselineAssessable 职责：基线物理门禁；使用：全计划推荐判定；边界：不改变原结果。
	BaselineAssessable bool
	// Candidates 职责：成功候选；使用：按枚举顺序评估；边界：只含已成功求解项。
	Candidates []SuggestCandidate
	// Failures 职责：失败候选码；使用：按枚举顺序诊断；边界：不伪造未执行项。
	Failures []SuggestFailure
	// Recommendation 职责：可选的最少整数机器候选；使用：调用方后续显式选择；边界：nil 表示不推荐，不触发应用或生成。
	Recommendation *SuggestCandidate
}

// Suggest 职责：串行评估显式授权的 Vanilla 喷涂候选；使用：内部纯 Go 计算；边界：最多二十秒合作式预算，只调用真实 Solve，无共享测试钩子。
func Suggest(parent context.Context, req Request, authorized []int) (SuggestResult, error) {
	return suggestWithSolver(parent, req, authorized, Solve)
}

// suggestWithSolver 职责：封装串行搜索及终态门禁；使用：生产入口与同包可控时序测试；边界：求解器仅参数注入，不修改生产全局状态。
// 逻辑流：1. 校验/枚举 => 2. 原请求基线 => 3. 前八项串行求解 => 4. 全计划物理检查与稳定选取；任意总 context 终止优先清除推荐。
func suggestWithSolver(parent context.Context, req Request, authorized []int, solve func(context.Context, Request) (Result, error)) (SuggestResult, error) {
	if parent == nil {
		return SuggestResult{}, qerror("Q_E_BAD_REQUEST", "候选搜索需要 context")
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if req.Dataset != "" && req.Dataset != "Vanilla" {
		return SuggestResult{}, qerror("Q_E_DATASET", "候选搜索仅支持 Vanilla")
	}
	d, err := LoadDataset(req.Dataset)
	if err != nil {
		return SuggestResult{}, err
	}
	if _, err := suggestIndices(d, authorized); err != nil {
		return SuggestResult{}, err
	}
	choices, choiceErr := suggestChoices(d, req, authorized)
	out := SuggestResult{Eligible: len(choices), Planned: min(len(choices), 8)}
	out.Omitted = out.Eligible - out.Planned
	if suggestStopped(ctx, &out) {
		return out, nil
	}
	base, err := solve(ctx, suggestCloneRequest(req))
	if suggestStopped(ctx, &out) {
		return out, nil
	}
	if err != nil {
		return SuggestResult{}, err
	}
	if choiceErr != nil {
		return SuggestResult{}, choiceErr
	}
	out.Baseline = &base
	out.BaselineAssessable = suggestAssess(d, req, base)
	allAssessable := out.BaselineAssessable
	for _, choice := range choices[:out.Planned] {
		if suggestStopped(ctx, &out) {
			return out, nil
		}
		candidateRequest := suggestCloneRequest(req)
		for len(candidateRequest.Scheme.SchemeForRecipe) <= choice.RecipeIndex {
			candidateRequest.Scheme.SchemeForRecipe = append(candidateRequest.Scheme.SchemeForRecipe, RecipeScheme{})
		}
		candidateRequest.Scheme.SchemeForRecipe[choice.RecipeIndex].ProductionMode = choice.Mode
		candidateRequest.Scheme.SchemeForRecipe[choice.RecipeIndex].IncreasePoints = choice.Points
		out.Attempted++
		result, solveErr := solve(ctx, candidateRequest)
		if suggestStopped(ctx, &out) {
			return out, nil
		}
		if solveErr != nil {
			out.Failed++
			code := "Q_E_INTERNAL"
			var qe *Error
			if errors.As(solveErr, &qe) {
				code = qe.Code
			}
			out.Failures = append(out.Failures, SuggestFailure{choice, code})
			continue
		}
		out.Succeeded++
		assessable := suggestAssess(d, candidateRequest, result)
		allAssessable = allAssessable && assessable
		out.Candidates = append(out.Candidates, SuggestCandidate{choice, result, assessable})
	}
	out.State = SuggestCompleted
	if out.Omitted > 0 {
		out.State = SuggestLimited
	}
	if out.State == SuggestCompleted && out.Planned > 0 && out.Succeeded == out.Planned && allAssessable {
		for i := range out.Candidates {
			candidate := &out.Candidates[i]
			if candidate.Result.MachineCount < base.MachineCount && (out.Recommendation == nil || suggestLess(*candidate, *out.Recommendation)) {
				out.Recommendation = candidate
			}
		}
	}
	suggestStopped(ctx, &out)
	return out, nil
}

// suggestStopped 职责：统一取消/总超时优先级；使用：每次调用边界与最终出口；边界：不把单次 Solve 超时升级为总超时。
func suggestStopped(ctx context.Context, out *SuggestResult) bool {
	if ctx.Err() == nil {
		return false
	}
	out.State = SuggestCanceled
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.State = SuggestTimeout
	}
	out.Recommendation = nil
	return true
}

// suggestLess 职责：冻结推荐比较次序；使用：完整评估后的选择；边界：不按 LP 成本、功率或耗时排序。
func suggestLess(a, b SuggestCandidate) bool {
	if a.Result.MachineCount != b.Result.MachineCount {
		return a.Result.MachineCount < b.Result.MachineCount
	}
	if a.RecipeIndex != b.RecipeIndex {
		return a.RecipeIndex < b.RecipeIndex
	}
	if a.Mode != b.Mode {
		return a.Mode < b.Mode
	}
	return a.Points < b.Points
}

// suggestIndices 职责：校验授权并复制排序；使用：基线前拒绝非法授权；边界：不读取配置以免遮蔽真实基线错误。
func suggestIndices(d *Dataset, authorized []int) ([]int, error) {
	if len(authorized) > 64 {
		return nil, qerror("Q_E_BAD_REQUEST", "授权配方最多 64 项")
	}
	indices := append([]int(nil), authorized...)
	sort.Ints(indices)
	for i, index := range indices {
		if index < 0 || index >= len(d.Recipes) || (i > 0 && index == indices[i-1]) {
			return nil, qerror("Q_E_BAD_REQUEST", "授权配方索引越界或重复")
		}
	}
	return indices, nil
}

// suggestChoices 职责：按固定序列枚举合法归一差异；使用：候选求解前确定完整 eligible；边界：不截断授权、不使用配方 ID、不修改数据集。
func suggestChoices(d *Dataset, req Request, authorized []int) ([]SuggestChoice, error) {
	indices, err := suggestIndices(d, authorized)
	if err != nil {
		return nil, err
	}
	var out []SuggestChoice
	for _, index := range indices {
		r := d.Recipes[index]
		if r.Proliferator == 8 {
			continue
		}
		original := RecipeScheme{}
		if index < len(req.Scheme.SchemeForRecipe) {
			original = req.Scheme.SchemeForRecipe[index]
		}
		normalized, err := normalizeRecipeScheme(d, r, original)
		if err != nil {
			return nil, err
		}
		for _, pair := range [][2]int{{0, 0}, {1, 1}, {1, 2}, {1, 4}, {2, 1}, {2, 2}, {2, 4}} {
			mode, points := pair[0], pair[1]
			if mode > 0 && r.Proliferator&(1<<uint(mode-1)) == 0 {
				continue
			}
			next := original
			next.ProductionMode, next.IncreasePoints = mode, points
			next, err = normalizeRecipeScheme(d, r, next)
			if err != nil {
				return nil, err
			}
			if next != normalized {
				out = append(out, SuggestChoice{index, mode, points})
			}
		}
	}
	return out, nil
}
