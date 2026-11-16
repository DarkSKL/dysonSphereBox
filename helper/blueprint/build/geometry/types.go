package geometry

// 文件职责：维护共同几何边界的量纲、星球和坐标框架身份。
// 使用：各模式只在网格、世界、球面和设备查询交接处转换；边界：不依赖装配器、工作流或服务，未知框架和非法数值显式失败。

import (
	"fmt"
	"math"
)

// GridLength 职责：表示蓝图细网格长度；使用：局部东/北坐标与跨度；边界：不是米或高度层。
type GridLength float64

// WorldLength 职责：表示游戏世界米制长度；使用：真实几何和物理查询；边界：不隐式转为细格。
type WorldLength float64

// HeightLayers 职责：表示蓝图高度层；使用：高度到世界半径转换；边界：与纬向细格严格分开。
type HeightLayers float64

// Degrees 职责：表示角度制；使用：显式角度边界；边界：不能直接传给弧度接口。
type Degrees float64

// Radians 职责：表示弧度；使用：球面和局部框架；边界：不与度数隐式混用。
type Radians float64

// ItemsPerMinute 职责：表示每分钟物品额定流量；使用：设备事实交接；边界：不等于每秒流量或实际运行保证。
type ItemsPerMinute float64

// Planet 职责：绑定已核对的星球规则身份；使用：所有几何框架；边界：不推断未知半径和Mod规则。
type Planet string

// Vanilla200 职责：标识已核对构建的原版200分段星球；使用：当前唯一球面规则；边界：不是用户存档运行版本探测。
const Vanilla200 Planet = "vanilla-200/build-25955035"

// Space 职责：区分坐标轴语义；使用：框架匹配；边界：同一数值在不同空间不能直接比较。
type Space string

const (
	// TangentGrid 职责：标识赤道切平面东/北细格；使用：普通平面装配边界；边界：不模拟高纬收缩。
	TangentGrid Space = "tangent-grid"
	// SurfaceGrid 职责：标识指定球面框架内东/北细格；使用：Area局部坐标；边界：必须提供显式球面变换。
	SurfaceGrid Space = "surface-grid"
	// TangentWorld 职责：标识x东、y上、z北的世界长度；使用：切平面OBB；边界：不能与行星世界点混算。
	TangentWorld Space = "tangent-world"
	// PlanetWorld 职责：标识游戏球心原点的x/y/z；使用：球面真实几何；边界：不是Area局部坐标。
	PlanetWorld Space = "planet-world"
)

const (
	// EquatorialGridLength 职责：保存原版基面赤道细格世界长度；使用：切平面唯一换算；边界：不得用于全部纬带。
	EquatorialGridLength WorldLength = 200.2 * 2 * math.Pi / 1000
	// LayerPitch 职责：保存蓝图一高度层的世界长度；使用：普通双精度转换；边界：游戏单精度需逐次舍入。
	LayerPitch WorldLength = 1.3333333
	// BaseLift 职责：保存建筑根点相对200半径基球的抬升；使用：普通格点；边界：不等于带中心抬升。
	BaseLift WorldLength = .2
	// BeltCenterLift 职责：保存平面带中心相对200半径基球的抬升；使用：带几何；边界：不改建筑根点。
	BeltCenterLift WorldLength = .4
)

// Frame 职责：明确所属星球、命名框架和轴空间；使用：点、方向和距离交接；边界：空ID、未知星球或空间均拒绝。
type Frame struct {
	// Planet 职责：绑定规则来源；使用：跨点一致性；边界：当前只支持Vanilla200。
	Planet Planet
	// ID 职责：区分不同局部原点；使用：拒绝不同Area混算；边界：不能为空。
	ID string
	// Space 职责：声明坐标轴语义；使用：转换分派；边界：未知值失败。
	Space Space
}

// Validate 职责：核对框架身份；使用：所有公共转换入口；边界：不为未知规则生成默认原点。
func (frame Frame) Validate() error {
	if frame.Planet != Vanilla200 || frame.ID == "" {
		return fmt.Errorf("未知星球或缺少坐标框架身份")
	}
	switch frame.Space {
	case TangentGrid, SurfaceGrid, TangentWorld, PlanetWorld:
		return nil
	}
	return fmt.Errorf("未知坐标空间 %q", frame.Space)
}

// PlanarGridFrame 职责：提供普通装配默认细格框架；使用：旧vector进入共同边界；边界：不能代替球面Area。
func PlanarGridFrame() Frame { return Frame{Vanilla200, "equatorial-tangent", TangentGrid} }

// PlanarWorldFrame 职责：提供对应切平面世界框架；使用：真实平面距离；边界：与行星世界不同。
func PlanarWorldFrame() Frame { return Frame{Vanilla200, "equatorial-tangent", TangentWorld} }

// PlanetWorldFrame 职责：提供球心原点世界框架；使用：球面碰撞和度量；边界：不包含Area局部原点。
func PlanetWorldFrame() Frame { return Frame{Vanilla200, "planet-center", PlanetWorld} }

// GridPoint 职责：保存有框架的东/北细格及高度层；使用：装配到几何边界；边界：字段可按值传递，每次转换仍核验有限值。
type GridPoint struct {
	// Frame 职责：声明局部原点；使用：转换一致性；边界：不能混用两个Area。
	Frame Frame
	// East/North 职责：保存横纵细格；使用：平面或球面步长；边界：不是米。
	East, North GridLength
	// Height 职责：保存相对基面的高度层；使用：世界高度；边界：不是细格长度。
	Height HeightLayers
}

// WorldPoint 职责：保存有框架的世界米制点；使用：真实物理度量；边界：不暴露无单位数组作为公共输入。
type WorldPoint struct {
	// Frame 职责：声明世界空间；使用：距离与球面度量；边界：框架不同即拒绝。
	Frame Frame
	// X/Y/Z 职责：保存世界坐标；使用：统一距离和几何适配；边界：三轴均为世界长度。
	X, Y, Z WorldLength
}

// SurfacePosition 职责：以显式弧度和星球世界身份表示球面位置；使用：世界转换；边界：经度可跨接缝，纬度只能在正负π/2内。
type SurfacePosition struct {
	// Frame 职责：标识球心世界框架；使用：跨位置一致性；边界：必须为PlanetWorld。
	Frame Frame
	// Longitude/Latitude 职责：保存经纬弧度；使用：球面三角计算；边界：不接受无量纲度数。
	Longitude, Latitude Radians
	// Height 职责：保存蓝图高度层；使用：世界半径；边界：与半径米数分开。
	Height HeightLayers
}

// finite 职责：统一拒绝非有限数值；使用：所有单位边界；边界：不替换为零或无限值。
func finite(values ...float64) error {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("坐标或单位包含非有限数值")
		}
	}
	return nil
}

// ToRadians 职责：显式由度转换弧度；使用：外部角度参数；边界：非有限值失败。
func ToRadians(value Degrees) (Radians, error) {
	if err := finite(float64(value)); err != nil {
		return 0, err
	}
	result := Radians(float64(value) * math.Pi / 180)
	return result, finite(float64(result))
}

// ToDegrees 职责：显式由弧度转换度；使用：展示及交接；边界：非有限值失败。
func ToDegrees(value Radians) (Degrees, error) {
	if err := finite(float64(value)); err != nil {
		return 0, err
	}
	result := Degrees(float64(value) * 180 / math.Pi)
	return result, finite(float64(result))
}

// Validate 职责：核对网格点空间及有限值；使用：转换边界；边界：不自动转换世界坐标。
func (point GridPoint) Validate() error {
	if err := point.Frame.Validate(); err != nil {
		return err
	}
	if point.Frame.Space != TangentGrid && point.Frame.Space != SurfaceGrid {
		return fmt.Errorf("网格点不能使用世界框架")
	}
	return finite(float64(point.East), float64(point.North), float64(point.Height))
}

// Validate 职责：核对世界点空间及有限值；使用：物理查询边界；边界：不以原点代替非法点。
func (point WorldPoint) Validate() error {
	if err := point.Frame.Validate(); err != nil {
		return err
	}
	if point.Frame.Space != TangentWorld && point.Frame.Space != PlanetWorld {
		return fmt.Errorf("世界点不能使用网格框架")
	}
	return finite(float64(point.X), float64(point.Y), float64(point.Z))
}

// Validate 职责：核对球面角度和框架；使用：世界映射；边界：零半径等后续转换错误仍必须拒绝。
func (point SurfacePosition) Validate() error {
	if err := point.Frame.Validate(); err != nil {
		return err
	}
	if point.Frame.Space != PlanetWorld {
		return fmt.Errorf("球面位置需要球心世界框架")
	}
	if err := finite(float64(point.Longitude), float64(point.Latitude), float64(point.Height)); err != nil {
		return err
	}
	if math.Abs(float64(point.Latitude)) > math.Pi/2+1e-7 {
		return fmt.Errorf("纬度超过已核对的球面范围")
	}
	return nil
}

// Validate 职责：核对流量单位值；使用：设备额定能力；边界：负值和非有限流量不合法。
func (rate ItemsPerMinute) Validate() error {
	if err := finite(float64(rate)); err != nil {
		return err
	}
	if rate < 0 {
		return fmt.Errorf("每分钟物品流量不能为负数")
	}
	return nil
}
