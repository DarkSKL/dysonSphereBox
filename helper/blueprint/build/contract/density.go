package contract

// 文件职责：统一密度请求枚举和规则版本。
// 使用：服务、工作区与算法边界；边界：版本标识实现规则，不声称游戏实测。

import "DysonSphereBox/helper/blueprint"

// DensityProfileVersion 职责：标识当前密集布局规则；使用：结果和工作区溯源；边界：改变几何或支持范围时更新。
const DensityProfileVersion = "dense-v7-spherical-refinery"

// NormalizeDensity 职责：兼容旧请求并拒绝未知密度；使用：生成与导出入口；边界：不把未知值降级为普通布局。
func NormalizeDensity(value string) (string, error) {
	if value == "" {
		return "standard", nil
	}
	if value != "standard" && value != "dense" {
		return "", &blueprint.Error{Code: "GEN_E_DENSITY", Message: "布局密度必须为普通或超密度"}
	}
	return value, nil
}
