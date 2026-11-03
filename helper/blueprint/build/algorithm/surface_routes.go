package build

// 文件职责：把最终 Area 局部坐标映射到球面并校验已知的带接线距离限制。
// 使用：球面结果编码前，对最终 float32 建筑记录运行。
// 边界：目前针对原版半径 200、未旋转的建议锚点；不模拟科技、地形或分拣器端口修正。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// surfaceAreaOrigin 职责：保存原生区域的经纬弧度；使用：最终坐标映射；边界：使用游戏 float32 运算。
type surfaceAreaOrigin struct{ longitude, latitude float32 }

// surfaceWorldPositions 职责：复现建议锚点下的建筑中心坐标；使用：带距离与后续碰撞窄相位；边界：未知主网格拒绝，不将格网单位直接当作米。
func (encoding *surfaceEncoding) surfaceWorldPositions(records []blueprint.Building) ([]vector, error) {
	if encoding.planar {
		positions := make([]vector, len(records))
		for i, record := range records {
			if record.AreaIndex != 0 {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "切平面规则适配不支持未经球面映射的多 Area"}
			}
			positions[i] = vector{float64(record.LocalOffset.X) * planarGridLength, float64(record.LocalOffset.Z)*1.3333333 + .2, float64(record.LocalOffset.Y) * planarGridLength}
		}
		return positions, nil
	}
	if encoding.plan.GridSegment != 200 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_UNVERIFIED", Message: "当前游戏坐标验证仅覆盖原版 200 分段行星"}
	}
	latitudeStep := float32(2*float32(math.Pi)) / 1000
	origins := make([]surfaceAreaOrigin, len(encoding.areas))
	resolved := make([]bool, len(encoding.areas))
	var resolve func(int) error
	resolve = func(index int) error {
		if index < 0 || index >= len(origins) {
			return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "区域父索引无效"}
		}
		if resolved[index] {
			return nil
		}
		area := encoding.areas[index]
		if index == encoding.plan.PrimaryAreaIndex {
			origins[index].latitude = float32(encoding.plan.AnchorLatitudeGrid) * latitudeStep
		} else {
			parent := int(area.ParentIndex)
			if parent == index || parent < 0 || parent >= len(origins) || absInt(parent-encoding.plan.PrimaryAreaIndex) >= absInt(index-encoding.plan.PrimaryAreaIndex) {
				return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "区域父链未朝主区域收敛"}
			}
			if err := resolve(parent); err != nil {
				return err
			}
			parentStep := float32(2*float32(math.Pi)) / float32(int(encoding.areas[parent].AreaSegments)*5)
			childStep := float32(2*float32(math.Pi)) / float32(int(area.AreaSegments)*5)
			origins[index] = surfaceAreaOrigin{longitude: origins[parent].longitude + float32(area.TropicAnchor)*parentStep - float32(area.AnchorLocalOffsetX)*childStep, latitude: origins[parent].latitude + float32(area.AnchorLocalOffsetY)*latitudeStep}
		}
		resolved[index] = true
		return nil
	}
	for i := range origins {
		if err := resolve(i); err != nil {
			return nil, err
		}
	}
	positions := make([]vector, len(records))
	for i, record := range records {
		area := int(record.AreaIndex)
		if area < 0 || area >= len(origins) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "建筑区域索引无效"}
		}
		longitudeStep := float32(2*float32(math.Pi)) / float32(int(encoding.areas[area].AreaSegments)*5)
		longitude := origins[area].longitude + record.LocalOffset.X*longitudeStep
		latitude := origins[area].latitude + record.LocalOffset.Y*latitudeStep
		radius := record.LocalOffset.Z*float32(1.3333333) + float32(0.2) + 200
		cosLatitude := float32(math.Cos(float64(latitude)))
		x := cosLatitude * float32(math.Sin(float64(longitude))) * radius
		y := float32(math.Sin(float64(latitude))) * radius
		z := cosLatitude * -float32(math.Cos(float64(longitude))) * radius
		positions[i] = vector{float64(x), float64(y), float64(z)}
	}
	return positions, nil
}

// validateBeltDistances 职责：检查游戏 TooFar 对应的带连接距离；使用：球面记录出口；边界：不把距离通过当作路线碰撞或科技高度通过。
func (encoding *surfaceEncoding) validateBeltDistances(records []blueprint.Building) error {
	return encoding.validateBeltDistancesContext(context.Background(), records)
}

// validateBeltDistancesContext 职责：检查球面带接线距离并响应生成取消；使用：候选锚点预检；边界：只验证已知距离，不能替代路线或端口碰撞规则。
func (encoding *surfaceEncoding) validateBeltDistancesContext(ctx context.Context, records []blueprint.Building) error {
	if ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带距离检查已取消或超时"}
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带距离检查已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		for _, connection := range []struct {
			index  int32
			output bool
		}{{record.OutputObjIdx, true}, {record.InputObjIdx, false}} {
			if connection.index < 0 {
				continue
			}
			j, exists := byIndex[connection.index]
			if !exists {
				return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "传送带连接指向蓝图外部"}
			}
			otherBelt := surfaceIsBelt(records[j].ItemID)
			if otherBelt && !connection.output {
				continue
			}
			// 游戏构建 25955035 的 CheckBuildConditions 直接用 sqrMagnitude
			// 与 5.3/16/28 比较；这些已经是平方阈值，不可再次平方放宽。
			limit := float32(16)
			if otherBelt {
				limit = 5.3
			} else if records[j].ItemID == 2307 {
				limit = 28
			}
			x := float32(positions[i].x) - float32(positions[j].x)
			y := float32(positions[i].y) - float32(positions[j].y)
			z := float32(positions[i].z) - float32(positions[j].z)
			if x*x+y*y+z*z > limit {
				return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("传送带 %d(物品%d,局部%.2f,%.2f,%.2f) 与建筑 %d(物品%d,局部%.2f,%.2f,%.2f) 在纬带 %d 的距离 %.3f 超出 %.3f 接线限制", record.Index, record.ItemID, record.LocalOffset.X, record.LocalOffset.Y, record.LocalOffset.Z, records[j].Index, records[j].ItemID, records[j].LocalOffset.X, records[j].LocalOffset.Y, records[j].LocalOffset.Z, record.AreaIndex, math.Sqrt(float64(x*x+y*y+z*z)), math.Sqrt(float64(limit)))}
			}
		}
	}
	return nil
}

// surfaceIsBelt 职责：识别已知原版传送带原型；使用：距离规则选择；边界：未适配 Mod 原型不自行归类。
func surfaceIsBelt(item int16) bool { return item >= 2001 && item <= 2003 }
