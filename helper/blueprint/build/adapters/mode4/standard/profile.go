package standard

// 文件职责：声明星际物流塔标准密度适配器组合。
// 使用：统一注册器同时读取组合身份与本目录装配的阶段。
// 边界：实体装配尚由旧引擎执行，布局阶段不再由注册器决定。

import "DysonSphereBox/helper/blueprint/build/adapterprofile"

// Definition 返回本目录所代表的生成组合。
// 职责：声明 mode 4 与 standard 的稳定组合键；使用：生成适配器注册；边界：不负责校验或运行请求。
func Definition() adapterprofile.Definition {
	adapter := Adapter()
	return adapterprofile.Definition{Mode: 4, Density: "standard", Station: &adapter}
}
