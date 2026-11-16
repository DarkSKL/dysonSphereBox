package build

// 文件职责：在原生单元候选纬度处提前验证普通带段与建筑附加通道。
// 使用：球面装箱的纬向搜索；边界：只核对同单元已连接的真实带，不推测未来跨单元线路，失败继续搜索其它合法位置。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
)

// validateNativeUnitBelts 职责：按当前候选纬度复用普通带交叉及附加通道规则；使用：单元装箱的动态预检；边界：只检查已存在连接，失败交给搜索继续换位。
func (b *builder) validateNativeUnitBelts(ctx context.Context, unit denseSpatialUnit) error {
	indices := make(map[*placed]int32)
	entities := make([]*placed, 0, len(unit.entities))
	for _, entity := range unit.entities {
		if _, exists := indices[entity]; !exists {
			indices[entity] = int32(len(entities))
			entities = append(entities, entity)
		}
	}
	records := make([]blueprint.Building, len(entities))
	positions := make([]vector, len(entities))
	for i, entity := range entities {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := entity.record
		r.Index = int32(i)
		r.AreaIndex = 0
		r.LocalOffset = blueprint.Point{X: float32(entity.a.x), Y: float32(entity.a.y), Z: float32(entity.a.z)}
		r.InputObjIdx, r.OutputObjIdx = -1, -1
		if previous, ok := indices[entity.input]; ok {
			r.InputObjIdx = previous
		}
		if next, ok := indices[entity.output]; ok {
			r.OutputObjIdx = next
		}
		records[i] = r
		world := b.polarFrame.world(entity.a, 0)
		positions[i] = vector{world[0], world[1], world[2]}
	}
	encoding := &surfaceEncoding{}
	if err := encoding.validateBeltCrossingsAtWorld(ctx, records, positions); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	encoding.positionSnapshot = &surfacePositionSnapshot{firstRecord: &records[0], count: len(records), first: positions}
	return encoding.validateBeltBuildingOccupancy(ctx, records)
}
