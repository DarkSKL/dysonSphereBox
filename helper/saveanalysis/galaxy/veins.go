package galaxy

// 文件职责：恢复未建厂星球的初始矿群和矿点。
// 使用：根据主题、存档倍率和地形生成资源；边界：已建厂星球必须读存档矿点。
import (
	"context"
	"math"
)

// GeneratedVein 职责：保存生成出的一个初始矿点；使用：未建厂星球资源与布局；边界：位置为局部游戏米。
type GeneratedVein struct {
	Group, Kind int
	Amount      int64
	Position    Vector
}

// veinCluster 职责：保存候选矿群和类型；使用：矿点展开；边界：出生矿群与普通矿群顺序固定。
type veinCluster struct {
	kind   int
	center terrainPoint
}

// Veins 职责：离线恢复星球矿群及每个矿点；使用：未建厂星球；边界：出生星球需要已有的存档矿点。
func Veins(planet Planet, star Star, theme SurfaceTheme, grid HeightGrid, multiplier float64) ([]GeneratedVein, bool) {
	nodes, ok, _ := VeinsContext(context.Background(), planet, star, theme, grid, multiplier)
	return nodes, ok
}

// VeinsContext 职责：按地形生成可取消的矿脉；使用：候选精化；边界：取消后不返回部分矿点。
func VeinsContext(ctx context.Context, planet Planet, star Star, theme SurfaceTheme, grid HeightGrid, multiplier float64) ([]GeneratedVein, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	profile, ok := vanillaVeins[theme.ID]
	if !ok || grid.Precision != 200 || theme.ID == 1 || multiplier <= 0 {
		return nil, false, nil
	}
	var spots [15]int
	var count, opacity [15]float32
	copy(spots[1:], profile.spots[:])
	copy(count[1:], profile.count[:])
	copy(opacity[1:], profile.opacity[:])
	roll := newRandom(planet.GenSeed)
	for i := 0; i < 5; i++ {
		roll.integer()
	}
	placement := newRandom(roll.integer())
	exponent := 1.0
	switch star.Kind {
	case 0:
		exponents := [...]float64{2.5, 1, .7, .6, 1, .4, 1.6}
		if star.Spectrum >= 0 && star.Spectrum < len(exponents) {
			exponent = exponents[star.Spectrum]
		}
	case 1:
		exponent = 2.5
	case 2:
		exponent = 3.5
		veinBonus(roll, &spots, &count, &opacity, 9, 2, .45, 1)
		veinBonus(roll, &spots, &count, &opacity, 10, 2, .45, 1)
		veinBonus(roll, &spots, &count, &opacity, 12, 1, .5, .3)
	case 3, 4:
		exponent = 4.5
		if star.Kind == 4 {
			exponent = 5
		}
		veinBonus(roll, &spots, &count, &opacity, 14, 1, .65, .3)
	}
	for _, rare := range profile.rare {
		probability := rare.odds
		if planet.StarID == 1 {
			probability = rare.birthOdds
		}
		probability = single(1 - power(1-single(probability), exponent))
		if roll.unit() >= probability {
			continue
		}
		kind := rare.kind
		veinOdds := float32(1 - power(1-single(rare.opacity), exponent))
		spots[kind]++
		count[kind], opacity[kind] = veinOdds, veinOdds
		for n := 1; n < 12 && roll.unit() < single(rare.cluster); n++ {
			spots[kind]++
		}
	}
	coefficient, oilMultiplier := starResourceCoefficient(star), 1.0
	if multiplier <= single(.1001) {
		if coefficient > 1 {
			coefficient = power(coefficient, .8)
		}
		coefficient = single(coefficient * single(.7))
		oilMultiplier = .5
	}
	spacing := float32(float64(float32(2.1)) / 200)
	origin := terrainPoint{veinDirection(placement), float32(placement.unit()) - .5, veinDirection(placement)}
	origin = origin.unit().scale(float32(placement.unit()*.4 + .2))
	clusters, err := generateVeinClusters(ctx, grid, theme.Algorithm, spots, placement, origin, spacing)
	if err != nil {
		return nil, false, err
	}
	out := make([]GeneratedVein, 0, len(clusters)*20)
	for j, cluster := range clusters {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		kind := cluster.kind
		target := int(math.RoundToEven(single(float64(count[kind]) * float64(veinRand(placement, 20, 25)))))
		if kind == 7 {
			target = 1
		}
		points := growVeinCluster(target, placement)
		center := cluster.center.unit()
		right, forward := veinTangentAxes(center)
		scale := coefficient
		if kind == 7 {
			scale = power(scale, .5)
		}
		amount := max(20, int(math.RoundToEven(single(float64(opacity[kind])*100000*scale))))
		spread := 15000
		if amount < 16000 {
			spread = int(math.Floor(single(float64(amount) * .9375)))
		}
		for _, point := range points {
			value := math.RoundToEven(single(float64(veinRand(placement, amount-spread, amount+spread+1)) * single(1.1)))
			switch {
			case kind == 7:
				value = math.RoundToEven(single(value * oilMultiplier))
			case multiplier >= 99.5:
				value = 1000000000
			default:
				value = math.RoundToEven(single(value * multiplier))
			}
			position := center.add(right.scale(point.x).add(forward.scale(point.y)).scale(spacing))
			if kind == 7 {
				position = oilGridSnap(position)
			}
			height := veinHeight(grid, position)
			if theme.Algorithm != 7 && theme.WaterItemID != 0 && height < 200 && kind != 7 {
				continue
			}
			position = position.unit().scale(float32(height))
			out = append(out, GeneratedVein{j + 1, kind, max(1, int64(value)), position.vector()})
		}
	}
	return out, true, nil
}

// veinBonus 职责：追加特殊恒星保底矿群；使用：星系种子恢复；边界：保留原版随机调用次数。
func veinBonus(r *random, spots *[15]int, count, opacity *[15]float32, kind, minimum int, odds, concentration float64) {
	spots[kind] += minimum
	for i := 1; i < 12 && r.unit() < single(odds); i++ {
		spots[kind]++
	}
	count[kind], opacity[kind] = .7, float32(concentration)
}

// generateVeinClusters 职责：确定矿群中心；使用：矿点展开；边界：最多恢复原版的 512 个中心。
func generateVeinClusters(ctx context.Context, grid HeightGrid, algorithm int, spots [15]int, r *random, origin terrainPoint, spacing float32) ([]veinCluster, error) {
	var clusters []veinCluster
	for kind := 1; kind < len(spots) && len(clusters) < 512; kind++ {
		attempts := spots[kind]
		if attempts > 1 {
			attempts += veinRand(r, -1, 2)
		}
		underwater, separation := 0, 196.0
		if kind == 7 {
			underwater = (attempts*15 + 99) / 100
			separation = 100
		}
		threshold := float64(spacing) * float64(spacing) * separation
		for n := 0; n < attempts && len(clusters) < 512; n++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for retry := 0; retry < 200; retry++ {
				center := terrainPoint{veinDirection(r), veinDirection(r), veinDirection(r)}
				if kind != 7 {
					center = center.add(origin)
				}
				center = center.unit()
				height := 0.0
				if algorithm != 7 || kind == 13 {
					height = veinHeight(grid, center)
				}
				if !veinCenterAllowed(algorithm, kind, height, underwater) {
					continue
				}
				open := true
				for _, existing := range clusters {
					if float64(existing.center.diff(center).square()) < threshold {
						open = false
						break
					}
				}
				if !open {
					continue
				}
				clusters = append(clusters, veinCluster{kind, center})
				if algorithm != 7 && kind == 7 && height <= 199 {
					underwater--
					attempts++
				}
				break
			}
		}
	}
	return clusters, nil
}

// veinCenterAllowed 职责：判断原版矿群水深及地形限制；使用：位置抽样；边界：算法 7 的覆盖规则单独处理。
func veinCenterAllowed(algorithm, kind int, height float64, underwater int) bool {
	if algorithm == 7 {
		return kind != 13 || height <= 196
	}
	if height < 200 && kind != 7 || kind == 7 && height < 200.5 && (height > 199 || underwater <= 0) {
		return false
	}
	if algorithm == 11 {
		return !(kind <= 2 && height > 200+single(.7) || (kind == 3 || kind == 4) && height <= 200+single(.7))
	}
	if algorithm == 12 && kind == 8 {
		return height >= 200+single(1.2)
	}
	return !(algorithm == 13 && kind <= 4 && height > 200+single(.7))
}

// growVeinCluster 职责：从群中心展开局部二维矿点；使用：原版矿点分布；边界：中心总是第一个点。
func growVeinCluster(count int, r *random) []terrainPoint {
	points := []terrainPoint{{}}
	for round := 0; round < 20 && len(points) < count; round++ {
		length := len(points)
		for i := 0; i < length && len(points) < count; i++ {
			base := points[i]
			if base.square() > 36 {
				continue
			}
			angle := r.unit() * math.Pi * 2
			direction := terrainPoint{x: float32(math.Cos(angle)), y: float32(math.Sin(angle))}
			point := base.add(direction.add(base.scale(.2)).unit())
			duplicate := false
			for _, existing := range points {
				if existing.diff(point).square() < .85 {
					duplicate = true
					break
				}
			}
			if !duplicate {
				points = append(points, point)
			}
		}
	}
	return points
}

// veinTangentAxes 职责：将水平基向量旋转到局部切平面；使用：定位单矿点；边界：在南极使用确定的反向轴。
func veinTangentAxes(up terrainPoint) (terrainPoint, terrainPoint) {
	denominator := 1 + float64(up.y)
	if denominator < 1e-6 {
		return terrainPoint{x: -1}, terrainPoint{z: 1}
	}
	x, z := float64(up.x), float64(up.z)
	v := float32(-x * z / denominator)
	return terrainPoint{float32(1 - x*x/denominator), -up.x, v}, terrainPoint{v, -up.z, float32(1 - z*z/denominator)}
}

// oilGridSnap 职责：按原版分段格将油井定位到最近的五分格；使用：油井落位；边界：适用半径 200 固态行星。
func oilGridSnap(p terrainPoint) terrainPoint {
	p = p.unit()
	tau := float32(math.Pi) * 2
	lat := float32(math.Asin(float64(p.y)))
	lon := float32(math.Atan2(float64(p.x), float64(-p.z)))
	latGrid := lat / tau * 200
	band := math.Floor(math.Max(0, math.Abs(float64(latGrid))-.1))
	cos := float32(math.Cos(float64(float32(band) / 50 * float32(math.Pi) * .5)))
	n := int(math.Ceil(math.Abs(float64(cos * 200))))
	longitudeSegments := 200
	thresholds := [...]int{0, 7, 15, 19, 27, 39, 53, 72, 90, 113, 139, 176}
	segments := [...]int{1, 4, 8, 16, 20, 32, 40, 60, 80, 100, 120, 160}
	for i, threshold := range thresholds {
		if n <= threshold {
			longitudeSegments = segments[i]
			break
		}
	}
	lonGrid := lon / tau * float32(longitudeSegments)
	lat = float32(math.RoundToEven(float64(latGrid*5))) / 5 / 200 * tau
	lon = float32(math.RoundToEven(float64(lonGrid*5))) / 5 / float32(longitudeSegments) * tau
	cosLat := float32(math.Cos(float64(lat)))
	return terrainPoint{cosLat * float32(math.Sin(float64(lon))), float32(math.Sin(float64(lat))), -cosLat * float32(math.Cos(float64(lon)))}
}

// veinHeight 职责：查询原版矿点落位高度；使用：矿群选择和位置；边界：遵循生成器的第零顶点回退。
func veinHeight(grid HeightGrid, p terrainPoint) float64 {
	if height, ok := TerrainHeightAt(grid, nil, p.vector()); ok {
		return height
	}
	return single(float64(grid.Heights[0]) * single(.01))
}

// veinRand 职责：抽取整数区间；使用：生成流程；边界：单元素区间不推进随机流。
func veinRand(r *random, lower, upper int) int {
	if upper-lower <= 1 {
		return lower
	}
	return lower + int(r.unit()*float64(upper-lower))
}

// veinDirection 职责：抽取单精度方向分量；使用：矿群候选；边界：分量位于 -1 到 1。
func veinDirection(r *random) float32 { return float32(r.unit())*2 - 1 }

// unit 职责：归一化球面或平面方向；使用：矿脉布局；边界：零向量返回零。
func (p terrainPoint) unit() terrainPoint {
	x, y, z := float64(p.x), float64(p.y), float64(p.z)
	length := float32(math.Sqrt(x*x + y*y + z*z))
	if length <= 1e-5 {
		return terrainPoint{}
	}
	return terrainPoint{p.x / length, p.y / length, p.z / length}
}

// vector 职责：转换局部方向或位置；使用：公开查询；边界：保留原始单精度分量。
func (p terrainPoint) vector() Vector { return Vector{float64(p.x), float64(p.y), float64(p.z)} }

// scale 职责：缩放局部向量；使用：矿点高度和采样半径；边界：保留单精度。
func (p terrainPoint) scale(scale float32) terrainPoint {
	return terrainPoint{p.x * scale, p.y * scale, p.z * scale}
}

// add 职责：组合两个局部向量；使用：矿群布局；边界：保留单精度。
func (p terrainPoint) add(other terrainPoint) terrainPoint {
	return terrainPoint{p.x + other.x, p.y + other.y, p.z + other.z}
}

// diff 职责：计算两个局部向量的位移；使用：距离筛选；边界：保留单精度。
func (p terrainPoint) diff(other terrainPoint) terrainPoint {
	return terrainPoint{p.x - other.x, p.y - other.y, p.z - other.z}
}

// square 职责：求向量长度平方；使用：归一化和距离筛选；边界：按原版表达式舍入。
func (p terrainPoint) square() float32 {
	return float32(float64(p.x)*float64(p.x) + float64(p.y)*float64(p.y) + float64(p.z)*float64(p.z))
}
