package transport

// 职责：按原版船只飞行阶段估算船队占用；使用：星际产业规划；边界：普通 200 米星球，以快照中心距离近似两塔航程。

import (
	"fmt"
	"math"
)

// VesselProfile 职责：保存固定物流倍率折算的船运参数；使用：按线路估算；边界：速度为游戏米/秒。
type VesselProfile struct {
	Cargo             float64
	CruiseSpeed       float64
	WarpSpeed         float64
	WarpDistance      float64
	TurnaroundSeconds float64
}

// VesselTrip 职责：估算往返船时和曲速器；使用：将不同货物折算为可加总的船队占用；边界：采用普通星球与快照中心距离，周转等待另加。
func (profile VesselProfile) VesselTrip(distance float64) (seconds float64, warped bool, err error) {
	return profile.vesselTrip(distance, distance)
}

// VesselDispatch 按原版 DetermineDispatch 使用中心距加两端半径核算一次派遣扣电和曲速门槛。飞行时长采用中心距模型。
func (profile VesselProfile) VesselDispatch(distance, originRadius, destinationRadius float64) (seconds float64, warped bool, energyMJ float64, err error) {
	trip := distance + originRadius + destinationRadius
	if originRadius <= 0 || destinationRadius <= 0 || math.IsNaN(trip) || math.IsInf(trip, 0) {
		return 0, false, 0, fmt.Errorf("需要有效的物流起讫星球半径与航程")
	}
	seconds, warped, err = profile.vesselTrip(distance, trip)
	if err != nil {
		return 0, false, 0, err
	}
	sail := float32(profile.CruiseSpeed)
	// CalcTripEnergyCost 对完整派遣扣一次整数焦耳，曲速附加费使用严格大于。
	velocity := min(trip*.03+100, float64(sail), 3000)
	propulsion := velocity * 200000
	if profile.WarpDistance > 0 && float32(profile.WarpSpeed) > sail+1 && trip > profile.WarpDistance {
		propulsion += 1e8
	}
	joules := 6e6 + trip*30 + propulsion
	if math.IsInf(joules, 0) || joules >= float64(math.MaxInt64) {
		return 0, false, 0, fmt.Errorf("物流船每次派遣用电超出计算范围")
	}
	return seconds, warped, math.Trunc(joules) / 1e6, nil
}

func (profile VesselProfile) vesselTrip(distance, dispatchDistance float64) (seconds float64, warped bool, err error) {
	for _, value := range []float64{distance, profile.Cargo, profile.CruiseSpeed, profile.WarpSpeed, profile.WarpDistance, profile.TurnaroundSeconds} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false, fmt.Errorf("船运参数必须为有限非负数")
		}
	}
	if profile.Cargo == 0 || profile.CruiseSpeed == 0 || distance == 0 {
		return 0, false, fmt.Errorf("缺少有效航程、巡航速度或货舱容量")
	}
	canWarp := profile.WarpDistance > 0 && dispatchDistance >= profile.WarpDistance && float32(profile.WarpSpeed) > float32(profile.CruiseSpeed)+1
	outbound, outwardWarp := vesselSingleTripTicks(distance, float32(profile.CruiseSpeed), float32(profile.WarpSpeed), canWarp, true)
	inbound, inwardWarp := vesselSingleTripTicks(distance, float32(profile.CruiseSpeed), float32(profile.WarpSpeed), canWarp, false)
	seconds = (outbound+inbound)/60 + profile.TurnaroundSeconds
	warped = outwardWarp || inwardWarp
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, false, fmt.Errorf("航时超出计算范围")
	}
	return seconds, warped, nil
}

// vesselSingleTripTicks 职责：保留原版 CalcRemoteSingleTripTime 的起降、加减速和曲速阶段；使用：分别计算去回程；边界：原版普通物流塔的星球半径为 200，未模拟公转、绕障和排队。
func vesselSingleTripTicks(distance float64, sail, warp float32, canWarp, outbound bool) (ticks float64, warped bool) {
	// 原函数的局部量为 Single；保持单精度运算与逐段整帧截断，便于直接用原 DLL 的结果对照。
	ratio := sail / 600
	scale := float32(math.Pow(float64(ratio), float64(float32(0.4))))
	flightScale := scale
	if flightScale > 1 {
		flightScale = float32(math.Log(float64(flightScale))) + 1
	}
	ratio = min(ratio, 500)
	dockRate := scale*0.006 + 0.00001
	launchAcceleration := sail * 0.03
	acceleration := sail * 0.12 * flightScale
	radiusFactor := float32(0.5)
	if outbound {
		radiusFactor = 2
	}
	launchSpeed := float32(math.Sqrt(float64(radiusFactor * 200 * launchAcceleration)))
	accelerationDistance := (sail + launchSpeed) * (sail - launchSpeed) / acceleration * 0.5
	braking := float32(0.382) * flightScale
	dockSpeed := float32(0.15)*ratio + 6*scale
	brakingDistance := (sail - dockSpeed) / braking
	brakingOffset := math.Log(float64(braking*6*flightScale/dockSpeed + 1))
	brakingTicks := math.Trunc((math.Log(float64(braking*brakingDistance/dockSpeed+1))-brakingOffset)/float64(braking)*60 + 1)
	ticks = 60 + math.Trunc(float64(1/dockRate+1)) + math.Trunc(float64(1.5/dockRate+1))
	arrivalDistance := 5000 + 0.25*float64(sail)
	departureDistance := max(accelerationDistance, 5000)
	// 原版 UI 最小启用距离为 20000 米；更短的调用也必须给离星和进站段留出空间，不能把负的曲速距离带入幂运算。
	canWarp = canWarp && distance > float64(departureDistance)+arrivalDistance
	if !canWarp {
		if distance > float64(accelerationDistance+brakingDistance) {
			ticks += math.Trunc(float64(launchSpeed/launchAcceleration)*60 + 1)
			ticks += math.Trunc(float64((sail-launchSpeed)/acceleration)*60 + 0.5)
			ticks += math.Trunc((distance-float64(accelerationDistance)-float64(brakingDistance))/float64(sail)*60 + 0.5)
			ticks += brakingTicks
		} else if float64((launchSpeed-dockSpeed)/braking) < distance {
			ticks += math.Trunc(float64(launchSpeed/launchAcceleration)*60 + 1)
			peakSpeed := float32(math.Sqrt(float64(2*acceleration*(float32(distance)+dockSpeed/braking)+launchSpeed*launchSpeed+acceleration*acceleration/braking/braking))) - acceleration/braking
			ticks += math.Trunc(float64((peakSpeed-launchSpeed)/acceleration)*60 + 0.5)
			brakingDistance = (peakSpeed - dockSpeed) / braking
			ticks += math.Trunc((math.Log(float64(braking*brakingDistance/dockSpeed+1))-brakingOffset)/float64(braking)*60 + 1)
		} else {
			peakSpeed := float32(float64(braking)*distance + float64(dockSpeed))
			ticks += math.Trunc(float64(peakSpeed/launchAcceleration)*60 + 1)
			brakingDistance = (peakSpeed - dockSpeed) / braking
			ticks += math.Trunc((math.Log(float64(braking*brakingDistance/dockSpeed+1))-brakingOffset)/float64(braking)*60 + 1)
		}
		return ticks, false
	}

	// 曲速上升、退出阶段的积分与帧数来自原函数；启用阈值只在外层判断，不是必须巡航的路程。
	warpSpeed := min(distance*2, float64(warp))
	transitionDistance := warpSpeed*0.18775714286 + float64(sail)*1.33333333333
	ticks += math.Trunc(float64(launchSpeed/launchAcceleration)*60 + 1)
	ticks += math.Trunc(float64((sail-launchSpeed)/acceleration)*60 + 0.5)
	if accelerationDistance < 5000 {
		ticks += math.Trunc(float64((5000-accelerationDistance)/sail) * 60)
	}
	if distance > transitionDistance+float64(departureDistance)+arrivalDistance {
		ticks += 80
		ticks += math.Trunc((distance - transitionDistance - float64(departureDistance) - arrivalDistance) / (warpSpeed + float64(sail)) * 60)
	} else {
		fraction := (distance - float64(departureDistance) - arrivalDistance) / transitionDistance
		ticks += math.Trunc(6400 / (79/math.Pow(fraction, 0.1428571492433548) + 1/fraction))
	}
	if arrivalDistance > float64(brakingDistance) {
		ticks += math.Trunc((arrivalDistance - float64(brakingDistance)) / float64(sail) * 60)
		ticks += brakingTicks
	} else {
		ticks += math.Trunc((math.Log(float64(braking)*arrivalDistance/float64(dockSpeed)+1)-brakingOffset)/float64(braking)*60 + 1)
	}
	return ticks, true
}
