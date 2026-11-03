package dense

// 文件职责：声明行星内物流塔 dense 密度适配器组合。
// 使用：统一注册器同时读取组合身份与本目录装配的阶段。
// 边界：通过模式内 common 显式复用稳定阶段，不依赖另一个密度目录。

import "DysonSphereBox/helper/blueprint/build/adapterprofile"

// Definition 返回本目录所代表的生成组合。
// 职责：声明 mode 3 与 dense 的独立组合键；使用：生成适配器注册；边界：不负责校验或运行请求。
func Definition() adapterprofile.Definition {
	adapter := Adapter()
	return adapterprofile.Definition{Mode: 3, Density: "dense", Station: &adapter}
}
