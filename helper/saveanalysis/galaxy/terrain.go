package galaxy

// 文件职责：生成原版星球的基础地形高度网格。
// 使用：按需空间放置及矿脉生成；边界：高度为百分之一游戏米，改造状态保留在存档层。
import (
	"context"
	"math"
	"sync"
)

// HeightGrid 职责：保存一颗星球的基础高度网格；使用：空间查询；边界：四面打包索引与游戏 PlanetRawData 一致。
type HeightGrid struct {
	Precision int
	Heights   []uint16
}

// terrainPoint 职责：保留球面网格的单精度原始顶点；使用：高度索引；边界：组件为单位球方向。
type terrainPoint struct{ x, y, z float32 }

var terrainVertices = sync.OnceValue(func() []terrainPoint {
	const precision = 200
	const faceSize = precision + 1
	const stride = faceSize * 2
	poles := [...]terrainPoint{{x: 1}, {x: -1}, {y: 1}, {y: -1}, {z: 1}, {z: -1}}
	faces := [8][3]int{{2, 0, 4}, {3, 4, 0}, {2, 4, 1}, {3, 1, 4}, {2, 1, 5}, {3, 5, 1}, {2, 5, 0}, {3, 0, 5}}
	out := make([]terrainPoint, stride*stride)
	for index := range out {
		column, row := index%stride, index/stride
		x, y := column%faceSize, row%faceSize
		face := (column/faceSize + 2*(row/faceSize)) * 2
		bottom, across := precision-x, y
		if x < y {
			face++
			bottom, across = x, precision-y
		}
		t := float32(0)
		if across < precision {
			t = float32(bottom) / float32(precision-across)
		}
		u := float32(across) / precision
		a, b, c := poles[faces[face][0]], poles[faces[face][1]], poles[faces[face][2]]
		out[index] = terrainSlerp(terrainSlerp(a, c, u), terrainSlerp(b, c, u), t)
	}
	return out
})

// terrainSlerp 职责：复现相邻方向间的 Unity 球面插值；使用：球面网格；边界：网格方向不会互相相反。
func terrainSlerp(a, b terrainPoint, t float32) terrainPoint {
	lengthA := float32(math.Sqrt(float64(a.x*a.x + a.y*a.y + a.z*a.z)))
	lengthB := float32(math.Sqrt(float64(b.x*b.x + b.y*b.y + b.z*b.z)))
	cosine := (b.x*a.x + b.y*a.y + b.z*a.z) / (lengthA * lengthB)
	if cosine > math.Float32frombits(0x3f7fff58) {
		inverse := 1 - t
		return terrainPoint{b.x*t + a.x*inverse, b.y*t + a.y*inverse, b.z*t + a.z*inverse}
	}
	x, y, z := b.z*a.y-b.y*a.z, b.x*a.z-b.z*a.x, b.y*a.x-b.x*a.y
	length := float32(math.Sqrt(float64(y*y + x*x + z*z)))
	x, y, z = x/length, y/length, z/length
	angle := terrainAcos(cosine) * t
	sin, cos := terrainSinCos(angle)
	m := 1 - cos
	ax, ay, az := a.x/lengthA, a.y/lengthA, a.z/lengthA
	magnitude := (1-t)*lengthA + lengthB*t
	return terrainPoint{
		((m*(y*x)-z*sin)*ay + (x*x*m+cos)*ax + (m*(z*x)+y*sin)*az) * magnitude,
		((y*y*m+cos)*ay + (m*(y*x)+z*sin)*ax + (m*(z*y)-x*sin)*az) * magnitude,
		((m*(z*y)+x*sin)*ay + (m*(z*x)-y*sin)*ax + (z*z*m+cos)*az) * magnitude,
	}
}

// TerrainHeights 职责：按主题和种子生成基础高度；使用：按需星球空间查询；边界：尚未支持的算法返回未知。
func TerrainHeights(planet Planet, theme SurfaceTheme) (HeightGrid, bool) {
	grid, ok, _ := TerrainHeightsContext(context.Background(), planet, theme)
	return grid, ok
}

// TerrainHeightsContext 职责：生成可取消的基础高度网格；使用：候选精化；边界：取消后不返回不完整网格。
func TerrainHeightsContext(ctx context.Context, planet Planet, theme SurfaceTheme) (HeightGrid, bool, error) {
	if err := ctx.Err(); err != nil {
		return HeightGrid{}, false, err
	}
	if theme.ID == 0 {
		return HeightGrid{}, false, nil
	}
	if theme.Algorithm == 0 {
		const precision = 64
		heights := make([]uint16, (precision+1)*(precision+1)*4)
		for i := range heights {
			heights[i] = 8000
		}
		return HeightGrid{precision, heights}, true, nil
	}
	height := terrainAlgorithm(planet.GenSeed, theme)
	if height == nil {
		return HeightGrid{}, false, nil
	}
	verts := terrainVertices()
	out := HeightGrid{200, make([]uint16, len(verts))}
	for i, p := range verts {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return HeightGrid{}, false, err
			}
		}
		out.Heights[i] = uint16(100 * height(float64(p.x)*200, float64(p.y)*200, float64(p.z)*200))
	}
	return out, true, nil
}

// terrainAlgorithm 职责：按原版编号选择基础高度函数；使用：网格及矿脉生成；边界：不生成生物分布。
func terrainAlgorithm(seed int32, theme SurfaceTheme) func(float64, float64, float64) float64 {
	r := newRandom(seed)
	a, b := newSimplexNoise(r.integer()), newSimplexNoise(r.integer())
	switch theme.Algorithm {
	case 1, 7:
		xFrequency, yFrequency, bias, offset := .01, .012, -.2, .2
		if theme.Algorithm == 7 {
			xFrequency, yFrequency, bias, offset = .008, .01, -2.4, 0
		}
		return func(x, y, z float64) float64 {
			base := a.fractal(x*xFrequency, y*yFrequency, z*.01, 6, .5, 2)*3 + bias
			blend := b.fractal(x*.0025, y*.0025, z*.0025, 3, .5, 2)*3*.9 + .5
			if blend > 0 {
				blend *= .5
			}
			value := base + blend
			if value > 0 {
				return 200 + terrainLevelize(value*.5, .7, 3) + offset
			}
			return 200 + terrainLevelize(value*1.6, .5, 2) + offset
		}
	case 2:
		modX := (3 - theme.ModX - theme.ModX) * theme.ModX * theme.ModX
		scale := 1 + 1.3*theme.ModY
		horizontal, vertical := .0035*scale, (.025*modX+.0035*(1-modX))*scale
		return func(x, y, z float64) float64 {
			noise := a.fractal(x*horizontal, y*vertical, z*horizontal, 6, .45, 1.8)
			value := .6/(math.Abs(noise*3+3*.4)+.6) - .25
			if value < 0 {
				value *= .3
			}
			return 200 + value + .1
		}
	case 3:
		return func(x, y, z float64) float64 {
			x += math.Sin(y*.15) * 3
			y += math.Sin(z*.15) * 3
			z += math.Sin(x*.15) * 3
			base := a.fractal(x*.007, y*.007*1.1, z*.007, 6, .5, 1.8)
			ridge := b.fractal(x*.007*1.3+.5, y*.007*2.8+.2, z*.007*1.3+.7, 3, .5, 2) * 2
			detail := b.fractal(x*.007*6, y*.007*12, z*.007*6, 2, .5, 2) * 2
			detail += (detail*.1 - detail) * theme.ModX
			blend := b.fractal(x*.007*.8, y*.007*.8, z*.007*.8, 2, .5, 2) * 2
			value := base*2 + .92 + math.Min(1, math.Max(0, single(ridge*math.Abs(single(single(blend)+.5))-.35)))
			if value < 0 {
				value *= 2
			}
			level := terrainLevelize(value, 1, 2)
			if level > 0 {
				terraced := terrainLevelize(level, 1, 4)
				level = terraced + (level-terraced)*theme.ModX
			}
			t := single(level)
			var low, high float64
			switch {
			case level <= 0:
				low, high = terrainLerp(-4, 0, t+1), terrainLerp(-1, 0, t+1)
			case level <= 1:
				low = terrainLerp(0, .3, t) + detail*.1
				high = low
			case level <= 2:
				low, high = terrainLerp(.3, 1.4, t-1)+detail*.12, terrainLerp(.3, 1.2, t-1)+detail*.12
			default:
				low, high = terrainLerp(1.4, 2.7, t-2)+detail*.12, terrainLerp(1.2, 2, t-2)+detail*.12
			}
			return 200 + (low + (high-low)*theme.ModX) + .2
		}
	case 4:
		return terrainCraterHeight(a, b, r)
	case 5, 6:
		return func(x, y, z float64) float64 {
			tx := terrainLevelize(x*.007, 1, 1) + a.noise(x*.05, y*.05, z*.05)*.04
			ty := terrainLevelize(y*.007, 1, 1) + a.noise(y*.05, z*.05, x*.05)*.04
			tz := terrainLevelize(z*.007, 1, 1) + a.noise(z*.05, x*.05, y*.05)*.04
			low := math.Abs(b.noise(tx, ty, tz))
			ridge := math.Min(1, math.Max(0, (.16-low)*10))
			ridge *= ridge
			blend := math.Min(1, math.Max(0, (a.fractal(y*.005, z*.005, x*.005, 4, .5, 2)+.22)*5))
			height := -ridge * 1.2 * blend
			if height >= 0 {
				detail := math.Abs(b.fractal(tx*1.5, ty*1.5, tz*1.5, 2, .5, 2))
				height += low*.25 + detail*.6
			}
			height -= .1
			depth := -.3 - height
			if depth > 0 {
				depth = math.Min(depth, 1)
				depth = (3 - depth - depth) * depth * depth
				height = -.3 - depth*single(3.7)
				if theme.Algorithm == 5 {
					detail := b.noise(x*.16, y*.16, z*.16) - 1
					height += depth * depth * depth * depth * detail * .5
				}
			}
			if theme.Algorithm == 6 {
				if height <= single(-.8) {
					shelf := terrainLevelize(math.Max(ridge, single(.3)), .7, 1)
					height = (-shelf - low) * single(.9)
				}
				height = math.Max(height, single(-1.2))
			}
			return 200 + height + .2
		}
	case 8:
		horizontal, vertical := .002*theme.ModX, .002*theme.ModX*theme.ModX*6.66667
		return func(x, y, z float64) float64 {
			value := math.Max(0, math.Min(2, single(single(a.fractal(x*horizontal, y*vertical, z*horizontal, 6, .45, 1.8))+1+single(theme.ModY)*single(.01))))
			base := 1.0
			if value >= 1 {
				value, base = single(value-1), 2
			}
			curve := single(single(math.Cos(single(value*single(math.Pi)))) * single(1.1))
			curve = math.Copysign(power(curve, 4), curve)
			curve = math.Max(-1, math.Min(1, curve))
			return 200 + single(base-(curve+1)*.5) + .1
		}
	case 9:
		return func(x, y, z float64) float64 {
			base := a.fractal(x*.01*.75, y*.012*.5, z*.01*.75, 6, .5, 2)*3 - .2
			blend := b.fractal(x*.0025, y*.0025, z*.0025, 3, .5, 2)*3*.9 + .5
			if blend > 0 {
				blend *= .5
			}
			value := base + blend
			level := terrainLevelize(value*1.6, .5, 2)
			if value > 0 {
				level = terrainLevelize(value*.5, .7, 3)
			}
			level += .618
			if level > -1 {
				level *= 1.5
			} else {
				level *= 4
			}
			if level < -theme.ModY {
				deepBase := a.fractal(x*.01*theme.ModX, y*.012*theme.ModX, z*.01*theme.ModX, 6, .5, 2)*3 - .2
				deep := math.Pow((deepBase+blend+5)*.13, 6)*24 - 24
				fraction := math.Min(math.Abs(level+theme.ModY)/5, 1)
				level = level*(1-fraction) + deep*fraction
			}
			if level > 0 {
				level *= .5
			}
			return 200 + level + .2
		}
	case 10:
		return terrainEllipticHeight(a, b, r)
	case 11:
		c := newSimplexNoise(r.integer())
		return func(x, y, z float64) float64 {
			low := b.fractal(x*.007*4, y*.007*8, z*.007*4, 3, .5, 2)
			first := a.fractal(x*.007*.6, y*.007*1.5*2.5, z*.007*.6, 6, .45, 1.8)*.95 + low*.05
			x1 := terrainLevelize(math.Pow((first+1)*.5, theme.ModY)+1, 1, 2)
			second := c.fractal(x*(.002*theme.ModX), y*(.002*theme.ModX*4), z*(.002*theme.ModX), 5, .55, 2)
			x2 := terrainLevelize(math.Pow((second+1)*.5, .65), 1, 3)
			return 200 + math.Max(-.3, (x2*x1-.4)*.9)
		}
	case 12:
		scale := 1.1 * theme.ModX
		return func(x, y, z float64) float64 {
			x, y, z = x/200, y/200, z/200
			latitude := math.Abs(math.Asin(y)) * 2 / math.Pi
			y *= 2.5 * theme.ModY
			warp := b.fractal(x*scale, y*scale, z*scale, 3, .4, 2) * .2
			ridge := a.fractalWithAmplitude(x*scale, y*scale-warp, z*scale, 6, .7, 2, .8, true)
			fractal := a.fractalWithAmplitude(x*scale, y*scale-warp, z*scale, 6, .6, 2, .7, false)
			fractal *= ridge + fractal
			fractal = .2 + 8*fractal*ridge
			value := ((fractal + .5) + 8) / 16
			value = math.Pow(math.Min(1, math.Max(0, value))+.5, 1.5)
			latitude /= 1 / .9
			latitude = single(latitude) / .6
			if latitude < 1 {
				oneMinus := 1 - latitude
				value -= math.Pow(oneMinus, 3) + math.Pow(oneMinus, 2)*3*latitude
			}
			return 200 + math.Min(2, math.Max(0, value*2))*1.1 - .2
		}
	case 13:
		scale := .007 * theme.ModX
		return func(x, y, z float64) float64 {
			base := a.fractal(x*scale, y*scale, z*scale, 6, .5, 2)
			value := math.Pow((base+1)*.5, theme.ModY) * 3.0625 * 2
			if value < 1 {
				value *= value
			}
			height := math.Min(value-.2, 4)
			switch {
			case height > 3.5:
				height = 1 + 2*(height-3.5)
			case height > 3:
				height = 1
			case height > 2:
				height = 2 - (height - 2)
			}
			return 200 + height + .1
		}
	}
	return nil
}

// terrainLevelize 职责：多次平滑分段小数；使用：原版地形台阶；边界：重复次数由对应算法指定。
func terrainLevelize(value, step float64, repeats int) float64 {
	value /= step
	whole := math.Floor(value)
	fragment := value - whole
	for i := 0; i < repeats; i++ {
		fragment = (3 - fragment - fragment) * fragment * fragment
	}
	return (whole + fragment) * step
}

// terrainLerp 职责：执行原版 Mathf.Lerp；使用：高度映射；边界：按单精度边界截断。
func terrainLerp(a, b, t float64) float64 {
	t = math.Min(1, math.Max(0, single(t)))
	return single(single(a) + (single(b)-single(a))*t)
}
