package standard

// 文件职责：装配星际塔标准组合的实际布局阶段与实体能力。
// 使用：组合注册器读取本目录 Adapter；边界：不维护 Builder，不跨组合引用实现。

import (
	"DysonSphereBox/helper/blueprint/build/adapters/mode4/common"
	"DysonSphereBox/helper/blueprint/build/contract"
)

// Adapter 职责：显式选择本组合布局实现；使用：注册器注入统一流水线；边界：每次返回独立值，专属阶段可在本目录替换。
func Adapter() contract.StationAdapter {
	return contract.StationAdapter{Config: common.Configuration(), Layout: common.Layout}
}
