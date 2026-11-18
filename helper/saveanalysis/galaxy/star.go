package galaxy

// 文件职责：恢复查询需要的恒星质量、光谱与轨道缩放参数。
// 使用：完整星系恢复内部；边界：只保留轨道与命名所需参数，不输出资源或产能推测。
import "math"

// starState 职责：保存星系生成中间值；使用：星球轨道和命名；边界：不作为查询 DTO。
type starState struct {
	id, kind, spectr           int
	seed, nameSeed, planetSeed int32
	mass, scale                float64
	position                   Vector
	name                       string
	planets                    []planetState
}

// single 职责：保留游戏单精度存储点；使用：兼容计算；边界：运算仍由双精度表达。
func single(v float64) float64 { return float64(float32(v)) }

// power 职责：恢复单精度幂函数结果；使用：恒星和星球参数；边界：输入先按单精度收窄。
func power(x, y float64) float64 { return single(math.Pow(single(x), single(y))) }

// lerp 职责：恢复单精度线性插值；使用：生成概率和轨道缩放；边界：调用参数已在单位区间。
func lerp(a, b, t float64) float64 { return single(a + (b-a)*single(t)) }

// normal 职责：恢复生成器的正态抽样；使用：恒星初始质量；边界：遵循中间单精度转换。
func normal(avg, dev, x, y float64) float64 {
	return single(avg + dev*single(math.Sqrt(-2*math.Log(1-x))*math.Sin(2*math.Pi*y)))
}

// makeStar 职责：恢复影响星球和名称的恒星参数；使用：按恒星 ID 顺序创建；边界：不计算无查询用途的亮度与资源字段。
func makeStar(id, count, kind, spectr int, seed int32, p Vector) starState {
	s := starState{id: id, seed: seed, kind: kind, position: p}
	r := newRandom(seed)
	s.nameSeed = r.integer()
	q := newRandom(r.integer())
	r.integer()
	s.planetSeed = r.integer()
	a, b, c, rn, rt := q.unit(), q.unit(), q.unit(), q.unit(), q.unit()
	age := 0.0
	if id == 1 {
		q.unit()
		q.unit()
		value := normal(0, single(0.08), a, b)
		s.mass = power(2, math.Max(single(-0.2), math.Min(single(0.2), value)))
		age = single(c*0.4 + 0.3)
	} else {
		jitter := (q.unit() - 0.5) * 0.2
		q.unit()
		giantChoice := q.unit()*0.4 - 0.2
		level := single(float64(id-1) / float64(count-1))
		avg := lerp(single(-0.98), single(0.88), level)
		if avg >= 0 {
			avg = single(avg + single(0.65))
		} else {
			avg = single(avg - single(0.65))
		}
		dev := single(0.33)
		if kind == 1 {
			avg = single(1.6)
			if giantChoice > -0.08 {
				avg = single(-1.5)
			}
			dev = single(0.3)
		}
		v := normal(avg, dev, a, b)
		if spectr == 0 {
			v = -3
		} else if spectr == 6 {
			v = 3
		}
		if v > 0 {
			v *= 2
		}
		v = single(math.Max(single(-2.4), math.Min(single(4.65), v)) + single(jitter) + 1)
		switch kind {
		case 4:
			s.mass = single(18 + single(a*b)*30)
		case 3:
			s.mass = single(7 + single(a)*11)
		case 2:
			s.mass = single(1 + single(b)*5)
		default:
			s.mass = power(2, v)
		}
		switch kind {
		case 1:
			age = single(single(c)*single(0.04) + single(0.96))
		case 2, 3, 4:
			age = single(single(c)*single(0.4) + 1)
		default:
			if s.mass < 0.5 {
				age = single(single(c)*single(0.12) + single(0.02))
			} else if s.mass < 0.8 {
				age = single(single(c)*single(0.4) + single(0.1))
			} else {
				age = single(single(c)*single(0.7) + single(0.2))
			}
		}
	}
	effective := single((1 - power(math.Min(1, math.Max(0, age)), 20)*0.5) * s.mass)
	temp := single(math.Pow(effective, 0.56+0.14/(math.Log10(effective+4)/math.Log10(5)))*4450 + 1300)
	factor := math.Log10((temp-1300)/4500)/math.Log10(2.6) - 0.5
	if factor < 0 {
		factor *= 4
	}
	factor = math.Max(-4, math.Min(2, factor))
	s.spectr = int(math.RoundToEven(single(single(factor) + 4)))
	s.scale = power(1.35, single(single(factor)+2))
	if s.scale < 1 {
		s.scale = lerp(s.scale, 1, 0.6)
	}
	n, m := single(rn*0.1+0.95), single(rt*0.4+0.8)
	if age >= 1 {
		if s.mass >= 18 {
			s.kind = 4
			s.mass = single(s.mass * (2.5 * m))
		} else if s.mass >= 7 {
			s.kind = 3
			s.mass = single(s.mass * (single(0.2) * n))
			s.scale = single(s.scale * (1.5 * n))
		} else {
			s.kind = 2
			s.mass = single(s.mass * (single(0.2) * n))
		}
		s.spectr = 7
	} else if age >= single(0.96) {
		s.kind = 1
		s.mass = single(single(1-power(age, 30)*0.5) * s.mass)
		s.scale = single(single(3.3) * s.scale)
	} else {
		s.kind = 0
	}
	return s
}
