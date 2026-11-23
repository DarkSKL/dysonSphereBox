package galaxy

// 文件职责：复现游戏 Windows 原生球面插值的单精度三角运算。
// 使用：生成原版球面顶点网格；边界：仅支持插值用到的角度范围。
// 算法参考 DSPSeedCalc 的 MIT 实现，授权见 DSPSEEDCALC-LICENSE.txt。
import "math"

// terrainSinCos 职责：计算 UnityPlayer 使用的单精度正余弦；使用：球面插值；边界：输入角度小于 2π。
func terrainSinCos(angle float32) (float32, float32) {
	x := float64(angle)
	abs := math.Abs(x)
	region := 0
	r := x
	if abs > math.Pi/4 {
		scaled := math.FMA(.6366197723675814, abs, .5)
		n := math.Trunc(scaled)
		r = math.FMA(-n, 1.5707963267341256, abs) - n*6.077100506506192e-11
		region = int(n) & 3
	}
	xx := r * r
	var sine, cosine float64
	switch {
	case abs < .0001220703125:
		sine, cosine = x, 1
	case abs < .0078125:
		sine = math.FMA(-(x * .5), xx, x)
		cosine = math.FMA(-(x * .5), x, 1)
	default:
		sin := math.FMA(xx, 2.7557319223985893e-6, -.0001984126984126984)
		sin = math.FMA(sin, xx, .008333333333333333)
		sin = math.FMA(sin, xx, -.16666666666666666)
		sine = math.FMA(sin, r*xx, r)
		cos := math.FMA(xx, -2.755731922398589e-7, 2.4801587301587298e-5)
		cos = math.FMA(cos, xx, -.0013888888888888887)
		cos = math.FMA(cos, xx, .041666666666666664)
		base := 1 - xx*.5
		if abs > math.Pi/4 {
			base = math.FMA(xx, -.5, 1)
		}
		cosine = math.FMA(cos, xx*xx, base)
	}
	switch region {
	case 1:
		sine, cosine = cosine, -sine
	case 2:
		sine, cosine = -sine, -cosine
	case 3:
		sine, cosine = -cosine, sine
	}
	if abs > math.Pi/4 && math.Signbit(x) {
		sine = -sine
	}
	return float32(sine), float32(cosine)
}

// terrainAcos 职责：计算 UnityPlayer 使用的单精度反余弦；使用：球面插值；边界：保持原版多项式舍入。
func terrainAcos(value float32) float32 {
	raw := math.Float32bits(value)
	exponent := (raw >> 23) & 255
	magnitude := math.Float32frombits(raw & 0x7fffffff)
	if exponent < 0x65 {
		return float32(math.Pi / 2)
	}
	if exponent >= 0x7f {
		return float32(math.Acos(float64(value)))
	}
	z, root := magnitude*magnitude, float32(0)
	if exponent >= 0x7e {
		z = (1 - magnitude) * .5
		root = float32(math.Sqrt(float64(z)))
	}
	fromBits := math.Float32frombits
	numerator := fromBits(0xbc5b3fe1) - z*fromBits(0x3b81ce6b)
	numerator = numerator*z - fromBits(0x3d678bdd)
	numerator = numerator*z + fromBits(0x3e3c94dc)
	numerator *= z
	denominator := fromBits(0x3f8d6fa5) - z*fromBits(0x3f561f0d)
	ratio := numerator / denominator
	const piOver2Low = 6.123233995736766e-17
	if exponent < 0x7e {
		product := ratio * value
		return float32(math.Pi/2 - (float64(value) - (piOver2Low - float64(product))))
	}
	if value >= 0 {
		high := fromBits(math.Float32bits(root) & 0xffff0000)
		rootTerm := (root + root) * ratio
		correction := (z - high*high) / (high + root)
		return rootTerm + (correction + correction) + high*2
	}
	product := root * ratio
	inner := float64(root) + (float64(product) - piOver2Low)
	return float32(math.Pi - (inner + inner))
}
