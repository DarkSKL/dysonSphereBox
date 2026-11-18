package galaxy

// 文件职责：实现受支持星系算法的确定性随机序列与恒星位置。
// 使用：生成器内部恢复位置；边界：不共享随机状态，不生成地形或资源。
import "math"

// random 职责：保存兼容的减法随机状态；使用：每个生成阶段单独创建；边界：不用于安全随机。
type random struct {
	values      [56]int32
	next, other int
}

// newRandom 职责：由保存的种子初始化序列；使用：生成阶段入口；边界：保持有符号 32 位运算。
func newRandom(seed int32) *random {
	r := &random{other: 31}
	n := seed
	if n < 0 {
		n = -n
	}
	if seed == math.MinInt32 {
		n = math.MaxInt32
	}
	a := int32(161803398) - n
	r.values[55] = a
	b := int32(1)
	for i := 1; i < 55; i++ {
		j := 21 * i % 55
		r.values[j] = b
		b = a - b
		if b < 0 {
			b += math.MaxInt32
		}
		a = r.values[j]
	}
	for k := 0; k < 4; k++ {
		for i := 1; i < 56; i++ {
			r.values[i] -= r.values[1+(i+30)%55]
			if r.values[i] < 0 {
				r.values[i] += math.MaxInt32
			}
		}
	}
	return r
}

// sample 职责：推进减法随机状态；使用：随机数换算；边界：保持游戏的 31 位序列。
func (r *random) sample() int32 {
	r.next++
	if r.next >= 56 {
		r.next = 1
	}
	r.other++
	if r.other >= 56 {
		r.other = 1
	}
	v := r.values[r.next] - r.values[r.other]
	if v < 0 {
		v += math.MaxInt32
	}
	r.values[r.next] = v
	return v
}

// unit 职责：转换为半开单位区间；使用：生成参数抽样；边界：保留双精度。
func (r *random) unit() float64 { return float64(r.sample()) * (1.0 / 2147483647.0) }

// positions 职责：恢复星系随机游走和抽样顺序；使用：恒星创建前；边界：最多返回请求数量，不用随机占位点补足。
func positions(seed int32, count int) []Vector {
	r := newRandom(seed)
	walkers := int(r.unit()*2 + 6)
	points := []Vector{{}}
	drunk := []Vector{}
	candidate := func(origin Vector) (Vector, bool) {
		x, y, z := r.unit()*2-1, (r.unit()*2-1)*0.18, r.unit()*2-1
		step := r.unit()
		length := x*x + y*y + z*z
		if length > 1 || length < 1e-8 {
			return Vector{}, false
		}
		scale := (step*(3.5-2.3) + 2) / math.Sqrt(length)
		p := Vector{origin.X + x*scale, origin.Y + y*scale, origin.Z + z*scale}
		for _, v := range points {
			dx, dy, dz := p.X-v.X, p.Y-v.Y, p.Z-v.Z
			if dx*dx+dy*dy+dz*dz < 4 {
				return Vector{}, false
			}
		}
		return p, true
	}
	for i := 0; i < walkers && len(points) < count*4; i++ {
		for j := 0; j < 256; j++ {
			if p, ok := candidate(Vector{}); ok {
				points = append(points, p)
				drunk = append(drunk, p)
				break
			}
		}
	}
	for round := 0; round < 256 && len(points) < count*4; round++ {
		for i := range drunk {
			if r.unit() > 0.7 {
				continue
			}
			for j := 0; j < 256; j++ {
				if p, ok := candidate(drunk[i]); ok {
					drunk[i] = p
					points = append(points, p)
					break
				}
			}
			if len(points) >= count*4 {
				break
			}
		}
	}
	for i := len(points) - 1; i >= 0 && len(points) > count; i-- {
		if i%4 != 0 {
			points = append(points[:i], points[i+1:]...)
		}
	}
	return points
}

// integer 职责：取得兼容随机整数；使用：派生种子；边界：遵循 Sample 换算。
func (r *random) integer() int32 { return int32(r.unit() * 2147483647.0) }
