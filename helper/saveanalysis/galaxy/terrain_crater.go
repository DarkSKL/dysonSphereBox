package galaxy

// 文件职责：生成带随机陨坑与椭圆地貌的原版基础地表。
// 使用：地形算法 4 与 10；边界：只计算放置和矿脉所需的高度。
// 算法参考 DSPSeedCalc 的 MIT 实现，授权见 DSPSEEDCALC-LICENSE.txt。
import (
	"math"
	"sync"
)

var sphericNormals = sync.OnceValue(func() []Vector {
	random := newRandom(1001)
	// 原版 RandomTable 使用系统 Mono Random，比游戏 DotNet35Random 少十格初始间隔。
	random.other = 21
	unit := func() float64 {
		value := random.sample()
		if value == math.MaxInt32 {
			value--
			random.values[random.next] = value
		}
		return float64(value) * (1.0 / 2147483647)
	}
	vectors := make([]Vector, 65536)
	for i := range vectors {
		for {
			x, y, z := unit()*2-1, unit()*2-1, unit()*2-1
			p := unit()
			angle := unit() * math.Pi * 2
			weight := math.Sqrt(-2*math.Log(1-p)) * math.Sin(angle)
			square := x*x + y*y + z*z
			if math.Abs(weight) > 5 || square > 1 || square < 1e-6 {
				continue
			}
			scale := weight / math.Sqrt(square)
			vectors[i] = Vector{x * scale, y * scale, z * scale}
			break
		}
	}
	return vectors
})

// terrainFeature 职责：保存地貌轮廓；使用：陨坑和椭圆高度；边界：坐标使用原版局部游戏米。
type terrainFeature struct {
	point                        terrainPoint
	square, height, eccentricity float64
}

// terrainCraterHeight 职责：生成算法 4 高度求值器；使用：基础地表；边界：轮廓由原版随机表抽取。
func terrainCraterHeight(a, b *simplexNoise, random *random) func(float64, float64, float64) float64 {
	table, seed := sphericNormals(), int(random.integer())
	features := make([]terrainFeature, 80)
	for i := range features {
		seed = (seed + 1) & 65535
		v := table[seed]
		p := terrainPoint{float32(v.X), float32(v.Y), float32(v.Z)}
		radius := float32(math.Sqrt(v.X*v.X+v.Y*v.Y+v.Z*v.Z))*8 + 8
		features[i] = terrainFeature{terrainFeatureCenter(p), float64(radius * radius), random.unit()*.4 + single(.2), 1}
	}
	return func(x, y, z float64) float64 {
		base := a.fractal(x*.007, y*.007, z*.007, 4, .45, 1.8) * 1.5
		detail := b.fractal(x*.007*5, y*.007*5, z*.007*5, 4, .5, 2) * .2
		layer := base*.08 + detail*2
		crest := 0.0
		for _, feature := range features {
			dx, dy, dz := float64(feature.point.x)-x, float64(feature.point.y)-y, float64(feature.point.z)-z
			distance := dx*dx + dy*dy + dz*dz
			if distance <= feature.square {
				t := math.Max(0, distance/feature.square+detail*1.2)
				shape := math.Max(0, -15*(t*t*t)+21.833333333334*(t*t)-7.533333333333*t+.7+detail)
				crest = math.Max(crest, shape*shape*feature.height)
			}
		}
		return 200 + (crest + layer + .2) + .1
	}
}

// terrainEllipticHeight 职责：生成算法 10 高度求值器；使用：基础地表；边界：只生成高度数据。
func terrainEllipticHeight(a, b *simplexNoise, random *random) func(float64, float64, float64) float64 {
	c, d := newSimplexNoise(random.integer()), newSimplexNoise(random.integer())
	table, seed := sphericNormals(), int(random.integer())
	features := make([]terrainFeature, 10)
	for i := range features {
		seed = (seed + 1) & 65535
		v := table[seed]
		p := terrainPoint{float32(v.X), float32(v.Y), float32(v.Z)}
		radius := single(random.unit()*10 + 40)
		shape := random.unit()
		ratioRoll := random.unit()
		ratio := terrainRemap(0, 1, .2, 1.0/3, ratioRoll)
		if shape > .5 {
			ratio = terrainRemap(0, 1, 3, 5, ratioRoll)
		}
		features[i] = terrainFeature{terrainFeatureCenter(p), radius * radius, 1 + random.unit(), ratio}
	}
	return func(x, y, z float64) float64 {
		tx := terrainLevelize(x*.007, 1, 1) + c.noise(x*.05, y*.05, z*.05)*.04
		ty := terrainLevelize(y*.007, 1, 1) + c.noise(y*.05, z*.05, x*.05)*.04
		tz := terrainLevelize(z*.007, 1, 1) + c.noise(z*.05, x*.05, y*.05)*.04
		ridge := math.Abs(d.noise(tx, ty, tz))
		cut := math.Min(1, math.Max(0, (.16-ridge)*10))
		cut *= cut
		broad := math.Min(1, math.Max(0, (c.fractal(y*.005, z*.005, x*.005, 4, .5, 2)+.22)*5))
		fine := math.Abs(d.fractal(tx*1.5, ty*1.5, tz*1.5, 2, .5, 2))
		ellipseNoise := b.fractal(x*.007*5, y*.007*5, z*.007*5, 4, .5, 2)
		crest := 0.0
		for _, feature := range features {
			dx, dy, dz := float64(feature.point.x)-x, float64(feature.point.y)-y, float64(feature.point.z)-z
			distance := (feature.eccentricity*dx*dx + dy*dy + dz*dz) * terrainRemap(-1, 1, .2, 5, ellipseNoise)
			if distance < feature.square {
				inverse := single(math.Sqrt(single(distance / feature.square)))
				profile := math.Max(0, 1-inverse*inverse*inverse*inverse+ellipseNoise*.2*2)
				crest = math.Max(crest, feature.height*profile)
			}
		}
		x += math.Sin(y*.15) * 2
		y += math.Sin(z*.15) * 2
		z += math.Sin(x*.15) * 2
		x, y, z = x*.007, y*.007, z*.007
		base := power((a.fractal(x*.6, y*.6, z*.6, 4, .5, 1.8)+1)*.5, 1.3)
		detail := terrainRemap(-1, 1, -.1, .15, b.fractal(x*6, y*6, z*6, 5, .5, 2))
		shift := b.fractal(x*5*3, y*5, z*5, 1, .5, 2)
		detail2 := b.fractal(x*5*3+shift*.3, y*5+shift*.3, z*5+shift*.3, 5, .5, 2) * .1
		base = math.Min(1, single(terrainLevelize(terrainLevelize(base, 1, 4), 1, 1)))
		switch {
		case base <= .4:
			base += detail
		case base <= .8:
			base += detail2
		}
		height := math.Max(base*2.5-base*crest, detail*2)
		attenuation := (2 - height) / 2
		height -= cut * 1.2 * broad * attenuation
		if height >= 0 {
			height += (ridge*.25 + fine*.6) * attenuation
		}
		height -= .1
		return 200 + height + .1
	}
}

// terrainFeatureCenter 职责：将原版 Vector4 归一化为地貌中心；使用：陨坑与椭圆；边界：与 Vector3 的中间舍入不同。
func terrainFeatureCenter(p terrainPoint) terrainPoint {
	length := float32(math.Sqrt(float64(p.square())))
	if length <= 1e-5 {
		return terrainPoint{}
	}
	return terrainPoint{p.x / length, p.y / length, p.z / length}.scale(200)
}

// terrainRemap 职责：将数值转换到另一区间；使用：地貌参数计算；边界：保留区间外值。
func terrainRemap(sourceMin, sourceMax, targetMin, targetMax, value float64) float64 {
	return (value-sourceMin)/(sourceMax-sourceMin)*(targetMax-targetMin) + targetMin
}
