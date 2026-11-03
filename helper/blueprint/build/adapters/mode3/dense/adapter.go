package dense

// 文件职责：装配行星内塔预留组合的布局阶段与实体能力。
// 使用：组合注册器读取本目录 Adapter；边界：当前显式复用模式内稳定实现，不依赖 standard 组合。

import (
	"DysonSphereBox/helper/blueprint/build/adapters/mode3/common"
	"DysonSphereBox/helper/blueprint/build/contract"
)

// Adapter 职责：为预留组合提供独立阶段装配入口；使用：注册器；边界：复用模式内能力不意味着已提供新的密度算法。
func Adapter() contract.StationAdapter {
	return contract.StationAdapter{Config: common.Configuration(), Layout: common.Layout}
}
