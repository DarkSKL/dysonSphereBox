package build

// 文件职责：在保持槽口横向精度的前提下解释球面短接头及向外过渡段。
// 使用：最终碰撞和路由共享几何；边界：只复现径向曲率和原生经纬格仿射映射，不放宽错槽或层高误差。

import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// splitterPortProbe 职责：保存需要在同一槽口坐标系验证的实体位置；使用：真实接头与相邻边共同核对；边界：short只给原生1.1米内的绑定节点。
type splitterPortProbe struct {
	point placement.Vector
	short bool
}

// splitterProjectedPoint 职责：把已知球面坐标恢复到分流器的米制局部切平面；使用：同一接头整段的一致投影；边界：mode1只消除弦高，mode2按经纬坐标恢复原生格距，不添加横向容差。
func splitterProjectedPoint(point, origin placement.Vector, mode int) placement.Vector {
	if mode == 0 {
		return point
	}
	radius := math.Sqrt(origin.Dot(origin))
	pointRadius := math.Sqrt(point.Dot(point))
	up := origin.Scale(1 / radius)
	height := pointRadius - radius
	delta := point.Sub(origin)
	if mode == 1 {
		return origin.Add(delta.Sub(up.Scale(delta.Dot(up)))).Add(up.Scale(height))
	}
	east := up.Cross(placement.Vector{0, 1, 0})
	length := math.Sqrt(east.Dot(east))
	if length < 1e-6 {
		return point
	}
	east = east.Scale(1 / length)
	north := east.Cross(up)
	latitude := math.Asin(math.Max(-1, math.Min(1, up[1])))
	pointLatitude := math.Asin(math.Max(-1, math.Min(1, point[1]/pointRadius)))
	longitude := math.Atan2(origin[0], -origin[2])
	pointLongitude := math.Atan2(point[0], -point[2])
	x := math.Remainder(pointLongitude-longitude, 2*math.Pi) * radius * math.Cos(latitude)
	y := (pointLatitude - latitude) * radius
	return origin.Add(east.Scale(x)).Add(north.Scale(y)).Add(up.Scale(height))
}

// splitterPortProbesAllowed 职责：用同一种已知映射核对接头及整条相邻带段；使用：避免远端弦高被误判成错槽；边界：所有投影均保留0.01米横向及层高要求，不能各点随意换坐标系。
func splitterPortProbesAllowed(origin, port, forward placement.Vector, probes []splitterPortProbe, spherical bool) bool {
	modes := 1
	if spherical {
		modes = 3
	}
	for mode := 0; mode < modes; mode++ {
		valid := true
		for _, probe := range probes {
			if !splitterPointOnPortRay(splitterProjectedPoint(probe.point, origin, mode), port, forward, probe.short) {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}
