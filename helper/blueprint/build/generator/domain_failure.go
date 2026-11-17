package generator

// 文件职责：核对完整领域错误树是否允许进入工作流备用；使用：工具错误适配的唯一分类器。
// 边界：取消、未知原因或空错误树都不降级，包装层不能仅凭其中一个已知原因获得重试权限。

import (
	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
)

// classifyDomainFailure 职责：逐叶确认白名单并保留准备阶段覆盖范围；使用：domainFailure处理普通包装及errors.Join。
// 边界：只有全部叶都来自同一准备边界时才发布准备类别；首个领域码用于显示，原始完整原因链仍由调用者保留。
func classifyDomainFailure(err error, preparation bool) (string, bool, bool) {
	switch failure := err.(type) {
	case *core.ConnectedPreparationError:
		return classifyDomainFailure(failure.Cause, true)
	case *blueprint.Error:
		for _, code := range solutionFailures {
			if failure.Code == code {
				return code, preparation, true
			}
		}
	case interface{ Unwrap() []error }:
		code, allPreparation, found := "", true, false
		for _, cause := range failure.Unwrap() {
			if cause == nil {
				continue
			}
			child, prepared, known := classifyDomainFailure(cause, preparation)
			if !known {
				return "", false, false
			}
			if !found {
				code, found = child, true
			}
			allPreparation = allPreparation && prepared
		}
		return code, allPreparation, found
	case interface{ Unwrap() error }:
		return classifyDomainFailure(failure.Unwrap(), preparation)
	}
	return "", false, false
}
