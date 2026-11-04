package build

// 文件职责：公开密度边界与版本，不包含布局算法。
// 使用：factory 服务和工作区导出；边界：密度是否合法不代表请求中的全部设备可生成。

import (
	algorithm "DysonSphereBox/helper/blueprint/build/algorithm"
	"DysonSphereBox/helper/blueprint/build/contract"
)

// DensityProfileVersion 职责：公开当前密集布局规则版本；使用：结果失效与导出溯源；边界：不接受客户端覆盖。
const DensityProfileVersion = contract.DensityProfileVersion

// NormalizeDensity 职责：提供稳定的密度输入校验；使用：业务服务；边界：旧空值按普通处理。
func NormalizeDensity(value string) (string, error) { return contract.NormalizeDensity(value) }

// DensityLatitudeOptions 职责：公开建造区域元数据；使用：数据集描述；边界：选区能力不代表任意蓝图均可放入。
func DensityLatitudeOptions() []DensityLatitudeOption { return algorithm.DensityLatitudeOptions() }

// NormalizeSurfaceLayout 职责：复用引擎的建造区域校验；使用：工作区导出；边界：只校验配置，不承诺当前产量可放入。
func NormalizeSurfaceLayout(layout *SurfaceLayout) (*SurfaceLayout, error) {
	return algorithm.NormalizeSurfaceLayout(layout)
}
