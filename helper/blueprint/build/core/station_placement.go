package build

// 文件职责：允许各共享物流塔独立靠近供需端口，同时保持真实塔间距。
// 使用：全场装箱的独立塔候选；边界：不把塔间禁放其它塔的圆域当作机器禁建区，最终球面间距仍逐塔核验。

import "math"

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
			minimum := math.Sqrt(float64(surfaceStationDistanceLimit(tower.record.ItemID, other.record.ItemID))) + planarGridLength
			if dx*dx+dy*dy < minimum*minimum {
				return false
			}
		}
	}
	return true
}
