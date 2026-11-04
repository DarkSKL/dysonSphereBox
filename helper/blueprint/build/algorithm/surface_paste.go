package build

// 文件职责：定义实际蓝图粘贴使用的纬度到经度格距映射。
// 使用：最终世界位置、重定位和独立反例；边界：AreaSegments是来源区域比例元数据，不能锁定玩家实际落点的物理格距。

import "math"

// surfacePasteLongitudeStep 职责：以实际区域起点纬度选择粘贴格距；使用：默认方向的主区和子区；边界：固定原版200分段，保留BlueprintUtils的单精度半值远离零取整。
func surfacePasteLongitudeStep(latitude float32) float32 {
	latitudeStep := float32(2*float32(math.Pi)) / 1000
	grid := latitude / latitudeStep
	if grid > 0 {
		grid += .5
	} else {
		grid -= .5
	}
	segments := longitudeSegmentCount(int(grid), 200)
	return float32(2*float32(math.Pi)) / float32(segments*5)
}
