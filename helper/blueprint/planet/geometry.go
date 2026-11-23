package planet

// 文件职责：投影局部蓝图并检查绝对落点；使用：生成器装配和存量避让；边界：默认北向零旋转。

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/geometry"
)

// ProjectDocument 职责：将现有生产蓝图投影至目标粘贴原点；使用：局部产线拼装；边界：原点纬带须与原生 Area 格数相符。
func ProjectDocument(ctx context.Context, document blueprint.Document, anchor Position, module string) ([]Object, error) {
	if !validPosition(anchor) || len(document.Areas) == 0 || document.PrimaryAreaIndex < 0 || int(document.PrimaryAreaIndex) >= len(document.Areas) || document.Reform != nil {
		return nil, fmt.Errorf("局部模块须具有有效主 Area、粘贴原点及独立地基策略")
	}
	latitudeStep, err := geometry.LatitudeStep32(geometry.Vanilla200)
	if err != nil {
		return nil, err
	}
	origins := make([]Position, len(document.Areas))
	steps := make([]float64, len(origins))
	states := make([]uint8, len(origins))
	primary := int(document.PrimaryAreaIndex)
	var resolve func(index int) error
	resolve = func(index int) error {
		if index < 0 || index >= len(origins) || states[index] == 1 {
			return fmt.Errorf("局部模块的 Area 父链无效")
		}
		if states[index] == 2 {
			return nil
		}
		area := document.Areas[index]
		if int(area.Index) != index || area.AreaSegments <= 0 {
			return fmt.Errorf("局部模块的 Area 编号或经度分段无效")
		}
		states[index] = 1
		origin := anchor
		parent := int(area.ParentIndex)
		if index != primary {
			if err := resolve(parent); err != nil {
				return err
			}
			origin.Latitude = float64(float32(origins[parent].Latitude) + float32(area.AnchorLocalOffsetY)*float32(latitudeStep))
		}
		step, err := geometry.PasteLongitudeStep32(geometry.Radians(origin.Latitude), geometry.Vanilla200)
		if err != nil {
			return err
		}
		if math.Abs(float64(step)*float64(area.AreaSegments)*5-2*math.Pi) > 1e-5 {
			return fmt.Errorf("粘贴原点与局部模块的 Area %d 纬带不符", index)
		}
		if index != primary {
			origin.Longitude = float64(float32(origins[parent].Longitude) + float32(area.TropicAnchor)*float32(steps[parent]) - float32(area.AnchorLocalOffsetX)*float32(step))
		}
		origins[index], steps[index], states[index] = origin, float64(step), 2
		return nil
	}
	for i := range origins {
		if err := resolve(i); err != nil {
			return nil, err
		}
	}
	objects := make([]Object, 0, len(document.Buildings))
	for _, building := range document.Buildings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := int(building.AreaIndex)
		if n < 0 || n >= len(origins) {
			return nil, fmt.Errorf("建筑 %d 的原始 Area 无效", building.Index)
		}
		project := func(p blueprint.Point) Position {
			return Position{
				Longitude: float64(float32(origins[n].Longitude) + p.X*float32(steps[n])),
				Latitude:  float64(float32(origins[n].Latitude) + p.Y*float32(latitudeStep)),
				Height:    float64(p.Z),
			}
		}
		// 远程槽位和分组会在落位后改写，每次投影须独立持有参数。
		building.Parameters = slices.Clone(building.Parameters)
		objects = append(objects, Object{Building: building, First: project(building.LocalOffset), Second: project(building.LocalOffset2), Module: module})
	}
	return objects, nil
}

// validateNative 职责：复用局部生成器的游戏规则；使用：最终 Area 与编号冻结后；边界：保留已知规则的数值精度。
func validateNative(ctx context.Context, document blueprint.Document, latitude int) error {
	return buildcore.ValidatePlanetGeometry(ctx, document, latitude)
}

// world 职责：映射存档球面端点；使用：建筑避让；边界：原版 200 半径及建筑层。
func world(p Position) (geometry.WorldPoint, error) {
	return geometry.SurfaceToWorld32(geometry.SurfacePosition{Frame: geometry.PlanetWorldFrame(), Longitude: geometry.Radians(p.Longitude), Latitude: geometry.Radians(p.Latitude), Height: geometry.HeightLayers(p.Height)})
}
