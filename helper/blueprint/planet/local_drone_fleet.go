package planet

// 文件职责：从原版满载无人机往返时间核对可用机位，并在不同货物上选择供方发货或需方取货。

import "math"

// droneTripMinutes 复用原版每段按 tick 取整的单程，再换算成一趟往返的游戏分钟。
func droneTripMinutes(arc, speed float64) float64 {
	oneWay := math.Trunc(arc/speed*60+1) + 2*math.Trunc(1.5/math.Sqrt(speed/8)*60+1)
	return 2 * oneWay / 3600
}

// hardLocalFleetFloor 逐个强制本地进带口取最近供方，供需两边可同时派机；仍超过总机位才判定此货实际无法运输。
func hardLocalFleetFloor(group *localCargoSlots, cargo, speed float64) (float64, int) {
	var needed float64
	var stations int
	for _, s := range group.sources {
		if s.rate > 0 {
			stations++
		}
	}
	for _, d := range group.sinks {
		if d.required > 0 && !math.IsInf(d.nearest, 1) {
			stations++
			needed += d.required / cargo * droneTripMinutes(d.nearest, speed)
		}
	}
	return needed, stations * 100
}

// mixedLocalDispatch 每种物料至少选齐一侧的全部真实塔，无论游戏怎样选配对，总有可发机的一端。单塔同时收发多货仍共享 100 个机位。
func mixedLocalDispatch(slots map[int]*localCargoSlots, fleet map[int]float64) map[int]bool {
	within := func(points []localDroneEndpoint) bool {
		for _, p := range points {
			if p.rate > 0 && math.Ceil(fleet[p.tower])+1 > 100 {
				return false
			}
		}
		return true
	}
	selected := map[int]bool{}
	for _, group := range slots {
		if !group.active {
			continue
		}
		side := group.sources
		if !within(side) {
			side = group.sinks
			if !within(side) {
				return nil
			}
		}
		for _, p := range side {
			if p.rate > 0 {
				selected[p.tower] = true
			}
		}
	}
	return selected
}
