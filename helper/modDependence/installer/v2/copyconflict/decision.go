package copyconflict

// 文件职责：匹配和合并 V2 安装冲突中的用户决策。
// 使用：copyfiles.CopyPlannedFiles 决定覆盖/跳过/失败动作，继续安装保存用户决策时调用 AppendOrReplaceDecision。
// 边界：只处理内存 DTO，不校验 action 语义、不修改文件、不写数据库。

import (
	"strings"

	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
)

// ResolveDecision 从用户已提交的冲突决策中匹配当前文件动作。
// 职责：优先按目标路径匹配，其次按组件 key 批量匹配。
// 使用：copyfiles.CopyPlannedFiles 在发现冲突后决定覆盖、跳过、失败或暂停等待用户选择。
// 边界：未匹配时返回空字符串，由上层转换为可恢复冲突错误。
func ResolveDecision(options contract.InstallRemotePackageOptions, file installplan.File) contract.InstallConflictAction {
	for _, decision := range options.Decisions {
		if installpath.Equal(decision.TargetPath, file.TargetPath) {
			return decision.Action
		}
	}
	for _, decision := range options.Decisions {
		if strings.TrimSpace(file.ComponentKey) != "" && decision.ComponentKey == file.ComponentKey {
			return decision.Action
		}
	}
	return ""
}

// AppendOrReplaceDecision 追加或替换用户冲突决策。
// 职责：按目标路径或组件 key 保证 manifest 中决策唯一。
// 使用：ContinueRemoteModPackageInstall 保存用户选择并继续安装时调用。
// 边界：只处理内存切片，不校验 action 是否有效。
func AppendOrReplaceDecision(decisions []contract.InstallConflictDecisionDTO, nextDecision contract.InstallConflictDecisionDTO) []contract.InstallConflictDecisionDTO {
	next := make([]contract.InstallConflictDecisionDTO, 0, len(decisions)+1)
	replaced := false
	for _, decision := range decisions {
		if sameDecisionScope(decision, nextDecision) {
			next = append(next, nextDecision)
			replaced = true
			continue
		}
		next = append(next, decision)
	}
	if !replaced {
		next = append(next, nextDecision)
	}
	return next
}

// sameDecisionScope 判断两个冲突决策是否命中同一作用域。
// 职责：复用路径归一化规则，支持按组件 key 批量覆盖决策。
// 使用：AppendOrReplaceDecision 合并决策列表时调用。
// 边界：右侧组件 key 为空时只允许路径匹配。
func sameDecisionScope(left contract.InstallConflictDecisionDTO, right contract.InstallConflictDecisionDTO) bool {
	if installpath.Equal(left.TargetPath, right.TargetPath) {
		return true
	}
	if strings.TrimSpace(right.ComponentKey) != "" && left.ComponentKey == right.ComponentKey {
		return true
	}
	return false
}
