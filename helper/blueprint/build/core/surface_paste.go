package build

// 文件职责：定义实际蓝图粘贴使用的纬度到经度格距映射。
// 使用：最终世界位置、重定位和独立反例；边界：AreaSegments是来源区域比例元数据，不能锁定玩家实际落点的物理格距。

import buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"

// surfacePasteLongitudeStep 职责：以实际区域起点纬度选择粘贴格距；使用：默认方向的主区和子区；边界：固定原版200分段，保留BlueprintUtils的单精度半值远离零取整。
func surfacePasteLongitudeStep(latitude float32) float32 {
	step, err := buildgeometry.PasteLongitudeStep32(buildgeometry.Radians(latitude), buildgeometry.Vanilla200)
	if err != nil {
		// 本私有标量入口只接收已核对的球面框架纬度；正式记录使用有error的共同入口。
		panic(err)
	}
	return float32(step)
}
