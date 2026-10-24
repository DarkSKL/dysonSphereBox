package gameport

// 文件职责：维护游戏端口自动识别的 provider 注册表，并向服务层返回所有游戏的探测结果。
// 使用：MultiplayerService.GamePortPresets 在创建房间弹窗打开时调用 DetectAll。
// 边界：只做注册与遍历；具体游戏探测在 dsp 等 provider 子包；不启动游戏、不写配置、不做前端投影。

import (
	"DysonSphereBox/helper/modDependence/gameport/contract"
	"DysonSphereBox/helper/modDependence/gameport/dsp"
)

// providers 是当前支持的游戏端口识别 provider 列表。
// 职责：集中声明可被创建房间表单自动识别的游戏；使用：DetectAll 遍历。
// 边界：新增游戏时实现 contract.Provider 并在此追加；不在此处写探测逻辑。
var providers = []contract.Provider{
	dsp.New(),
}

// DetectAll 依次执行所有已注册 provider 的只读探测。
// 职责：即使某个游戏不可用也返回完整列表，保证前端始终能渲染禁用项。
// 使用：MultiplayerService.GamePortPresets。
// 边界：按注册顺序返回；provider 的失败由各自转换为不可用 Preset，不在此聚合错误。
func DetectAll() []contract.Preset {
	presets := make([]contract.Preset, 0, len(providers))
	for _, provider := range providers {
		presets = append(presets, provider.Detect())
	}

	return presets
}
