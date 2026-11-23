package galaxy

// 文件职责：实现行星地形所需的带种子三维单纯形噪声。
// 使用：原版地形算法；边界：保留原版梯度、随机交换与运算顺序。
import "math"

// simplexNoise 职责：保存地形梯度置换；使用：单球高度求值；边界：置换表构建后只读。
type simplexNoise [512]int

// newSimplexNoise 职责：由种子建立原版置换表；使用：选择地形算法时初始化；边界：独立于其他地形随机流。
func newSimplexNoise(seed int32) *simplexNoise {
	var permutation simplexNoise
	random := newRandom(seed)
	for i := 0; i < 256; i++ {
		permutation[i] = i
	}
	for i := 0; i < 256; i++ {
		j := int(random.unit() * 256)
		permutation[i], permutation[j] = permutation[j], permutation[i]
	}
	copy(permutation[256:], permutation[:256])
	return &permutation
}

// noise 职责：计算单个三维噪声值；使用：地形及 FBM；边界：使用原版缩放系数。
func (p *simplexNoise) noise(x, y, z float64) float64 {
	const pointSpacing = 1.0 / 6
	skew := (x + y + z) * (1.0 / 3)
	i, j, k := int(math.Floor(x+skew)), int(math.Floor(y+skew)), int(math.Floor(z+skew))
	unskew := float64(i+j+k) * pointSpacing
	origin := Vector{x - (float64(i) - unskew), y - (float64(j) - unskew), z - (float64(k) - unskew)}

	axes := [3]int{0, 1, 2}
	coords := [3]float64{origin.X, origin.Y, origin.Z}
	for a := 1; a < len(axes); a++ {
		for b := a; b > 0 && coords[axes[b]] > coords[axes[b-1]]; b-- {
			axes[b], axes[b-1] = axes[b-1], axes[b]
		}
	}
	gradients := [...]Vector{
		{1, 1, 0}, {-1, 1, 0}, {1, -1, 0}, {-1, -1, 0},
		{1, 0, 1}, {-1, 0, 1}, {1, 0, -1}, {-1, 0, -1},
		{0, 1, 1}, {0, -1, 1}, {0, 1, -1}, {0, -1, -1},
	}
	i, j, k = i&255, j&255, k&255
	corner, sum := [3]int{}, 0.0
	for n := 0; n < 4; n++ {
		dx := origin.X - float64(corner[0]) + float64(n)*pointSpacing
		dy := origin.Y - float64(corner[1]) + float64(n)*pointSpacing
		dz := origin.Z - float64(corner[2]) + float64(n)*pointSpacing
		attenuation := .6 - dx*dx - dy*dy - dz*dz
		if attenuation >= 0 {
			gradient := gradients[p[i+corner[0]+p[j+corner[1]+p[k+corner[2]]]]%12]
			attenuation *= attenuation
			sum += attenuation * attenuation * (gradient.X*dx + gradient.Y*dy + gradient.Z*dz)
		}
		if n < 3 {
			corner[axes[n]] = 1
		}
	}
	return sum * 32.696434
}

// fractal 职责：叠加多层噪声；使用：高度生成；边界：振幅和波长分别按原版参数推进。
func (p *simplexNoise) fractal(x, y, z float64, octaves int, persistence, lacunarity float64) float64 {
	return p.fractalWithAmplitude(x, y, z, octaves, persistence, lacunarity, .5, false)
}

// fractalWithAmplitude 职责：累积带初始振幅的噪声；使用：冰面地形；边界：ridge 模式逐层取绝对值。
func (p *simplexNoise) fractalWithAmplitude(x, y, z float64, octaves int, persistence, lacunarity, amplitude float64, ridge bool) float64 {
	sum := 0.0
	for i := 0; i < octaves; i++ {
		noise := p.noise(x, y, z) * amplitude
		if ridge {
			noise = math.Abs(noise)
		}
		sum += noise
		amplitude *= persistence
		x, y, z = x*lacunarity, y*lacunarity, z*lacunarity
	}
	return sum
}
