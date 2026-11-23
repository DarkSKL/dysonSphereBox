package galaxy

// 文件职责：恢复全部星球的轨道参数与保存时刻的位置。
// 使用：星系查询内部；边界：不生成地形、主题或矿脉，不以轨道原点替代真实位置。
import "math"

// planetState 职责：保存轨道生成中间态；使用：卫星关系和位置计算；边界：索引只在所属恒星内有效。
type planetState struct {
	parent, number, orbit                         int
	radius, inclination, longitude, phase, period float64
	gas                                           bool
	infoSeed, genSeed                             int32
	rotation                                      quaternion
}

// quaternion 职责：保存单精度轨道旋转；使用：位置换算；边界：不用于球体编辑。
type quaternion struct{ x, y, z, w float64 }

// multiply 职责：组合轨道旋转；使用：父行星与卫星；边界：保持单精度结果。
func (a quaternion) multiply(b quaternion) quaternion {
	return quaternion{single(a.w*b.x + a.x*b.w + a.y*b.z - a.z*b.y), single(a.w*b.y + a.y*b.w + a.z*b.x - a.x*b.z), single(a.w*b.z + a.z*b.w + a.x*b.y - a.y*b.x), single(a.w*b.w - a.x*b.x - a.y*b.y - a.z*b.z)}
}

// angle 职责：创建轴角旋转；使用：轨道倾角与经度；边界：输入角度，轴为 Y 或 Z。
func angle(degrees float64, y bool) quaternion {
	half := single(degrees / 360)
	half = single(single(half+half)*single(math.Pi)) * 0.5
	s, c := single(math.Sin(half)), single(math.Cos(half))
	if y {
		return quaternion{y: s, w: c}
	}
	return quaternion{z: s, w: c}
}

// rotate 职责：将双精度位置转换到恒星坐标轴；使用：轨道位置；边界：保留游戏四元数旋转表达的计算顺序。
func (q quaternion) rotate(v Vector) Vector {
	v.X *= 2
	v.Y *= 2
	v.Z *= 2
	a := single(q.w*q.w) - 0.5
	b := q.x*v.X + q.y*v.Y + q.z*v.Z
	return Vector{v.X*a + (q.y*v.Z-q.z*v.Y)*q.w + q.x*b, v.Y*a + (q.z*v.X-q.x*v.Z)*q.w + q.y*b, v.Z*a + (q.x*v.Y-q.y*v.X)*q.w + q.z*b}
}

// addPlanet 职责：从独立星球种子恢复轨道；使用：按生成顺序添加；边界：父行星必须已创建。
func (s *starState) addPlanet(parentNumber, orbit, number int, gas bool, seed, genSeed int32) {
	r := newRandom(seed)
	a, b, c, d, e := r.unit(), r.unit(), r.unit(), r.unit(), r.unit()
	p := planetState{parent: -1, number: number, orbit: orbit, gas: gas, infoSeed: seed, genSeed: genSeed}
	factor := power(1.2, single(a*(b-0.5)*0.5))
	if parentNumber == 0 {
		radii := []float64{0, .4, .7, 1, 1.4, 1.9, 2.5, 3.3, 4.3, 5.5, 6.9, 8.4, 10, 11.7, 13.5, 15.4, 17.5}
		base := single(single(radii[orbit]) * s.scale)
		p.radius = single(base * single((factor-1)/math.Max(1, base)+1))
	} else {
		for i, v := range s.planets {
			if v.parent < 0 && v.number == parentNumber {
				p.parent = i
				break
			}
		}
		bodyRadius := 200.0
		if s.planets[p.parent].gas {
			bodyRadius = 800
		}
		p.radius = single(((1600*float64(orbit)+200)*power(s.scale, 0.3)*lerp(factor, 1, 0.5) + bodyRadius) / 40000)
	}
	p.inclination = single(c*16 - 8)
	if p.parent >= 0 {
		p.inclination = single(p.inclination * single(2.2))
	}
	if s.kind >= 3 {
		if p.inclination > 0 {
			p.inclination = single(p.inclination + 3)
		} else {
			p.inclination = single(p.inclination - 3)
		}
	}
	p.longitude = single(d * 360)
	p.phase = single(e * 360)
	gravity := 1.3538551990520382e-6 * s.mass
	if p.parent >= 0 {
		gravity = 1.0830842106853677e-8
	}
	p.period = math.Sqrt(39.478417604357432 * p.radius * p.radius * p.radius / gravity)
	p.rotation = angle(p.longitude, true).multiply(angle(p.inclination, false))
	if p.parent >= 0 {
		p.rotation = s.planets[p.parent].rotation.multiply(p.rotation)
	}
	s.planets = append(s.planets, p)
}

// makePlanets 职责：恢复每种恒星下的星球数量及卫星顺序；使用：恒星参数完成后；边界：随机调用顺序固定，不因省略地形而跳过轨道阶段抽样。
func (s *starState) makePlanets() {
	r := newRandom(s.planetSeed)
	choice, arrangement, offset := r.unit(), r.unit(), r.unit()
	r.unit()
	r.unit()
	r.unit()
	r.unit()
	add := func(parent, orbit, number int, gas bool) {
		seed := r.integer()
		s.addPlanet(parent, orbit, number, gas, seed, r.integer())
	}
	firstOrbit := 2
	if offset > 0.5 {
		firstOrbit = 3
	}
	switch s.kind {
	case 3, 4:
		add(0, 3, 1, false)
		return
	case 2:
		if choice < single(.7) {
			add(0, 3, 1, false)
		} else if arrangement < single(.3) {
			add(0, 3, 1, false)
			add(0, 4, 2, false)
		} else {
			add(0, 4, 1, true)
			add(1, 1, 1, false)
		}
		return
	case 1:
		if choice < single(.3) {
			add(0, firstOrbit, 1, false)
		} else if choice < single(.8) {
			if arrangement < .25 {
				add(0, firstOrbit, 1, false)
				add(0, firstOrbit+1, 2, false)
			} else {
				add(0, 3, 1, true)
				add(1, 1, 1, false)
			}
		} else if arrangement < single(.15) {
			add(0, firstOrbit, 1, false)
			add(0, firstOrbit+1, 2, false)
			add(0, firstOrbit+2, 3, false)
		} else if arrangement < .75 {
			add(0, firstOrbit, 1, false)
			add(0, 4, 2, true)
			add(2, 1, 1, false)
		} else {
			add(0, firstOrbit+1, 1, true)
			add(1, 1, 1, false)
			add(1, 2, 2, false)
		}
		return
	}
	count := 1
	prob := [6]float64{}
	thresholds := [][]float64{{.1, .3, .8}, {.1, .2, .7, .95}, {.4, .9}, {.35, .8}, {.3, .75}, {.3, .75}, {.5}}
	bases := []int{1, 1, 3, 3, 3, 4, 5}
	if s.id == 1 {
		count = 4
	} else {
		count = bases[s.spectr]
		for _, v := range thresholds[s.spectr] {
			if choice >= v {
				count++
			}
		}
		if count <= 3 {
			v := .2
			if s.spectr == 1 || s.spectr == 2 {
				v = .18
			}
			prob[0], prob[1] = v, v
		} else {
			switch s.spectr {
			case 0:
				prob = [6]float64{0, .2, .3}
			case 1:
				prob = [6]float64{0, .18, .28, .28}
			case 2:
				prob = [6]float64{0, .2, .3, .3}
			case 3:
				prob = [6]float64{0, .22, .31, .31}
			case 4:
				prob = [6]float64{.1, .28, .3, .35}
			case 5:
				prob = [6]float64{.1, .22, .28, .35, .35}
			case 6:
				prob = [6]float64{.1, .2, .25, .3, .32, .35}
			}
		}
	}
	number, moon, parent, orbit := 0, 0, 0, 1
	for i := 0; i < count; i++ {
		seed := r.integer()
		genSeed := r.integer()
		gasRoll, endMoon := r.unit(), r.unit()
		gas := false
		if parent == 0 {
			number++
			if i < count-1 && gasRoll < prob[i] {
				gas = true
				if orbit < 3 {
					orbit = 3
				}
			}
			for {
				if s.id == 1 && orbit == 3 {
					gas = true
					break
				}
				remaining, available := count-i, 9-orbit
				if available <= remaining {
					break
				}
				p := single(float64(remaining) / float64(available))
				t := .45
				if orbit <= 3 {
					t = .15
				}
				p = single(lerp(p, 1, t) + single(.01))
				if r.unit() < p {
					break
				}
				orbit++
			}
		} else {
			moon++
		}
		selectedOrbit, selectedNumber := orbit, number
		if parent != 0 {
			selectedOrbit, selectedNumber = moon, moon
		}
		s.addPlanet(parent, selectedOrbit, selectedNumber, gas, seed, genSeed)
		orbit++
		if gas {
			parent = number
			moon = 0
		}
		if moon >= 1 && endMoon < .8 {
			parent = 0
			moon = 0
		}
	}
}

// relativePosition 职责：预测保存 tick 的恒星相对位置；使用：只读坐标查询；边界：返回游戏 AU，包含父行星位移。
func (s *starState) relativePosition(index int, seconds float64) Vector {
	p := s.planets[index]
	phase := seconds/p.period + p.phase/360
	phase -= float64(int(phase + .1))
	a := phase * 2 * math.Pi
	v := p.rotation.rotate(Vector{math.Cos(a) * p.radius, 0, math.Sin(a) * p.radius})
	if p.parent >= 0 {
		parent := s.relativePosition(p.parent, seconds)
		v.X += parent.X
		v.Y += parent.Y
		v.Z += parent.Z
	}
	return v
}
