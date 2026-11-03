package adapterprofile

// 文件职责：定义生成适配器组合的轻量注册契约。
// 使用：各模式/密度目录声明自己提供的组合，统一 build pipeline 负责挂接执行阶段。
// 边界：不依赖 build 包，不持有算法状态或生成请求。

import "DysonSphereBox/helper/blueprint/build/contract"

// Definition 描述可被适配器解析器选择的模式/密度组合。
// 职责：提供组合身份与已独立的阶段；使用：各组合包工厂及注册表；边界：只引用 contract，不依赖流水线或具体组合。
type Definition struct {
	// Mode 职责：标识既有生成模式；使用：组合注册；边界：具体合法范围由上层服务校验。
	Mode int
	// Density 职责：标识布局密度键；使用：组合注册；边界：不声明组合内部算法实现。
	Density string
	// Station 职责：携带由物流塔组合目录装配的实际阶段；使用：统一注册器绑定；边界：非物流塔组合为空，不在此实现算法。
	Station *contract.StationAdapter
}

// Key 唯一标识一个适配器组合。
// 职责：为组合注册表提供可比较键；使用：pipeline 和模式目录；边界：只包含静态模式与密度身份。
type Key struct {
	// Mode 职责：标识生成模式；使用：组合查找；边界：合法范围由入口策略决定。
	Mode int
	// Density 职责：标识布局密度配置；使用：组合查找；边界：需与已登记密度 profile 完全匹配。
	Density string
}
