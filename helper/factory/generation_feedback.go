package factory

// 文件职责：把生成器稳定错误码转换为简短原因、操作建议和原始诊断。
// 使用：服务桥接状态转换；边界：不改变引擎成败、不推断全局无解，未知错误保留已有文案。

import contract "DysonSphereBox/helper/factory/contract"

// generationFailureStatus 职责：分开生成失败的用户说明与技术证据；使用：Status处理蓝图引擎错误；边界：完整包装错误仅放在可展开详情，建议不承诺一定可行。
func generationFailureStatus(code, message string, err error) contract.Status {
	status := contract.Status{Code: code, Message: message}
	switch code {
	case "GEN_E_TIMEOUT":
		status.Message = "本次蓝图未能在生成时限内完成。"
		status.Suggestion = "可尝试更换运输模式，或先降低目标产量验证配置；同一配置反复超时时，请保留技术详情反馈。"
	case "GEN_E_CANCELED":
		status.Message = "本次蓝图生成已取消。"
	case "GEN_E_DENSITY_LAYOUT", "GEN_E_DENSITY_STREET_SEAM":
		status.Message = "暂未找到能放置并接通整条产线的超密度布局。"
		status.Suggestion = "可尝试更宽的建造地区或普通模式；反馈时请附上目标产量、运输模式与设备等级。"
	case "GEN_E_SURFACE_CAPACITY":
		status.Message = "暂未找到符合所选建造地区范围的完整布局。"
		status.Suggestion = "可选择更宽的建造地区，或降低目标产量后重新生成。"
	case "GEN_E_ROUTE":
		status.Message = "当前布局未能完成全部物料接线。"
		status.Suggestion = "可尝试其他运输模式或更高等级的传送带、分拣器；搜索预算耗尽不代表一定无法接线。"
	case "GEN_E_BELT_CAPACITY", "GEN_E_INSERTER_CAPACITY", "GEN_E_OVERFLOW":
		status.Message = "自动升档后，仍未找到满足输送能力的布局。"
		status.Suggestion = "可调整运输模式或拆分产线；技术详情保留了实际等级、所需流量与具体端口。"
	case "GEN_E_POWER_COVERAGE", "GEN_E_POWER_NETWORK":
		status.Message = "当前布局未能满足全部建筑的供电或电网连通要求。"
		status.Suggestion = "可尝试更宽的建造地区或调整研究站层数；若仍失败，请保留技术详情反馈。"
	case "GEN_E_COLLISION":
		status.Message = "当前布局未通过建筑或线路碰撞检查。"
		status.Suggestion = "可尝试其他建造地区或普通模式；技术详情中保留了碰撞对象及位置。"
	case "GEN_E_COLLISION_UNVERIFIED":
		status.Message = "当前布局的碰撞校验未能完成。"
		status.Suggestion = "请保留技术详情反馈；程序不会跳过这项检查交付蓝图。"
	case "GEN_E_DENSITY_UNSUPPORTED":
		status.Message = "当前设备尚未完成超密度布局适配。"
		status.Suggestion = "可暂用普通模式；反馈时请附上目标产物、产量与设备等级。"
	case "GEN_E_FLOW", "GEN_E_CONNECTIVITY":
		status.Message = "蓝图未通过完整物料连通或流量检查。"
		status.Suggestion = "请重新生成；若重复出现，请保留技术详情反馈，当前失败结果不会作为可用蓝图交付。"
	case "GEN_E_SURFACE_UNVERIFIED":
		status.Message = "当前布局未通过所选地区的球面校验。"
		status.Suggestion = "可尝试其他建造地区；请勿将未通过校验的布局当作可用蓝图。"
	}
	if status.Message != message || status.Suggestion != "" {
		status.Detail = err.Error()
	}
	return status
}
