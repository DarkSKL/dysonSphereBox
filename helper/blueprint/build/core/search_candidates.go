package build

// 文件职责：为有限布局搜索生成有界、稳定且与错误返回解耦的候选证据。
// 使用：直连、配送和塔组候选循环完成一次真实评估后调用。
// 边界：只复制摘要，不改变搜索预算、布局选择、取消流程或实际错误返回；不收录逐格寻路和装箱探测。

import (
	"errors"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// searchCandidateReasonLimit 职责：限制每项原因的字符数；使用：防止错误链重复放大诊断；边界：只截断证据文案，原错误保持完整。
const searchCandidateReasonLimit = 512

// newSearchCandidateDiagnostic 职责：将一次已评估候选转为纯值诊断；使用：各模式有限候选循环；边界：普通错误仅作诊断归类，不能替代调用方的原错误。
func newSearchCandidateDiagnostic(scope string, index int, parameters, stage string, err error) SearchCandidateDiagnostic {
	diagnostic := SearchCandidateDiagnostic{
		Key: fmt.Sprintf("%s/%d", scope, index+1), Parameters: parameters,
		Scope: scope, Stage: stage, Status: "passed", Reason: "候选通过本阶段完整检查",
	}
	if err == nil {
		return diagnostic
	}
	diagnostic.Status, diagnostic.ErrorCode = "rejected", "GEN_E_INTERNAL"
	var failure *blueprint.Error
	if errors.As(err, &failure) && failure != nil && failure.Code != "" {
		diagnostic.ErrorCode = failure.Code
	}
	reason := []rune("候选被拒绝：" + err.Error())
	if len(reason) > searchCandidateReasonLimit {
		reason = append(reason[:searchCandidateReasonLimit-1], '…')
	}
	diagnostic.Reason = string(reason)
	return diagnostic
}
