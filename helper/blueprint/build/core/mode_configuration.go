package build

// 文件职责：从规范请求和唯一原生网格表导出模式选择事实。
// 使用：配置层读取半球、跨带许可及经向容量边界；边界：不选择策略，不虚构请求不存在的绝对经度，不修改物理阈值。

import "fmt"

// ModeGeographyFacts 职责：运输只读地理事实；使用：登记的YAML属性；边界：纬度单位明确，经度仅描述真实环容量。
type ModeGeographyFacts struct {
	// Class 职责：标识平面或真实纬带类别；使用：地理分支；边界：自动选区保留automatic。
	Class string `json:"class"`
	// Hemisphere 职责：标识南北及赤道；使用：搜索顺序；边界：平面和自动为unspecified。
	Hemisphere string `json:"hemisphere"`
	// LatitudeGrid 职责：保存规范锚点细格索引；使用：边界解释；边界：不是度数。
	LatitudeGrid int `json:"latitudeGrid"`
	// LatitudeDegrees 职责：提供每细格0.36度的换算；使用：解释事实；边界：不作为新物理阈值。
	LatitudeDegrees float64 `json:"latitudeDegrees"`
	// LongitudeSegments 职责：保存锚点纬圈的实际经度段数；使用：环容量分支；边界：来自共同网格表。
	LongitudeSegments int `json:"longitudeSegments"`
	// LongitudeBoundary 职责：区分赤道完整环和收缩环；使用：经向搜索顺序；边界：没有绝对经度位置含义。
	LongitudeBoundary string `json:"longitudeBoundary"`
	// CrossBands 职责：描述真实跨带许可；使用：YAML优先级；边界：不授予额外区域权限。
	CrossBands bool `json:"crossBands"`
}

// ModeGeography 职责：读取真实纬度和经向环容量；使用：模式配置属性提供器；边界：空输入或未知网格返回错误，绝不计算候选。
func (input *PreparedInput) ModeGeography() (ModeGeographyFacts, error) {
	if input == nil {
		return ModeGeographyFacts{}, fmt.Errorf("模式地理事实需要规范输入")
	}
	facts := ModeGeographyFacts{Class: input.SurfaceClass(), Hemisphere: "unspecified", LongitudeBoundary: "unspecified"}
	layout := input.request.SurfaceLayout
	if layout == nil || layout.Mode == "planar" {
		return facts, nil
	}
	if layout.GridSegment != defaultSurfaceGridSegment {
		return facts, fmt.Errorf("模式地理事实只接受已核验网格")
	}
	facts.CrossBands = layout.ExpandAdjacent || layout.Mode == "autoTropic"
	if layout.Mode == "autoTropic" {
		return facts, nil
	}
	facts.LatitudeGrid = layout.LatitudeGridIndex
	facts.LatitudeDegrees = float64(facts.LatitudeGrid) * 90 / float64(layout.GridSegment*5/4)
	facts.Hemisphere = "equator"
	if facts.LatitudeGrid < 0 {
		facts.Hemisphere = "south"
	} else if facts.LatitudeGrid > 0 {
		facts.Hemisphere = "north"
	}
	facts.LongitudeSegments = longitudeSegmentCount(facts.LatitudeGrid, layout.GridSegment)
	facts.LongitudeBoundary = "equatorial-ring"
	if facts.LongitudeSegments != longitudeSegmentCount(0, layout.GridSegment) {
		facts.LongitudeBoundary = "reduced-ring"
	}
	// 未指定五档键时按同一纬带表寻找实际档位，不按猜测的角度阈值分类。
	if layout.LatitudeZone == "" {
		bands := surfaceLatitudeBands(layout.GridSegment)
		center := 0
		for i, band := range bands {
			if band.low <= 0 && band.high >= 0 {
				center = i
				break
			}
		}
		for i, band := range bands {
			if band.low <= facts.LatitudeGrid && facts.LatitudeGrid <= band.high {
				tier := min(5, absInt(i-center)+1)
				facts.Class = "equatorial"
				if tier >= 4 {
					facts.Class = "polar"
				} else if tier >= 2 {
					facts.Class = "temperate"
				}
				break
			}
		}
	}
	return facts, nil
}
