package build

// 文件职责：允许各共享物流塔独立靠近供需端口，同时保持真实塔间距。
// 使用：全场装箱的独立塔候选；边界：不把塔间禁放其它塔的圆域当作机器禁建区，最终球面间距仍逐塔核验。

import (
	"context"
	"math"
)

// independentStationPlacementKey 职责：标记取消固定塔阵列的候选；使用：私有候选上下文；边界：保留原阵列作为已验证的比较基线。
type independentStationPlacementKey struct{}

// stationPlacementClear 职责：验证待放置单元与已落位塔的间距；使用：整数及细格装箱；边界：只比较塔，不禁止机器或带利用塔间空隙。
func stationPlacementClear(moving, fixed []*placed, delta vector) bool {
	for _, tower := range moving {
		if !surfaceIsStation(tower.record.ItemID) {
			continue
		}
		for _, other := range fixed {
			dx, dy := (tower.a.x+delta.x-other.a.x)*planarGridLength, (tower.a.y+delta.y-other.a.y)*planarGridLength
			// 额外一格容纳小幅球面投影变化；它只约束塔中心，未扩大塔的真实碰撞轮廓。
			minimum := stationPackingMinimum(tower, other)
			if dx*dx+dy*dy < minimum*minimum {
				return false
			}
		}
	}
	return true
}

// stationPackingMinimum 职责：返回含球面投影余量的塔中心最小距离；使用：局部净距与新增搜索范围；边界：单位为米，最终仍按原球面规则核查。
func stationPackingMinimum(left, right *placed) float64 {
	return math.Sqrt(float64(surfaceStationDistanceLimit(left.record.ItemID, right.record.ItemID))) + planarGridLength
}

// stationPackingLongitudeExtra 职责：为有已落位塔的独立塔单元补一段净距搜索；使用：原装箱无法落塔后的单次重试；边界：仅扩大经向试探范围，保持纬向容量和原位置检查预算。
func stationPackingLongitudeExtra(unit denseSpatialUnit, fixed []*placed) int {
	if len(fixed) == 0 {
		return 0
	}
	minimum := 0.
	for _, entity := range unit.entities {
		if surfaceIsStation(entity.record.ItemID) {
			for _, tower := range fixed {
				minimum = math.Max(minimum, stationPackingMinimum(entity, tower))
			}
		}
	}
	return int(math.Ceil(minimum / planarGridLength))
}

// preserveIndependentStationPacking 职责：保存独立多塔装箱会改写的姿态；使用：装箱失败时以原坐标和端口朝向重试；边界：原生极区另有重排，单塔无塔距冲突，两者均走原路径。
func preserveIndependentStationPacking(ctx context.Context, units []denseSpatialUnit) func() {
	independent, _ := ctx.Value(independentStationPlacementKey{}).(bool)
	polarRoot, _ := ctx.Value(densityPolarRootKey{}).(bool)
	if !independent || polarRoot {
		return nil
	}
	stationUnits := 0
	for _, unit := range units {
		for _, entity := range unit.entities {
			if surfaceIsStation(entity.record.ItemID) {
				stationUnits++
				break
			}
		}
		if stationUnits == 2 {
			break
		}
	}
	if stationUnits < 2 {
		return nil
	}
	type entityPose struct {
		entity              *placed
		a, b, cargoRelative vector
		yaw, yaw2           float32
	}
	type rowPose struct {
		row   *transportRow
		turns int
	}
	var entities []entityPose
	var rows []rowPose
	for _, unit := range units {
		for _, entity := range unit.entities {
			entities = append(entities, entityPose{entity, entity.a, entity.b, entity.densityCargo.relative, entity.record.Yaw, entity.record.Yaw2})
		}
		for _, row := range unit.transportRows() {
			rows = append(rows, rowPose{row, row.quarterTurns})
		}
	}
	return func() {
		for _, pose := range entities {
			entity := pose.entity
			entity.a, entity.b, entity.densityCargo.relative = pose.a, pose.b, pose.cargoRelative
			entity.record.Yaw, entity.record.Yaw2 = pose.yaw, pose.yaw2
		}
		for _, pose := range rows {
			pose.row.quarterTurns = pose.turns
		}
	}
}
