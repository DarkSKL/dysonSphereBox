package generator

// 文件职责：以封闭字段类型合并模式层级，保留每个生效值的来源。
// 使用：YAML按公共、模式、子模式和地理顺序调用；边界：不递归合并、不解释脚本、不修改设备事实，节点with继续由工作流原生绑定。

import (
	"fmt"

	core "DysonSphereBox/helper/blueprint/build/core"
)

// modePatch 职责：限定可覆盖的合法搜索参数；使用：四个固定配置层；边界：缺失表示继承，空切片表示显式清空，未知字段由注册类型拒绝。
type modePatch struct {
	// PolarWindowRows 职责：覆盖polarWindowRows搜索策略；使用：显式配置层；边界：不改变物理能力。
	PolarWindowRows []int `json:"polarWindowRows"`
	// RowSpans 职责：覆盖rowSpans搜索策略；使用：显式配置层；边界：不改变物理能力。
	RowSpans []float64 `json:"rowSpans"`
	// StreetGroups 职责：覆盖streetGroups搜索策略；使用：显式配置层；边界：不改变物理能力。
	StreetGroups []int `json:"streetGroups"`
	// StreetRotations 职责：覆盖streetRotations搜索策略；使用：显式配置层；边界：不改变物理能力。
	StreetRotations []int `json:"streetRotations"`
	// DenseOrders 职责：覆盖denseOrders搜索策略；使用：显式配置层；边界：不改变物理能力。
	DenseOrders []int `json:"denseOrders"`
	// BentSpans 职责：覆盖bentSpans搜索策略；使用：显式配置层；边界：不改变物理能力。
	BentSpans []float64 `json:"bentSpans"`
	// StationFamily 职责：覆盖stationFamily搜索策略；使用：显式配置层；边界：不改变物理能力。
	StationFamily *string `json:"stationFamily"`
	// BusTighten 职责：覆盖busTighten搜索策略；使用：显式配置层；边界：不改变物理能力。
	BusTighten *bool `json:"busTighten"`
}

// modeValues 职责：冻结生效搜索参数及来源；使用：枚举器只读提供器；边界：仅含类型化策略，不含候选或调度器。
type modeValues struct {
	// Patch 职责：保存完整已验证值；使用：只读提供器；边界：不含缺省指针。
	Patch modePatch
	// Sources 职责：保存逐字段覆盖层；使用：诊断追踪；边界：不是任意字段访问入口。
	Sources map[string]string
}

// initialModeValues 职责：从现有唯一核心默认工厂导出注册层；使用：公共层之前；边界：不复制一套物理或完整算法默认值。
func initialModeValues() modeValues {
	c, s := core.DefaultConnectedSearchOptions(), core.DefaultSurfaceSearchOptions()
	family, tighten := core.DefaultStationSearchOptions().Family, true
	values := modeValues{Patch: modePatch{PolarWindowRows: s.PolarWindowRows, RowSpans: c.DenseSpans, StreetGroups: c.StreetGroups, StreetRotations: c.StreetRotations, DenseOrders: c.DenseOrders, BentSpans: c.BentSpans, StationFamily: &family, BusTighten: &tighten}, Sources: map[string]string{}}
	values.Sources["polarWindowRows"] = "registered"
	values.Sources["rowSpans"] = "registered"
	values.Sources["streetGroups"] = "registered"
	values.Sources["streetRotations"] = "registered"
	values.Sources["denseOrders"] = "registered"
	values.Sources["bentSpans"] = "registered"
	values.Sources["stationFamily"] = "registered"
	values.Sources["busTighten"] = "registered"
	return values
}

// mergeModeValues 职责：执行纯类型字段整体覆盖并验证有效组合；使用：固定层工具；边界：不允许空值冒充缺省，不通过反射任意字段路径赋值。
func mergeModeValues(previous modeValues, patch modePatch, layer string) (modeValues, error) {
	values, err := cloneModeValues(previous)
	if err != nil {
		return modeValues{}, err
	}
	if patch.PolarWindowRows != nil {
		values.Patch.PolarWindowRows = patch.PolarWindowRows
		values.Sources["polarWindowRows"] = layer
	}
	if patch.RowSpans != nil {
		values.Patch.RowSpans = patch.RowSpans
		values.Sources["rowSpans"] = layer
	}
	if patch.StreetGroups != nil {
		values.Patch.StreetGroups = patch.StreetGroups
		values.Sources["streetGroups"] = layer
	}
	if patch.StreetRotations != nil {
		values.Patch.StreetRotations = patch.StreetRotations
		values.Sources["streetRotations"] = layer
	}
	if patch.DenseOrders != nil {
		values.Patch.DenseOrders = patch.DenseOrders
		values.Sources["denseOrders"] = layer
	}
	if patch.BentSpans != nil {
		values.Patch.BentSpans = patch.BentSpans
		values.Sources["bentSpans"] = layer
	}
	if patch.StationFamily != nil {
		values.Patch.StationFamily = patch.StationFamily
		values.Sources["stationFamily"] = layer
	}
	if patch.BusTighten != nil {
		values.Patch.BusTighten = patch.BusTighten
		values.Sources["busTighten"] = layer
	}
	c, s := core.DefaultConnectedSearchOptions(), core.DefaultSurfaceSearchOptions()
	c.DenseSpans, c.StreetGroups, c.StreetRotations, c.DenseOrders, c.BentSpans = values.Patch.RowSpans, values.Patch.StreetGroups, values.Patch.StreetRotations, values.Patch.DenseOrders, values.Patch.BentSpans
	s.PolarWindowRows = values.Patch.PolarWindowRows
	if err := core.ValidateConnectedSearchOptions(c); err != nil {
		return modeValues{}, err
	}
	if err := core.ValidateSurfaceSearchOptions(s); err != nil {
		return modeValues{}, err
	}
	if values.Patch.StationFamily == nil || values.Patch.BusTighten == nil {
		return modeValues{}, fmt.Errorf("配置值缺失")
	}
	if err := core.ValidateStationSearchOptions(core.StationSearchOptions{Family: *values.Patch.StationFamily}); err != nil {
		return modeValues{}, err
	}
	return cloneModeValues(values)
}

// cloneModeValues 职责：保留显式空列表语义并复制所有可变值；使用：层级合并和实例隔离；边界：omitempty的序列化不能把空列表误变为未覆盖。
func cloneModeValues(values modeValues) (modeValues, error) {
	result := modeValues{Patch: values.Patch, Sources: map[string]string{}}
	for k, v := range values.Sources {
		result.Sources[k] = v
	}
	if values.Patch.PolarWindowRows != nil {
		result.Patch.PolarWindowRows = append([]int{}, values.Patch.PolarWindowRows...)
	}
	if values.Patch.RowSpans != nil {
		result.Patch.RowSpans = append([]float64{}, values.Patch.RowSpans...)
	}
	if values.Patch.StreetGroups != nil {
		result.Patch.StreetGroups = append([]int{}, values.Patch.StreetGroups...)
	}
	if values.Patch.StreetRotations != nil {
		result.Patch.StreetRotations = append([]int{}, values.Patch.StreetRotations...)
	}
	if values.Patch.DenseOrders != nil {
		result.Patch.DenseOrders = append([]int{}, values.Patch.DenseOrders...)
	}
	if values.Patch.BentSpans != nil {
		result.Patch.BentSpans = append([]float64{}, values.Patch.BentSpans...)
	}
	if values.Patch.StationFamily != nil {
		family := *values.Patch.StationFamily
		result.Patch.StationFamily = &family
	}
	if values.Patch.BusTighten != nil {
		tighten := *values.Patch.BusTighten
		result.Patch.BusTighten = &tighten
	}
	return result, nil
}
