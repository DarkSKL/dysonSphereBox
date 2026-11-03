package dense

// 文件职责：声明物流配送器 dense 密度适配器组合。
// 使用：统一适配器注册器将组合身份绑定到配送器阶段实现。
// 边界：当前阶段实现沿用稳定流程，后续可在本目录替换专属阶段。

import "DysonSphereBox/helper/blueprint/build/adapterprofile"

// Definition 返回本目录所代表的生成组合。
// 职责：声明 mode 2 与 dense 的独立组合键；使用：生成适配器注册；边界：不负责校验或运行请求。
func Definition() adapterprofile.Definition {
	return adapterprofile.Definition{Mode: 2, Density: "dense"}
}
