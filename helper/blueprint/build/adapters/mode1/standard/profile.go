package standard

// 文件职责：声明直连带标准密度适配器组合。
// 使用：统一适配器注册器将组合身份绑定到直连带阶段实现。
// 边界：不实现生成算法；算法能力由共享 build pipeline 提供。

import "DysonSphereBox/helper/blueprint/build/adapterprofile"

// Definition 返回本目录所代表的生成组合。
// 职责：声明 mode 1 与 standard 的稳定组合键；使用：生成适配器注册；边界：不负责校验或运行请求。
func Definition() adapterprofile.Definition {
	return adapterprofile.Definition{Mode: 1, Density: "standard"}
}
