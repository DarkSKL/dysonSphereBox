package multiplayer

// 文件职责：定义创建房间表单可用的游戏端口自动识别结果，并暴露只读查询方法。
// 使用：前端打开创建房间弹窗时通过 Wails 绑定调用 GamePortPresets，把结果渲染为端口类型选项。
// 边界：只投影 helper 的探测结果；不启动/停止游戏、不写配置、不参与创建房间的端口校验。

import "DysonSphereBox/helper/modDependence/gameport"

// GamePortPreset 是前端可渲染的一个游戏自动识别选项。
// 职责：同时携带展示信息、禁用原因和可回填端口。
// 使用：创建房间弹窗的端口类型下拉项；不可用时也返回，供前端展示禁用态。
// 边界：Available 为 false 时 Ports 为空，Reason 为简短中文原因。
type GamePortPreset struct {
	// ID 职责：稳定的游戏标识；使用：前端选项值与已选状态匹配；边界：同一响应内唯一，小写。
	ID string `json:"id"`
	// Name 职责：面向用户显示的游戏名；使用：端口类型下拉项；边界：由对应游戏 provider 维护。
	Name string `json:"name"`
	// Available 职责：当前是否可选；使用：控制下拉项是否禁用；边界：需要游戏进程运行且主机端口已监听。
	Available bool `json:"available"`
	// Reason 职责：不可用时的简短原因；使用：拼接在禁用选项文本中；边界：可用时为空。
	Reason string `json:"reason"`
	// Ports 职责：识别到的端口声明；使用：选中后回填协议与端口；边界：可用时至少一项，不可用为空。
	Ports []GamePortDeclaration `json:"ports"`
}

// GamePortDeclaration 是一个可回填的端口声明。
// 职责：描述协议与端口，不包含连续范围和名称。
// 使用：GamePortPreset.Ports；由前端转换为创建房间端口行。
// 边界：Proto 只能是 tcp、udp 或 both；Port 为 1–65535。
type GamePortDeclaration struct {
	// Proto 职责：端口协议；使用：回填创建房间端口行；边界：tcp、udp 或 both，小写。
	Proto string `json:"proto"`
	// Port 职责：游戏实际绑定的端口；使用：回填创建房间端口输入；边界：1–65535。
	Port int `json:"port"`
}

// GamePortPresets 返回所有已注册游戏的端口自动识别结果。
// 职责：把 gameport 的探测结果投影为前端可渲染的 DTO，不可用游戏也保留禁用项。
// 使用：前端打开创建房间弹窗时调用；只读，不要求控制链路在线。
// 边界：探测失败由各 provider 转换为不可用项；本方法不返回错误，避免阻断创建房间弹窗。
func (service *MultiplayerService) GamePortPresets() []GamePortPreset {
	presets := gameport.DetectAll()
	result := make([]GamePortPreset, 0, len(presets))
	for _, preset := range presets {
		item := GamePortPreset{
			ID:        preset.ID,
			Name:      preset.Name,
			Available: preset.Available,
			Reason:    preset.Reason,
			Ports:     make([]GamePortDeclaration, 0, len(preset.Ports)),
		}
		for _, declaration := range preset.Ports {
			item.Ports = append(item.Ports, GamePortDeclaration{Proto: declaration.Proto, Port: declaration.Port})
		}
		result = append(result, item)
	}

	return result
}
