package planet

// 文件职责：从蓝图记录读取原版物流塔的充电设定，供施工清单与供电核验共用。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
)

// droneStationCharging 按实装台数、最远可配对塔、各货物满载发运量和全星配送预算取可证明的最小上界。
func droneStationCharging(slots map[int]*localCargoSlots, drones map[int]int, cargo, speed, totalMW float64) map[int]float64 {
	type bounds struct{ perDrone, cargoMW float64 }
	stations := map[int]bounds{}
	landingSeconds := math.Trunc(1.5/math.Sqrt(speed/8)*60+1) / 60
	// 省去巡航最后一 tick 的取整可得到往返用时下界；该比值对航程单调，两端中较大者覆盖全部配对。
	perDrone := func(arc float64) float64 { return (.8 + .04*arc) / (2 * (arc/speed + 2*landingSeconds)) }
	for _, g := range slots {
		if !g.active {
			continue
		}
		for _, side := range [][]localDroneEndpoint{g.sources, g.sinks} {
			for _, p := range side {
				if drones[p.tower] == 0 {
					continue
				}
				b := stations[p.tower]
				b.perDrone = max(b.perDrone, perDrone(0), perDrone(p.chargeArc))
				b.cargoMW += g.rate * (.8 + .04*p.chargeArc) / cargo / 60
				stations[p.tower] = b
			}
		}
	}
	result := map[int]float64{}
	for i, b := range stations {
		result[i] = min(float64(drones[i])*b.perDrone, b.cargoMW, totalMW)
	}
	return result
}

// configureFleetCharging 用各塔分得的真实船时和本地配送上界核验充电；全星制造、待机及运输的持续功率已在供电合同中分别计过。
func configureFleetCharging(req *Request, plan industryplan.PlanetPlan, actual *industryplan.PlanetActual) error {
	ships := map[int]float64{}
	for _, station := range actual.Fleet {
		var mw float64
		for _, cargo := range station.Cargo {
			if cargo.MaxVesselMW <= 0 && plan.OutboundVesselMW > 0 {
				// 旧单星合同没有各货种航时，原定整星远航总耗电是本站可取的上界。
				mw = plan.OutboundVesselMW
				break
			}
			mw += cargo.VesselLoad * cargo.MaxVesselMW
		}
		ships[int(station.BuildingIndex)] = mw
	}
	for i := range req.Objects {
		ship, drone := ships[i], req.droneChargingMW[i]
		if ship+drone <= 0 {
			continue
		}
		o := &req.Objects[i]
		notice, err := ConfigureDispatchCharging(&o.Building, math.Remainder(o.First.Longitude, 2*math.Pi)*180/math.Pi, o.First.Latitude*180/math.Pi, ship, drone)
		if err != nil {
			return &Error{PlanetID: req.PlanetID, Code: "PLANET_STATION_CHARGE", Message: err.Error()}
		}
		if notice != "" {
			actual.Warnings = append(actual.Warnings, notice)
		}
	}
	return nil
}

// ConfigureDispatchCharging 对原版星际塔选择能覆盖满载发运预算的最低合法档位，保留足额原值与其他参数。经纬度为游戏角度。
func ConfigureDispatchCharging(b *blueprint.Building, longitude, latitude, shipMW, droneMW float64) (string, error) {
	if b.ItemID != 2104 || len(b.Parameters) != 2048 {
		return "", fmt.Errorf("承担发运的 %d 号塔缺少可核对的星际物流塔充电参数", b.Index)
	}
	// 原版普通塔每 tick 固定扣 1000 J（0.060 MW）。改变充电档位不会改变这笔持续用电。
	required := .060 + shipMW + droneMW
	where := fmt.Sprintf("%d 号塔（经度 %.2f°、纬度 %.2f°）", b.Index, longitude, latitude)
	if required > 300 {
		return "", fmt.Errorf("%s 的飞船、无人机及待机持续用电上界为 %.3f MW，原版单塔最高可设 300 MW。当前派运方案未通过充电核验，需分流到其他塔或调整星球分工", where, required)
	}
	energy, err := stationChargingEnergy(*b)
	if err != nil {
		return "", err
	}
	if required <= float64(energy)*.00006 {
		return "", nil
	}
	steps := max(10, int32(math.Ceil(required/3)))
	b.Parameters[320] = steps * 50000
	return fmt.Sprintf("%s 的充电上限已设为 %d MW，覆盖远航 %.3f MW、本地配送 %.3f MW 及待机 0.060 MW。", where, 3*steps, shipMW, droneMW), nil
}

// StationChargingMW 累计所有行星/星际物流站同时按设定充电的上限；轨道采集器与大型矿机不使用该充电参数。
func StationChargingMW(records []blueprint.Building) (float64, error) {
	var joulesPerTick int64
	for _, b := range records {
		if b.ItemID != 2103 && b.ItemID != 2104 {
			continue
		}
		energy, err := stationChargingEnergy(b)
		if err != nil {
			return 0, err
		}
		joulesPerTick += energy
	}
	return float64(joulesPerTick) * .00006, nil
}

// stationChargingEnergy 与原版 ApplyPrebuildParametersToEntity 一致，仅由正的参数 320 覆盖原型的每 tick 焦耳数。
func stationChargingEnergy(b blueprint.Building) (int64, error) {
	if len(b.Parameters) > 320 && b.Parameters[320] > 0 {
		return int64(b.Parameters[320]), nil
	}
	catalog, err := facts.Load()
	if err != nil {
		return 0, err
	}
	item, ok := catalog.Lookup(int(b.ItemID))
	if !ok || item.Power == nil {
		return 0, fmt.Errorf("无法确定物流塔 %d 的默认充电功率", b.ItemID)
	}
	return item.Power.Work, nil
}
