package build

// 文件职责：把运输档位重试变为显式、有限的领域规格。
// 使用：工作流枚举档位并在容量失败时选择下一规格；边界：本文件不调度、不求解布局、不改变生产设备或产量。

import (
	"context"
	"fmt"
	"sort"
)

// TransportCandidateSpec 职责：描述一次实际运输设备选择；使用：外层候选输入；边界：档位只允许不低于已经验证的输入。
type TransportCandidateSpec struct {
	BeltLevel     int `json:"beltLevel"`
	InserterLevel int `json:"inserterLevel"`
}

// EnumerateTransportCandidates 职责：按最少升档步数稳定列举至多十二种等级；使用：引擎first候选组；边界：是否尝试下一档由引擎的容量失败白名单决定。
func EnumerateTransportCandidates(ctx context.Context, input *PreparedInput) ([]TransportCandidateSpec, error) {
	if input == nil {
		return nil, fmt.Errorf("运输规格缺少规范输入")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base := input.request.Setting
	result := []TransportCandidateSpec{}
	for belt := base.BeltLv; belt <= 3; belt++ {
		for inserter := base.InserterLv; inserter <= 4; inserter++ {
			result = append(result, TransportCandidateSpec{belt, inserter})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		// 超大厂的独立lane会主导实体数和全图布线；先试最高带档，保留全部原等级作为后续候选。
		// 这是有限搜索的工作量排序，不声称低档物理不可行，也不改变原请求、制造设备或目标产量。
		if largeFactoryWorkload(ctx) && left.BeltLevel != right.BeltLevel {
			return left.BeltLevel > right.BeltLevel
		}
		if left.BeltLevel+left.InserterLevel != right.BeltLevel+right.InserterLevel {
			return left.BeltLevel+left.InserterLevel < right.BeltLevel+right.InserterLevel
		}
		return left.BeltLevel < right.BeltLevel
	})
	return result, nil
}

// ApplyTransportCandidate 职责：只应用一个明确运输档位并生成不可变计划；使用：运输再准备第一步；边界：不自动升档、不执行容量校验、不重载设备数据或流图。
func ApplyTransportCandidate(ctx context.Context, input *PreparedInput, spec TransportCandidateSpec) (*InputTransportPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input == nil || input.fingerprint == "" || input.dataset == nil || input.graph == nil || input.policy == nil {
		return nil, inputStageError("运输档位应用缺少完整原输入")
	}
	setting := input.request.Setting
	if spec.BeltLevel < setting.BeltLv || spec.BeltLevel > 3 || spec.InserterLevel < setting.InserterLv || spec.InserterLevel > 4 {
		return nil, fmt.Errorf("运输规格超出已验证档位范围")
	}
	request, err := input.RequestSnapshot()
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	request.Setting.BeltLv, request.Setting.InserterLv = spec.BeltLevel, spec.InserterLevel
	// 只同步明确等级对应的设备事实，不把旧BeltSpeed当作可配置能力。
	request.Setting, err = normalizeSetting(request.Setting)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "applied-transport-plan", inputRequestIdentity(request), input.fingerprint)
	if err != nil {
		return nil, err
	}
	identity.bytes += input.bytes
	return &InputTransportPlan{inputStageIdentity: identity, request: request, dataset: input.dataset, policy: input.policy, base: input}, nil
}
