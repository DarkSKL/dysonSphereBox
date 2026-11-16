package geometry

// 文件职责：拥有切平面及游戏单精度球面转换、距离和显式逆转换。
// 使用：正式记录、供电及球面边界共用；边界：普通根点与带中心抬升分开，单精度路径不合并乘加或提前转双精度。

import (
	"fmt"
	"math"
)

// PlanarToWorld 职责：将普通东/北细格和高度层转换为米制切平面；使用：静态根点或带中心；边界：lift必须明确，不能用于球面Area。
func PlanarToWorld(point GridPoint, lift WorldLength) (WorldPoint, error) {
	if err := point.Validate(); err != nil {
		return WorldPoint{}, err
	}
	if point.Frame.Space != TangentGrid {
		return WorldPoint{}, fmt.Errorf("切平面转换不能消费球面网格")
	}
	if err := finite(float64(lift)); err != nil {
		return WorldPoint{}, err
	}
	result := WorldPoint{Frame: Frame{point.Frame.Planet, point.Frame.ID, TangentWorld}, X: WorldLength(point.East) * EquatorialGridLength, Y: WorldLength(point.Height)*LayerPitch + lift, Z: WorldLength(point.North) * EquatorialGridLength}
	return result, result.Validate()
}

// WorldToPlanar 职责：显式执行同框架切平面逆转换；使用：设备端口回到装配细格；边界：不能以固定赤道格长逆算高纬世界点。
func WorldToPlanar(point WorldPoint, target Frame, lift WorldLength) (GridPoint, error) {
	if err := point.Validate(); err != nil {
		return GridPoint{}, err
	}
	if err := target.Validate(); err != nil {
		return GridPoint{}, err
	}
	if point.Frame.Space != TangentWorld || target.Space != TangentGrid || target.ID != point.Frame.ID || target.Planet != point.Frame.Planet {
		return GridPoint{}, fmt.Errorf("切平面逆转换的框架不匹配")
	}
	if err := finite(float64(lift)); err != nil {
		return GridPoint{}, err
	}
	result := GridPoint{Frame: target, East: GridLength(point.X / EquatorialGridLength), North: GridLength(point.Z / EquatorialGridLength), Height: HeightLayers((point.Y - lift) / LayerPitch)}
	return result, result.Validate()
}

// PlanarDeltaLength 职责：先求网格差再按各轴实际单位计算长度；使用：配送覆盖；边界：保留原算法先相减后乘格长的舍入顺序，不改为两个绝对世界点相减。
func PlanarDeltaLength(first, second GridPoint) (WorldLength, error) {
	if err := first.Validate(); err != nil {
		return 0, err
	}
	if err := second.Validate(); err != nil {
		return 0, err
	}
	if first.Frame != second.Frame || first.Frame.Space != TangentGrid {
		return 0, fmt.Errorf("切平面距离的网格框架不匹配")
	}
	x := float64(first.East-second.East) * float64(EquatorialGridLength)
	y := float64(first.North-second.North) * float64(EquatorialGridLength)
	z := float64(first.Height-second.Height) * float64(LayerPitch)
	result := WorldLength(math.Sqrt(x*x + y*y + z*z))
	return result, finite(float64(result))
}

// DistanceSquared32 职责：以游戏float32逐分量相减并累加距离平方；使用：电力硬阈值；边界：不同世界框架和溢出明确失败。
func DistanceSquared32(first, second WorldPoint) (float32, error) {
	if err := first.Validate(); err != nil {
		return 0, err
	}
	if err := second.Validate(); err != nil {
		return 0, err
	}
	if first.Frame != second.Frame {
		return 0, fmt.Errorf("世界距离的坐标框架不匹配")
	}
	x, y, z := float32(first.X)-float32(second.X), float32(first.Y)-float32(second.Y), float32(first.Z)-float32(second.Z)
	result := x*x + y*y + z*z
	return result, finite(float64(result))
}

// SurfaceToWorld32 职责：保留游戏浮点三角函数和半径逐步乘加；使用：正式球面记录；边界：高度先转float32，半径按层高、.2、200顺序累加。
func SurfaceToWorld32(position SurfacePosition) (WorldPoint, error) {
	if err := position.Validate(); err != nil {
		return WorldPoint{}, err
	}
	latitude, longitude := float32(position.Latitude), float32(position.Longitude)
	radius := float32(position.Height)*float32(LayerPitch) + float32(BaseLift) + 200
	if radius <= 0 {
		return WorldPoint{}, fmt.Errorf("球面半径必须为正数")
	}
	cosLatitude := float32(math.Cos(float64(latitude)))
	result := WorldPoint{Frame: position.Frame,
		X: WorldLength(cosLatitude * float32(math.Sin(float64(longitude))) * radius),
		Y: WorldLength(float32(math.Sin(float64(latitude))) * radius),
		Z: WorldLength(cosLatitude * -float32(math.Cos(float64(longitude))) * radius)}
	return result, result.Validate()
}

// WorldToSurface 职责：从有框架的球心世界点恢复弧度及层高；使用：明确的诊断和往返转换；边界：双精度逆解不冒充游戏自动吸附或逐位可逆。
func WorldToSurface(point WorldPoint) (SurfacePosition, error) {
	if err := point.Validate(); err != nil {
		return SurfacePosition{}, err
	}
	if point.Frame.Space != PlanetWorld {
		return SurfacePosition{}, fmt.Errorf("球面逆转换需要球心世界点")
	}
	x, y, z := float64(point.X), float64(point.Y), float64(point.Z)
	radius := math.Sqrt(x*x + y*y + z*z)
	if radius <= 1e-5 || math.IsInf(radius, 0) {
		return SurfacePosition{}, fmt.Errorf("球面逆转换缺少有效径向位置")
	}
	result := SurfacePosition{Frame: point.Frame, Longitude: Radians(math.Atan2(x, -z)), Latitude: Radians(math.Asin(y / radius)), Height: HeightLayers((radius - 200 - float64(BaseLift)) / float64(LayerPitch))}
	return result, result.Validate()
}

// SurfaceFrame 职责：保存Area局部网格到球面的显式框架和步长；使用：正式记录与原生框架共享；边界：原生固定步长与实际粘贴步长由调用者明确选择。
type SurfaceFrame struct {
	// Grid 职责：标识被消费的Area网格；使用：匹配局部点；边界：不同Area不混算。
	Grid Frame
	// Origin 职责：保存球面原点及世界身份；使用：游戏逐次乘加；边界：原点高度必须为零。
	Origin SurfacePosition
	// LongitudeStep/LatitudeStep 职责：保存每细格弧度步长；使用：真实纬带网格；边界：必须为有限正数。
	LongitudeStep, LatitudeStep Radians
}

// Project32 职责：按origin+float32(offset)*step顺序映射一个Area点；使用：球面正式路径；边界：不合并origin和offset，不用赤道格长近似经向长度。
func (frame SurfaceFrame) Project32(point GridPoint) (WorldPoint, error) {
	return frame.project32(point, false)
}

// ProjectNative32 职责：保留零经度原生框架直接乘经向步长的语义；使用：极区路由；边界：省去加零以保留负零，不与正式粘贴origin加法路径机械合并。
func (frame SurfaceFrame) ProjectNative32(point GridPoint) (WorldPoint, error) {
	if frame.Origin.Longitude != 0 {
		return WorldPoint{}, fmt.Errorf("原生框架需要零经度起点")
	}
	return frame.project32(point, true)
}

// project32 职责：共用已验证框架及球面三角核；使用：正式粘贴或原生具名路径；边界：只在显式native分支省略经度加零。
func (frame SurfaceFrame) project32(point GridPoint, native bool) (WorldPoint, error) {
	if err := frame.Grid.Validate(); err != nil {
		return WorldPoint{}, err
	}
	if err := frame.Origin.Validate(); err != nil {
		return WorldPoint{}, err
	}
	if err := point.Validate(); err != nil {
		return WorldPoint{}, err
	}
	if frame.Grid.Space != SurfaceGrid || point.Frame != frame.Grid || frame.Grid.Planet != frame.Origin.Frame.Planet || frame.Origin.Height != 0 {
		return WorldPoint{}, fmt.Errorf("球面Area框架不匹配")
	}
	if err := finite(float64(frame.LongitudeStep), float64(frame.LatitudeStep)); err != nil {
		return WorldPoint{}, err
	}
	if frame.LongitudeStep <= 0 || frame.LatitudeStep <= 0 {
		return WorldPoint{}, fmt.Errorf("球面细格步长必须为正数")
	}
	longitude := float32(frame.Origin.Longitude) + float32(point.East)*float32(frame.LongitudeStep)
	if native {
		longitude = float32(point.East) * float32(frame.LongitudeStep)
	}
	latitude := float32(frame.Origin.Latitude) + float32(point.North)*float32(frame.LatitudeStep)
	return SurfaceToWorld32(SurfacePosition{Frame: frame.Origin.Frame, Longitude: Radians(longitude), Latitude: Radians(latitude), Height: point.Height})
}
