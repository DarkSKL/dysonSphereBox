package planet

// 文件职责：让三台射线接收器共用真实物流塔侧口；使用：光子及透镜发电落位；边界：每台设备保持独立带链，不生成传送带分叉。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// energyReceiverBatch 职责：装配一至三台接收器及共享输入输出塔；使用：能源模块扫描器提供绝对中心与经格步长；边界：只支持原版光子/发电模式和透镜，不负责地表及外部障碍选址。
func energyReceiverBatch(index int32, position Position, step float64, count, mode, incoming, outgoing int, plan industryplan.PlanetPlan, catalog *placement.Catalog) ([]Object, error) {
	if count < 1 || count > 3 || mode != 0 && mode != 1208 || incoming != 0 && incoming != 1209 || outgoing != 0 && (outgoing != 1208 || mode != 1208) {
		return nil, fmt.Errorf("射线接收模块的数量、模式或物品不符")
	}
	models := catalog.ModelsForItem(2208)
	if len(models) == 0 {
		return nil, fmt.Errorf("缺少射线接收器模型")
	}
	receiver, err := catalog.Geometry(2208, models[0])
	if err != nil {
		return nil, err
	}
	station, err := catalog.Geometry(2104, 50)
	if err != nil {
		return nil, err
	}
	if len(receiver.PortPoses) != 2 || len(station.PortPoses) != 12 {
		return nil, fmt.Errorf("射线接收器或星际塔缺少已核验的侧口")
	}
	latitudeStep, err := geometry.LatitudeStep32(geometry.Vanilla200)
	if err != nil {
		return nil, err
	}
	offset := func(x, y float64) Position {
		return Position{Longitude: position.Longitude + x*step, Latitude: position.Latitude + y*float64(latitudeStep), Height: position.Height}
	}
	rows := []int{0, 10, -10}
	objects := make([]Object, 0, count*64+2)
	for i := 0; i < count; i++ {
		p := offset(0, float64(rows[i]))
		objects = append(objects, Object{First: p, Second: p, Module: "射线接收", Building: blueprint.Building{
			Index: index + int32(i), ItemID: 2208, ModelIndex: models[0], Yaw: 90, Yaw2: 90,
			InputObjIdx: -1, OutputObjIdx: -1, Parameters: []int32{int32(mode)},
		}})
	}
	for _, io := range []struct {
		item, sign, devicePort int
		input                  bool
		stationPorts           [3]int
	}{{incoming, -1, 1, true, [3]int{10, 11, 9}}, {outgoing, 1, 0, false, [3]int{4, 3, 5}}} {
		if io.item == 0 {
			continue
		}
		local, remote := 1, 0
		if io.input {
			local = 2
		}
		for _, port := range plan.Ports {
			if port.ItemID == io.item && io.input && port.Kind == "import" {
				remote = 2
			} else if port.ItemID == io.item && !io.input && port.Kind == "export" {
				remote = 1
			}
		}
		towerIndex := index + int32(len(objects))
		tower := stationObject(towerIndex, offset(float64(io.sign*18), 0), io.item, local, remote, false)
		for lane := 0; lane < count; lane++ {
			port := io.stationPorts[lane]
			if io.input {
				tower.Building.Parameters[192+port*4], tower.Building.Parameters[193+port*4] = 1, 1
			} else {
				tower.Building.Parameters[192+port*4] = 2
			}
		}
		objects = append(objects, tower)
		for lane := 0; lane < count; lane++ {
			row := rows[lane]
			port := io.stationPorts[lane]
			device := objects[lane]
			grid := 200 * math.Cos(device.First.Latitude) * step
			portDistance := math.Abs(float64(receiver.PortPoses[io.devicePort].Position[2]))
			first := max(1, int(math.Round(portDistance/grid)))
			// 从接收器向塔展开；侧路只在距塔五格以外转向，保持三个原生塔口各自独立。
			points := []Position{offset(float64(io.sign)*(portDistance/grid-.025), float64(row))}
			turn := 16
			if row != 0 {
				turn = 13
			}
			for x := first + 1; x <= turn; x++ {
				points = append(points, offset(float64(io.sign*x), float64(row)))
			}
			if row != 0 {
				direction := 1
				if row < 0 {
					direction = -1
				}
				for y := row - direction; y != 0; y -= direction {
					points = append(points, offset(float64(io.sign*13), float64(y)))
				}
				for x := 14; x <= 16; x++ {
					points = append(points, offset(float64(io.sign*x), float64(direction)))
				}
			}
			// 沿用单机模块已验证的塔内接带深度，纬向端点取对应原型侧口。
			points[len(points)-1] = offset(float64(io.sign)*16.1464, float64(station.PortPoses[port].Position[2])/(200*float64(latitudeStep)))
			if io.input {
				for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
					points[left], points[right] = points[right], points[left]
				}
			}
			for i, p := range points {
				idx := index + int32(len(objects))
				b := blueprint.Building{Index: idx, ItemID: 2003, ModelIndex: 37, InputObjIdx: -1, OutputObjIdx: idx + 1, InputToSlot: 1}
				if i == 0 {
					b.InputObjIdx, b.InputFromSlot = device.Building.Index, int8(io.devicePort)
					if io.input {
						b.InputObjIdx, b.InputFromSlot = towerIndex, int8(port)
						b.Parameters = []int32{int32(io.item), 0}
					}
				}
				if i == len(points)-1 {
					b.OutputObjIdx, b.OutputToSlot = towerIndex, int8(port)
					if io.input {
						b.OutputObjIdx, b.OutputToSlot = device.Building.Index, int8(io.devicePort)
					} else {
						b.Parameters = []int32{int32(io.item), 0}
					}
				}
				from, to := p, p
				if i+1 < len(points) {
					to = points[i+1]
				} else {
					from = points[i-1]
				}
				b.Yaw = float32(math.Atan2((to.Longitude-from.Longitude)*math.Cos(p.Latitude), to.Latitude-from.Latitude) * 180 / math.Pi)
				b.Yaw2 = b.Yaw
				objects = append(objects, Object{First: p, Second: p, Item: io.item, Module: "接收器共享物流", Building: b})
			}
		}
	}
	return objects, nil
}
