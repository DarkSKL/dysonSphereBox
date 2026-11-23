package galaxy

// 文件职责：按原版类型、温度与生成顺序选择星球主题。
// 使用：气体产出、基础地形及矿脉生成；边界：未知主题表保留未知状态。
import "math"

const (
	planetVolcano = iota + 1
	planetOcean
	planetDesert
	planetIce
	planetGas
)

// SurfaceTheme 职责：保存已匹配的原版主题；使用：地表及含水资源；边界：零 ID 代表未知来源。
type SurfaceTheme struct {
	ID          int
	Algorithm   int
	WaterItemID int64
	WaterHeight float64
	ModX        float64
	ModY        float64
}

// themeRow 职责：保存生成需要的主题数据；使用：版本支持范围内的主题筛选；边界：负水面物品代表不可采集液面。
type themeRow struct {
	kind, distribution, algorithm int
	temperature                   float64
	waterItemID                   int64
	waterHeight                   float64
}

var vanillaThemes = [...]themeRow{
	{},
	{2, 1, 1, 0, 1000, 0}, {5, 0, 0, 2, 0, 0}, {5, 0, 0, 1, 0, 0},
	{5, 0, 0, -1, 0, 0}, {5, 0, 0, -2, 0, 0}, {3, 0, 2, 2, 0, 0},
	{3, 0, 2, -1, 0, 0}, {2, 2, 1, 0, 1000, 0}, {1, 0, 5, 5, -1, -2.5},
	{4, 0, 3, -5, 1000, 0}, {3, 0, 4, -2, 0, 0}, {3, 0, 3, 1, 0, 0},
	{1, 2, 3, 4, 1116, 0}, {2, 2, 1, 0, 1000, 0}, {2, 2, 6, 0, 1000, 0},
	{2, 2, 7, 0, 1000, 0}, {3, 0, 2, 1, 0, 0}, {2, 2, 1, 0, 1000, 0},
	{3, 2, 8, 1, 0, 0}, {3, 0, 9, -2, -2, 0}, {5, 2, 0, 1, 0, 0},
	{2, 2, 10, 0, 1000, -.7}, {3, 2, 11, .08, 0, 0}, {4, 0, 12, -4, 0, 0},
	{2, 2, 13, 0, 0, -2},
}

var vanillaTerrainMods = map[int][4]float64{
	6: {1, 1, 0, 0}, 7: {0, 0, 1, 1}, 10: {0, 0, .34, -.15},
	11: {0, 0, 1, 1}, 12: {0, 0, 1, 1}, 13: {0, 0, 1, 1}, 15: {0, 0, 1, 1},
	16: {0, 0, 1, 1}, 17: {0, 0, 1, 1}, 19: {1.5, 1.5, -5, -5}, 20: {6, 6, 8, 8},
	22: {1, 1, 1, 1}, 23: {1.5, 1.5, 1, 1}, 24: {1, 1, 1, 1}, 25: {1, 1, 3, 3},
}

// Themes 职责：复现原版星系主题选择；使用：资源快照；边界：仅接受可识别的存档出生主题列表。
func Themes(stars []Star, planets []Planet, savedThemeIDs []int64) map[int64]SurfaceTheme {
	out := make(map[int64]SurfaceTheme)
	enabled := make(map[int]bool)
	for _, id := range savedThemeIDs {
		if id < 1 || id >= int64(len(vanillaThemes)) {
			return out
		}
		enabled[int(id)] = true
	}
	if len(enabled) == 0 {
		return out
	}
	used := make(map[int]bool)
	orbitRadii := make(map[int]float64)
	previousStar, habitable := 0, 0
	target := math.Max(11, math.Ceil(single(float64(len(stars))*single(.29))))
	for _, planet := range planets {
		if previousStar != planet.StarID {
			clear(used)
			previousStar = planet.StarID
		}
		orbitRadii[planet.ID] = planet.OrbitRadius
		random := newRandom(planet.InfoSeed)
		for i := 0; i < 10; i++ {
			random.unit()
		}
		oceanRoll, typeRoll, themeRoll := random.unit(), random.unit(), random.unit()
		random.unit()
		random.unit()
		modXRoll, modYRoll := random.unit(), random.unit()
		kind, temperature := planetGas, 0.0
		if !planet.Gas {
			radius := planet.OrbitRadius
			if planet.ParentPlanetID != 0 {
				radius = orbitRadii[planet.ParentPlanetID]
			}
			habitableRadius := stars[planet.StarID-1].HabitableRadius
			distance, bias := 1000.0, 1000.0
			if habitableRadius > 0 && radius > 0 {
				distance = single(radius / habitableRadius)
				bias = math.Abs(single(math.Log(distance)))
			}
			scale := single(math.Max(1, math.Min(2, single(math.Sqrt(habitableRadius)))) - single(.04))
			chance := single(float64(int(target)-habitable) / float64(len(stars)-planet.StarID+1))
			chance = math.Max(single(.08), math.Min(single(.8), lerp(chance, single(.35), .5)))
			temperature = single(single(1.2)/(distance+single(.2)) - 1)
			limit := power(math.Min(1, math.Max(0, single(single(bias*scale)/chance))), single(chance*10))
			switch {
			case planet.StarID > 1 && oceanRoll > limit, planet.StarID == 1 && planet.ParentPlanetID != 0 && planet.OrbitIndex == 1:
				kind = planetOcean
				habitable++
			case distance < single(.833333):
				kind = planetVolcano
				if typeRoll < math.Max(single(.15), single(distance*2.5-single(.85))) {
					kind = planetDesert
				}
			case distance < single(1.2), typeRoll < single(single(.9)/distance-single(.1)):
				kind = planetDesert
			default:
				kind = planetIce
			}
		}
		var candidates []int
		for id, theme := range vanillaThemes {
			if !enabled[id] || used[id] {
				continue
			}
			if planet.StarID == 1 && kind == planetOcean {
				if theme.distribution == 1 {
					candidates = append(candidates, id)
				}
				continue
			}
			validTemperature := single(theme.temperature)*temperature >= single(-.1)
			if kind == planetDesert && math.Abs(theme.temperature) < .5 {
				validTemperature = math.Abs(temperature) < math.Abs(single(theme.temperature))+single(.1)
			}
			if kind == theme.kind && validTemperature && (theme.distribution == 0 || theme.distribution == 2 && planet.StarID > 1) {
				candidates = append(candidates, id)
			}
		}
		for _, allowReuse := range []bool{false, true} {
			if len(candidates) > 0 {
				break
			}
			for id, theme := range vanillaThemes {
				if enabled[id] && theme.kind == planetDesert && (allowReuse || !used[id]) {
					candidates = append(candidates, id)
				}
			}
		}
		if len(candidates) == 0 {
			continue
		}
		id := candidates[int(themeRoll*float64(len(candidates)))]
		used[id] = true
		theme := vanillaThemes[id]
		mod := vanillaTerrainMods[id]
		out[int64(planet.ID)] = SurfaceTheme{
			ID: id, Algorithm: theme.algorithm, WaterItemID: theme.waterItemID, WaterHeight: single(theme.waterHeight),
			ModX: single(mod[0]) + modXRoll*single(single(mod[1])-single(mod[0])),
			ModY: single(mod[2]) + modYRoll*single(single(mod[3])-single(mod[2])),
		}
	}
	return out
}
