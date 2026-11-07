package build

import algorithm "DysonSphereBox/helper/blueprint/build/algorithm"

// Setting 职责：公开生成设置；使用：请求配置；边界：与引擎类型保持一致。
type Setting = algorithm.Setting

// Request 职责：公开生成请求；使用：服务绑定；边界：不依赖内部构建器。
type Request = algorithm.Request

// SurfaceLayout 职责：公开球面布局配置；使用：指定落点策略；边界：由引擎校验。
type SurfaceLayout = algorithm.SurfaceLayout

// DensityLatitudeOption 职责：公开密铺建造区域选项；使用：前端简化位置选择；边界：由后端定义合法键。
type DensityLatitudeOption = algorithm.DensityLatitudeOption

// SurfaceBandPlan 职责：公开纬带规划；使用：结果展示；边界：不执行布局。
type SurfaceBandPlan = algorithm.SurfaceBandPlan

// SurfacePlan 职责：公开球面规划结果；使用：落点诊断；边界：不代替游戏内校验。
type SurfacePlan = algorithm.SurfacePlan

// Zone 职责：公开产区配置；使用：请求分区；边界：不保存运行时状态。
type Zone = algorithm.Zone

// ProductionBinding 职责：公开生产与建筑关联；使用：结果查询；边界：不参与生成调度。
type ProductionBinding = algorithm.ProductionBinding

// Result 职责：公开蓝图生成结果；使用：服务返回；边界：内部布局状态由引擎维护。
type Result = algorithm.Result
