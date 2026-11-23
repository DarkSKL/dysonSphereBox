package galaxy

// 文件职责：复现原版星球气体主题与基础产出。
// 使用：选档时从星系与存档主题列表计算；边界：未知主题集不提供产出。
import "math"

// GasYield 职责：保存一个气体产出；使用：轨道采集规划；边界：基础速率单位为每秒。
type GasYield struct {
	ItemID int64
	Rate   float64
}

// GasYields 职责：按恒星和星球生成顺序复现气体主题去重；使用：资源快照；边界：主题表超出支持范围时只保留已知气态轨道标志。
func GasYields(stars []Star, planets []Planet, themes map[int64]SurfaceTheme) map[int64][]GasYield {
	type theme struct {
		items [2]int64
		rates [2]float64
	}
	gasThemes := map[int]theme{
		2:  {[2]int64{1120, 1121}, [2]float64{.96, .04}},
		3:  {[2]int64{1120, 1121}, [2]float64{.96, .04}},
		4:  {[2]int64{1011, 1120}, [2]float64{.7, .3}},
		5:  {[2]int64{1011, 1120}, [2]float64{.7, .3}},
		21: {[2]int64{1120, 1121}, [2]float64{.84, .16}},
	}
	out := make(map[int64][]GasYield)
	for _, planet := range planets {
		if !planet.Gas {
			continue
		}
		out[int64(planet.ID)] = nil
		chosen, ok := gasThemes[themes[int64(planet.ID)].ID]
		if !ok {
			continue
		}
		r := newRandom(planet.InfoSeed)
		for i := 0; i < 17; i++ {
			r.unit()
		}
		gas := newRandom(r.integer())
		scale := power(starResourceCoefficient(stars[planet.StarID-1]), .3)
		for i, item := range chosen.items {
			rate := single(single(chosen.rates[i]) * (single(gas.unit())*single(.19090915) + single(.9090909)))
			out[int64(planet.ID)] = append(out[int64(planet.ID)], GasYield{item, single(rate * scale)})
		}
	}
	return out
}

// starResourceCoefficient 职责：恢复恒星资源系数；使用：矿脉与气体生成；边界：坐标输入单位为游戏米。
func starResourceCoefficient(star Star) float64 {
	p := star.Position
	distance := single(math.Sqrt(p.X*p.X+p.Y*p.Y+p.Z*p.Z) / 2400000)
	factor := single(distance / 32)
	if factor > 1 {
		for i := 0; i < 5; i++ {
			factor = single(single(math.Log(factor)) + 1)
		}
	}
	return single(power(7, factor) * single(.6))
}
